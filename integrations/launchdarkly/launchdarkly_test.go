package launchdarkly

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
	assert.Equal(t, "launchdarkly", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"access_token": "api-token"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAccessToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_token is required")
}

func TestConfigure_RejectsSDKKeys(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"server sdk key", "sdk-11111111-2222-3333-4444-555555555555"},
		{"mobile key", "mob-11111111-2222-3333-4444-555555555555"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := New()
			err := i.Configure(context.Background(), mcp.Credentials{"access_token": tt.token})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "personal or service access token")
		})
	}
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	l := &launchdarkly{client: &http.Client{}, baseURL: defaultBaseURL}
	err := l.Configure(context.Background(), mcp.Credentials{
		"access_token": "api-token",
		"base_url":     "https://app.eu.launchdarkly.com/",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://app.eu.launchdarkly.com", l.baseURL)
}

func TestHealthy_Unconfigured(t *testing.T) {
	l := &launchdarkly{}
	assert.False(t, l.Healthy(context.Background()))
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

func TestTools_AllHaveLaunchDarklyPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "launchdarkly_")
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
		if tool.Name == "launchdarkly_list_projects" {
			assert.Contains(t, tool.Description, "Start here")
			found = true
		}
	}
	assert.True(t, found)
}

func TestExecute_UnknownTool(t *testing.T) {
	l := &launchdarkly{accessToken: "t", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := l.Execute(context.Background(), "launchdarkly_nonexistent", nil)
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

func configured(ts *httptest.Server) *launchdarkly {
	return &launchdarkly{accessToken: "api-token", client: ts.Client(), baseURL: ts.URL}
}

func TestDoRequest_TokenAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "api-token", r.Header.Get("Authorization"))
		assert.Equal(t, apiVersion, r.Header.Get("LD-API-Version"))
		assert.Equal(t, "/api/v2/projects", r.URL.Path)
		_, _ = w.Write([]byte(`{"items":[{"key":"default"}],"totalCount":1}`))
	}))
	defer ts.Close()

	l := configured(ts)
	data, err := l.get(context.Background(), "/projects")
	require.NoError(t, err)
	assert.Contains(t, string(data), `"key":"default"`)
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"code":"unauthorized","message":"Invalid access token"}`))
	}))
	defer ts.Close()

	l := configured(ts)
	_, err := l.get(context.Background(), "/projects")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "launchdarkly API error (401)")
	assert.Contains(t, err.Error(), "Invalid access token")
}

func TestDoRequest_Retryable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"code":"rate_limited","message":"slow down"}`))
	}))
	defer ts.Close()

	l := configured(ts)
	_, err := l.get(context.Background(), "/projects")
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

	l := configured(ts)
	data, err := l.doRequest(context.Background(), http.MethodDelete, "/x", "", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestHealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/projects", r.URL.Path)
		assert.Equal(t, "1", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"items":[],"totalCount":0}`))
	}))
	defer ts.Close()

	l := configured(ts)
	assert.True(t, l.Healthy(context.Background()))
}

func TestListProjects_Defaults(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/projects", r.URL.Path)
		assert.Equal(t, "20", r.URL.Query().Get("limit"))
		assert.Equal(t, "0", r.URL.Query().Get("offset"))
		assert.Empty(t, r.URL.Query().Get("filter"))
		_, _ = w.Write([]byte(`{"items":[{"_id":"p1","key":"default","name":"Default Project","tags":[]}],"totalCount":1}`))
	}))
	defer ts.Close()

	l := configured(ts)
	result, err := l.Execute(context.Background(), "launchdarkly_list_projects", nil)
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Default Project")
}

func TestListProjects_Filters(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "query:web", r.URL.Query().Get("filter"))
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		assert.Equal(t, "30", r.URL.Query().Get("offset"))
		_, _ = w.Write([]byte(`{"items":[],"totalCount":0}`))
	}))
	defer ts.Close()

	l := configured(ts)
	result, err := l.Execute(context.Background(), "launchdarkly_list_projects", map[string]any{
		"query":  "web",
		"limit":  10,
		"offset": 30,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestListEnvironments(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/projects/my-proj/environments", r.URL.Path)
		assert.Equal(t, "20", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"items":[{"_id":"e1","key":"production","name":"Production","color":"417505","critical":true}],"totalCount":1}`))
	}))
	defer ts.Close()

	l := configured(ts)
	result, err := l.Execute(context.Background(), "launchdarkly_list_environments", map[string]any{"project_key": "my-proj"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Production")
}

func TestListEnvironments_MissingProjectKey(t *testing.T) {
	l := &launchdarkly{accessToken: "t", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := l.Execute(context.Background(), "launchdarkly_list_environments", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "project_key")
}

func TestListFlags_Defaults(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/flags/my-proj", r.URL.Path)
		assert.Equal(t, "20", r.URL.Query().Get("limit"))
		assert.Equal(t, "0", r.URL.Query().Get("offset"))
		assert.Empty(t, r.URL.Query().Get("env"))
		_, _ = w.Write([]byte(`{"items":[{"key":"dark-mode","name":"Dark Mode","kind":"boolean","environments":{"production":{"on":true}}}],"totalCount":1}`))
	}))
	defer ts.Close()

	l := configured(ts)
	result, err := l.Execute(context.Background(), "launchdarkly_list_flags", map[string]any{"project_key": "my-proj"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "dark-mode")
}

