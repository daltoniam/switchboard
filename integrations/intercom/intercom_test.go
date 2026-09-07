package intercom

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
	assert.Equal(t, "intercom", i.Name())
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
	c := &intercom{client: &http.Client{}, baseURL: defaultBaseURL}
	err := c.Configure(context.Background(), mcp.Credentials{
		"access_token": "tok",
		"base_url":     "https://api.eu.intercom.io/",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://api.eu.intercom.io", c.baseURL)
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

func TestTools_AllHaveIntercomPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "intercom_")
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
		if tool.Name == "intercom_search_conversations" {
			assert.Contains(t, tool.Description, "Start here")
			found = true
		}
	}
	assert.True(t, found)
}

func TestExecute_UnknownTool(t *testing.T) {
	c := &intercom{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := c.Execute(context.Background(), "intercom_nonexistent", nil)
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

func TestDoRequest_BearerAndVersion(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		assert.Equal(t, apiVersion, r.Header.Get("Intercom-Version"))
		assert.Equal(t, "/me", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"1","type":"admin"}`))
	}))
	defer ts.Close()

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := c.get(context.Background(), "/me")
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"1"`)
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"type":"error.list","errors":[{"code":"unauthorized"}]}`))
	}))
	defer ts.Close()

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := c.get(context.Background(), "/me")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "intercom API error (401)")
}

func TestDoRequest_Retryable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`rate limited`))
	}))
	defer ts.Close()

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := c.get(context.Background(), "/me")
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

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := c.doRequest(context.Background(), "DELETE", "/x", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestSearchConversations_StateFilter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/conversations/search", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		query := body["query"].(map[string]any)
		assert.Equal(t, "state", query["field"])
		assert.Equal(t, "open", query["value"])
		_, _ = w.Write([]byte(`{"conversations":[{"id":"c1","state":"open"}]}`))
	}))
	defer ts.Close()

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := c.Execute(context.Background(), "intercom_search_conversations", map[string]any{"state": "open"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "c1")
}

func TestGetConversation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/conversations/c1", r.URL.Path)
		assert.Equal(t, "plaintext", r.URL.Query().Get("display_as"))
		_, _ = w.Write([]byte(`{"id":"c1","state":"open"}`))
	}))
	defer ts.Close()

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := c.Execute(context.Background(), "intercom_get_conversation", map[string]any{"conversation_id": "c1"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "c1")
}

func TestReplyConversation_AdminComment(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/conversations/c1/reply", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "comment", body["message_type"])
		assert.Equal(t, "admin", body["type"])
		assert.Equal(t, "991", body["admin_id"])
		assert.Equal(t, "hello", body["body"])
		_, _ = w.Write([]byte(`{"id":"c1"}`))
	}))
	defer ts.Close()

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := c.Execute(context.Background(), "intercom_reply_conversation", map[string]any{
		"conversation_id": "c1",
		"admin_id":        "991",
		"body":            "hello",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestReplyConversation_MissingAdminID(t *testing.T) {
	c := &intercom{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := c.Execute(context.Background(), "intercom_reply_conversation", map[string]any{
		"conversation_id": "c1",
		"body":            "hello",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "admin_id is required")
}

func TestReplyConversation_MissingUserID(t *testing.T) {
	c := &intercom{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := c.Execute(context.Background(), "intercom_reply_conversation", map[string]any{
		"conversation_id": "c1",
		"body":            "hello",
		"type":            "user",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "intercom_user_id is required")
}

func TestReplyConversation_InvalidType(t *testing.T) {
	c := &intercom{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := c.Execute(context.Background(), "intercom_reply_conversation", map[string]any{
		"conversation_id": "c1",
		"body":            "hello",
		"type":            "note",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, `type must be admin or user, got "note"`)
}

func TestCloseConversation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/conversations/c1/parts", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "close", body["message_type"])
		assert.Equal(t, "991", body["admin_id"])
		_, _ = w.Write([]byte(`{"id":"c1","state":"closed"}`))
	}))
	defer ts.Close()

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := c.Execute(context.Background(), "intercom_close_conversation", map[string]any{
		"conversation_id": "c1",
		"admin_id":        "991",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "closed")
}

func TestAssignConversation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "assignment", body["message_type"])
		assert.Equal(t, "992", body["assignee_id"])
		_, _ = w.Write([]byte(`{"id":"c1"}`))
	}))
	defer ts.Close()

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := c.Execute(context.Background(), "intercom_assign_conversation", map[string]any{
		"conversation_id": "c1",
		"admin_id":        "991",
		"assignee_id":     "992",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestSearchContacts_Email(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/contacts/search", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		query := body["query"].(map[string]any)
		assert.Equal(t, "email", query["field"])
		assert.Equal(t, "a@b.com", query["value"])
		_, _ = w.Write([]byte(`{"data":[{"id":"u1","email":"a@b.com"}]}`))
	}))
	defer ts.Close()

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := c.Execute(context.Background(), "intercom_search_contacts", map[string]any{"email": "a@b.com"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "u1")
}

func TestSearchContacts_MissingFilter(t *testing.T) {
	c := &intercom{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := c.Execute(context.Background(), "intercom_search_contacts", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "provide email or query")
}

func TestCreateContact(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/contacts", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "a@b.com", body["email"])
		assert.Equal(t, "Ada", body["name"])
		_, _ = w.Write([]byte(`{"id":"u1","email":"a@b.com"}`))
	}))
	defer ts.Close()

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := c.Execute(context.Background(), "intercom_create_contact", map[string]any{
		"email": "a@b.com",
		"name":  "Ada",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "u1")
}

func TestCreateContact_MissingIdentifier(t *testing.T) {
	c := &intercom{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := c.Execute(context.Background(), "intercom_create_contact", map[string]any{
		"name": "Ada",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "provide email or external_id")
}

func TestSearchArticles(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/articles/search", r.URL.Path)
		assert.Equal(t, "billing", r.URL.Query().Get("phrase"))
		_, _ = w.Write([]byte(`{"data":{"articles":[{"id":"a1","title":"Billing"}]}}`))
	}))
	defer ts.Close()

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := c.Execute(context.Background(), "intercom_search_articles", map[string]any{"phrase": "billing"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Billing")
}

func TestUpdateTicket_OpenFalse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/tickets/t1", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, false, body["open"])
		_, _ = w.Write([]byte(`{"id":"t1","open":false}`))
	}))
	defer ts.Close()

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := c.Execute(context.Background(), "intercom_update_ticket", map[string]any{
		"ticket_id": "t1",
		"open":      false,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestUpdateTicket_AssigneeID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "991", body["assignee_id"])
		_, hasAssignment := body["assignment"]
		assert.False(t, hasAssignment)
		_, _ = w.Write([]byte(`{"id":"t1"}`))
	}))
	defer ts.Close()

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := c.Execute(context.Background(), "intercom_update_ticket", map[string]any{
		"ticket_id":         "t1",
		"admin_assignee_id": "991",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestHealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/me", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer ts.Close()

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	assert.True(t, c.Healthy(context.Background()))
}

func TestHealthy_Unreachable(t *testing.T) {
	c := &intercom{client: &http.Client{}, baseURL: "http://127.0.0.1:1"}
	assert.False(t, c.Healthy(context.Background()))
}

func TestTagConversation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/conversations/c1/tags", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "tag1", body["id"])
		assert.Equal(t, "991", body["admin_id"])
		_, _ = w.Write([]byte(`{"type":"tag","id":"tag1"}`))
	}))
	defer ts.Close()

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := c.Execute(context.Background(), "intercom_tag_conversation", map[string]any{
		"conversation_id": "c1",
		"tag_id":          "tag1",
		"admin_id":        "991",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestSnoozeConversation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "snoozed", body["message_type"])
		assert.Equal(t, float64(1735641600), body["snoozed_until"])
		_, _ = w.Write([]byte(`{"id":"c1","state":"snoozed"}`))
	}))
	defer ts.Close()

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := c.Execute(context.Background(), "intercom_snooze_conversation", map[string]any{
		"conversation_id": "c1",
		"admin_id":        "991",
		"snoozed_until":   1735641600,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestCreateTicket(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/tickets", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "tt1", body["ticket_type_id"])
		contacts := body["contacts"].([]any)
		assert.Equal(t, "u1", contacts[0].(map[string]any)["id"])
		assignment := body["assignment"].(map[string]any)
		assert.Equal(t, "991", assignment["admin_assignee_id"])
		_, hasTeam := assignment["team_assignee_id"]
		assert.False(t, hasTeam)
		_, _ = w.Write([]byte(`{"id":"t1"}`))
	}))
	defer ts.Close()

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := c.Execute(context.Background(), "intercom_create_ticket", map[string]any{
		"ticket_type_id":    "tt1",
		"contact_id":        "u1",
		"title":             "Help",
		"admin_assignee_id": "991",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestCreateTicket_MissingContact(t *testing.T) {
	c := &intercom{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := c.Execute(context.Background(), "intercom_create_ticket", map[string]any{
		"ticket_type_id": "tt1",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "provide contact_id or contacts")
}

func TestSearchTickets_StateFilter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/tickets/search", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		query := body["query"].(map[string]any)
		assert.Equal(t, "state", query["field"])
		assert.Equal(t, "submitted", query["value"])
		_, _ = w.Write([]byte(`{"tickets":[{"id":"t1"}]}`))
	}))
	defer ts.Close()

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := c.Execute(context.Background(), "intercom_search_tickets", map[string]any{"state": "submitted"})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestListTicketTypes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/ticket_types", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"id":"tt1","name":"Bug"}]}`))
	}))
	defer ts.Close()

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := c.Execute(context.Background(), "intercom_list_ticket_types", map[string]any{})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Bug")
}

func TestListConversations(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/conversations", r.URL.Path)
		assert.Equal(t, "20", r.URL.Query().Get("per_page"))
		_, _ = w.Write([]byte(`{"conversations":[{"id":"c1"}]}`))
	}))
	defer ts.Close()

	c := &intercom{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := c.Execute(context.Background(), "intercom_list_conversations", map[string]any{})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "c1")
}
