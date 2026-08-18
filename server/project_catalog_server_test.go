package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

	got, err = parseCatalogURI("project://registry/projects/acme")
	require.NoError(t, err)
	assert.Equal(t, "project", got.kind)
	assert.Equal(t, project.ProjectID("acme"), got.projectID)

	got, err = parseCatalogURI("project://registry/projects/acme/resources")
	require.NoError(t, err)
	assert.Equal(t, "resources", got.kind)

	got, err = parseCatalogURI("project://registry/projects/acme/resources/architecture")
	require.NoError(t, err)
	assert.Equal(t, "resource", got.kind)
	assert.Equal(t, "architecture", got.resourceID)

	got, err = parseCatalogURI("project://registry/projects/acme/resources/architecture/content")
	require.NoError(t, err)
	assert.Equal(t, "resource-content", got.kind)

	got, err = parseCatalogURI("project://registry/projects/switchboard/resources/project-specs/files/docs%2Fspecs%2Fproject-interop%2FREADME.md")
	require.NoError(t, err)
	assert.Equal(t, "resource-file", got.kind)
	assert.Equal(t, "project-specs", got.resourceID)
	assert.Equal(t, "docs/specs/project-interop/README.md", got.path)

	got, err = parseCatalogURI("project://registry/projects/acme/context/AGENTS.md?rootUri=file:///tmp/wt")
	require.NoError(t, err)
	assert.Equal(t, "context", got.kind)
	assert.Equal(t, "AGENTS.md", got.path)
	assert.Equal(t, "file:///tmp/wt", got.rootURI)

	// Compatibility alias.
	got, err = parseCatalogURI("project://registry/projects/acme/definition")
	require.NoError(t, err)
	assert.Equal(t, "definition", got.kind)
}

