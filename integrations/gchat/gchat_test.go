package gchat

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
	assert.Equal(t, "gchat", i.Name())
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
	g := &gchat{client: &http.Client{}, baseURL: "https://chat.googleapis.com/v1"}
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

func TestTools_AllHaveGchatPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "gchat_", "tool %s missing gchat_ prefix", tool.Name)
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
		if tool.Name == mcp.ToolName("gchat_list_spaces") {
			assert.Contains(t, tool.Description, "Start here",
				"entry-point tool gchat_list_spaces must include 'Start here' for wayfinding")
			return
		}
	}
	t.Fatal("gchat_list_spaces tool not found")
}

// ── Dispatch parity ─────────────────────────────────────────────────

func TestExecute_UnknownTool(t *testing.T) {
	g := &gchat{accessToken: "test", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gchat_nonexistent", nil)
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
		_, _ = w.Write([]byte(`{"spaces":[]}`))
	}))
	defer ts.Close()

	g := &gchat{accessToken: "test-token", client: ts.Client(), baseURL: ts.URL}
	data, err := g.get(context.Background(), "/spaces")
	require.NoError(t, err)
	assert.Contains(t, string(data), "spaces")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":{"message":"Not Found"}}`))
	}))
	defer ts.Close()

	g := &gchat{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.get(context.Background(), "/spaces/missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gchat API error (404)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	g := &gchat{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := g.delete(context.Background(), "/spaces/x/messages/y")
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestDoRequest_5xxIsRetryable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(503)
		_, _ = w.Write([]byte(`{"error":{"message":"Service Unavailable"}}`))
	}))
	defer ts.Close()

	g := &gchat{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.get(context.Background(), "/spaces")
	require.Error(t, err)
	re, ok := err.(*mcp.RetryableError)
	require.True(t, ok, "expected mcp.RetryableError")
	assert.Equal(t, 503, re.StatusCode)
}

// ── Handler: listSpaces ─────────────────────────────────────────────

func TestListSpaces(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/spaces", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "50", q.Get("pageSize"))
		assert.Equal(t, "tok-2", q.Get("pageToken"))
		assert.Equal(t, `space_type = "SPACE"`, q.Get("filter"))
		_, _ = w.Write([]byte(`{"spaces":[{"name":"spaces/A","displayName":"Eng"}]}`))
	}))
	defer ts.Close()

	g := &gchat{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gchat_list_spaces", map[string]any{
		"page_size":  50,
		"page_token": "tok-2",
		"filter":     `space_type = "SPACE"`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Eng")
}

// ── Handler: getSpace ───────────────────────────────────────────────

func TestGetSpace(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/spaces/AAQAtuk0o-A", r.URL.Path)
		_, _ = w.Write([]byte(`{"name":"spaces/AAQAtuk0o-A","displayName":"General"}`))
	}))
	defer ts.Close()

	g := &gchat{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gchat_get_space", map[string]any{
		"space_id": "AAQAtuk0o-A",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "General")
}

func TestGetSpace_AcceptsResourceName(t *testing.T) {
	// "spaces/AAQAtuk0o-A" should be normalized to "AAQAtuk0o-A" and not
	// produce a path like /spaces/spaces%2FAAQA...
	var seenPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	g := &gchat{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gchat_get_space", map[string]any{
		"space_id": "spaces/AAQAtuk0o-A",
	})
	require.NoError(t, err)
	assert.Equal(t, "/spaces/AAQAtuk0o-A", seenPath)
}

func TestGetSpace_MissingID(t *testing.T) {
	g := &gchat{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gchat_get_space", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "space_id is required")
}

// ── Handler: listMessages ───────────────────────────────────────────

func TestListMessages(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/spaces/AAA/messages", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "25", q.Get("pageSize"))
		assert.Equal(t, "page-2", q.Get("pageToken"))
		assert.Equal(t, "createTime desc", q.Get("orderBy"))
		assert.Equal(t, "true", q.Get("showDeleted"))
		assert.Equal(t, `createTime > "2024-01-01T00:00:00Z"`, q.Get("filter"))
		_, _ = w.Write([]byte(`{"messages":[]}`))
	}))
	defer ts.Close()

	g := &gchat{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gchat_list_messages", map[string]any{
		"space_id":     "AAA",
		"page_size":    25,
		"page_token":   "page-2",
		"order_by":     "createTime desc",
		"show_deleted": true,
		"filter":       `createTime > "2024-01-01T00:00:00Z"`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListMessages_MissingID(t *testing.T) {
	g := &gchat{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gchat_list_messages", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "space_id is required")
}

// ── Handler: getMessage ─────────────────────────────────────────────

func TestGetMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		// Message IDs contain dots — should be passed through unescaped.
		assert.Equal(t, "/spaces/AAA/messages/UMOmwAAAAAE.UMOmwAAAAAE", r.URL.Path)
		_, _ = w.Write([]byte(`{"name":"spaces/AAA/messages/UMOmwAAAAAE.UMOmwAAAAAE","text":"hi"}`))
	}))
	defer ts.Close()

	g := &gchat{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gchat_get_message", map[string]any{
		"space_id":   "AAA",
		"message_id": "UMOmwAAAAAE.UMOmwAAAAAE",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "hi")
}

