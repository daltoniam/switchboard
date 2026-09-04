package recoll

import (
	"context"
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
	assert.Equal(t, "recoll", i.Name())
}

func TestConfigure(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		i := New()
		err := i.Configure(context.Background(), mcp.Credentials{"base_url": "https://recoll.example.com/search/"})
		require.NoError(t, err)
		assert.Equal(t, "https://recoll.example.com/search", i.(*recoll).baseURL)
	})

	t.Run("missing base URL", func(t *testing.T) {
		err := New().Configure(context.Background(), mcp.Credentials{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "base_url is required")
	})
}

func TestTools(t *testing.T) {
	i := New()
	seen := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		assert.NotEmpty(t, tool.Name)
		assert.NotEmpty(t, tool.Description)
		assert.Contains(t, string(tool.Name), "recoll_")
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}

	search, ok := seen["recoll_search"]
	assert.True(t, ok && search)
	for _, tool := range i.Tools() {
		if tool.Name == "recoll_search" {
			assert.Contains(t, tool.Description, "Start here")
		}
	}
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range New().Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range New().Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	r := &recoll{baseURL: "http://localhost", client: http.DefaultClient}
	result, err := r.Execute(context.Background(), "recoll_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestSearch_RoundTrip(t *testing.T) {
	var gotQuery map[string][]string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, http.MethodGet, req.Method)
		assert.Equal(t, "/recoll/", req.URL.Path)
		gotQuery = req.URL.Query()
		_, _ = w.Write([]byte(`
			<html><body>
			<p>2 result(s) for <strong>quarterly &amp; report</strong></p>
			<div class="result"><div class="path">/documents/quarterly-report.pdf</div><div class="meta"> YXBwbGljYXRpb24vcGRm</div></div>
			<div class="result"><div class="path">/documents/notes.txt</div><div class="meta"> dGV4dC9wbGFpbg==</div></div>
			</body></html>`))
	}))
	t.Cleanup(ts.Close)

	r := configuredRecoll(t, ts.URL+"/recoll")
	result, err := r.Execute(context.Background(), "recoll_search", map[string]any{"query": "quarterly & report"})
	require.NoError(t, err)
	assert.False(t, result.IsError, result.Data)
	assert.Equal(t, []string{"quarterly & report"}, gotQuery["q"])
	assert.NotContains(t, gotQuery, "query")
	assert.JSONEq(t, `{
		"query":"quarterly & report",
		"count":2,
		"results":[
			{"path":"/documents/quarterly-report.pdf","mime_type":"application/pdf"},
			{"path":"/documents/notes.txt","mime_type":"text/plain"}
		]
	}`, result.Data)
}

func TestDoRequest(t *testing.T) {
	t.Run("API error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized"))
		}))
		t.Cleanup(ts.Close)

		_, err := configuredRecoll(t, ts.URL).get(context.Background(), "/json")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "recoll WebUI error (401)")
	})

	t.Run("empty response", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		t.Cleanup(ts.Close)

		data, err := configuredRecoll(t, ts.URL).get(context.Background(), "/json")
		require.NoError(t, err)
		assert.JSONEq(t, `{"status":"success"}`, string(data))
	})
}

func TestHealthy(t *testing.T) {
	r := New().(*recoll)
	assert.False(t, r.Healthy(context.Background()))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "/", req.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ts.Close)
	r = configuredRecoll(t, ts.URL)
	assert.True(t, r.Healthy(context.Background()))
}

func configuredRecoll(t *testing.T, baseURL string) *recoll {
	t.Helper()
	r := New().(*recoll)
	require.NoError(t, r.Configure(context.Background(), mcp.Credentials{"base_url": baseURL}))
	return r
}