func TestListFlags_Filters(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, "production", q.Get("env"))
		assert.Equal(t, "query:dark", q.Get("filter"))
		assert.Equal(t, "beta", q.Get("tag"))
		assert.Equal(t, "-creationDate", q.Get("sort"))
		assert.Equal(t, "5", q.Get("limit"))
		assert.Equal(t, "10", q.Get("offset"))
		_, _ = w.Write([]byte(`{"items":[],"totalCount":0}`))
	}))
	defer ts.Close()

	l := configured(ts)
	result, err := l.Execute(context.Background(), "launchdarkly_list_flags", map[string]any{
		"project_key":     "my-proj",
		"environment_key": "production",
		"query":           "dark",
		"tag":             "beta",
		"sort":            "-creationDate",
		"limit":           5,
		"offset":          10,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestListFlags_MissingProjectKey(t *testing.T) {
	l := &launchdarkly{accessToken: "t", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := l.Execute(context.Background(), "launchdarkly_list_flags", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "project_key")
}

func TestGetFlag(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/flags/my-proj/dark-mode", r.URL.Path)
		assert.Equal(t, "production", r.URL.Query().Get("env"))
		_, _ = w.Write([]byte(`{"key":"dark-mode","name":"Dark Mode","environments":{"production":{"on":true,"version":3}}}`))
	}))
	defer ts.Close()

	l := configured(ts)
	result, err := l.Execute(context.Background(), "launchdarkly_get_flag", map[string]any{
		"project_key":     "my-proj",
		"flag_key":        "dark-mode",
		"environment_key": "production",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Dark Mode")
}

func TestGetFlag_MissingArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{"missing project_key", map[string]any{"flag_key": "f"}, "project_key"},
		{"missing flag_key", map[string]any{"project_key": "p"}, "flag_key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &launchdarkly{accessToken: "t", client: &http.Client{}, baseURL: "http://localhost"}
			result, err := l.Execute(context.Background(), "launchdarkly_get_flag", tt.args)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tt.want)
		})
	}
}

func TestListFlagStatuses(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/flag-statuses/my-proj/production", r.URL.Path)
		_, _ = w.Write([]byte(`{"items":[{"name":"active","lastRequested":"2026-01-01T00:00:00Z","default":false,"_links":{"parent":{"href":"/api/v2/flags/my-proj/dark-mode"}}}]}`))
	}))
	defer ts.Close()

	l := configured(ts)
	result, err := l.Execute(context.Background(), "launchdarkly_list_flag_statuses", map[string]any{
		"project_key":     "my-proj",
		"environment_key": "production",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "active")
}

func TestListFlagStatuses_MissingArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{"missing project_key", map[string]any{"environment_key": "production"}, "project_key"},
		{"missing environment_key", map[string]any{"project_key": "p"}, "environment_key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &launchdarkly{accessToken: "t", client: &http.Client{}, baseURL: "http://localhost"}
			result, err := l.Execute(context.Background(), "launchdarkly_list_flag_statuses", tt.args)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tt.want)
		})
	}
}

func TestToggleFlag(t *testing.T) {
	tests := []struct {
		name     string
		on       any
		wantKind string
	}{
		{"turn on", true, "turnFlagOn"},
		{"turn off", false, "turnFlagOff"},
		{"string true", "true", "turnFlagOn"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPatch, r.Method)
				assert.Equal(t, "/api/v2/flags/my-proj/dark-mode", r.URL.Path)
				assert.Equal(t, semanticPatchContentType, r.Header.Get("Content-Type"))
				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, "production", body["environmentKey"])
				assert.Equal(t, "ops toggle", body["comment"])
				instructions := body["instructions"].([]any)
				require.Len(t, instructions, 1)
				assert.Equal(t, tt.wantKind, instructions[0].(map[string]any)["kind"])
				_, _ = w.Write([]byte(`{"key":"dark-mode","environments":{"production":{"on":true,"version":4,"lastModified":1700000002000,"salt":"SECRET","sel":"SECRET2"}}}`))
			}))
			defer ts.Close()

			l := configured(ts)
			result, err := l.Execute(context.Background(), "launchdarkly_toggle_flag", map[string]any{
				"project_key":     "my-proj",
				"flag_key":        "dark-mode",
				"environment_key": "production",
				"on":              tt.on,
				"comment":         "ops toggle",
			})
			require.NoError(t, err)
			require.False(t, result.IsError)
			var confirmation map[string]any
			require.NoError(t, json.Unmarshal([]byte(result.Data), &confirmation))
			assert.Equal(t, "dark-mode", confirmation["key"])
			assert.Equal(t, "production", confirmation["environment_key"])
			assert.Equal(t, true, confirmation["on"])
			assert.Equal(t, float64(4), confirmation["version"])
			assert.Equal(t, float64(1700000002000), confirmation["lastModified"])
			assert.NotContains(t, result.Data, "SECRET")
		})
	}
}

