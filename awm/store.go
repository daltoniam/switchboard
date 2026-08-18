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

	"github.com/daltoniam/switchboard/project"
	"github.com/gofrs/flock"
)

// DefaultWorkProfileID is the exact work_profile_id preselected by clients
// when callers omit a profile choice.
const DefaultWorkProfileID = "default"

// ProjectCatalog is the subset of the project catalog used to pin immutable
// Project revisions/snapshots into WorkSessions.
type ProjectCatalog interface {
	Get(ctx context.Context, id project.ProjectID) (project.Snapshot, error)
	GetRevision(ctx context.Context, id project.ProjectID, rev project.Revision) (project.RevisionSnapshot, error)
}

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
	catalog    ProjectCatalog
	mu         sync.Mutex
}

// NewStore creates an AWM store under configRoot (projects + awm/ subtree).
func NewStore(configRoot string) *Store {
	return &Store{
		configRoot: configRoot,
		root:       filepath.Join(configRoot, "awm"),
	}
}

// SetCatalog attaches the authoritative project catalog used for revision
// archive lookups and ProjectSnapshot pinning. Optional for pure unit tests
// that only exercise profile CRUD without project-bound sessions.
func (s *Store) SetCatalog(c ProjectCatalog) {
	s.catalog = c
}

// SnapshotIDForRevision returns the deterministic project_snapshot_id for a
// pinned Project revision (canonical revision resource URI).
func SnapshotIDForRevision(projectID, revision string) string {
	return fmt.Sprintf("project://registry/projects/%s/revisions/%s", projectID, revision)
}

