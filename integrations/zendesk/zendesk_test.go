package zendesk

import (
	"context"
	"encoding/base64"
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
	assert.Equal(t, "zendesk", i.Name())
}

func TestConfigure_APIToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"subdomain": "acme",
		"email":     "agent@acme.com",
		"api_token": "tok",
	})
	assert.NoError(t, err)
}

func TestConfigure_OAuthToken(t *testing.T) {
	z := &zendesk{client: &http.Client{}}
	err := z.Configure(context.Background(), mcp.Credentials{
		"subdomain":    "acme",
		"access_token": "oauth-token",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://acme.zendesk.com/api/v2", z.baseURL)
}

func TestConfigure_MissingAuth(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"subdomain": "acme"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_token or email + api_token is required")
}

func TestConfigure_MissingSubdomain(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"email":     "a@b.com",
		"api_token": "tok",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "subdomain is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	z := &zendesk{client: &http.Client{}}
	err := z.Configure(context.Background(), mcp.Credentials{
		"email":     "a@b.com",
		"api_token": "tok",
		"base_url":  "https://example.test/api/v2/",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://example.test/api/v2", z.baseURL)
}

func TestConfigure_NormalizesSubdomain(t *testing.T) {
	z := &zendesk{client: &http.Client{}}
	err := z.Configure(context.Background(), mcp.Credentials{
		"subdomain":    "https://acme.zendesk.com",
		"access_token": "tok",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://acme.zendesk.com/api/v2", z.baseURL)
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

func TestTools_AllHaveZendeskPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "zendesk_")
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
		if tool.Name == "zendesk_search_tickets" {
			assert.Contains(t, tool.Description, "Start here")
			found = true
		}
	}
	assert.True(t, found)
}

func TestExecute_UnknownTool(t *testing.T) {
	z := &zendesk{email: "a@b.com", apiToken: "t", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := z.Execute(context.Background(), "zendesk_nonexistent", nil)
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

func TestDoRequest_APITokenAuth(t *testing.T) {
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("agent@acme.com/token:tok"))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, want, r.Header.Get("Authorization"))
		assert.Equal(t, "/users/me.json", r.URL.Path)
		_, _ = w.Write([]byte(`{"user":{"id":1}}`))
	}))
	defer ts.Close()

	z := &zendesk{email: "agent@acme.com", apiToken: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := z.get(context.Background(), "/users/me.json")
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":1`)
}

func TestDoRequest_BearerAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer oauth-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"user":{"id":1}}`))
	}))
	defer ts.Close()

	z := &zendesk{accessToken: "oauth-token", client: ts.Client(), baseURL: ts.URL}
	_, err := z.get(context.Background(), "/users/me.json")
	require.NoError(t, err)
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer ts.Close()

	z := &zendesk{email: "a@b.com", apiToken: "t", client: ts.Client(), baseURL: ts.URL}
	_, err := z.get(context.Background(), "/users/me.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "zendesk API error (401)")
}

func TestDoRequest_Retryable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`rate limited`))
	}))
	defer ts.Close()

	z := &zendesk{email: "a@b.com", apiToken: "t", client: ts.Client(), baseURL: ts.URL}
	_, err := z.get(context.Background(), "/users/me.json")
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

	z := &zendesk{email: "a@b.com", apiToken: "t", client: ts.Client(), baseURL: ts.URL}
	data, err := z.doRequest(context.Background(), "DELETE", "/tickets/1.json", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestSearchTickets_DoesNotTreatPrototypeAsTypeFilter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "type:ticket prototype: broken login", r.URL.Query().Get("query"))
		_, _ = w.Write([]byte(`{"results":[],"count":0}`))
	}))
	defer ts.Close()

	z := &zendesk{email: "a@b.com", apiToken: "t", client: ts.Client(), baseURL: ts.URL}
	result, err := z.Execute(context.Background(), "zendesk_search_tickets", map[string]any{
		"query": "prototype: broken login",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestCreateTicket_NumericIDsAndNativeCustomFields(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		ticket := body["ticket"].(map[string]any)
		assert.Equal(t, float64(10), ticket["requester_id"])
		assert.Equal(t, float64(20), ticket["assignee_id"])
		assert.Equal(t, float64(30), ticket["group_id"])
		fields := ticket["custom_fields"].([]any)
		require.Len(t, fields, 1)
		field := fields[0].(map[string]any)
		assert.Equal(t, float64(100), field["id"])
		assert.Equal(t, "web", field["value"])
		_, _ = w.Write([]byte(`{"ticket":{"id":9}}`))
	}))
	defer ts.Close()

	z := &zendesk{email: "a@b.com", apiToken: "t", client: ts.Client(), baseURL: ts.URL}
	result, err := z.Execute(context.Background(), "zendesk_create_ticket", map[string]any{
		"subject":       "Cannot login",
		"comment":       "Password reset failed",
		"requester_id":  10,
		"assignee_id":   "20",
		"group_id":      float64(30),
		"custom_fields": []any{map[string]any{"id": 100, "value": "web"}},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestSearchTickets(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search.json", r.URL.Path)
		assert.Equal(t, "type:ticket status:open", r.URL.Query().Get("query"))
		_, _ = w.Write([]byte(`{"results":[{"id":1,"subject":"Login broken","status":"open"}],"count":1}`))
	}))
	defer ts.Close()

	z := &zendesk{email: "a@b.com", apiToken: "t", client: ts.Client(), baseURL: ts.URL}
	result, err := z.Execute(context.Background(), "zendesk_search_tickets", map[string]any{
		"query": "status:open",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Login broken")
}

func TestGetTicket(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/tickets/42.json", r.URL.Path)
		_, _ = w.Write([]byte(`{"ticket":{"id":42,"subject":"Outage"}}`))
	}))
	defer ts.Close()

	z := &zendesk{email: "a@b.com", apiToken: "t", client: ts.Client(), baseURL: ts.URL}
	result, err := z.Execute(context.Background(), "zendesk_get_ticket", map[string]any{"ticket_id": 42})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Outage")
}

func TestCreateTicket(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/tickets.json", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		ticket := body["ticket"].(map[string]any)
		assert.Equal(t, "Cannot login", ticket["subject"])
		comment := ticket["comment"].(map[string]any)
		assert.Equal(t, "Password reset failed", comment["body"])
		_, _ = w.Write([]byte(`{"ticket":{"id":9,"subject":"Cannot login"}}`))
	}))
	defer ts.Close()

	z := &zendesk{email: "a@b.com", apiToken: "t", client: ts.Client(), baseURL: ts.URL}
	result, err := z.Execute(context.Background(), "zendesk_create_ticket", map[string]any{
		"subject": "Cannot login",
		"comment": "Password reset failed",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, `"id":9`)
}

func TestCreateTicket_MissingComment(t *testing.T) {
	z := &zendesk{email: "a@b.com", apiToken: "t", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := z.Execute(context.Background(), "zendesk_create_ticket", map[string]any{
		"subject": "No body",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "comment is required")
}

func TestAddTicketComment(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/tickets/7.json", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		ticket := body["ticket"].(map[string]any)
		comment := ticket["comment"].(map[string]any)
		assert.Equal(t, "Looking into this", comment["body"])
		assert.Equal(t, false, comment["public"])
		_, _ = w.Write([]byte(`{"ticket":{"id":7}}`))
	}))
	defer ts.Close()

	z := &zendesk{email: "a@b.com", apiToken: "t", client: ts.Client(), baseURL: ts.URL}
	result, err := z.Execute(context.Background(), "zendesk_add_ticket_comment", map[string]any{
		"ticket_id": "7",
		"body":      "Looking into this",
		"public":    "FALSE",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestCreateTicket_TagsSliceAndPublicFalse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		ticket := body["ticket"].(map[string]any)
		comment := ticket["comment"].(map[string]any)
		assert.Equal(t, false, comment["public"])
		assert.Equal(t, []any{"auth", "vip"}, ticket["tags"])
		_, _ = w.Write([]byte(`{"ticket":{"id":9}}`))
	}))
	defer ts.Close()

	z := &zendesk{email: "a@b.com", apiToken: "t", client: ts.Client(), baseURL: ts.URL}
	result, err := z.Execute(context.Background(), "zendesk_create_ticket", map[string]any{
		"subject": "Cannot login",
		"comment": "Password reset failed",
		"public":  "False",
		"tags":    []any{"auth", "vip"},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestListTickets_CursorPagination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/tickets.json", r.URL.Path)
		assert.Equal(t, "25", r.URL.Query().Get("page[size]"))
		assert.Equal(t, "abc", r.URL.Query().Get("page[after]"))
		_, _ = w.Write([]byte(`{"tickets":[{"id":1}],"meta":{"has_more":false}}`))
	}))
	defer ts.Close()

	z := &zendesk{email: "a@b.com", apiToken: "t", client: ts.Client(), baseURL: ts.URL}
	result, err := z.Execute(context.Background(), "zendesk_list_tickets", map[string]any{
		"page_after": "abc",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, `"id":1`)
}

func TestGetCurrentUser(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/users/me.json", r.URL.Path)
		_, _ = w.Write([]byte(`{"user":{"id":99,"email":"me@acme.com"}}`))
	}))
	defer ts.Close()

	z := &zendesk{email: "a@b.com", apiToken: "t", client: ts.Client(), baseURL: ts.URL}
	result, err := z.Execute(context.Background(), "zendesk_get_current_user", nil)
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "me@acme.com")
}

func TestSearchArticles(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/help_center/articles/search.json", r.URL.Path)
		assert.Equal(t, "password reset", r.URL.Query().Get("query"))
		_, _ = w.Write([]byte(`{"results":[{"id":3,"title":"Reset your password"}]}`))
	}))
	defer ts.Close()

	z := &zendesk{email: "a@b.com", apiToken: "t", client: ts.Client(), baseURL: ts.URL}
	result, err := z.Execute(context.Background(), "zendesk_search_articles", map[string]any{
		"query": "password reset",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Reset your password")
}

func TestHealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/users/me.json")
		_, _ = w.Write([]byte(`{"user":{"id":1}}`))
	}))
	defer ts.Close()

	z := &zendesk{email: "a@b.com", apiToken: "t", client: ts.Client(), baseURL: ts.URL}
	assert.True(t, z.Healthy(context.Background()))
}

func TestHealthy_Unconfigured(t *testing.T) {
	z := &zendesk{client: &http.Client{}, baseURL: "http://127.0.0.1:1"}
	assert.False(t, z.Healthy(context.Background()))
}

func TestRawResult(t *testing.T) {
	data := json.RawMessage(`{"key":"value"}`)
	result, err := mcp.RawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"key":"value"}`, result.Data)
}

func TestErrResult(t *testing.T) {
	result, err := mcp.ErrResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}
