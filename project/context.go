package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ContextEntry describes a single file in the assembled context bundle.
type ContextEntry struct {
	Path     string `json:"path"`
	Source   string `json:"source"`
	MIMEType string `json:"mimeType"`
	Size     int    `json:"sizeBytes"`
}

// AssembleManifest builds the context manifest for a project definition.
// Layer 1: repoIncludes (committed files from the repository)
// Layer 2: files (non-committed files from the context store)
// Deduplication: higher layer wins.
func AssembleManifest(def *Definition, configDir string) []ContextEntry {
	if def.Context == nil {
		return nil
	}
	return assembleManifestFromConfig(def.Context, def.ResolvedRepo(), configDir, def.Name)
}

// AssembleManifestWithRole applies role context overrides before assembling.
func AssembleManifestWithRole(def *Definition, configDir string, role string) []ContextEntry {
	ctx := effectiveContext(def, role)
	if ctx == nil {
		return nil
	}
	return assembleManifestFromConfig(ctx, def.ResolvedRepo(), configDir, def.Name)
}

func effectiveContext(def *Definition, role string) *ContextConfig {
	if def.Context == nil && role == "" {
		return nil
	}
	base := def.Context
	if base == nil {
		base = &ContextConfig{}
	}
	if role == "" || def.Agents == nil || def.Agents.Roles == nil {
		return base
	}
	roleDef, ok := def.Agents.Roles[role]
	if !ok || roleDef.ContextOverrides == nil {
		return base
	}

	result := &ContextConfig{}
	ov := roleDef.ContextOverrides

	if len(ov.Files) > 0 {
		result.Files = ov.Files
	} else {
		result.Files = base.Files
	}
	if len(ov.RepoIncludes) > 0 {
		result.RepoIncludes = ov.RepoIncludes
	} else {
		result.RepoIncludes = base.RepoIncludes
	}
	if ov.MaxBytes > 0 {
		result.MaxBytes = ov.MaxBytes
	} else {
		result.MaxBytes = base.MaxBytes
	}
	return result
}

func assembleManifestFromConfig(ctx *ContextConfig, repoRoot, configDir, projectName string) []ContextEntry {
	var entries []ContextEntry

	for _, inc := range ctx.RepoIncludes {
		if repoRoot == "" {
			continue
		}
		full := filepath.Join(repoRoot, inc)
		matches, _ := filepath.Glob(full)
		if len(matches) == 0 {
			matches = []string{full}
		}
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil {
				continue
			}
			rel, _ := filepath.Rel(repoRoot, m)
			entries = append(entries, ContextEntry{
				Path:     rel,
				Source:   "repo",
				MIMEType: GuessMIME(rel),
				Size:     int(info.Size()),
			})
		}
	}

	contextDir := filepath.Join(configDir, "context", projectName)
	for _, f := range ctx.Files {
		full := filepath.Join(contextDir, f)
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		entries = append(entries, ContextEntry{
			Path:     f,
			Source:   "store",
			MIMEType: GuessMIME(f),
			Size:     int(info.Size()),
		})
	}

	return deduplicate(entries)
}

// ReadContextFile reads a context file by path, searching the context store first,
// then the repo. Returns content and an error if not found.
func ReadContextFile(def *Definition, configDir, path string) (string, error) {
	contextDir := filepath.Join(configDir, "context", def.Name)

	candidates := []string{
		filepath.Join(contextDir, path),
	}
	if repoRoot := def.ResolvedRepo(); repoRoot != "" {
		candidates = append(candidates, filepath.Join(repoRoot, path))
	}

	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return string(data), nil
		}
	}
	return "", fmt.Errorf("context file not found: %s", path)
}

// AssembleBundle assembles the full context bundle as a concatenated string.
// Respects maxBytes if set.
func AssembleBundle(def *Definition, configDir string, role string) (string, []ContextEntry) {
	entries := AssembleManifestWithRole(def, configDir, role)
	if len(entries) == 0 {
		return "", nil
	}

	ctx := effectiveContext(def, role)
	maxBytes := 0
	if ctx != nil {
		maxBytes = ctx.MaxBytes
	}

	var sb strings.Builder
	var included []ContextEntry
	for _, entry := range entries {
		content, err := ReadContextFile(def, configDir, entry.Path)
		if err != nil {
			continue
		}
		section := fmt.Sprintf("--- %s (%s) ---\n%s\n\n", entry.Path, entry.Source, content)
		if maxBytes > 0 && sb.Len()+len(section) > maxBytes {
			fmt.Fprintf(&sb, "--- TRUNCATED: context exceeded %d bytes ---\n", maxBytes)
			break
		}
		sb.WriteString(section)
		included = append(included, entry)
	}
	return sb.String(), included
}

func deduplicate(entries []ContextEntry) []ContextEntry {
	seen := make(map[string]int)
	for i, e := range entries {
		seen[e.Path] = i
	}
	var result []ContextEntry
	added := make(map[string]bool)
	for _, e := range entries {
		idx := seen[e.Path]
		if !added[e.Path] {
			result = append(result, entries[idx])
			added[e.Path] = true
		}
	}
	return result
}

// GuessMIME returns a MIME type based on file extension.
func GuessMIME(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "text/yaml"
	default:
		return "text/plain"
	}
}
