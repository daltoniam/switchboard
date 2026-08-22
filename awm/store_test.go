package awm

import (
	"context"
	"encoding/json"
	"github.com/daltoniam/switchboard/project"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkProfileCRUD(t *testing.T) {
	s := NewStore(t.TempDir())
	ctx := context.Background()
	_, err := s.PutProject(ctx, Project{Version: "1", ProjectID: "switchboard"})
	require.NoError(t, err)
	p, err := s.PutWorkProfile(ctx, WorkProfile{
		Version: "1", WorkProfileID: "code-review", DisplayName: "Code review",
		ProjectIDs: []string{"switchboard"},
	})
	require.NoError(t, err)
	assert.Equal(t, "code-review", p.WorkProfileID)

	got, err := s.GetWorkProfile(ctx, "code-review")
	require.NoError(t, err)
	assert.Equal(t, "Code review", got.DisplayName)

	list, err := s.ListWorkProfiles(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)

	require.NoError(t, s.DeleteWorkProfile(ctx, "code-review"))
	_, err = s.GetWorkProfile(ctx, "code-review")
	assert.Error(t, err)
}

func TestAgentProfileCRUD(t *testing.T) {
	s := NewStore(t.TempDir())
	ctx := context.Background()
	_, err := s.PutAgentProfile(ctx, AgentProfile{
		Version: "1", AgentProfileID: "reviewer", DisplayName: "Reviewer",
		Capabilities: []string{"read-repo", "comment"},
	})
	require.NoError(t, err)
	got, err := s.GetAgentProfile(ctx, "reviewer")
	require.NoError(t, err)
	assert.Equal(t, []string{"read-repo", "comment"}, got.Capabilities)
	require.NoError(t, s.DeleteAgentProfile(ctx, "reviewer"))
}

func TestWorkSessionLifecycle(t *testing.T) {
	s := NewStore(t.TempDir())
	ctx := context.Background()
	_, err := s.PutWorkProfile(ctx, WorkProfile{Version: "1", WorkProfileID: "incident"})
	require.NoError(t, err)
	_, err = s.PutAgentProfile(ctx, AgentProfile{Version: "1", AgentProfileID: "triage"})
	require.NoError(t, err)

	_, err = s.PutProject(ctx, Project{Version: "1", ProjectID: "switchboard", Description: "sb"})
	require.NoError(t, err)
	sess, err := s.CreateWorkSession(ctx, WorkSession{
		Version: "1", WorkSessionID: "ws-1", DisplayName: "Outage",
		ProjectID:     "switchboard",
		WorkProfileID: "incident", AgentProfileIDs: []string{"triage"},
		State: StateProposed,
	})
	require.NoError(t, err)
	assert.Equal(t, StateProposed, sess.State)

	open, err := s.TransitionWorkSession(ctx, "ws-1", StateOpen)
	require.NoError(t, err)
	assert.Equal(t, StateOpen, open.State)

	_, err = s.TransitionWorkSession(ctx, "ws-1", StateProposed)
	assert.Error(t, err)
	assert.True(t, IsCode(err, CodeInvalidTransition))

	closed, err := s.TransitionWorkSession(ctx, "ws-1", StateClosed)
	require.NoError(t, err)
	assert.Equal(t, StateClosed, closed.State)
	require.NotNil(t, closed.ClosedAt)

	list, err := s.ListWorkSessions(ctx, StateClosed, "switchboard")
	require.NoError(t, err)
	require.Len(t, list, 1)
}

func TestWorkSession_RejectsMissingProfile(t *testing.T) {
	s := NewStore(t.TempDir())
	_, err := s.CreateWorkSession(context.Background(), WorkSession{
		Version: "1", WorkSessionID: "ws-x", WorkProfileID: "missing", State: StateOpen,
	})
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeInvalidReference) || strings.Contains(err.Error(), "not found"))
}

