package compass

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// RepoInstructionsFile is the filename for repo-local instructions stored in .git/switchboard/
const RepoInstructionsFile = "instructions.json"

// ProjectInstructionsFile is the filename for project instructions in repo root
const ProjectInstructionsFile = ".switchboard/instructions.json"

// RepoInstructionsConfig holds instructions stored in a git repository.
// This is stored in .git/switchboard/instructions.json for worktree-shared,
// uncommitted instructions.
type RepoInstructionsConfig struct {
	DefaultTier  string            `json:"default_tier,omitempty"`
	ModelTiers   map[string]string `json:"model_tiers,omitempty"`
	Instructions []*mcp.Instruction `json:"instructions,omitempty"`
}

// FindGitDir finds the .git directory for the given path.
// It walks up the directory tree until it finds a .git directory or file (worktree).
// Returns empty string if no git repository is found.
func FindGitDir(startPath string) string {
	if startPath == "" {
		var err error
		startPath, err = os.Getwd()
		if err != nil {
			return ""
		}
	}

	path, err := filepath.Abs(startPath)
	if err != nil {
		return ""
	}

	for {
		gitPath := filepath.Join(path, ".git")
		info, err := os.Stat(gitPath)
		if err == nil {
			if info.IsDir() {
				// Regular git repo
				return gitPath
			}
			// Worktree: .git is a file containing "gitdir: /path/to/actual/.git/worktrees/name"
			content, err := os.ReadFile(gitPath)
			if err == nil {
				return resolveWorktreeGitDir(string(content))
			}
		}

		parent := filepath.Dir(path)
		if parent == path {
			// Reached root
			return ""
		}
		path = parent
	}
}

// resolveWorktreeGitDir parses a worktree .git file and returns the common git dir.
// Worktree .git files contain: "gitdir: /path/to/.git/worktrees/worktree-name"
// We want the parent .git directory, not the worktree-specific one.
func resolveWorktreeGitDir(content string) string {
	// Format: "gitdir: /path/to/.git/worktrees/name\n"
	if len(content) < 8 || content[:8] != "gitdir: " {
		return ""
	}
	gitDir := strings.TrimSpace(content[8:])
	
	// Check if this is a worktree path (.git/worktrees/name)
	// If so, return the parent .git directory
	dir := gitDir
	for {
		base := filepath.Base(dir)
		parent := filepath.Dir(dir)
		if base == "worktrees" {
			// parent should be .git
			return parent
		}
		if parent == dir {
			// Reached root without finding worktrees
			// This might be a direct reference, return as-is
			return gitDir
		}
		dir = parent
	}
}

// LoadRepoInstructions loads instructions from .git/switchboard/instructions.json
// Returns nil if the file doesn't exist or can't be read.
func LoadRepoInstructions(gitDir string) *RepoInstructionsConfig {
	if gitDir == "" {
		return nil
	}
	
	path := filepath.Join(gitDir, "switchboard", RepoInstructionsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	
	var config RepoInstructionsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil
	}
	
	return &config
}

// LoadProjectInstructions loads instructions from .switchboard/instructions.json in repo root.
// Returns nil if the file doesn't exist or can't be read.
func LoadProjectInstructions(repoRoot string) *RepoInstructionsConfig {
	if repoRoot == "" {
		return nil
	}
	
	path := filepath.Join(repoRoot, ProjectInstructionsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	
	var config RepoInstructionsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil
	}
	
	return &config
}

// SaveRepoInstructions saves instructions to .git/switchboard/instructions.json
func SaveRepoInstructions(gitDir string, config *RepoInstructionsConfig) error {
	if gitDir == "" {
		return os.ErrNotExist
	}
	
	dir := filepath.Join(gitDir, "switchboard")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	
	path := filepath.Join(dir, RepoInstructionsFile)
	return os.WriteFile(path, data, 0600)
}

// GetRepoRoot returns the repository root directory from a git dir path.
// For regular repos, this is the parent of .git.
// For worktrees, this finds the worktree root.
func GetRepoRoot(gitDir string) string {
	if gitDir == "" {
		return ""
	}
	// For regular repos, .git is at repo root
	// Parent of .git is the repo root
	return filepath.Dir(gitDir)
}

