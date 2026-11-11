package openbao

import (
	"context"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const metadataFixture = `{"data":{"current_version":2,"oldest_version":1,"created_time":"2026-01-01T00:00:00Z","updated_time":"2026-01-02T00:00:00Z","custom_metadata":{"password":"SECRET_SENTINEL"},"data":{"password":"SECRET_SENTINEL"},"versions":{"1":{"created_time":"2026-01-01T00:00:00Z","deletion_time":"","destroyed":false,"annotation":"SECRET_SENTINEL"},"2":{"created_time":"2026-01-02T00:00:00Z","deletion_time":"","destroyed":false}}},"warnings":["SECRET_SENTINEL"]}`

func testAdapter(t *testing.T, handler http.HandlerFunc) *openbao {
	t.Helper()
	provider := httptest.NewTLSServer(handler)
	t.Cleanup(provider.Close)
	adapter := New().(*openbao)
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials{
		"base_url": provider.URL, "token": "test-token", "mount": "secret", "path_prefix": "approved/app",
		"ca_cert": strings.ReplaceAll(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})), "\n", `\n`),
	}))
	return adapter
}

func execute(t *testing.T, adapter mcp.Integration, name mcp.ToolName, args map[string]any) *mcp.ToolResult {
	t.Helper()
	result, err := adapter.Execute(t.Context(), name, args)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func TestConfigure(t *testing.T) {
	valid := mcp.Credentials{"base_url": "https://bao.example.com/bao", "token": "short-lived", "mount": "secret", "path_prefix": "approved/app"}
	cases := []struct{ key, value string }{
		{"base_url", ""}, {"token", ""}, {"mount", ""}, {"path_prefix", ""},
		{"base_url", "http://bao.example.com"}, {"base_url", "https://user:pass@bao.example.com"},
		{"base_url", "https://bao.example.com?token=secret"}, {"base_url", "https://bao.example.com/#fragment"},
		{"base_url", "https://bao.example.com/a/../b"}, {"base_url", "https://bao.example.com/%2fsecret"},
		{"mount", "../secret"}, {"mount", "secret/data"}, {"path_prefix", "/"},
		{"path_prefix", "approved/../other"}, {"path_prefix", "approved%2fother"},
		{"path_prefix", "approved//app"}, {"path_prefix", "approved\\app"},
		{"token", "bad\nheader"}, {"ca_cert", "not a certificate"},
	}
	for _, test := range cases {
		t.Run(test.key+"="+test.value, func(t *testing.T) {
			creds := mcp.Credentials{}
			for key, value := range valid {
				creds[key] = value
			}
			creds[test.key] = test.value
			adapter := New()
			require.Error(t, adapter.Configure(t.Context(), creds))
			assert.False(t, adapter.Healthy(t.Context()))
		})
	}
	adapter := New()
	require.NoError(t, adapter.Configure(t.Context(), valid))
	assert.Equal(t, "openbao", adapter.Name())
	assert.False(t, New().Healthy(t.Context()))
	assert.True(t, execute(t, New(), "openbao_get_health", nil).IsError)
	assert.True(t, execute(t, adapter, "openbao_unknown", nil).IsError)
	require.Error(t, adapter.Configure(t.Context(), nil))
	assert.True(t, execute(t, adapter, "openbao_get_health", nil).IsError)
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	seen := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, string(tool.Name))
		assert.False(t, seen[tool.Name])
		seen[tool.Name] = true
		assert.Equal(t, mcp.AccessRead, tool.Access)
		assert.NotEmpty(t, tool.Description)
		assert.True(t, strings.HasPrefix(string(tool.Name), "openbao_"))
	}
	assert.Len(t, seen, 3)
	assert.Contains(t, New().Tools()[0].Description, "Start here")
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	for name := range dispatch {
		found := false
		for _, tool := range New().Tools() {
			found = found || tool.Name == name
		}
		assert.True(t, found, string(name))
	}
}