func TestEnsureDefaultWorkProfile_Idempotent(t *testing.T) {
	s := NewStore(t.TempDir())
	ctx := context.Background()
	p1, err := s.EnsureDefaultWorkProfile(ctx)
	require.NoError(t, err)
	assert.Equal(t, DefaultWorkProfileID, p1.WorkProfileID)
	assert.Equal(t, "default", p1.DisplayName)

	// Customize display name; reseeding must not overwrite.
	p1.DisplayName = "Default (custom)"
	_, err = s.PutWorkProfile(ctx, p1)
	require.NoError(t, err)

	p2, err := s.EnsureDefaultWorkProfile(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Default (custom)", p2.DisplayName)
}

func TestWorkSession_RejectsIneligibleProfile(t *testing.T) {
	s := NewStore(t.TempDir())
	ctx := context.Background()
	_, err := s.PutProject(ctx, Project{Version: "1", ProjectID: "alpha"})
	require.NoError(t, err)
	_, err = s.PutProject(ctx, Project{Version: "1", ProjectID: "beta"})
	require.NoError(t, err)
	_, err = s.PutWorkProfile(ctx, WorkProfile{
		Version: "1", WorkProfileID: "alpha-only", ProjectIDs: []string{"alpha"},
	})
	require.NoError(t, err)
	_, err = s.CreateWorkSession(ctx, WorkSession{
		Version: "1", WorkSessionID: "ws-bad", ProjectID: "beta", WorkProfileID: "alpha-only", State: StateProposed,
	})
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeInvalidReference))
}

func TestDeleteWorkProfile_Referenced(t *testing.T) {
	s := NewStore(t.TempDir())
	ctx := context.Background()
	_, err := s.PutWorkProfile(ctx, WorkProfile{Version: "1", WorkProfileID: "wp"})
	require.NoError(t, err)
	_, err = s.PutProject(ctx, Project{Version: "1", ProjectID: "p"})
	require.NoError(t, err)
	_, err = s.CreateWorkSession(ctx, WorkSession{
		Version: "1", WorkSessionID: "ws", ProjectID: "p", WorkProfileID: "wp", State: StateOpen,
	})
	require.NoError(t, err)
	err = s.DeleteWorkProfile(ctx, "wp")
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeReferenced))
}

func TestDeleteProject_Referenced(t *testing.T) {
	s := NewStore(t.TempDir())
	ctx := context.Background()
	_, err := s.PutWorkProfile(ctx, WorkProfile{Version: "1", WorkProfileID: "wp"})
	require.NoError(t, err)
	_, err = s.PutProject(ctx, Project{Version: "1", ProjectID: "p"})
	require.NoError(t, err)
	_, err = s.CreateWorkSession(ctx, WorkSession{
		Version: "1", WorkSessionID: "ws", ProjectID: "p", WorkProfileID: "wp", State: StateOpen,
	})
	require.NoError(t, err)
	err = s.DeleteProject(ctx, "p")
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeReferenced))
}

func TestWorkSession_IdempotentCreate(t *testing.T) {
	s := NewStore(t.TempDir())
	ctx := context.Background()
	_, err := s.PutWorkProfile(ctx, WorkProfile{Version: "1", WorkProfileID: "wp"})
	require.NoError(t, err)
	_, err = s.PutProject(ctx, Project{Version: "1", ProjectID: "p"})
	require.NoError(t, err)
	first, err := s.CreateWorkSession(ctx, WorkSession{
		Version: "1", WorkSessionID: "ws", ProjectID: "p", WorkProfileID: "wp", State: StateProposed,
	})
	require.NoError(t, err)
	second, err := s.CreateWorkSession(ctx, WorkSession{
		Version: "1", WorkSessionID: "ws", ProjectID: "p", WorkProfileID: "wp", State: StateProposed,
	})
	require.NoError(t, err)
	assert.Equal(t, first.WorkSessionID, second.WorkSessionID)
}

