package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDef(name string) Definition {
	return Definition{
		Version:   "1",
		Name:      name,
		Resources: map[string]Resource{},
	}
}

func testDefRepo(name, repoPath, branch string) Definition {
	d := testDef(name)
	resID := "main"
	d.Resources[resID] = Resource{Type: ResourceTypeRepo, Path: repoPath, Branch: branch}
	return d
}

func TestDefinition_Validate(t *testing.T) {
	tests := []struct {
		name    string
		def     Definition
		wantErr string
	}{
		{
			name:    "valid minimal",
			def:     testDef("my-project"),
			wantErr: "",
		},
		{
			name:    "missing version",
			def:     Definition{Version: "", Name: "my-project", Resources: map[string]Resource{}},
			wantErr: "unsupported version",
		},
		{
			name:    "wrong version",
			def:     Definition{Version: "2", Name: "my-project", Resources: map[string]Resource{}},
			wantErr: "unsupported version",
		},
		{
			name:    "missing name",
			def:     Definition{Version: "1", Name: "", Resources: map[string]Resource{}},
			wantErr: "name is required",
		},
		{
			name:    "name too long",
			def:     Definition{Version: "1", Name: string(make([]byte, 129)), Resources: map[string]Resource{}},
			wantErr: "exceeds 128 characters",
		},
		{
			name:    "name starts with dash",
			def:     Definition{Version: "1", Name: "-bad", Resources: map[string]Resource{}},
			wantErr: "does not match pattern",
		},
		{
			name:    "name with spaces",
			def:     Definition{Version: "1", Name: "bad name", Resources: map[string]Resource{}},
			wantErr: "does not match pattern",
		},
		{
			name:    "name with dots and dashes",
			def:     testDef("my-project.v2"),
			wantErr: "",
		},
		{
			name:    "name with underscore",
			def:     testDef("my_project"),
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.def.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestDefinition_ValidateResources(t *testing.T) {
	t.Run("rejects files with both repo and root", func(t *testing.T) {
		def := testDef("p")
		def.Resources["docs"] = Resource{
			Type:    ResourceTypeFiles,
			Repo:    "main",
			Root:    "/tmp",
			Include: []string{"**/*.md"},
		}
		def.Resources["main"] = Resource{Type: ResourceTypeRepo, Path: "/tmp/repo"}
		err := def.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "both repo and root")
	})

	t.Run("rejects dangling repo ref", func(t *testing.T) {
		def := testDef("p")
		def.Resources["readme"] = Resource{
			Type: ResourceTypeFile,
			Repo: "missing",
			Path: "README.md",
		}
		err := def.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "does not reference")
	})

	t.Run("rejects repo ref to non-repo", func(t *testing.T) {
		def := testDef("p")
		def.Resources["notes"] = Resource{Type: ResourceTypeFile, Path: "a.md"}
		def.Resources["readme"] = Resource{Type: ResourceTypeFile, Repo: "notes", Path: "README.md"}
		err := def.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "type repo")
	})

	t.Run("accepts example project", func(t *testing.T) {
		raw := `{
  "$schema": "https://switchboard.local/schemas/project-v1.json",
  "version": "1",
  "name": "switchboard",
  "description": "Switchboard and related projects",
  "resources": {
    "switchboard": {"type": "repo", "path": "/home/aleks/work/projects/switchboard/repo", "branch": "main"},
    "awesometree": {"type": "repo", "path": "/home/aleks/work/projects/awesometree/repo", "branch": "master"},
    "architecture": {"type": "file", "repo": "switchboard", "path": "docs/architecture.md"},
    "project-instructions": {"type": "file", "repo": "switchboard", "path": "AGENTS.md"},
    "private-notes": {"type": "file", "path": "~/.config/project-interop/context/switchboard/notes.md"},
    "project-specs": {
      "type": "files",
      "repo": "awesometree",
      "include": ["docs/specs/project-interop/**/*.md", "docs/specs/project-interop/schemas/*.json"],
      "exclude": ["**/testdata/**"]
    }
  }
}`
		var def Definition
		require.NoError(t, json.Unmarshal([]byte(raw), &def))
		require.NoError(t, def.Validate())
		assert.Equal(t, "switchboard", def.Name)
		assert.Equal(t, "Switchboard and related projects", def.Description)
		assert.Len(t, def.Resources, 6)
		assert.Equal(t, ResourceTypeRepo, def.Resources["switchboard"].Type)
		assert.Equal(t, ResourceTypeFiles, def.Resources["project-specs"].Type)
		assert.Equal(t, []string{"**/testdata/**"}, def.Resources["project-specs"].Exclude)
	})
}

