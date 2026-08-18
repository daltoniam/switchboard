package project

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var nameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// Resource type constants.
const (
	ResourceTypeRepo  = "repo"
	ResourceTypeFile  = "file"
	ResourceTypeFiles = "files"
)

// Definition is a Switchboard project catalog entry (resources model).
// Optional Launch/Tools/Agents remain for gateway scoping compatibility and
// are not part of the required catalog identity.
type Definition struct {
	Schema      string                     `json:"$schema,omitempty"`
	Version     string                     `json:"version"`
	Name        string                     `json:"name"`
	Description string                     `json:"description,omitempty"`
	Resources   map[string]Resource        `json:"resources"`
	Launch      *LaunchConfig              `json:"launch,omitempty"`
	Tools       map[string]*ScopeRule      `json:"tools,omitempty"`
	Agents      *AgentsConfig              `json:"agents,omitempty"`
	Extensions  map[string]any             `json:"extensions,omitempty"`
	Additional  map[string]json.RawMessage `json:"-"`

	// baseDir is the directory containing the project JSON file; used for
	// relative path resolution. Not serialized.
	baseDir string
}

// Resource is a typed catalog resource (repo | file | files).
type Resource struct {
	Type        string   `json:"type"`
	Path        string   `json:"path,omitempty"`
	Branch      string   `json:"branch,omitempty"`
	Repo        string   `json:"repo,omitempty"`
	Root        string   `json:"root,omitempty"`
	Include     []string `json:"include,omitempty"`
	Exclude     []string `json:"exclude,omitempty"`
	Description string   `json:"description,omitempty"`
	Optional    bool     `json:"optional,omitempty"`
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

// ContextConfig is retained for role overrides and legacy test helpers.
// Catalog assembly prefers Resources; ContextConfig is only used when
// role overrides supply explicit files/includes.
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
	if d.Resources == nil {
		// Treat missing resources as empty map for in-memory constructors;
		// JSON without the key still unmarshals as nil and is rejected on load
		// only when we want strictness — schema requires the key, so normalize.
		d.Resources = map[string]Resource{}
	}
	for id, res := range d.Resources {
		if err := validateID(id); err != nil {
			return fmt.Errorf("resource %q: %w", id, err)
		}
		if err := res.Validate(id, d.Resources); err != nil {
			return err
		}
	}
	return nil
}

func validateID(id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}
	if len(id) > 128 {
		return fmt.Errorf("id exceeds 128 characters")
	}
	if !nameRE.MatchString(id) {
		return fmt.Errorf("id %q does not match pattern ^[a-zA-Z0-9][a-zA-Z0-9._-]*$", id)
	}
	return nil
}

// Validate checks a single resource against the catalog schema and semantics.
func (r Resource) Validate(id string, all map[string]Resource) error {
	switch r.Type {
	case ResourceTypeRepo:
		if strings.TrimSpace(r.Path) == "" {
			return fmt.Errorf("resource %q: path is required for type repo", id)
		}
		if r.Repo != "" || r.Root != "" || len(r.Include) > 0 {
			return fmt.Errorf("resource %q: repo resources only allow type, path, branch, description", id)
		}
	case ResourceTypeFile:
		if strings.TrimSpace(r.Path) == "" {
			return fmt.Errorf("resource %q: path is required for type file", id)
		}
		if r.Root != "" || len(r.Include) > 0 || r.Branch != "" {
			return fmt.Errorf("resource %q: file resources only allow type, path, repo, description, optional", id)
		}
		if r.Repo != "" {
			if err := requireRepoRef(r.Repo, all, id); err != nil {
				return err
			}
		}
	case ResourceTypeFiles:
		if len(r.Include) == 0 {
			return fmt.Errorf("resource %q: include is required for type files", id)
		}
		for i, pat := range r.Include {
			if strings.TrimSpace(pat) == "" {
				return fmt.Errorf("resource %q: include[%d] must be non-empty", id, i)
			}
		}
		for i, pat := range r.Exclude {
			if strings.TrimSpace(pat) == "" {
				return fmt.Errorf("resource %q: exclude[%d] must be non-empty", id, i)
			}
		}
		if r.Repo != "" && r.Root != "" {
			return fmt.Errorf("resource %q: files resource cannot set both repo and root", id)
		}
		if r.Path != "" || r.Branch != "" {
			return fmt.Errorf("resource %q: files resources do not allow path or branch", id)
		}
		if r.Repo != "" {
			if err := requireRepoRef(r.Repo, all, id); err != nil {
				return err
			}
		}
	case "":
		return fmt.Errorf("resource %q: type is required", id)
	default:
		return fmt.Errorf("resource %q: unsupported type %q", id, r.Type)
	}
	return nil
}