// EnsureDefaultWorkProfile seeds work_profile_id "default" if absent.
// It never overwrites an operator-modified existing default record.
func (s *Store) EnsureDefaultWorkProfile(ctx context.Context) (WorkProfile, error) {
	existing, err := s.GetWorkProfile(ctx, DefaultWorkProfileID)
	if err == nil {
		return existing, nil
	}
	if !IsCode(err, CodeNotFound) {
		// Tolerate legacy string errors during transition.
		if !strings.Contains(err.Error(), "not found") {
			return WorkProfile{}, err
		}
	}
	return s.PutWorkProfile(ctx, WorkProfile{
		Version:           "1",
		WorkProfileID:     DefaultWorkProfileID,
		DisplayName:       DefaultWorkProfileID,
		Description:       "Default WorkSession blueprint",
		IntendedResources: []string{},
		DefaultPolicy:     map[string]any{},
	})
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
		return invalidInput(err.Error())
	}
	return s.withLock(ctx, func() error {
		if ref := s.firstSessionReferencingProjectUnlocked(id); ref != "" {
			return &Error{
				Code:          CodeReferenced,
				Message:       "project is referenced by a retained work session",
				EntityKind:    "project",
				EntityID:      id,
				ProjectID:     id,
				WorkSessionID: ref,
			}
		}
		removed := false
		for _, path := range []string{s.projectPath(id), s.projectAltPath(id)} {
			if err := os.Remove(path); err == nil {
				removed = true
			} else if !os.IsNotExist(err) {
				return err
			}
		}
		if !removed {
			return notFound("project", id)
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
		return WorkProfile{}, invalidInput(err.Error())
	}
	if err := s.validateProfileProjectIDs(ctx, p); err != nil {
		return WorkProfile{}, err
	}
	err := s.withLock(ctx, func() error {
		return atomicWriteJSON(s.workProfilePath(p.WorkProfileID), p)
	})
	return p, err
}

func (s *Store) validateProfileProjectIDs(ctx context.Context, p WorkProfile) error {
	for _, pid := range p.ProjectIDs {
		if err := validateID("project_id", pid); err != nil {
			return invalidInput(err.Error())
		}
		if _, err := s.lookupProjectUnlocked(pid); err != nil {
			// Prefer catalog when available.
			if s.catalog != nil {
				if _, cerr := s.catalog.Get(ctx, project.ProjectID(pid)); cerr != nil {
					return invalidRef("project_id", pid, "project_id not found")
				}
				continue
			}
			return invalidRef("project_id", pid, "project_id not found")
		}
	}
	return nil
}

// GetWorkProfile loads a work profile by id.
func (s *Store) GetWorkProfile(ctx context.Context, id string) (WorkProfile, error) {
	if err := ctx.Err(); err != nil {
		return WorkProfile{}, err
	}
	if err := validateID("work_profile_id", id); err != nil {
		return WorkProfile{}, invalidInput(err.Error())
	}
	p, err := readJSON[WorkProfile](s.workProfilePath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return WorkProfile{}, notFound("work_profile", id)
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

// DeleteWorkProfile removes a work profile when no retained WorkSession references it.
func (s *Store) DeleteWorkProfile(ctx context.Context, id string) error {
	if err := validateID("work_profile_id", id); err != nil {
		return invalidInput(err.Error())
	}
	return s.withLock(ctx, func() error {
		path := s.workProfilePath(id)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return notFound("work_profile", id)
			}
			return err
		}
		if ref := s.firstSessionReferencingProfileUnlocked(id); ref != "" {
			return &Error{
				Code:          CodeReferenced,
				Message:       "work profile is referenced by a retained work session",
				EntityKind:    "work_profile",
				EntityID:      id,
				WorkProfileID: id,
				WorkSessionID: ref,
			}
		}
		return os.Remove(path)
	})
}

func (s *Store) firstSessionReferencingProfileUnlocked(profileID string) string {
	ids, err := listJSONIDs(s.workSessionsDir(), ".json")
	if err != nil {
		return ""
	}
	for _, id := range ids {
		sess, err := readJSON[WorkSession](s.workSessionPath(id))
		if err != nil {
			continue
		}
		if sess.WorkProfileID == profileID {
			return sess.WorkSessionID
		}
	}
	return ""
}

func (s *Store) firstSessionReferencingProjectUnlocked(projectID string) string {
	ids, err := listJSONIDs(s.workSessionsDir(), ".json")
	if err != nil {
		return ""
	}
	for _, id := range ids {
		sess, err := readJSON[WorkSession](s.workSessionPath(id))
		if err != nil {
			continue
		}
		if sess.ProjectID == projectID {
			return sess.WorkSessionID
		}
	}
	return ""
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
// Project-bound sessions pin one exact immutable Project revision/snapshot.
// Idempotent: repeating create with the same work_session_id and matching
// fields returns the existing record; mismatched fields yield conflict.
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
		return WorkSession{}, invalidInput(err.Error())
	}

	// Resolve profile eligibility and pin outside the lock where catalog I/O is needed.
	if err := s.resolveSessionPin(ctx, &sess); err != nil {
		return WorkSession{}, err
	}
	if err := s.validateSessionProfile(ctx, sess); err != nil {
		return WorkSession{}, err
	}
	if err := s.validateSessionPolicy(sess); err != nil {
		return WorkSession{}, err
	}

	var out WorkSession
	err := s.withLock(ctx, func() error {
		path := s.workSessionPath(sess.WorkSessionID)
		if existing, err := readJSON[WorkSession](path); err == nil {
			if sessionsCompatible(existing, sess) {
				out = existing
				return nil
			}
			return alreadyExists("work_session", sess.WorkSessionID)
		} else if !os.IsNotExist(err) {
			return err
		}
		// Referential checks (missing targets fail closed).
		if sess.ProjectID != "" {
			if _, err := s.lookupProjectUnlocked(sess.ProjectID); err != nil {
				if s.catalog == nil {
					return invalidRef("project_id", sess.ProjectID, "project_id not found")
				}
				// Catalog already validated the pin; allow if pin fields set.
				if sess.ProjectRevision == "" {
					return invalidRef("project_id", sess.ProjectID, "project_id not found")
				}
			}
		}
		if sess.WorkProfileID != "" {
			if _, err := readJSON[WorkProfile](s.workProfilePath(sess.WorkProfileID)); err != nil {
				return invalidRef("work_profile_id", sess.WorkProfileID, "work_profile_id not found")
			}
		}
		for _, id := range sess.AgentProfileIDs {
			if _, err := readJSON[AgentProfile](s.agentProfilePath(id)); err != nil {
				return invalidRef("agent_profile_id", id, "agent_profile_id not found")
			}
		}
		if err := atomicWriteJSON(path, sess); err != nil {
			return err
		}
		out = sess
		return nil
	})
	return out, err
}

func sessionsCompatible(existing, want WorkSession) bool {
	if existing.WorkSessionID != want.WorkSessionID {
		return false
	}
	if want.ProjectID != "" && existing.ProjectID != want.ProjectID {
		return false
	}
	if want.WorkProfileID != "" && existing.WorkProfileID != want.WorkProfileID {
		return false
	}
	if want.ProjectRevision != "" && existing.ProjectRevision != want.ProjectRevision {
		return false
	}
	if want.ProjectSnapshotID != "" && existing.ProjectSnapshotID != want.ProjectSnapshotID {
		return false
	}
	return true
}

