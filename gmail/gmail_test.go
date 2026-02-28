package gmail

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "gmail", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{"access_token": "ya29.test-token"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAccessToken(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{"access_token": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_token is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	g := &gmail{client: &http.Client{}, baseURL: "https://gmail.googleapis.com"}
	err := g.Configure(mcp.Credentials{
		"access_token": "ya29.test",
		"base_url":     "https://custom.example.com/",
	})
	assert.NoError(t, err)
	assert.Equal(t, "https://custom.example.com", g.baseURL)
}

func TestTools(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.NotEmpty(t, tools)

	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveGmailPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "gmail_", "tool %s missing gmail_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[string]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	g := &gmail{accessToken: "test", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gmail_nonexistent", nil)
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
	toolNames := make(map[string]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

// --- HTTP helper tests ---

func TestDoRequest_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"emailAddress":"test@gmail.com"}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "test-token", client: ts.Client(), baseURL: ts.URL}
	data, err := g.get(context.Background(), "/test")
	require.NoError(t, err)
	assert.Contains(t, string(data), "test@gmail.com")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":{"message":"Forbidden"}}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "bad-token", client: ts.Client(), baseURL: ts.URL}
	_, err := g.get(context.Background(), "/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gmail API error (403)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	data, err := g.doRequest(context.Background(), "DELETE", "/test", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestPost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "val", body["key"])
		_, _ = w.Write([]byte(`{"created":true}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	data, err := g.post(context.Background(), "/test", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "created")
}

func TestPatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		_, _ = w.Write([]byte(`{"updated":true}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	data, err := g.patch(context.Background(), "/test", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "updated")
}

func TestPut(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		_, _ = w.Write([]byte(`{"updated":true}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	data, err := g.put(context.Background(), "/test", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "updated")
}

// --- result helper tests ---

func TestRawResult(t *testing.T) {
	data := json.RawMessage(`{"key":"value"}`)
	result, err := rawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"key":"value"}`, result.Data)
}

func TestErrResult(t *testing.T) {
	result, err := errResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

// --- argument helper tests ---

func TestArgStr(t *testing.T) {
	assert.Equal(t, "val", argStr(map[string]any{"k": "val"}, "k"))
	assert.Empty(t, argStr(map[string]any{}, "k"))
}

func TestArgInt(t *testing.T) {
	assert.Equal(t, 42, argInt(map[string]any{"n": float64(42)}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": 42}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": "42"}, "n"))
	assert.Equal(t, 0, argInt(map[string]any{}, "n"))
}

func TestArgBool(t *testing.T) {
	assert.True(t, argBool(map[string]any{"b": true}, "b"))
	assert.False(t, argBool(map[string]any{"b": false}, "b"))
	assert.True(t, argBool(map[string]any{"b": "true"}, "b"))
	assert.False(t, argBool(map[string]any{}, "b"))
}

func TestArgStrSlice(t *testing.T) {
	result := argStrSlice(map[string]any{"ids": "a,b,c"}, "ids")
	assert.Equal(t, []string{"a", "b", "c"}, result)

	result = argStrSlice(map[string]any{"ids": "a, b, c"}, "ids")
	assert.Equal(t, []string{"a", "b", "c"}, result)

	assert.Nil(t, argStrSlice(map[string]any{}, "ids"))
}

func TestQueryEncode(t *testing.T) {
	t.Run("with values", func(t *testing.T) {
		result := queryEncode(map[string]string{"key": "val", "empty": ""})
		assert.Contains(t, result, "key=val")
		assert.NotContains(t, result, "empty")
		assert.True(t, result[0] == '?')
	})

	t.Run("all empty", func(t *testing.T) {
		result := queryEncode(map[string]string{"empty": ""})
		assert.Empty(t, result)
	})
}

func TestUser(t *testing.T) {
	assert.Equal(t, "me", user(map[string]any{}))
	assert.Equal(t, "user@example.com", user(map[string]any{"user_id": "user@example.com"}))
}

func TestParseJSON_Valid(t *testing.T) {
	args := map[string]any{"criteria": `{"from":"test@example.com"}`}
	result, err := parseJSON(args, "criteria")
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestParseJSON_Invalid(t *testing.T) {
	args := map[string]any{"criteria": `{bad json}`}
	result, err := parseJSON(args, "criteria")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid JSON for criteria")
}

func TestParseJSON_Empty(t *testing.T) {
	args := map[string]any{}
	result, err := parseJSON(args, "criteria")
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestBuildRawMessage(t *testing.T) {
	t.Run("from to/subject/body", func(t *testing.T) {
		raw := buildRawMessage(map[string]any{
			"to":      "user@example.com",
			"subject": "Hello",
			"body":    "World",
		})
		assert.NotEmpty(t, raw)
	})

	t.Run("raw passthrough", func(t *testing.T) {
		raw := buildRawMessage(map[string]any{"raw": "preencoded-base64"})
		assert.Equal(t, "preencoded-base64", raw)
	})

	t.Run("empty", func(t *testing.T) {
		raw := buildRawMessage(map[string]any{})
		assert.Empty(t, raw)
	})
}

// --- handler integration tests ---

func TestGetProfile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/gmail/v1/users/me/profile")
		_, _ = w.Write([]byte(`{"emailAddress":"test@gmail.com","messagesTotal":1000}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmail_get_profile", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "test@gmail.com")
}

func TestListMessages(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/gmail/v1/users/me/messages")
		assert.Equal(t, "is:unread", r.URL.Query().Get("q"))
		_, _ = w.Write([]byte(`{"messages":[{"id":"msg1","threadId":"thread1"}]}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmail_list_messages", map[string]any{
		"q": "is:unread",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "msg1")
}

func TestGetMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/gmail/v1/users/me/messages/msg123")
		_, _ = w.Write([]byte(`{"id":"msg123","snippet":"Hello world"}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmail_get_message", map[string]any{
		"message_id": "msg123",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "msg123")
}

func TestSendMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/gmail/v1/users/me/messages/send")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.NotEmpty(t, body["raw"])
		_, _ = w.Write([]byte(`{"id":"sent1","labelIds":["SENT"]}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmail_send_message", map[string]any{
		"to":      "user@example.com",
		"subject": "Test",
		"body":    "Hello",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "sent1")
}

func TestTrashMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/gmail/v1/users/me/messages/msg1/trash")
		_, _ = w.Write([]byte(`{"id":"msg1","labelIds":["TRASH"]}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmail_trash_message", map[string]any{
		"message_id": "msg1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "TRASH")
}

func TestModifyMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		addLabels, _ := body["addLabelIds"].([]any)
		assert.Contains(t, addLabels, "STARRED")
		_, _ = w.Write([]byte(`{"id":"msg1","labelIds":["STARRED","INBOX"]}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmail_modify_message", map[string]any{
		"message_id":   "msg1",
		"add_label_ids": "STARRED",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "STARRED")
}

func TestListLabels(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/gmail/v1/users/me/labels")
		_, _ = w.Write([]byte(`{"labels":[{"id":"INBOX","name":"INBOX"}]}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmail_list_labels", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "INBOX")
}

func TestCreateLabel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "MyLabel", body["name"])
		_, _ = w.Write([]byte(`{"id":"Label_1","name":"MyLabel"}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmail_create_label", map[string]any{
		"name": "MyLabel",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "MyLabel")
}

func TestListThreads(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/gmail/v1/users/me/threads")
		_, _ = w.Write([]byte(`{"threads":[{"id":"thread1","snippet":"Thread test"}]}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmail_list_threads", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "thread1")
}

func TestCreateDraft(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/gmail/v1/users/me/drafts")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		msg := body["message"].(map[string]any)
		assert.NotEmpty(t, msg["raw"])
		_, _ = w.Write([]byte(`{"id":"draft1","message":{"id":"msg2"}}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmail_create_draft", map[string]any{
		"to":      "user@example.com",
		"subject": "Draft Test",
		"body":    "Draft body",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "draft1")
}

func TestSendDraft(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/gmail/v1/users/me/drafts/send")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "draft1", body["id"])
		_, _ = w.Write([]byte(`{"id":"msg3","labelIds":["SENT"]}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmail_send_draft", map[string]any{
		"draft_id": "draft1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "SENT")
}

func TestGetVacation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/gmail/v1/users/me/settings/vacation")
		_, _ = w.Write([]byte(`{"enableAutoReply":false}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmail_get_vacation", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "enableAutoReply")
}

func TestListFilters(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/gmail/v1/users/me/settings/filters")
		_, _ = w.Write([]byte(`{"filter":[{"id":"filter1"}]}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmail_list_filters", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "filter1")
}

func TestCreateFilter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		criteria := body["criteria"].(map[string]any)
		assert.Equal(t, "user@example.com", criteria["from"])
		_, _ = w.Write([]byte(`{"id":"filter2"}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmail_create_filter", map[string]any{
		"criteria": `{"from":"user@example.com"}`,
		"action":   `{"addLabelIds":["IMPORTANT"]}`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "filter2")
}

func TestCreateFilter_InvalidJSON(t *testing.T) {
	g := &gmail{accessToken: "token", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gmail_create_filter", map[string]any{
		"criteria": "{bad json}",
		"action":   `{"addLabelIds":["IMPORTANT"]}`,
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid JSON for criteria")
}

func TestListSendAs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/gmail/v1/users/me/settings/sendAs")
		_, _ = w.Write([]byte(`{"sendAs":[{"sendAsEmail":"user@gmail.com"}]}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmail_list_send_as", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "user@gmail.com")
}

func TestListDelegates(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/gmail/v1/users/me/settings/delegates")
		_, _ = w.Write([]byte(`{"delegates":[{"delegateEmail":"delegate@gmail.com"}]}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmail_list_delegates", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "delegate@gmail.com")
}

func TestUserOverride(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/gmail/v1/users/custom@example.com/")
		_, _ = w.Write([]byte(`{"emailAddress":"custom@example.com"}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmail_get_profile", map[string]any{
		"user_id": "custom@example.com",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListHistory(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/gmail/v1/users/me/history")
		assert.Equal(t, "12345", r.URL.Query().Get("startHistoryId"))
		_, _ = w.Write([]byte(`{"history":[{"id":"12346"}],"historyId":"12346"}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmail_list_history", map[string]any{
		"start_history_id": "12345",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "12346")
}

func TestBatchModify(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/gmail/v1/users/me/messages/batchModify")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		ids, _ := body["ids"].([]any)
		assert.Len(t, ids, 2)
		w.WriteHeader(204)
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmail_batch_modify", map[string]any{
		"message_ids":    "msg1,msg2",
		"add_label_ids":  "IMPORTANT",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetAttachment(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/gmail/v1/users/me/messages/msg1/attachments/att1")
		_, _ = w.Write([]byte(`{"attachmentId":"att1","size":1024,"data":"base64data"}`))
	}))
	defer ts.Close()

	g := &gmail{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmail_get_attachment", map[string]any{
		"message_id":    "msg1",
		"attachment_id": "att1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "att1")
}
