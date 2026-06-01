package notion

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// newV1WithServer wires a *notionV1 against a test HTTP server and runs
// Configure (which hits /users/me). Returns the configured backend so
// tests can call handlers directly.
func newV1WithServer(t *testing.T, handler http.HandlerFunc) (*notionV1, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	v1 := newV1Backend("test-access-token")
	v1.baseURL = ts.URL
	v1.client = ts.Client()
	return v1, ts
}

func TestV1_ConfigureHitsUsersMe(t *testing.T) {
	var path string
	var authHeader string
	var versionHeader string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		authHeader = r.Header.Get("Authorization")
		versionHeader = r.Header.Get("Notion-Version")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"user","id":"u1","type":"bot"}`))
	}))
	defer ts.Close()

	n := New().(*notion)
	// Point the v1 client to the test server. New() wires the v3 client
	// but Configure builds the v1 backend lazily, so we have to inject
	// after Configure rejects (or override baseURL via creds).
	err := n.Configure(context.Background(), mcp.Credentials{
		"access_token": "sk-test",
		"base_url":     ts.URL,
	})
	require.NoError(t, err)
	require.NotNil(t, n.v1, "v1 backend should be selected when access_token is present")
	assert.Equal(t, "/users/me", path)
	assert.Equal(t, "Bearer sk-test", authHeader)
	assert.Equal(t, defaultV1APIVersion, versionHeader)
}

func TestV1_PreferredOverTokenV2WhenBothPresent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"user","id":"u1"}`))
	}))
	defer ts.Close()

	n := New().(*notion)
	err := n.Configure(context.Background(), mcp.Credentials{
		"access_token": "sk-test",
		"token_v2":     "should-be-ignored",
		"base_url":     ts.URL,
	})
	require.NoError(t, err)
	assert.NotNil(t, n.v1, "OAuth backend should win when both creds are set")
	// tokenV2 field should be untouched (v3 path skipped entirely).
	assert.Empty(t, n.tokenV2, "v3 fields should not be populated when v1 wins")
}

func TestV1_ExecuteRoutesToV1Dispatch(t *testing.T) {
	var hitPath string
	v1, _ := newV1WithServer(t, func(w http.ResponseWriter, r *http.Request) {
		hitPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"page","id":"page-1"}`))
	})

	n := &notion{v1: v1}
	res, err := n.Execute(context.Background(), mcp.ToolName("notion_retrieve_page"), map[string]any{
		"page_id": "page-1",
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.False(t, res.IsError)
	assert.Equal(t, "/pages/page-1", hitPath)
	assert.Contains(t, res.Data, `"object":"page"`)
}

func TestV1_RetrievePage(t *testing.T) {
	v1, _ := newV1WithServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/pages/abc-123", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"page","id":"abc-123"}`))
	})
	res, err := v1RetrievePage(context.Background(), v1, map[string]any{"page_id": "abc-123"})
	require.NoError(t, err)
	require.False(t, res.IsError)
	assert.Contains(t, res.Data, `"id":"abc-123"`)
}

func TestV1_CreatePageBuildsBody(t *testing.T) {
	var gotBody map[string]any
	v1, _ := newV1WithServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/pages", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"page","id":"new"}`))
	})

	res, err := v1CreatePage(context.Background(), v1, map[string]any{
		"parent": map[string]any{"page_id": "parent-1"},
		"title":  "Hello",
	})
	require.NoError(t, err)
	require.False(t, res.IsError)

	parent, _ := gotBody["parent"].(map[string]any)
	assert.Equal(t, "page_id", parent["type"])
	assert.Equal(t, "parent-1", parent["page_id"])

	props, _ := gotBody["properties"].(map[string]any)
	require.NotNil(t, props, "title convenience should populate properties.title")
	titleArr, _ := props["title"].(map[string]any)["title"].([]any)
	require.NotEmpty(t, titleArr)
	first, _ := titleArr[0].(map[string]any)
	text, _ := first["text"].(map[string]any)
	assert.Equal(t, "Hello", text["content"])
}

func TestV1_UpdatePagePartialUpdates(t *testing.T) {
	var gotBody map[string]any
	v1, _ := newV1WithServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"page","id":"p1"}`))
	})

	_, err := v1UpdatePage(context.Background(), v1, map[string]any{
		"page_id":  "p1",
		"archived": true,
	})
	require.NoError(t, err)
	assert.Equal(t, true, gotBody["archived"])
}

func TestV1_UpdatePageRejectsNoFields(t *testing.T) {
	v1 := newV1Backend("tok")
	res, err := v1UpdatePage(context.Background(), v1, map[string]any{"page_id": "p1"})
	require.NoError(t, err)
	require.True(t, res.IsError)
	assert.Contains(t, res.Data, "at least one of properties or archived")
}

func TestV1_MovePageSendsParentObject(t *testing.T) {
	var gotBody map[string]any
	v1, _ := newV1WithServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"page","id":"p1"}`))
	})
	_, err := v1MovePage(context.Background(), v1, map[string]any{
		"page_id": "p1",
		"parent":  map[string]any{"page_id": "new-parent"},
	})
	require.NoError(t, err)
	parent, _ := gotBody["parent"].(map[string]any)
	assert.Equal(t, "page_id", parent["type"])
	assert.Equal(t, "new-parent", parent["page_id"])
}

