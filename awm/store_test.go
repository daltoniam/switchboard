package awm

import (
	"context"
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
		ProjectID: "switchboard", ProjectRevision: "sha256:abc",
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
	assert.True(t, strings.Contains(err.Error(), "not found"))
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

	require.NoError(t, s.DeleteProject(ctx, "newproj"))
}
