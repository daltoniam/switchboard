package servicenow

import (
	"context"
	"encoding/base64"
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
	assert.Equal(t, "servicenow", i.Name())
}

func TestConfigure_SuccessBasic(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"instance_url": "https://dev12345.service-now.com",
		"username":     "admin",
		"password":     "secret",
	})
	assert.NoError(t, err)
}

func TestConfigure_SuccessToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"instance_url": "dev12345.service-now.com",
		"access_token": "tok",
	})
	assert.NoError(t, err)
}

func TestConfigure_MissingInstance(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"username": "u", "password": "p"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "instance_url is required")
}

func TestConfigure_MissingAuth(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"instance_url": "https://example.service-now.com"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_token or username and password")
}

func TestConfigure_NormalizesInstance(t *testing.T) {
	s := &servicenow{client: &http.Client{}}
	err := s.Configure(context.Background(), mcp.Credentials{
		"instance_url": "dev12345",
		"access_token": "tok",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://dev12345.service-now.com", s.instanceURL)
}

func TestConfigure_TrimsTrailingSlash(t *testing.T) {
	s := &servicenow{client: &http.Client{}}
	err := s.Configure(context.Background(), mcp.Credentials{
		"instance_url": "https://dev12345.service-now.com/",
		"access_token": "tok",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://dev12345.service-now.com", s.instanceURL)
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
		assert.Contains(t, string(tool.Name), "servicenow_")
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
		if tool.Name == "servicenow_list_incidents" {
			assert.Contains(t, tool.Description, "Start here")
			found = true
		}
	}
	assert.True(t, found)
}

func TestExecute_UnknownTool(t *testing.T) {
	s := &servicenow{accessToken: "t", client: &http.Client{}, instanceURL: "http://localhost"}
	result, err := s.Execute(context.Background(), "servicenow_nonexistent", nil)
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

func configured(ts *httptest.Server) *servicenow {
	return &servicenow{username: "admin", password: "secret", client: ts.Client(), instanceURL: ts.URL}
}

func TestDoRequest_BasicAuth(t *testing.T) {
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret"))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, want, r.Header.Get("Authorization"))
		assert.Equal(t, "/api/now/table/incident", r.URL.Path)
		_, _ = w.Write([]byte(`{"result":[{"sys_id":"1"}]}`))
	}))
	defer ts.Close()

	s := configured(ts)
	data, err := s.get(context.Background(), "/api/now/table/incident")
	require.NoError(t, err)
	assert.Contains(t, string(data), `"sys_id":"1"`)
}

func TestDoRequest_BearerAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"result":[]}`))
	}))
	defer ts.Close()

	s := &servicenow{accessToken: "tok", client: ts.Client(), instanceURL: ts.URL}
	_, err := s.get(context.Background(), "/api/now/table/sys_user")
	require.NoError(t, err)
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"message":"User Not Authenticated"},"status":"failure"}`))
	}))
	defer ts.Close()

	s := configured(ts)
	_, err := s.get(context.Background(), "/api/now/table/incident")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "servicenow API error (401)")
}

func TestDoRequest_Retryable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`rate limited`))
	}))
	defer ts.Close()

	s := configured(ts)
	_, err := s.get(context.Background(), "/api/now/table/incident")
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

	s := configured(ts)
	data, err := s.doRequest(context.Background(), "PATCH", "/x", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestListIncidents_InvalidOffset(t *testing.T) {
	s := &servicenow{accessToken: "t", client: &http.Client{}, instanceURL: "http://localhost"}
	result, err := s.Execute(context.Background(), "servicenow_list_incidents", map[string]any{
		"offset": "abc",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "offset")
}

func TestListIncidents(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/now/table/incident", r.URL.Path)
		assert.Equal(t, "active=true", r.URL.Query().Get("sysparm_query"))
		assert.Equal(t, "10", r.URL.Query().Get("sysparm_limit"))
		assert.Equal(t, "all", r.URL.Query().Get("sysparm_display_value"))
		_, _ = w.Write([]byte(`{"result":[{"sys_id":"abc","number":"INC0010001","short_description":"VPN down"}]}`))
	}))
	defer ts.Close()

	s := configured(ts)
	result, err := s.Execute(context.Background(), "servicenow_list_incidents", map[string]any{
		"query": "active=true",
		"limit": 10,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "INC0010001")
}

func TestGetIncident(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/now/table/incident/abc", r.URL.Path)
		_, _ = w.Write([]byte(`{"result":{"sys_id":"abc","number":"INC0010001"}}`))
	}))
	defer ts.Close()

	s := configured(ts)
	result, err := s.Execute(context.Background(), "servicenow_get_incident", map[string]any{"sys_id": "abc"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "INC0010001")
}

