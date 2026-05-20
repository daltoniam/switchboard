package gforms

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/googleoauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Constructor / config ────────────────────────────────────────────

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "gforms", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"access_token": "ya29.test-token"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAccessToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"access_token": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_token is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	g := &gforms{client: &http.Client{}, baseURL: "https://forms.googleapis.com/v1"}
	err := g.Configure(context.Background(), mcp.Credentials{
		"access_token": "ya29.test",
		"base_url":     "https://custom.example.com/",
	})
	assert.NoError(t, err)
	assert.Equal(t, "https://custom.example.com", g.baseURL)
}

// ── Tools metadata ──────────────────────────────────────────────────

func TestTools(t *testing.T) {
	i := New()
	defs := i.Tools()
	assert.NotEmpty(t, defs)
	for _, tool := range defs {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveGformsPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "gforms_", "tool %s missing gforms_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestTools_EntryPointHasStartHere(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		if tool.Name == mcp.ToolName("gforms_get_form") {
			assert.Contains(t, tool.Description, "Start here",
				"entry-point tool gforms_get_form must include 'Start here' for wayfinding")
			return
		}
	}
	t.Fatal("gforms_get_form tool not found")
}

// ── Dispatch parity ─────────────────────────────────────────────────

func TestExecute_UnknownTool(t *testing.T) {
	g := &gforms{accessToken: "test", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gforms_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	i := New()
	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

// ── Compaction parity ──────────────────────────────────────────────

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		_, ok := dispatch[name]
		assert.True(t, ok, "compaction spec %s has no dispatch handler", name)
	}
}

// ── HTTP helpers ────────────────────────────────────────────────────

func TestDoRequest_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"formId":"f-1","info":{"title":"Test"}}`))
	}))
	defer ts.Close()

	g := &gforms{accessToken: "test-token", client: ts.Client(), baseURL: ts.URL}
	data, err := g.get(context.Background(), "/forms/f-1")
	require.NoError(t, err)
	assert.Contains(t, string(data), "Test")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":{"message":"Not Found"}}`))
	}))
	defer ts.Close()

	g := &gforms{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.get(context.Background(), "/forms/missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gforms API error (404)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	g := &gforms{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := g.post(context.Background(), "/forms/x:batchUpdate", map[string]any{})
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

// ── Handler: getForm ───────────────────────────────────────────────

func TestGetForm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/forms/f-1", r.URL.Path)
		_, _ = w.Write([]byte(`{"formId":"f-1","info":{"title":"Hello"}}`))
	}))
	defer ts.Close()

	g := &gforms{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gforms_get_form", map[string]any{
		"form_id": "f-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Hello")
}

func TestGetForm_MissingID(t *testing.T) {
	g := &gforms{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gforms_get_form", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "form_id is required")
}

// ── Handler: createForm ────────────────────────────────────────────

func TestCreateForm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/forms", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		info, _ := body["info"].(map[string]any)
		assert.Equal(t, "My Survey", info["title"])
		assert.Equal(t, "Internal Survey", info["documentTitle"])
		_, _ = w.Write([]byte(`{"formId":"new-1","info":{"title":"My Survey"}}`))
	}))
	defer ts.Close()

	g := &gforms{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gforms_create_form", map[string]any{
		"title":          "My Survey",
		"document_title": "Internal Survey",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "new-1")
}

