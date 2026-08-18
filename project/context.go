package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ContextEntry describes a single file in the assembled context bundle.
type ContextEntry struct {
	Path     string `json:"path"`
	Source   string `json:"source"`
	MIMEType string `json:"mimeType"`
	Size     int    `json:"sizeBytes"`
}

// ResourceFileEntry is one matched file inside a files resource manifest.
type ResourceFileEntry struct {
	Path     string `json:"path"`
	URI      string `json:"uri,omitempty"`
	MIMEType string `json:"mimeType"`
	Size     int    `json:"sizeBytes"`
}

// FilesManifest is the content payload for a files resource.
type FilesManifest struct {
	Resource string              `json:"resource"`
	Files    []ResourceFileEntry `json:"files"`
}

// AssembleManifest builds the context manifest from project resources.
// File and files resources contribute entries; missing optional resources are skipped.
func AssembleManifest(def *Definition, configDir string) []ContextEntry {
	_ = configDir
	return AssembleManifestAtRoot(def, configDir, def.ResolvedRepo())
}

// AssembleManifestAtRoot assembles context against an explicit worktree root
// used when resolving a single-repo overlay/worktree.
func AssembleManifestAtRoot(def *Definition, configDir, root string) []ContextEntry {
	_ = configDir
	if def == nil {
		return nil
	}
	entries := assembleFromResources(def, root)
	return deduplicate(entries)
}

// AssembleManifestWithRole applies role context overrides before assembling.
// When a role supplies ContextOverrides, those files/includes are used in
// addition to (or instead of empty) resource assembly.
func AssembleManifestWithRole(def *Definition, configDir string, role string) []ContextEntry {
	if def == nil {
		return nil
	}
	entries := assembleFromResources(def, "")
	if ctx := roleContextOverride(def, role); ctx != nil {
		entries = append(entries, assembleManifestFromConfig(ctx, def.ResolvedRepo(), configDir, def.Name)...)
	}
	return deduplicate(entries)
}

func roleContextOverride(def *Definition, role string) *ContextConfig {
	if role == "" || def.Agents == nil || def.Agents.Roles == nil {
		return nil
	}
	roleDef, ok := def.Agents.Roles[role]
	if !ok {
		return nil
	}
	return roleDef.ContextOverrides
}

