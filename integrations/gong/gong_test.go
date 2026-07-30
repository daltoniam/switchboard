package gong

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
	assert.Equal(t, "gong", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"access_key":        "key",
		"access_key_secret": "secret",
	})
	assert.NoError(t, err)
}

func TestConfigure_MissingAccessKey(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"access_key_secret": "s"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_key is required")
}

func TestConfigure_MissingSecret(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"access_key": "k"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_key_secret is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	g := &gong{client: &http.Client{}, baseURL: "https://api.gong.io"}
	err := g.Configure(context.Background(), mcp.Credentials{
		"access_key":        "k",
		"access_key_secret": "s",
		"base_url":          "https://us-1234.api.gong.io/",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://us-1234.api.gong.io", g.baseURL)
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

func TestTools_AllHaveGongPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "gong_")
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
		if tool.Name == "gong_list_calls" {
			assert.Contains(t, tool.Description, "Start here")
			found = true
		}
	}
	assert.True(t, found)
}

func TestExecute_UnknownTool(t *testing.T) {
	g := &gong{accessKey: "k", accessKeySecret: "s", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gong_nonexistent", nil)
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

func TestDoRequest_BasicAuth(t *testing.T) {
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("key:secret"))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, want, r.Header.Get("Authorization"))
		assert.Equal(t, "/v2/users", r.URL.Path)
		_, _ = w.Write([]byte(`{"users":[{"id":"1"}]}`))
	}))
	defer ts.Close()

	g := &gong{accessKey: "key", accessKeySecret: "secret", client: ts.Client(), baseURL: ts.URL}
	data, err := g.get(context.Background(), "/v2/users")
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"1"`)
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"errors":["unauthorized"]}`))
	}))
	defer ts.Close()

	g := &gong{accessKey: "k", accessKeySecret: "s", client: ts.Client(), baseURL: ts.URL}
	_, err := g.get(context.Background(), "/v2/users")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gong API error (401)")
}

func TestDoRequest_Retryable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`rate limited`))
	}))
	defer ts.Close()

	g := &gong{accessKey: "k", accessKeySecret: "s", client: ts.Client(), baseURL: ts.URL}
	_, err := g.get(context.Background(), "/v2/users")
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

	g := &gong{accessKey: "k", accessKeySecret: "s", client: ts.Client(), baseURL: ts.URL}
	data, err := g.doRequest(context.Background(), "DELETE", "/x", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestListCalls(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/calls", r.URL.Path)
		assert.Equal(t, "2024-01-01T00:00:00Z", r.URL.Query().Get("fromDateTime"))
		assert.Equal(t, "2024-01-08T00:00:00Z", r.URL.Query().Get("toDateTime"))
		_, _ = w.Write([]byte(`{"calls":[{"id":"c1","title":"Demo"}]}`))
	}))
	defer ts.Close()

	g := &gong{accessKey: "k", accessKeySecret: "s", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gong_list_calls", map[string]any{
		"from_date_time": "2024-01-01T00:00:00Z",
		"to_date_time":   "2024-01-08T00:00:00Z",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Demo")
}

func TestGetCall(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/calls/c1", r.URL.Path)
		_, _ = w.Write([]byte(`{"call":{"id":"c1"}}`))
	}))
	defer ts.Close()

	g := &gong{accessKey: "k", accessKeySecret: "s", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gong_get_call", map[string]any{"call_id": "c1"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "c1")
}

func TestGetTranscripts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v2/calls/transcript", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		filter := body["filter"].(map[string]any)
		assert.Equal(t, "2024-01-01T00:00:00Z", filter["fromDateTime"])
		_, _ = w.Write([]byte(`{"callTranscripts":[{"callId":"c1"}]}`))
	}))
	defer ts.Close()

	g := &gong{accessKey: "k", accessKeySecret: "s", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gong_get_transcripts", map[string]any{
		"from_date_time": "2024-01-01T00:00:00Z",
		"to_date_time":   "2024-01-08T00:00:00Z",
		"call_ids":       `["c1"]`,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "c1")
}