func TestDefaultConfigDir_IsolatedFromProjectInterop(t *testing.T) {
	dir := DefaultConfigDir()
	assert.Contains(t, dir, "switchboard")
	assert.NotContains(t, dir, "project-interop")

	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config")
	dir = DefaultConfigDir()
	assert.Equal(t, filepath.Join("/tmp/xdg-config", "switchboard"), dir)
	assert.NotContains(t, dir, "project-interop")
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, "work"), ExpandHome("~/work"))
	assert.Equal(t, "/absolute/path", ExpandHome("/absolute/path"))
	assert.Equal(t, "relative/path", ExpandHome("relative/path"))
}

func TestMerge_Resources(t *testing.T) {
	base := testDefRepo("myproject", "~/work/base", "main")
	overlay := testDef("myproject")
	overlay.Resources = map[string]Resource{
		"main": {Type: ResourceTypeRepo, Path: "~/work/base", Branch: "develop"},
	}

	result, err := Merge(&base, &overlay)
	require.NoError(t, err)
	assert.Equal(t, "myproject", result.Name)
	assert.Equal(t, "~/work/base", result.PrimaryRepo())
	assert.Equal(t, "develop", result.PrimaryBranch())
}

func TestMerge_ConflictingNames(t *testing.T) {
	base := testDef("a")
	overlay := testDef("b")
	_, err := Merge(&base, &overlay)
	assert.ErrorContains(t, err, "different names")
}

func TestMerge_Launch(t *testing.T) {
	base := testDef("p")
	base.Launch = &LaunchConfig{
		Prompt: "base prompt",
		Env:    map[string]string{"A": "1", "B": "2"},
	}
	overlay := testDef("p")
	overlay.Launch = &LaunchConfig{
		Prompt: "overlay prompt",
		Env:    map[string]string{"B": "3", "C": "4"},
	}

	result, err := Merge(&base, &overlay)
	require.NoError(t, err)
	assert.Equal(t, "overlay prompt", result.Launch.Prompt)
	assert.Equal(t, "1", result.Launch.Env["A"])
	assert.Equal(t, "3", result.Launch.Env["B"])
	assert.Equal(t, "4", result.Launch.Env["C"])
}

func TestMerge_Tools(t *testing.T) {
	base := testDef("p")
	base.Tools = map[string]*ScopeRule{
		"proxy": {
			Allow: []string{"github_*"},
			Deny:  []string{"github_delete_*"},
			Defaults: map[string]map[string]any{
				"github_*": {"owner": "base-org"},
			},
		},
	}
	overlay := testDef("p")
	overlay.Tools = map[string]*ScopeRule{
		"proxy": {
			Allow: []string{"linear_*"},
			Deny:  []string{"linear_delete_*"},
			Defaults: map[string]map[string]any{
				"github_*": {"owner": "overlay-org"},
			},
		},
	}

	result, err := Merge(&base, &overlay)
	require.NoError(t, err)

	rule := result.Tools["proxy"]
	assert.Equal(t, []string{"github_*", "linear_*"}, rule.Allow)
	assert.Equal(t, []string{"github_delete_*", "linear_delete_*"}, rule.Deny)
	assert.Equal(t, "overlay-org", rule.Defaults["github_*"]["owner"])
}

func TestStore_CRUD(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	def := testDefRepo("test-project", "~/work/test", "")
	require.NoError(t, store.CreateDefinition(&def))

	got, ok := store.Definition("test-project")
	require.True(t, ok)
	assert.Equal(t, "test-project", got.Name)

	names := store.Names()
	assert.Contains(t, names, "test-project")

	all := store.All()
	assert.Len(t, all, 1)

	_, err := store.Update("test-project", json.RawMessage(`{"resources":{"main":{"type":"repo","path":"~/work/test","branch":"develop"}}}`))
	require.NoError(t, err)

	got, _ = store.Definition("test-project")
	assert.Equal(t, "develop", got.PrimaryBranch())

	require.NoError(t, store.DeleteDefinition("test-project"))
	_, ok = store.Definition("test-project")
	assert.False(t, ok)
}

func TestStore_CreateDuplicate(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	def := testDef("dup")
	require.NoError(t, store.CreateDefinition(&def))

	err := store.CreateDefinition(&def)
	assert.ErrorContains(t, err, "already exists")
}

func TestStore_Load(t *testing.T) {
	dir := t.TempDir()
	projDir := filepath.Join(dir, "projects")
	require.NoError(t, os.MkdirAll(projDir, 0700))

	def := testDefRepo("loaded-project", "~/work/test", "")
	data, _ := json.Marshal(def)
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "loaded-project.project.json"), data, 0600))

	store := NewStore(dir)
	require.NoError(t, store.Load())

	got, ok := store.Definition("loaded-project")
	require.True(t, ok)
	assert.Equal(t, "loaded-project", got.Name)
}

