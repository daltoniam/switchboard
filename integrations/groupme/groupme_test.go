package groupme

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *groupme {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	g := New().(*groupme)
	require.NoError(t, g.Configure(context.Background(), mcp.Credentials{"access_token": "test-token", "base_url": ts.URL}))
	return g
}

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "groupme", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	g := New().(*groupme)
	require.NoError(t, g.Configure(context.Background(), mcp.Credentials{"access_token": " test-token "}))
	assert.Equal(t, "test-token", g.token)
	assert.Equal(t, "https://api.groupme.com/v3", g.baseURL)
}

func TestConfigure_MissingAccessToken(t *testing.T) {
	err := New().Configure(context.Background(), mcp.Credentials{"access_token": ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access_token is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	g := New().(*groupme)
	require.NoError(t, g.Configure(context.Background(), mcp.Credentials{"access_token": "t", "base_url": "https://example.test/v3/"}))
	assert.Equal(t, "https://example.test/v3", g.baseURL)
}

func TestTools_Metadata(t *testing.T) {
	seen := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
		assert.True(t, strings.HasPrefix(string(tool.Name), "groupme_"), "tool %s missing groupme_ prefix", tool.Name)
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		assert.True(t, tool.Access.Declared(), "tool %s has no access class", tool.Name)
		seen[tool.Name] = true
	}
}

func TestTools_EntryPointSaysStartHere(t *testing.T) {
	for _, tool := range New().Tools() {
		if tool.Name == "groupme_list_groups" {
			assert.Contains(t, tool.Description, "Start here")
			return
		}
	}
	t.Fatal("groupme_list_groups not defined")
}

func TestTools_SendToolsAreMarkedWrites(t *testing.T) {
	writes := map[mcp.ToolName]bool{
		"groupme_send_group_message":  true,
		"groupme_send_direct_message": true,
		"groupme_post_bot_message":    true,
	}
	for _, tool := range New().Tools() {
		if writes[tool.Name] {
			assert.Equal(t, mcp.AccessWrite, tool.Access, "%s must be a write", tool.Name)
			assert.Contains(t, tool.Description, "WRITE:", "%s must say it sends a real message", tool.Name)
		} else {
			assert.Equal(t, mcp.AccessRead, tool.Access, "%s must be a read", tool.Name)
		}
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	g := New().(*groupme)
	require.NoError(t, g.Configure(context.Background(), mcp.Credentials{"access_token": "t"}))
	result, err := g.Execute(context.Background(), "groupme_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestExecute_NotConfigured(t *testing.T) {
	result, err := New().Execute(context.Background(), "groupme_list_groups", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "not configured")
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range New().Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	names := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		names[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, names[name], "dispatch handler %s has no tool definition", name)
	}
}

// --- HTTP helper ---

func TestDoRequest_SendsTokenInHeaderNotURL(t *testing.T) {
	g := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-token", r.Header.Get("X-Access-Token"))
		assert.NotContains(t, r.URL.RawQuery, "token")
		_, _ = io.WriteString(w, `{"response":{"id":"1"},"meta":{"code":200}}`)
	})
	data, err := g.doRequest(context.Background(), http.MethodGet, "/users/me", nil, nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":"1"}`, string(data))
}

func TestDoRequest_APIErrorUsesMetaErrors(t *testing.T) {
	g := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"response":null,"meta":{"code":401,"errors":["unauthorized"]}}`)
	})
	_, err := g.doRequest(context.Background(), http.MethodGet, "/users/me", nil, nil)
	require.Error(t, err)
	assert.False(t, mcp.IsRetryable(err))
	assert.Equal(t, "groupme API error (401): unauthorized", err.Error())
}

func TestDoRequest_NotModifiedMeansNoMessages(t *testing.T) {
	g := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	})
	data, err := g.doRequest(context.Background(), http.MethodGet, "/groups/1/messages", nil, nil)
	require.NoError(t, err)
	assert.Nil(t, data)
}

func TestDoRequest_EmptySuccessBody(t *testing.T) {
	g := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	data, err := g.doRequest(context.Background(), http.MethodPost, "/bots/post", nil, map[string]any{"bot_id": "b"})
	require.NoError(t, err)
	assert.Nil(t, data)
}

func TestDoRequest_DoesNotFollowRedirects(t *testing.T) {
	leaked := false
	other := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		leaked = r.Header.Get("X-Access-Token") != ""
	}))
	t.Cleanup(other.Close)
	g := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/steal", http.StatusFound)
	})
	_, err := g.doRequest(context.Background(), http.MethodGet, "/users/me", nil, nil)
	require.Error(t, err)
	assert.False(t, leaked, "token must not follow a redirect")
}

func TestDoRequest_Retryability(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		status    int
		retryable bool
	}{
		{"read rate limited", http.MethodGet, http.StatusTooManyRequests, true},
		{"read enhance your calm", http.MethodGet, 420, true},
		{"read server error", http.MethodGet, http.StatusBadGateway, true},
		{"send rate limited", http.MethodPost, http.StatusTooManyRequests, true},
		{"send enhance your calm", http.MethodPost, 420, true},
		// A send that failed with 5xx may still have been delivered; retrying
		// it would post the message twice.
		{"send server error", http.MethodPost, http.StatusBadGateway, false},
		{"bad request", http.MethodGet, http.StatusBadRequest, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
			})
			_, err := g.doRequest(context.Background(), tc.method, "/x", nil, nil)
			require.Error(t, err)
			assert.Equal(t, tc.retryable, mcp.IsRetryable(err))
		})
	}
}

// --- Handlers ---

type capturedRequest struct {
	method string
	path   string
	query  map[string]string
	body   map[string]any
}

func captureClient(t *testing.T, response string, got *capturedRequest) *groupme {
	t.Helper()
	return newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		got.method = r.Method
		got.path = r.URL.Path
		got.query = map[string]string{}
		for k := range r.URL.Query() {
			got.query[k] = r.URL.Query().Get(k)
		}
		got.body = nil
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			require.NoError(t, json.Unmarshal(raw, &got.body))
		}
		_, _ = io.WriteString(w, `{"response":`+response+`,"meta":{"code":200}}`)
	})
}

func TestHandlers_Requests(t *testing.T) {
	tests := []struct {
		tool      mcp.ToolName
		args      map[string]any
		method    string
		path      string
		query     map[string]string
		body      map[string]any
		response  string
		wantInOut string
	}{
		{tool: "groupme_get_me", method: "GET", path: "/users/me", query: map[string]string{}, response: `{"id":"9","name":"Me"}`, wantInOut: `"Me"`},
		{tool: "groupme_list_groups", args: map[string]any{}, method: "GET", path: "/groups", query: map[string]string{"omit": "memberships"}, response: `[{"id":"1"}]`, wantInOut: `"1"`},
		{tool: "groupme_list_groups", args: map[string]any{"page": float64(2), "per_page": float64(50)}, method: "GET", path: "/groups", query: map[string]string{"omit": "memberships", "page": "2", "per_page": "50"}, response: `[]`, wantInOut: `[]`},
		{tool: "groupme_get_group", args: map[string]any{"group_id": "123"}, method: "GET", path: "/groups/123", query: map[string]string{}, response: `{"id":"123"}`, wantInOut: `"123"`},
		{tool: "groupme_list_group_messages", args: map[string]any{"group_id": "123", "before_id": "55", "limit": float64(5)}, method: "GET", path: "/groups/123/messages", query: map[string]string{"before_id": "55", "limit": "5"}, response: `{"count":1,"messages":[{"id":"54"}]}`, wantInOut: `"54"`},
		{tool: "groupme_list_group_messages", args: map[string]any{"group_id": "123", "since_id": "55", "after_id": "50"}, method: "GET", path: "/groups/123/messages", query: map[string]string{"since_id": "55", "after_id": "50"}, response: `{"count":0,"messages":[]}`, wantInOut: `"messages"`},
		{tool: "groupme_list_chats", args: map[string]any{"per_page": float64(5)}, method: "GET", path: "/chats", query: map[string]string{"per_page": "5"}, response: `[{"other_user":{"id":"7"}}]`, wantInOut: `"7"`},
		{tool: "groupme_list_direct_messages", args: map[string]any{"other_user_id": "7", "before_id": "88"}, method: "GET", path: "/direct_messages", query: map[string]string{"other_user_id": "7", "before_id": "88"}, response: `{"count":1,"direct_messages":[{"id":"87"}]}`, wantInOut: `"87"`},
		{tool: "groupme_list_bots", method: "GET", path: "/bots", query: map[string]string{}, response: `[{"bot_id":"b1"}]`, wantInOut: `"b1"`},
		{tool: "groupme_send_group_message", args: map[string]any{"group_id": "123", "text": "hi", "source_guid": "g-1"}, method: "POST", path: "/groups/123/messages", query: map[string]string{},
			body: map[string]any{"message": map[string]any{"source_guid": "g-1", "text": "hi"}}, response: `{"message":{"id":"m1"}}`, wantInOut: `"m1"`},
		{tool: "groupme_send_direct_message", args: map[string]any{"recipient_id": "7", "text": "yo", "source_guid": "g-2"}, method: "POST", path: "/direct_messages", query: map[string]string{},
			body: map[string]any{"direct_message": map[string]any{"source_guid": "g-2", "recipient_id": "7", "text": "yo"}}, response: `{"direct_message":{"id":"d1"}}`, wantInOut: `"d1"`},
		{tool: "groupme_post_bot_message", args: map[string]any{"bot_id": "b1", "text": "beep", "picture_url": "https://i.groupme.com/x.png"}, method: "POST", path: "/bots/post", query: map[string]string{},
			body: map[string]any{"bot_id": "b1", "text": "beep", "picture_url": "https://i.groupme.com/x.png"}, response: `null`, wantInOut: `"sent"`},
	}
	for _, tc := range tests {
		t.Run(string(tc.tool), func(t *testing.T) {
			var got capturedRequest
			g := captureClient(t, tc.response, &got)
			result, err := g.Execute(context.Background(), tc.tool, tc.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			assert.Equal(t, tc.method, got.method)
			assert.Equal(t, tc.path, got.path)
			assert.Equal(t, tc.query, got.query)
			assert.Equal(t, tc.body, got.body)
			assert.Contains(t, result.Data, tc.wantInOut)
		})
	}
}

func TestListGroupMessages_NotModifiedReturnsEmptyPage(t *testing.T) {
	g := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	})
	result, err := g.Execute(context.Background(), "groupme_list_group_messages", map[string]any{"group_id": "1", "before_id": "2"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.JSONEq(t, `{"count":0,"messages":[]}`, result.Data)
}

func TestListDirectMessages_NotModifiedReturnsEmptyPage(t *testing.T) {
	g := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	})
	result, err := g.Execute(context.Background(), "groupme_list_direct_messages", map[string]any{"other_user_id": "1"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.JSONEq(t, `{"count":0,"direct_messages":[]}`, result.Data)
}

func TestSend_GeneratesSourceGUID(t *testing.T) {
	var guids []string
	g := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Message struct {
				SourceGUID string `json:"source_guid"`
			} `json:"message"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		guids = append(guids, body.Message.SourceGUID)
		_, _ = io.WriteString(w, `{"response":{"message":{"id":"m"}}}`)
	})
	for range 2 {
		result, err := g.Execute(context.Background(), "groupme_send_group_message", map[string]any{"group_id": "1", "text": "hi"})
		require.NoError(t, err)
		require.False(t, result.IsError)
	}
	require.Len(t, guids, 2)
	assert.NotEmpty(t, guids[0])
	assert.NotEqual(t, guids[0], guids[1])
}

