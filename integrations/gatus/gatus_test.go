package gatus

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

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

const endpointFixture = `{"key":"production_api","name":"API","group":"production","results":[{"timestamp":"2026-11-18T10:00:00Z","success":true,"status":200,"duration":1500000,"hostname":"SECRET","errors":["SECRET"],"conditionResults":[{"condition":"SECRET"}]},{"timestamp":"2026-11-18T11:00:00Z","success":false,"status":503,"duration":2500000}],"events":[{"type":"SECRET"}],"url":"SECRET"}`

func testAdapter(t *testing.T, handler http.HandlerFunc) (*gatus, *httptest.Server) {
	t.Helper()
	provider := httptest.NewTLSServer(handler)
	t.Cleanup(provider.Close)
	adapter := New().(*gatus)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials{"base_url": provider.URL + "/gatus", "username": "reader", "password": "test-secret", "ca_cert": string(cert)}))
	t.Cleanup(func() { require.NoError(t, adapter.Close()) })
	return adapter, provider
}

func TestConfigure(t *testing.T) {
	for _, tc := range []struct{ name, key, value string }{
		{"missing URL", "base_url", ""}, {"HTTP", "base_url", "http://status.example"},
		{"userinfo", "base_url", "https://user:secret@status.example"}, {"query", "base_url", "https://status.example?x=1"},
		{"fragment", "base_url", "https://status.example#x"}, {"encoded prefix", "base_url", "https://status.example/%2f"},
		{"traversal", "base_url", "https://status.example/a/../b"}, {"port zero", "base_url", "https://status.example:0"},
		{"port large", "base_url", "https://status.example:65536"}, {"port empty", "base_url", "https://status.example:"},
		{"missing username", "username", ""}, {"username colon", "username", "a:b"}, {"username newline", "username", "a\nb"},
		{"missing password", "password", ""}, {"password control", "password", "secret\x00"}, {"invalid CA", "ca_cert", "invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter := New()
			creds := mcp.Credentials{"base_url": "https://status.example", "username": "reader", "password": "test-secret"}
			require.NoError(t, adapter.Configure(t.Context(), creds))
			creds[tc.key] = tc.value
			err := adapter.Configure(t.Context(), creds)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "test-secret")
			require.False(t, adapter.Healthy(t.Context()))
		})
	}
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	adapter := New()
	require.Equal(t, "gatus", adapter.Name())
	require.Len(t, adapter.Tools(), 3)
	require.Contains(t, adapter.Tools()[0].Description, "Start here")
	seen := map[mcp.ToolName]bool{}
	for _, tool := range adapter.Tools() {
		require.NotEmpty(t, tool.Description)
		require.True(t, strings.HasPrefix(string(tool.Name), "gatus_"))
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

func TestTools(t *testing.T) {
	for _, tc := range []struct {
		name                                 mcp.ToolName
		args                                 map[string]any
		path, query, body, contentType, want string
	}{
		{"gatus_list_endpoints", nil, "/gatus/api/v1/endpoints/statuses", "page=1&pageSize=1", "[" + endpointFixture + "]", "application/json", `"duration_ms":2.5`},
		{"gatus_get_endpoint", map[string]any{"key": "production_api", "page": 2, "page_size": 3}, "/gatus/api/v1/endpoints/production_api/statuses", "page=2&pageSize=3", endpointFixture, "application/json", `"page":2`},
		{"gatus_get_uptime", map[string]any{"key": "production_api"}, "/gatus/api/v1/endpoints/production_api/uptimes/24h", "", "0.975000", "text/plain; charset=utf-8", `"ratio":0.975`},
	} {
		t.Run(string(tc.name), func(t *testing.T) {
			var calls atomic.Int32
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				require.Equal(t, http.MethodGet, r.Method)
				username, password, ok := r.BasicAuth()
				require.True(t, ok)
				require.Equal(t, "reader", username)
				require.Equal(t, "test-secret", password)
				require.Equal(t, tc.path, r.URL.Path)
				require.Equal(t, tc.query, r.URL.RawQuery)
				w.Header().Set("Content-Type", tc.contentType)
				_, _ = io.WriteString(w, tc.body)
			})
			result, err := adapter.Execute(t.Context(), tc.name, tc.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			require.Contains(t, result.Data, tc.want)
			require.NotContains(t, result.Data, "SECRET")
			require.NotContains(t, result.Data, "test-secret")
			require.EqualValues(t, 1, calls.Load())
			var raw any
			require.NoError(t, json.Unmarshal([]byte(result.Data), &raw))
			fields, ok := adapter.CompactSpec(tc.name)
			require.True(t, ok)
			require.Equal(t, raw, mcp.CompactAny(raw, fields))
		})
	}
}

