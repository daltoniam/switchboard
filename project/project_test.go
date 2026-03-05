package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefinition_Validate(t *testing.T) {
	tests := []struct {
		name    string
		def     Definition
		wantErr string
	}{
		{
			name:    "valid minimal",
			def:     Definition{Version: "1", Name: "my-project"},
			wantErr: "",
		},
		{
			name:    "missing version",
			def:     Definition{Version: "", Name: "my-project"},
			wantErr: "unsupported version",
		},
		{
			name:    "wrong version",
			def:     Definition{Version: "2", Name: "my-project"},
			wantErr: "unsupported version",
		},
		{
			name:    "missing name",
			def:     Definition{Version: "1", Name: ""},
			wantErr: "name is required",
		},
		{
			name:    "name too long",
			def:     Definition{Version: "1", Name: string(make([]byte, 129))},
			wantErr: "exceeds 128 characters",
		},
		{
			name:    "name starts with dash",
			def:     Definition{Version: "1", Name: "-bad"},
			wantErr: "does not match pattern",
		},
		{
			name:    "name with spaces",
			def:     Definition{Version: "1", Name: "bad name"},
			wantErr: "does not match pattern",
		},
		{
			name:    "name with dots and dashes",
			def:     Definition{Version: "1", Name: "my-project.v2"},
			wantErr: "",
		},
		{
			name:    "name with underscore",
			def:     Definition{Version: "1", Name: "my_project"},
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

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, "work"), ExpandHome("~/work"))
	assert.Equal(t, "/absolute/path", ExpandHome("/absolute/path"))
	assert.Equal(t, "relative/path", ExpandHome("relative/path"))
}

func TestMerge_Scalars(t *testing.T) {
	base := &Definition{
		Version: "1",
		Name:    "myproject",
		Repo:    "~/work/base",
		Branch:  "main",
	}
	overlay := &Definition{
		Version: "1",
		Name:    "myproject",
		Branch:  "develop",
	}

	result, err := Merge(base, overlay)
	require.NoError(t, err)
	assert.Equal(t, "myproject", result.Name)
	assert.Equal(t, "~/work/base", result.Repo)
	assert.Equal(t, "develop", result.Branch)
}

func TestMerge_ConflictingNames(t *testing.T) {
	base := &Definition{Version: "1", Name: "a"}
	overlay := &Definition{Version: "1", Name: "b"}
	_, err := Merge(base, overlay)
	assert.ErrorContains(t, err, "different names")
}

func TestMerge_Launch(t *testing.T) {
	base := &Definition{
		Version: "1",
		Name:    "p",
		Launch: &LaunchConfig{
			Prompt: "base prompt",
			Env:    map[string]string{"A": "1", "B": "2"},
		},
	}
	overlay := &Definition{
		Version: "1",
		Name:    "p",
		Launch: &LaunchConfig{
			Prompt: "overlay prompt",
			Env:    map[string]string{"B": "3", "C": "4"},
		},
	}

	result, err := Merge(base, overlay)
	require.NoError(t, err)
	assert.Equal(t, "overlay prompt", result.Launch.Prompt)
	assert.Equal(t, "1", result.Launch.Env["A"])
	assert.Equal(t, "3", result.Launch.Env["B"])
	assert.Equal(t, "4", result.Launch.Env["C"])
}

func TestMerge_Tools(t *testing.T) {
	base := &Definition{
		Version: "1",
		Name:    "p",
		Tools: map[string]*ScopeRule{
			"proxy": {
				Allow: []string{"github_*"},
				Deny:  []string{"github_delete_*"},
				Defaults: map[string]map[string]any{
					"github_*": {"owner": "base-org"},
				},
			},
		},
	}
	overlay := &Definition{
		Version: "1",
		Name:    "p",
		Tools: map[string]*ScopeRule{
			"proxy": {
				Allow: []string{"linear_*"},
				Deny:  []string{"linear_delete_*"},
				Defaults: map[string]map[string]any{
					"github_*": {"owner": "overlay-org"},
				},
			},
		},
	}

	result, err := Merge(base, overlay)
	require.NoError(t, err)

	rule := result.Tools["proxy"]
	assert.Equal(t, []string{"github_*", "linear_*"}, rule.Allow)
	assert.Equal(t, []string{"github_delete_*", "linear_delete_*"}, rule.Deny)
	assert.Equal(t, "overlay-org", rule.Defaults["github_*"]["owner"])
}