func TestGetMessage_MissingArgs(t *testing.T) {
	g := &gchat{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	r1, _ := g.Execute(context.Background(), "gchat_get_message", map[string]any{"message_id": "x"})
	assert.True(t, r1.IsError)
	assert.Contains(t, r1.Data, "space_id is required")
	r2, _ := g.Execute(context.Background(), "gchat_get_message", map[string]any{"space_id": "AAA"})
	assert.True(t, r2.IsError)
	assert.Contains(t, r2.Data, "message_id is required")
}

// ── Handler: createMessage ──────────────────────────────────────────

func TestCreateMessage_Text(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/spaces/AAA/messages", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD", q.Get("messageReplyOption"))
		assert.Equal(t, "client-custom-1", q.Get("messageId"))

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Hello, team!", body["text"])
		thread, ok := body["thread"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "release-2024-06", thread["threadKey"])
		_, _ = w.Write([]byte(`{"name":"spaces/AAA/messages/m1","text":"Hello, team!"}`))
	}))
	defer ts.Close()

	g := &gchat{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gchat_create_message", map[string]any{
		"space_id":             "AAA",
		"text":                 "Hello, team!",
		"thread_key":           "release-2024-06",
		"message_reply_option": "REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD",
		"message_id":           "client-custom-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Hello")
}