func TestHandlers_RequiredArgs(t *testing.T) {
	tests := []struct {
		tool    mcp.ToolName
		args    map[string]any
		missing string
	}{
		{"groupme_get_group", map[string]any{}, "group_id"},
		{"groupme_list_group_messages", map[string]any{}, "group_id"},
		{"groupme_list_direct_messages", map[string]any{}, "other_user_id"},
		{"groupme_send_group_message", map[string]any{"text": "hi"}, "group_id"},
		{"groupme_send_group_message", map[string]any{"group_id": "1", "text": "  "}, "text"},
		{"groupme_send_direct_message", map[string]any{"text": "hi"}, "recipient_id"},
		{"groupme_send_direct_message", map[string]any{"recipient_id": "1"}, "text"},
		{"groupme_post_bot_message", map[string]any{"text": "hi"}, "bot_id"},
		{"groupme_post_bot_message", map[string]any{"bot_id": "b"}, "text"},
	}
	for _, tc := range tests {
		t.Run(string(tc.tool)+"/"+tc.missing, func(t *testing.T) {
			g := newTestClient(t, func(http.ResponseWriter, *http.Request) {
				t.Error("no request expected when a required argument is missing")
			})
			result, err := g.Execute(context.Background(), tc.tool, tc.args)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tc.missing+" is required")
		})
	}
}

func TestHandlers_EscapePathIDs(t *testing.T) {
	var escaped string
	g := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		escaped = r.URL.EscapedPath()
		_, _ = io.WriteString(w, `{"response":{}}`)
	})
	result, err := g.Execute(context.Background(), "groupme_get_group", map[string]any{"group_id": "1/../users/me"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Equal(t, "/groups/1%2F..%2Fusers%2Fme", escaped)
}

func TestHealthy(t *testing.T) {
	assert.False(t, New().Healthy(context.Background()), "unconfigured adapter is not healthy")

	ok := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/users/me", r.URL.Path)
		_, _ = io.WriteString(w, `{"response":{"id":"1"}}`)
	})
	assert.True(t, ok.Healthy(context.Background()))

	bad := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	assert.False(t, bad.Healthy(context.Background()))
}