func assembleFromResources(def *Definition, overrideRoot string) []ContextEntry {
	if def.Resources == nil {
		return nil
	}
	ids := make([]string, 0, len(def.Resources))
	for id := range def.Resources {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var entries []ContextEntry
	for _, id := range ids {
		res := def.Resources[id]
		switch res.Type {
		case ResourceTypeFile:
			path, source, ok := resolveFileResource(def, res, overrideRoot)
			if !ok {
				continue
			}
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			rel := res.Path
			if res.Repo == "" {
				rel = path
			}
			entries = append(entries, ContextEntry{
				Path:     rel,
				Source:   source,
				MIMEType: GuessMIME(rel),
				Size:     int(info.Size()),
			})
		case ResourceTypeFiles:
			files, err := ExpandFilesResource(def, id, res, overrideRoot)
			if err != nil {
				continue
			}
			for _, f := range files {
				entries = append(entries, ContextEntry{
					Path:     f.Path,
					Source:   id,
					MIMEType: f.MIMEType,
					Size:     f.Size,
				})
			}
		}
	}
	return entries
}

func resolveFileResource(def *Definition, res Resource, overrideRoot string) (absPath, source string, ok bool) {
	//nolint:nestif // primary-repo override vs named repo resolution
	if res.Repo != "" {
		root := overrideRoot
		if root == "" {
			repoRes, exists := def.Resources[res.Repo]
			if !exists || repoRes.Type != ResourceTypeRepo {
				return "", "", false
			}
			// Only use override root when it targets this project's primary repo.
			root = def.ResolveRepoPath(repoRes)
			if overrideRoot != "" {
				if id, _, single := def.PrimaryRepoResource(); single && id == res.Repo {
					root = overrideRoot
				}
			}
		} else if id, _, single := def.PrimaryRepoResource(); single && id == res.Repo {
			root = overrideRoot
		} else {
			repoRes := def.Resources[res.Repo]
			root = def.ResolveRepoPath(repoRes)
		}
		candidate, within := pathWithinRoot(root, res.Path)
		if !within {
			return "", "", false
		}
		return candidate, res.Repo, true
	}
	abs := ResolvePath(def.baseDir, res.Path)
	return abs, "local", true
}

// ExpandFilesResource expands include/exclude globs for a files resource.
func ExpandFilesResource(def *Definition, id string, res Resource, overrideRoot string) ([]ResourceFileEntry, error) {
	root := ""
	if res.Repo != "" {
		repoRes, ok := def.Resources[res.Repo]
		if !ok || repoRes.Type != ResourceTypeRepo {
			return nil, fmt.Errorf("repo %q not found", res.Repo)
		}
		root = def.ResolveRepoPath(repoRes)
		if overrideRoot != "" {
			if pid, _, single := def.PrimaryRepoResource(); single && pid == res.Repo {
				root = overrideRoot
			}
		}
	} else if res.Root != "" {
		root = ResolvePath(def.baseDir, res.Root)
	} else {
		return nil, fmt.Errorf("files resource %q needs repo or root", id)
	}
	if root == "" {
		return nil, fmt.Errorf("files resource %q has empty root", id)
	}

	seen := make(map[string]ResourceFileEntry)
	for _, pattern := range res.Include {
		matches, err := expandGlobUnderRoot(root, pattern)
		if err != nil {
			continue
		}
		for _, m := range matches {
			rel, err := filepath.Rel(root, m)
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			if excluded(rel, res.Exclude) {
				continue
			}
			info, err := os.Stat(m)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			seen[rel] = ResourceFileEntry{
				Path:     rel,
				MIMEType: GuessMIME(rel),
				Size:     int(info.Size()),
			}
		}
	}
	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	out := make([]ResourceFileEntry, 0, len(paths))
	for _, p := range paths {
		out = append(out, seen[p])
	}
	return out, nil
}

func expandGlobUnderRoot(root, pattern string) ([]string, error) {
	pattern = filepath.ToSlash(pattern)
	if pattern == "" {
		return nil, nil
	}
	// Reject patterns that escape root via .. segments before expansion.
	cleanPat := pathCleanSlash(pattern)
	if cleanPat == ".." || strings.HasPrefix(cleanPat, "../") {
		return nil, fmt.Errorf("pattern escapes root")
	}

	// filepath.Glob does not support **; walk when needed.
	if strings.Contains(pattern, "**") {
		var kept []string
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}
			rel = filepath.ToSlash(rel)
			if matchGlob(pattern, rel) {
				if resolved, ok := pathWithinRootAbs(root, path); ok {
					kept = append(kept, resolved)
				}
			}
			return nil
		})
		return kept, nil
	}

	full := filepath.Join(root, filepath.FromSlash(pattern))
	matches, err := filepath.Glob(full)
	if err != nil {
		return nil, err
	}
	var kept []string
	for _, m := range matches {
		resolved, ok := pathWithinRootAbs(root, m)
		if !ok {
			continue
		}
		kept = append(kept, resolved)
	}
	return kept, nil
}

func pathCleanSlash(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	parts := strings.Split(p, "/")
	var stack []string
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			} else {
				stack = append(stack, "..")
			}
		default:
			stack = append(stack, part)
		}
	}
	return strings.Join(stack, "/")
}

func excluded(rel string, patterns []string) bool {
	rel = filepath.ToSlash(rel)
	for _, p := range patterns {
		p = filepath.ToSlash(p)
		if matchGlob(p, rel) {
			return true
		}
	}
	return false
}

// matchGlob supports ** and standard path.Match semantics on slash paths.
func matchGlob(pattern, name string) bool {
	pattern = filepath.ToSlash(pattern)
	name = filepath.ToSlash(name)
	if strings.Contains(pattern, "**") {
		return matchDoubleStar(pattern, name)
	}
	ok, _ := filepath.Match(pattern, name)
	if ok {
		return true
	}
	// Also try matching basename-only patterns.
	ok, _ = filepath.Match(pattern, filepath.Base(name))
	return ok
}

func matchDoubleStar(pattern, name string) bool {
	// Simplified ** matcher: split on /**/ or leading/trailing **.
	parts := strings.Split(pattern, "**")
	if len(parts) == 1 {
		ok, _ := filepath.Match(pattern, name)
		return ok
	}
	// Must match prefix, then each middle, then suffix.
	rest := name
	// prefix
	prefix := strings.TrimSuffix(parts[0], "/")
	if prefix != "" {
		if !strings.HasPrefix(rest, strings.TrimSuffix(prefix, "/")) && !matchPrefixGlob(prefix, rest) {
			// try path.Match on prefix segment
			if !hasPrefixMatch(prefix, rest) {
				return false
			}
		}
		// advance rest past prefix match length approximately
		rest = trimPrefixMatch(prefix, rest)
	}
	for i := 1; i < len(parts)-1; i++ {
		mid := strings.Trim(parts[i], "/")
		if mid == "" {
			continue
		}
		idx := indexGlob(rest, mid)
		if idx < 0 {
			return false
		}
		rest = rest[idx+len(mid):]
		rest = strings.TrimPrefix(rest, "/")
	}
	suffix := strings.TrimPrefix(parts[len(parts)-1], "/")
	if suffix == "" {
		return true
	}
	// suffix may contain globs
	suffix = strings.TrimPrefix(suffix, "/")
	return matchSuffixGlob(suffix, rest)
}

