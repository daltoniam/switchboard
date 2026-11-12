package semaphoreui

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixture(t *testing.T, handle http.HandlerFunc) (*semaphoreui, mcp.Credentials) {
	t.Helper()
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "Bearer fixture-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/prefix/api/user":
			_, _ = io.WriteString(w, `{"id":2,"admin":false}`)
		case "/prefix/api/info":
			_, _ = io.WriteString(w, `{"version":"v2.19.12-012ed06-1788086369"}`)
		case "/prefix/api/project/7/role":
			_, _ = io.WriteString(w, `{"role":"guest","permissions":0}`)
		default:
			handle(w, r)
		}
	}))
	t.Cleanup(provider.Close)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})
	credentials := mcp.Credentials{"base_url": provider.URL + "/prefix", "api_token": "fixture-token", "project_id": "7", "ca_cert": string(cert)}
	adapter := New().(*semaphoreui)
	require.NoError(t, adapter.Configure(t.Context(), credentials))
	t.Cleanup(func() { require.NoError(t, adapter.Close()) })
	return adapter, credentials
}

func execute(t *testing.T, adapter mcp.Integration, name mcp.ToolName, args map[string]any) *mcp.ToolResult {
	t.Helper()
	result, err := adapter.Execute(t.Context(), name, args)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func TestConfigure(t *testing.T) {
	tests := []struct{ name, key, value string }{
		{"missing URL", "base_url", ""},
		{"HTTP", "base_url", "http://localhost"},
		{"userinfo", "base_url", "https://user:secret@example.com"},
		{"query", "base_url", "https://example.com?token=secret"},
		{"fragment", "base_url", "https://example.com#secret"},
		{"encoded path", "base_url", "https://example.com/%2e%2e"},
		{"traversal", "base_url", "https://example.com/a/../b"},
		{"bad port", "base_url", "https://example.com:99999"},
		{"missing token", "api_token", ""},
		{"token newline", "api_token", "secret\n"},
		{"token prefix", "api_token", "Bearer secret"},
		{"missing project", "project_id", ""},
		{"negative project", "project_id", "-1"},
		{"path project", "project_id", "7/role"},
		{"large project", "project_id", "9223372036854775808"},
		{"invalid CA", "ca_cert", "not PEM"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			credentials := mcp.Credentials{"base_url": "https://example.com", "api_token": "secret", "project_id": "7"}
			credentials[test.key] = test.value
			err := New().Configure(t.Context(), credentials)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret")
		})
	}
}

func TestAuthorizationBoundary(t *testing.T) {
	tests := []struct{ name, user, role, version string }{
		{"guest", `{"admin":false}`, `{"role":"guest","permissions":0}`, "v2.19.12-abc"},
		{"admin guest", `{"admin":true}`, `{"role":"guest","permissions":0}`, "v2.19.12"},
		{"missing admin", `{}`, `{"role":"guest","permissions":0}`, "v2.19.12"},
		{"runner", `{"admin":false}`, `{"role":"task_runner","permissions":1}`, "v2.19.12"},
		{"custom guest", `{"admin":false}`, `{"role":"guest","permissions":1}`, "v2.19.12"},
		{"missing permission", `{"admin":false}`, `{"role":"guest"}`, "v2.19.12"},
		{"unknown version", `{"admin":false}`, `{"role":"guest","permissions":0}`, "v2.19.13"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var drift atomic.Bool
			var reads atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/user":
					_, _ = io.WriteString(w, test.user)
				case "/api/info":
					_ = json.NewEncoder(w).Encode(map[string]string{"version": test.version})
				case "/api/project/7/role":
					if drift.Load() {
						_, _ = io.WriteString(w, `{"role":"manager","permissions":3}`)
					} else {
						_, _ = io.WriteString(w, test.role)
					}
				default:
					reads.Add(1)
					_, _ = io.WriteString(w, `{"id":7,"name":"approved"}`)
				}
			}))
			defer server.Close()
			cert := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))
			adapter := New().(*semaphoreui)
			defer func() { require.NoError(t, adapter.Close()) }()
			err := adapter.Configure(t.Context(), mcp.Credentials{"base_url": server.URL, "api_token": "fixture-token", "project_id": "7", "ca_cert": cert})
			if test.name != "guest" {
				require.Error(t, err)
				require.False(t, adapter.Healthy(t.Context()))
				require.Zero(t, reads.Load())
				return
			}
			require.NoError(t, err)
			require.False(t, execute(t, adapter, "semaphoreui_get_project", nil).IsError)
			require.Equal(t, int32(1), reads.Load())
			drift.Store(true)
			require.True(t, execute(t, adapter, "semaphoreui_get_project", nil).IsError)
			require.Equal(t, int32(1), reads.Load())
		})
	}
}

