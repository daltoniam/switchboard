package wazuh

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

func testToken(expiry time.Time) string {
	return "eyJhbGciOiJIUzI1NiJ9." + base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, expiry.Unix()))) + ".signature"
}

func testAdapter(t *testing.T, handler http.HandlerFunc) (*wazuh, *httptest.Server) {
	t.Helper()
	provider := httptest.NewTLSServer(handler)
	t.Cleanup(provider.Close)
	adapter := New().(*wazuh)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials{"base_url": provider.URL + "/manager", "username": "fleet-reader", "password": "secret-password", "ca_cert": string(cert)}))
	t.Cleanup(func() { require.NoError(t, adapter.Close()) })
	return adapter, provider
}

func agentPageFixture() string {
	return `{"error":0,"data":{"affected_items":[{"id":"001","name":"endpoint-one","status":"disconnected","version":"Wazuh v4.14.8","lastKeepAlive":"2026-11-01T00:00:00Z","group_config_status":"not synced","ip":"SECRET_SENTINEL","configSum":"SECRET_SENTINEL","os":{"name":"Ubuntu","version":"24.04","uname":"SECRET_SENTINEL"}}],"total_affected_items":2,"total_failed_items":0,"failed_items":[]},"message":"SECRET_SENTINEL"}`
}

func authOr(t *testing.T, handler http.HandlerFunc) http.HandlerFunc {
	t.Helper()
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/manager/security/user/authenticate" {
			require.Equal(t, http.MethodPost, request.Method)
			user, password, ok := request.BasicAuth()
			require.True(t, ok)
			require.Equal(t, "fleet-reader", user)
			require.Equal(t, "secret-password", password)
			_, _ = fmt.Fprintf(writer, `{"error":0,"data":{"token":%q}}`, testToken(time.Now().Add(time.Hour)))
			return
		}
		require.Equal(t, http.MethodGet, request.Method)
		require.True(t, strings.HasPrefix(request.Header.Get("Authorization"), "Bearer "))
		handler(writer, request)
	}
}

func TestConstructorAndCatalog(t *testing.T) {
	adapter := New()
	require.Equal(t, "wazuh", adapter.Name())
	require.False(t, adapter.Healthy(t.Context()))
	require.Len(t, adapter.Tools(), 3)
	seen := map[mcp.ToolName]bool{}
	for _, tool := range adapter.Tools() {
		require.True(t, strings.HasPrefix(string(tool.Name), "wazuh_"))
		require.NotEmpty(t, tool.Description)
		require.Equal(t, mcp.AccessRead, tool.Access)
		require.False(t, seen[tool.Name])
		seen[tool.Name] = true
	}
	require.Contains(t, adapter.Tools()[0].Description, "Start here")
	for _, name := range []mcp.ToolName{"unknown", "wazuh_list_agents"} {
		result, err := adapter.Execute(t.Context(), name, nil)
		require.NoError(t, err)
		require.True(t, result.IsError)
	}
	require.NoError(t, adapter.(io.Closer).Close())
	require.NoError(t, adapter.(io.Closer).Close())
}

func TestConfigure(t *testing.T) {
	valid := mcp.Credentials{"base_url": "https://manager.example:55000/prefix", "username": "fleet-reader", "password": "secret-password"}
	for _, test := range []struct{ key, value string }{
		{"base_url", ""}, {"base_url", "http://manager.example"}, {"base_url", "https://user:pass@manager.example"},
		{"base_url", "https://manager.example?x=1"}, {"base_url", "https://manager.example/#fragment"},
		{"base_url", "https://manager.example/%2e%2e"}, {"base_url", "https://manager.example/../admin"},
		{"base_url", "https://manager.example:0"}, {"base_url", "https://manager.example:65536"}, {"base_url", "https://manager.example:"},
		{"username", ""}, {"username", "wazuh"}, {"username", "wazuh-wui"}, {"username", "admin"}, {"username", "bad:user"},
		{"username", "bad\nuser"}, {"password", ""}, {"password", "bad\npassword"}, {"ca_cert", "invalid"},
	} {
		t.Run(test.key+"/"+test.value, func(t *testing.T) {
			adapter := New()
			require.NoError(t, adapter.Configure(t.Context(), valid))
			creds := mcp.Credentials{}
			for key, value := range valid {
				creds[key] = value
			}
			creds[test.key] = test.value
			require.Error(t, adapter.Configure(t.Context(), creds))
			require.False(t, adapter.Healthy(t.Context()))
		})
	}
}

