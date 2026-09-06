package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testWeb(t *testing.T, store *project.Store) *WebServer {
	t.Helper()
	services := &mcp.Services{Config: newMockConfigService(map[string]*mcp.IntegrationConfig{}), Registry: newMockRegistry()}
	return New(services, 0, nil, nil, WithProjectCatalog(store))
}

func TestProjectsList_RendersCatalog(t *testing.T) {
	store := project.NewStore(t.TempDir())
	require.NoError(t, store.Load())
	_, err := store.Create(context.Background(), project.CreateRequest{
		Definition: project.Definition{Version: "1", Name: "browse-me", Description: "hi"},
	})
	require.NoError(t, err)
	ws := testWeb(t, store)
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/projects", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, "browse-me")
	assert.Contains(t, body, "hi")
}

func TestProjectsList_SearchQuery(t *testing.T) {
	store := project.NewStore(t.TempDir())
	require.NoError(t, store.Load())
	_, err := store.Create(context.Background(), project.CreateRequest{Definition: project.Definition{Version: "1", Name: "alpha"}})
	require.NoError(t, err)
	_, err = store.Create(context.Background(), project.CreateRequest{Definition: project.Definition{Version: "1", Name: "beta"}})
	require.NoError(t, err)
	ws := testWeb(t, store)
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/projects?q=alp", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, "alpha")
	assert.NotContains(t, body, ">beta<")
}

func TestProjectDetail_RendersDefinition(t *testing.T) {
	store := project.NewStore(t.TempDir())
	require.NoError(t, store.Load())
	_, err := store.Create(context.Background(), project.CreateRequest{
		Definition: project.Definition{Version: "1", Name: "detail-me", Description: "desc"},
	})
	require.NoError(t, err)
	ws := testWeb(t, store)
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/projects/detail-me", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, "detail-me")
	assert.Contains(t, body, "desc")
	assert.Contains(t, body, "sha256:")
}

func TestProjectDetail_NotFound(t *testing.T) {
	store := project.NewStore(t.TempDir())
	require.NoError(t, store.Load())
	ws := testWeb(t, store)
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/projects/missing", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "not found")
}
