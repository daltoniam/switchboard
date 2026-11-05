package argoworkflows

import (
	"context"
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
	"github.com/stretchr/testify/require"
)

const workflowFixture = `{"metadata":{"name":"failed-run","namespace":"approved","annotations":{"secret":"secret-sentinel"}},"spec":{"arguments":{"parameters":[{"value":"secret-sentinel"}]}},"status":{"phase":"Failed","startedAt":"2026-11-01T10:00:00Z","finishedAt":"2026-11-01T10:01:00Z","progress":"0/1","message":"secret-sentinel","nodes":{"failed-run":{"name":"failed-run","displayName":"secret-sentinel","phase":"Failed","message":"secret-sentinel","outputs":{"result":"secret-sentinel"}}}}}`

func configured(t *testing.T, handler http.HandlerFunc) (*argoworkflows, *httptest.Server) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	adapter := New().(*argoworkflows)
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials{"base_url": server.URL + "/prefix", "namespace": "approved", "token": "test-token", "ca_cert": strings.ReplaceAll(string(cert), "\n", `\n`)}))
	return adapter, server
}

func TestConfigure(t *testing.T) {
	tests := []struct{ name, key, value string }{
		{"missing URL", "base_url", ""},
		{"HTTP", "base_url", "http://localhost"},
		{"userinfo", "base_url", "https://user:password@example.test"},
		{"query", "base_url", "https://example.test?token=secret"},
		{"fragment", "base_url", "https://example.test#secret"},
		{"encoded path", "base_url", "https://example.test/%2e%2e"},
		{"traversal prefix", "base_url", "https://example.test/a/../b"},
		{"missing namespace", "namespace", ""},
		{"namespace separator", "namespace", "a/b"},
		{"namespace too long", "namespace", strings.Repeat("a", 64)},
		{"missing token", "token", ""},
		{"header injection", "token", "token\r\nInjected: yes"},
		{"bearer prefix", "token", "Bearer token"},
		{"invalid CA", "ca_cert", "not a certificate"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			creds := mcp.Credentials{"base_url": "https://argo.example.test/prefix", "namespace": "approved", "token": "test-token"}
			creds[test.key] = test.value
			require.Error(t, New().Configure(t.Context(), creds))
		})
	}
	require.Equal(t, "argoworkflows", New().Name())
	require.NoError(t, New().Configure(t.Context(), mcp.Credentials{"base_url": "https://argo.example.test/prefix/", "namespace": "approved", "token": "test-token"}))
	require.False(t, New().Healthy(t.Context()))
}

func TestExecuteProjection(t *testing.T) {
	adapter, _ := configured(t, func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodGet, request.Method)
		require.Equal(t, "Bearer test-token", request.Header.Get("Authorization"))
		require.NotEmpty(t, request.URL.Query().Get("fields"))
		switch request.URL.Path {
		case "/prefix/api/v1/workflows/approved":
			require.Equal(t, "2", request.URL.Query().Get("listOptions.limit"))
			require.Equal(t, "opaque+/=", request.URL.Query().Get("listOptions.continue"))
			require.Equal(t, "team=infra", request.URL.Query().Get("listOptions.labelSelector"))
			_, _ = fmt.Fprintf(writer, `{"metadata":{"continue":"next+/="},"items":[%s]}`, workflowFixture)
		case "/prefix/api/v1/workflows/approved/failed-run":
			_, _ = io.WriteString(writer, workflowFixture)
		default:
			t.Errorf("unexpected path %s", request.URL.Path)
			http.NotFound(writer, request)
		}
	})
	for _, test := range []struct {
		name mcp.ToolName
		args map[string]any
	}{
		{"argoworkflows_list_workflows", map[string]any{"limit": 2, "cursor": "opaque+/=", "label_selector": "team=infra"}},
		{"argoworkflows_get_workflow", map[string]any{"name": "failed-run"}},
	} {
		t.Run(string(test.name), func(t *testing.T) {
			result, err := adapter.Execute(t.Context(), test.name, test.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			require.Contains(t, result.Data, "failed-run")
			require.Contains(t, result.Data, "Failed")
			require.NotContains(t, result.Data, "secret-sentinel")
			require.NotContains(t, result.Data, "spec")
			if test.name == "argoworkflows_list_workflows" {
				require.Contains(t, result.Data, "next+/=")
			} else {
				require.Contains(t, result.Data, "failed_nodes")
				require.Contains(t, result.Data, "messages_omitted")
			}
			fields, ok := adapter.CompactSpec(test.name)
			require.True(t, ok)
			compacted, err := mcp.CompactJSON([]byte(result.Data), fields)
			require.NoError(t, err)
			require.Contains(t, string(compacted), "failed-run")
			require.NotContains(t, string(compacted), "secret-sentinel")
		})
	}
}

