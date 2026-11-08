package obsidian

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func newTestServer(t *testing.T, handler http.HandlerFunc) *obsidian {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	o := New().(*obsidian)
	require.NoError(t, o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL, "api_key": testKey}))
	return o
}

func execute(t *testing.T, o *obsidian, tool string, args map[string]any) *mcp.ToolResult {
	t.Helper()
	result, err := o.Execute(context.Background(), mcp.ToolName(tool), args)
	require.NoError(t, err)
	return result
}

func decodeResult(t *testing.T, result *mcp.ToolResult) map[string]any {
	t.Helper()
	require.False(t, result.IsError, result.Data)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &out))
	return out
}

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "obsidian", i.Name())
}

func TestConfigure(t *testing.T) {
	tests := []struct {
		name     string
		creds    mcp.Credentials
		wantURL  string
		insecure bool
	}{
		{"default base_url", mcp.Credentials{"api_key": testKey}, "https://127.0.0.1:27124", false},
		{"http port", mcp.Credentials{"api_key": testKey, "base_url": "http://127.0.0.1:27123"}, "http://127.0.0.1:27123", false},
		{"trailing slash trimmed", mcp.Credentials{"api_key": testKey, "base_url": " https://obsidian.lan:27124/ "}, "https://obsidian.lan:27124", false},
		{"insecure skip verify", mcp.Credentials{"api_key": testKey, "insecure_skip_tls_verify": "true"}, "https://127.0.0.1:27124", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := New().(*obsidian)
			require.NoError(t, o.Configure(context.Background(), tt.creds))
			assert.Equal(t, tt.wantURL, o.baseURL)
			assert.Equal(t, testKey, o.apiKey)
			transport := o.client.Transport.(*http.Transport)
			assert.Equal(t, tt.insecure, transport.TLSClientConfig.InsecureSkipVerify)
		})
	}
}

