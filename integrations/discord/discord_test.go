package discord

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

func newTestClient(t *testing.T, handler http.HandlerFunc) *discord {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	d := New().(*discord)
	require.NoError(t, d.Configure(context.Background(), mcp.Credentials{"bot_token": "test-token", "base_url": ts.URL}))
	return d
}

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "discord", i.Name())
}

func TestConfigure(t *testing.T) {
	tests := []struct {
		name      string
		creds     mcp.Credentials
		wantToken string
		wantBase  string
		wantErr   string
	}{
		{name: "bare token", creds: mcp.Credentials{"bot_token": " abc.def "}, wantToken: "abc.def", wantBase: "https://discord.com/api/v10"},
		{name: "pasted with Bot prefix", creds: mcp.Credentials{"bot_token": "Bot abc.def"}, wantToken: "abc.def", wantBase: "https://discord.com/api/v10"},
		{name: "custom base url", creds: mcp.Credentials{"bot_token": "t", "base_url": "https://example.test/api/v10/"}, wantToken: "t", wantBase: "https://example.test/api/v10"},
		{name: "missing token", creds: mcp.Credentials{"bot_token": ""}, wantErr: "bot_token is required"},
		{name: "only prefix", creds: mcp.Credentials{"bot_token": "Bot "}, wantErr: "bot_token is required"},
		{name: "token with newline", creds: mcp.Credentials{"bot_token": "a\nb"}, wantErr: "must not contain"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := New().(*discord)
			err := d.Configure(context.Background(), tc.creds)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantToken, d.token)
			assert.Equal(t, tc.wantBase, d.baseURL)
		})
	}
}

func TestTools_Metadata(t *testing.T) {
	seen := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
		assert.True(t, strings.HasPrefix(string(tool.Name), "discord_"), "tool %s missing discord_ prefix", tool.Name)
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		assert.True(t, tool.Access.Declared(), "tool %s has no access class", tool.Name)
		seen[tool.Name] = true
	}
}

func TestTools_EntryPointSaysStartHere(t *testing.T) {
	for _, tool := range New().Tools() {
		if tool.Name == "discord_list_guilds" {
			assert.Contains(t, tool.Description, "Start here")
			return
		}
	}
	t.Fatal("discord_list_guilds not defined")
}

func TestTools_SendToolsAreMarkedWrites(t *testing.T) {
	writes := map[mcp.ToolName]bool{
		"discord_send_channel_message": true,
		"discord_send_direct_message":  true,
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
	d := New().(*discord)
	require.NoError(t, d.Configure(context.Background(), mcp.Credentials{"bot_token": "t"}))
	result, err := d.Execute(context.Background(), "discord_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestExecute_NotConfigured(t *testing.T) {
	result, err := New().Execute(context.Background(), "discord_list_guilds", nil)
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

func TestDoRequest_SendsBotAuthAndUserAgent(t *testing.T) {
	d := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bot test-token", r.Header.Get("Authorization"))
		assert.True(t, strings.HasPrefix(r.Header.Get("User-Agent"), "DiscordBot (https://github.com/daltoniam/switchboard, "), r.Header.Get("User-Agent"))
		_, _ = io.WriteString(w, `{"id":"1"}`)
	})
	data, err := d.doRequest(context.Background(), http.MethodGet, "/users/@me", nil, nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":"1"}`, string(data))
}

func TestDoRequest_APIErrorUsesMessageAndCode(t *testing.T) {
	d := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"message":"Missing Access","code":50001}`)
	})
	_, err := d.doRequest(context.Background(), http.MethodGet, "/guilds/1/channels", nil, nil)
	require.Error(t, err)
	assert.False(t, mcp.IsRetryable(err))
	assert.Equal(t, "discord API error (403): Missing Access (code 50001)", err.Error())
}

func TestDoRequest_NoContent(t *testing.T) {
	d := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	data, err := d.doRequest(context.Background(), http.MethodPost, "/x", nil, map[string]any{})
	require.NoError(t, err)
	assert.JSONEq(t, `{"status":"success"}`, string(data))
}

func TestDoRequest_DoesNotFollowRedirects(t *testing.T) {
	reached := false
	other := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { reached = true }))
	t.Cleanup(other.Close)
	d := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/steal", http.StatusFound)
	})
	_, err := d.doRequest(context.Background(), http.MethodGet, "/users/@me", nil, nil)
	require.Error(t, err)
	assert.False(t, reached, "redirects must not be followed")
}

func TestDoRequest_RateLimitUsesRetryAfterBody(t *testing.T) {
	d := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"message":"You are being rate limited.","retry_after":1.5,"global":false}`)
	})
	_, err := d.doRequest(context.Background(), http.MethodGet, "/x", nil, nil)
	var re *mcp.RetryableError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, "1.5s", re.RetryAfter.String())
}