// resolveSessionPin validates/fills project_revision and project_snapshot_id
// for project-bound sessions against the real revision archive when available.
func (s *Store) resolveSessionPin(ctx context.Context, sess *WorkSession) error {
	if sess.ProjectID == "" {
		if sess.ProjectRevision != "" || sess.ProjectSnapshotID != "" {
			return invalidInput("project_revision/project_snapshot_id require project_id")
		}
		return nil
	}
	if s.catalog == nil {
		// Without a catalog, still require consistency of provided pin fields.
		if sess.ProjectRevision != "" && sess.ProjectSnapshotID != "" {
			expected := SnapshotIDForRevision(sess.ProjectID, sess.ProjectRevision)
			if sess.ProjectSnapshotID != expected && sess.ProjectSnapshotID != sess.ProjectRevision {
				return invalidRef("project_snapshot_id", sess.ProjectSnapshotID, "project_snapshot_id does not match project_id+project_revision")
			}
		}
		return nil
	}
	pid := project.ProjectID(sess.ProjectID)
	var rev project.Revision
	if sess.ProjectRevision != "" {
		parsed, err := project.ParseRevision(project.Revision(sess.ProjectRevision))
		if err != nil {
			return invalidRef("project_revision", sess.ProjectRevision, "invalid project_revision")
		}
		revSnap, err := s.catalog.GetRevision(ctx, pid, parsed)
		if err != nil {
			return invalidRef("project_revision", sess.ProjectRevision, "project revision archive not found")
		}
		rev = revSnap.Revision
	} else {
		live, err := s.catalog.Get(ctx, pid)
		if err != nil {
			return invalidRef("project_id", sess.ProjectID, "project_id not found")
		}
		rev = live.Revision
	}
	sess.ProjectRevision = string(rev)
	expectedSnap := SnapshotIDForRevision(sess.ProjectID, sess.ProjectRevision)
	if sess.ProjectSnapshotID == "" {
		sess.ProjectSnapshotID = expectedSnap
	} else if sess.ProjectSnapshotID != expectedSnap && sess.ProjectSnapshotID != sess.ProjectRevision {
		return invalidRef("project_snapshot_id", sess.ProjectSnapshotID, "project_snapshot_id does not match project revision")
	} else {
		// Normalize to canonical form.
		sess.ProjectSnapshotID = expectedSnap
	}
	return nil
}

func (s *Store) validateSessionProfile(ctx context.Context, sess WorkSession) error {
	if sess.WorkProfileID == "" {
		return nil
	}
	p, err := s.GetWorkProfile(ctx, sess.WorkProfileID)
	if err != nil {
		return invalidRef("work_profile_id", sess.WorkProfileID, "work_profile_id not found")
	}
	if len(p.ProjectIDs) == 0 {
		return nil // globally applicable
	}
	if sess.ProjectID == "" {
		return invalidRef("work_profile_id", sess.WorkProfileID, "work profile requires a project_id")
	}
	for _, id := range p.ProjectIDs {
		if id == sess.ProjectID {
			return nil
		}
	}
	return invalidRef("work_profile_id", sess.WorkProfileID, "work profile is not associated with project")
}

// validateSessionPolicy ensures WorkSession policy only narrows profile/project policy.
// Empty session policy is always allowed. Non-empty keys present in parent policies
// must equal the parent value (no broadening). Keys absent from parents are allowed
// as further restrictions only when parent has no conflicting key.
func (s *Store) validateSessionPolicy(sess WorkSession) error {
	if len(sess.Policy) == 0 {
		return nil
	}
	var parents []map[string]any
	if sess.WorkProfileID != "" {
		if p, err := readJSON[WorkProfile](s.workProfilePath(sess.WorkProfileID)); err == nil && len(p.DefaultPolicy) > 0 {
			parents = append(parents, p.DefaultPolicy)
		}
	}
	if sess.ProjectID != "" {
		if p, err := s.lookupProjectUnlocked(sess.ProjectID); err == nil && len(p.Policy) > 0 {
			parents = append(parents, p.Policy)
		}
	}
	for _, parent := range parents {
		if err := policyNarrows(parent, sess.Policy); err != nil {
			return err
		}
	}
	return nil
}