func TestPolicyNarrowing(t *testing.T) {
	s := NewStore(t.TempDir())
	ctx := context.Background()
	_, err := s.PutWorkProfile(ctx, WorkProfile{
		Version: "1", WorkProfileID: "wp",
		DefaultPolicy: PolicyDocument{"network": false, "write": true},
	})
	require.NoError(t, err)
	_, err = s.PutProject(ctx, Project{Version: "1", ProjectID: "p"})
	require.NoError(t, err)
	// Broadening network false→true must fail.
	_, err = s.CreateWorkSession(ctx, WorkSession{
		Version: "1", WorkSessionID: "ws-bad", ProjectID: "p", WorkProfileID: "wp",
		State: StateProposed, Policy: PolicyDocument{"network": true},
	})
	require.Error(t, err)
	assert.True(t, IsCode(err, CodePolicyBroadening))
	// Narrowing write true→false is ok.
	_, err = s.CreateWorkSession(ctx, WorkSession{
		Version: "1", WorkSessionID: "ws-ok", ProjectID: "p", WorkProfileID: "wp",
		State: StateProposed, Policy: PolicyDocument{"write": false},
	})
	require.NoError(t, err)
}

func TestSnapshotIDForRevision(t *testing.T) {
	id := SnapshotIDForRevision("p", "sha256:"+strings.Repeat("a", 64))
	assert.Contains(t, id, "project://registry/projects/p/revisions/sha256:")
}

func TestStore_LivesUnderSwitchboardRoot(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)
	assert.Equal(t, root, s.ConfigRoot())
	assert.Equal(t, filepath.Join(root, "awm"), s.Root())
	assert.NotContains(t, s.Root(), "project-interop")
	_, err := s.PutAgentProfile(context.Background(), AgentProfile{Version: "1", AgentProfileID: "a"})
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(root, "awm", "agent_profiles", "a.json"))
	require.NoError(t, err)
}

func TestCanTransition(t *testing.T) {
	assert.True(t, CanTransition(StateProposed, StateOpen))
	assert.True(t, CanTransition(StateOpen, StateClosed))
	assert.False(t, CanTransition(StateClosed, StateOpen))
}

func TestProjectCRUD_AndLegacyFile(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)
	ctx := context.Background()

	// Legacy-shaped file under projects/
	projDir := filepath.Join(root, "projects")
	require.NoError(t, os.MkdirAll(projDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "legacy.project.json"), []byte(`{"version":"1","name":"legacy","description":"from disk"}`), 0o600))

	got, err := s.GetProject(ctx, "legacy")
	require.NoError(t, err)
	assert.Equal(t, "legacy", got.ProjectID)
	assert.Equal(t, "from disk", got.Description)

	p, err := s.PutProject(ctx, Project{Version: "1", ProjectID: "newproj", Description: "created"})
	require.NoError(t, err)
	assert.Equal(t, "newproj", p.Name)

	list, err := s.ListProjects(ctx)
	require.NoError(t, err)
	ids := map[string]bool{}
	for _, x := range list {
		ids[x.ProjectID] = true
	}
	assert.True(t, ids["legacy"])
	assert.True(t, ids["newproj"])

	// Work session requires project when project_id set
	_, err = s.CreateWorkSession(ctx, WorkSession{
		Version: "1", WorkSessionID: "ws-p", ProjectID: "missing", State: StateOpen,
	})
	require.Error(t, err)

	_, err = s.PutWorkProfile(ctx, WorkProfile{Version: "1", WorkProfileID: "wp"})
	require.NoError(t, err)
	sess, err := s.CreateWorkSession(ctx, WorkSession{
		Version: "1", WorkSessionID: "ws-p", ProjectID: "newproj", WorkProfileID: "wp", State: StateOpen,
	})
	require.NoError(t, err)
	assert.Equal(t, "newproj", sess.ProjectID)

	// Referenced project cannot be deleted until the session is removed.
	require.Error(t, s.DeleteProject(ctx, "newproj"))
	require.NoError(t, s.DeleteWorkSession(ctx, "ws-p"))
	require.NoError(t, s.DeleteProject(ctx, "newproj"))
}