func TestListAndGetSafeProjection(t *testing.T) {
	adapter, _ := testAdapter(t, authOr(t, func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/manager/agents", request.URL.Path)
		require.Equal(t, "id,name,status,version,os.name,os.version,lastKeepAlive,group_config_status", request.URL.Query().Get("select"))
		require.Equal(t, "id", request.URL.Query().Get("sort"))
		_, _ = io.WriteString(writer, agentPageFixture())
	}))
	for _, test := range []struct {
		name mcp.ToolName
		args map[string]any
	}{
		{"wazuh_list_agents", map[string]any{"search": "endpoint", "status": "disconnected", "limit": 1}},
		{"wazuh_get_agent", map[string]any{"agent_id": "001"}},
	} {
		t.Run(string(test.name), func(t *testing.T) {
			result, err := adapter.Execute(t.Context(), test.name, test.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			require.Contains(t, result.Data, "endpoint-one")
			require.NotContains(t, result.Data, "SECRET_SENTINEL")
			require.NotContains(t, result.Data, "secret-password")
			var value map[string]any
			require.NoError(t, json.Unmarshal([]byte(result.Data), &value))
			if test.name == "wazuh_list_agents" {
				require.Equal(t, float64(1), value["next_offset"])
			}
		})
	}
	require.True(t, adapter.Healthy(t.Context()))
}

func TestListQueryAndPagination(t *testing.T) {
	adapter, _ := testAdapter(t, authOr(t, func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		require.Equal(t, "25", query.Get("limit"))
		require.Equal(t, "25", query.Get("offset"))
		require.Equal(t, "a&status=active", query.Get("search"))
		require.Equal(t, "pending", query.Get("status"))
		_, _ = io.WriteString(writer, `{"error":0,"data":{"affected_items":[],"total_affected_items":25,"total_failed_items":0,"failed_items":[]}}`)
	}))
	result, err := adapter.Execute(t.Context(), "wazuh_list_agents", map[string]any{"search": "a&status=active", "status": "pending", "offset": 25})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	require.Contains(t, result.Data, `"next_offset":null`)
}

func TestArgumentRejections(t *testing.T) {
	var calls atomic.Int32
	adapter, _ := testAdapter(t, func(http.ResponseWriter, *http.Request) { calls.Add(1) })
	for _, test := range []struct {
		name mcp.ToolName
		args map[string]any
	}{
		{"wazuh_list_agents", map[string]any{"base_url": "https://evil.example"}},
		{"wazuh_list_agents", map[string]any{"status": "all"}},
		{"wazuh_list_agents", map[string]any{"search": strings.Repeat("x", 257)}},
		{"wazuh_list_agents", map[string]any{"search": "bad\nfilter"}},
		{"wazuh_list_agents", map[string]any{"limit": 101}},
		{"wazuh_list_agents", map[string]any{"limit": 0}},
		{"wazuh_list_agents", map[string]any{"limit": 1.5}},
		{"wazuh_list_agents", map[string]any{"offset": -1}},
		{"wazuh_list_agents", map[string]any{"offset": 2147483648.0}},
		{"wazuh_list_agents", map[string]any{"limit": []any{1}}},
		{"wazuh_get_agent", nil},
		{"wazuh_get_agent", map[string]any{"agent_id": "001,002"}},
		{"wazuh_get_agent", map[string]any{"agent_id": "../security"}},
		{"wazuh_get_agent", map[string]any{"agent_id": "https://evil.example"}},
		{"wazuh_get_agent", map[string]any{"agent_id": "all"}},
		{"wazuh_get_agent_status_summary", map[string]any{"path": "/security"}},
	} {
		t.Run(fmt.Sprint(test.name, test.args), func(t *testing.T) {
			result, err := adapter.Execute(t.Context(), test.name, test.args)
			require.NoError(t, err)
			require.True(t, result.IsError, result.Data)
		})
	}
	require.Zero(t, calls.Load())
}