func policyNarrows(parent, child map[string]any) error {
	for k, pv := range parent {
		cv, ok := child[k]
		if !ok {
			// Omitting a parent key is narrowing (not asserting the permission).
			continue
		}
		// Boolean: child may only be false when parent is true (narrow), equal ok.
		if pb, ok := pv.(bool); ok {
			cb, ok := cv.(bool)
			if !ok {
				return &Error{Code: CodePolicyBroadening, Message: "policy type mismatch for key " + k, EntityKind: "policy", EntityID: k}
			}
			if cb && !pb {
				return &Error{Code: CodePolicyBroadening, Message: "policy broadens parent at key " + k, EntityKind: "policy", EntityID: k}
			}
			continue
		}
		// Other scalars must match exactly (cannot broaden by changing).
		pb, _ := json.Marshal(pv)
		cb, _ := json.Marshal(cv)
		if string(pb) != string(cb) {
			return &Error{Code: CodePolicyBroadening, Message: "policy differs from parent at key " + k, EntityKind: "policy", EntityID: k}
		}
	}
	return nil
}

// GetWorkSession loads a work session by id.
func (s *Store) GetWorkSession(ctx context.Context, id string) (WorkSession, error) {
	if err := ctx.Err(); err != nil {
		return WorkSession{}, err
	}
	if err := validateID("work_session_id", id); err != nil {
		return WorkSession{}, invalidInput(err.Error())
	}
	sess, err := readJSON[WorkSession](s.workSessionPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return WorkSession{}, notFound("work_session", id)
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
				return notFound("work_session", id)
			}
			return err
		}
		if !CanTransition(sess.State, toState) {
			return &Error{
				Code:          CodeInvalidTransition,
				Message:       fmt.Sprintf("cannot transition work session from %q to %q", sess.State, toState),
				EntityKind:    "work_session",
				EntityID:      id,
				WorkSessionID: id,
				Expected:      toState,
				Current:       sess.State,
			}
		}
		sess.State = toState
		sess.UpdatedAt = time.Now().UTC()
		if toState == StateClosed || toState == StateAborted {
			now := sess.UpdatedAt
			sess.ClosedAt = &now
		}
		if err := sess.Validate(); err != nil {
			return invalidInput(err.Error())
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
				return notFound("work_session", id)
			}
			return err
		}
		if sess.State == StateClosed || sess.State == StateAborted {
			return &Error{
				Code:          CodeInvalidTransition,
				Message:       fmt.Sprintf("cannot patch work session in terminal state %q", sess.State),
				EntityKind:    "work_session",
				EntityID:      id,
				WorkSessionID: id,
				Current:       sess.State,
			}
		}
		if displayName != nil {
			sess.DisplayName = *displayName
		}
		if agentProfileIDs != nil {
			for _, ap := range *agentProfileIDs {
				if _, err := readJSON[AgentProfile](s.agentProfilePath(ap)); err != nil {
					return invalidRef("agent_profile_id", ap, "agent_profile_id not found")
				}
			}
			sess.AgentProfileIDs = append([]string(nil), (*agentProfileIDs)...)
		}
		if policy != nil {
			sess.Policy = policy
			if err := s.validateSessionPolicy(sess); err != nil {
				return err
			}
		}
		sess.UpdatedAt = time.Now().UTC()
		if err := sess.Validate(); err != nil {
			return invalidInput(err.Error())
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
		return invalidInput(err.Error())
	}
	return s.withLock(ctx, func() error {
		if err := os.Remove(s.workSessionPath(id)); err != nil {
			if os.IsNotExist(err) {
				return notFound("work_session", id)
			}
			return err
		}
		return nil
	})
}

// AssertProjectDeletable returns an error when a retained WorkSession still
// references the project. Call before catalog Project delete.
func (s *Store) AssertProjectDeletable(ctx context.Context, projectID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateID("project_id", projectID); err != nil {
		return invalidInput(err.Error())
	}
	var ref string
	err := s.withLock(ctx, func() error {
		ref = s.firstSessionReferencingProjectUnlocked(projectID)
		return nil
	})
	if err != nil {
		return err
	}
	if ref != "" {
		return &Error{
			Code:          CodeReferenced,
			Message:       "project is referenced by a retained work session",
			EntityKind:    "project",
			EntityID:      projectID,
			ProjectID:     projectID,
			WorkSessionID: ref,
		}
	}
	return nil
}
