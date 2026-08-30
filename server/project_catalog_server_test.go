package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/daltoniam/switchboard/project"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCatalogTestServer(t *testing.T, writes bool) (*httptest.Server, *project.Store) {
	t.Helper()
	store := project.NewStore(t.TempDir())
	require.NoError(t, store.Load())
	cat := NewProjectCatalogServer(store, store, store, store, ProjectCatalogOptions{WritesEnabled: writes})
	mux := BuildHTTPMux(HTTPMuxConfig{ProjectCatalog: cat.Handler()})
	httpSrv := httptest.NewServer(mux)
	t.Cleanup(httpSrv.Close)
	return httpSrv, store
}

func TestProjectCatalog_NoBearerRequired(t *testing.T) {
	httpSrv, _ := newCatalogTestServer(t, false)
	status, _, body := postJSONRPC(t, httpSrv.URL+"/project-catalog/mcp", jsonRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "server/discover",
		Params: map[string]any{"_meta": modernRequestMeta()},
	}, modernHTTPHeaders("server/discover", ""))
	require.Equal(t, http.StatusOK, status, body)
}

func TestProjectCatalog_CreateListGet(t *testing.T) {
	httpSrv, _ := newCatalogTestServer(t, true)
	client := newCatalogClient(t, httpSrv.URL+"/project-catalog/mcp")
	created, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "project_create",
		Arguments: map[string]any{
			"definition": map[string]any{"version": "1", "name": "acme", "description": "A project"},
		},
	})
	require.NoError(t, err)
	require.False(t, created.IsError, "%v", created.StructuredContent)

	listed, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "project_list", Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.False(t, listed.IsError)

	got, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "project_get", Arguments: map[string]any{"projectId": "acme"},
	})
	require.NoError(t, err)
	require.False(t, got.IsError)
	raw, _ := json.Marshal(got.StructuredContent)
	assert.Contains(t, string(raw), "A project")
}

func TestProjectCatalog_GetPreservesResources(t *testing.T) {
	// Live catalog files often carry a resources map in Additional.
	// project_get must not fail MCP outputSchema validation on those fields.
	httpSrv, store := newCatalogTestServer(t, true)
	dir := store.ConfigDir()
	path := filepath.Join(dir, "projects", "with-resources.project.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(`{
		"version": "1",
		"name": "with-resources",
		"description": "has resources",
		"resources": {
			"main": {"type": "repo", "path": "/tmp/x", "branch": "main"}
		}
	}`), 0o600))
	require.NoError(t, store.Load())

	client := newCatalogClient(t, httpSrv.URL+"/project-catalog/mcp")
	got, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "project_get", Arguments: map[string]any{"projectId": "with-resources"},
	})
	require.NoError(t, err)
	require.False(t, got.IsError, "%v", got.StructuredContent)
	raw, err := json.Marshal(got.StructuredContent)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"resources"`)
	assert.Contains(t, string(raw), "with-resources")
	assert.Contains(t, string(raw), "/tmp/x")
}

func TestProjectCatalog_UpdateAndConflict(t *testing.T) {
	httpSrv, store := newCatalogTestServer(t, true)
	created, err := store.Create(context.Background(), project.CreateRequest{
		Definition: project.Definition{Version: "1", Name: "acme", Description: "one"},
	})
	require.NoError(t, err)
	client := newCatalogClient(t, httpSrv.URL+"/project-catalog/mcp")
	ok, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "project_update",
		Arguments: map[string]any{
			"projectId": "acme", "expectedSourceRevision": string(created.SourceRevision),
			"patch": map[string]any{"description": "two"},
		},
	})
	require.NoError(t, err)
	require.False(t, ok.IsError, "%v", ok.StructuredContent)

	conflict, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "project_update",
		Arguments: map[string]any{
			"projectId": "acme", "expectedSourceRevision": string(created.SourceRevision),
			"patch": map[string]any{"description": "stale"},
		},
	})
	require.NoError(t, err)
	require.True(t, conflict.IsError)
}

