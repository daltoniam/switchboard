// Package awm implements a minimal Agent Work Model subset for Switchboard:
// Project, Resource, ResourceBinding, WorkProfile (session blueprint),
// WorkSession, and AgentProfile.
//
// Source vocabulary: Agent Work Model (project-catalog / profile-catalog /
// native-resource-provider / work-session-coordinator roles). Unqualified "session" is avoided in field
// names; WorkSession is never an MCP transport connection.
package awm

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/daltoniam/switchboard/project"
)

var idRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// PolicyDocument is the shared closed project/work policy shape.
type PolicyDocument = project.PolicyDocument

// AgentConstraints is the closed AgentProfile constraint subset implemented by
// Switchboard. ProjectID is retained for compatibility with older files;
// ProjectIDs is the canonical multi-project spelling.
type AgentConstraints struct {
	ProjectID  string   `json:"project_id,omitempty"`
	ProjectIDs []string `json:"project_ids,omitempty"`
}

// AllowsProject reports whether the constraint admits projectID. An empty
// constraint is global. The legacy singular field and canonical list are OR'd.
func (c *AgentConstraints) AllowsProject(projectID string) bool {
	if c == nil || (c.ProjectID == "" && len(c.ProjectIDs) == 0) {
		return true
	}
	if c.ProjectID == projectID {
		return true
	}
	for _, id := range c.ProjectIDs {
		if id == projectID {
			return true
		}
	}
	return false
}

// Project is a durable named collaboration scope (AWM Project).
// Catalog identity is project_id (name) plus optional description.
// Tools remain optional for project-scoped gateway policy when projected.
type Project struct {
	Version     string         `json:"version"`
	ProjectID   string         `json:"project_id"`
	Name        string         `json:"name,omitempty"` // alias of project_id when present
	DisplayName string         `json:"display_name,omitempty"`
	Description string         `json:"description,omitempty"`
	Policy      PolicyDocument `json:"policy,omitempty"`
}

// Resource is an independently addressable thing relevant to work. Its native
// provider remains authoritative; AWM records only observe or bind it.
type Resource struct {
	Version     string `json:"version"`
	ResourceID  string `json:"resource_id"`
	URI         string `json:"uri"`
	Kind        string `json:"kind"`
	DisplayName string `json:"display_name,omitempty"`
}

// ResourceBinding is a WorkSession-specific resolution and narrowed grant for
// one Resource. It does not own the Resource.
type ResourceBinding struct {
	Version           string         `json:"version"`
	ResourceBindingID string         `json:"resource_binding_id"`
	WorkSessionID     string         `json:"work_session_id"`
	ResourceID        string         `json:"resource_id"`
	Grant             PolicyDocument `json:"grant,omitempty"`
	ResolvedLocator   string         `json:"resolved_locator,omitempty"`
	State             string         `json:"state"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// ResourceBinding lifecycle states (AWM).
const (
	BindingStateProposed = "proposed"
	BindingStateBound    = "bound"
	BindingStateRevoked  = "revoked"
)

// WorkProfile is a reusable blueprint for a kind of WorkSession (AWM WorkProfile).
// Also referred to informally as a "session profile" in product language.
type WorkProfile struct {
	Version           string         `json:"version"`
	WorkProfileID     string         `json:"work_profile_id"`
	DisplayName       string         `json:"display_name,omitempty"`
	Description       string         `json:"description,omitempty"`
	ProjectIDs        []string       `json:"project_ids,omitempty"`
	IntendedResources []string       `json:"intended_resources,omitempty"`
	DefaultPolicy     PolicyDocument `json:"default_policy,omitempty"`
}

// AgentProfile is a declarative eligible agent kind (not a running instance).
type AgentProfile struct {
	Version        string            `json:"version"`
	AgentProfileID string            `json:"agent_profile_id"`
	DisplayName    string            `json:"display_name,omitempty"`
	Description    string            `json:"description,omitempty"`
	Capabilities   []string          `json:"capabilities,omitempty"`
	Constraints    *AgentConstraints `json:"constraints,omitempty"`
}

// WorkSession is a bounded episode of work. It is never an MCP connection
// and never a host chat transcript.
type WorkSession struct {
	Version           string         `json:"version"`
	WorkSessionID     string         `json:"work_session_id"`
	DisplayName       string         `json:"display_name,omitempty"`
	ProjectID         string         `json:"project_id,omitempty"`
	ProjectSnapshotID string         `json:"project_snapshot_id,omitempty"`
	ProjectRevision   string         `json:"project_revision,omitempty"`
	WorkProfileID     string         `json:"work_profile_id,omitempty"`
	AgentProfileIDs   []string       `json:"agent_profile_ids,omitempty"`
	State             string         `json:"state"`
	Policy            PolicyDocument `json:"policy,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	ClosedAt          *time.Time     `json:"closed_at,omitempty"`
}

// WorkSession lifecycle states (AWM).
const (
	StateProposed = "proposed"
	StateOpen     = "open"
	StatePaused   = "paused"
	StateClosed   = "closed"
	StateAborted  = "aborted"
)

func validateAddressURI(name, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" {
		return fmt.Errorf("%s must be an absolute URI", name)
	}
	if parsed.User != nil {
		if _, hasPassword := parsed.User.Password(); hasPassword {
			return fmt.Errorf("%s must not contain credentials", name)
		}
	}
	return nil
}

