package compass

import (
	"os"
	"path/filepath"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindGitDir(t *testing.T) {
	// Test with current directory (should find .git since we're in a git repo)
	gitDir := FindGitDir("")
	if gitDir != "" {
		assert.True(t, filepath.IsAbs(gitDir))
		assert.Contains(t, gitDir, ".git")
	}
}

func TestFindGitDir_NonExistent(t *testing.T) {
	// Test with a directory that doesn't exist
	gitDir := FindGitDir("/nonexistent/path/that/does/not/exist")
	assert.Empty(t, gitDir)
}

func TestResolveWorktreeGitDir(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "regular worktree path",
			content: "gitdir: /home/user/repo/.git/worktrees/feature-branch\n",
			want:    "/home/user/repo/.git",
		},
		{
			name:    "invalid format",
			content: "invalid content",
			want:    "",
		},
		{
			name:    "direct git reference",
			content: "gitdir: /home/user/repo/.git\n",
			want:    "/home/user/repo/.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveWorktreeGitDir(tt.content)
			if tt.want != "" {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestLoadSaveRepoInstructions(t *testing.T) {
	// Create temp directory to simulate .git
	tmpDir := t.TempDir()

	config := &RepoInstructionsConfig{
		DefaultTier: "small",
		ModelTiers: map[string]string{
			"my-model": "large",
		},
		Instructions: []*mcp.Instruction{
			{
				ID:       "test-inst",
				Name:     "Test Instruction",
				Template: "Hello from repo",
				Enabled:  true,
			},
		},
	}

	// Save
	err := SaveRepoInstructions(tmpDir, config)
	require.NoError(t, err)

	// Verify file was created
	path := filepath.Join(tmpDir, "switchboard", RepoInstructionsFile)
	_, err = os.Stat(path)
	require.NoError(t, err)

	// Load
	loaded := LoadRepoInstructions(tmpDir)
	require.NotNil(t, loaded)

	assert.Equal(t, "small", loaded.DefaultTier)
	assert.Equal(t, "large", loaded.ModelTiers["my-model"])
	assert.Len(t, loaded.Instructions, 1)
	assert.Equal(t, "test-inst", loaded.Instructions[0].ID)
}

func TestLoadRepoInstructions_NotFound(t *testing.T) {
	result := LoadRepoInstructions("/nonexistent")
	assert.Nil(t, result)
}

func TestLoadRepoInstructions_EmptyDir(t *testing.T) {
	result := LoadRepoInstructions("")
	assert.Nil(t, result)
}

func TestMergeInstructions(t *testing.T) {
	global := &mcp.InstructionsConfig{
		DefaultTier: "large",
		ModelTiers: map[string]string{
			"model-a": "large",
			"model-b": "small",
		},
		Instructions: []*mcp.Instruction{
			{ID: "global-1", Name: "Global 1", Template: "G1", Enabled: true},
			{ID: "shared", Name: "Shared Global", Template: "SG", Enabled: true},
		},
	}

	project := &mcp.InstructionsConfig{
		DefaultTier: "small",
		ModelTiers: map[string]string{
			"model-b": "large", // Override
			"model-c": "small", // New
		},
		Instructions: []*mcp.Instruction{
			{ID: "project-1", Name: "Project 1", Template: "P1", Enabled: true},
			{ID: "shared", Name: "Shared Project", Template: "SP", Enabled: true}, // Override
		},
	}

	repoLocal := &mcp.InstructionsConfig{
		ModelTiers: map[string]string{
			"model-d": "large", // New
		},
		Instructions: []*mcp.Instruction{
			{ID: "local-1", Name: "Local 1", Template: "L1", Enabled: true},
			{ID: "shared", Name: "Shared Local", Template: "SL", Enabled: true}, // Override
		},
	}

	result := MergeInstructions(global, project, repoLocal)

	// DefaultTier: project overrides global, repo-local has empty so project's "small" wins
	assert.Equal(t, "small", result.DefaultTier)

	// ModelTiers: all should be merged with proper priority
	assert.Equal(t, "large", result.ModelTiers["model-a"]) // From global
	assert.Equal(t, "large", result.ModelTiers["model-b"]) // Project overrides global's "small"
	assert.Equal(t, "small", result.ModelTiers["model-c"]) // From project
	assert.Equal(t, "large", result.ModelTiers["model-d"]) // From repo-local

	// Instructions: should have 4 (global-1, project-1, local-1, shared)
	assert.Len(t, result.Instructions, 4)

	// Find the shared instruction - should be from repo-local
	var sharedInst *mcp.Instruction
	for _, inst := range result.Instructions {
		if inst.ID == "shared" {
			sharedInst = inst
			break
		}
	}
	require.NotNil(t, sharedInst)
	assert.Equal(t, "Shared Local", sharedInst.Name)
	assert.Equal(t, "SL", sharedInst.Template)
}

func TestMergeInstructions_NilInputs(t *testing.T) {
	result := MergeInstructions(nil, nil, nil)
	assert.NotNil(t, result)
	assert.Empty(t, result.Instructions)
	assert.NotNil(t, result.ModelTiers)
}

func TestMergeInstructions_GlobalOnly(t *testing.T) {
	global := &mcp.InstructionsConfig{
		DefaultTier: "large",
		Instructions: []*mcp.Instruction{
			{ID: "g1", Name: "G1"},
		},
	}

	result := MergeInstructions(global, nil, nil)
	assert.Equal(t, "large", result.DefaultTier)
	assert.Len(t, result.Instructions, 1)
}

func TestGetRepoRoot(t *testing.T) {
	tests := []struct {
		gitDir string
		want   string
	}{
		{"/home/user/repo/.git", "/home/user/repo"},
		{"/Users/dev/project/.git", "/Users/dev/project"},
		{"", ""},
	}

	for _, tt := range tests {
		got := GetRepoRoot(tt.gitDir)
		assert.Equal(t, tt.want, got)
	}
}

func TestRepoInstructionsConfigToMCP(t *testing.T) {
	repo := &RepoInstructionsConfig{
		DefaultTier: "small",
		ModelTiers:  map[string]string{"a": "large"},
		Instructions: []*mcp.Instruction{
			{ID: "test"},
		},
	}

	result := RepoInstructionsConfigToMCP(repo)
	assert.Equal(t, "small", result.DefaultTier)
	assert.Equal(t, "large", result.ModelTiers["a"])
	assert.Len(t, result.Instructions, 1)
}

func TestRepoInstructionsConfigToMCP_Nil(t *testing.T) {
	result := RepoInstructionsConfigToMCP(nil)
	assert.Nil(t, result)
}
