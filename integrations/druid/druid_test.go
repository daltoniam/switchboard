package druid

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

func configured(t *testing.T, handler http.HandlerFunc) *druid {
	t.Helper()
	provider := httptest.NewTLSServer(handler)
	t.Cleanup(provider.Close)
	adapter := New().(*druid)
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials{"base_url": provider.URL + "/proxy", "username": "diagnostic", "password": "secret-sentinel", "ca_cert": strings.ReplaceAll(string(ca), "\n", `\n`)}))
	return adapter
}

func TestConfigure(t *testing.T) {
	for _, test := range []struct{ name, key, value string }{
		{"missing_url", "base_url", ""}, {"http", "base_url", "http://127.0.0.1:8080"}, {"userinfo", "base_url", "https://user:password@example.test"}, {"query", "base_url", "https://example.test?token=x"}, {"fragment", "base_url", "https://example.test#x"}, {"traversal", "base_url", "https://example.test/a/../b"}, {"username", "username", ""}, {"password", "password", ""}, {"admin", "username", "admin"}, {"system", "username", "druid_system"}, {"colon", "username", "user:other"}, {"bad_ca", "ca_cert", "invalid"},
	} {
		t.Run(test.name, func(t *testing.T) {
			credentials := mcp.Credentials{"base_url": "https://example.test/proxy", "username": "diagnostic", "password": "secret"}
			credentials[test.key] = test.value
			require.Error(t, New().Configure(t.Context(), credentials))
		})
	}
	require.NoError(t, New().Configure(t.Context(), mcp.Credentials{"base_url": "https://example.test/proxy", "username": "diagnostic", "password": "secret"}))
	require.False(t, New().Healthy(t.Context()))
	result, err := New().Execute(t.Context(), "druid_list_datasources", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	require.Equal(t, "druid", New().Name())
	seen := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		require.Contains(t, dispatch, tool.Name)
		require.False(t, seen[tool.Name])
		seen[tool.Name] = true
		require.Equal(t, mcp.AccessRead, tool.Access)
		require.True(t, strings.HasPrefix(string(tool.Name), "druid_"))
		require.NotEmpty(t, tool.Description)
	}
	require.Len(t, seen, 5)
	require.Contains(t, New().Tools()[0].Description, "Start here")
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	names := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		names[tool.Name] = true
	}
	for name := range dispatch {
		require.True(t, names[name])
	}
	result, err := New().Execute(t.Context(), "druid_unknown", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Data, "unknown tool")
}

func TestToolsRequests(t *testing.T) {
	for _, test := range []struct {
		name                     mcp.ToolName
		args                     map[string]any
		route, payload, expected string
	}{
		{"druid_list_datasources", nil, "/druid/coordinator/v1/datasources", `["events","other"]`, "events"},
		{"druid_get_datasource", map[string]any{"datasource": "events"}, "/druid/coordinator/v1/datasources/events", `{"segments":{"count":2,"size":123,"replicatedSize":246,"minTime":"2026-01-01","maxTime":"2026-01-02","secret":"secret-sentinel"},"tiers":{"private-host":{}}}`, `"count":2`},
		{"druid_get_load_status", map[string]any{"datasource": "events"}, "/druid/coordinator/v1/loadstatus", `{"events":50.0,"other":100}`, `"percent_loaded":50`},
		{"druid_list_tasks", map[string]any{"datasource": "events", "type": "index_parallel", "interval": "2026-01-01T00:00:00Z_2026-01-02T00:00:00Z", "limit": 2}, "/druid/indexer/v1/tasks", `[{"id":"task-1","type":"index_parallel","dataSource":"events","statusCode":"FAILED","runnerStatusCode":"NONE","createdTime":"2026-01-01","duration":12,"errorMsg":"secret-sentinel /private/path https://private.test/token","location":{"host":"private-host"},"payload":"secret-sentinel"}]`, `"statusCode":"FAILED"`},
		{"druid_get_task_status", map[string]any{"task_id": "task-1"}, "/druid/indexer/v1/task/task-1/status", `{"task":"task-1","status":{"id":"task-1","status":"FAILED","duration":12,"errorMsg":"secret-sentinel /private/path","location":{"host":"private-host"}},"payload":"secret-sentinel"}`, `"status":"FAILED"`},
	} {
		t.Run(string(test.name), func(t *testing.T) {
			adapter := configured(t, func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "/proxy"+test.route, r.URL.Path)
				user, password, ok := r.BasicAuth()
				require.True(t, ok)
				require.Equal(t, "diagnostic", user)
				require.Equal(t, "secret-sentinel", password)
				if test.name == "druid_list_tasks" {
					require.Equal(t, "complete", r.URL.Query().Get("state"))
					require.Equal(t, "2", r.URL.Query().Get("max"))
					require.Equal(t, test.args["interval"], r.URL.Query().Get("createdTimeInterval"))
				}
				_, _ = fmt.Fprint(w, test.payload)
			})
			result, err := adapter.Execute(t.Context(), test.name, test.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			require.Contains(t, result.Data, test.expected)
			for _, secret := range []string{"secret-sentinel", "private-host", "/private/path", "private.test", "payload", "loadSpec"} {
				require.NotContains(t, result.Data, secret)
			}
			require.True(t, json.Valid([]byte(result.Data)))
			spec, ok := adapter.CompactSpec(test.name)
			require.True(t, ok)
			require.NotEmpty(t, spec)
			projected, err := mcp.CompactJSON([]byte(result.Data), spec)
			require.NoError(t, err)
			require.Contains(t, string(projected), test.expected)
			for _, secret := range []string{"secret-sentinel", "private-host", "/private/path", "private.test", "payload", "loadSpec"} {
				require.NotContains(t, string(projected), secret)
			}
		})
	}
}

