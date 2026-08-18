package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	cat := NewProjectCatalogServer(store, store, store, store, ProjectCatalogOptions{
		WritesEnabled: writes,
	})
	mux := BuildHTTPMux(HTTPMuxConfig{ProjectCatalog: cat.Handler()})
	httpSrv := httptest.NewServer(mux)
	t.Cleanup(httpSrv.Close)
	return httpSrv, store
}

func catalogHeaders(method, name string) http.Header {
	return modernHTTPHeaders(method, name)
}

func TestProjectCatalog_NoBearerRequired(t *testing.T) {
	httpSrv, _ := newCatalogTestServer(t, false)
	status, _, body := postJSONRPC(t, httpSrv.URL+"/project-catalog/mcp", jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "server/discover",
		Params:  map[string]any{"_meta": modernRequestMeta()},
	}, modernHTTPHeaders("server/discover", ""))
	require.Equal(t, http.StatusOK, status, body)
	result := jsonRPCResult(t, body)
	assert.Equal(t, "complete", result["resultType"])
}

func TestProjectCatalog_DiscoverAndListResources(t *testing.T) {
	httpSrv, store := newCatalogTestServer(t, true)
	_, err := store.Create(context.Background(), project.CreateRequest{Definition: project.Definition{Version: "1", Name: "acme", Resources: map[string]project.Resource{}}})
	require.NoError(t, err)

	status, headers, body := postJSONRPC(t, httpSrv.URL+"/project-catalog/mcp", jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "server/discover",
		Params:  map[string]any{"_meta": modernRequestMeta()},
	}, catalogHeaders("server/discover", ""))
	require.Equal(t, http.StatusOK, status, body)
	assert.Empty(t, headers.Get(mcpSessionIDH))
	result := jsonRPCResult(t, body)
	assert.Contains(t, result["supportedVersions"], mcpProtocol20260728)
	caps, _ := result["capabilities"].(map[string]any)
	require.NotNil(t, caps)
	resCaps, _ := caps["resources"].(map[string]any)
	require.NotNil(t, resCaps)
	assert.Equal(t, true, resCaps["listChanged"])
	assert.Equal(t, true, resCaps["subscribe"])

	status, _, body = postJSONRPC(t, httpSrv.URL+"/project-catalog/mcp", jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "resources/list",
		Params:  map[string]any{"_meta": modernRequestMeta()},
	}, catalogHeaders("resources/list", ""))
	require.Equal(t, http.StatusOK, status, body)
	listed := jsonRPCResult(t, body)
	assert.Equal(t, "private", listed["cacheScope"])
	uris := resourceURIs(t, listed)
	assert.Contains(t, uris, catalogResourceURI)
}

func TestProjectCatalog_ReadDefinitionAndRevision(t *testing.T) {
	httpSrv, store := newCatalogTestServer(t, true)
	created, err := store.Create(context.Background(), project.CreateRequest{Definition: project.Definition{Version: "1", Name: "acme", Resources: map[string]project.Resource{"main": {Type: project.ResourceTypeRepo, Path: "/tmp/acme", Branch: "one"}}}})
	require.NoError(t, err)
	r1 := created.Revision
	_, err = store.Patch(context.Background(), project.PatchRequest{
		ProjectID:              "acme",
		ExpectedSourceRevision: created.SourceRevision,
		Patch:                  json.RawMessage(`{"resources":{"main":{"type":"repo","path":"/tmp/acme","branch":"two"}}}`),
	})
	require.NoError(t, err)

	current := readResourceRawAuth(t, httpSrv.URL+"/project-catalog/mcp", definitionResourceURI("acme"))
	contents := resultArray(t, current, "contents")
	require.NotEmpty(t, contents)
	text, _ := contents[0].(map[string]any)["text"].(string)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &env))
	assert.Equal(t, "acme", env["projectId"])
	assert.NotEqual(t, string(r1), env["revision"])

	rev := readResourceRawAuth(t, httpSrv.URL+"/project-catalog/mcp", revisionResourceURI("acme", r1))
	revContents := resultArray(t, rev, "contents")
	revText, _ := revContents[0].(map[string]any)["text"].(string)
	var revEnv map[string]any
	require.NoError(t, json.Unmarshal([]byte(revText), &revEnv))
	def, _ := revEnv["definition"].(map[string]any)
	resources, _ := def["resources"].(map[string]any)
	main, _ := resources["main"].(map[string]any)
	assert.Equal(t, "one", main["branch"])
	assert.NotContains(t, revEnv, "sources")
}

func TestProjectCatalog_WritesDisabledByDefault(t *testing.T) {
	httpSrv, _ := newCatalogTestServer(t, false)
	client := newCatalogClient(t, httpSrv.URL+"/project-catalog/mcp")
	res, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "project.create",
		Arguments: map[string]any{"definition": map[string]any{"version": "1", "name": "nope", "resources": map[string]any{}}},
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
	raw, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "write_disabled")
}

func TestProjectCatalog_CreateSearchResolve(t *testing.T) {
	httpSrv, _ := newCatalogTestServer(t, true)
	client := newCatalogClient(t, httpSrv.URL+"/project-catalog/mcp")
	created, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "project.create",
		Arguments: map[string]any{"definition": map[string]any{"version": "1", "name": "acme", "resources": map[string]any{"main": map[string]any{"type": "repo", "path": "/tmp/acme", "branch": "main"}}}},
	})
	require.NoError(t, err)
	require.False(t, created.IsError)

	search, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "project.search",
		Arguments: map[string]any{"query": "acme"},
	})
	require.NoError(t, err)
	require.False(t, search.IsError)

	resolved, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "project.resolve",
		Arguments: map[string]any{"projectId": "acme"},
	})
	require.NoError(t, err)
	require.False(t, resolved.IsError)
}

func TestProjectCatalog_MissingResourceIsInvalidParams(t *testing.T) {
	httpSrv, _ := newCatalogTestServer(t, false)
	status, _, body := postJSONRPC(t, httpSrv.URL+"/project-catalog/mcp", jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      9,
		Method:  "resources/read",
		Params: map[string]any{
			"_meta": modernRequestMeta(),
			"uri":   "project://registry/projects/missing/definition",
		},
	}, catalogHeaders("resources/read", "project://registry/projects/missing/definition"))
	require.True(t, status == http.StatusOK || status == http.StatusBadRequest, "status=%d body=%v", status, body)
	rpcErr, _ := body["error"].(map[string]any)
	require.NotNil(t, rpcErr, body)
	assert.EqualValues(t, -32602, rpcErr["code"])
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

func readResourceRawAuth(t *testing.T, endpoint, uri string) map[string]any {
	t.Helper()
	status, _, body := postJSONRPC(t, endpoint, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "resources/read",
		Params:  map[string]any{"_meta": modernRequestMeta(), "uri": uri},
	}, catalogHeaders("resources/read", uri))
	require.Equal(t, http.StatusOK, status, body)
	return jsonRPCResult(t, body)
}

func TestParseCatalogURI(t *testing.T) {
	got, err := parseCatalogURI("project://registry/catalog")
	require.NoError(t, err)
	assert.Equal(t, "catalog", got.kind)

	got, err = parseCatalogURI("project://registry/projects/acme/context/AGENTS.md?rootUri=file:///tmp/wt")
	require.NoError(t, err)
	assert.Equal(t, "context", got.kind)
	assert.Equal(t, "AGENTS.md", got.path)
	assert.Equal(t, "file:///tmp/wt", got.rootURI)
}

var _ = strings.TrimSpace