func TestTransitionAndPatch_RejectInvalidID(t *testing.T) {
	s := NewStore(t.TempDir())
	ctx := context.Background()
	// Path traversal must not touch catalog files under projects/.
	bad := "../../projects/acme.project"
	_, err := s.TransitionWorkSession(ctx, bad, StateOpen)
	require.Error(t, err)
	_, err = s.PatchWorkSession(ctx, bad, nil, nil, nil)
	require.Error(t, err)
}

func TestDeleteAgentProfile_Referenced(t *testing.T) {
	s := NewStore(t.TempDir())
	ctx := context.Background()
	_, err := s.PutAgentProfile(ctx, AgentProfile{Version: "1", AgentProfileID: "ag"})
	require.NoError(t, err)
	_, err = s.PutWorkProfile(ctx, WorkProfile{Version: "1", WorkProfileID: "wp"})
	require.NoError(t, err)
	_, err = s.PutProject(ctx, Project{Version: "1", ProjectID: "p"})
	require.NoError(t, err)
	_, err = s.CreateWorkSession(ctx, WorkSession{
		Version: "1", WorkSessionID: "ws", ProjectID: "p", WorkProfileID: "wp",
		AgentProfileIDs: []string{"ag"}, State: StateOpen,
	})
	require.NoError(t, err)
	err = s.DeleteAgentProfile(ctx, "ag")
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeReferenced))
}

func TestGetAgentProfile_NotFoundTyped(t *testing.T) {
	s := NewStore(t.TempDir())
	_, err := s.GetAgentProfile(context.Background(), "missing")
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeNotFound), "%v", err)
}