func TestListFiltersAndLatest(t *testing.T) {
	body := "[" + endpointFixture + `,{"key":"other_empty","name":"Empty","group":"other","results":[]},{"key":"production_ok","name":"Healthy","group":"production","results":[{"timestamp":"2026-11-18T12:00:00Z","success":true,"duration":0}]}]`
	for _, tc := range []struct {
		name               string
		args               map[string]any
		matching, returned int
		truncated          bool
	}{
		{"all", nil, 3, 3, false},
		{"limit", map[string]any{"limit": 1}, 3, 1, true},
		{"group", map[string]any{"group": "production"}, 2, 2, false},
		{"name", map[string]any{"name": "API"}, 1, 1, false},
		{"unhealthy excludes unknown", map[string]any{"only_unhealthy": true}, 1, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, body)
			})
			result, err := adapter.Execute(t.Context(), "gatus_list_endpoints", tc.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			var page struct {
				TotalFetched, MatchingCount, ReturnedCount int
				Truncated                                  bool
			}
			var raw map[string]any
			require.NoError(t, json.Unmarshal([]byte(result.Data), &raw))
			page.TotalFetched = int(raw["total_fetched"].(float64))
			page.MatchingCount = int(raw["matching_count"].(float64))
			page.ReturnedCount = int(raw["returned_count"].(float64))
			page.Truncated = raw["truncated"].(bool)
			require.Equal(t, 3, page.TotalFetched)
			require.Equal(t, tc.matching, page.MatchingCount)
			require.Equal(t, tc.returned, page.ReturnedCount)
			require.Equal(t, tc.truncated, page.Truncated)
			if tc.name == "unhealthy excludes unknown" {
				require.Contains(t, result.Data, `"success":false`)
			}
		})
	}
}

func TestInvalidArguments(t *testing.T) {
	var calls atomic.Int32
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); _, _ = io.WriteString(w, "[]") })
	cases := []struct {
		tool mcp.ToolName
		args map[string]any
	}{
		{"gatus_list_endpoints", map[string]any{"limit": 0}}, {"gatus_list_endpoints", map[string]any{"limit": 51}},
		{"gatus_list_endpoints", map[string]any{"limit": 1.5}}, {"gatus_list_endpoints", map[string]any{"name": true}},
		{"gatus_list_endpoints", map[string]any{"only_unhealthy": "yes"}}, {"gatus_list_endpoints", map[string]any{"base_url": "https://other.example"}},
		{"gatus_get_endpoint", nil}, {"gatus_get_endpoint", map[string]any{"key": "ok", "page": 0}},
		{"gatus_get_endpoint", map[string]any{"key": "ok", "page_size": 21}}, {"gatus_get_endpoint", map[string]any{"key": "ok", "page": 1.5}},
		{"gatus_get_uptime", map[string]any{"key": "ok", "duration": "2h"}},
	}
	for _, key := range []string{"../secret", "a/b", `a\b`, "a%2fb", "a?b", "a#b", "a\nb", "a+b", strings.Repeat("a", 257)} {
		cases = append(cases, struct {
			tool mcp.ToolName
			args map[string]any
		}{"gatus_get_endpoint", map[string]any{"key": key}})
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

func TestUnknownUnconfiguredAndClose(t *testing.T) {
	adapter := New().(*gatus)
	for _, tool := range []mcp.ToolName{"unknown", "gatus_list_endpoints"} {
		result, err := adapter.Execute(t.Context(), tool, nil)
		require.NoError(t, err)
		require.True(t, result.IsError)
	}
	require.False(t, adapter.Healthy(context.Background()))
	require.NoError(t, adapter.Close())
	require.NoError(t, adapter.Close())
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	require.Len(t, fieldCompactionSpecs, 3)
	for name := range fieldCompactionSpecs {
		require.Contains(t, dispatch, name)
	}
}