func TestToggleFlag_MissingEnvironmentInResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"key":"dark-mode","environments":{"staging":{"on":false,"salt":"SECRET"}}}`))
	}))
	defer ts.Close()

	l := configured(ts)
	result, err := l.Execute(context.Background(), "launchdarkly_toggle_flag", map[string]any{
		"project_key":     "my-proj",
		"flag_key":        "dark-mode",
		"environment_key": "production",
		"on":              true,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	var confirmation map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &confirmation))
	assert.Equal(t, "dark-mode", confirmation["key"])
	assert.Equal(t, "production", confirmation["environment_key"])
	_, hasOn := confirmation["on"]
	assert.False(t, hasOn, "on must not be fabricated when the environment is absent from the response")
	assert.NotContains(t, result.Data, "SECRET")
}

func TestListFlags_ScrubsEnvironmentSecrets(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"key":"dark-mode","environments":{"production":{"on":true,"version":3,"salt":"SECRET","sel":"SECRET2","_site":{"href":"/ui"}}}}],"totalCount":1}`))
	}))
	defer ts.Close()

	l := configured(ts)
	result, err := l.Execute(context.Background(), "launchdarkly_list_flags", map[string]any{"project_key": "my-proj"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.NotContains(t, result.Data, "SECRET")
	assert.NotContains(t, result.Data, "_site")
	assert.Contains(t, result.Data, `"on":true`)
	assert.Contains(t, result.Data, `"version":3`)
	assert.Contains(t, result.Data, `"totalCount":1`)
}

func TestGetFlag_ScrubsEnvironmentSecrets(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"key":"dark-mode","environments":{"production":{"on":true,"version":3,"salt":"SECRET","sel":"SECRET2","_site":{"href":"/ui"}},"staging":{"on":false,"salt":"SECRET3"}}}`))
	}))
	defer ts.Close()

	l := configured(ts)
	result, err := l.Execute(context.Background(), "launchdarkly_get_flag", map[string]any{"project_key": "my-proj", "flag_key": "dark-mode"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.NotContains(t, result.Data, "SECRET")
	assert.NotContains(t, result.Data, "_site")
	assert.Contains(t, result.Data, `"on":true`)
	assert.Contains(t, result.Data, `"staging"`)
}

func TestToggleFlag_OmitsEmptyComment(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_, hasComment := body["comment"]
		assert.False(t, hasComment)
		_, _ = w.Write([]byte(`{"key":"dark-mode"}`))
	}))
	defer ts.Close()

	l := configured(ts)
	result, err := l.Execute(context.Background(), "launchdarkly_toggle_flag", map[string]any{
		"project_key":     "my-proj",
		"flag_key":        "dark-mode",
		"environment_key": "production",
		"on":              false,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestToggleFlag_MissingArgs(t *testing.T) {
	base := map[string]any{
		"project_key":     "my-proj",
		"flag_key":        "dark-mode",
		"environment_key": "production",
		"on":              true,
	}
	tests := []struct {
		name string
		drop string
	}{
		{"missing project_key", "project_key"},
		{"missing flag_key", "flag_key"},
		{"missing environment_key", "environment_key"},
		{"missing on", "on"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := make(map[string]any, len(base))
			for k, v := range base {
				if k != tt.drop {
					args[k] = v
				}
			}
			l := &launchdarkly{accessToken: "t", client: &http.Client{}, baseURL: "http://localhost"}
			result, err := l.Execute(context.Background(), "launchdarkly_toggle_flag", args)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tt.drop)
		})
	}
}

func TestToggleFlag_InvalidOn(t *testing.T) {
	l := &launchdarkly{accessToken: "t", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := l.Execute(context.Background(), "launchdarkly_toggle_flag", map[string]any{
		"project_key":     "my-proj",
		"flag_key":        "dark-mode",
		"environment_key": "production",
		"on":              "maybe",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "on")
}