func TestPutProject_PreservesAdditionalFields(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)
	ctx := context.Background()
	path := filepath.Join(root, "projects", "p.project.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(`{
		"version":"1","name":"p","description":"old",
		"resources":{"main":{"type":"repo","path":"/tmp/x"}},
		"tools":{"github":{"allow":["*"]}}
	}`), 0o600))
	_, err := s.PutProject(ctx, Project{
		Version: "1", ProjectID: "p", DisplayName: "Project P", Description: "new",
		Policy: PolicyDocument{"network": false},
	})
	require.NoError(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	assert.Equal(t, "new", doc["description"])
	assert.Equal(t, "Project P", doc["display_name"])
	assert.Equal(t, false, doc["policy"].(map[string]any)["network"])
	assert.NotNil(t, doc["resources"])
	assert.NotNil(t, doc["tools"])
}

func TestPutProject_PreservesKnownResourceIDs(t *testing.T) {
	s := NewStore(t.TempDir())
	ctx := context.Background()
	_, err := s.PutProject(ctx, Project{
		Version: "1", ProjectID: "obs", Description: "observed",
		KnownResourceIDs: []string{"repo", "worktree-root"},
	})
	require.NoError(t, err)
	got, err := s.GetProject(ctx, "obs")
	require.NoError(t, err)
	assert.Equal(t, []string{"repo", "worktree-root"}, got.KnownResourceIDs)
}

func TestPutResource_RejectsInvalidShape(t *testing.T) {
	tests := []struct {
		name     string
		resource Resource
	}{
		{"relative uri", Resource{Version: "1", ResourceID: "r", URI: "/tmp/r", Kind: "repo"}},
		{"credential uri", Resource{Version: "1", ResourceID: "r", URI: "https://user:secret@example.com/r", Kind: "repo"}},
		{"missing kind", Resource{Version: "1", ResourceID: "r", URI: "file:///tmp/r"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewStore(t.TempDir()).PutResource(context.Background(), tt.resource)
			require.Error(t, err)
			assert.True(t, IsCode(err, CodeInvalidInput), "%v", err)
		})
	}
}

func TestResourceBinding_LifecycleAndReferentialIntegrity(t *testing.T) {
	s := NewStore(t.TempDir())
	ctx := context.Background()
	_, err := s.PutResource(ctx, Resource{
		Version: "1", ResourceID: "repo.local", URI: "file:///work/repo", Kind: "git-repository",
	})
	require.NoError(t, err)
	resources, err := s.ListResources(ctx)
	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, "repo.local", resources[0].ResourceID)
	_, err = s.CreateWorkSession(ctx, WorkSession{
		Version: "1", WorkSessionID: "ws", State: StateOpen,
		Policy: PolicyDocument{"filesystem.write": false},
	})
	require.NoError(t, err)

	_, err = s.CreateResourceBinding(ctx, ResourceBinding{
		Version: "1", ResourceBindingID: "binding", WorkSessionID: "ws", ResourceID: "repo.local",
		ResolvedLocator: "file:///work/repo", State: BindingStateProposed,
		Grant: PolicyDocument{"filesystem.write": true},
	})
	require.Error(t, err)
	assert.True(t, IsCode(err, CodePolicyBroadening), "%v", err)

	created, err := s.CreateResourceBinding(ctx, ResourceBinding{
		Version: "1", ResourceBindingID: "binding", WorkSessionID: "ws", ResourceID: "repo.local",
		ResolvedLocator: "file:///work/repo", State: BindingStateProposed,
		Grant: PolicyDocument{"filesystem.write": false},
	})
	require.NoError(t, err)
	assert.Equal(t, BindingStateProposed, created.State)
	idempotent, err := s.CreateResourceBinding(ctx, created)
	require.NoError(t, err)
	assert.Equal(t, created.ResourceBindingID, idempotent.ResourceBindingID)
	conflicting := created
	conflicting.Grant = PolicyDocument{"filesystem.write": true}
	_, err = s.CreateResourceBinding(ctx, conflicting)
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeAlreadyExists), "%v", err)

	listed, err := s.ListResourceBindings(ctx, BindingStateProposed, "ws", "repo.local")
	require.NoError(t, err)
	require.Len(t, listed, 1)
	got, err := s.GetResourceBinding(ctx, "binding")
	require.NoError(t, err)
	assert.Equal(t, "file:///work/repo", got.ResolvedLocator)

	bound, err := s.TransitionResourceBinding(ctx, "binding", BindingStateBound)
	require.NoError(t, err)
	assert.Equal(t, BindingStateBound, bound.State)

	locator := "file:///work/repo/worktree"
	patched, err := s.PatchResourceBinding(ctx, "binding", &locator, PolicyDocument{"filesystem.write": false})
	require.NoError(t, err)
	assert.Equal(t, locator, patched.ResolvedLocator)

	err = s.DeleteResource(ctx, "repo.local")
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeReferenced), "%v", err)
	err = s.DeleteWorkSession(ctx, "ws")
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeReferenced), "%v", err)

	revoked, err := s.TransitionResourceBinding(ctx, "binding", BindingStateRevoked)
	require.NoError(t, err)
	assert.Equal(t, BindingStateRevoked, revoked.State)
	require.NoError(t, s.DeleteResourceBinding(ctx, "binding"))
	require.NoError(t, s.DeleteResource(ctx, "repo.local"))
	require.NoError(t, s.DeleteWorkSession(ctx, "ws"))
}

func TestCreateResourceBinding_RejectsInvalidReferences(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*testing.T, *Store)
		binding ResourceBinding
		code    string
	}{
		{
			name: "missing session",
			prepare: func(t *testing.T, s *Store) {
				_, err := s.PutResource(context.Background(), Resource{Version: "1", ResourceID: "r", URI: "file:///r", Kind: "repo"})
				require.NoError(t, err)
			},
			binding: ResourceBinding{Version: "1", ResourceBindingID: "b", WorkSessionID: "missing", ResourceID: "r", State: BindingStateProposed},
			code:    CodeInvalidReference,
		},
		{
			name: "missing resource",
			prepare: func(t *testing.T, s *Store) {
				_, err := s.CreateWorkSession(context.Background(), WorkSession{Version: "1", WorkSessionID: "ws", State: StateOpen})
				require.NoError(t, err)
			},
			binding: ResourceBinding{Version: "1", ResourceBindingID: "b", WorkSessionID: "ws", ResourceID: "missing", State: BindingStateProposed},
			code:    CodeInvalidReference,
		},
		{
			name: "bound without locator",
			prepare: func(t *testing.T, s *Store) {
				_, err := s.PutResource(context.Background(), Resource{Version: "1", ResourceID: "r", URI: "file:///r", Kind: "repo"})
				require.NoError(t, err)
				_, err = s.CreateWorkSession(context.Background(), WorkSession{Version: "1", WorkSessionID: "ws", State: StateOpen})
				require.NoError(t, err)
			},
			binding: ResourceBinding{Version: "1", ResourceBindingID: "b", WorkSessionID: "ws", ResourceID: "r", State: BindingStateBound},
			code:    CodeInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStore(t.TempDir())
			tt.prepare(t, s)
			_, err := s.CreateResourceBinding(context.Background(), tt.binding)
			require.Error(t, err)
			assert.True(t, IsCode(err, tt.code), "%v", err)
		})
	}
}

func TestCanTransitionResourceBinding(t *testing.T) {
	tests := []struct {
		from, to string
		want     bool
	}{
		{BindingStateProposed, BindingStateBound, true},
		{BindingStateProposed, BindingStateRevoked, true},
		{BindingStateBound, BindingStateRevoked, true},
		{BindingStateBound, BindingStateProposed, false},
		{BindingStateRevoked, BindingStateBound, false},
	}
	for _, tt := range tests {
		t.Run(tt.from+"_to_"+tt.to, func(t *testing.T) {
			assert.Equal(t, tt.want, CanTransitionResourceBinding(tt.from, tt.to))
		})
	}
}

func TestCreateWorkSession_RejectsDeletedProjectEvenWithRevisionPin(t *testing.T) {
	root := t.TempDir()
	cat := project.NewStore(root)
	require.NoError(t, cat.Load())
	ctx := context.Background()
	snap, err := cat.Create(ctx, project.CreateRequest{Definition: project.Definition{Version: "1", Name: "gone"}})
	require.NoError(t, err)
	s := NewStore(root)
	s.SetCatalog(cat)
	_, err = s.PutWorkProfile(ctx, WorkProfile{Version: "1", WorkProfileID: "wp"})
	require.NoError(t, err)
	// Delete project (no sessions yet).
	require.NoError(t, cat.Delete(ctx, project.DeleteRequest{
		ProjectID: "gone", ExpectedSourceRevision: snap.SourceRevision,
	}))
	// Revision archive may still exist; new sessions must still fail.
	_, err = s.CreateWorkSession(ctx, WorkSession{
		Version: "1", WorkSessionID: "ws-orphan",
		ProjectID: "gone", ProjectRevision: string(snap.Revision),
		WorkProfileID: "wp", State: StateOpen,
	})
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeInvalidReference) || IsCode(err, CodeNotFound), "%v", err)
}

func TestCreateWorkSession_PinReadFailureIsNotMissingProject(t *testing.T) {
	root := t.TempDir()
	cat := project.NewStore(root)
	require.NoError(t, cat.Load())
	ctx := context.Background()
	snap, err := cat.Create(ctx, project.CreateRequest{Definition: project.Definition{Version: "1", Name: "p"}})
	require.NoError(t, err)

	path := filepath.Join(root, "revisions", "p", snap.Revision.DigestHex()+".json")
	require.FileExists(t, path)
	require.NoError(t, os.Chmod(path, 0o000))
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	s := NewStore(root)
	s.SetCatalog(cat)
	_, err = s.PutWorkProfile(ctx, WorkProfile{Version: "1", WorkProfileID: "wp"})
	require.NoError(t, err)

	_, err = s.CreateWorkSession(ctx, WorkSession{
		Version: "1", WorkSessionID: "ws-io",
		ProjectID: "p", ProjectRevision: string(snap.Revision),
		WorkProfileID: "wp", State: StateOpen,
	})
	require.Error(t, err)
	// Must not collapse archive I/O into invalid_reference / "not found".
	assert.False(t, IsCode(err, CodeInvalidReference), "%v", err)
	assert.False(t, IsCode(err, CodeNotFound), "%v", err)
	assert.True(t, project.IsCode(err, project.CodeInternalError), "%v", err)
}

func TestPutWorkProfile_CatalogGetFailureIsNotMissingProject(t *testing.T) {
	s := NewStore(t.TempDir())
	s.SetCatalog(errCatalog{err: &project.Error{Code: project.CodeInternalError, Message: "catalog get failed"}})
	_, err := s.PutWorkProfile(context.Background(), WorkProfile{
		Version: "1", WorkProfileID: "wp", ProjectIDs: []string{"p"},
	})
	require.Error(t, err)
	assert.False(t, IsCode(err, CodeInvalidReference), "%v", err)
	assert.False(t, IsCode(err, CodeNotFound), "%v", err)
	assert.True(t, project.IsCode(err, project.CodeInternalError), "%v", err)
}

type errCatalog struct {
	err error
}

func (e errCatalog) Get(context.Context, project.ProjectID) (project.Snapshot, error) {
	return project.Snapshot{}, e.err
}

func (e errCatalog) GetRevision(context.Context, project.ProjectID, project.Revision) (project.RevisionSnapshot, error) {
	return project.RevisionSnapshot{}, e.err
}

func TestCreateWorkSession_LiveProjectReadFailureIsNotMissing(t *testing.T) {
	s := NewStore(t.TempDir())
	ctx := context.Background()
	_, err := s.PutWorkProfile(ctx, WorkProfile{Version: "1", WorkProfileID: "wp"})
	require.NoError(t, err)
	_, err = s.PutProject(ctx, Project{Version: "1", ProjectID: "p"})
	require.NoError(t, err)

	path := s.projectPath("p")
	require.FileExists(t, path)
	require.NoError(t, os.Chmod(path, 0o000))
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	_, err = s.CreateWorkSession(ctx, WorkSession{
		Version: "1", WorkSessionID: "ws-io-live",
		ProjectID: "p", WorkProfileID: "wp", State: StateOpen,
	})
	require.Error(t, err)
	assert.False(t, IsCode(err, CodeInvalidReference), "%v", err)
	assert.False(t, IsCode(err, CodeNotFound), "%v", err)
}

func TestCreateWorkSession_ProfileReadFailureIsNotMissing(t *testing.T) {
	s := NewStore(t.TempDir())
	ctx := context.Background()
	_, err := s.PutWorkProfile(ctx, WorkProfile{Version: "1", WorkProfileID: "wp"})
	require.NoError(t, err)
	_, err = s.PutProject(ctx, Project{Version: "1", ProjectID: "p"})
	require.NoError(t, err)

	path := s.workProfilePath("wp")
	require.FileExists(t, path)
	require.NoError(t, os.Chmod(path, 0o000))
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	_, err = s.CreateWorkSession(ctx, WorkSession{
		Version: "1", WorkSessionID: "ws-io-profile",
		ProjectID: "p", WorkProfileID: "wp", State: StateOpen,
	})
	require.Error(t, err)
	assert.False(t, IsCode(err, CodeInvalidReference), "%v", err)
	assert.False(t, IsCode(err, CodeNotFound), "%v", err)
}

func TestCreateWorkSession_PolicyMismatchConflicts(t *testing.T) {
	s := NewStore(t.TempDir())
	ctx := context.Background()
	_, err := s.PutWorkProfile(ctx, WorkProfile{Version: "1", WorkProfileID: "wp"})
	require.NoError(t, err)
	_, err = s.PutProject(ctx, Project{Version: "1", ProjectID: "p"})
	require.NoError(t, err)
	_, err = s.CreateWorkSession(ctx, WorkSession{
		Version: "1", WorkSessionID: "ws", ProjectID: "p", WorkProfileID: "wp",
		State: StateProposed, Policy: PolicyDocument{"write": false},
	})
	require.NoError(t, err)

	_, err = s.CreateWorkSession(ctx, WorkSession{
		Version: "1", WorkSessionID: "ws", ProjectID: "p", WorkProfileID: "wp",
		State: StateProposed, Policy: PolicyDocument{"network": false},
	})
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeAlreadyExists), "%v", err)
}