func TestProjectCatalog_ResourceURIsAndContent(t *testing.T) {
	httpSrv, store := newCatalogTestServer(t, true)

	repoA := t.TempDir()
	repoB := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(repoA, "AGENTS.md"), []byte("# agents\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(repoB, "docs", "specs"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(repoB, "docs", "specs", "one.md"), []byte("spec one"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(repoB, "docs", "specs", "two.md"), []byte("spec two"), 0o600))
	notes := filepath.Join(t.TempDir(), "notes.md")
	require.NoError(t, os.WriteFile(notes, []byte("private"), 0o600))

	def := map[string]any{
		"version":     "1",
		"name":        "switchboard",
		"description": "Switchboard and related projects",
		"resources": map[string]any{
			"switchboard": map[string]any{"type": "repo", "path": repoA, "branch": "main"},
			"awesometree": map[string]any{"type": "repo", "path": repoB, "branch": "master"},
			"architecture": map[string]any{
				"type": "file", "repo": "switchboard", "path": "AGENTS.md",
			},
			"private-notes": map[string]any{"type": "file", "path": notes},
			"project-specs": map[string]any{
				"type": "files", "repo": "awesometree",
				"include": []string{"docs/specs/**/*.md"},
			},
		},
	}

	client := newCatalogClient(t, httpSrv.URL+"/project-catalog/mcp")
	created, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "project.create",
		Arguments: map[string]any{"definition": def},
	})
	require.NoError(t, err)
	require.False(t, created.IsError, "%v", created.StructuredContent)

	// project.list / project.get clean names
	listed, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "project.list",
		Arguments: map[string]any{"query": "switch"},
	})
	require.NoError(t, err)
	require.False(t, listed.IsError)

	got, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "project.get",
		Arguments: map[string]any{"projectId": "switchboard"},
	})
	require.NoError(t, err)
	require.False(t, got.IsError)

	// Envelope URI
	envRaw := readResourceRawAuth(t, httpSrv.URL+"/project-catalog/mcp", projectResourceURI("switchboard"))
	envText, _ := resultArray(t, envRaw, "contents")[0].(map[string]any)["text"].(string)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(envText), &env))
	assert.Equal(t, "switchboard", env["projectId"])
	assert.Contains(t, env, "definition")
	assert.Contains(t, env, "summary")

	// Resources list
	listRaw := readResourceRawAuth(t, httpSrv.URL+"/project-catalog/mcp", projectResourcesURI("switchboard"))
	listText, _ := resultArray(t, listRaw, "contents")[0].(map[string]any)["text"].(string)
	var listEnv map[string]any
	require.NoError(t, json.Unmarshal([]byte(listText), &listEnv))
	resources, _ := listEnv["resources"].([]any)
	assert.GreaterOrEqual(t, len(resources), 5)

	// Repo metadata
	repoRaw := readResourceRawAuth(t, httpSrv.URL+"/project-catalog/mcp", projectResourceMetaURI("switchboard", "switchboard"))
	repoText, _ := resultArray(t, repoRaw, "contents")[0].(map[string]any)["text"].(string)
	var repoMeta map[string]any
	require.NoError(t, json.Unmarshal([]byte(repoText), &repoMeta))
	assert.Equal(t, "repo", repoMeta["type"])
	assert.Equal(t, "main", repoMeta["branch"])
	assert.Equal(t, repoA, repoMeta["path"])

	// Repo content is metadata
	repoContentRaw := readResourceRawAuth(t, httpSrv.URL+"/project-catalog/mcp", projectResourceContentURI("switchboard", "switchboard"))
	repoContentText, _ := resultArray(t, repoContentRaw, "contents")[0].(map[string]any)["text"].(string)
	assert.Contains(t, repoContentText, `"type":"repo"`)

	// File content
	fileRaw := readResourceRawAuth(t, httpSrv.URL+"/project-catalog/mcp", projectResourceContentURI("switchboard", "architecture"))
	fileText, _ := resultArray(t, fileRaw, "contents")[0].(map[string]any)["text"].(string)
	assert.Contains(t, fileText, "# agents")

	localRaw := readResourceRawAuth(t, httpSrv.URL+"/project-catalog/mcp", projectResourceContentURI("switchboard", "private-notes"))
	localText, _ := resultArray(t, localRaw, "contents")[0].(map[string]any)["text"].(string)
	assert.Equal(t, "private", localText)

	// Files manifest with progressive URIs
	manifestRaw := readResourceRawAuth(t, httpSrv.URL+"/project-catalog/mcp", projectResourceContentURI("switchboard", "project-specs"))
	manifestText, _ := resultArray(t, manifestRaw, "contents")[0].(map[string]any)["text"].(string)
	var manifest map[string]any
	require.NoError(t, json.Unmarshal([]byte(manifestText), &manifest))
	assert.Equal(t, "project-specs", manifest["resource"])
	files, _ := manifest["files"].([]any)
	require.NotEmpty(t, files)
	first, _ := files[0].(map[string]any)
	uri, _ := first["uri"].(string)
	assert.Contains(t, uri, "/resources/project-specs/files/")
	assert.Contains(t, uri, "docs")

	// Progressive file read
	progRaw := readResourceRawAuth(t, httpSrv.URL+"/project-catalog/mcp", uri)
	progText, _ := resultArray(t, progRaw, "contents")[0].(map[string]any)["text"].(string)
	assert.Contains(t, progText, "spec")

	// validate semantic repo refs
	bad, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "project.validate",
		Arguments: map[string]any{
			"definition": map[string]any{
				"version": "1",
				"name":    "bad",
				"resources": map[string]any{
					"doc": map[string]any{"type": "file", "repo": "missing", "path": "x.md"},
				},
			},
		},
	})
	require.NoError(t, err)
	require.False(t, bad.IsError)
	raw, err := json.Marshal(bad.StructuredContent)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"valid":false`)

	// multi-repo search still works
	search, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "project.search",
		Arguments: map[string]any{"query": "switchboard"},
	})
	require.NoError(t, err)
	require.False(t, search.IsError)

	// ensure store isolation path remains under configured dir (temp store)
	assert.NotContains(t, store.ConfigDir(), "project-interop")
}

func TestProjectCatalog_UpdateAlias(t *testing.T) {
	httpSrv, store := newCatalogTestServer(t, true)
	snap, err := store.Create(context.Background(), project.CreateRequest{
		Definition: project.Definition{
			Version: "1", Name: "upd",
			Resources: map[string]project.Resource{
				"main": {Type: project.ResourceTypeRepo, Path: "/tmp/upd", Branch: "one"},
			},
		},
	})
	require.NoError(t, err)

	client := newCatalogClient(t, httpSrv.URL+"/project-catalog/mcp")
	updated, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "project.update",
		Arguments: map[string]any{
			"projectId":              "upd",
			"expectedSourceRevision": string(snap.SourceRevision),
			"patch": map[string]any{
				"resources": map[string]any{
					"main": map[string]any{"type": "repo", "path": "/tmp/upd", "branch": "two"},
				},
			},
		},
	})
	require.NoError(t, err)
	require.False(t, updated.IsError, "%v", updated.StructuredContent)

	got, err := store.Get(context.Background(), "upd")
	require.NoError(t, err)
	assert.Equal(t, "two", got.Definition.Resources["main"].Branch)
}

var _ = strings.TrimSpace
