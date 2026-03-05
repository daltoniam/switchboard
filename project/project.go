package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var nameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// Definition represents a project-interop project definition (.project.json).
type Definition struct {
	Schema     string                `json:"$schema,omitempty"`
	Version    string                `json:"version"`
	Name       string                `json:"name"`
	Repo       string                `json:"repo,omitempty"`
	Branch     string                `json:"branch,omitempty"`
	Launch     *LaunchConfig         `json:"launch,omitempty"`
	Tools      map[string]*ScopeRule `json:"tools,omitempty"`
	Context    *ContextConfig        `json:"context,omitempty"`
	Agents     *AgentsConfig         `json:"agents,omitempty"`
	Extensions map[string]any        `json:"extensions,omitempty"`
}

// LaunchConfig controls how agents are bootstrapped.
type LaunchConfig struct {
	Prompt     string            `json:"prompt,omitempty"`
	PromptFile string            `json:"promptFile,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
}

// ScopeRule defines allow/deny/defaults for a single MCP server.
type ScopeRule struct {
	Allow    []string                  `json:"allow,omitempty"`
	Deny     []string                  `json:"deny,omitempty"`
	Defaults map[string]map[string]any `json:"defaults,omitempty"`
}

// ContextConfig specifies context files and assembly limits.
type ContextConfig struct {
	Files        []string `json:"files,omitempty"`
	RepoIncludes []string `json:"repoIncludes,omitempty"`
	MaxBytes     int      `json:"maxBytes,omitempty"`
}

// AgentsConfig holds multi-agent coordination settings.
type AgentsConfig struct {
	MaxConcurrent int                        `json:"maxConcurrent,omitempty"`
	Roles         map[string]*RoleDefinition `json:"roles,omitempty"`
}

// RoleDefinition scopes tool access and context for an agent role.
type RoleDefinition struct {
	Description      string                `json:"description,omitempty"`
	ToolOverrides    map[string]*ScopeRule `json:"toolOverrides,omitempty"`
	ContextOverrides *ContextConfig        `json:"contextOverrides,omitempty"`
}

// Validate checks that the definition has required fields and valid values.
func (d *Definition) Validate() error {
	if d.Version != "1" {
		return fmt.Errorf("unsupported version %q (must be \"1\")", d.Version)
	}
	if d.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(d.Name) > 128 {
		return fmt.Errorf("name exceeds 128 characters")
	}
	if !nameRE.MatchString(d.Name) {
		return fmt.Errorf("name %q does not match pattern ^[a-zA-Z0-9][a-zA-Z0-9._-]*$", d.Name)
	}
	return nil
}

// ExpandHome expands a leading ~/ to $HOME.
func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// ResolvedRepo returns the absolute path to the project repository.
func (d *Definition) ResolvedRepo() string {
	if d.Repo == "" {
		return ""
	}
	return ExpandHome(d.Repo)
}

// Merge merges a higher-precedence definition onto a base, returning a new Definition.
// The base is the user-level definition; overlay is the repo-local override.
func Merge(base, overlay *Definition) (*Definition, error) {
	if base.Name != "" && overlay.Name != "" && base.Name != overlay.Name {
		return nil, fmt.Errorf("cannot merge projects with different names: %q vs %q", base.Name, overlay.Name)
	}

	result := &Definition{}

	data, _ := json.Marshal(base)
	_ = json.Unmarshal(data, result)

	if overlay.Schema != "" {
		result.Schema = overlay.Schema
	}
	if overlay.Repo != "" {
		result.Repo = overlay.Repo
	}
	if overlay.Branch != "" {
		result.Branch = overlay.Branch
	}

	result.Launch = mergeLaunch(base.Launch, overlay.Launch)
	result.Tools = mergeTools(base.Tools, overlay.Tools)
	result.Context = mergeContext(base.Context, overlay.Context)
	result.Agents = mergeAgents(base.Agents, overlay.Agents)
	result.Extensions = mergeExtensions(base.Extensions, overlay.Extensions)

	return result, nil
}

func mergeLaunch(base, overlay *LaunchConfig) *LaunchConfig {
	if overlay == nil {
		return base
	}
	if base == nil {
		return overlay
	}
	result := &LaunchConfig{}
	if overlay.Prompt != "" {
		result.Prompt = overlay.Prompt
	} else {
		result.Prompt = base.Prompt
	}
	if overlay.PromptFile != "" {
		result.PromptFile = overlay.PromptFile
	} else {
		result.PromptFile = base.PromptFile
	}
	result.Env = make(map[string]string)
	for k, v := range base.Env {
		result.Env[k] = v
	}
	for k, v := range overlay.Env {
		result.Env[k] = v
	}
	return result
}

func mergeTools(base, overlay map[string]*ScopeRule) map[string]*ScopeRule {
	if overlay == nil {
		return base
	}
	if base == nil {
		return overlay
	}
	result := make(map[string]*ScopeRule)
	for k, v := range base {
		result[k] = copyScopeRule(v)
	}
	for k, v := range overlay {
		existing, ok := result[k]
		if !ok {
			result[k] = copyScopeRule(v)
			continue
		}
		existing.Allow = append(existing.Allow, v.Allow...)
		existing.Deny = append(existing.Deny, v.Deny...)
		if v.Defaults != nil {
			if existing.Defaults == nil {
				existing.Defaults = make(map[string]map[string]any)
			}
			for pattern, args := range v.Defaults {
				existing.Defaults[pattern] = args
			}
		}
	}
	return result
}

func copyScopeRule(r *ScopeRule) *ScopeRule {
	c := &ScopeRule{}
	c.Allow = append(c.Allow, r.Allow...)
	c.Deny = append(c.Deny, r.Deny...)
	if r.Defaults != nil {
		c.Defaults = make(map[string]map[string]any)
		for k, v := range r.Defaults {
			c.Defaults[k] = v
		}
	}
	return c
}

func mergeContext(base, overlay *ContextConfig) *ContextConfig {
	if overlay == nil {
		return base
	}
	if base == nil {
		return overlay
	}
	result := &ContextConfig{}
	result.Files = append(result.Files, base.Files...)
	result.Files = append(result.Files, overlay.Files...)
	result.RepoIncludes = append(result.RepoIncludes, base.RepoIncludes...)
	result.RepoIncludes = append(result.RepoIncludes, overlay.RepoIncludes...)
	if overlay.MaxBytes > 0 {
		result.MaxBytes = overlay.MaxBytes
	} else {
		result.MaxBytes = base.MaxBytes
	}
	return result
}

func mergeAgents(base, overlay *AgentsConfig) *AgentsConfig {
	if overlay == nil {
		return base
	}
	if base == nil {
		return overlay
	}
	result := &AgentsConfig{
		MaxConcurrent: base.MaxConcurrent,
	}
	if overlay.MaxConcurrent > 0 {
		result.MaxConcurrent = overlay.MaxConcurrent
	}
	result.Roles = make(map[string]*RoleDefinition)
	for k, v := range base.Roles {
		result.Roles[k] = v
	}
	for k, v := range overlay.Roles {
		result.Roles[k] = v
	}
	return result
}

func mergeExtensions(base, overlay map[string]any) map[string]any {
	if overlay == nil {
		return base
	}
	if base == nil {
		return overlay
	}
	result := make(map[string]any)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overlay {
		result[k] = v
	}
	return result
}

// Store manages project definitions in the user-level store.
type Store struct {
	configDir string
	mu        sync.RWMutex
	projects  map[string]*Definition
}

// NewStore creates a store rooted at the project-interop config directory.
func NewStore(configDir string) *Store {
	return &Store{
		configDir: configDir,
		projects:  make(map[string]*Definition),
	}
}

// DefaultConfigDir returns the default project-interop config directory.
func DefaultConfigDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "project-interop")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "project-interop")
}

// Load discovers and loads all project definitions from the user-level store.
// For each project with a repo path, it also attempts to merge a repo-local .project.json.
func (s *Store) Load() error {
	dir := filepath.Join(s.configDir, "projects")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading projects dir: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".project.json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var def Definition
		if err := json.Unmarshal(data, &def); err != nil {
			continue
		}
		if err := def.Validate(); err != nil {
			continue
		}

		merged, err := s.mergeRepoLocal(&def)
		if err == nil && merged != nil {
			s.projects[merged.Name] = merged
		} else {
			s.projects[def.Name] = &def
		}
	}
	return nil
}

func (s *Store) mergeRepoLocal(base *Definition) (*Definition, error) {
	repoRoot := base.ResolvedRepo()
	if repoRoot == "" {
		return nil, nil
	}
	repoLocalPath := filepath.Join(repoRoot, ".project.json")
	data, err := os.ReadFile(repoLocalPath)
	if err != nil {
		return nil, nil
	}
	var overlay Definition
	if err := json.Unmarshal(data, &overlay); err != nil {
		return nil, err
	}
	return Merge(base, &overlay)
}

// Get returns a project definition by name.
func (s *Store) Get(name string) (*Definition, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.projects[name]
	return d, ok
}

// All returns all loaded project definitions.
func (s *Store) All() map[string]*Definition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]*Definition, len(s.projects))
	for k, v := range s.projects {
		result[k] = v
	}
	return result
}

// Names returns all project names.
func (s *Store) Names() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.projects))
	for k := range s.projects {
		names = append(names, k)
	}
	return names
}

// Create writes a new project definition to the user-level store.
func (s *Store) Create(def *Definition) error {
	if err := def.Validate(); err != nil {
		return fmt.Errorf("invalid project definition: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.projects[def.Name]; exists {
		return fmt.Errorf("project %q already exists", def.Name)
	}

	if err := s.writeToDisk(def); err != nil {
		return err
	}
	s.projects[def.Name] = def
	return nil
}

// Update applies a JSON merge patch to the user-level store file.
func (s *Store) Update(name string, patch json.RawMessage) (*Definition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.projects[name]
	if !ok {
		return nil, fmt.Errorf("project %q not found", name)
	}

	base, err := json.Marshal(existing)
	if err != nil {
		return nil, err
	}

	var baseMap map[string]any
	_ = json.Unmarshal(base, &baseMap)

	var patchMap map[string]any
	if err := json.Unmarshal(patch, &patchMap); err != nil {
		return nil, fmt.Errorf("invalid patch: %w", err)
	}

	merged := jsonMergePatch(baseMap, patchMap)
	result, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}

	var def Definition
	if err := json.Unmarshal(result, &def); err != nil {
		return nil, err
	}
	if err := def.Validate(); err != nil {
		return nil, fmt.Errorf("invalid project definition after patch: %w", err)
	}

	if err := s.writeToDisk(&def); err != nil {
		return nil, err
	}
	s.projects[name] = &def
	return &def, nil
}

// Delete removes a project definition from the user-level store.
func (s *Store) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[name]; !ok {
		return fmt.Errorf("project %q not found", name)
	}

	path := filepath.Join(s.configDir, "projects", name+".project.json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	delete(s.projects, name)
	return nil
}

func (s *Store) writeToDisk(def *Definition) error {
	dir := filepath.Join(s.configDir, "projects")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(def, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, def.Name+".project.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// jsonMergePatch implements RFC 7396 JSON Merge Patch.
func jsonMergePatch(base, patch map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range patch {
		if v == nil {
			delete(result, k)
			continue
		}
		if patchObj, ok := v.(map[string]any); ok {
			if baseObj, ok := result[k].(map[string]any); ok {
				result[k] = jsonMergePatch(baseObj, patchObj)
				continue
			}
		}
		result[k] = v
	}
	return result
}

// ConfigDir returns the store's config directory root.
func (s *Store) ConfigDir() string {
	return s.configDir
}