func TestProjectCatalog_UpdateWithFullDefinition(t *testing.T) {
	// Full definition bodies always include name; update must strip matching name.
	httpSrv, store := newCatalogTestServer(t, true)
	created, err := store.Create(context.Background(), project.CreateRequest{
		Definition: project.Definition{Version: "1", Name: "acme", Description: "one"},
	})
	require.NoError(t, err)
	client := newCatalogClient(t, httpSrv.URL+"/project-catalog/mcp")
	ok, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "project_update",
		Arguments: map[string]any{
			"projectId":              "acme",
			"expectedSourceRevision": string(created.SourceRevision),
			"definition": map[string]any{
				"version":     "1",
				"name":        "acme",
				"description": "via-definition",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, ok.IsError, "%v", ok.StructuredContent)

	got, err := store.Get(context.Background(), "acme")
	require.NoError(t, err)
	assert.Equal(t, "via-definition", got.Definition.Description)
}

func TestProjectCatalog_KnownResourceIDsRoundTrip(t *testing.T) {
	httpSrv, store := newCatalogTestServer(t, true)
	client := newCatalogClient(t, httpSrv.URL+"/project-catalog/mcp")
	created, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "project_create",
		Arguments: map[string]any{
			"definition": map[string]any{
				"version": "1", "name": "obs", "description": "observed",
				"known_resource_ids": []string{"repo", "worktree-root"},
			},
		},
	})
	require.NoError(t, err)
	require.False(t, created.IsError, "%v", created.StructuredContent)

	got, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "project_get", Arguments: map[string]any{"projectId": "obs"},
	})
	require.NoError(t, err)
	require.False(t, got.IsError, "%v", got.StructuredContent)
	raw, _ := json.Marshal(got.StructuredContent)
	assert.Contains(t, string(raw), "known_resource_ids")
	assert.Contains(t, string(raw), "worktree-root")

	snap, err := store.Get(context.Background(), "obs")
	require.NoError(t, err)
	assert.Equal(t, []string{"repo", "worktree-root"}, snap.Definition.KnownResourceIDs)

	updated, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "project_update",
		Arguments: map[string]any{
			"projectId":              "obs",
			"expectedSourceRevision": string(snap.SourceRevision),
			"definition": map[string]any{
				"version": "1", "name": "obs", "known_resource_ids": []string{"repo"},
			},
		},
	})
	require.NoError(t, err)
	require.False(t, updated.IsError, "%v", updated.StructuredContent)
	snap, err = store.Get(context.Background(), "obs")
	require.NoError(t, err)
	assert.Equal(t, []string{"repo"}, snap.Definition.KnownResourceIDs)
}

func TestProjectCatalog_WritesDisabled(t *testing.T) {
	httpSrv, _ := newCatalogTestServer(t, false)
	client := newCatalogClient(t, httpSrv.URL+"/project-catalog/mcp")
	res, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "project_create",
		Arguments: map[string]any{"definition": map[string]any{"version": "1", "name": "nope"}},
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
	raw, _ := json.Marshal(res.StructuredContent)
	assert.Contains(t, string(raw), "write_disabled")
}

func TestReadProjectEnvelope_InternalErrorIsNotNotFound(t *testing.T) {
	internal := &project.Error{Code: project.CodeInternalError, Message: "archive write failed"}
	s := NewProjectCatalogServer(errCatalog{err: internal}, nil, nil, nil, ProjectCatalogOptions{})
	uri := "project://registry/projects/p"
	_, err := s.readProjectEnvelope(context.Background(), uri, "p")
	require.Error(t, err)
	assert.True(t, project.IsCode(err, project.CodeInternalError), "%v", err)
	assert.NotEqual(t, resourceNotFound(uri).Error(), err.Error())
}

func TestHandleReadResource_InternalErrorIsNotNotFound(t *testing.T) {
	internal := &project.Error{Code: project.CodeInternalError, Message: "revision unreadable"}
	s := NewProjectCatalogServer(errCatalog{err: internal}, nil, nil, nil, ProjectCatalogOptions{})
	uri := "project://registry/projects/p/revisions/sha256:" + "aa"
	_, err := s.handleReadResource(context.Background(), &mcpsdk.ReadResourceRequest{
		Params: &mcpsdk.ReadResourceParams{URI: uri},
	})
	require.Error(t, err)
	assert.True(t, project.IsCode(err, project.CodeInternalError), "%v", err)
}

type errCatalog struct {
	err error
}

func (e errCatalog) List(context.Context, string) (project.Page, error) {
	return project.Page{}, e.err
}
func (e errCatalog) Search(context.Context, project.SearchRequest) (project.Page, error) {
	return project.Page{}, e.err
}
func (e errCatalog) Get(context.Context, project.ProjectID) (project.Snapshot, error) {
	return project.Snapshot{}, e.err
}
func (e errCatalog) GetRevision(context.Context, project.ProjectID, project.Revision) (project.RevisionSnapshot, error) {
	return project.RevisionSnapshot{}, e.err
}
func (e errCatalog) Diagnostics(context.Context, project.ProjectID, *url.URL) (project.DiagnosticsEnvelope, error) {
	return project.DiagnosticsEnvelope{}, e.err
}
func (e errCatalog) Resolve(context.Context, project.ResolveRequest) (project.Snapshot, error) {
	return project.Snapshot{}, e.err
}

func newCatalogClient(t *testing.T, endpoint string) *mcpsdk.ClientSession {
	t.Helper()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "catalog-test", Version: "0"}, nil)
	session, err := client.Connect(context.Background(), &mcpsdk.StreamableClientTransport{
		Endpoint:             endpoint,
		HTTPClient:           http.DefaultClient,
		DisableStandaloneSSE: true,
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}