func TestStore_LoadEmpty(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	require.NoError(t, store.Load())
	assert.Empty(t, store.Names())
}

func TestStore_LoadWithRepoLocal(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0700))

	projDir := filepath.Join(dir, "projects")
	require.NoError(t, os.MkdirAll(projDir, 0700))

	baseDef := testDefRepo("merged", repoDir, "")
	baseDef.Resources["notes"] = Resource{Type: ResourceTypeFile, Path: "a.md", Optional: true}
	data, _ := json.Marshal(baseDef)
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "merged.project.json"), data, 0600))

	overlayDef := testDef("merged")
	overlayDef.Resources = map[string]Resource{
		"extra": {Type: ResourceTypeFile, Path: "b.md", Optional: true},
	}
	data, _ = json.Marshal(overlayDef)
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".project.json"), data, 0600))

	store := NewStore(dir)
	require.NoError(t, store.Load())

	got, ok := store.Definition("merged")
	require.True(t, ok)
	assert.Contains(t, got.Resources, "notes")
	assert.Contains(t, got.Resources, "extra")
	assert.Contains(t, got.Resources, "main")
}

func TestStore_DeleteNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	err := store.DeleteDefinition("nonexistent")
	assert.ErrorContains(t, err, "not found")
}

func TestStore_UpdateNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	_, err := store.Update("nonexistent", json.RawMessage(`{}`))
	assert.ErrorContains(t, err, "not found")
}

func TestStore_UpdatePreservesUnknownFieldsAndRepoOverride(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repo")
	projectsDir := filepath.Join(dir, "projects")
	require.NoError(t, os.MkdirAll(repoDir, 0700))
	require.NoError(t, os.MkdirAll(projectsDir, 0700))
	userJSON := `{
		"version":"1","name":"acme",
		"resources":{"main":{"type":"repo","path":` + jsonString(repoDir) + `,"branch":"main"}},
		"custom":{"keep":true}
	}`
	require.NoError(t, os.WriteFile(filepath.Join(projectsDir, "acme.project.json"), []byte(userJSON), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".project.json"), []byte(`{
		"version":"1","name":"acme",
		"resources":{"main":{"type":"repo","path":`+jsonString(repoDir)+`,"branch":"repo-branch"}},
		"repoOnly":"not-user-data"
	}`), 0600))

	store := NewStore(dir)
	require.NoError(t, store.Load())
	updated, err := store.Update("acme", json.RawMessage(`{"launch":{"prompt":"updated"}}`))
	require.NoError(t, err)
	assert.Equal(t, "repo-branch", updated.PrimaryBranch())

	data, err := os.ReadFile(filepath.Join(projectsDir, "acme.project.json"))
	require.NoError(t, err)
	var saved map[string]any
	require.NoError(t, json.Unmarshal(data, &saved))
	resources := saved["resources"].(map[string]any)
	main := resources["main"].(map[string]any)
	assert.Equal(t, "main", main["branch"])
	assert.Equal(t, map[string]any{"keep": true}, saved["custom"])
	assert.NotContains(t, saved, "repoOnly")
}

func TestJsonMergePatch(t *testing.T) {
	base := map[string]any{"a": "1", "b": "2", "c": map[string]any{"d": "3"}}
	patch := map[string]any{"b": nil, "c": map[string]any{"e": "4"}}
	result := jsonMergePatch(base, patch)
	assert.Equal(t, "1", result["a"])
	assert.Nil(t, result["b"])
	inner := result["c"].(map[string]any)
	assert.Equal(t, "3", inner["d"])
	assert.Equal(t, "4", inner["e"])
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestOptionalMissingFile_DoesNotInvalidate(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	def := testDef("opt")
	def.Resources["missing"] = Resource{
		Type:     ResourceTypeFile,
		Path:     filepath.Join(dir, "nope.md"),
		Optional: true,
	}
	require.NoError(t, store.CreateDefinition(&def))
	got, ok := store.Definition("opt")
	require.True(t, ok)
	assert.Equal(t, "opt", got.Name)
	// assembly skips missing optional
	entries := AssembleManifest(got, store.ConfigDir())
	assert.Empty(t, entries)
}

func TestResolvePathRelativeToProjectFile(t *testing.T) {
	base := "/cfg/projects"
	assert.Equal(t, filepath.Join(base, "rel"), ResolvePath(base, "rel"))
	assert.True(t, strings.HasSuffix(ResolvePath(base, "~/x"), filepath.Join("x")))
}