func TestDoRequest_Retryability(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		status    int
		retryable bool
	}{
		{"read rate limited", http.MethodGet, http.StatusTooManyRequests, true},
		{"read server error", http.MethodGet, http.StatusBadGateway, true},
		{"send rate limited", http.MethodPost, http.StatusTooManyRequests, true},
		// A send that failed with 5xx may still have been delivered; retrying
		// it would post the message twice.
		{"send server error", http.MethodPost, http.StatusBadGateway, false},
		{"unauthorized", http.MethodGet, http.StatusUnauthorized, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
			})
			_, err := d.doRequest(context.Background(), tc.method, "/x", nil, nil)
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

func TestHandlers_Requests(t *testing.T) {
	noPings := map[string]any{"parse": []any{"users"}}
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
		{tool: "discord_get_me", method: "GET", path: "/users/@me", query: map[string]string{}, response: `{"id":"9","username":"hermes"}`, wantInOut: `"hermes"`},
		{tool: "discord_list_guilds", args: map[string]any{}, method: "GET", path: "/users/@me/guilds", query: map[string]string{}, response: `[{"id":"1"}]`, wantInOut: `"1"`},
		{tool: "discord_list_guilds", args: map[string]any{"limit": float64(10), "after": "5"}, method: "GET", path: "/users/@me/guilds", query: map[string]string{"limit": "10", "after": "5"}, response: `[]`, wantInOut: `[]`},
		{tool: "discord_list_channels", args: map[string]any{"guild_id": "1"}, method: "GET", path: "/guilds/1/channels", query: map[string]string{}, response: `[{"id":"2","name":"general"}]`, wantInOut: `"general"`},
		{tool: "discord_list_channel_messages", args: map[string]any{"channel_id": "2", "before": "99", "limit": float64(5)}, method: "GET", path: "/channels/2/messages", query: map[string]string{"before": "99", "limit": "5"}, response: `[{"id":"98"}]`, wantInOut: `"98"`},
		{tool: "discord_list_channel_messages", args: map[string]any{"channel_id": "2", "after": "1", "around": "3"}, method: "GET", path: "/channels/2/messages", query: map[string]string{"after": "1", "around": "3"}, response: `[]`, wantInOut: `[]`},
		{tool: "discord_get_user", args: map[string]any{"user_id": "7"}, method: "GET", path: "/users/7", query: map[string]string{}, response: `{"id":"7","username":"ann"}`, wantInOut: `"ann"`},
		{tool: "discord_send_channel_message", args: map[string]any{"channel_id": "2", "content": "hi"}, method: "POST", path: "/channels/2/messages", query: map[string]string{},
			body: map[string]any{"content": "hi", "allowed_mentions": noPings}, response: `{"id":"m1","channel_id":"2"}`, wantInOut: `"m1"`},
		{tool: "discord_send_channel_message", args: map[string]any{"channel_id": "2", "content": "re", "reply_to_message_id": "98"}, method: "POST", path: "/channels/2/messages", query: map[string]string{},
			body: map[string]any{"content": "re", "allowed_mentions": noPings, "message_reference": map[string]any{"message_id": "98", "fail_if_not_exists": false}}, response: `{"id":"m2"}`, wantInOut: `"m2"`},
	}
	for _, tc := range tests {
		t.Run(string(tc.tool), func(t *testing.T) {
			var got capturedRequest
			d := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				got = capture(t, r)
				_, _ = io.WriteString(w, tc.response)
			})
			result, err := d.Execute(context.Background(), tc.tool, tc.args)
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

func capture(t *testing.T, r *http.Request) capturedRequest {
	t.Helper()
	got := capturedRequest{method: r.Method, path: r.URL.Path, query: map[string]string{}}
	for k := range r.URL.Query() {
		got.query[k] = r.URL.Query().Get(k)
	}
	if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
		require.NoError(t, json.Unmarshal(raw, &got.body))
	}
	return got
}

