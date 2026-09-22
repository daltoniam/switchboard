package pagerduty

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
	assert.Equal(t, "pagerduty", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_token": "token"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAPIToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_token is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	p := &pagerduty{client: &http.Client{}, baseURL: defaultBaseURL}
	err := p.Configure(context.Background(), mcp.Credentials{
		"api_token":  "token",
		"from_email": "oncall@example.com",
		"base_url":   "https://api.eu.pagerduty.com/",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://api.eu.pagerduty.com", p.baseURL)
	assert.Equal(t, "oncall@example.com", p.fromEmail)
}

func TestHealthy_Unconfigured(t *testing.T) {
	p := &pagerduty{}
	assert.False(t, p.Healthy(context.Background()))
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

func TestTools_AllHavePagerDutyPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "pagerduty_")
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
		if tool.Name == "pagerduty_list_incidents" {
			assert.Contains(t, tool.Description, "Start here")
			found = true
		}
	}
	assert.True(t, found)
}

func TestExecute_UnknownTool(t *testing.T) {
	p := &pagerduty{apiToken: "t", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := p.Execute(context.Background(), "pagerduty_nonexistent", nil)
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

func configured(ts *httptest.Server) *pagerduty {
	return &pagerduty{apiToken: "pd-token", client: ts.Client(), baseURL: ts.URL}
}

func TestDoRequest_TokenAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Token token=pd-token", r.Header.Get("Authorization"))
		assert.Equal(t, apiAccept, r.Header.Get("Accept"))
		assert.Equal(t, "/incidents", r.URL.Path)
		_, _ = w.Write([]byte(`{"incidents":[{"id":"P1"}]}`))
	}))
	defer ts.Close()

	p := configured(ts)
	data, err := p.get(context.Background(), "/incidents")
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"P1"`)
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"message":"Unauthorized"}}`))
	}))
	defer ts.Close()

	p := configured(ts)
	_, err := p.get(context.Background(), "/incidents")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pagerduty API error (401)")
}

func TestDoRequest_Retryable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`rate limited`))
	}))
	defer ts.Close()

	p := configured(ts)
	_, err := p.get(context.Background(), "/incidents")
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

	p := configured(ts)
	data, err := p.doRequest(context.Background(), http.MethodDelete, "/x", "", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestHealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/incidents", r.URL.Path)
		assert.Equal(t, "1", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"incidents":[]}`))
	}))
	defer ts.Close()

	p := configured(ts)
	assert.True(t, p.Healthy(context.Background()))
}

func TestListIncidents_DefaultStatuses(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/incidents", r.URL.Path)
		assert.Equal(t, []string{"triggered", "acknowledged"}, r.URL.Query()["statuses[]"])
		assert.Equal(t, "25", r.URL.Query().Get("limit"))
		assert.Equal(t, "0", r.URL.Query().Get("offset"))
		_, _ = w.Write([]byte(`{"incidents":[{"id":"P1","title":"DB down","status":"triggered"}],"limit":25,"offset":0,"more":false}`))
	}))
	defer ts.Close()

	p := configured(ts)
	result, err := p.Execute(context.Background(), "pagerduty_list_incidents", nil)
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "DB down")
}

func TestListIncidents_Filters(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, []string{"resolved"}, r.URL.Query()["statuses[]"])
		assert.Equal(t, []string{"high"}, r.URL.Query()["urgencies[]"])
		assert.Equal(t, []string{"PABC123"}, r.URL.Query()["service_ids[]"])
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		assert.Equal(t, "20", r.URL.Query().Get("offset"))
		_, _ = w.Write([]byte(`{"incidents":[]}`))
	}))
	defer ts.Close()

	p := configured(ts)
	result, err := p.Execute(context.Background(), "pagerduty_list_incidents", map[string]any{
		"statuses":    "resolved",
		"urgencies":   "high",
		"service_ids": "PABC123",
		"limit":       10,
		"offset":      20,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestGetIncident(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/incidents/P1", r.URL.Path)
		_, _ = w.Write([]byte(`{"incident":{"id":"P1","title":"DB down"}}`))
	}))
	defer ts.Close()

	p := configured(ts)
	result, err := p.Execute(context.Background(), "pagerduty_get_incident", map[string]any{"incident_id": "P1"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "P1")
}

func TestGetIncident_MissingID(t *testing.T) {
	p := &pagerduty{apiToken: "t", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := p.Execute(context.Background(), "pagerduty_get_incident", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "incident_id")
}

func TestListOncalls(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/oncalls", r.URL.Path)
		assert.Equal(t, "25", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"oncalls":[{"escalation_level":1,"user":{"id":"U1","summary":"Ada"}}]}`))
	}))
	defer ts.Close()

	p := configured(ts)
	result, err := p.Execute(context.Background(), "pagerduty_list_oncalls", nil)
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Ada")
}