func TestInvalidArguments(t *testing.T) {
	adapter := configured(t, func(http.ResponseWriter, *http.Request) { t.Error("invalid arguments reached provider") })
	for _, test := range []struct {
		name mcp.ToolName
		args map[string]any
	}{
		{"druid_get_datasource", nil}, {"druid_get_datasource", map[string]any{"datasource": "../secrets"}}, {"druid_get_task_status", map[string]any{"task_id": "%2Fsecrets"}},
		{"druid_list_tasks", map[string]any{"state": "failed"}}, {"druid_list_tasks", map[string]any{"limit": 101}}, {"druid_list_tasks", map[string]any{"interval": "2026-01-01/2026-01-02"}}, {"druid_list_tasks", map[string]any{"interval": "2026-01-02T00:00:00Z_2026-01-01T00:00:00Z"}},
		{"druid_list_datasources", map[string]any{"limit": 0}}, {"druid_list_datasources", map[string]any{"limit": "invalid"}}, {"druid_get_load_status", map[string]any{"datasource": map[string]any{"invalid": true}}},
	} {
		t.Run(fmt.Sprint(test.name, test.args), func(t *testing.T) {
			result, err := adapter.Execute(t.Context(), test.name, test.args)
			require.NoError(t, err)
			require.True(t, result.IsError)
		})
	}
}

func TestHTTPFailures(t *testing.T) {
	for _, status := range []int{204, 301, 400, 401, 403, 404, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			attempts := 0
			adapter := configured(t, func(w http.ResponseWriter, r *http.Request) {
				attempts++
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(status)
				_, _ = fmt.Fprint(w, "secret-sentinel /private/path")
			})
			result, err := adapter.Execute(t.Context(), "druid_list_datasources", nil)
			if status == 429 {
				require.Error(t, err)
				require.True(t, mcp.IsRetryable(err))
				require.Equal(t, 3, attempts)
			} else {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.NotContains(t, result.Data, "secret-sentinel")
				require.Equal(t, 1, attempts)
			}
		})
	}
	for _, body := range []string{`null`, `{}`, `[] trailing`, strings.Repeat("x", maxResponseSize+1)} {
		t.Run(fmt.Sprint(len(body)), func(t *testing.T) {
			adapter := configured(t, func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, body) })
			result, err := adapter.Execute(t.Context(), "druid_list_datasources", nil)
			require.NoError(t, err)
			require.True(t, result.IsError)
		})
	}
}