func TestV1_AppendBlockChildrenTranslatesV3Shape(t *testing.T) {
	var gotBody map[string]any
	v1, _ := newV1WithServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/blocks/parent-page/children", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","results":[]}`))
	})

	// Caller passes legacy v3-shaped blocks.
	_, err := v1AppendBlockChildren(context.Background(), v1, map[string]any{
		"block_id": "parent-page",
		"children": []any{
			map[string]any{
				"type": "header",
				"properties": map[string]any{
					"title": []any{[]any{"Section 1"}},
				},
			},
			map[string]any{
				"type": "text",
				"properties": map[string]any{
					"title": []any{[]any{"A paragraph."}},
				},
			},
		},
	})
	require.NoError(t, err)

	children, _ := gotBody["children"].([]any)
	require.Len(t, children, 2)

	// First child: heading_1
	b1, _ := children[0].(map[string]any)
	assert.Equal(t, "block", b1["object"])
	assert.Equal(t, "heading_1", b1["type"])
	h1, _ := b1["heading_1"].(map[string]any)
	rt, _ := h1["rich_text"].([]any)
	require.NotEmpty(t, rt)
	first, _ := rt[0].(map[string]any)
	txt, _ := first["text"].(map[string]any)
	assert.Equal(t, "Section 1", txt["content"])

	// Second child: paragraph
	b2, _ := children[1].(map[string]any)
	assert.Equal(t, "paragraph", b2["type"])
}

func TestV1_AppendBlockChildrenPassesThroughV1Shape(t *testing.T) {
	var gotBody map[string]any
	v1, _ := newV1WithServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","results":[]}`))
	})

	// Caller passes a fully-formed v1 block — should pass through with
	// nothing beyond the {"object":"block"} wrapper added.
	_, err := v1AppendBlockChildren(context.Background(), v1, map[string]any{
		"block_id": "parent",
		"children": []any{
			map[string]any{
				"type": "paragraph",
				"paragraph": map[string]any{
					"rich_text": []any{
						map[string]any{
							"type": "text",
							"text": map[string]any{"content": "Bold!"},
							"annotations": map[string]any{
								"bold": true,
							},
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)
	children, _ := gotBody["children"].([]any)
	require.Len(t, children, 1)
	b, _ := children[0].(map[string]any)
	p, _ := b["paragraph"].(map[string]any)
	rt, _ := p["rich_text"].([]any)
	first, _ := rt[0].(map[string]any)
	ann, _ := first["annotations"].(map[string]any)
	assert.Equal(t, true, ann["bold"], "annotations on the rich_text run should pass through untouched")
}

func TestV1_QueryDataSourcePrefersDataSourcesEndpoint(t *testing.T) {
	var paths []string
	v1, _ := newV1WithServer(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","results":[]}`))
	})
	_, err := v1QueryDataSource(context.Background(), v1, map[string]any{
		"data_source_id": "ds-1",
	})
	require.NoError(t, err)
	require.Len(t, paths, 1)
	assert.Equal(t, "/data_sources/ds-1/query", paths[0])
}

func TestV1_QueryDataSourceFallsBackToDatabases(t *testing.T) {
	var paths []string
	v1, _ := newV1WithServer(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/data_sources/") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"object":"error","code":"object_not_found","message":"not a data source"}`))
			return
		}
		_, _ = w.Write([]byte(`{"object":"list","results":[{"id":"row1"}]}`))
	})
	res, err := v1QueryDataSource(context.Background(), v1, map[string]any{
		"data_source_id": "old-db-id",
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Len(t, paths, 2)
	assert.Equal(t, "/data_sources/old-db-id/query", paths[0])
	assert.Equal(t, "/databases/old-db-id/query", paths[1])
	assert.Contains(t, res.Data, `"row1"`)
}

func TestV1_SearchTranslatesDatabaseTypeToDataSource(t *testing.T) {
	var gotBody map[string]any
	v1, _ := newV1WithServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","results":[]}`))
	})
	_, err := v1Search(context.Background(), v1, map[string]any{
		"query": "roadmap",
		"type":  "database",
	})
	require.NoError(t, err)
	filter, _ := gotBody["filter"].(map[string]any)
	assert.Equal(t, "object", filter["property"])
	assert.Equal(t, "data_source", filter["value"], "type=database should be translated to data_source for v1")
	assert.Equal(t, "roadmap", gotBody["query"])
}

func TestV1_RetryableErrorFor429(t *testing.T) {
	v1, _ := newV1WithServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "3")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"object":"error","code":"rate_limited","message":"slow down"}`))
	})
	_, err := v1.get(context.Background(), "/users/me")
	require.Error(t, err)
	var re *mcp.RetryableError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, http.StatusTooManyRequests, re.StatusCode)
	assert.Contains(t, err.Error(), "rate_limited")
}

func TestV1_FormattedErrorOn400(t *testing.T) {
	v1, _ := newV1WithServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"object":"error","code":"validation_error","message":"bad parent"}`))
	})
	_, err := v1.get(context.Background(), "/pages/x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
	assert.Contains(t, err.Error(), "validation_error")
	assert.Contains(t, err.Error(), "bad parent")
}

