package awm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

// Store is a filesystem-backed AWM catalog under a Switchboard config root.
// Layout:
//
//	<root>/projects/<id>.project.json   (Project — preferred legacy-compatible path)
//	<root>/awm/projects/<id>.json       (optional alternate)
//	<root>/awm/work_profiles/<id>.json
//	<root>/awm/agent_profiles/<id>.json
//	<root>/awm/work_sessions/<id>.json
//
// Root should be ~/.config/switchboard (never project-interop).
type Store struct {
	configRoot string // Switchboard config root
	root       string // <configRoot>/awm
	mu         sync.Mutex
}

// NewStore creates an AWM store under configRoot (projects + awm/ subtree).
func NewStore(configRoot string) *Store {
	return &Store{
		configRoot: configRoot,
		root:       filepath.Join(configRoot, "awm"),
	}
}

// Root returns the awm subdirectory path.
func (s *Store) Root() string { return s.root }

// ConfigRoot returns the Switchboard config root (parent of awm/).
func (s *Store) ConfigRoot() string { return s.configRoot }

func (s *Store) projectsDir() string      { return filepath.Join(s.configRoot, "projects") }
func (s *Store) awmProjectsDir() string   { return filepath.Join(s.root, "projects") }
func (s *Store) workProfilesDir() string  { return filepath.Join(s.root, "work_profiles") }
func (s *Store) agentProfilesDir() string { return filepath.Join(s.root, "agent_profiles") }
func (s *Store) workSessionsDir() string  { return filepath.Join(s.root, "work_sessions") }
func (s *Store) lockPath() string         { return filepath.Join(s.root, ".awm.lock") }

func (s *Store) withLock(ctx context.Context, fn func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(s.root, 0700); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fl := flock.New(s.lockPath())
	deadline := time.Now().Add(10 * time.Second)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		locked, err := fl.TryLock()
		if err != nil {
			return err
		}
		if locked {
			defer func() { _ = fl.Unlock() }()
			return fn()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("awm lock timeout")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func atomicWriteJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func readJSON[T any](path string) (T, error) {
	var zero T
	data, err := os.ReadFile(path)
	if err != nil {
		return zero, err
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return zero, err
	}
	return v, nil
}

func listJSONIDs(dir, suffix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		ids = append(ids, strings.TrimSuffix(e.Name(), suffix))
	}
	sort.Strings(ids)
	return ids, nil
}

// --- Project ---

func (s *Store) projectPath(id string) string {
	// Prefer top-level projects/ for compatibility with the existing catalog UI.
	return filepath.Join(s.projectsDir(), id+".project.json")
}

func (s *Store) projectAltPath(id string) string {
	return filepath.Join(s.awmProjectsDir(), id+".json")
}

// normalizeProject fills dual id fields from file contents.
func normalizeProject(p Project) Project {
	if p.Version == "" {
		p.Version = "1"
	}
	if p.ProjectID == "" {
		p.ProjectID = p.Name
	}
	if p.Name == "" {
		p.Name = p.ProjectID
	}
	if p.DisplayName == "" {
		p.DisplayName = p.Name
	}
	return p
}

// PutProject creates or replaces a Project definition.
func (s *Store) PutProject(ctx context.Context, p Project) (Project, error) {
	p = normalizeProject(p)
	if err := p.Validate(); err != nil {
		return Project{}, err
	}
	err := s.withLock(ctx, func() error {
		return atomicWriteJSON(s.projectPath(p.ProjectID), p)
	})
	return p, err
}

// GetProject loads a project by id from projects/ or awm/projects/.
func (s *Store) GetProject(ctx context.Context, id string) (Project, error) {
	if err := ctx.Err(); err != nil {
		return Project{}, err
	}
	if err := validateID("project_id", id); err != nil {
		return Project{}, err
	}
	return s.lookupProjectUnlocked(id)
}

// ListProjects returns all projects sorted by id.
func (s *Store) ListProjects(ctx context.Context) ([]Project, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	seen := map[string]Project{}
	// Preferred directory.
	ids, _ := listProjectFileIDs(s.projectsDir(), ".project.json")
	for _, id := range ids {
		p, err := s.GetProject(ctx, id)
		if err == nil {
			seen[id] = p
		}
	}
	// Alternate directory fills gaps only.
	alts, _ := listJSONIDs(s.awmProjectsDir(), ".json")
	for _, id := range alts {
		if _, ok := seen[id]; ok {
			continue
		}
		p, err := s.GetProject(ctx, id)
		if err == nil {
			seen[id] = p
		}
	}
	out := make([]Project, 0, len(seen))
	for _, p := range seen {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ProjectID < out[j].ProjectID })
	return out, nil
}

