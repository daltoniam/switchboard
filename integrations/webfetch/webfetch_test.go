package webfetch

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	assert.Equal(t, "web", i.Name())
}

func TestConfigure(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.NoError(t, err)
}

func TestHealthy(t *testing.T) {
	i := New()
	assert.True(t, i.Healthy(context.Background()))
}

func TestTools(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.NotEmpty(t, tools)

	seen := make(map[mcp.ToolName]bool)
	for _, tool := range tools {
		assert.True(t, strings.HasPrefix(string(tool.Name), "web_"), "tool %s missing web_ prefix", tool.Name)
		assert.NotEmpty(t, tool.Description, "tool %s has no description", tool.Name)
		assert.False(t, seen[tool.Name], "duplicate tool: %s", tool.Name)
		seen[tool.Name] = true
	}
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

func TestExecute_UnknownTool(t *testing.T) {
	i := New()
	res, err := i.Execute(context.Background(), "web_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Data, "unknown tool")
}

func TestFetchURL_Success(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.Header.Get("User-Agent"), "switchboard-mcp")
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "hello world")
	}))
	defer srv.Close()

	wf := &webfetch{client: srv.Client(), allowPrivateAddr: true}
	res, err := wf.Execute(context.Background(), "web_fetch", map[string]any{
		"url": srv.URL,
	})
	require.NoError(t, err)
	assert.False(t, res.IsError)
	assert.Contains(t, res.Data, "hello world")
	assert.Contains(t, res.Data, "Source:")
	assert.Contains(t, res.Data, "Fetched:")
}

func TestFetchURL_HTMLExtraction(t *testing.T) {
	html := `<html><head><script>var x=1;</script><style>body{}</style></head>
<body><nav>menu</nav><main><h1>Title</h1><p>Content here</p>
<pre><code>func main() {}</code></pre></main><footer>footer</footer></body></html>`

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, html)
	}))
	defer srv.Close()

	wf := &webfetch{client: srv.Client(), allowPrivateAddr: true}
	res, err := wf.Execute(context.Background(), "web_fetch", map[string]any{
		"url": srv.URL,
	})
	require.NoError(t, err)
	assert.False(t, res.IsError)
	assert.Contains(t, res.Data, "Title")
	assert.Contains(t, res.Data, "Content here")
	assert.Contains(t, res.Data, "func main() {}")
	assert.NotContains(t, res.Data, "var x=1")
	assert.NotContains(t, res.Data, "<script")
}

func TestFetchURL_HTTP404(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	wf := &webfetch{client: srv.Client(), allowPrivateAddr: true}
	res, err := wf.Execute(context.Background(), "web_fetch", map[string]any{
		"url": srv.URL,
	})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Data, "404")
}

func TestFetchURL_MissingURL(t *testing.T) {
	i := New()
	res, err := i.Execute(context.Background(), "web_fetch", map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Data, "url parameter is required")
}

func TestFetchURL_HTTPSchemeRejected(t *testing.T) {
	i := New()
	res, err := i.Execute(context.Background(), "web_fetch", map[string]any{
		"url": "http://example.com",
	})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Data, "only https://")
}

func TestFetchURL_PrivateAddressRejected(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"localhost", "https://localhost/test"},
		{"loopback", "https://127.0.0.1/test"},
		{"private 10.x", "https://10.0.0.1/test"},
		{"private 192.168.x", "https://192.168.1.1/test"},
		{"private 172.16.x", "https://172.16.0.1/test"},
		{"ipv6 loopback", "https://[::1]/test"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			i := New()
			res, err := i.Execute(context.Background(), "web_fetch", map[string]any{
				"url": tc.url,
			})
			require.NoError(t, err)
			assert.True(t, res.IsError)
			assert.Contains(t, res.Data, "private/local addresses")
		})
	}
}

func TestFetchURL_Truncation(t *testing.T) {
	longContent := strings.Repeat("x", maxBody+100)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, longContent)
	}))
	defer srv.Close()

	wf := &webfetch{client: srv.Client(), allowPrivateAddr: true}
	res, err := wf.Execute(context.Background(), "web_fetch", map[string]any{
		"url": srv.URL,
	})
	require.NoError(t, err)
	assert.False(t, res.IsError)
	assert.Contains(t, res.Data, "[truncated")
}

func TestFetchURL_TimeoutClamped(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	wf := &webfetch{client: srv.Client(), allowPrivateAddr: true}
	res, err := wf.Execute(context.Background(), "web_fetch", map[string]any{
		"url":     srv.URL,
		"timeout": 999,
	})
	require.NoError(t, err)
	assert.False(t, res.IsError)
	assert.Contains(t, res.Data, "ok")
}

func TestIsPrivateHost(t *testing.T) {
	tests := []struct {
		host    string
		private bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"192.168.1.1", true},
		{"172.16.0.1", true},
		{"::1", true},
		{"8.8.8.8", false},
		{"example.com", false},
	}
	for _, tc := range tests {
		t.Run(tc.host, func(t *testing.T) {
			assert.Equal(t, tc.private, isPrivateHost(tc.host))
		})
	}
}

func TestIsPlainText(t *testing.T) {
	tests := []struct {
		ct    string
		url   string
		plain bool
	}{
		{"text/plain", "", true},
		{"text/markdown", "", true},
		{"application/json", "", true},
		{"text/html", "", false},
		{"", "https://example.com/file.md", true},
		{"", "https://example.com/file.txt", true},
		{"", "https://raw.githubusercontent.com/foo/bar/main/README.md", true},
		{"text/html", "https://example.com/page", false},
	}
	for _, tc := range tests {
		t.Run(tc.ct+"_"+tc.url, func(t *testing.T) {
			assert.Equal(t, tc.plain, isPlainText(tc.ct, tc.url))
		})
	}
}

func TestExtractReadableText(t *testing.T) {
	input := `<html><script>alert("xss")</script><style>.x{}</style>
<nav>nav</nav><h1>Hello</h1><p>World</p><pre><code>code_block</code></pre>
<footer>foot</footer></html>`

	out := extractReadableText(input)
	assert.Contains(t, out, "Hello")
	assert.Contains(t, out, "World")
	assert.Contains(t, out, "code_block")
	assert.NotContains(t, out, "alert")
	assert.NotContains(t, out, "<script")
	assert.NotContains(t, out, "nav")
	assert.NotContains(t, out, "foot")
}
