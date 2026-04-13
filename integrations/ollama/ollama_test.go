package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Constructor + Name ---

func TestNew(t *testing.T) {
	i := New()
	assert.NotNil(t, i)
	assert.Equal(t, "ollama", i.Name())
}

// --- Configure ---

func TestConfigure_DefaultBaseURL(t *testing.T) {
	o := New().(*ollama)
	err := o.Configure(context.Background(), mcp.Credentials{})
	require.NoError(t, err)
	assert.Equal(t, defaultBaseURL, o.baseURL)
	assert.Empty(t, o.apiKey)
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	o := New().(*ollama)
	err := o.Configure(context.Background(), mcp.Credentials{
		"base_url": "http://myhost:8080/",
	})
	require.NoError(t, err)
	assert.Equal(t, "http://myhost:8080", o.baseURL, "trailing slash should be trimmed")
}

func TestConfigure_WithAPIKey(t *testing.T) {
	o := New().(*ollama)
	err := o.Configure(context.Background(), mcp.Credentials{
		"api_key": "sk-test-123",
	})
	require.NoError(t, err)
	assert.Equal(t, "sk-test-123", o.apiKey)
}

// --- PlainTextKeys / OptionalKeys ---

func TestPlainTextKeys(t *testing.T) {
	o := New().(*ollama)
	assert.Equal(t, []string{"base_url"}, o.PlainTextKeys())
}

func TestOptionalKeys(t *testing.T) {
	o := New().(*ollama)
	assert.Equal(t, []string{"api_key"}, o.OptionalKeys())
}

// --- Tools metadata ---

func TestTools_AllHavePrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.True(t, strings.HasPrefix(string(tool.Name), "ollama_"),
			"tool %s does not have ollama_ prefix", tool.Name)
	}
}

func TestTools_AllHaveDescriptions(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.NotEmpty(t, tool.Description, "tool %s has no description", tool.Name)
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
	for _, tool := range i.Tools() {
		if tool.Name == "ollama_list_models" {
			assert.Contains(t, tool.Description, "Start here")
			return
		}
	}
	t.Fatal("ollama_list_models not found in tools")
}

// --- Dispatch parity ---

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

// --- Execute unknown tool ---

func TestExecute_UnknownTool(t *testing.T) {
	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{})
	result, err := o.Execute(context.Background(), "ollama_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

// --- Healthy ---

func TestHealthy_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/version" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(versionResponse{Version: "0.20.5"})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	assert.True(t, o.Healthy(context.Background()))
}

func TestHealthy_Unconfigured(t *testing.T) {
	o := &ollama{}
	assert.False(t, o.Healthy(context.Background()))
}

func TestHealthy_ServerDown(t *testing.T) {
	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": "http://localhost:1"})
	assert.False(t, o.Healthy(context.Background()))
}

// --- HTTP helpers ---

func TestHTTP_AuthHeaderPresent(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"version":"test"}`))
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL, "api_key": "sk-test"})
	_, _ = o.get(context.Background(), "/api/version")
	assert.Equal(t, "Bearer sk-test", gotAuth)
}

func TestHTTP_NoAuthHeaderWhenNoKey(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"version":"test"}`))
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	_, _ = o.get(context.Background(), "/api/version")
	assert.Empty(t, gotAuth)
}

func TestHTTP_400Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"model not found"}`))
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	_, err := o.get(context.Background(), "/api/show")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model not found")
	assert.False(t, mcp.IsRetryable(err))
}

func TestHTTP_404Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"error":"model 'nonexistent' not found"}`))
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	_, err := o.post(context.Background(), "/api/show", map[string]any{"model": "nonexistent"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.False(t, mcp.IsRetryable(err))
}

func TestHTTP_429Retryable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(429)
		w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	_, err := o.get(context.Background(), "/api/tags")
	assert.Error(t, err)
	assert.True(t, mcp.IsRetryable(err))
}

func TestHTTP_500Retryable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`internal error`))
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	_, err := o.get(context.Background(), "/api/tags")
	assert.Error(t, err)
	assert.True(t, mcp.IsRetryable(err))
}

func TestHTTP_204EmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	data, err := o.del(context.Background(), "/api/delete", map[string]any{"model": "test"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}