func TestV1_GetPageContentCombinesPageAndChildren(t *testing.T) {
	v1, _ := newV1WithServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/pages/p1":
			_, _ = w.Write([]byte(`{"object":"page","id":"p1","properties":{}}`))
		case "/blocks/p1/children":
			_, _ = w.Write([]byte(`{"object":"list","results":[{"id":"b1","type":"paragraph"}],"has_more":false}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	res, err := v1GetPageContent(context.Background(), v1, map[string]any{"page_id": "p1"})
	require.NoError(t, err)
	require.False(t, res.IsError)
	assert.Contains(t, res.Data, `"id":"p1"`)
	assert.Contains(t, res.Data, `"id":"b1"`)
	assert.Contains(t, res.Data, `"blocks"`)
	assert.Contains(t, res.Data, `"page"`)
}

func TestV1_ExecuteUnknownToolErrors(t *testing.T) {
	n := &notion{v1: newV1Backend("tok")}
	res, err := n.Execute(context.Background(), mcp.ToolName("nope_does_not_exist"), nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	assert.Contains(t, res.Data, "not yet supported on the Notion OAuth (v1) backend")
}

func TestV1_ListUsersHitsCorrectEndpoint(t *testing.T) {
	v1, _ := newV1WithServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/users", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","results":[{"id":"u1"}]}`))
	})
	res, err := v1ListUsers(context.Background(), v1, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	assert.Contains(t, res.Data, `"u1"`)
}

func TestV1_CreateCommentNewDiscussion(t *testing.T) {
	var gotBody map[string]any
	v1, _ := newV1WithServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/comments", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"comment","id":"c1"}`))
	})
	_, err := v1CreateComment(context.Background(), v1, map[string]any{
		"page_id": "page-1",
		"text":    "first!",
	})
	require.NoError(t, err)
	parent, _ := gotBody["parent"].(map[string]any)
	assert.Equal(t, "page-1", parent["page_id"])
	rt, _ := gotBody["rich_text"].([]any)
	require.NotEmpty(t, rt)
}

func TestV1_CreateCommentReply(t *testing.T) {
	var gotBody map[string]any
	v1, _ := newV1WithServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"comment","id":"c2"}`))
	})
	_, err := v1CreateComment(context.Background(), v1, map[string]any{
		"discussion_id": "disc-1",
		"text":          "replying",
	})
	require.NoError(t, err)
	assert.Equal(t, "disc-1", gotBody["discussion_id"])
	_, hasParent := gotBody["parent"]
	assert.False(t, hasParent, "discussion_id reply should not set parent")
}

func TestV1BuildParentObjectVariants(t *testing.T) {
	cases := []struct {
		name    string
		in      map[string]any
		wantKey string
		wantVal any
	}{
		{"page", map[string]any{"page_id": "p"}, "page_id", "p"},
		{"database", map[string]any{"database_id": "d"}, "database_id", "d"},
		{"data_source", map[string]any{"data_source_id": "ds"}, "data_source_id", "ds"},
		{"workspace", map[string]any{"workspace": true}, "workspace", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := v1BuildParentObject(tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.wantKey, out["type"])
			assert.Equal(t, tc.wantVal, out[tc.wantKey])
		})
	}
}

func TestV1BuildParentObjectRejectsEmpty(t *testing.T) {
	_, err := v1BuildParentObject(map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page_id")
}

// --- Dispatch parity (v1) ---
//
// Mirror the v3 dispatch parity tests so a misspelled handler name or a
// new tool added to Tools() without a v1 implementation fails at test
// time rather than silently routing to the "not yet supported on v1
// backend" error at runtime.

func TestDispatchV1_EveryToolHasHandler(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		_, ok := dispatchV1[tool.Name]
		assert.True(t, ok, "tool %s has no v1 dispatch handler", tool.Name)
	}
}

func TestDispatchV1_NoOrphanHandlers(t *testing.T) {
	i := New()
	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatchV1 {
		assert.True(t, toolNames[name], "v1 dispatch handler %s has no tool definition", name)
	}
}

// --- v1 compact specs (compact_v1.yaml) ---

// configuredV1 returns a *notion already pointed at a v1 backend with a
// stub HTTP client. Used by CompactSpec/MaxBytes/Views tests so they
// exercise the v1 branch rather than the default v3 one.
func configuredV1(t *testing.T) *notion {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"user","id":"u1","type":"bot"}`))
	}))
	t.Cleanup(ts.Close)
	n := New().(*notion)
	v1 := newV1Backend("test-token")
	v1.baseURL = ts.URL
	v1.client = ts.Client()
	require.NoError(t, v1.configure(context.Background()))
	n.v1 = v1
	return n
}

