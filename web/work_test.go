package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/awm"
	"github.com/daltoniam/switchboard/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testWebWithStores(t *testing.T, catalog *project.Store, work *awm.Store) *WebServer {
	t.Helper()
	services := &mcp.Services{Config: newMockConfigService(map[string]*mcp.IntegrationConfig{}), Registry: newMockRegistry()}
	opts := []Option{}
	if catalog != nil {
		opts = append(opts, WithProjectCatalog(catalog))
	}
	if work != nil {
		opts = append(opts, WithAWMStore(work))
	}
	return New(services, 0, nil, nil, opts...)
}

func seedProjectWork(t *testing.T, catalog *project.Store, work *awm.Store) {
	t.Helper()
	ctx := context.Background()
	_, err := catalog.Create(ctx, project.CreateRequest{
		Definition: project.Definition{Version: "1", Name: "switchboard", Description: "sb"},
	})
	require.NoError(t, err)
	_, err = work.PutProject(ctx, awm.Project{Version: "1", ProjectID: "switchboard", Description: "sb"})
	require.NoError(t, err)
	_, err = work.PutWorkProfile(ctx, awm.WorkProfile{
		Version: "1", WorkProfileID: "switchboard.default", DisplayName: "default",
		Description: "Review PRs", ProjectIDs: []string{"switchboard"},
	})
	require.NoError(t, err)
	_, err = work.PutWorkProfile(ctx, awm.WorkProfile{
		Version: "1", WorkProfileID: "other.default", DisplayName: "other",
		ProjectIDs: []string{"other"},
	})
	require.NoError(t, err)
	_, err = work.PutAgentProfile(ctx, awm.AgentProfile{
		Version: "1", AgentProfileID: "switchboard.reviewer", DisplayName: "Reviewer",
		Capabilities: []string{"read-repo", "comment"},
	})
	require.NoError(t, err)
	_, err = work.PutAgentProfile(ctx, awm.AgentProfile{
		Version: "1", AgentProfileID: "unrelated", DisplayName: "Unrelated",
	})
	require.NoError(t, err)
	_, err = work.CreateWorkSession(ctx, awm.WorkSession{
		Version: "1", WorkSessionID: "ws-1", DisplayName: "PR sweep",
		ProjectID: "switchboard", WorkProfileID: "switchboard.default",
		AgentProfileIDs: []string{"switchboard.reviewer"}, State: awm.StateOpen,
	})
	require.NoError(t, err)
}

func TestProjectDetail_ShowsWorkLinks(t *testing.T) {
	catalog := project.NewStore(t.TempDir())
	require.NoError(t, catalog.Load())
	work := awm.NewStore(catalog.ConfigDir())
	seedProjectWork(t, catalog, work)
	ws := testWebWithStores(t, catalog, work)

	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/projects/switchboard", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, "/projects/switchboard/work")
	assert.Contains(t, body, "Work profiles")
	assert.Contains(t, body, "Work sessions")
}

func TestProjectWorkHub_FiltersByProject(t *testing.T) {
	catalog := project.NewStore(t.TempDir())
	require.NoError(t, catalog.Load())
	work := awm.NewStore(catalog.ConfigDir())
	seedProjectWork(t, catalog, work)
	ws := testWebWithStores(t, catalog, work)

	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/projects/switchboard/work", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, "switchboard.default")
	assert.Contains(t, body, "PR sweep")
	assert.Contains(t, body, "Reviewer")
	assert.NotContains(t, body, "other.default")
	assert.NotContains(t, body, "Unrelated")
}

func TestProjectWorkProfiles_AndDetail(t *testing.T) {
	catalog := project.NewStore(t.TempDir())
	require.NoError(t, catalog.Load())
	work := awm.NewStore(catalog.ConfigDir())
	seedProjectWork(t, catalog, work)
	ws := testWebWithStores(t, catalog, work)

	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/projects/switchboard/work/profiles", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "switchboard.default")
	assert.NotContains(t, rr.Body.String(), "other.default")

	rr = httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/projects/switchboard/work/profiles/switchboard.default", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Review PRs")

	rr = httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/projects/switchboard/work/profiles/other.default", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "not found")
}

func TestProjectAgentProfiles_AndDetail(t *testing.T) {
	catalog := project.NewStore(t.TempDir())
	require.NoError(t, catalog.Load())
	work := awm.NewStore(catalog.ConfigDir())
	seedProjectWork(t, catalog, work)
	ws := testWebWithStores(t, catalog, work)

	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/projects/switchboard/work/agents", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, "Reviewer")
	assert.NotContains(t, body, "Unrelated")

	rr = httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/projects/switchboard/work/agents/switchboard.reviewer", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "read-repo")
}

func TestProjectWorkSessions_FilterAndDetail(t *testing.T) {
	catalog := project.NewStore(t.TempDir())
	require.NoError(t, catalog.Load())
	work := awm.NewStore(catalog.ConfigDir())
	seedProjectWork(t, catalog, work)
	ws := testWebWithStores(t, catalog, work)

	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/projects/switchboard/work/sessions?state=open", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "PR sweep")

	rr = httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/projects/switchboard/work/sessions?state=closed", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.NotContains(t, rr.Body.String(), "PR sweep")

	rr = httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/projects/switchboard/work/sessions/ws-1", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "work_session_id")
}

func TestProjectWorkHub_NilStore(t *testing.T) {
	services := &mcp.Services{Config: newMockConfigService(map[string]*mcp.IntegrationConfig{}), Registry: newMockRegistry()}
	ws := New(services, 0, nil, nil)
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/projects/switchboard/work", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Work")
}