func TestSummary(t *testing.T) {
	adapter, _ := testAdapter(t, authOr(t, func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/manager/agents/summary/status", request.URL.Path)
		_, _ = io.WriteString(writer, `{"error":0,"data":{"connection":{"active":1,"disconnected":2,"never_connected":3,"pending":4,"total":10,"token":"SECRET_SENTINEL"},"configuration":{"synced":6,"not_synced":4,"total":10},"secret":"SECRET_SENTINEL"}}`)
	}))
	result, err := adapter.Execute(t.Context(), "wazuh_get_agent_status_summary", nil)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	require.Contains(t, result.Data, `"not_synced":4`)
	require.NotContains(t, result.Data, "SECRET_SENTINEL")
}

func TestPartialFailuresAndMissingAgent(t *testing.T) {
	for _, test := range []struct {
		name, payload string
		failed        bool
	}{
		{"partial", `{"error":2,"data":{"affected_items":[{"id":"001","name":"endpoint-one"}],"total_affected_items":1,"total_failed_items":1,"failed_items":[{"id":["002"],"error":{"code":4000,"message":"SECRET_SENTINEL","remediation":"SECRET_SENTINEL"}}]}}`, false},
		{"forbidden", `{"error":1,"data":{"affected_items":[],"total_affected_items":0,"total_failed_items":1,"failed_items":[{"id":["001"],"error":{"code":4000,"message":"SECRET_SENTINEL"}}]}}`, true},
		{"missing", `{"error":0,"data":{"affected_items":[],"total_affected_items":0,"total_failed_items":0,"failed_items":[]}}`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			adapter, _ := testAdapter(t, authOr(t, func(writer http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(writer, test.payload) }))
			name, args := mcp.ToolName("wazuh_list_agents"), map[string]any{}
			if test.failed {
				name, args = "wazuh_get_agent", map[string]any{"agent_id": "001"}
			}
			result, err := adapter.Execute(t.Context(), name, args)
			require.NoError(t, err)
			require.Equal(t, test.failed, result.IsError, result.Data)
			require.NotContains(t, result.Data, "SECRET_SENTINEL")
			if !test.failed {
				require.Contains(t, result.Data, `"partial":true`)
				require.Contains(t, result.Data, "4000")
			}
		})
	}
}

func TestResponseFailures(t *testing.T) {
	for _, test := range []struct {
		name    string
		status  int
		payload string
	}{
		{"unauthorized", 401, "SECRET_SENTINEL"}, {"forbidden", 403, "SECRET_SENTINEL"}, {"missing", 404, "SECRET_SENTINEL"},
		{"empty", 204, ""}, {"malformed", 200, "{"}, {"missing data", 200, `{"error":0}`},
		{"missing counts", 200, `{"error":0,"data":{"affected_items":[]}}`},
		{"negative count", 200, `{"error":0,"data":{"affected_items":[],"total_affected_items":-1,"total_failed_items":0,"failed_items":[]}}`},
		{"api error", 200, `{"error":1000,"message":"SECRET_SENTINEL"}`},
		{"oversized", 200, strings.Repeat("x", 2*1024*1024+1)},
		{"long string", 200, strings.Replace(agentPageFixture(), "endpoint-one", strings.Repeat("x", 257), 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			adapter, _ := testAdapter(t, authOr(t, func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.status)
				_, _ = io.WriteString(writer, test.payload)
			}))
			result, err := adapter.Execute(t.Context(), "wazuh_list_agents", nil)
			require.NoError(t, err)
			require.True(t, result.IsError, result.Data)
			require.NotContains(t, result.Data, "SECRET_SENTINEL")
			require.NotContains(t, result.Data, "secret-password")
		})
	}
}

func TestJWTRefreshConcurrent(t *testing.T) {
	var logins atomic.Int32
	var reject atomic.Bool
	adapter, _ := testAdapter(t, func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasSuffix(request.URL.Path, "/authenticate") {
			logins.Add(1)
			_, _ = fmt.Fprintf(writer, `{"error":0,"data":{"token":%q}}`, testToken(time.Now().Add(time.Hour)))
			return
		}
		if reject.Swap(false) {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(writer, agentPageFixture())
	})
	var workers sync.WaitGroup
	for range 12 {
		workers.Go(func() {
			result, err := adapter.Execute(t.Context(), "wazuh_list_agents", nil)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
		})
	}
	workers.Wait()
	require.EqualValues(t, 1, logins.Load())
	adapter.mu.Lock()
	adapter.expires = time.Now().Add(-time.Second)
	adapter.mu.Unlock()
	require.True(t, adapter.Healthy(t.Context()))
	require.EqualValues(t, 2, logins.Load())
	reject.Store(true)
	require.True(t, adapter.Healthy(t.Context()))
	require.EqualValues(t, 3, logins.Load())
}

