package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/awm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testWebWithAWM(t *testing.T, store *awm.Store) *WebServer {
	t.Helper()
	services := &mcp.Services{Config: newMockConfigService(map[string]*mcp.IntegrationConfig{}), Registry: newMockRegistry()}
	return New(services, 0, nil, nil, WithAWMStore(store))
}

func seedWorkModel(t *testing.T, s *awm.Store) {
	t.Helper()
	ctx := context.Background()
	_, err := s.PutProject(ctx, awm.Project{Version: "1", ProjectID: "switchboard", Description: "sb"})
	require.NoError(t, err)
	_, err = s.PutWorkProfile(ctx, awm.WorkProfile{
		Version: "1", WorkProfileID: "code-review", DisplayName: "Code review",
		Description: "Review PRs", ProjectIDs: []string{"switchboard"},
	})
	require.NoError(t, err)
	_, err = s.PutAgentProfile(ctx, awm.AgentProfile{
		Version: "1", AgentProfileID: "reviewer", DisplayName: "Reviewer",
		Capabilities: []string{"read-repo", "comment"},
	})
	require.NoError(t, err)
	_, err = s.CreateWorkSession(ctx, awm.WorkSession{
		Version: "1", WorkSessionID: "ws-1", DisplayName: "PR sweep",
		ProjectID: "switchboard", WorkProfileID: "code-review",
		AgentProfileIDs: []string{"reviewer"}, State: awm.StateOpen,
	})
	require.NoError(t, err)
}

func TestWorkHub_RendersLists(t *testing.T) {
	s := awm.NewStore(t.TempDir())
	seedWorkModel(t, s)
	ws := testWebWithAWM(t, s)
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/work", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, "Code review")
	assert.Contains(t, body, "Reviewer")
	assert.Contains(t, body, "PR sweep")
	assert.Contains(t, body, "code-review")
}

func TestWorkProfilesList_AndDetail(t *testing.T) {
	s := awm.NewStore(t.TempDir())
	seedWorkModel(t, s)
	ws := testWebWithAWM(t, s)

	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/work/profiles", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Code review")

	rr = httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/work/profiles/code-review", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, "Review PRs")
	assert.Contains(t, body, "work_profile_id")
}

func TestAgentProfilesList_AndDetail(t *testing.T) {
	s := awm.NewStore(t.TempDir())
	seedWorkModel(t, s)
	ws := testWebWithAWM(t, s)

	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/work/agents", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Reviewer")

	rr = httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/work/agents/reviewer", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, "read-repo")
	assert.Contains(t, body, "agent_profile_id")
}

func TestWorkSessionsList_FilterAndDetail(t *testing.T) {
	s := awm.NewStore(t.TempDir())
	seedWorkModel(t, s)
	ws := testWebWithAWM(t, s)

	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/work/sessions?state=open&project_id=switchboard", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, "PR sweep")
	assert.Contains(t, body, "ws-1")

	rr = httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/work/sessions?state=closed", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.NotContains(t, rr.Body.String(), "PR sweep")

	rr = httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/work/sessions/ws-1", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	body = rr.Body.String()
	assert.Contains(t, body, "PR sweep")
	assert.Contains(t, body, "work_session_id")
	assert.Contains(t, body, "open")
}

func TestWorkProfileDetail_NotFound(t *testing.T) {
	s := awm.NewStore(t.TempDir())
	ws := testWebWithAWM(t, s)
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/work/profiles/missing", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "not found")
}

func TestWorkHub_NilStore(t *testing.T) {
	services := &mcp.Services{Config: newMockConfigService(map[string]*mcp.IntegrationConfig{}), Registry: newMockRegistry()}
	ws := New(services, 0, nil, nil)
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/work", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Work")
}
