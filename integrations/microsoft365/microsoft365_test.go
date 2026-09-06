package microsoft365

import (
	"context"
	"encoding/json"
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
	assert.Equal(t, "microsoft365", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"access_token": "tok"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAccessToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_token is required")
}

func TestConfigure_CustomBaseURLAndTenant(t *testing.T) {
	m := &m365{client: &http.Client{}, baseURL: defaultBaseURL}
	err := m.Configure(context.Background(), mcp.Credentials{
		"access_token": "tok",
		"base_url":     "https://graph.microsoft.us/v1.0/",
		"tenant_id":    "contoso.onmicrosoft.com",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://graph.microsoft.us/v1.0", m.baseURL)
	assert.Equal(t, "contoso.onmicrosoft.com", m.tenantID)
}

func TestTools(t *testing.T) {
	i := New()
	tls := i.Tools()
	assert.NotEmpty(t, tls)
	for _, tool := range tls {
		assert.NotEmpty(t, tool.Name)
		assert.NotEmpty(t, tool.Description)
	}
}

func TestTools_AllHavePrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "microsoft365_")
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
	found := false
	for _, tool := range i.Tools() {
		if tool.Name == "microsoft365_get_me" {
			assert.Contains(t, tool.Description, "Start here")
			found = true
		}
	}
	assert.True(t, found)
}

func TestExecute_UnknownTool(t *testing.T) {
	m := &m365{accessToken: "t", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := m.Execute(context.Background(), "microsoft365_nonexistent", nil)
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

func TestHealthy_Unconfigured(t *testing.T) {
	m := &m365{client: &http.Client{Timeout: 1}, baseURL: "http://127.0.0.1:1"}
	assert.False(t, m.Healthy(context.Background()))
}

func configured(ts *httptest.Server) *m365 {
	return &m365{accessToken: "tok", client: ts.Client(), baseURL: ts.URL, tenantID: defaultTenant}
}

func TestDoRequest_BearerAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		assert.Equal(t, "/me", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"u1","displayName":"Ada"}`))
	}))
	defer ts.Close()

	m := configured(ts)
	data, err := m.get(context.Background(), "/me")
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"u1"`)
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"code":"InvalidAuthenticationToken"}}`))
	}))
	defer ts.Close()

	m := configured(ts)
	_, err := m.get(context.Background(), "/me")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "microsoft365 API error (401)")
}

func TestDoRequest_Retryable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`throttled`))
	}))
	defer ts.Close()

	m := configured(ts)
	_, err := m.get(context.Background(), "/me")
	require.Error(t, err)
	var re *mcp.RetryableError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, 429, re.StatusCode)
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	m := configured(ts)
	data, err := m.doRequest(context.Background(), "DELETE", ts.URL+"/me/messages/1", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestGetMe(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/me", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"u1","displayName":"Ada Lovelace"}`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "microsoft365_get_me", nil)
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Ada Lovelace")
}

func TestListMessages(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/me/messages", r.URL.Path)
		assert.Equal(t, "from:ada", r.URL.Query().Get("$search"))
		assert.Equal(t, "25", r.URL.Query().Get("$top"))
		_, _ = w.Write([]byte(`{"value":[{"id":"m1","subject":"Hello"}],"@odata.nextLink":"https://graph.microsoft.com/v1.0/me/messages?$skiptoken=abc"}`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "microsoft365_list_messages", map[string]any{
		"search": "from:ada",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Hello")
	assert.Contains(t, result.Data, `"next_link"`)
	assert.NotContains(t, result.Data, `@odata.nextLink`)
}

func TestListMessages_NextLink(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/me/messages", r.URL.Path)
		assert.Equal(t, "page2", r.URL.Query().Get("$skiptoken"))
		_, _ = w.Write([]byte(`{"value":[{"id":"m2"}]}`))
	}))
	defer ts.Close()

	next := ts.URL + "/me/messages?$skiptoken=page2"
	result, err := configured(ts).Execute(context.Background(), "microsoft365_list_messages", map[string]any{
		"next_link": next,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "m2")
}

func TestListMessages_RejectsForeignNextLink(t *testing.T) {
	m := &m365{accessToken: "t", client: &http.Client{}, baseURL: defaultBaseURL}
	result, err := m.Execute(context.Background(), "microsoft365_list_messages", map[string]any{
		"next_link": "https://evil.example/steal",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Contains(t, result.Data, "next_link must be a Microsoft Graph URL")
}

func TestSendMail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/me/sendMail", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		msg := body["message"].(map[string]any)
		assert.Equal(t, "Hello", msg["subject"])
		w.WriteHeader(202)
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "microsoft365_send_mail", map[string]any{
		"to":      "ada@contoso.com",
		"subject": "Hello",
		"body":    "Hi",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "success")
}

func TestCreateEvent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/me/events", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Standup", body["subject"])
		_, _ = w.Write([]byte(`{"id":"e1","subject":"Standup"}`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "microsoft365_create_event", map[string]any{
		"subject": "Standup",
		"start":   "2024-03-15T09:00:00",
		"end":     "2024-03-15T09:30:00",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "e1")
}

func TestListEvents_CalendarView(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/me/calendarView", r.URL.Path)
		assert.Equal(t, "2024-03-15T00:00:00Z", r.URL.Query().Get("startDateTime"))
		_, _ = w.Write([]byte(`{"value":[{"id":"e1"}]}`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "microsoft365_list_events", map[string]any{
		"start": "2024-03-15T00:00:00Z",
		"end":   "2024-03-16T00:00:00Z",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "e1")
}

func TestSearchDrive(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/me/drive/root/search")
		_, _ = w.Write([]byte(`{"value":[{"id":"f1","name":"budget.xlsx"}]}`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "microsoft365_search_drive", map[string]any{
		"q": "budget",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "budget.xlsx")
}

func TestDownloadDriveItem_Text(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/me/drive/items/f1/content", r.URL.Path)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello file"))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "microsoft365_download_drive_item", map[string]any{
		"item_id": "f1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "hello file")
	assert.NotContains(t, result.Data, "content_base64")
}

func TestSendChannelMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/teams/t1/channels/c1/messages", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"msg1"}`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "microsoft365_send_channel_message", map[string]any{
		"team_id":    "t1",
		"channel_id": "c1",
		"content":    "hello team",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "msg1")
}

func TestCreateTodoTask(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/me/todo/lists/l1/tasks", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Ship adapter", body["title"])
		_, _ = w.Write([]byte(`{"id":"task1","title":"Ship adapter"}`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "microsoft365_create_todo_task", map[string]any{
		"list_id": "l1",
		"title":   "Ship adapter",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "task1")
}

func TestListUsers_SearchHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "eventual", r.Header.Get("ConsistencyLevel"))
		assert.Equal(t, "true", r.URL.Query().Get("$count"))
		_, _ = w.Write([]byte(`{"value":[{"id":"u1"}]}`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "microsoft365_list_users", map[string]any{
		"search": `"displayName:Ada"`,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestPlaceholdersAndOptionalKeys(t *testing.T) {
	m := New().(*m365)
	assert.Contains(t, m.OptionalKeys(), "tenant_id")
	assert.Equal(t, "Microsoft Graph OAuth access token", m.Placeholders()["access_token"])
	assert.Contains(t, m.PlainTextKeys(), "base_url")
}