func TestV1FieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, v1FieldCompactionSpecs, "v1FieldCompactionSpecs should not be empty")
}

// TestV1FieldCompactionSpecs_NoDuplicateTools confirms the YAML loader
// kept every tool entry — parse losslessness check.
func TestV1FieldCompactionSpecs_NoDuplicateTools(t *testing.T) {
	var sf compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactV1YAML, &sf))
	// Multi-view tools register their default-view spec under Specs in
	// addition to Views, so Specs == Tools always (Tools count == top-level
	// keys, regardless of which form they use).
	assert.Equal(t, len(sf.Tools), len(v1FieldCompactionSpecs))
}

func TestV1FieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for tool := range v1FieldCompactionSpecs {
		_, ok := dispatchV1[tool]
		assert.True(t, ok, "v1 compaction spec %s has no dispatch handler", tool)
	}
}

func TestV1FieldCompactionSpecs_NoMutationTools(t *testing.T) {
	mutationPrefixes := []string{"create", "update", "delete", "move", "append"}
	for toolName := range v1FieldCompactionSpecs {
		for _, prefix := range mutationPrefixes {
			assert.NotContains(t, string(toolName), "_"+prefix+"_",
				"mutation tool %q should not have a v1 field compaction spec", toolName)
		}
	}
}

// TestV1FieldCompactionSpecs_ParityWithV3 asserts every tool with a v3
// spec has a v1 spec, and vice versa. Without this, a tool added to
// compact.yaml but forgotten in compact_v1.yaml would silently fall
// through to "no compaction" on the OAuth backend — losing the savings
// users on Notion v1 are paying for.
func TestV1FieldCompactionSpecs_ParityWithV3(t *testing.T) {
	for tool := range fieldCompactionSpecs {
		_, ok := v1FieldCompactionSpecs[tool]
		assert.True(t, ok, "tool %s has a v3 spec but no v1 spec in compact_v1.yaml", tool)
	}
	for tool := range v1FieldCompactionSpecs {
		_, ok := fieldCompactionSpecs[tool]
		assert.True(t, ok, "tool %s has a v1 spec but no v3 spec in compact.yaml", tool)
	}
}

// TestCompactSpec_SwitchesOnBackend verifies CompactSpec routes to the
// v1 spec map when a v1 backend is configured. The two maps are *not*
// equal byte-for-byte (different JSON shapes mean different paths), so
// any field present in the v1 spec for a tool but absent from v3 (e.g.
// `results[].properties.title` on search, which uses different parent
// keys in v3) is enough to distinguish them.
func TestCompactSpec_SwitchesOnBackend(t *testing.T) {
	n3 := New().(*notion) // v1 == nil → v3 specs
	n1 := configuredV1(t) // v1 != nil → v1 specs

	v3Spec, ok := n3.CompactSpec("notion_search")
	require.True(t, ok)
	v1Spec, ok := n1.CompactSpec("notion_search")
	require.True(t, ok)

	// Same tool, different specs.
	assert.NotEqual(t, v3Spec, v1Spec, "v1 and v3 CompactSpec should return distinct specs for the same tool")
}

func TestViews_SwitchesOnBackend(t *testing.T) {
	n3 := New().(*notion)
	n1 := configuredV1(t)

	// notion_search has views in both files; the spec contents differ.
	v3Views, ok3 := n3.Views("notion_search")
	require.True(t, ok3)
	v1Views, ok1 := n1.Views("notion_search")
	require.True(t, ok1)
	assert.NotEqual(t, v3Views.Views["titles"].Spec, v1Views.Views["titles"].Spec,
		"v1 and v3 should publish distinct titles-view specs")
}
