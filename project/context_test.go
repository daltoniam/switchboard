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
	base := t.TempDir()
	configDir = filepath.Join(base, "config")
	repoDir = filepath.Join(base, "repo")

	require.NoError(t, os.MkdirAll(filepath.Join(configDir, "context", "test-project"), 0700))
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "docs"), 0700))

	require.NoError(t, os.WriteFile(filepath.Join(configDir, "context", "test-project", "sprint.md"), []byte("Sprint goals"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "AGENTS.md"), []byte("Agent instructions"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "docs", "arch.md"), []byte("Architecture"), 0600))

	return configDir, repoDir
}

func TestAssembleManifest(t *testing.T) {
	configDir, repoDir := setupContextTestDirs(t)

	def := &Definition{
		Version: "1",
		Name:    "test-project",
		Repo:    repoDir,
		Context: &ContextConfig{
			Files:        []string{"sprint.md"},
			RepoIncludes: []string{"AGENTS.md", "docs/arch.md"},
		},
	}

	entries := AssembleManifest(def, configDir)
	assert.Len(t, entries, 3)

	assert.Equal(t, "AGENTS.md", entries[0].Path)
	assert.Equal(t, "repo", entries[0].Source)
	assert.Equal(t, "text/markdown", entries[0].MIMEType)

	assert.Equal(t, "docs/arch.md", entries[1].Path)
	assert.Equal(t, "repo", entries[1].Source)

	assert.Equal(t, "sprint.md", entries[2].Path)
	assert.Equal(t, "store", entries[2].Source)
}

func TestAssembleManifest_NilContext(t *testing.T) {
	def := &Definition{Version: "1", Name: "p"}
	entries := AssembleManifest(def, "/tmp/nonexistent")
	assert.Nil(t, entries)
}

func TestAssembleManifest_MissingFiles(t *testing.T) {
	def := &Definition{
		Version: "1",
		Name:    "p",
		Context: &ContextConfig{
			Files:        []string{"nonexistent.md"},
			RepoIncludes: []string{"nonexistent.md"},
		},
	}
	entries := AssembleManifest(def, "/tmp/nonexistent")
	assert.Empty(t, entries)
}

func TestAssembleManifest_Dedup(t *testing.T) {
	configDir, repoDir := setupContextTestDirs(t)

	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, "context", "test-project", "AGENTS.md"),
		[]byte("Store version"), 0600,
	))

	def := &Definition{
		Version: "1",
		Name:    "test-project",
		Repo:    repoDir,
		Context: &ContextConfig{
			Files:        []string{"AGENTS.md"},
			RepoIncludes: []string{"AGENTS.md"},
		},
	}

	entries := AssembleManifest(def, configDir)
	assert.Len(t, entries, 1)
	assert.Equal(t, "store", entries[0].Source)
}

func TestReadContextFile(t *testing.T) {
	configDir, repoDir := setupContextTestDirs(t)

	def := &Definition{
		Version: "1",
		Name:    "test-project",
		Repo:    repoDir,
	}

	t.Run("reads from store", func(t *testing.T) {
		content, err := ReadContextFile(def, configDir, "sprint.md")
		require.NoError(t, err)
		assert.Equal(t, "Sprint goals", content)
	})

	t.Run("reads from repo", func(t *testing.T) {
		content, err := ReadContextFile(def, configDir, "AGENTS.md")
		require.NoError(t, err)
		assert.Equal(t, "Agent instructions", content)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := ReadContextFile(def, configDir, "nonexistent.md")
		assert.ErrorContains(t, err, "context file not found")
	})
}

func TestAssembleManifestWithRole(t *testing.T) {
	configDir, repoDir := setupContextTestDirs(t)

	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, "context", "test-project", "review.md"),
		[]byte("Review checklist"), 0600,
	))

	def := &Definition{
		Version: "1",
		Name:    "test-project",
		Repo:    repoDir,
		Context: &ContextConfig{
			Files:        []string{"sprint.md"},
			RepoIncludes: []string{"AGENTS.md"},
		},
		Agents: &AgentsConfig{
			Roles: map[string]*RoleDefinition{
				"reviewer": {
					ContextOverrides: &ContextConfig{
						Files: []string{"review.md"},
					},
				},
			},
		},
	}

	t.Run("no role uses project context", func(t *testing.T) {
		entries := AssembleManifestWithRole(def, configDir, "")
		assert.Len(t, entries, 2)
	})

	t.Run("role replaces files", func(t *testing.T) {
		entries := AssembleManifestWithRole(def, configDir, "reviewer")
		assert.Len(t, entries, 2)
		paths := make([]string, len(entries))
		for i, e := range entries {
			paths[i] = e.Path
		}
		assert.Contains(t, paths, "AGENTS.md")
		assert.Contains(t, paths, "review.md")
	})
}

func TestAssembleBundle(t *testing.T) {
	configDir, repoDir := setupContextTestDirs(t)

	def := &Definition{
		Version: "1",
		Name:    "test-project",
		Repo:    repoDir,
		Context: &ContextConfig{
			Files:        []string{"sprint.md"},
			RepoIncludes: []string{"AGENTS.md"},
		},
	}

	bundle, entries := AssembleBundle(def, configDir, "")
	assert.Len(t, entries, 2)
	assert.Contains(t, bundle, "Agent instructions")
	assert.Contains(t, bundle, "Sprint goals")
}

func TestAssembleBundle_MaxBytes(t *testing.T) {
	configDir, repoDir := setupContextTestDirs(t)

	def := &Definition{
		Version: "1",
		Name:    "test-project",
		Repo:    repoDir,
		Context: &ContextConfig{
			Files:        []string{"sprint.md"},
			RepoIncludes: []string{"AGENTS.md"},
			MaxBytes:     50,
		},
	}

	bundle, _ := AssembleBundle(def, configDir, "")
	assert.Contains(t, bundle, "TRUNCATED")
}

func TestGuessMIME(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"readme.md", "text/markdown"},
		{"notes.txt", "text/plain"},
		{"config.json", "application/json"},
		{"config.yaml", "text/yaml"},
		{"config.yml", "text/yaml"},
		{"unknown.xyz", "text/plain"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, GuessMIME(tt.path))
		})
	}
}
