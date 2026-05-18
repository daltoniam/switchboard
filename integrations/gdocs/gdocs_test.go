package gdocs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Constructor / config ────────────────────────────────────────────

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "gdocs", i.Name())
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
	g := &gdocs{client: &http.Client{}, baseURL: "https://docs.googleapis.com/v1"}
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
	tools := i.Tools()
	assert.NotEmpty(t, tools)
	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveGdocsPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "gdocs_", "tool %s missing gdocs_ prefix", tool.Name)
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
		if tool.Name == mcp.ToolName("gdocs_get_document") {
			assert.Contains(t, tool.Description, "Start here",
				"entry-point tool gdocs_get_document must include 'Start here' for wayfinding")
			return
		}
	}
	t.Fatal("gdocs_get_document tool not found")
}

// ── Dispatch parity ─────────────────────────────────────────────────

func TestExecute_UnknownTool(t *testing.T) {
	g := &gdocs{accessToken: "test", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gdocs_nonexistent", nil)
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
		_, _ = w.Write([]byte(`{"documentId":"doc-1","title":"Test"}`))
	}))
	defer ts.Close()

	g := &gdocs{accessToken: "test-token", client: ts.Client(), baseURL: ts.URL}
	data, err := g.get(context.Background(), "/documents/doc-1")
	require.NoError(t, err)
	assert.Contains(t, string(data), "Test")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":{"message":"Not Found"}}`))
	}))
	defer ts.Close()

	g := &gdocs{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.get(context.Background(), "/documents/missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gdocs API error (404)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	g := &gdocs{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := g.post(context.Background(), "/documents/x:batchUpdate", map[string]any{})
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

// ── Handler tests ───────────────────────────────────────────────────

func TestGetDocument(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/documents/doc-1", r.URL.Path)
		assert.Equal(t, "SUGGESTIONS_INLINE", r.URL.Query().Get("suggestionsViewMode"))
		_, _ = w.Write([]byte(`{"documentId":"doc-1","title":"Hello"}`))
	}))
	defer ts.Close()

	g := &gdocs{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdocs_get_document", map[string]any{
		"document_id":           "doc-1",
		"suggestions_view_mode": "SUGGESTIONS_INLINE",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "doc-1")
}

func TestGetDocument_MissingID(t *testing.T) {
	g := &gdocs{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gdocs_get_document", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "document_id is required")
}

func TestCreateDocument(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/documents", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "My Doc", body["title"])
		_, _ = w.Write([]byte(`{"documentId":"new-doc","title":"My Doc"}`))
	}))
	defer ts.Close()

	g := &gdocs{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdocs_create_document", map[string]any{
		"title": "My Doc",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "new-doc")
}

func TestCreateDocument_NoTitle(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_, hasTitle := body["title"]
		assert.False(t, hasTitle, "title should be omitted when empty")
		_, _ = w.Write([]byte(`{"documentId":"x"}`))
	}))
	defer ts.Close()

	g := &gdocs{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gdocs_create_document", map[string]any{})
	require.NoError(t, err)
}

func TestBatchUpdate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/documents/doc-1:batchUpdate", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		reqs, _ := body["requests"].([]any)
		assert.Len(t, reqs, 1)
		_, _ = w.Write([]byte(`{"documentId":"doc-1","replies":[{}]}`))
	}))
	defer ts.Close()

	g := &gdocs{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdocs_batch_update", map[string]any{
		"document_id": "doc-1",
		"requests":    `[{"insertText":{"location":{"index":1},"text":"Hi"}}]`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "replies")
}

func TestBatchUpdate_BadJSON(t *testing.T) {
	g := &gdocs{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gdocs_batch_update", map[string]any{
		"document_id": "doc-1",
		"requests":    `not json`,
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "JSON array")
}

func TestBatchUpdate_WriteControl(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		wc, _ := body["writeControl"].(map[string]any)
		assert.Equal(t, "rev-42", wc["requiredRevisionId"])
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	g := &gdocs{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gdocs_batch_update", map[string]any{
		"document_id":            "doc-1",
		"requests":               `[]`,
		"write_control_revision": "rev-42",
	})
	require.NoError(t, err)
}