// MergeInstructions merges instructions from multiple sources.
// Priority (highest to lowest):
// 1. Repo-local (.git/switchboard/instructions.json) - worktree-shared, uncommitted
// 2. Project (.switchboard/instructions.json) - committed, version-controlled
// 3. Global (~/.config/switchboard/config.json) - user-wide defaults
//
// Instructions with the same ID from higher priority sources replace lower priority ones.
// ModelTiers and DefaultTier also merge with the same priority.
func MergeInstructions(global, project, repoLocal *mcp.InstructionsConfig) *mcp.InstructionsConfig {
	result := &mcp.InstructionsConfig{
		Instructions: make([]*mcp.Instruction, 0),
		ModelTiers:   make(map[string]string),
	}
	
	// Start with global
	if global != nil {
		result.DefaultTier = global.DefaultTier
		for k, v := range global.ModelTiers {
			result.ModelTiers[k] = v
		}
		result.Instructions = append(result.Instructions, global.Instructions...)
	}
	
	// Override with project
	if project != nil {
		if project.DefaultTier != "" {
			result.DefaultTier = project.DefaultTier
		}
		for k, v := range project.ModelTiers {
			result.ModelTiers[k] = v
		}
		result.Instructions = mergeInstructionLists(result.Instructions, project.Instructions)
	}
	
	// Override with repo-local
	if repoLocal != nil {
		if repoLocal.DefaultTier != "" {
			result.DefaultTier = repoLocal.DefaultTier
		}
		for k, v := range repoLocal.ModelTiers {
			result.ModelTiers[k] = v
		}
		result.Instructions = mergeInstructionLists(result.Instructions, repoLocal.Instructions)
	}
	
	return result
}

// mergeInstructionLists merges two instruction lists, with newer replacing older by ID.
func mergeInstructionLists(base, overlay []*mcp.Instruction) []*mcp.Instruction {
	if len(overlay) == 0 {
		return base
	}
	
	// Build a map of existing instructions by ID
	byID := make(map[string]int)
	for i, inst := range base {
		byID[inst.ID] = i
	}
	
	// Merge overlay
	result := make([]*mcp.Instruction, len(base))
	copy(result, base)
	
	for _, inst := range overlay {
		if idx, exists := byID[inst.ID]; exists {
			// Replace existing
			result[idx] = inst
		} else {
			// Append new
			result = append(result, inst)
			byID[inst.ID] = len(result) - 1
		}
	}
	
	return result
}

// RepoInstructionsConfigToMCP converts RepoInstructionsConfig to mcp.InstructionsConfig
func RepoInstructionsConfigToMCP(r *RepoInstructionsConfig) *mcp.InstructionsConfig {
	if r == nil {
		return nil
	}
	return &mcp.InstructionsConfig{
		DefaultTier:  r.DefaultTier,
		ModelTiers:   r.ModelTiers,
		Instructions: r.Instructions,
	}
}

// LoadAllInstructions loads and merges instructions from all sources for a given working directory.
// Returns the merged instruction config with proper priority handling.
func LoadAllInstructions(workDir string, globalConfig *mcp.InstructionsConfig) *mcp.InstructionsConfig {
	gitDir := FindGitDir(workDir)
	
	var projectConfig, repoLocalConfig *mcp.InstructionsConfig
	
	if gitDir != "" {
		repoRoot := GetRepoRoot(gitDir)
		
		// Load project instructions (.switchboard/instructions.json)
		if proj := LoadProjectInstructions(repoRoot); proj != nil {
			projectConfig = RepoInstructionsConfigToMCP(proj)
		}
		
		// Load repo-local instructions (.git/switchboard/instructions.json)
		if local := LoadRepoInstructions(gitDir); local != nil {
			repoLocalConfig = RepoInstructionsConfigToMCP(local)
		}
	}
	
	return MergeInstructions(globalConfig, projectConfig, repoLocalConfig)
}
