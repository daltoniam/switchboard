package ntfy

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

const fixture = `{"event":"message","id":"AbCd1234EfGh","time":1700000000,"priority":4,"tags":["warning"],"title":"Backup alert","message":"Backup failed","expires":1700043200,"topic":"ops","click":"SECRET","attachment":{"url":"SECRET"},"actions":[{"url":"SECRET"}],"sender":"SECRET"}`

func testAdapter(t *testing.T, handler http.HandlerFunc) (*ntfy, *httptest.Server) {
	t.Helper()
	provider := httptest.NewTLSServer(handler)
	t.Cleanup(provider.Close)
	adapter := New().(*ntfy)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials{"base_url": provider.URL + "/ntfy", "topic": "ops", "api_token": "test-secret", "ca_cert": string(cert)}))
	t.Cleanup(func() { require.NoError(t, adapter.Close()) })
	return adapter, provider
}

func resultJSON(t *testing.T, adapter *ntfy, tool mcp.ToolName, args map[string]any) (*mcp.ToolResult, map[string]any) {
	t.Helper()
	result, err := adapter.Execute(t.Context(), tool, args)
	require.NoError(t, err)
	var value map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &value), result.Data)
	require.Less(t, len(result.Data), 40<<10)
	return result, value
}

func TestConfigure(t *testing.T) {
	for _, tc := range []struct{ name, key, value string }{
		{"missing URL", "base_url", ""}, {"HTTP", "base_url", "http://ntfy.example"},
		{"userinfo", "base_url", "https://reader:SECRET@ntfy.example"}, {"query", "base_url", "https://ntfy.example?auth=SECRET"},
		{"fragment", "base_url", "https://ntfy.example#SECRET"}, {"encoded path", "base_url", "https://ntfy.example/%2f"},
		{"traversal", "base_url", "https://ntfy.example/a/../b"}, {"backslash", "base_url", `https://ntfy.example/a\b`},
		{"zero port", "base_url", "https://ntfy.example:0"}, {"large port", "base_url", "https://ntfy.example:65536"},
		{"empty port", "base_url", "https://ntfy.example:"}, {"empty topic", "topic", ""},
		{"multi topic", "topic", "ops,other"}, {"path topic", "topic", "ops/publish"}, {"URL topic", "topic", "https://evil.example"},
		{"long topic", "topic", strings.Repeat("x", 65)}, {"token missing", "api_token", ""},
		{"token newline", "api_token", "SECRET\n"}, {"token whitespace", "api_token", " SECRET"}, {"bad CA", "ca_cert", "SECRET"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter := New().(*ntfy)
			creds := mcp.Credentials{"base_url": "https://ntfy.example", "topic": "ops", "api_token": "test-secret"}
			require.NoError(t, adapter.Configure(t.Context(), creds))
			creds[tc.key] = tc.value
			err := adapter.Configure(t.Context(), creds)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "SECRET")
			require.NotContains(t, err.Error(), "test-secret")
			require.False(t, adapter.Healthy(t.Context()))
		})
	}
	for _, topic := range []string{"docs", "static", "file", "app", "metrics", "account", "settings", "signup", "login", "v1"} {
		t.Run("reserved "+topic, func(t *testing.T) {
			require.Error(t, New().Configure(t.Context(), mcp.Credentials{"base_url": "https://ntfy.example", "topic": topic, "api_token": "test-secret"}))
		})
	}
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	adapter := New()
	require.Equal(t, "ntfy", adapter.Name())
	require.Len(t, adapter.Tools(), 2)
	require.Contains(t, adapter.Tools()[0].Description, "Start here")
	seen := map[mcp.ToolName]bool{}
	for _, tool := range adapter.Tools() {
		require.NotEmpty(t, tool.Description)
		require.True(t, strings.HasPrefix(string(tool.Name), "ntfy_"))
		require.Equal(t, mcp.AccessRead, tool.Access)
		require.False(t, seen[tool.Name])
		seen[tool.Name] = true
		require.Contains(t, dispatch, tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	for name := range dispatch {
		found := false
		for _, tool := range New().Tools() {
			found = found || name == tool.Name
		}
		require.True(t, found, name)
	}
}

func TestPollContractAndCompaction(t *testing.T) {
	since := strconv.FormatInt(time.Now().Unix()-300, 10)
	for _, tc := range []struct {
		name  mcp.ToolName
		args  map[string]any
		query string
	}{
		{"ntfy_list_messages", nil, "poll=1&since=latest"},
		{"ntfy_list_messages", map[string]any{"since": since, "priority": 4, "tags": "warning", "limit": 1}, "poll=1&priority=4&since=" + since + "&tags=warning"},
		{"ntfy_get_message", map[string]any{"id": "AbCd1234EfGh", "since": since}, "id=AbCd1234EfGh&poll=1&since=" + since},
	} {
		t.Run(tc.query, func(t *testing.T) {
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "/ntfy/ops/json", r.URL.Path)
				require.Equal(t, tc.query, r.URL.RawQuery)
				require.Equal(t, "Bearer test-secret", r.Header.Get("Authorization"))
				require.Empty(t, r.Header.Get("X-Subscriber-Rate"))
				w.Header().Set("Content-Type", "application/x-ndjson")
				_, _ = io.WriteString(w, `{"event":"open","id":"ignored"}`+"\n"+fixture+"\n"+`{"event":"keepalive"}`+"\n")
			})
			result, value := resultJSON(t, adapter, tc.name, tc.args)
			require.False(t, result.IsError, result.Data)
			require.NotContains(t, result.Data, "SECRET")
			require.NotContains(t, result.Data, "test-secret")
			require.Len(t, value["messages"], 1)
			require.Equal(t, "ok", value["status"])
			require.Equal(t, false, value["incomplete"])
			fields, ok := adapter.CompactSpec(tc.name)
			require.True(t, ok)
			require.Equal(t, value, mcp.CompactAny(value, fields))
		})
	}
}