func TestReadsAndCompaction(t *testing.T) {
	cases := []struct {
		name                   mcp.ToolName
		args                   map[string]any
		method, route, fixture string
		keep                   []string
	}{
		{"openbao_list_secret_keys", map[string]any{"limit": 2, "after": "alpha"}, "LIST", "/v1/secret/metadata/approved/app/", `{"data":{"keys":["beta","folder/","zeta"],"key_info":{"secret":"SECRET_SENTINEL"}}}`, []string{`"keys":["beta","folder/"]`, `"next_after":"folder/"`, `"has_more":true`}},
		{"openbao_get_secret_metadata", map[string]any{"path": "service/key"}, "GET", "/v1/secret/metadata/approved/app/service/key", metadataFixture, []string{`"current_version":2`, `"version":1`, `"version":2`, `"destroyed":false`, `"created_time":"2026-01-01T00:00:00Z"`}},
		{"openbao_get_health", nil, "GET", "/v1/sys/health", `{"initialized":true,"sealed":false,"standby":false,"version":"2.7.2","cluster_name":"SECRET_SENTINEL"}`, []string{`"initialized":true`, `"sealed":false`, `"standby":false`, `"version":"2.7.2"`}},
	}
	for _, test := range cases {
		t.Run(string(test.name), func(t *testing.T) {
			adapter := testAdapter(t, func(writer http.ResponseWriter, request *http.Request) {
				assert.Equal(t, test.method, request.Method)
				assert.Equal(t, test.route, request.URL.Path)
				if test.name == "openbao_get_health" {
					assert.Empty(t, request.Header.Get("X-Vault-Token"))
				} else {
					assert.Equal(t, "test-token", request.Header.Get("X-Vault-Token"))
				}
				if test.name == "openbao_list_secret_keys" {
					assert.Equal(t, "3", request.URL.Query().Get("limit"))
					assert.Equal(t, "alpha", request.URL.Query().Get("after"))
				}
				_, _ = io.WriteString(writer, test.fixture)
			})
			result := execute(t, adapter, test.name, test.args)
			require.False(t, result.IsError, result.Data)
			fields, ok := adapter.CompactSpec(test.name)
			require.True(t, ok)
			compacted, err := mcp.CompactJSON([]byte(result.Data), fields)
			require.NoError(t, err)
			for _, output := range []string{result.Data, string(compacted)} {
				for _, expected := range test.keep {
					assert.Contains(t, output, expected)
				}
				assert.NotContains(t, output, "SECRET_SENTINEL")
				assert.NotContains(t, output, "test-token")
			}
		})
	}
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		_, ok := dispatch[name]
		assert.True(t, ok, string(name))
	}
	for _, tool := range New().Tools() {
		fields, ok := New().(mcp.FieldCompactionIntegration).CompactSpec(tool.Name)
		assert.True(t, ok)
		assert.NotEmpty(t, fields)
	}
}

func TestRejectUnsafeArguments(t *testing.T) {
	adapter := testAdapter(t, func(http.ResponseWriter, *http.Request) { t.Error("invalid arguments must not make requests") })
	cases := []struct {
		name mcp.ToolName
		args map[string]any
	}{
		{"openbao_get_secret_metadata", nil},
		{"openbao_get_secret_metadata", map[string]any{"path": "../outside"}},
		{"openbao_get_secret_metadata", map[string]any{"path": "/outside"}},
		{"openbao_get_secret_metadata", map[string]any{"path": "a%2f..%2foutside"}},
		{"openbao_get_secret_metadata", map[string]any{"path": "a?list=true"}},
		{"openbao_get_secret_metadata", map[string]any{"path": []any{"bad"}}},
		{"openbao_list_secret_keys", map[string]any{"path": "../outside"}},
		{"openbao_list_secret_keys", map[string]any{"limit": 0}},
		{"openbao_list_secret_keys", map[string]any{"limit": 101}},
		{"openbao_list_secret_keys", map[string]any{"limit": 1.5}},
		{"openbao_list_secret_keys", map[string]any{"after": "../bad"}},
		{"openbao_get_health", map[string]any{"base_url": "https://other.example.com"}},
	}
	for index, test := range cases {
		t.Run(fmt.Sprint(index), func(t *testing.T) { assert.True(t, execute(t, adapter, test.name, test.args).IsError) })
	}
}

