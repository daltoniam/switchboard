package gslides

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
	assert.Equal(t, "gslides", i.Name())
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
	g := &gslides{client: &http.Client{}, baseURL: "https://slides.googleapis.com/v1"}
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

func TestTools_AllHaveGslidesPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "gslides_", "tool %s missing gslides_ prefix", tool.Name)
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
		if tool.Name == mcp.ToolName("gslides_get_presentation") {
			assert.Contains(t, tool.Description, "Start here",
				"entry-point tool gslides_get_presentation must include 'Start here' for wayfinding")
			return
		}
	}
	t.Fatal("gslides_get_presentation tool not found")
}

// ── Dispatch parity ─────────────────────────────────────────────────

func TestExecute_UnknownTool(t *testing.T) {
	g := &gslides{accessToken: "test", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gslides_nonexistent", nil)
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
		_, _ = w.Write([]byte(`{"presentationId":"p-1","title":"Test"}`))
	}))
	defer ts.Close()

	g := &gslides{accessToken: "test-token", client: ts.Client(), baseURL: ts.URL}
	data, err := g.get(context.Background(), "/presentations/p-1")
	require.NoError(t, err)
	assert.Contains(t, string(data), "Test")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":{"message":"Not Found"}}`))
	}))
	defer ts.Close()

	g := &gslides{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.get(context.Background(), "/presentations/missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gslides API error (404)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	g := &gslides{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := g.post(context.Background(), "/presentations/x:batchUpdate", map[string]any{})
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

// ── Handler: getPresentation ───────────────────────────────────────

func TestGetPresentation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/presentations/p-1", r.URL.Path)
		assert.Equal(t, "slides.objectId,title", r.URL.Query().Get("fields"))
		_, _ = w.Write([]byte(`{"presentationId":"p-1","title":"Hello"}`))
	}))
	defer ts.Close()

	g := &gslides{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gslides_get_presentation", map[string]any{
		"presentation_id": "p-1",
		"fields":          "slides.objectId,title",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Hello")
}

func TestGetPresentation_MissingID(t *testing.T) {
	g := &gslides{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gslides_get_presentation", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "presentation_id is required")
}

// ── Handler: createPresentation ────────────────────────────────────

func TestCreatePresentation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/presentations", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "My Deck", body["title"])
		_, _ = w.Write([]byte(`{"presentationId":"new-1","title":"My Deck"}`))
	}))
	defer ts.Close()

	g := &gslides{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gslides_create_presentation", map[string]any{
		"title": "My Deck",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "new-1")
}

func TestCreatePresentation_NoTitle(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_, hasTitle := body["title"]
		assert.False(t, hasTitle, "no title field should be sent when title is empty")
		_, _ = w.Write([]byte(`{"presentationId":"new-2"}`))
	}))
	defer ts.Close()

	g := &gslides{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gslides_create_presentation", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// ── Handler: getPage ───────────────────────────────────────────────

func TestGetPage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/presentations/p-1/pages/slide-abc", r.URL.Path)
		_, _ = w.Write([]byte(`{"objectId":"slide-abc"}`))
	}))
	defer ts.Close()

	g := &gslides{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gslides_get_page", map[string]any{
		"presentation_id": "p-1",
		"page_object_id":  "slide-abc",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetPage_MissingArgs(t *testing.T) {
	g := &gslides{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	r1, _ := g.Execute(context.Background(), "gslides_get_page", map[string]any{"page_object_id": "x"})
	assert.True(t, r1.IsError)
	assert.Contains(t, r1.Data, "presentation_id is required")
	r2, _ := g.Execute(context.Background(), "gslides_get_page", map[string]any{"presentation_id": "p"})
	assert.True(t, r2.IsError)
	assert.Contains(t, r2.Data, "page_object_id is required")
}

// ── Handler: getPageThumbnail ──────────────────────────────────────

func TestGetPageThumbnail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/presentations/p-1/pages/slide-1/thumbnail", r.URL.Path)
		assert.Equal(t, "LARGE", r.URL.Query().Get("thumbnailProperties.thumbnailSize"))
		assert.Equal(t, "PNG", r.URL.Query().Get("thumbnailProperties.mimeType"))
		_, _ = w.Write([]byte(`{"contentUrl":"https://example.com/thumb.png","width":1600,"height":900}`))
	}))
	defer ts.Close()

	g := &gslides{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gslides_get_page_thumbnail", map[string]any{
		"presentation_id": "p-1",
		"page_object_id":  "slide-1",
		"thumbnail_size":  "LARGE",
		"mime_type":       "PNG",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "thumb.png")
}

// ── Handler: batchUpdate ───────────────────────────────────────────

func TestBatchUpdate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/presentations/p-1:batchUpdate", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		reqs, _ := body["requests"].([]any)
		assert.Len(t, reqs, 1)
		wc, _ := body["writeControl"].(map[string]any)
		assert.Equal(t, "rev-123", wc["requiredRevisionId"])
		_, _ = w.Write([]byte(`{"replies":[{}]}`))
	}))
	defer ts.Close()

	g := &gslides{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gslides_batch_update", map[string]any{
		"presentation_id":        "p-1",
		"requests":               `[{"createSlide":{}}]`,
		"write_control_revision": "rev-123",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestBatchUpdate_BadJSON(t *testing.T) {
	g := &gslides{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gslides_batch_update", map[string]any{
		"presentation_id": "p-1",
		"requests":        "not-json",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "JSON array")
}

func TestBatchUpdate_MissingRequests(t *testing.T) {
	g := &gslides{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gslides_batch_update", map[string]any{
		"presentation_id": "p-1",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "requests is required")
}

// ── Healthy ────────────────────────────────────────────────────────

func TestHealthy_TrueOn404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
	}))
	defer ts.Close()

	g := &gslides{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	assert.True(t, g.Healthy(context.Background()))
}

func TestHealthy_FalseOn401(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
	}))
	defer ts.Close()

	g := &gslides{accessToken: "bad", client: ts.Client(), baseURL: ts.URL}
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

	g := &gslides{
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

func TestPresentationIDIsURLEscaped(t *testing.T) {
	var seenPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"presentationId":"foo bar"}`))
	}))
	defer ts.Close()

	g := &gslides{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gslides_get_presentation", map[string]any{
		"presentation_id": "foo bar/baz",
	})
	require.NoError(t, err)
	assert.True(t, strings.Contains(seenPath, "foo%20bar") || strings.Contains(seenPath, "foo+bar"),
		"presentation id with space should be URL-escaped; got %s", seenPath)
}

func TestPageObjectIDIsURLEscaped(t *testing.T) {
	var seenPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"objectId":"x"}`))
	}))
	defer ts.Close()

	g := &gslides{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gslides_get_page", map[string]any{
		"presentation_id": "p-1",
		"page_object_id":  "slide one",
	})
	require.NoError(t, err)
	assert.True(t, strings.Contains(seenPath, "slide%20one"),
		"page id with space should be URL-escaped; got %s", seenPath)
}