func validatePolicy(name string, policy PolicyDocument) error {
	for capability := range policy {
		if strings.TrimSpace(capability) == "" {
			return fmt.Errorf("%s capability names must not be empty", name)
		}
	}
	return nil
}

func validateID(kind, id string) error {
	if id == "" {
		return fmt.Errorf("%s is required", kind)
	}
	if len(id) > 128 {
		return fmt.Errorf("%s exceeds 128 characters", kind)
	}
	if !idRE.MatchString(id) {
		return fmt.Errorf("%s %q does not match ^[A-Za-z0-9][A-Za-z0-9._-]*$", kind, id)
	}
	return nil
}

// Validate checks Project required fields.
func (p *Project) Validate() error {
	if p.Version != "1" {
		return fmt.Errorf(`unsupported version %q (must be "1")`, p.Version)
	}
	id := p.ProjectID
	if id == "" {
		id = p.Name
	}
	if err := validateID("project_id", id); err != nil {
		return err
	}
	// Normalize dual id fields.
	p.ProjectID = id
	if p.Name == "" {
		p.Name = id
	}
	return nil
}

// Validate checks Resource required fields.
func (r *Resource) Validate() error {
	if r.Version != "1" {
		return fmt.Errorf(`unsupported version %q (must be "1")`, r.Version)
	}
	if err := validateID("resource_id", r.ResourceID); err != nil {
		return err
	}
	if err := validateAddressURI("resource uri", r.URI); err != nil {
		return err
	}
	if strings.TrimSpace(r.Kind) == "" {
		return fmt.Errorf("resource kind is required")
	}
	return nil
}

// Validate checks ResourceBinding required fields and lifecycle state.
func (b *ResourceBinding) Validate() error {
	if b.Version != "1" {
		return fmt.Errorf(`unsupported version %q (must be "1")`, b.Version)
	}
	if err := validateID("resource_binding_id", b.ResourceBindingID); err != nil {
		return err
	}
	if err := validateID("work_session_id", b.WorkSessionID); err != nil {
		return err
	}
	if err := validateID("resource_id", b.ResourceID); err != nil {
		return err
	}
	switch b.State {
	case BindingStateProposed, BindingStateBound, BindingStateRevoked:
	case "":
		return fmt.Errorf("resource binding state is required")
	default:
		return fmt.Errorf("unsupported resource binding state %q", b.State)
	}
	if b.State == BindingStateBound && strings.TrimSpace(b.ResolvedLocator) == "" {
		return fmt.Errorf("resolved_locator is required for a bound resource binding")
	}
	if b.ResolvedLocator != "" {
		if err := validateAddressURI("resolved_locator", b.ResolvedLocator); err != nil {
			return err
		}
	}
	return validatePolicy("grant", b.Grant)
}

// CanTransitionResourceBinding reports whether a binding lifecycle move is allowed.
func CanTransitionResourceBinding(from, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case BindingStateProposed:
		return to == BindingStateBound || to == BindingStateRevoked
	case BindingStateBound:
		return to == BindingStateRevoked
	default:
		return false
	}
}

// Validate checks WorkProfile required fields.
func (p *WorkProfile) Validate() error {
	if p.Version != "1" {
		return fmt.Errorf(`unsupported version %q (must be "1")`, p.Version)
	}
	if err := validateID("work_profile_id", p.WorkProfileID); err != nil {
		return err
	}
	return validatePolicy("default_policy", p.DefaultPolicy)
}

// Validate checks AgentProfile required fields.
func (p *AgentProfile) Validate() error {
	if p.Version != "1" {
		return fmt.Errorf(`unsupported version %q (must be "1")`, p.Version)
	}
	if err := validateID("agent_profile_id", p.AgentProfileID); err != nil {
		return err
	}
	if p.Constraints != nil {
		if p.Constraints.ProjectID != "" {
			if err := validateID("project_id", p.Constraints.ProjectID); err != nil {
				return err
			}
		}
		for _, id := range p.Constraints.ProjectIDs {
			if err := validateID("project_id", id); err != nil {
				return err
			}
		}
	}
	return nil
}

// Validate checks WorkSession required fields and lifecycle state.
func (s *WorkSession) Validate() error {
	if s.Version != "1" {
		return fmt.Errorf(`unsupported version %q (must be "1")`, s.Version)
	}
	if err := validateID("work_session_id", s.WorkSessionID); err != nil {
		return err
	}
	switch s.State {
	case StateProposed, StateOpen, StatePaused, StateClosed, StateAborted:
	case "":
		return fmt.Errorf("state is required")
	default:
		return fmt.Errorf("unsupported work session state %q", s.State)
	}
	if s.ProjectID != "" {
		if err := validateID("project_id", s.ProjectID); err != nil {
			return err
		}
	}
	if s.WorkProfileID != "" {
		if err := validateID("work_profile_id", s.WorkProfileID); err != nil {
			return err
		}
	}
	for _, id := range s.AgentProfileIDs {
		if err := validateID("agent_profile_id", id); err != nil {
			return err
		}
	}
	return validatePolicy("policy", s.Policy)
}

// CanTransition reports whether a lifecycle move is allowed.
func CanTransition(from, to string) bool {
	if from == to {
		return true
	}
	allowed := map[string][]string{
		StateProposed: {StateOpen, StateAborted},
		StateOpen:     {StatePaused, StateClosed, StateAborted},
		StatePaused:   {StateOpen, StateClosed, StateAborted},
	}
	for _, next := range allowed[from] {
		if next == to {
			return true
		}
	}
	return false
}