func listProjectFileIDs(dir, suffix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		ids = append(ids, strings.TrimSuffix(e.Name(), suffix))
	}
	sort.Strings(ids)
	return ids, nil
}

// DeleteProject removes a project definition.
func (s *Store) DeleteProject(ctx context.Context, id string) error {
	if err := validateID("project_id", id); err != nil {
		return err
	}
	return s.withLock(ctx, func() error {
		removed := false
		for _, path := range []string{s.projectPath(id), s.projectAltPath(id)} {
			if err := os.Remove(path); err == nil {
				removed = true
			} else if !os.IsNotExist(err) {
				return err
			}
		}
		if !removed {
			return fmt.Errorf("project %q not found", id)
		}
		return nil
	})
}

// --- WorkProfile ---

func (s *Store) workProfilePath(id string) string {
	return filepath.Join(s.workProfilesDir(), id+".json")
}

// PutWorkProfile creates or replaces a work profile.
func (s *Store) PutWorkProfile(ctx context.Context, p WorkProfile) (WorkProfile, error) {
	if p.Version == "" {
		p.Version = "1"
	}
	if err := p.Validate(); err != nil {
		return WorkProfile{}, err
	}
	err := s.withLock(ctx, func() error {
		return atomicWriteJSON(s.workProfilePath(p.WorkProfileID), p)
	})
	return p, err
}

// GetWorkProfile loads a work profile by id.
func (s *Store) GetWorkProfile(ctx context.Context, id string) (WorkProfile, error) {
	if err := ctx.Err(); err != nil {
		return WorkProfile{}, err
	}
	if err := validateID("work_profile_id", id); err != nil {
		return WorkProfile{}, err
	}
	p, err := readJSON[WorkProfile](s.workProfilePath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return WorkProfile{}, fmt.Errorf("work profile %q not found", id)
		}
		return WorkProfile{}, err
	}
	return p, nil
}

// ListWorkProfiles returns all work profiles sorted by id.
func (s *Store) ListWorkProfiles(ctx context.Context) ([]WorkProfile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ids, err := listJSONIDs(s.workProfilesDir(), ".json")
	if err != nil {
		return nil, err
	}
	out := make([]WorkProfile, 0, len(ids))
	for _, id := range ids {
		p, err := s.GetWorkProfile(ctx, id)
		if err != nil {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// DeleteWorkProfile removes a work profile.
func (s *Store) DeleteWorkProfile(ctx context.Context, id string) error {
	if err := validateID("work_profile_id", id); err != nil {
		return err
	}
	return s.withLock(ctx, func() error {
		path := s.workProfilePath(id)
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("work profile %q not found", id)
			}
			return err
		}
		return nil
	})
}

// --- AgentProfile ---

func (s *Store) agentProfilePath(id string) string {
	return filepath.Join(s.agentProfilesDir(), id+".json")
}

// PutAgentProfile creates or replaces an agent profile.
func (s *Store) PutAgentProfile(ctx context.Context, p AgentProfile) (AgentProfile, error) {
	if p.Version == "" {
		p.Version = "1"
	}
	if err := p.Validate(); err != nil {
		return AgentProfile{}, err
	}
	err := s.withLock(ctx, func() error {
		return atomicWriteJSON(s.agentProfilePath(p.AgentProfileID), p)
	})
	return p, err
}

// GetAgentProfile loads an agent profile by id.
func (s *Store) GetAgentProfile(ctx context.Context, id string) (AgentProfile, error) {
	if err := ctx.Err(); err != nil {
		return AgentProfile{}, err
	}
	if err := validateID("agent_profile_id", id); err != nil {
		return AgentProfile{}, err
	}
	p, err := readJSON[AgentProfile](s.agentProfilePath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return AgentProfile{}, fmt.Errorf("agent profile %q not found", id)
		}
		return AgentProfile{}, err
	}
	return p, nil
}

