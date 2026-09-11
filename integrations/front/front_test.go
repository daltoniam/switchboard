package front

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
	assert.Equal(t, "front", i.Name())
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

func TestConfigure_CustomBaseURL(t *testing.T) {
	f := &front{client: &http.Client{}, baseURL: defaultBaseURL}
	err := f.Configure(context.Background(), mcp.Credentials{
		"access_token": "tok",
		"base_url":     "https://acme.api.frontapp.com/",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://acme.api.frontapp.com", f.baseURL)
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

func TestTools_AllHaveFrontPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "front_")
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
		if tool.Name == "front_search_conversations" {
			assert.Contains(t, tool.Description, "Start here")
			found = true
		}
	}
	assert.True(t, found)
}

func TestExecute_UnknownTool(t *testing.T) {
	f := &front{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := f.Execute(context.Background(), "front_nonexistent", nil)
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

func TestDoRequest_Bearer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		assert.Equal(t, "/teammates", r.URL.Path)
		_, _ = w.Write([]byte(`{"_results":[{"id":"tea_1"}]}`))
	}))
	defer ts.Close()

	f := &front{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := f.get(context.Background(), "/teammates")
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"tea_1"`)
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"_error":{"status":401,"title":"Unauthenticated"}}`))
	}))
	defer ts.Close()

	f := &front{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := f.get(context.Background(), "/teammates")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "front API error (401)")
}

func TestDoRequest_Retryable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`rate limited`))
	}))
	defer ts.Close()

	f := &front{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := f.get(context.Background(), "/teammates")
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

	f := &front{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := f.doRequest(context.Background(), "PATCH", "/conversations/cnv_1", map[string]any{"status": "archived"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestSearchConversations(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/conversations/search/is:open", r.URL.Path)
		assert.Equal(t, "25", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"_results":[{"id":"cnv_1","status":"unassigned"}]}`))
	}))
	defer ts.Close()

	f := &front{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := f.Execute(context.Background(), "front_search_conversations", map[string]any{"query": "is:open"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "cnv_1")
}

func TestListConversations_Statuses(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/conversations", r.URL.Path)
		assert.Equal(t, []string{"unassigned", "assigned"}, r.URL.Query()["q[statuses]"])
		_, _ = w.Write([]byte(`{"_results":[{"id":"cnv_2"}]}`))
	}))
	defer ts.Close()

	f := &front{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := f.Execute(context.Background(), "front_list_conversations", map[string]any{"statuses": "unassigned,assigned"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "cnv_2")
}

func TestGetConversation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/conversations/cnv_1", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"cnv_1","subject":"Help"}`))
	}))
	defer ts.Close()

	f := &front{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := f.Execute(context.Background(), "front_get_conversation", map[string]any{"conversation_id": "cnv_1"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "cnv_1")
}

func TestAddComment(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/conversations/cnv_1/comments", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "looks good", body["body"])
		_, _ = w.Write([]byte(`{"id":"com_1"}`))
	}))
	defer ts.Close()

	f := &front{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := f.Execute(context.Background(), "front_add_comment", map[string]any{
		"conversation_id": "cnv_1",
		"body":            "looks good",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "com_1")
}

func TestAssignConversation_Unassign(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/conversations/cnv_1", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Nil(t, body["assignee_id"])
		w.WriteHeader(204)
	}))
	defer ts.Close()

	f := &front{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := f.Execute(context.Background(), "front_assign_conversation", map[string]any{
		"conversation_id": "cnv_1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestUpdateConversation_RequiresField(t *testing.T) {
	f := &front{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := f.Execute(context.Background(), "front_update_conversation", map[string]any{
		"conversation_id": "cnv_1",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "provide at least one")
}

func TestSearchContacts_EmailAlias(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/contacts/alt:email:ada@example.com", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"crd_1","name":"Ada"}`))
	}))
	defer ts.Close()

	f := &front{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := f.Execute(context.Background(), "front_search_contacts", map[string]any{"email": "ada@example.com"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "crd_1")
}

func TestSearchContacts_MissingLookup(t *testing.T) {
	f := &front{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := f.Execute(context.Background(), "front_search_contacts", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "provide email or contact_id")
}

func TestCreateMessage_RequiresRecipient(t *testing.T) {
	f := &front{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := f.Execute(context.Background(), "front_create_message", map[string]any{
		"channel_id": "cha_1",
		"body":       "hello",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "provide to, cc, or bcc")
}

func TestCreateMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/channels/cha_1/messages", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, []any{"ada@example.com"}, body["to"])
		assert.Equal(t, "hello", body["body"])
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"status":"accepted","message_uid":"abc"}`))
	}))
	defer ts.Close()

	f := &front{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := f.Execute(context.Background(), "front_create_message", map[string]any{
		"channel_id": "cha_1",
		"to":         "ada@example.com",
		"body":       "hello",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "accepted")
}

func TestReplyConversation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/conversations/cnv_1/messages", r.URL.Path)
		_, _ = w.Write([]byte(`{"status":"accepted"}`))
	}))
	defer ts.Close()

	f := &front{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := f.Execute(context.Background(), "front_reply_conversation", map[string]any{
		"conversation_id": "cnv_1",
		"body":            "thanks",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestCreateDraft(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/channels/cha_1/drafts", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "draft body", body["body"])
		_, _ = w.Write([]byte(`{"id":"msg_draft"}`))
	}))
	defer ts.Close()

	f := &front{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := f.Execute(context.Background(), "front_create_draft", map[string]any{
		"channel_id": "cha_1",
		"body":       "draft body",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "msg_draft")
}

func TestListTeammates(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/teammates", r.URL.Path)
		_, _ = w.Write([]byte(`{"_results":[{"id":"tea_1","email":"ada@example.com"}]}`))
	}))
	defer ts.Close()

	f := &front{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := f.Execute(context.Background(), "front_list_teammates", map[string]any{})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "tea_1")
}

func TestHealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/teammates", r.URL.Path)
		_, _ = w.Write([]byte(`{"_results":[]}`))
	}))
	defer ts.Close()

	f := &front{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	assert.True(t, f.Healthy(context.Background()))

	f = &front{client: &http.Client{}, baseURL: "http://127.0.0.1:1"}
	assert.False(t, f.Healthy(context.Background()))
}

func TestMaxBytesUnknown(t *testing.T) {
	f := &front{}
	_, ok := f.MaxBytes("front_nonexistent")
	assert.False(t, ok)
}
