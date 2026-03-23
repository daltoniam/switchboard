package mixpanel

import (
	"context"
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
	assert.Equal(t, "mixpanel", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"username":   "svc-account",
		"secret":     "test-secret",
		"project_id": "12345",
	})
	assert.NoError(t, err)
}

func TestConfigure_MissingUsername(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"username":   "",
		"secret":     "test-secret",
		"project_id": "12345",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "username is required")
}

func TestConfigure_MissingSecret(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"username":   "svc-account",
		"secret":     "",
		"project_id": "12345",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secret is required")
}

func TestConfigure_MissingProjectID(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"username":   "svc-account",
		"secret":     "test-secret",
		"project_id": "",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project_id is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	mp := &mixpanel{client: &http.Client{}, baseURL: "https://mixpanel.com/api/query"}
	err := mp.Configure(context.Background(), mcp.Credentials{
		"username":   "svc-account",
		"secret":     "test-secret",
		"project_id": "12345",
		"base_url":   "https://eu.mixpanel.com/api/query/",
	})
	assert.NoError(t, err)
	assert.Equal(t, "https://eu.mixpanel.com/api/query", mp.baseURL)
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

func TestTools_AllHaveMixpanelPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "mixpanel_", "tool %s missing mixpanel_ prefix", tool.Name)
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
	mp := &mixpanel{username: "u", secret: "s", projectID: "1", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := mp.Execute(context.Background(), "mixpanel_nonexistent", nil)
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

func TestDoGet_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "testuser", user)
		assert.Equal(t, "testsecret", pass)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"123"}`))
	}))
	defer ts.Close()

	mp := &mixpanel{username: "testuser", secret: "testsecret", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	data, err := mp.doGet(context.Background(), "/test", map[string]string{})
	require.NoError(t, err)
	assert.Contains(t, string(data), "123")
}

func TestDoGet_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer ts.Close()

	mp := &mixpanel{username: "u", secret: "s", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	_, err := mp.doGet(context.Background(), "/test", map[string]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mixpanel API error (403)")
}

func TestDoGet_RetryableError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer ts.Close()

	mp := &mixpanel{username: "u", secret: "s", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	_, err := mp.doGet(context.Background(), "/test", map[string]string{})
	assert.Error(t, err)
	var re *mcp.RetryableError
	assert.ErrorAs(t, err, &re)
	assert.Equal(t, 429, re.StatusCode)
}

func TestDoPostForm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		assert.NoError(t, r.ParseForm())
		assert.Equal(t, "user123", r.PostForm.Get("distinct_id"))
		_, _ = w.Write([]byte(`{"results":[],"total":0}`))
	}))
	defer ts.Close()

	mp := &mixpanel{username: "u", secret: "s", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	form := make(map[string][]string)
	form["distinct_id"] = []string{"user123"}
	data, err := mp.doPostForm(context.Background(), "/engage", map[string]string{}, form)
	require.NoError(t, err)
	assert.Contains(t, string(data), "results")
}

// --- result helper tests ---

func TestRawResult(t *testing.T) {
	data := []byte(`{"key":"value"}`)
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

func TestProjFromArgs(t *testing.T) {
	mp := &mixpanel{projectID: "default-proj"}

	assert.Equal(t, "default-proj", mp.projFromArgs(map[string]any{}))
	assert.Equal(t, "custom-proj", mp.projFromArgs(map[string]any{"project_id": "custom-proj"}))
}

// --- handler integration tests ---

func TestQueryInsights(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/insights")
		assert.Equal(t, "12345", r.URL.Query().Get("bookmark_id"))
		assert.Equal(t, "1", r.URL.Query().Get("project_id"))
		_, _ = w.Write([]byte(`{"results":{"series":["2024-01-01"]}}`))
	}))
	defer ts.Close()

	mp := &mixpanel{username: "u", secret: "s", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := mp.Execute(context.Background(), "mixpanel_query_insights", map[string]any{
		"bookmark_id": "12345",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "series")
}

func TestQueryFunnels(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/funnels")
		assert.Equal(t, "99", r.URL.Query().Get("funnel_id"))
		assert.Equal(t, "2024-01-01", r.URL.Query().Get("from_date"))
		assert.Equal(t, "2024-01-31", r.URL.Query().Get("to_date"))
		_, _ = w.Write([]byte(`{"meta":{},"data":{"2024-01-01":{"steps":[]}}}`))
	}))
	defer ts.Close()

	mp := &mixpanel{username: "u", secret: "s", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := mp.Execute(context.Background(), "mixpanel_query_funnels", map[string]any{
		"funnel_id": "99",
		"from_date": "2024-01-01",
		"to_date":   "2024-01-31",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "steps")
}

func TestQueryRetention(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/retention")
		assert.Equal(t, "2024-01-01", r.URL.Query().Get("from_date"))
		_, _ = w.Write([]byte(`{"results":{"2024-01-01":{"counts":[100,50,25]}}}`))
	}))
	defer ts.Close()

	mp := &mixpanel{username: "u", secret: "s", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := mp.Execute(context.Background(), "mixpanel_query_retention", map[string]any{
		"from_date": "2024-01-01",
		"to_date":   "2024-01-31",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "counts")
}

func TestQuerySegmentation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/segmentation")
		assert.Equal(t, "signup", r.URL.Query().Get("event"))
		_, _ = w.Write([]byte(`{"data":{"series":["2024-01-01"],"values":{"signup":{"2024-01-01":42}}},"legend_size":1}`))
	}))
	defer ts.Close()

	mp := &mixpanel{username: "u", secret: "s", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := mp.Execute(context.Background(), "mixpanel_query_segmentation", map[string]any{
		"event":     "signup",
		"from_date": "2024-01-01",
		"to_date":   "2024-01-31",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "signup")
}

func TestQueryEventProperties(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/events/properties")
		assert.Equal(t, "pageview", r.URL.Query().Get("event"))
		assert.Equal(t, "browser", r.URL.Query().Get("name"))
		_, _ = w.Write([]byte(`{"data":{"series":["2024-01-01"],"values":{"Chrome":{"2024-01-01":100}}},"legend_size":1}`))
	}))
	defer ts.Close()

	mp := &mixpanel{username: "u", secret: "s", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := mp.Execute(context.Background(), "mixpanel_query_event_properties", map[string]any{
		"event":     "pageview",
		"name":      "browser",
		"from_date": "2024-01-01",
		"to_date":   "2024-01-31",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Chrome")
}

func TestQueryProfiles(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		assert.NoError(t, r.ParseForm())
		assert.Equal(t, "user-abc", r.PostForm.Get("distinct_id"))
		_, _ = w.Write([]byte(`{"page":0,"page_size":100,"session_id":"abc","status":"ok","total":1,"results":[{"$distinct_id":"user-abc","$properties":{"name":"Alice"}}]}`))
	}))
	defer ts.Close()

	mp := &mixpanel{username: "u", secret: "s", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := mp.Execute(context.Background(), "mixpanel_query_profiles", map[string]any{
		"distinct_id": "user-abc",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "user-abc")
	assert.Contains(t, result.Data, "Alice")
}

func TestQueryProfiles_Pagination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.NoError(t, r.ParseForm())
		assert.Equal(t, "session-123", r.PostForm.Get("session_id"))
		assert.Equal(t, "2", r.PostForm.Get("page"))
		_, _ = w.Write([]byte(`{"page":2,"page_size":100,"session_id":"session-123","status":"ok","total":300,"results":[]}`))
	}))
	defer ts.Close()

	mp := &mixpanel{username: "u", secret: "s", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := mp.Execute(context.Background(), "mixpanel_query_profiles", map[string]any{
		"session_id": "session-123",
		"page":       "2",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "session-123")
}

func TestProjectOverride(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "99", r.URL.Query().Get("project_id"))
		_, _ = w.Write([]byte(`{"results":{}}`))
	}))
	defer ts.Close()

	mp := &mixpanel{username: "u", secret: "s", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := mp.Execute(context.Background(), "mixpanel_query_insights", map[string]any{
		"bookmark_id": "1",
		"project_id":  "99",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}