func TestMerge_Context(t *testing.T) {
	base := &Definition{
		Version: "1",
		Name:    "p",
		Context: &ContextConfig{
			Files:        []string{"a.md"},
			RepoIncludes: []string{"AGENTS.md"},
			MaxBytes:     1000,
		},
	}
	overlay := &Definition{
		Version: "1",
		Name:    "p",
		Context: &ContextConfig{
			Files:        []string{"b.md"},
			RepoIncludes: []string{"docs/arch.md"},
			MaxBytes:     2000,
		},
	}

	result, err := Merge(base, overlay)
	require.NoError(t, err)
	assert.Equal(t, []string{"a.md", "b.md"}, result.Context.Files)
	assert.Equal(t, []string{"AGENTS.md", "docs/arch.md"}, result.Context.RepoIncludes)
	assert.Equal(t, 2000, result.Context.MaxBytes)
}

func TestStore_CRUD(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	def := &Definition{
		Version: "1",
		Name:    "test-project",
		Repo:    "~/work/test",
	}

	require.NoError(t, store.Create(def))

	got, ok := store.Get("test-project")
	require.True(t, ok)
	assert.Equal(t, "test-project", got.Name)

	names := store.Names()
	assert.Contains(t, names, "test-project")

	all := store.All()
	assert.Len(t, all, 1)

	_, err := store.Update("test-project", json.RawMessage(`{"branch": "develop"}`))
	require.NoError(t, err)

	got, _ = store.Get("test-project")
	assert.Equal(t, "develop", got.Branch)

	require.NoError(t, store.Delete("test-project"))
	_, ok = store.Get("test-project")
	assert.False(t, ok)
}

func TestStore_CreateDuplicate(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	def := &Definition{Version: "1", Name: "dup"}
	require.NoError(t, store.Create(def))

	err := store.Create(def)
	assert.ErrorContains(t, err, "already exists")
}

func TestStore_Load(t *testing.T) {
	dir := t.TempDir()
	projDir := filepath.Join(dir, "projects")
	require.NoError(t, os.MkdirAll(projDir, 0700))

	def := &Definition{Version: "1", Name: "loaded-project", Repo: "~/work/test"}
	data, _ := json.Marshal(def)
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "loaded-project.project.json"), data, 0600))

	store := NewStore(dir)
	require.NoError(t, store.Load())

	got, ok := store.Get("loaded-project")
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

	baseDef := &Definition{
		Version: "1",
		Name:    "merged",
		Repo:    repoDir,
		Context: &ContextConfig{Files: []string{"a.md"}},
	}
	data, _ := json.Marshal(baseDef)
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "merged.project.json"), data, 0600))

	overlayDef := &Definition{
		Version: "1",
		Name:    "merged",
		Context: &ContextConfig{Files: []string{"b.md"}},
	}
	data, _ = json.Marshal(overlayDef)
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".project.json"), data, 0600))

	store := NewStore(dir)
	require.NoError(t, store.Load())

	got, ok := store.Get("merged")
	require.True(t, ok)
	assert.Equal(t, []string{"a.md", "b.md"}, got.Context.Files)
}

func TestStore_DeleteNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	err := store.Delete("nonexistent")
	assert.ErrorContains(t, err, "not found")
}

func TestStore_UpdateNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	_, err := store.Update("nonexistent", json.RawMessage(`{}`))
	assert.ErrorContains(t, err, "not found")
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