func hasPrefixMatch(prefix, name string) bool {
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		return true
	}
	// Match path elements
	nameParts := strings.Split(name, "/")
	prefParts := strings.Split(prefix, "/")
	if len(prefParts) > len(nameParts) {
		return false
	}
	for i, p := range prefParts {
		ok, _ := filepath.Match(p, nameParts[i])
		if !ok {
			return false
		}
	}
	return true
}

func trimPrefixMatch(prefix, name string) string {
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		return name
	}
	nameParts := strings.Split(name, "/")
	prefParts := strings.Split(prefix, "/")
	if len(prefParts) > len(nameParts) {
		return name
	}
	return strings.Join(nameParts[len(prefParts):], "/")
}

func indexGlob(name, mid string) int {
	// linear search for mid as substring with glob on segments is complex;
	// use plain contains for non-glob mid, else scan.
	if !strings.ContainsAny(mid, "*?[") {
		return strings.Index(name, mid)
	}
	nameParts := strings.Split(name, "/")
	midParts := strings.Split(mid, "/")
	for i := 0; i+len(midParts) <= len(nameParts); i++ {
		ok := true
		for j, mp := range midParts {
			m, _ := filepath.Match(mp, nameParts[i+j])
			if !m {
				ok = false
				break
			}
		}
		if ok {
			// return byte index
			return len(strings.Join(nameParts[:i], "/"))
		}
	}
	return -1
}

func matchSuffixGlob(suffix, rest string) bool {
	suffix = strings.TrimPrefix(suffix, "/")
	if suffix == "" {
		return true
	}
	if !strings.ContainsAny(suffix, "*?[") {
		return strings.HasSuffix(rest, suffix) || rest == suffix
	}
	// try matching end segments
	sufParts := strings.Split(suffix, "/")
	restParts := strings.Split(rest, "/")
	if len(sufParts) > len(restParts) {
		// ** already consumed prefix; allow match anywhere ending
		for i := 0; i+len(sufParts) <= len(restParts); i++ {
			if segmentsMatch(sufParts, restParts[i:i+len(sufParts)]) {
				return true
			}
		}
		return false
	}
	start := len(restParts) - len(sufParts)
	return segmentsMatch(sufParts, restParts[start:])
}

func segmentsMatch(pattern, name []string) bool {
	if len(pattern) != len(name) {
		return false
	}
	for i := range pattern {
		ok, _ := filepath.Match(pattern[i], name[i])
		if !ok {
			return false
		}
	}
	return true
}

func matchPrefixGlob(prefix, name string) bool {
	return hasPrefixMatch(prefix, name)
}

func assembleManifestFromConfig(ctx *ContextConfig, repoRoot, configDir, projectName string) []ContextEntry {
	if ctx == nil {
		return nil
	}
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

	return entries
}

// ReadContextFile reads a context file by path from resources or legacy roots.
func ReadContextFile(def *Definition, configDir, path string) (string, error) {
	if def == nil {
		return "", fmt.Errorf("context file not found: %s", path)
	}

	// Try matching a file resource by relative path or absolute.
	//nolint:nestif // walks file and files resources
	if def.Resources != nil {
		for _, res := range def.Resources {
			if res.Type != ResourceTypeFile {
				continue
			}
			abs, _, ok := resolveFileResource(def, res, "")
			if !ok {
				continue
			}
			if res.Path == path || abs == path || filepath.Base(abs) == path {
				data, err := os.ReadFile(abs)
				if err == nil {
					return string(data), nil
				}
			}
			// also allow reading by resource-relative path when under repo
			if res.Repo != "" && res.Path == path {
				data, err := os.ReadFile(abs)
				if err == nil {
					return string(data), nil
				}
			}
		}
		// files resource members
		for id, res := range def.Resources {
			if res.Type != ResourceTypeFiles {
				continue
			}
			files, err := ExpandFilesResource(def, id, res, "")
			if err != nil {
				continue
			}
			root, _ := def.ResolveResourceRoot(res)
			for _, f := range files {
				if f.Path == path || filepath.ToSlash(f.Path) == filepath.ToSlash(path) {
					candidate, ok := pathWithinRoot(root, f.Path)
					if !ok {
						continue
					}
					data, err := os.ReadFile(candidate)
					if err == nil {
						return string(data), nil
					}
				}
			}
		}
	}

	// Fallback: primary repo + context store (role/legacy paths).
	contextDir := filepath.Join(configDir, "context", def.Name)
	roots := []string{contextDir}
	if repoRoot := def.ResolvedRepo(); repoRoot != "" {
		roots = append(roots, repoRoot)
	}
	for _, root := range roots {
		candidate, ok := pathWithinRoot(root, path)
		if !ok {
			continue
		}
		data, err := os.ReadFile(candidate)
		if err == nil {
			return string(data), nil
		}
	}
	return "", fmt.Errorf("context file not found: %s", path)
}