func TestToolsAndLifecycle(t *testing.T) {
	adapter := New().(*semaphoreui)
	require.Equal(t, "semaphoreui", adapter.Name())
	require.False(t, adapter.Healthy(t.Context()))
	require.True(t, execute(t, adapter, "semaphoreui_get_project", nil).IsError)
	require.Contains(t, execute(t, adapter, "unknown", nil).Data, "unknown tool")
	require.Len(t, adapter.Tools(), 5)
	require.Contains(t, adapter.Tools()[0].Description, "Start here")
	names := map[mcp.ToolName]bool{}
	for _, tool := range adapter.Tools() {
		require.False(t, names[tool.Name])
		names[tool.Name] = true
		require.True(t, strings.HasPrefix(string(tool.Name), "semaphoreui_"))
		require.NotEmpty(t, tool.Description)
		require.Equal(t, mcp.AccessRead, tool.Access)
	}
	require.NoError(t, adapter.Close())
	require.NoError(t, adapter.Close())
	configured, credentials := fixture(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `{"id":7,"name":"approved"}`) })
	require.True(t, configured.Healthy(t.Context()))
	require.NoError(t, configured.Close())
	require.False(t, configured.Healthy(t.Context()))
	require.NoError(t, configured.Configure(t.Context(), credentials))
	require.True(t, configured.Healthy(t.Context()))
	credentials["api_token"] = ""
	require.Error(t, configured.Configure(t.Context(), credentials))
	require.False(t, configured.Healthy(t.Context()))
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range New().Tools() {
		require.Contains(t, dispatch, tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	for name := range dispatch {
		found := false
		for _, tool := range New().Tools() {
			found = found || tool.Name == name
		}
		require.True(t, found, name)
	}
}

func TestReadProjection(t *testing.T) {
	tests := []struct {
		tool                            mcp.ToolName
		path, query, response, expected string
		args                            map[string]any
	}{
		{"semaphoreui_get_project", "/prefix/api/project/7", "", `{"id":7,"name":"approved","alert":"SECRET_SENTINEL","unexpected":"SECRET_SENTINEL"}`, `{"id":7,"name":"approved"}`, nil},
		{"semaphoreui_list_templates", "/prefix/api/project/7/templates", "order=asc&sort=name", `[{"id":3,"project_id":7,"name":"safe","app":"ansible","type":"build","playbook":"SECRET_SENTINEL","last_task":{"id":9,"status":"error","secret":"SECRET_SENTINEL"}}]`, `{"templates":[{"id":3,"project_id":7,"name":"safe","app":"ansible","type":"build","last_task":{"id":9,"status":"error"}}],"truncated":false}`, nil},
		{"semaphoreui_get_template", "/prefix/api/project/7/templates/3", "", `{"id":3,"project_id":7,"name":"safe","app":"bash","arguments":"SECRET_SENTINEL"}`, `{"id":3,"project_id":7,"name":"safe","app":"bash"}`, map[string]any{"template_id": 3}},
		{"semaphoreui_list_tasks", "/prefix/api/project/7/tasks/last", "count=50", `[{"id":9,"project_id":7,"template_id":3,"status":"error","created":"2026-11-01T10:00:00Z","start":null,"end":null,"secret":"SECRET_SENTINEL"}]`, `{"tasks":[{"id":9,"project_id":7,"template_id":3,"status":"error","created":"2026-11-01T10:00:00Z"}],"has_next":false,"pagination_known":true}`, nil},
		{"semaphoreui_get_task", "/prefix/api/project/7/tasks/9", "", `{"id":9,"project_id":7,"template_id":3,"status":"success","created":"2026-11-01T10:00:00Z","start":"2026-11-01T10:00:01Z","end":"2026-11-01T10:00:02Z","message":"SECRET_SENTINEL","environment":"SECRET_SENTINEL"}`, `{"id":9,"project_id":7,"template_id":3,"status":"success","created":"2026-11-01T10:00:00Z","start":"2026-11-01T10:00:01Z","end":"2026-11-01T10:00:02Z"}`, map[string]any{"task_id": 9}},
	}
	for _, test := range tests {
		t.Run(string(test.tool), func(t *testing.T) {
			adapter, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, test.path, r.URL.Path)
				require.Equal(t, test.query, r.URL.RawQuery)
				w.Header().Set("X-Has-Next", "false")
				_, _ = io.WriteString(w, test.response)
			})
			result := execute(t, adapter, test.tool, test.args)
			require.False(t, result.IsError, result.Data)
			require.JSONEq(t, test.expected, result.Data)
			require.NotContains(t, result.Data, "SECRET_SENTINEL")
			fields, ok := adapter.CompactSpec(test.tool)
			require.True(t, ok)
			compacted, err := mcp.CompactJSON([]byte(result.Data), fields)
			require.NoError(t, err)
			require.NotContains(t, string(compacted), "SECRET_SENTINEL")
			require.Contains(t, string(compacted), "id")
		})
	}
}