func TestInvalidArguments(t *testing.T) {
	var calls atomic.Int32
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) { calls.Add(1) })
	now := time.Now().Unix()
	cases := []struct {
		tool mcp.ToolName
		args map[string]any
	}{
		{"ntfy_list_messages", map[string]any{"limit": 0}}, {"ntfy_list_messages", map[string]any{"limit": 21}},
		{"ntfy_list_messages", map[string]any{"limit": 1.5}}, {"ntfy_list_messages", map[string]any{"limit": nil}},
		{"ntfy_list_messages", map[string]any{"priority": 0}}, {"ntfy_list_messages", map[string]any{"priority": 6}},
		{"ntfy_list_messages", map[string]any{"priority": true}}, {"ntfy_list_messages", map[string]any{"tags": "a,,b"}},
		{"ntfy_list_messages", map[string]any{"tags": strings.Repeat("a", 65)}}, {"ntfy_list_messages", map[string]any{"tags": "a\nb"}},
		{"ntfy_list_messages", map[string]any{"topic": "other"}}, {"ntfy_list_messages", map[string]any{"scheduled": true}},
		{"ntfy_get_message", nil}, {"ntfy_get_message", map[string]any{"id": "AbCd1234EfGh"}},
		{"ntfy_get_message", map[string]any{"id": "AbCd1234EfGh", "since": "latest"}},
	}
	for _, since := range []any{"", "all", "1h", now - 3602, now + 60, 1.5, true, nil, "-1", "0000000000000000000000"} {
		cases = append(cases, struct {
			tool mcp.ToolName
			args map[string]any
		}{"ntfy_list_messages", map[string]any{"since": since}})
	}
	for _, id := range []string{"a", "AbCd1234EfG!", "AbCd1234EfG/", "AbCd1234EfGhX", "../publish"} {
		cases = append(cases, struct {
			tool mcp.ToolName
			args map[string]any
		}{"ntfy_get_message", map[string]any{"id": id, "since": now - 300}})
	}
	for index, tc := range cases {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			result, err := adapter.Execute(t.Context(), tc.tool, tc.args)
			require.NoError(t, err)
			require.True(t, result.IsError, result.Data)
		})
	}
	require.Zero(t, calls.Load())
}

func TestEmptyNotFoundAndUpstreamTruncation(t *testing.T) {
	for _, truncated := range []bool{false, true} {
		t.Run(fmt.Sprint(truncated), func(t *testing.T) {
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/x-ndjson")
				if truncated {
					w.Header().Set("X-Messages-Truncated", "1")
				}
			})
			result, value := resultJSON(t, adapter, "ntfy_get_message", map[string]any{"id": "AbCd1234EfGh", "since": time.Now().Unix() - 300})
			require.False(t, result.IsError, result.Data)
			require.Equal(t, truncated, value["upstream_truncated"])
			require.Equal(t, truncated, value["incomplete"])
			if truncated {
				require.Equal(t, "incomplete", value["status"])
			} else {
				require.Equal(t, "not_found", value["status"])
			}
			require.Contains(t, value["caveat"], "cache")
			require.True(t, adapter.Healthy(t.Context()))
		})
	}
}

func TestUnknownUnconfiguredAndClose(t *testing.T) {
	adapter := New().(*ntfy)
	for _, tool := range []mcp.ToolName{"unknown", "ntfy_list_messages"} {
		result, err := adapter.Execute(t.Context(), tool, nil)
		require.NoError(t, err)
		require.True(t, result.IsError)
	}
	require.False(t, adapter.Healthy(context.Background()))
	require.NoError(t, adapter.Close())
	require.NoError(t, adapter.Close())
	configured, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) { w.Header().Set("Content-Type", "application/x-ndjson") })
	require.True(t, configured.Healthy(t.Context()))
	require.NoError(t, configured.Close())
	require.False(t, configured.Healthy(t.Context()))
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	require.Len(t, fieldCompactionSpecs, 2)
	for name := range fieldCompactionSpecs {
		require.Contains(t, dispatch, name)
	}
}