func TestCreateIncident(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/now/table/incident", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Email outage", body["short_description"])
		assert.Equal(t, "1", body["urgency"])
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"result":{"sys_id":"n1","number":"INC0010099"}}`))
	}))
	defer ts.Close()

	s := configured(ts)
	result, err := s.Execute(context.Background(), "servicenow_create_incident", map[string]any{
		"short_description": "Email outage",
		"urgency":           "1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "INC0010099")
}

func TestCreateIncident_RequiresFields(t *testing.T) {
	s := &servicenow{accessToken: "t", client: &http.Client{}, instanceURL: "http://localhost"}
	result, err := s.Execute(context.Background(), "servicenow_create_incident", map[string]any{
		"short_description": "",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "at least one field")
}

func TestUpdateIncident(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/api/now/table/incident/abc", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "6", body["state"])
		_, _ = w.Write([]byte(`{"result":{"sys_id":"abc","state":"6"}}`))
	}))
	defer ts.Close()

	s := configured(ts)
	result, err := s.Execute(context.Background(), "servicenow_update_incident", map[string]any{
		"sys_id": "abc",
		"state":  "6",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, `"state":"6"`)
}

func TestUpdateIncident_RequiresFields(t *testing.T) {
	s := &servicenow{accessToken: "t", client: &http.Client{}, instanceURL: "http://localhost"}
	result, err := s.Execute(context.Background(), "servicenow_update_incident", map[string]any{"sys_id": "abc"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "at least one field")
}

func TestListRecords_InvalidTable(t *testing.T) {
	s := &servicenow{accessToken: "t", client: &http.Client{}, instanceURL: "http://localhost"}
	result, err := s.Execute(context.Background(), "servicenow_list_records", map[string]any{
		"table": "../etc/passwd",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid table")
}

func TestAggregate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/now/stats/incident", r.URL.Path)
		assert.Equal(t, "true", r.URL.Query().Get("sysparm_count"))
		assert.Equal(t, "priority", r.URL.Query().Get("sysparm_group_by"))
		_, _ = w.Write([]byte(`{"result":{"stats":{"count":"12"}}}`))
	}))
	defer ts.Close()

	s := configured(ts)
	result, err := s.Execute(context.Background(), "servicenow_aggregate", map[string]any{
		"table":    "incident",
		"group_by": "priority",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, `"count":"12"`)
}

func TestAddComment(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/api/now/table/incident/abc", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "looking into it", body["work_notes"])
		_, _ = w.Write([]byte(`{"result":{"sys_id":"abc"}}`))
	}))
	defer ts.Close()

	s := configured(ts)
	result, err := s.Execute(context.Background(), "servicenow_add_comment", map[string]any{
		"sys_id":    "abc",
		"work_note": "looking into it",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestAddComment_RequiresBody(t *testing.T) {
	s := &servicenow{accessToken: "t", client: &http.Client{}, instanceURL: "http://localhost"}
	result, err := s.Execute(context.Background(), "servicenow_add_comment", map[string]any{"sys_id": "abc"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "comment and/or work_note")
}

func TestListComments(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/now/table/sys_journal_field", r.URL.Path)
		assert.Contains(t, r.URL.Query().Get("sysparm_query"), "element_id=abc")
		_, _ = w.Write([]byte(`{"result":[{"sys_id":"j1","value":"hello","element":"work_notes"}]}`))
	}))
	defer ts.Close()

	s := configured(ts)
	result, err := s.Execute(context.Background(), "servicenow_list_comments", map[string]any{"sys_id": "abc"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "hello")
}

func TestListAttachments(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/now/table/sys_attachment", r.URL.Path)
		assert.Equal(t, "table_name=incident^table_sys_id=abc", r.URL.Query().Get("sysparm_query"))
		_, _ = w.Write([]byte(`{"result":[{"sys_id":"a1","file_name":"log.txt"}]}`))
	}))
	defer ts.Close()

	s := configured(ts)
	result, err := s.Execute(context.Background(), "servicenow_list_attachments", map[string]any{
		"table":  "incident",
		"sys_id": "abc",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "log.txt")
}

func TestListCIs_CustomTable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/now/table/cmdb_ci_linux_server", r.URL.Path)
		_, _ = w.Write([]byte(`{"result":[{"sys_id":"ci1","name":"web-1"}]}`))
	}))
	defer ts.Close()

	s := configured(ts)
	result, err := s.Execute(context.Background(), "servicenow_list_cis", map[string]any{
		"table": "cmdb_ci_linux_server",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "web-1")
}

func TestCreateRecord_RequiresFields(t *testing.T) {
	s := &servicenow{accessToken: "t", client: &http.Client{}, instanceURL: "http://localhost"}
	result, err := s.Execute(context.Background(), "servicenow_create_record", map[string]any{
		"table": "incident",
		"data":  `{}`,
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "at least one field")
}

func TestUpdateRecord_RequiresFields(t *testing.T) {
	s := &servicenow{accessToken: "t", client: &http.Client{}, instanceURL: "http://localhost"}
	result, err := s.Execute(context.Background(), "servicenow_update_record", map[string]any{
		"table":  "incident",
		"sys_id": "abc",
		"data":   `{}`,
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "at least one field")
}

func TestCreateRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/now/table/problem", r.URL.Path)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"result":{"sys_id":"p1"}}`))
	}))
	defer ts.Close()

	s := configured(ts)
	result, err := s.Execute(context.Background(), "servicenow_create_record", map[string]any{
		"table": "problem",
		"data":  `{"short_description":"recurring outage"}`,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "p1")
}

func TestHealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/now/table/sys_user", r.URL.Path)
		_, _ = w.Write([]byte(`{"result":[]}`))
	}))
	defer ts.Close()

	s := configured(ts)
	assert.True(t, s.Healthy(context.Background()))
}

func TestHealthy_Unreachable(t *testing.T) {
	s := &servicenow{client: &http.Client{}, instanceURL: "http://127.0.0.1:1"}
	assert.False(t, s.Healthy(context.Background()))
}

func TestNormalizeInstance(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"https://x.service-now.com/", "https://x.service-now.com"},
		{"http://localhost:8080", "http://localhost:8080"},
		{"acme.service-now.com", "https://acme.service-now.com"},
		{"acme", "https://acme.service-now.com"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeInstance(tt.in))
		})
	}
}