func TestArgumentsFailBeforeRequest(t *testing.T) {
	var calls atomic.Int32
	adapter, _ := configured(t, func(writer http.ResponseWriter, _ *http.Request) { calls.Add(1); _, _ = io.WriteString(writer, `{}`) })
	tests := []struct {
		name string
		tool mcp.ToolName
		args map[string]any
	}{
		{"unknown tool", "argoworkflows_delete_workflow", nil},
		{"missing name", "argoworkflows_get_workflow", nil},
		{"traversal", "argoworkflows_get_workflow", map[string]any{"name": "../secret"}},
		{"encoded separator", "argoworkflows_get_workflow", map[string]any{"name": "a%2fb"}},
		{"alias", "argoworkflows_get_workflow", map[string]any{"name": "@latest"}},
		{"override namespace", "argoworkflows_get_workflow", map[string]any{"name": "safe", "namespace": "other"}},
		{"override URL", "argoworkflows_list_workflows", map[string]any{"url": "https://other.test"}},
		{"negative limit", "argoworkflows_list_workflows", map[string]any{"limit": -1}},
		{"excessive limit", "argoworkflows_list_workflows", map[string]any{"limit": 101}},
		{"invalid limit", "argoworkflows_list_workflows", map[string]any{"limit": "many"}},
		{"long cursor", "argoworkflows_list_workflows", map[string]any{"cursor": strings.Repeat("a", 8193)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := adapter.Execute(t.Context(), test.tool, test.args)
			require.NoError(t, err)
			require.True(t, result.IsError)
		})
	}
	require.Zero(t, calls.Load())
}

func TestUpstreamFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"unauthorized", 401, "secret-sentinel", "401"},
		{"forbidden", 403, "secret-sentinel", "403"},
		{"not found", 404, "secret-sentinel", "404"},
		{"expired cursor", 410, "secret-sentinel", "restart"},
		{"rate limited", 429, "secret-sentinel", "429"},
		{"server error", 500, "secret-sentinel", "500"},
		{"redirect", 302, "secret-sentinel", "302"},
		{"malformed", 200, "not JSON", "JSON"},
		{"trailing data", 200, `{"items":[]} {"secret":"sentinel"}`, "JSON"},
		{"oversized", 200, strings.Repeat(" ", 1024*1024+1), "limit"},
		{"ignored limit", 200, `{"items":[{},{}]}`, "limit"},
		{"wrong shape", 200, `{"unexpected":"value"}`, "items"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapter, _ := configured(t, func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Retry-After", "0")
				writer.WriteHeader(test.status)
				_, _ = io.WriteString(writer, test.body)
			})
			result, err := adapter.Execute(t.Context(), "argoworkflows_list_workflows", map[string]any{"limit": 1})
			require.NoError(t, err)
			require.True(t, result.IsError, result.Data)
			require.Contains(t, result.Data, test.want)
			require.NotContains(t, result.Data, "secret-sentinel")
		})
	}
}

func TestTLSAndCancellation(t *testing.T) {
	adapter, server := configured(t, func(writer http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(writer, `{"items":[]}`) })
	require.True(t, adapter.Healthy(t.Context()))
	untrusted := New()
	require.NoError(t, untrusted.Configure(t.Context(), mcp.Credentials{"base_url": server.URL, "namespace": "approved", "token": "test-token"}))
	require.False(t, untrusted.Healthy(t.Context()))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.False(t, adapter.Healthy(ctx))
	ctx, cancel = context.WithTimeout(t.Context(), time.Nanosecond)
	defer cancel()
	require.False(t, adapter.Healthy(ctx))
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range New().Tools() {
		require.Contains(t, dispatch, tool.Name)
		require.Equal(t, mcp.AccessRead, tool.Access)
		require.NotEmpty(t, tool.Description)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	known := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		known[tool.Name] = true
	}
	for name := range dispatch {
		require.True(t, known[name], name)
	}
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	require.Len(t, fieldCompactionSpecs, 2)
	for name := range fieldCompactionSpecs {
		require.Contains(t, dispatch, name)
	}
}
