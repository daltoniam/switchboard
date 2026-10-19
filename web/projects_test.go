package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type countingProjectCatalog struct {
	project.Catalog
	searchRequests []project.SearchRequest
	listCursors    []string
	searchErr      error
	listErr        error
}

func TestProjectsList_CountFromSearch(t *testing.T) {
	for _, tc := range []struct {
		name, hxRequest, target, cursor string
		wantCount                       bool
	}{
		{name: "document", wantCount: true},
		{name: "filter", hxRequest: "true", target: "project-results"},
		{name: "pagination", hxRequest: "true", target: "project-results", cursor: "search-page-2"},
		{name: "refresh", hxRequest: "true", wantCount: true},
		{name: "navigation", hxRequest: "true", target: "main", wantCount: true},
		{name: "target without HTMX", hxRequest: "false", target: "project-results", wantCount: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			catalog := &countingProjectCatalog{}
			rr := requestProjectList(t, catalog, tc.hxRequest, tc.target, tc.cursor)
			require.Equal(t, http.StatusOK, rr.Code)
			assert.Contains(t, rr.Header().Get("Vary"), "HX-Request")
			assert.Contains(t, rr.Header().Get("Vary"), "HX-Target")
			expectedRequests := []project.SearchRequest{{Query: "alp", Cursor: tc.cursor}}
			assert.Equal(t, expectedRequests, catalog.searchRequests)
			assert.Contains(t, rr.Body.String(), `id="project-results"`)
			assert.Contains(t, rr.Body.String(), "alpha")
			assert.Contains(t, rr.Body.String(), `>1 of 3 projects</span>`)
			assert.Contains(t, rr.Body.String(), "search-page-3")
			assert.Contains(t, rr.Body.String(), `hx-push-url="true" data-project-page`)
			assert.Empty(t, catalog.listCursors, "counting must reuse the search snapshot without listing the catalog again")
			if !tc.wantCount {
				assert.Contains(t, rr.Body.String(), `hx-swap-oob="outerHTML"`)
				assert.NotContains(t, rr.Body.String(), "<html")
				assert.NotContains(t, rr.Body.String(), `id="project-search"`)
				return
			}
			assert.Contains(t, rr.Body.String(), "<html")
			assert.Contains(t, rr.Body.String(), `>1 of 3 projects</span>`)
		})
	}
}

func TestProjectsList_WithoutCatalog(t *testing.T) {
	for _, tc := range []struct {
		name, hxRequest, target string
		fragment                bool
	}{
		{name: "document"},
		{name: "filter", hxRequest: "true", target: "project-results", fragment: true},
		{name: "refresh", hxRequest: "true", target: "main"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rr := requestProjectList(t, nil, tc.hxRequest, tc.target, "")
			require.Equal(t, http.StatusOK, rr.Code)
			assert.Contains(t, rr.Body.String(), `id="project-results"`)
			assert.Contains(t, rr.Body.String(), "No projects match this view.")
			if tc.fragment {
				assert.NotContains(t, rr.Body.String(), "<html")
				return
			}
			assert.Contains(t, rr.Body.String(), "<html")
			assert.Contains(t, rr.Body.String(), `>0 of 0 projects</span>`)
		})
	}
}

func TestProjectsList_FilterIgnoresCountFailure(t *testing.T) {
	catalog := &countingProjectCatalog{listErr: errors.New("count unavailable")}
	rr := requestProjectList(t, catalog, "true", "project-results", "")
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Empty(t, catalog.listCursors)
	assert.Contains(t, rr.Body.String(), "alpha")
}

func TestProjectsList_FilterSearchFailure(t *testing.T) {
	catalog := &countingProjectCatalog{searchErr: errors.New("search unavailable")}
	rr := requestProjectList(t, catalog, "true", "project-results", "")
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Empty(t, catalog.listCursors)
	assert.Contains(t, rr.Body.String(), "Failed to load projects")
}

func TestProjectsList_RefreshIgnoresListFailure(t *testing.T) {
	catalog := &countingProjectCatalog{listErr: errors.New("count unavailable")}
	rr := requestProjectList(t, catalog, "true", "", "")
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Empty(t, catalog.listCursors)
	assert.Contains(t, rr.Body.String(), `>1 of 3 projects</span>`)
}

func requestProjectList(t *testing.T, catalog project.Catalog, hxRequest, target, cursor string) *httptest.ResponseRecorder {
	t.Helper()
	ws, _, _ := setupTestWeb()
	ws.catalog = catalog
	req := httptest.NewRequest(http.MethodGet, "/projects?q=alp&cursor="+cursor, nil)
	req.Header.Set("HX-Request", hxRequest)
	req.Header.Set("HX-Target", target)
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, req)
	return rr
}

func (c *countingProjectCatalog) Search(_ context.Context, req project.SearchRequest) (project.Page, error) {
	c.searchRequests = append(c.searchRequests, req)
	page := project.Page{
		Projects:     []project.ProjectSummary{{ProjectID: "alpha", Title: "alpha"}},
		NextCursor:   "search-page-3",
		TotalCount:   1,
		CatalogCount: 3,
	}
	return page, c.searchErr
}

func (c *countingProjectCatalog) List(_ context.Context, cursor string) (project.Page, error) {
	c.listCursors = append(c.listCursors, cursor)
	return project.Page{}, c.listErr
}

func TestNavigationSearch_ProjectMatches(t *testing.T) {
	store := project.NewStore(t.TempDir())
	require.NoError(t, store.Load())
	definition := project.Definition{Version: "1", Name: "website", Description: "Handles payments"}
	_, err := store.Create(context.Background(), project.CreateRequest{Definition: definition})
	require.NoError(t, err)
	ws := testWeb(t, store)
	for _, query := range []string{"payments", "website", "WeBsItE"} {
		t.Run(query, func(t *testing.T) {
			rr := httptest.NewRecorder()
			ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/navigation?q="+query, nil))
			require.Equal(t, http.StatusOK, rr.Code)
			assert.JSONEq(t, `[{"label":"Project: website","path":"/projects/website"}]`, rr.Body.String())
		})
	}
}

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
	assert.Contains(t, body, ">1 of 2 projects</span>")
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