// ReadResourceContent returns text content for a named file resource, or a
// files manifest for a files resource, or repo metadata JSON is handled by callers.
func ReadResourceContent(def *Definition, resourceID string) (content string, mime string, err error) {
	if def == nil || def.Resources == nil {
		return "", "", fmt.Errorf("resource %q not found", resourceID)
	}
	res, ok := def.Resources[resourceID]
	if !ok {
		return "", "", fmt.Errorf("resource %q not found", resourceID)
	}
	switch res.Type {
	case ResourceTypeFile:
		abs, _, ok := resolveFileResource(def, res, "")
		if !ok {
			if res.Optional {
				return "", "", fmt.Errorf("optional resource %q missing", resourceID)
			}
			return "", "", fmt.Errorf("resource %q path invalid", resourceID)
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			if res.Optional {
				return "", "", fmt.Errorf("optional resource %q missing", resourceID)
			}
			return "", "", err
		}
		return string(data), GuessMIME(res.Path), nil
	case ResourceTypeFiles:
		files, err := ExpandFilesResource(def, resourceID, res, "")
		if err != nil {
			return "", "", err
		}
		manifest := FilesManifest{Resource: resourceID, Files: files}
		if manifest.Files == nil {
			manifest.Files = []ResourceFileEntry{}
		}
		raw, err := jsonMarshal(manifest)
		if err != nil {
			return "", "", err
		}
		return string(raw), "application/json", nil
	case ResourceTypeRepo:
		meta := map[string]any{
			"id":   resourceID,
			"type": ResourceTypeRepo,
			"path": def.ResolveRepoPath(res),
		}
		if res.Branch != "" {
			meta["branch"] = res.Branch
		}
		if res.Description != "" {
			meta["description"] = res.Description
		}
		raw, err := jsonMarshal(meta)
		if err != nil {
			return "", "", err
		}
		return string(raw), "application/json", nil
	default:
		return "", "", fmt.Errorf("unsupported resource type %q", res.Type)
	}
}

func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func pathWithinRoot(root, relativePath string) (string, bool) {
	if filepath.IsAbs(relativePath) {
		return "", false
	}
	clean := filepath.Clean(relativePath)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", false
	}
	joined := filepath.Join(root, clean)
	return pathWithinRootAbs(root, joined)
}

func pathWithinRootAbs(root, candidate string) (string, bool) {
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		// Non-existent paths stay lexical; callers treat missing files as absent.
		absRoot, rootErr := filepath.Abs(root)
		if rootErr != nil {
			return "", false
		}
		absJoined, joinErr := filepath.Abs(candidate)
		if joinErr != nil {
			return "", false
		}
		rel, relErr := filepath.Rel(absRoot, absJoined)
		if relErr != nil || strings.HasPrefix(rel, "..") {
			return "", false
		}
		return candidate, true
	}
	absRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		absRoot = root
	}
	rel, err := filepath.Rel(absRoot, resolved)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}
	info, err := os.Lstat(resolved)
	if err != nil {
		return "", false
	}
	// Allow directories for glob intermediate checks; regular files for reads.
	if !info.Mode().IsRegular() && !info.IsDir() {
		return "", false
	}
	return resolved, true
}

// AssembleBundle assembles the full context bundle as a concatenated string.
// Respects maxBytes from role override if set.
func AssembleBundle(def *Definition, configDir string, role string) (string, []ContextEntry) {
	entries := AssembleManifestWithRole(def, configDir, role)
	if len(entries) == 0 {
		return "", nil
	}
	maxBytes := 0
	if ctx := roleContextOverride(def, role); ctx != nil {
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