func TestResponseFailures(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"unauthorized", 401, `{"errors":["test-token SECRET_SENTINEL"]}`},
		{"forbidden", 403, `{"errors":["SECRET_SENTINEL"]}`},
		{"missing", 404, `{"errors":[]}`},
		{"rate_limit", 429, `{"errors":["SECRET_SENTINEL"]}`},
		{"server_error", 500, `{"errors":["SECRET_SENTINEL"]}`},
		{"malformed", 200, `{"SECRET_SENTINEL":`},
		{"empty", 204, ""},
		{"missing_data", 200, `{}`},
		{"oversized", 200, strings.Repeat("x", 1024*1024+1)},
		{"null", 200, `{"data":null}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			adapter := testAdapter(t, func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(test.status)
				_, _ = io.WriteString(writer, test.body)
			})
			result, err := adapter.Execute(t.Context(), "openbao_get_secret_metadata", map[string]any{"path": "key"})
			if test.status == 429 || test.status == 500 {
				require.Error(t, err)
				assert.NotContains(t, err.Error(), "SECRET_SENTINEL")
				assert.NotContains(t, err.Error(), "test-token")
				return
			}
			require.NoError(t, err)
			assert.True(t, result.IsError, result.Data)
			assert.NotContains(t, result.Data, "SECRET_SENTINEL")
			assert.NotContains(t, result.Data, "test-token")
		})
	}
}

func TestHealthStates(t *testing.T) {
	for _, status := range []int{200, 429, 501, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			adapter := testAdapter(t, func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(status)
				_, _ = io.WriteString(writer, `{"initialized":true,"sealed":true,"standby":false,"version":"2.7.2"}`)
			})
			result := execute(t, adapter, "openbao_get_health", nil)
			assert.False(t, result.IsError, result.Data)
			assert.Contains(t, result.Data, `"sealed":true`)
		})
	}
}

func TestRedirectDenied(t *testing.T) {
	called := false
	target := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer target.Close()
	adapter := testAdapter(t, func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusTemporaryRedirect)
	})
	result := execute(t, adapter, "openbao_get_secret_metadata", map[string]any{"path": "key"})
	assert.True(t, result.IsError)
	assert.False(t, called)
}

func TestHealthyValidatesMountAndCredentials(t *testing.T) {
	cases := []struct {
		name, mount string
		status      int
		want        bool
	}{
		{"valid", `{"data":{"type":"kv","path":"secret/","options":{"version":"2"}}}`, 200, true},
		{"v1", `{"data":{"type":"kv","path":"secret/","options":{"version":"1"}}}`, 200, false},
		{"wrong_mount", `{"data":{"type":"kv","path":"other/","options":{"version":"2"}}}`, 200, false},
		{"wrong_engine", `{"data":{"type":"database","path":"secret/","options":{"version":"2"}}}`, 200, false},
		{"bad_token", `{"data":{"type":"kv","path":"secret/","options":{"version":"2"}}}`, 403, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			adapter := testAdapter(t, func(writer http.ResponseWriter, request *http.Request) {
				assert.Equal(t, "test-token", request.Header.Get("X-Vault-Token"))
				if request.URL.Path == "/v1/sys/internal/ui/mounts/secret/metadata/approved/app/" {
					_, _ = io.WriteString(writer, test.mount)
					return
				}
				assert.Equal(t, "/v1/secret/metadata/approved/app/", request.URL.Path)
				writer.WriteHeader(test.status)
				_, _ = io.WriteString(writer, `{"data":{"keys":["key"]}}`)
			})
			assert.Equal(t, test.want, adapter.Healthy(context.Background()))
		})
	}
}