// ListAgentProfiles returns all agent profiles sorted by id.
func (s *Store) ListAgentProfiles(ctx context.Context) ([]AgentProfile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ids, err := listJSONIDs(s.agentProfilesDir(), ".json")
	if err != nil {
		return nil, err
	}
	out := make([]AgentProfile, 0, len(ids))
	for _, id := range ids {
		p, err := s.GetAgentProfile(ctx, id)
		if err != nil {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// DeleteAgentProfile removes an agent profile.
func (s *Store) DeleteAgentProfile(ctx context.Context, id string) error {
	if err := validateID("agent_profile_id", id); err != nil {
		return err
	}
	return s.withLock(ctx, func() error {
		path := s.agentProfilePath(id)
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("agent profile %q not found", id)
			}
			return err
		}
		return nil
	})
}

// --- WorkSession ---

func (s *Store) workSessionPath(id string) string {
	return filepath.Join(s.workSessionsDir(), id+".json")
}

// lookupProjectUnlocked reads a project without taking the store lock.
func (s *Store) lookupProjectUnlocked(id string) (Project, error) {
	for _, path := range []string{s.projectPath(id), s.projectAltPath(id)} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var p Project
		if err := json.Unmarshal(data, &p); err == nil {
			if p.ProjectID == "" {
				p.ProjectID = p.Name
			}
			if p.Name == "" {
				p.Name = p.ProjectID
			}
			if p.Version == "" {
				p.Version = "1"
			}
			if p.ProjectID == id || p.Name == id {
				return normalizeProject(p), nil
			}
		}
		var raw map[string]any
		if json.Unmarshal(data, &raw) != nil {
			continue
		}
		name, _ := raw["name"].(string)
		pid, _ := raw["project_id"].(string)
		if pid == "" {
			pid = name
		}
		if pid != "" && pid != id && name != id {
			continue
		}
		if pid == "" {
			pid = id
		}
		desc, _ := raw["description"].(string)
		dn, _ := raw["display_name"].(string)
		ver, _ := raw["version"].(string)
		if ver == "" {
			ver = "1"
		}
		return normalizeProject(Project{Version: ver, ProjectID: pid, Name: name, DisplayName: dn, Description: desc}), nil
	}
	return Project{}, fmt.Errorf("project %q not found", id)
}

// CreateWorkSession creates a new work session in proposed or open state.
func (s *Store) CreateWorkSession(ctx context.Context, sess WorkSession) (WorkSession, error) {
	if sess.Version == "" {
		sess.Version = "1"
	}
	if sess.State == "" {
		sess.State = StateProposed
	}
	now := time.Now().UTC()
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = now
	}
	sess.UpdatedAt = now
	if err := sess.Validate(); err != nil {
		return WorkSession{}, err
	}
	err := s.withLock(ctx, func() error {
		path := s.workSessionPath(sess.WorkSessionID)
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("work session %q already exists", sess.WorkSessionID)
		}
		// Referential checks (missing targets fail closed).
		if sess.ProjectID != "" {
			if _, err := s.lookupProjectUnlocked(sess.ProjectID); err != nil {
				return fmt.Errorf("project_id %q not found", sess.ProjectID)
			}
		}
		if sess.WorkProfileID != "" {
			if _, err := readJSON[WorkProfile](s.workProfilePath(sess.WorkProfileID)); err != nil {
				return fmt.Errorf("work_profile_id %q not found", sess.WorkProfileID)
			}
		}
		for _, id := range sess.AgentProfileIDs {
			if _, err := readJSON[AgentProfile](s.agentProfilePath(id)); err != nil {
				return fmt.Errorf("agent_profile_id %q not found", id)
			}
		}
		return atomicWriteJSON(path, sess)
	})
	return sess, err
}