func TestConfigure_CACert(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"status":"OK","authenticated":true}`)
	}))
	t.Cleanup(srv.Close)
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}))

	untrusted := New().(*obsidian)
	require.NoError(t, untrusted.Configure(context.Background(), mcp.Credentials{"api_key": testKey, "base_url": srv.URL}))
	assert.False(t, untrusted.Healthy(context.Background()), "a self-signed certificate must not be trusted by default")

	// A web UI text input drops line breaks, so the PEM may arrive with
	// escaped newlines or with none at all.
	forms := map[string]string{
		"multi-line":      certPEM,
		"escaped newline": strings.ReplaceAll(certPEM, "\n", `\n`),
		"single line":     strings.ReplaceAll(certPEM, "\n", ""),
	}
	for name, ca := range forms {
		t.Run(name, func(t *testing.T) {
			trusted := New().(*obsidian)
			require.NoError(t, trusted.Configure(context.Background(), mcp.Credentials{"api_key": testKey, "base_url": srv.URL, "ca_cert": ca}))
			assert.True(t, trusted.Healthy(context.Background()))
		})
	}
}

func TestConfigure_Errors(t *testing.T) {
	tests := []struct {
		name  string
		creds mcp.Credentials
		want  string
	}{
		{"missing api_key", mcp.Credentials{}, "api_key is required"},
		{"blank api_key", mcp.Credentials{"api_key": "  "}, "api_key is required"},
		{"no scheme", mcp.Credentials{"api_key": testKey, "base_url": "127.0.0.1:27124"}, "http(s) URL"},
		{"bad scheme", mcp.Credentials{"api_key": testKey, "base_url": "ftp://127.0.0.1"}, "http(s) URL"},
		{"bad ca_cert", mcp.Credentials{"api_key": testKey, "ca_cert": "not a pem"}, "ca_cert"},
		{"bad insecure flag", mcp.Credentials{"api_key": testKey, "insecure_skip_tls_verify": "maybe"}, "insecure_skip_tls_verify"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New().Configure(context.Background(), tt.creds)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestHealthy(t *testing.T) {
	tests := []struct {
		name string
		body string
		code int
		want bool
	}{
		{"authenticated", `{"status":"OK","authenticated":true}`, 200, true},
		{"key rejected", `{"status":"OK","authenticated":false}`, 200, false},
		{"server error", `oops`, 500, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotAuth string
			o := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("Authorization")
				assert.Equal(t, "/", r.URL.Path)
				w.WriteHeader(tt.code)
				_, _ = io.WriteString(w, tt.body)
			})
			assert.Equal(t, tt.want, o.Healthy(context.Background()))
			assert.Equal(t, "Bearer "+testKey, gotAuth)
		})
	}
}

func TestHealthy_NotConfigured(t *testing.T) {
	assert.False(t, New().Healthy(context.Background()))
}

func TestExecute_NotConfigured(t *testing.T) {
	o := New().(*obsidian)
	result := execute(t, o, "obsidian_list_files", nil)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "not configured")
}

func TestExecute_UnknownTool(t *testing.T) {
	result := execute(t, New().(*obsidian), "obsidian_nope", nil)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestCredentialHints(t *testing.T) {
	o := New().(*obsidian)
	assert.Equal(t, []string{"base_url", "ca_cert", "insecure_skip_tls_verify"}, o.PlainTextKeys())
	assert.Equal(t, []string{"base_url", "ca_cert", "insecure_skip_tls_verify"}, o.OptionalKeys())
	hints := o.Placeholders()
	for _, key := range []string{"base_url", "api_key", "ca_cert", "insecure_skip_tls_verify"} {
		assert.NotEmpty(t, hints[key], key)
	}
	assert.Contains(t, hints["base_url"], "27124")
}

func TestTools(t *testing.T) {
	tools := New().Tools()
	require.Len(t, tools, 13)
	seen := map[mcp.ToolName]bool{}
	startHere := 0
	for _, tool := range tools {
		assert.True(t, strings.HasPrefix(string(tool.Name), "obsidian_"), tool.Name)
		assert.NotEmpty(t, tool.Description, tool.Name)
		assert.True(t, tool.Access.Declared(), "%s has no Access", tool.Name)
		assert.False(t, seen[tool.Name], "duplicate tool %s", tool.Name)
		seen[tool.Name] = true
		for _, req := range tool.Required {
			assert.Contains(t, tool.Parameters, req, "%s requires undocumented parameter %s", tool.Name, req)
		}
		if strings.Contains(tool.Description, "Start here") {
			startHere++
		}
	}
	assert.Equal(t, 1, startHere, "exactly one entry-point tool says Start here")
}

func TestTools_Access(t *testing.T) {
	want := map[string]mcp.ToolAccess{
		"obsidian_list_files":       mcp.AccessRead,
		"obsidian_read_note":        mcp.AccessRead,
		"obsidian_get_document_map": mcp.AccessRead,
		"obsidian_search":           mcp.AccessRead,
		"obsidian_query":            mcp.AccessRead,
		"obsidian_list_tags":        mcp.AccessRead,
		"obsidian_list_commands":    mcp.AccessRead,
		"obsidian_get_active_note":  mcp.AccessRead,
		"obsidian_append_note":      mcp.AccessWrite,
		"obsidian_patch_note":       mcp.AccessWrite,
		"obsidian_write_note":       mcp.AccessDestructive,
		"obsidian_delete_note":      mcp.AccessDestructive,
		"obsidian_execute_command":  mcp.AccessDestructive,
	}
	for _, tool := range New().Tools() {
		assert.Equal(t, want[string(tool.Name)], tool.Access, tool.Name)
	}
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range New().Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	names := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		names[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, names[name], "dispatch handler %s has no tool definition", name)
	}
}

func TestAPIErrors(t *testing.T) {
	tests := []struct {
		name      string
		code      int
		body      string
		retryable bool
		want      string
	}{
		{"unauthorized", 401, `{"message":"Authorization required.","errorCode":40101}`, false, "api_key"},
		{"not found", 404, `{"message":"Not Found","errorCode":40400}`, false, "not found"},
		{"bad request", 400, `{"message":"Unknown or invalid Content-Type","errorCode":40012}`, false, "Unknown or invalid Content-Type"},
		{"server error", 500, `{"message":"boom","errorCode":50000}`, true, "boom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.code)
				_, _ = io.WriteString(w, tt.body)
			})
			result, err := o.Execute(context.Background(), "obsidian_read_note", map[string]any{"path": "Notes/a.md"})
			if tt.retryable {
				require.Error(t, err)
				assert.True(t, mcp.IsRetryable(err))
				assert.Contains(t, err.Error(), tt.want)
				return
			}
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, strings.ToLower(result.Data), strings.ToLower(tt.want))
		})
	}
}