func TestInvalidJWT(t *testing.T) {
	for _, token := range []string{"bad-token", testToken(time.Now().Add(-time.Hour)), "a." + base64.RawURLEncoding.EncodeToString([]byte(`{"exp":0}`)) + ".b", "a.invalid.b"} {
		t.Run(token, func(t *testing.T) {
			adapter, _ := testAdapter(t, func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprintf(writer, `{"error":0,"data":{"token":%q}}`, token)
			})
			require.False(t, adapter.Healthy(t.Context()))
		})
	}
}

func TestRedirectAndTLS(t *testing.T) {
	var leaked atomic.Bool
	destination := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { leaked.Store(true) }))
	defer destination.Close()
	adapter, provider := testAdapter(t, func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, destination.URL, http.StatusFound)
	})
	require.False(t, adapter.Healthy(t.Context()))
	require.False(t, leaked.Load())
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials{"base_url": provider.URL, "username": "reader", "password": "password"}))
	require.False(t, adapter.Healthy(t.Context()))
}

func TestRateLimitAndCancellation(t *testing.T) {
	var reads atomic.Int32
	adapter, _ := testAdapter(t, authOr(t, func(writer http.ResponseWriter, _ *http.Request) {
		if reads.Add(1) == 1 {
			writer.Header().Set("Retry-After", "0")
			writer.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = io.WriteString(writer, agentPageFixture())
	}))
	require.True(t, adapter.Healthy(t.Context()))
	require.EqualValues(t, 2, reads.Load())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result, err := adapter.Execute(ctx, "wazuh_list_agents", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestExactAgentQuery(t *testing.T) {
	adapter, _ := testAdapter(t, authOr(t, func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "001", request.URL.Query().Get("agents_list"))
		require.Equal(t, "1", request.URL.Query().Get("limit"))
		_, _ = io.WriteString(writer, strings.Replace(agentPageFixture(), `"total_affected_items":2`, `"total_affected_items":1`, 1))
	}))
	result, err := adapter.Execute(t.Context(), "wazuh_get_agent", map[string]any{"agent_id": "001"})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range New().Tools() {
		require.Contains(t, dispatch, tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	names := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		names[tool.Name] = true
	}
	for name := range dispatch {
		require.True(t, names[name], name)
	}
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		require.Contains(t, dispatch, name)
	}
	require.Len(t, fieldCompactionSpecs, 3)
}

func TestCompactionShapes(t *testing.T) {
	for _, tool := range New().Tools() {
		t.Run(string(tool.Name), func(t *testing.T) {
			fields, ok := New().(mcp.FieldCompactionIntegration).CompactSpec(tool.Name)
			require.True(t, ok)
			data := `{"affected_items":[{"id":"001","name":"endpoint","status":"active","version":"4.14.8","os":{"name":"Linux","version":"1"},"lastKeepAlive":"today","group_config_status":"synced"}],"total_affected_items":1,"total_failed_items":0,"failed_items":[],"partial":false,"next_offset":null,"connection":{"active":1,"disconnected":0,"never_connected":0,"pending":0,"total":1},"configuration":{"synced":1,"not_synced":0,"total":1},"noise":"SECRET_SENTINEL"}`
			compacted, err := mcp.CompactJSON([]byte(data), fields)
			require.NoError(t, err)
			require.NotContains(t, string(compacted), "SECRET_SENTINEL")
			if tool.Name == "wazuh_get_agent_status_summary" {
				require.Contains(t, string(compacted), "not_synced")
			} else {
				require.Contains(t, string(compacted), "endpoint")
			}
		})
	}
}

func TestURLQueryEncoding(t *testing.T) {
	value := url.Values{"search": {"a&b"}}
	require.Equal(t, "search=a%26b", value.Encode())
}