func TestCreateMessage_CardsV2(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		cards, ok := body["cardsV2"].([]any)
		require.True(t, ok)
		assert.Len(t, cards, 1)
		_, _ = w.Write([]byte(`{"name":"spaces/AAA/messages/m2"}`))
	}))
	defer ts.Close()

	g := &gchat{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gchat_create_message", map[string]any{
		"space_id": "AAA",
		"cards_v2": `[{"card":{"header":{"title":"Hi"}}}]`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCreateMessage_InvalidCardsV2(t *testing.T) {
	g := &gchat{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gchat_create_message", map[string]any{
		"space_id": "AAA",
		"cards_v2": `not-json`,
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "cards_v2 is not valid JSON")
}

func TestCreateMessage_MissingArgs(t *testing.T) {
	g := &gchat{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	r1, _ := g.Execute(context.Background(), "gchat_create_message", map[string]any{"text": "hi"})
	assert.True(t, r1.IsError)
	assert.Contains(t, r1.Data, "space_id is required")
	r2, _ := g.Execute(context.Background(), "gchat_create_message", map[string]any{"space_id": "AAA"})
	assert.True(t, r2.IsError)
	assert.Contains(t, r2.Data, "text or cards_v2 is required")
}

// ── Handler: updateMessage ──────────────────────────────────────────

func TestUpdateMessage_TextOnly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/spaces/AAA/messages/m1", r.URL.Path)
		assert.Equal(t, "text", r.URL.Query().Get("updateMask"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Updated!", body["text"])
		_, ok := body["cardsV2"]
		assert.False(t, ok, "cardsV2 should not be in body when only text passed")
		_, _ = w.Write([]byte(`{"name":"spaces/AAA/messages/m1","text":"Updated!"}`))
	}))
	defer ts.Close()

	g := &gchat{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gchat_update_message", map[string]any{
		"space_id":   "AAA",
		"message_id": "m1",
		"text":       "Updated!",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestUpdateMessage_BothFields(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mask := r.URL.Query().Get("updateMask")
		assert.True(t, mask == "text,cardsV2" || mask == "cardsV2,text",
			"updateMask should include both text and cardsV2; got %q", mask)
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), "cardsV2")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	g := &gchat{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gchat_update_message", map[string]any{
		"space_id":   "AAA",
		"message_id": "m1",
		"text":       "Edit",
		"cards_v2":   `[{"card":{}}]`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestUpdateMessage_NoFields(t *testing.T) {
	g := &gchat{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gchat_update_message", map[string]any{
		"space_id":   "AAA",
		"message_id": "m1",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "text or cards_v2 is required")
}

func TestUpdateMessage_MissingArgs(t *testing.T) {
	g := &gchat{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	r1, _ := g.Execute(context.Background(), "gchat_update_message", map[string]any{"message_id": "m"})
	assert.True(t, r1.IsError)
	assert.Contains(t, r1.Data, "space_id is required")
	r2, _ := g.Execute(context.Background(), "gchat_update_message", map[string]any{"space_id": "AAA"})
	assert.True(t, r2.IsError)
	assert.Contains(t, r2.Data, "message_id is required")
}

// ── Handler: deleteMessage ──────────────────────────────────────────

func TestDeleteMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/spaces/AAA/messages/m1", r.URL.Path)
		assert.Equal(t, "", r.URL.Query().Get("force"))
		w.WriteHeader(204)
	}))
	defer ts.Close()

	g := &gchat{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gchat_delete_message", map[string]any{
		"space_id":   "AAA",
		"message_id": "m1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestDeleteMessage_Force(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "true", r.URL.Query().Get("force"))
		w.WriteHeader(204)
	}))
	defer ts.Close()

	g := &gchat{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gchat_delete_message", map[string]any{
		"space_id":   "AAA",
		"message_id": "m1",
		"force":      true,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// ── Handler: listMembers ────────────────────────────────────────────

func TestListMembers(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/spaces/AAA/members", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "50", q.Get("pageSize"))
		assert.Equal(t, "true", q.Get("showInvited"))
		assert.Equal(t, `member.type = "HUMAN"`, q.Get("filter"))
		_, _ = w.Write([]byte(`{"memberships":[]}`))
	}))
	defer ts.Close()

	g := &gchat{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gchat_list_members", map[string]any{
		"space_id":     "AAA",
		"page_size":    50,
		"show_invited": true,
		"filter":       `member.type = "HUMAN"`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListMembers_MissingID(t *testing.T) {
	g := &gchat{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gchat_list_members", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "space_id is required")
}

// ── Healthy ────────────────────────────────────────────────────────

func TestHealthy_TrueOn200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"spaces":[]}`))
	}))
	defer ts.Close()

	g := &gchat{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	assert.True(t, g.Healthy(context.Background()))
}

func TestHealthy_FalseOn401(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
	}))
	defer ts.Close()

	g := &gchat{accessToken: "bad", client: ts.Client(), baseURL: ts.URL}
	assert.False(t, g.Healthy(context.Background()))
}

// TestHealthy_TrueAfterRefresh verifies that an expired access token does
// not flip the health badge red so long as the refresh credentials are
// configured: the 401 from the API should trigger a transparent refresh
// through g.get() and the retried call should succeed.
func TestHealthy_TrueAfterRefresh(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		assert.Equal(t, "refresh_token", vals.Get("grant_type"))
		assert.Equal(t, "rtok", vals.Get("refresh_token"))
		_, _ = w.Write([]byte(`{"access_token":"new-token","expires_in":3600}`))
	}))
	defer tokenSrv.Close()
	googleoauth.SetTokenURLForTest(t, tokenSrv.URL)

	calls := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") == "Bearer expired" {
			w.WriteHeader(401)
			return
		}
		_, _ = w.Write([]byte(`{"spaces":[]}`))
	}))
	defer api.Close()

	g := &gchat{
		accessToken:  "expired",
		refreshToken: "rtok",
		clientID:     "cid",
		clientSecret: "csec",
		client:       api.Client(),
		baseURL:      api.URL,
	}
	assert.True(t, g.Healthy(context.Background()))
	assert.Equal(t, "new-token", g.accessToken)
	assert.Equal(t, 2, calls, "expected initial 401 + retried 200")
}

// ── Path escaping ───────────────────────────────────────────────────

func TestSpaceIDIsURLEscaped(t *testing.T) {
	var seenPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	g := &gchat{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gchat_get_space", map[string]any{
		"space_id": "id with space/x",
	})
	require.NoError(t, err)
	assert.True(t, strings.Contains(seenPath, "id%20with%20space") || strings.Contains(seenPath, "id+with+space"),
		"space id with space should be URL-escaped; got %s", seenPath)
}

// ── Helpers ─────────────────────────────────────────────────────────

func TestNormalizeSpaceID(t *testing.T) {
	assert.Equal(t, "AAA", normalizeSpaceID("AAA"))
	assert.Equal(t, "AAA", normalizeSpaceID("spaces/AAA"))
	assert.Equal(t, "", normalizeSpaceID(""))
}

func TestParseCardsV2_String(t *testing.T) {
	got, err := parseCardsV2(`[{"card":{"header":{"title":"X"}}}]`)
	require.NoError(t, err)
	arr, ok := got.([]any)
	require.True(t, ok)
	assert.Len(t, arr, 1)
}

func TestParseCardsV2_Invalid(t *testing.T) {
	_, err := parseCardsV2("not-json")
	assert.Error(t, err)
}

func TestParseCardsV2_EmptyString(t *testing.T) {
	got, err := parseCardsV2("")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestParseCardsV2_Nil(t *testing.T) {
	got, err := parseCardsV2(nil)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestParseCardsV2_AlreadyParsed(t *testing.T) {
	in := []any{map[string]any{"card": map[string]any{}}}
	got, err := parseCardsV2(in)
	require.NoError(t, err)
	assert.Equal(t, in, got)
}

// ── PlainTextKeys ──────────────────────────────────────────────────

func TestPlainTextKeys(t *testing.T) {
	g := &gchat{}
	keys := g.PlainTextKeys()
	assert.Contains(t, keys, "base_url")
}