func TestTaskPagination(t *testing.T) {
	for _, header := range []string{"true", "false", "", "unsupported"} {
		t.Run("header="+header, func(t *testing.T) {
			adapter, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "before=20&count=2", r.URL.RawQuery)
				w.Header().Set("X-Has-Next", header)
				_, _ = io.WriteString(w, `[{"id":19,"project_id":7,"template_id":3,"status":"success"},{"id":12,"project_id":7,"template_id":4,"status":"error"}]`)
			})
			result := execute(t, adapter, "semaphoreui_list_tasks", map[string]any{"before": 20, "count": 2, "status": "waiting", "template_id": 3})
			require.False(t, result.IsError, result.Data)
			var page map[string]any
			require.NoError(t, json.Unmarshal([]byte(result.Data), &page))
			require.Empty(t, page["tasks"])
			require.Equal(t, header == "true" || header == "false", page["pagination_known"])
			if header == "true" {
				require.Equal(t, float64(12), page["next_before"])
				require.Equal(t, true, page["has_next"])
			} else {
				require.NotContains(t, page, "next_before")
			}
		})
	}
}

func TestInvalidArguments(t *testing.T) {
	adapter, _ := fixture(t, func(http.ResponseWriter, *http.Request) { t.Error("invalid arguments reached provider") })
	for _, test := range []struct {
		tool mcp.ToolName
		args map[string]any
	}{
		{"semaphoreui_get_project", map[string]any{"project_id": 8}},
		{"semaphoreui_get_template", nil},
		{"semaphoreui_get_template", map[string]any{"template_id": "../3"}},
		{"semaphoreui_get_task", map[string]any{"task_id": -1}},
		{"semaphoreui_get_task", map[string]any{"task_id": 1.5}},
		{"semaphoreui_list_tasks", map[string]any{"count": 101}},
		{"semaphoreui_list_tasks", map[string]any{"count": 0}},
		{"semaphoreui_list_tasks", map[string]any{"count": 1.5}},
		{"semaphoreui_list_tasks", map[string]any{"count": "bad"}},
		{"semaphoreui_list_tasks", map[string]any{"before": -1}},
		{"semaphoreui_list_tasks", map[string]any{"template_id": "bad"}},
		{"semaphoreui_list_tasks", map[string]any{"status": "not a status"}},
	} {
		t.Run(fmt.Sprint(test.tool, test.args), func(t *testing.T) {
			require.True(t, execute(t, adapter, test.tool, test.args).IsError)
		})
	}
}

func TestProviderFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{"unauthorized", 401, "SECRET_SENTINEL"}, {"forbidden", 403, "SECRET_SENTINEL"}, {"missing", 404, "SECRET_SENTINEL"},
		{"redirect", 302, "SECRET_SENTINEL"}, {"server", 500, "SECRET_SENTINEL"}, {"empty", 204, ""},
		{"malformed", 200, "{"}, {"trailing JSON", 200, `{"id":7} {}`}, {"null", 200, "null"},
		{"wrong project", 200, `{"id":8,"name":"other"}`}, {"oversized", 200, strings.Repeat(" ", maxResponseBytes+1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapter, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", "https://example.com/credential-leak")
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			})
			result := execute(t, adapter, "semaphoreui_get_project", nil)
			require.True(t, result.IsError, result.Data)
			require.NotContains(t, result.Data, "SECRET_SENTINEL")
		})
	}
}

func TestTruncationAndIdentity(t *testing.T) {
	adapter, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		items := make([]map[string]any, 101)
		for index := range items {
			items[index] = map[string]any{"id": index + 1, "project_id": 7, "name": "safe"}
		}
		_ = json.NewEncoder(w).Encode(items)
	})
	result := execute(t, adapter, "semaphoreui_list_templates", nil)
	require.False(t, result.IsError, result.Data)
	var page struct {
		Templates []any `json:"templates"`
		Truncated bool  `json:"truncated"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Data), &page))
	require.Len(t, page.Templates, 100)
	require.True(t, page.Truncated)
	for _, body := range []string{`{"id":3,"project_id":8}`, `{"id":4,"project_id":7}`, `null`} {
		t.Run(body, func(t *testing.T) {
			other, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, body) })
			require.True(t, execute(t, other, "semaphoreui_get_template", map[string]any{"template_id": 3}).IsError)
		})
	}
}

func TestRetryAndCancellation(t *testing.T) {
	var calls atomic.Int32
	adapter, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(429)
			return
		}
		_, _ = io.WriteString(w, `{"id":7,"name":"approved"}`)
	})
	start := time.Now()
	require.False(t, execute(t, adapter, "semaphoreui_get_project", nil).IsError)
	require.GreaterOrEqual(t, time.Since(start), time.Second)
	require.Equal(t, int32(2), calls.Load())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result, err := adapter.Execute(ctx, "semaphoreui_get_project", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		require.Contains(t, dispatch, name)
	}
	for _, tool := range New().Tools() {
		fields, ok := New().(mcp.FieldCompactionIntegration).CompactSpec(tool.Name)
		assert.True(t, ok, tool.Name)
		assert.NotEmpty(t, fields)
	}
}