func requireRepoRef(ref string, all map[string]Resource, fromID string) error {
	if err := validateID(ref); err != nil {
		return fmt.Errorf("resource %q: repo ref: %w", fromID, err)
	}
	target, ok := all[ref]
	if !ok {
		return fmt.Errorf("resource %q: repo %q does not reference a resource", fromID, ref)
	}
	if target.Type != ResourceTypeRepo {
		return fmt.Errorf("resource %q: repo %q must reference a resource with type repo", fromID, ref)
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

// SetBaseDir records the directory of the project JSON for relative path resolution.
func (d *Definition) SetBaseDir(dir string) {
	if d == nil {
		return
	}
	d.baseDir = dir
}

// BaseDir returns the project file directory used for relative paths.
func (d *Definition) BaseDir() string {
	if d == nil {
		return ""
	}
	return d.baseDir
}

// ResolvePath expands ~/ and resolves relative paths against base (project file dir).
func ResolvePath(base, path string) string {
	path = ExpandHome(path)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	if base == "" {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(base, path))
}

// RepoResourceIDs returns sorted ids of type=repo resources.
func (d *Definition) RepoResourceIDs() []string {
	if d == nil || d.Resources == nil {
		return nil
	}
	ids := make([]string, 0)
	for id, r := range d.Resources {
		if r.Type == ResourceTypeRepo {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// PrimaryRepoResource returns the sole repo resource when exactly one exists.
func (d *Definition) PrimaryRepoResource() (id string, res Resource, ok bool) {
	ids := d.RepoResourceIDs()
	if len(ids) != 1 {
		return "", Resource{}, false
	}
	return ids[0], d.Resources[ids[0]], true
}

// ResolvedRepo returns the absolute path of the primary repo resource, if any.
// Multi-repo projects return "".
func (d *Definition) ResolvedRepo() string {
	_, res, ok := d.PrimaryRepoResource()
	if !ok {
		return ""
	}
	return d.ResolveRepoPath(res)
}

// ResolveRepoPath resolves a repo resource path against the project base dir.
func (d *Definition) ResolveRepoPath(res Resource) string {
	base := ""
	if d != nil {
		base = d.baseDir
	}
	return ResolvePath(base, res.Path)
}

// ResolveResourceRoot returns the filesystem root for a file/files resource.
func (d *Definition) ResolveResourceRoot(res Resource) (string, error) {
	if d == nil {
		return "", fmt.Errorf("nil definition")
	}
	switch res.Type {
	case ResourceTypeRepo:
		return d.ResolveRepoPath(res), nil
	case ResourceTypeFile:
		if res.Repo != "" {
			repoRes, ok := d.Resources[res.Repo]
			if !ok || repoRes.Type != ResourceTypeRepo {
				return "", fmt.Errorf("repo %q not found", res.Repo)
			}
			return d.ResolveRepoPath(repoRes), nil
		}
		// Absolute/home path: parent is not a root; caller uses full path.
		return "", nil
	case ResourceTypeFiles:
		if res.Repo != "" {
			repoRes, ok := d.Resources[res.Repo]
			if !ok || repoRes.Type != ResourceTypeRepo {
				return "", fmt.Errorf("repo %q not found", res.Repo)
			}
			return d.ResolveRepoPath(repoRes), nil
		}
		if res.Root != "" {
			return ResolvePath(d.baseDir, res.Root), nil
		}
		return "", fmt.Errorf("files resource needs repo or root")
	default:
		return "", fmt.Errorf("unsupported resource type %q", res.Type)
	}
}

// PrimaryRepo returns the unresolved path of the sole repo resource, if any.
func (d *Definition) PrimaryRepo() string {
	_, res, ok := d.PrimaryRepoResource()
	if !ok {
		return ""
	}
	return res.Path
}

// PrimaryBranch returns the branch of the sole repo resource, if any.
func (d *Definition) PrimaryBranch() string {
	_, res, ok := d.PrimaryRepoResource()
	if !ok {
		return ""
	}
	return res.Branch
}

// Merge merges a higher-precedence definition onto a base, returning a new Definition.
// The base is the user-level definition; overlay is the repo-local override.
func Merge(base, overlay *Definition) (*Definition, error) {
	if base.Name != "" && overlay.Name != "" && base.Name != overlay.Name {
		return nil, fmt.Errorf("cannot merge projects with different names: %q vs %q", base.Name, overlay.Name)
	}

	result := cloneDefinition(base)
	if result == nil {
		result = &Definition{}
	}

	if overlay.Schema != "" {
		result.Schema = overlay.Schema
	}
	if overlay.Description != "" {
		result.Description = overlay.Description
	}
	if overlay.Version != "" {
		result.Version = overlay.Version
	}
	if overlay.baseDir != "" {
		result.baseDir = overlay.baseDir
	}

	result.Resources = mergeResources(base.Resources, overlay.Resources)
	result.Launch = mergeLaunch(base.Launch, overlay.Launch)
	result.Tools = mergeTools(base.Tools, overlay.Tools)
	result.Agents = mergeAgents(base.Agents, overlay.Agents)
	result.Extensions = mergeExtensions(base.Extensions, overlay.Extensions)
	result.Additional = mergeAdditional(base.Additional, overlay.Additional)

	return result, nil
}

func mergeResources(base, overlay map[string]Resource) map[string]Resource {
	if overlay == nil {
		return cloneResources(base)
	}
	if base == nil {
		return cloneResources(overlay)
	}
	result := cloneResources(base)
	for k, v := range overlay {
		result[k] = cloneResource(v)
	}
	return result
}

func mergeAdditional(base, overlay map[string]json.RawMessage) map[string]json.RawMessage {
	if overlay == nil {
		return cloneAdditional(base)
	}
	if base == nil {
		return cloneAdditional(overlay)
	}
	result := cloneAdditional(base)
	for k, v := range overlay {
		result[k] = append(json.RawMessage(nil), v...)
	}
	return result
}

func mergeLaunch(base, overlay *LaunchConfig) *LaunchConfig {
	if overlay == nil {
		return cloneLaunch(base)
	}
	if base == nil {
		return cloneLaunch(overlay)
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
		return cloneTools(base)
	}
	if base == nil {
		return cloneTools(overlay)
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
	if r == nil {
		return nil
	}
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

func mergeAgents(base, overlay *AgentsConfig) *AgentsConfig {
	if overlay == nil {
		return cloneAgents(base)
	}
	if base == nil {
		return cloneAgents(overlay)
	}
	result := &AgentsConfig{
		MaxConcurrent: base.MaxConcurrent,
	}
	if overlay.MaxConcurrent > 0 {
		result.MaxConcurrent = overlay.MaxConcurrent
	}
	result.Roles = make(map[string]*RoleDefinition)
	for k, v := range base.Roles {
		result.Roles[k] = cloneRole(v)
	}
	for k, v := range overlay.Roles {
		result.Roles[k] = cloneRole(v)
	}
	return result
}

func mergeExtensions(base, overlay map[string]any) map[string]any {
	if overlay == nil {
		return cloneExtensions(base)
	}
	if base == nil {
		return cloneExtensions(overlay)
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
// The JSON files under configDir/projects remain the source of truth;
// in-memory maps are a rebuilt index, never authority.
type Store struct {
	configDir string
	mu        sync.RWMutex
	projects  map[string]*Definition
	index     map[ProjectID]*catalogRecord
	bus       *EventBus
}

var (
	_ Catalog             = (*Store)(nil)
	_ CatalogValidator    = (*Store)(nil)
	_ CatalogWriter       = (*Store)(nil)
	_ CompatibilityWriter = (*Store)(nil)
)

// NewStore creates a store rooted at the Switchboard project catalog config directory.
func NewStore(configDir string) *Store {
	return &Store{
		configDir: configDir,
		projects:  make(map[string]*Definition),
		index:     make(map[ProjectID]*catalogRecord),
	}
}

// DefaultConfigDir returns the default Switchboard project catalog directory.
// It is always under "switchboard" (never "project-interop").
func DefaultConfigDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "switchboard")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "switchboard")
}

// Load discovers and loads all project definitions from the user-level store.
// For each project with a primary repo path, it also attempts to merge a repo-local .project.json.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rebuildIndex()
}

func (s *Store) Definition(name string) (*Definition, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.rebuildIndex()
	d, ok := s.projects[name]
	return cloneDefinition(d), ok
}

// All returns all loaded project definitions.
func (s *Store) All() map[string]*Definition {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.rebuildIndex()
	result := make(map[string]*Definition, len(s.projects))
	for k, v := range s.projects {
		result[k] = cloneDefinition(v)
	}
	return result
}

// Names returns all project names.
func (s *Store) Names() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.rebuildIndex()
	names := make([]string, 0, len(s.projects))
	for k := range s.projects {
		names = append(names, k)
	}
	return names
}

// CreateDefinition writes a new project definition to the user-level store.
// Compatibility adapters should prefer CreateCompatibility.
func (s *Store) CreateDefinition(def *Definition) error {
	if def == nil {
		return fmt.Errorf("invalid project definition: name is required")
	}
	_, _, err := s.CreateCompatibility(context.Background(), CreateRequest{Definition: *def})
	if err != nil {
		if IsCode(err, CodeProjectAlreadyExists) {
			return fmt.Errorf("project %q already exists", def.Name)
		}
		if e, ok := AsError(err); ok && e.Code == CodeInvalidDefinition {
			return fmt.Errorf("invalid project definition: %s", e.Message)
		}
		return err
	}
	return nil
}

// Update applies a JSON merge patch to the user-level store file.
func (s *Store) Update(name string, patch json.RawMessage) (*Definition, error) {
	snap, err := s.PatchCompatibility(context.Background(), ProjectID(name), patch)
	if err != nil {
		if IsCode(err, CodeProjectNotFound) {
			return nil, fmt.Errorf("project %q not found", name)
		}
		return nil, err
	}
	return cloneDefinition(&snap.Definition), nil
}

// DeleteDefinition removes a project definition from the user-level store.
func (s *Store) DeleteDefinition(name string) error {
	err := s.DeleteCompatibility(context.Background(), ProjectID(name))
	if IsCode(err, CodeProjectNotFound) {
		return fmt.Errorf("project %q not found", name)
	}
	return err
}

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