func TestListCallsExtensive_DefaultSelector(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		cs, ok := body["contentSelector"].(map[string]any)
		require.True(t, ok)
		exposed, ok := cs["exposedFields"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, true, exposed["parties"])
		_, hasMedia := exposed["media"]
		assert.False(t, hasMedia)
		_, _ = w.Write([]byte(`{"calls":[]}`))
	}))
	defer ts.Close()

	g := &gong{accessKey: "k", accessKeySecret: "s", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gong_list_calls_extensive", map[string]any{
		"from_date_time": "2024-01-01T00:00:00Z",
		"to_date_time":   "2024-01-08T00:00:00Z",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestListCallsExtensive_ExplicitSelectorWins(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		cs := body["contentSelector"].(map[string]any)
		assert.Equal(t, true, cs["exposedFields"].(map[string]any)["media"])
		_, _ = w.Write([]byte(`{"calls":[]}`))
	}))
	defer ts.Close()

	g := &gong{accessKey: "k", accessKeySecret: "s", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gong_list_calls_extensive", map[string]any{
		"from_date_time":   "2024-01-01T00:00:00Z",
		"to_date_time":     "2024-01-08T00:00:00Z",
		"content_selector": `{"exposedFields":{"media":true}}`,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestListLogs_RequiresLogType(t *testing.T) {
	g := &gong{accessKey: "k", accessKeySecret: "s", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gong_list_logs", map[string]any{
		"from_date_time": "2024-01-01T00:00:00Z",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Contains(t, result.Data, "log_type")
}

func TestListLogs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/logs", r.URL.Path)
		assert.Equal(t, "AccessLog", r.URL.Query().Get("logType"))
		assert.Equal(t, "2024-01-01T00:00:00Z", r.URL.Query().Get("fromDateTime"))
		_, _ = w.Write([]byte(`{"logEntries":[]}`))
	}))
	defer ts.Close()

	g := &gong{accessKey: "k", accessKeySecret: "s", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gong_list_logs", map[string]any{
		"log_type":       "AccessLog",
		"from_date_time": "2024-01-01T00:00:00Z",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestGetDataPrivacy_Email(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/data-privacy/data-for-email-address", r.URL.Path)
		assert.Equal(t, "a@b.com", r.URL.Query().Get("emailAddress"))
		_, _ = w.Write([]byte(`{"requestId":"1"}`))
	}))
	defer ts.Close()

	g := &gong{accessKey: "k", accessKeySecret: "s", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gong_get_data_privacy", map[string]any{
		"email": "a@b.com",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestListStatsScorecards_Body(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		filter, ok := body["filter"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "2024-01-01", filter["callFromDate"])
		assert.Equal(t, "2024-01-31", filter["callToDate"])
		_, _ = w.Write([]byte(`{"answeredScorecards":[]}`))
	}))
	defer ts.Close()

	g := &gong{accessKey: "k", accessKeySecret: "s", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gong_list_stats_scorecards", map[string]any{
		"call_from_date": "2024-01-01",
		"call_to_date":   "2024-01-31",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestGetTranscripts_RequiresFilter(t *testing.T) {
	g := &gong{accessKey: "k", accessKeySecret: "s", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gong_get_transcripts", map[string]any{})
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Contains(t, result.Data, "from_date_time")
}

func TestHealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/v2/users")
		_, _ = w.Write([]byte(`{"users":[]}`))
	}))
	defer ts.Close()

	g := &gong{accessKey: "k", accessKeySecret: "s", client: ts.Client(), baseURL: ts.URL}
	assert.True(t, g.Healthy(context.Background()))
}

func TestHealthy_Unconfigured(t *testing.T) {
	g := &gong{client: &http.Client{}, baseURL: "http://127.0.0.1:1"}
	assert.False(t, g.Healthy(context.Background()))
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
