package okta

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
	assert.Equal(t, "okta", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"api_token": "00token",
		"org_url":   "https://acme.okta.com",
	})
	assert.NoError(t, err)
}

func TestConfigure_MissingAPIToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"org_url": "https://acme.okta.com"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_token is required")
}

func TestConfigure_MissingOrgURL(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_token": "t"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "org_url is required")
}

func TestConfigure_NormalizesOrgURL(t *testing.T) {
	o := &okta{client: &http.Client{}}
	err := o.Configure(context.Background(), mcp.Credentials{
		"api_token": "t",
		"org_url":   "acme.okta.com/api/v1/",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://acme.okta.com", o.orgURL)
	assert.Equal(t, "https://acme.okta.com/api/v1", o.baseURL)
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

func TestTools_AllHaveOktaPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "okta_")
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
		if tool.Name == "okta_list_users" {
			assert.Contains(t, tool.Description, "Start here")
			found = true
		}
	}
	assert.True(t, found)
}

func TestExecute_UnknownTool(t *testing.T) {
	o := &okta{apiToken: "t", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := o.Execute(context.Background(), "okta_nonexistent", nil)
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

func TestDoRequest_SSWSAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "SSWS tok", r.Header.Get("Authorization"))
		assert.Equal(t, "/api/v1/org", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"org1","subdomain":"acme"}`))
	}))
	defer ts.Close()

	o := &okta{apiToken: "tok", client: ts.Client(), baseURL: ts.URL + "/api/v1"}
	data, err := o.get(context.Background(), "/org")
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"org1"`)
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"errorCode":"E0000011","errorSummary":"Invalid token"}`))
	}))
	defer ts.Close()

	o := &okta{apiToken: "t", client: ts.Client(), baseURL: ts.URL + "/api/v1"}
	_, err := o.get(context.Background(), "/users")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "okta API error (401)")
}

func TestDoRequest_Retryable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`rate limited`))
	}))
	defer ts.Close()

	o := &okta{apiToken: "t", client: ts.Client(), baseURL: ts.URL + "/api/v1"}
	_, err := o.get(context.Background(), "/users")
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

	o := &okta{apiToken: "t", client: ts.Client(), baseURL: ts.URL + "/api/v1"}
	data, err := o.doRequest(context.Background(), "DELETE", "/groups/g1/users/u1", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestListUsers_WrapsItemsAndCursor(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/users", r.URL.Path)
		assert.Equal(t, "jane", r.URL.Query().Get("q"))
		assert.Equal(t, "20", r.URL.Query().Get("limit"))
		w.Header().Set("Link", `<https://acme.okta.com/api/v1/users?after=00unext&limit=20>; rel="next"`)
		_, _ = w.Write([]byte(`[{"id":"00u1","status":"ACTIVE","profile":{"login":"jane@acme.com","email":"jane@acme.com","firstName":"Jane","lastName":"Doe"}}]`))
	}))
	defer ts.Close()

	o := &okta{apiToken: "t", client: ts.Client(), baseURL: ts.URL + "/api/v1"}
	result, err := o.Execute(context.Background(), "okta_list_users", map[string]any{
		"q":     "jane",
		"limit": 20,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, `"items"`)
	assert.Contains(t, result.Data, "00u1")
	assert.Contains(t, result.Data, `"next_after":"00unext"`)
}

func TestGetUser(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/users/jane@acme.com", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"00u1","status":"ACTIVE"}`))
	}))
	defer ts.Close()

	o := &okta{apiToken: "t", client: ts.Client(), baseURL: ts.URL + "/api/v1"}
	result, err := o.Execute(context.Background(), "okta_get_user", map[string]any{"user_id": "jane@acme.com"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "00u1")
}

func TestCreateUser(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/users", r.URL.Path)
		assert.Equal(t, "true", r.URL.Query().Get("activate"))
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		profile := body["profile"].(map[string]any)
		assert.Equal(t, "jane@acme.com", profile["email"])
		_, _ = w.Write([]byte(`{"id":"00u1","status":"ACTIVE"}`))
	}))
	defer ts.Close()

	o := &okta{apiToken: "t", client: ts.Client(), baseURL: ts.URL + "/api/v1"}
	result, err := o.Execute(context.Background(), "okta_create_user", map[string]any{
		"profile":  `{"firstName":"Jane","lastName":"Doe","email":"jane@acme.com","login":"jane@acme.com"}`,
		"activate": "true",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "00u1")
}

func TestActivateUser(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/users/00u1/lifecycle/activate", r.URL.Path)
		assert.Equal(t, "false", r.URL.Query().Get("sendEmail"))
		_, _ = w.Write([]byte(`{"activationUrl":"https://acme.okta.com/welcome/x"}`))
	}))
	defer ts.Close()

	o := &okta{apiToken: "t", client: ts.Client(), baseURL: ts.URL + "/api/v1"}
	result, err := o.Execute(context.Background(), "okta_activate_user", map[string]any{
		"user_id":    "00u1",
		"send_email": "false",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "activationUrl")
}

func TestAddGroupMember(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/api/v1/groups/00g1/users/00u1", r.URL.Path)
		w.WriteHeader(204)
	}))
	defer ts.Close()

	o := &okta{apiToken: "t", client: ts.Client(), baseURL: ts.URL + "/api/v1"}
	result, err := o.Execute(context.Background(), "okta_add_group_member", map[string]any{
		"group_id": "00g1",
		"user_id":  "00u1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "success")
}

func TestListLogs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/logs", r.URL.Path)
		assert.Equal(t, `eventType eq "user.session.start"`, r.URL.Query().Get("filter"))
		_, _ = w.Write([]byte(`[{"uuid":"e1","eventType":"user.session.start","outcome":{"result":"SUCCESS"}}]`))
	}))
	defer ts.Close()

	o := &okta{apiToken: "t", client: ts.Client(), baseURL: ts.URL + "/api/v1"}
	result, err := o.Execute(context.Background(), "okta_list_logs", map[string]any{
		"filter": `eventType eq "user.session.start"`,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "user.session.start")
	assert.Contains(t, result.Data, `"items"`)
}

func TestHealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/org", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"org1"}`))
	}))
	defer ts.Close()

	o := &okta{apiToken: "t", client: ts.Client(), baseURL: ts.URL + "/api/v1"}
	assert.True(t, o.Healthy(context.Background()))
}

func TestHealthy_Unconfigured(t *testing.T) {
	o := &okta{client: &http.Client{}, baseURL: "http://127.0.0.1:1"}
	assert.False(t, o.Healthy(context.Background()))
}

func TestParseNextAfter(t *testing.T) {
	link := `<https://acme.okta.com/api/v1/users?limit=20>; rel="self", <https://acme.okta.com/api/v1/users?after=00unext&limit=20>; rel="next"`
	assert.Equal(t, "00unext", parseNextAfter(link))
	assert.Equal(t, "", parseNextAfter(""))
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