func TestListServices(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/services", r.URL.Path)
		assert.Equal(t, "api", r.URL.Query().Get("query"))
		_, _ = w.Write([]byte(`{"services":[{"id":"S1","name":"API"}]}`))
	}))
	defer ts.Close()

	p := configured(ts)
	result, err := p.Execute(context.Background(), "pagerduty_list_services", map[string]any{"query": "api"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "API")
}

func TestAddIncidentNote(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/incidents/P1/notes", r.URL.Path)
		assert.Equal(t, "oncall@example.com", r.Header.Get("From"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		note := body["note"].(map[string]any)
		assert.Equal(t, "mitigating", note["content"])
		_, _ = w.Write([]byte(`{"note":{"id":"N1","content":"mitigating"}}`))
	}))
	defer ts.Close()

	p := configured(ts)
	result, err := p.Execute(context.Background(), "pagerduty_add_incident_note", map[string]any{
		"incident_id": "P1",
		"content":     "mitigating",
		"from_email":  "oncall@example.com",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "mitigating")
}

func TestAddIncidentNote_UsesConfiguredFrom(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "ops@example.com", r.Header.Get("From"))
		_, _ = w.Write([]byte(`{"note":{"id":"N1"}}`))
	}))
	defer ts.Close()

	p := configured(ts)
	p.fromEmail = "ops@example.com"
	result, err := p.Execute(context.Background(), "pagerduty_add_incident_note", map[string]any{
		"incident_id": "P1",
		"content":     "noted",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestAddIncidentNote_RequiresFrom(t *testing.T) {
	p := &pagerduty{apiToken: "t", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := p.Execute(context.Background(), "pagerduty_add_incident_note", map[string]any{
		"incident_id": "P1",
		"content":     "noted",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "from_email")
}

func TestAcknowledgeIncident(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/incidents/P1", r.URL.Path)
		assert.Equal(t, "oncall@example.com", r.Header.Get("From"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		inc := body["incident"].(map[string]any)
		assert.Equal(t, "incident", inc["type"])
		assert.Equal(t, "acknowledged", inc["status"])
		_, _ = w.Write([]byte(`{"incident":{"id":"P1","status":"acknowledged"}}`))
	}))
	defer ts.Close()

	p := configured(ts)
	result, err := p.Execute(context.Background(), "pagerduty_acknowledge_incident", map[string]any{
		"incident_id": "P1",
		"from_email":  "oncall@example.com",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "acknowledged")
}

func TestResolveIncident(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/incidents/P1", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		inc := body["incident"].(map[string]any)
		assert.Equal(t, "resolved", inc["status"])
		_, _ = w.Write([]byte(`{"incident":{"id":"P1","status":"resolved"}}`))
	}))
	defer ts.Close()

	p := configured(ts)
	result, err := p.Execute(context.Background(), "pagerduty_resolve_incident", map[string]any{
		"incident_id": "P1",
		"from_email":  "oncall@example.com",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "resolved")
}