func TestSendDirectMessage_OpensDMThenSends(t *testing.T) {
	var calls []capturedRequest
	d := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, capture(t, r))
		switch r.URL.Path {
		case "/users/@me/channels":
			_, _ = io.WriteString(w, `{"id":"555","type":1}`)
		case "/channels/555/messages":
			_, _ = io.WriteString(w, `{"id":"m9","channel_id":"555"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	result, err := d.Execute(context.Background(), "discord_send_direct_message", map[string]any{"recipient_id": "7", "content": "yo"})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	require.Len(t, calls, 2)
	assert.Equal(t, "POST", calls[0].method)
	assert.Equal(t, "/users/@me/channels", calls[0].path)
	assert.Equal(t, map[string]any{"recipient_id": "7"}, calls[0].body)
	assert.Equal(t, "/channels/555/messages", calls[1].path)
	assert.Equal(t, "yo", calls[1].body["content"])
	assert.Contains(t, result.Data, `"m9"`)
	assert.Contains(t, result.Data, `"555"`)
}

func TestSendDirectMessage_DMOpenFailureSendsNothing(t *testing.T) {
	var paths []string
	d := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"message":"Cannot send messages to this user","code":50007}`)
	})
	result, err := d.Execute(context.Background(), "discord_send_direct_message", map[string]any{"recipient_id": "7", "content": "yo"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "50007")
	assert.Equal(t, []string{"/users/@me/channels"}, paths)
}

func TestHandlers_RequiredArgs(t *testing.T) {
	tests := []struct {
		tool    mcp.ToolName
		args    map[string]any
		missing string
	}{
		{"discord_list_channels", map[string]any{}, "guild_id"},
		{"discord_list_channel_messages", map[string]any{}, "channel_id"},
		{"discord_get_user", map[string]any{}, "user_id"},
		{"discord_send_channel_message", map[string]any{"content": "hi"}, "channel_id"},
		{"discord_send_channel_message", map[string]any{"channel_id": "1", "content": " "}, "content"},
		{"discord_send_direct_message", map[string]any{"content": "hi"}, "recipient_id"},
		{"discord_send_direct_message", map[string]any{"recipient_id": "1"}, "content"},
	}
	for _, tc := range tests {
		t.Run(string(tc.tool)+"/"+tc.missing, func(t *testing.T) {
			d := newTestClient(t, func(http.ResponseWriter, *http.Request) {
				t.Error("no request expected when a required argument is missing")
			})
			result, err := d.Execute(context.Background(), tc.tool, tc.args)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tc.missing+" is required")
		})
	}
}

func TestHandlers_RejectNonSnowflakeIDs(t *testing.T) {
	d := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Error("no request expected for an invalid ID")
	})
	result, err := d.Execute(context.Background(), "discord_get_user", map[string]any{"user_id": "../guilds/1"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "user_id must be a numeric Discord ID")
}

func TestHealthy(t *testing.T) {
	assert.False(t, New().Healthy(context.Background()), "unconfigured adapter is not healthy")

	ok := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/users/@me", r.URL.Path)
		_, _ = io.WriteString(w, `{"id":"1","bot":true}`)
	})
	assert.True(t, ok.Healthy(context.Background()))

	bad := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	assert.False(t, bad.Healthy(context.Background()))
}
