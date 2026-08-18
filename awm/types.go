// Package awm implements a minimal Agent Work Model subset for Switchboard:
// WorkProfile (session blueprint), WorkSession, and AgentProfile.
//
// Source vocabulary: Agent Work Model (project-catalog / profile-catalog /
// work-session-coordinator roles). Unqualified "session" is avoided in field
// names; WorkSession is never an MCP transport connection.
package awm

import (
	"fmt"
	"regexp"
	"time"
)

var idRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// WorkProfile is a reusable blueprint for a kind of WorkSession (AWM WorkProfile).
// Also referred to informally as a "session profile" in product language.
type WorkProfile struct {
	Version           string         `json:"version"`
	WorkProfileID     string         `json:"work_profile_id"`
	DisplayName       string         `json:"display_name,omitempty"`
	Description       string         `json:"description,omitempty"`
	ProjectIDs        []string       `json:"project_ids,omitempty"`
	IntendedResources []string       `json:"intended_resources,omitempty"`
	DefaultPolicy     map[string]any `json:"default_policy,omitempty"`
}

// AgentProfile is a declarative eligible agent kind (not a running instance).
type AgentProfile struct {
	Version        string         `json:"version"`
	AgentProfileID string         `json:"agent_profile_id"`
	DisplayName    string         `json:"display_name,omitempty"`
	Description    string         `json:"description,omitempty"`
	Capabilities   []string       `json:"capabilities,omitempty"`
	Constraints    map[string]any `json:"constraints,omitempty"`
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
	Policy            map[string]any `json:"policy,omitempty"`
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

// Validate checks WorkProfile required fields.
func (p *WorkProfile) Validate() error {
	if p.Version != "1" {
		return fmt.Errorf("unsupported version %q (must be \"1\")", p.Version)
	}
	return validateID("work_profile_id", p.WorkProfileID)
}

// Validate checks AgentProfile required fields.
func (p *AgentProfile) Validate() error {
	if p.Version != "1" {
		return fmt.Errorf("unsupported version %q (must be \"1\")", p.Version)
	}
	return validateID("agent_profile_id", p.AgentProfileID)
}

// Validate checks WorkSession required fields and lifecycle state.
func (s *WorkSession) Validate() error {
	if s.Version != "1" {
		return fmt.Errorf("unsupported version %q (must be \"1\")", s.Version)
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
	return nil
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