// GetWorkSession loads a work session by id.
func (s *Store) GetWorkSession(ctx context.Context, id string) (WorkSession, error) {
	if err := ctx.Err(); err != nil {
		return WorkSession{}, err
	}
	if err := validateID("work_session_id", id); err != nil {
		return WorkSession{}, err
	}
	sess, err := readJSON[WorkSession](s.workSessionPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return WorkSession{}, fmt.Errorf("work session %q not found", id)
		}
		return WorkSession{}, err
	}
	return sess, nil
}

// ListWorkSessions returns sessions, optionally filtered by state and/or project.
func (s *Store) ListWorkSessions(ctx context.Context, state, projectID string) ([]WorkSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ids, err := listJSONIDs(s.workSessionsDir(), ".json")
	if err != nil {
		return nil, err
	}
	out := make([]WorkSession, 0, len(ids))
	for _, id := range ids {
		sess, err := s.GetWorkSession(ctx, id)
		if err != nil {
			continue
		}
		if state != "" && sess.State != state {
			continue
		}
		if projectID != "" && sess.ProjectID != projectID {
			continue
		}
		out = append(out, sess)
	}
	return out, nil
}

// TransitionWorkSession moves a session to a new lifecycle state.
func (s *Store) TransitionWorkSession(ctx context.Context, id, toState string) (WorkSession, error) {
	var out WorkSession
	err := s.withLock(ctx, func() error {
		sess, err := readJSON[WorkSession](s.workSessionPath(id))
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("work session %q not found", id)
			}
			return err
		}
		if !CanTransition(sess.State, toState) {
			return fmt.Errorf("cannot transition work session from %q to %q", sess.State, toState)
		}
		sess.State = toState
		sess.UpdatedAt = time.Now().UTC()
		if toState == StateClosed || toState == StateAborted {
			now := sess.UpdatedAt
			sess.ClosedAt = &now
		}
		if err := sess.Validate(); err != nil {
			return err
		}
		if err := atomicWriteJSON(s.workSessionPath(id), sess); err != nil {
			return err
		}
		out = sess
		return nil
	})
	return out, err
}

// PatchWorkSession updates mutable descriptive fields without changing state.
func (s *Store) PatchWorkSession(ctx context.Context, id string, displayName *string, agentProfileIDs *[]string, policy map[string]any) (WorkSession, error) {
	var out WorkSession
	err := s.withLock(ctx, func() error {
		sess, err := readJSON[WorkSession](s.workSessionPath(id))
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("work session %q not found", id)
			}
			return err
		}
		if sess.State == StateClosed || sess.State == StateAborted {
			return fmt.Errorf("cannot patch work session in terminal state %q", sess.State)
		}
		if displayName != nil {
			sess.DisplayName = *displayName
		}
		if agentProfileIDs != nil {
			for _, ap := range *agentProfileIDs {
				if _, err := readJSON[AgentProfile](s.agentProfilePath(ap)); err != nil {
					return fmt.Errorf("agent_profile_id %q not found", ap)
				}
			}
			sess.AgentProfileIDs = append([]string(nil), (*agentProfileIDs)...)
		}
		if policy != nil {
			sess.Policy = policy
		}
		sess.UpdatedAt = time.Now().UTC()
		if err := sess.Validate(); err != nil {
			return err
		}
		if err := atomicWriteJSON(s.workSessionPath(id), sess); err != nil {
			return err
		}
		out = sess
		return nil
	})
	return out, err
}

// DeleteWorkSession removes a session record (does not cascade to projects).
func (s *Store) DeleteWorkSession(ctx context.Context, id string) error {
	if err := validateID("work_session_id", id); err != nil {
		return err
	}
	return s.withLock(ctx, func() error {
		if err := os.Remove(s.workSessionPath(id)); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("work session %q not found", id)
			}
			return err
		}
		return nil
	})
}
