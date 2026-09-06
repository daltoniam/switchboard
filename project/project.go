package project

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var nameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// PolicyDocument is the closed project policy shape implemented by
// Switchboard. Keys name capabilities; values indicate whether each
// capability is allowed.
type PolicyDocument map[string]bool

// Definition is a Switchboard project catalog entry. The AWM projection is
// version, name, display name, description, boolean capability policy, and
// known_resource_ids (the typed Project.known_resources observation).
// Tools, Agents, and Additional remain compatibility-only fields for existing
// project-scoped gateway definitions and are not writable through AWM gRPC.
type Definition struct {
	Schema           string                     `json:"$schema,omitempty"`
	Version          string                     `json:"version"`
	Name             string                     `json:"name"`
	DisplayName      string                     `json:"display_name,omitempty"`
	Description      string                     `json:"description,omitempty"`
	Policy           PolicyDocument             `json:"policy,omitempty"`
	KnownResourceIDs []string                   `json:"known_resource_ids,omitempty"`
	Tools            map[string]*ScopeRule      `json:"tools,omitempty"`
	Agents           *AgentsConfig              `json:"agents,omitempty"`
	Additional       map[string]json.RawMessage `json:"-"`
}

// ScopeRule defines allow/deny/defaults for a single MCP server.
type ScopeRule struct {
	Allow    []string                  `json:"allow,omitempty"`
	Deny     []string                  `json:"deny,omitempty"`
	Defaults map[string]map[string]any `json:"defaults,omitempty"`
}

// ContextConfig is retained for role context overrides.
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

// Validate checks required catalog fields.
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
	for capability := range d.Policy {
		if strings.TrimSpace(capability) == "" {
			return fmt.Errorf("policy capability names must not be empty")
		}
	}
	seen := make(map[string]struct{}, len(d.KnownResourceIDs))
	for i, id := range d.KnownResourceIDs {
		id = strings.TrimSpace(id)
		if err := validateKnownResourceID(id); err != nil {
			return err
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("duplicate known_resource_ids entry %q", id)
		}
		seen[id] = struct{}{}
		d.KnownResourceIDs[i] = id
	}
	return nil
}

func validateKnownResourceID(id string) error {
	if id == "" {
		return fmt.Errorf("known_resource_ids entries must not be empty")
	}
	if len(id) > 128 {
		return fmt.Errorf("known_resource_ids entry exceeds 128 characters")
	}
	if !nameRE.MatchString(id) {
		return fmt.Errorf("known_resource_ids entry %q does not match pattern ^[a-zA-Z0-9][a-zA-Z0-9._-]*$", id)
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

// ResolvedRepo returns empty; projects no longer bind a primary repository path.
func (d *Definition) ResolvedRepo() string { return "" }

// PrimaryRepo returns empty under the id+description schema.
func (d *Definition) PrimaryRepo() string { return "" }

// PrimaryBranch returns empty under the id+description schema.
func (d *Definition) PrimaryBranch() string { return "" }

// Merge merges a higher-precedence definition onto a base.
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
	if overlay.DisplayName != "" {
		result.DisplayName = overlay.DisplayName
	}
	if overlay.Description != "" {
		result.Description = overlay.Description
	}
	if overlay.Policy != nil {
		result.Policy = clonePolicy(overlay.Policy)
	}
	if overlay.KnownResourceIDs != nil {
		result.KnownResourceIDs = cloneKnownResourceIDs(overlay.KnownResourceIDs)
	}
	if overlay.Version != "" {
		result.Version = overlay.Version
	}
	result.Tools = mergeTools(base.Tools, overlay.Tools)
	result.Agents = mergeAgents(base.Agents, overlay.Agents)
	result.Additional = mergeAdditional(base.Additional, overlay.Additional)
	return result, nil
}

func clonePolicy(in PolicyDocument) PolicyDocument {
	if in == nil {
		return nil
	}
	out := make(PolicyDocument, len(in))
	for capability, allowed := range in {
		out[capability] = allowed
	}
	return out
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

func mergeTools(base, overlay map[string]*ScopeRule) map[string]*ScopeRule {
	if overlay == nil {
		return cloneTools(base)
	}
	if base == nil {
		return cloneTools(overlay)
	}
	result := make(map[string]*ScopeRule, len(base)+len(overlay))
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
		c.Defaults = make(map[string]map[string]any, len(r.Defaults))
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
	result := &AgentsConfig{MaxConcurrent: base.MaxConcurrent}
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

// Store manages project definitions in the user-level store.
// The JSON files under configDir/projects remain the source of truth;
// in-memory maps are a rebuilt index, never authority.
// DeleteGuard rejects Project deletion while retained dependents still reference it.
// Used by both canonical delete and DeleteCompatibility so dual writers share one rule.
// WithExclusive + Unlocked assert keep the check and remove race-free against session create.
type DeleteGuard interface {
	AssertProjectDeletable(ctx context.Context, projectID string) error
	AssertProjectDeletableUnlocked(projectID string) error
	WithExclusive(ctx context.Context, fn func() error) error
}

type Store struct {
	configDir string
	mu        sync.RWMutex
	projects  map[string]*Definition
	index     map[ProjectID]*catalogRecord
	bus       *EventBus
	delGuard  DeleteGuard
	resources ResourcePresence
}

// SetDeleteGuard attaches a referential-integrity check used by Delete and
// DeleteCompatibility (e.g. AWM work-session references).
func (s *Store) SetDeleteGuard(g DeleteGuard) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delGuard = g
}

// SetResourcePresence attaches the Resource store used to check that
// known_resource_ids currently exist. Presence is advisory at write time and
// is not an atomic foreign key across the two stores.
func (s *Store) SetResourcePresence(p ResourcePresence) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resources = p
}

var (
	_ Catalog             = (*Store)(nil)
	_ CatalogValidator    = (*Store)(nil)
	_ DefinitionValidator = (*Store)(nil)
	_ CatalogWriter       = (*Store)(nil)
	_ CatalogReplacer     = (*Store)(nil)
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

// ConfigDir returns the store's config directory root.
func (s *Store) ConfigDir() string {
	return s.configDir
}
