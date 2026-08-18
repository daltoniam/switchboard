package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupContextTestDirs(t *testing.T) (configDir, repoDir string) {
	t.Helper()
	configDir = t.TempDir()
	repoDir = filepath.Join(configDir, "repo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "docs"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "AGENTS.md"), []byte("# agents"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "docs", "arch.md"), []byte("# arch"), 0600))
	ctxDir := filepath.Join(configDir, "context", "proj")
	require.NoError(t, os.MkdirAll(ctxDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(ctxDir, "sprint.md"), []byte("sprint"), 0600))
	return configDir, repoDir
}

func resourceDef(name, repoDir string) *Definition {
	d := testDefRepo(name, repoDir, "")
	d.Resources["agents"] = Resource{Type: ResourceTypeFile, Repo: "main", Path: "AGENTS.md"}
	d.Resources["arch"] = Resource{Type: ResourceTypeFile, Repo: "main", Path: "docs/arch.md"}
	d.SetBaseDir(filepath.Dir(repoDir))
	return &d
}

func TestAssembleManifest(t *testing.T) {
	configDir, repoDir := setupContextTestDirs(t)
	def := resourceDef("proj", repoDir)

	entries := AssembleManifest(def, configDir)
	require.NotEmpty(t, entries)
	paths := map[string]bool{}
	for _, e := range entries {
		paths[e.Path] = true
	}
	assert.True(t, paths["AGENTS.md"])
	assert.True(t, paths["docs/arch.md"])
}

func TestAssembleManifest_NilResources(t *testing.T) {
	def := &Definition{Version: "1", Name: "x"}
	assert.Empty(t, AssembleManifest(def, t.TempDir()))
}

func TestAssembleManifest_MissingFiles(t *testing.T) {
	def := testDef("p")
	def.Resources["gone"] = Resource{Type: ResourceTypeFile, Path: "/no/such/file.md"}
	entries := AssembleManifest(&def, t.TempDir())
	assert.Empty(t, entries)
}

func TestAssembleManifest_FilesGlob(t *testing.T) {
	configDir, repoDir := setupContextTestDirs(t)
	def := testDefRepo("proj", repoDir, "")
	def.Resources["docs"] = Resource{
		Type:    ResourceTypeFiles,
		Repo:    "main",
		Include: []string{"docs/**/*.md", "AGENTS.md"},
		Exclude: []string{"**/testdata/**"},
	}
	def.SetBaseDir(configDir)

	entries := AssembleManifest(&def, configDir)
	paths := map[string]bool{}
	for _, e := range entries {
		paths[e.Path] = true
	}
	assert.True(t, paths["docs/arch.md"])
	assert.True(t, paths["AGENTS.md"])
}

func TestAssembleManifest_Dedup(t *testing.T) {
	configDir, repoDir := setupContextTestDirs(t)
	def := testDefRepo("proj", repoDir, "")
	def.Resources["a"] = Resource{Type: ResourceTypeFile, Repo: "main", Path: "AGENTS.md"}
	def.Resources["b"] = Resource{Type: ResourceTypeFile, Repo: "main", Path: "AGENTS.md"}
	def.SetBaseDir(configDir)

	entries := AssembleManifest(&def, configDir)
	count := 0
	for _, e := range entries {
		if e.Path == "AGENTS.md" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestReadContextFile(t *testing.T) {
	configDir, repoDir := setupContextTestDirs(t)
	def := resourceDef("proj", repoDir)
	def.Resources["sprint"] = Resource{
		Type: ResourceTypeFile,
		Path: filepath.Join(configDir, "context", "proj", "sprint.md"),
	}

	t.Run("local file resource", func(t *testing.T) {
		content, err := ReadContextFile(def, configDir, filepath.Join(configDir, "context", "proj", "sprint.md"))
		require.NoError(t, err)
		assert.Equal(t, "sprint", content)
	})

	t.Run("repo file", func(t *testing.T) {
		content, err := ReadContextFile(def, configDir, "AGENTS.md")
		require.NoError(t, err)
		assert.Contains(t, content, "agents")
	})

	t.Run("missing", func(t *testing.T) {
		_, err := ReadContextFile(def, configDir, "nonexistent.md")
		assert.Error(t, err)
	})

	t.Run("path escape", func(t *testing.T) {
		_, err := ReadContextFile(def, configDir, "../../secret.txt")
		assert.Error(t, err)
	})
}

func TestAssembleManifestWithRole(t *testing.T) {
	configDir, repoDir := setupContextTestDirs(t)
	def := resourceDef("proj", repoDir)
	def.Agents = &AgentsConfig{
		Roles: map[string]*RoleDefinition{
			"reviewer": {
				ContextOverrides: &ContextConfig{
					RepoIncludes: []string{"AGENTS.md"},
				},
			},
		},
	}

	entries := AssembleManifestWithRole(def, configDir, "reviewer")
	require.NotEmpty(t, entries)
}

func TestAssembleBundle(t *testing.T) {
	configDir, repoDir := setupContextTestDirs(t)
	def := resourceDef("proj", repoDir)
	bundle, included := AssembleBundle(def, configDir, "")
	assert.NotEmpty(t, bundle)
	assert.NotEmpty(t, included)
}

func TestAssembleBundle_MaxBytes(t *testing.T) {
	configDir, repoDir := setupContextTestDirs(t)
	def := resourceDef("proj", repoDir)
	def.Agents = &AgentsConfig{
		Roles: map[string]*RoleDefinition{
			"tiny": {ContextOverrides: &ContextConfig{MaxBytes: 20, RepoIncludes: []string{"AGENTS.md"}}},
		},
	}
	bundle, _ := AssembleBundle(def, configDir, "tiny")
	assert.Contains(t, bundle, "TRUNCATED")
}

func TestGuessMIME(t *testing.T) {
	assert.Equal(t, "text/markdown", GuessMIME("a.md"))
	assert.Equal(t, "application/json", GuessMIME("a.json"))
	assert.Equal(t, "text/plain", GuessMIME("a.bin"))
}

func TestReadResourceContent_RepoMeta(t *testing.T) {
	def := testDefRepo("p", "/tmp/repo", "main")
	content, mime, err := ReadResourceContent(&def, "main")
	require.NoError(t, err)
	assert.Equal(t, "application/json", mime)
	assert.Contains(t, content, `"type":"repo"`)
	assert.Contains(t, content, "main")
}