func TestBoundsAndDefaults(t *testing.T) {
	adapter := configured(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "tasks") {
			query := r.URL.Query()
			require.Equal(t, "complete", query.Get("state"))
			require.Equal(t, "50", query.Get("max"))
			bounds := strings.Split(query.Get("createdTimeInterval"), "_")
			require.Len(t, bounds, 2)
			start, err := time.Parse(time.RFC3339, bounds[0])
			require.NoError(t, err)
			end, err := time.Parse(time.RFC3339, bounds[1])
			require.NoError(t, err)
			require.Equal(t, 24*time.Hour, end.Sub(start))
			_, _ = fmt.Fprint(w, `[]`)
			return
		}
		_, _ = fmt.Fprint(w, `["one","two","three"]`)
	})
	result, err := adapter.Execute(t.Context(), "druid_list_datasources", map[string]any{"limit": 2})
	require.NoError(t, err)
	require.Contains(t, result.Data, `"truncated":true`)
	require.NotContains(t, result.Data, "two")
	result, err = adapter.Execute(t.Context(), "druid_list_tasks", nil)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result, err = adapter.Execute(ctx, "druid_list_datasources", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestRedirectTLSAndRetry(t *testing.T) {
	t.Run("redirect", func(t *testing.T) {
		target := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("redirect target was contacted") }))
		defer target.Close()
		adapter := configured(t, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
		})
		result, err := adapter.Execute(t.Context(), "druid_list_datasources", nil)
		require.NoError(t, err)
		require.True(t, result.IsError)
	})
	t.Run("untrusted_tls", func(t *testing.T) {
		provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, `[]`) }))
		defer provider.Close()
		adapter := New()
		require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials{"base_url": provider.URL, "username": "diagnostic", "password": "secret"}))
		require.False(t, adapter.Healthy(t.Context()))
	})
	t.Run("retry_success", func(t *testing.T) {
		attempts := 0
		adapter := configured(t, func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts == 1 {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(429)
				return
			}
			_, _ = fmt.Fprint(w, `[]`)
		})
		require.True(t, adapter.Healthy(t.Context()))
		require.Equal(t, 2, attempts)
	})
	t.Run("long_retry_after", func(t *testing.T) {
		adapter := configured(t, func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Retry-After", "120"); w.WriteHeader(429) })
		before := time.Now()
		_, err := adapter.Execute(t.Context(), "druid_list_datasources", nil)
		require.Error(t, err)
		require.True(t, mcp.IsRetryable(err))
		require.Less(t, time.Since(before), time.Second)
	})
}

func TestOutputBoundsAndNulls(t *testing.T) {
	for _, test := range []struct {
		name     string
		tool     mcp.ToolName
		args     map[string]any
		payload  string
		expected string
	}{
		{"null_fields", "druid_get_task_status", map[string]any{"task_id": "task-1"}, `{"task":"task-1","status":{"id":"task-1","status":null,"duration":null,"errorMsg":null}}`, `"id":"task-1"`},
		{"unknown_status", "druid_get_task_status", map[string]any{"task_id": "task-1"}, `{"task":"task-1","status":{"id":"task-1","status":"FUTURE_STATE"}}`, `"status":"FUTURE_STATE"`},
		{"active_cap", "druid_list_tasks", map[string]any{"state": "running", "limit": 1}, `[{"id":"task-1"},{"id":"task-2"}]`, `"truncated":true`},
		{"load_cap", "druid_get_load_status", map[string]any{"limit": 1}, `{"one":10,"two":20}`, `"truncated":true`},
	} {
		t.Run(test.name, func(t *testing.T) {
			adapter := configured(t, func(w http.ResponseWriter, r *http.Request) {
				if test.name == "active_cap" {
					require.Empty(t, r.URL.Query().Get("max"))
				}
				_, _ = fmt.Fprint(w, test.payload)
			})
			result, err := adapter.Execute(t.Context(), test.tool, test.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			require.Contains(t, result.Data, test.expected)
			require.NotContains(t, result.Data, "SUCCESS")
		})
	}
	adapter := configured(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"task":"task-1","status":{"id":"task-1","errorMsg":"`+strings.Repeat("あ", 400)+`"}}`)
	})
	result, err := adapter.Execute(t.Context(), "druid_get_task_status", map[string]any{"task_id": "task-1"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	var status taskStatus
	require.NoError(t, json.Unmarshal([]byte(result.Data), &status))
	require.LessOrEqual(t, len(*status.Status.ErrorMsg), 512)
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		require.Contains(t, dispatch, name)
	}
	require.Len(t, fieldCompactionSpecs, 5)
}