func TestInsertText(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		reqs := body["requests"].([]any)
		require.Len(t, reqs, 1)
		req := reqs[0].(map[string]any)
		insert := req["insertText"].(map[string]any)
		loc := insert["location"].(map[string]any)
		assert.EqualValues(t, 5, loc["index"])
		assert.Equal(t, "Hello", insert["text"])
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	g := &gdocs{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdocs_insert_text", map[string]any{
		"document_id": "doc-1",
		"text":        "Hello",
		"index":       5,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestInsertText_InvalidIndex(t *testing.T) {
	g := &gdocs{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gdocs_insert_text", map[string]any{
		"document_id": "doc-1",
		"text":        "Hello",
		"index":       0,
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "index must be >= 1")
}

func TestAppendText(t *testing.T) {
	// First call is a GET to find endIndex; second is the batchUpdate.
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			assert.Equal(t, "GET", r.Method)
			_, _ = w.Write([]byte(`{"body":{"content":[{"endIndex":42}]}}`))
		case 2:
			assert.Equal(t, "POST", r.Method)
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			reqs := body["requests"].([]any)
			req := reqs[0].(map[string]any)
			insert := req["insertText"].(map[string]any)
			loc := insert["location"].(map[string]any)
			// endIndex 42 → insertAt 41
			assert.EqualValues(t, 41, loc["index"])
			// Default leading_newline=true, so payload starts with \n
			assert.Equal(t, "\nappended", insert["text"])
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer ts.Close()

	g := &gdocs{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdocs_append_text", map[string]any{
		"document_id": "doc-1",
		"text":        "appended",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, 2, calls)
}

func TestAppendText_NoLeadingNewline(t *testing.T) {
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			_, _ = w.Write([]byte(`{"body":{"content":[{"endIndex":10}]}}`))
		case 2:
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			req := body["requests"].([]any)[0].(map[string]any)
			insert := req["insertText"].(map[string]any)
			assert.Equal(t, "x", insert["text"])
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer ts.Close()

	g := &gdocs{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gdocs_append_text", map[string]any{
		"document_id":     "doc-1",
		"text":            "x",
		"leading_newline": "false",
	})
	require.NoError(t, err)
}

func TestReplaceText(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		req := body["requests"].([]any)[0].(map[string]any)
		replace := req["replaceAllText"].(map[string]any)
		contains := replace["containsText"].(map[string]any)
		assert.Equal(t, "old", contains["text"])
		assert.Equal(t, true, contains["matchCase"])
		assert.Equal(t, "new", replace["replaceText"])
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	g := &gdocs{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gdocs_replace_text", map[string]any{
		"document_id": "doc-1",
		"find":        "old",
		"replace":     "new",
		"match_case":  "true",
	})
	require.NoError(t, err)
}

func TestReplaceText_EmptyFind(t *testing.T) {
	g := &gdocs{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gdocs_replace_text", map[string]any{
		"document_id": "doc-1",
		"find":        "",
		"replace":     "new",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "find is required")
}

func TestDeleteContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		req := body["requests"].([]any)[0].(map[string]any)
		del := req["deleteContentRange"].(map[string]any)
		rng := del["range"].(map[string]any)
		assert.EqualValues(t, 5, rng["startIndex"])
		assert.EqualValues(t, 10, rng["endIndex"])
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	g := &gdocs{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gdocs_delete_content", map[string]any{
		"document_id": "doc-1",
		"start_index": 5,
		"end_index":   10,
	})
	require.NoError(t, err)
}

func TestDeleteContent_InvalidRange(t *testing.T) {
	g := &gdocs{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gdocs_delete_content", map[string]any{
		"document_id": "doc-1",
		"start_index": 10,
		"end_index":   5,
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "must be > start_index")
}

// ── Helper tests ────────────────────────────────────────────────────

func TestLastBodyEndIndex(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantIndex int
		wantErr   bool
	}{
		{
			name:      "single element",
			body:      `{"body":{"content":[{"endIndex":42}]}}`,
			wantIndex: 42,
		},
		{
			name:      "multiple elements picks max",
			body:      `{"body":{"content":[{"endIndex":10},{"endIndex":50},{"endIndex":30}]}}`,
			wantIndex: 50,
		},
		{
			name:    "empty body",
			body:    `{"body":{"content":[]}}`,
			wantErr: true,
		},
		{
			name:    "no endIndex",
			body:    `{"body":{"content":[{"startIndex":0}]}}`,
			wantErr: true,
		},
		{
			name:    "malformed json",
			body:    `not json`,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := lastBodyEndIndex([]byte(tc.body))
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantIndex, got)
		})
	}
}

// ── Plain text credentials ──────────────────────────────────────────

func TestPlainTextKeys(t *testing.T) {
	g := &gdocs{}
	keys := g.PlainTextKeys()
	assert.Contains(t, keys, "base_url")
}

// ── Healthy() ───────────────────────────────────────────────────────

func TestHealthy_TrueOn404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// 404 indicates the request reached the API and credentials worked.
		w.WriteHeader(404)
	}))
	defer ts.Close()

	g := &gdocs{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	assert.True(t, g.Healthy(context.Background()))
}

func TestHealthy_FalseOn401(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
	}))
	defer ts.Close()

	g := &gdocs{accessToken: "bad", client: ts.Client(), baseURL: ts.URL}
	assert.False(t, g.Healthy(context.Background()))
}

// ── Path escaping ───────────────────────────────────────────────────

func TestDocumentIDIsURLEscaped(t *testing.T) {
	var seenPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"documentId":"foo bar"}`))
	}))
	defer ts.Close()

	g := &gdocs{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gdocs_get_document", map[string]any{
		"document_id": "foo bar/baz",
	})
	require.NoError(t, err)
	// PathEscape preserves '/' but escapes spaces.
	assert.True(t, strings.Contains(seenPath, "foo%20bar") || strings.Contains(seenPath, "foo+bar"),
		"document id with space should be URL-escaped; got %s", seenPath)
}