func TestCreateForm_MissingTitle(t *testing.T) {
	g := &gforms{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gforms_create_form", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "title is required")
}

// ── Handler: batchUpdate ───────────────────────────────────────────

func TestBatchUpdate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/forms/f-1:batchUpdate", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		reqs, _ := body["requests"].([]any)
		assert.Len(t, reqs, 1)
		assert.Equal(t, true, body["includeFormInResponse"])
		wc, _ := body["writeControl"].(map[string]any)
		assert.Equal(t, "rev-99", wc["requiredRevisionId"])
		_, _ = w.Write([]byte(`{"replies":[{}],"form":{"formId":"f-1"}}`))
	}))
	defer ts.Close()

	g := &gforms{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gforms_batch_update", map[string]any{
		"form_id":                  "f-1",
		"requests":                 `[{"createItem":{}}]`,
		"include_form_in_response": true,
		"write_control_revision":   "rev-99",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestBatchUpdate_TargetRevisionOverride(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		wc, _ := body["writeControl"].(map[string]any)
		assert.Equal(t, "target-1", wc["targetRevisionId"])
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	g := &gforms{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gforms_batch_update", map[string]any{
		"form_id":                       "f-1",
		"requests":                      `[{"createItem":{}}]`,
		"write_control_target_revision": "target-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestBatchUpdate_BadJSON(t *testing.T) {
	g := &gforms{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gforms_batch_update", map[string]any{
		"form_id":  "f-1",
		"requests": "not-json",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "JSON array")
}

func TestBatchUpdate_MissingRequests(t *testing.T) {
	g := &gforms{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gforms_batch_update", map[string]any{
		"form_id": "f-1",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "requests is required")
}

// ── Handler: listResponses ─────────────────────────────────────────

func TestListResponses(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/forms/f-1/responses", r.URL.Path)
		assert.Equal(t, "100", r.URL.Query().Get("pageSize"))
		assert.Equal(t, "next-token", r.URL.Query().Get("pageToken"))
		assert.Equal(t, "timestamp > 2024-01-01T00:00:00Z", r.URL.Query().Get("filter"))
		_, _ = w.Write([]byte(`{"responses":[],"nextPageToken":""}`))
	}))
	defer ts.Close()

	g := &gforms{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gforms_list_responses", map[string]any{
		"form_id":    "f-1",
		"page_size":  100,
		"page_token": "next-token",
		"filter":     "timestamp > 2024-01-01T00:00:00Z",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListResponses_MissingID(t *testing.T) {
	g := &gforms{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gforms_list_responses", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "form_id is required")
}

// ── Handler: getResponse ───────────────────────────────────────────

func TestGetResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/forms/f-1/responses/r-1", r.URL.Path)
		_, _ = w.Write([]byte(`{"responseId":"r-1"}`))
	}))
	defer ts.Close()

	g := &gforms{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gforms_get_response", map[string]any{
		"form_id":     "f-1",
		"response_id": "r-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "r-1")
}

func TestGetResponse_MissingArgs(t *testing.T) {
	g := &gforms{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	r1, _ := g.Execute(context.Background(), "gforms_get_response", map[string]any{"response_id": "x"})
	assert.True(t, r1.IsError)
	assert.Contains(t, r1.Data, "form_id is required")
	r2, _ := g.Execute(context.Background(), "gforms_get_response", map[string]any{"form_id": "f"})
	assert.True(t, r2.IsError)
	assert.Contains(t, r2.Data, "response_id is required")
}

// ── Healthy ────────────────────────────────────────────────────────

func TestHealthy_TrueOn404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
	}))
	defer ts.Close()

	g := &gforms{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	assert.True(t, g.Healthy(context.Background()))
}

func TestHealthy_FalseOn401(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
	}))
	defer ts.Close()

	g := &gforms{accessToken: "bad", client: ts.Client(), baseURL: ts.URL}
	assert.False(t, g.Healthy(context.Background()))
}

// TestHealthy_TrueAfterRefresh verifies that an expired access token does
// not flip the health badge red so long as refresh credentials are
// configured: the 401 from the API triggers a transparent refresh
// through g.get() and the retried sentinel probe still allows a 404.
func TestHealthy_TrueAfterRefresh(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		assert.Equal(t, "refresh_token", vals.Get("grant_type"))
		_, _ = w.Write([]byte(`{"access_token":"new-token","expires_in":3600}`))
	}))
	defer tokenSrv.Close()
	googleoauth.SetTokenURLForTest(t, tokenSrv.URL)

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer expired" {
			w.WriteHeader(401)
			return
		}
		// Sentinel-probe path: 404 means auth worked, probe doc absent.
		w.WriteHeader(404)
	}))
	defer api.Close()

	g := &gforms{
		accessToken:  "expired",
		refreshToken: "rtok",
		clientID:     "cid",
		clientSecret: "csec",
		client:       api.Client(),
		baseURL:      api.URL,
	}
	assert.True(t, g.Healthy(context.Background()))
	assert.Equal(t, "new-token", g.accessToken)
}

// ── Path escaping ───────────────────────────────────────────────────

func TestFormIDIsURLEscaped(t *testing.T) {
	var seenPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"formId":"foo bar"}`))
	}))
	defer ts.Close()

	g := &gforms{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gforms_get_form", map[string]any{
		"form_id": "foo bar/baz",
	})
	require.NoError(t, err)
	assert.True(t, strings.Contains(seenPath, "foo%20bar") || strings.Contains(seenPath, "foo+bar"),
		"form id with space should be URL-escaped; got %s", seenPath)
}

func TestResponseIDIsURLEscaped(t *testing.T) {
	var seenPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"responseId":"x"}`))
	}))
	defer ts.Close()

	g := &gforms{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gforms_get_response", map[string]any{
		"form_id":     "f-1",
		"response_id": "resp with space",
	})
	require.NoError(t, err)
	assert.True(t, strings.Contains(seenPath, "resp%20with%20space"),
		"response id with space should be URL-escaped; got %s", seenPath)
}
