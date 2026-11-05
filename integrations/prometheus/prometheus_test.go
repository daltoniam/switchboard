package prometheus

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func configured(t *testing.T, handler http.HandlerFunc) *prometheus {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	integration := New().(*prometheus)
	require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL, "bearer_token": "test-secret", "headers": `{"X-Scope-OrgID":"tenant"}`}))
	return integration
}

func TestConfigure(t *testing.T) {
	cases := []struct {
		name  string
		creds mcp.Credentials
		valid bool
	}{
		{"missing_url", mcp.Credentials{}, false},
		{"invalid_scheme", mcp.Credentials{"base_url": "file:///tmp"}, false},
		{"userinfo", mcp.Credentials{"base_url": "https://user:secret@metrics.example"}, false},
		{"query", mcp.Credentials{"base_url": "https://metrics.example?token=secret"}, false},
		{"fragment", mcp.Credentials{"base_url": "https://metrics.example#x"}, false},
		{"unsafe_http", mcp.Credentials{"base_url": "http://metrics.example"}, false},
		{"missing_password", mcp.Credentials{"base_url": "https://metrics.example", "username": "user"}, false},
		{"missing_username", mcp.Credentials{"base_url": "https://metrics.example", "password": "secret"}, false},
		{"ambiguous_auth", mcp.Credentials{"base_url": "https://metrics.example", "username": "user", "password": "secret", "bearer_token": "token"}, false},
		{"header_override", mcp.Credentials{"base_url": "https://metrics.example", "headers": `{"Authorization":"secret"}`}, false},
		{"cookie_header", mcp.Credentials{"base_url": "https://metrics.example", "headers": `{"Cookie":"secret"}`}, false},
		{"header_injection", mcp.Credentials{"base_url": "https://metrics.example", "headers": "{\"X-Tenant\":\"bad\\r\\nheader\"}"}, false},
		{"invalid_headers", mcp.Credentials{"base_url": "https://metrics.example", "headers": "[]"}, false},
		{"invalid_ca", mcp.Credentials{"base_url": "https://metrics.example", "ca_cert": "garbage"}, false},
		{"anonymous_tls", mcp.Credentials{"base_url": "https://metrics.example/prometheus"}, true},
		{"basic", mcp.Credentials{"base_url": "https://metrics.example", "username": "user", "password": "secret"}, true},
		{"bearer", mcp.Credentials{"base_url": "https://metrics.example", "bearer_token": "token"}, true},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			integration := New()
			err := integration.Configure(context.Background(), item.creds)
			if item.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
	require.False(t, New().Healthy(context.Background()))
	result, err := New().Execute(context.Background(), "prometheus_query", map[string]any{"query": "up"})
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	seen := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		require.Contains(t, dispatch, tool.Name)
		require.False(t, seen[tool.Name])
		seen[tool.Name] = true
		require.True(t, strings.HasPrefix(string(tool.Name), "prometheus_"))
		require.NotEmpty(t, tool.Description)
	}
	require.Len(t, seen, 9)
	require.Contains(t, New().Tools()[0].Description, "Start here")
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	names := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		names[tool.Name] = true
	}
	for name := range dispatch {
		require.True(t, names[name], "%s", name)
	}
	result, err := New().Execute(context.Background(), "prometheus_unknown", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Data, "unknown tool")
}

func TestToolsRequests(t *testing.T) {
	cases := []struct {
		name     mcp.ToolName
		args     map[string]any
		paths    []string
		payload  string
		expected string
	}{
		{"prometheus_query", map[string]any{"query": "up", "time": "now-1h", "limit": 2}, []string{"/api/v1/query"}, `{"resultType":"vector","result":[{"metric":{"job":"api"},"value":[123,"1"]}]}`, `"job":"api"`},
		{"prometheus_query_range", map[string]any{"query": "up", "start": "100", "end": "160", "step": "30s"}, []string{"/api/v1/query_range"}, `{"resultType":"matrix","result":[{"metric":{"job":"api"},"values":[[100,"1"],[130,"0"],[160,"1"]]}]}`, `"timestamps":[100,130,160]`},
		{"prometheus_list_alerts", map[string]any{"state": "firing"}, []string{"/api/v1/alerts"}, `{"alerts":[{"state":"pending","labels":{"alertname":"Other"}},{"state":"firing","labels":{"alertname":"Down"},"value":"1"}]}`, "Down"},
		{"prometheus_list_rules", map[string]any{"type": "alert", "rule_name": []any{"Down"}, "group": "api", "group_limit": 2, "group_next_token": "next"}, []string{"/api/v1/rules"}, `{"groups":[{"name":"api","rules":[{"name":"Down","health":"ok","query":"up == 0","type":"alerting"}]}],"groupNextToken":"more"}`, "more"},
		{"prometheus_list_targets", map[string]any{"state": "active", "scrape_pool": "api"}, []string{"/api/v1/targets"}, `{"activeTargets":[{"health":"down","lastError":"connection refused","scrapePool":"api","labels":{"job":"api"}}],"droppedTargets":[]}`, "connection refused"},
		{"prometheus_search_metrics", map[string]any{"match": []any{"{job=\"api\"}"}, "limit": 2}, []string{"/api/v1/label/__name__/values", "/api/v1/metadata"}, `[]`, "http_requests_total"},
		{"prometheus_list_label_values", map[string]any{"label": "job", "match": []any{"up"}}, []string{"/api/v1/label/job/values"}, `["api"]`, "api"},
		{"prometheus_list_series", map[string]any{"match": []any{"up"}, "start": "100", "end": "160"}, []string{"/api/v1/series"}, `[{"__name__":"up","job":"api"}]`, "up"},
		{"prometheus_server_info", nil, []string{"/api/v1/status/buildinfo", "/api/v1/status/runtimeinfo"}, `{"version":"3.0","reloadConfigSuccess":true}`, "3.0"},
	}
	for _, item := range cases {
		t.Run(string(item.name), func(t *testing.T) {
			var paths []string
			integration := configured(t, func(w http.ResponseWriter, req *http.Request) {
				require.Equal(t, "GET", req.Method)
				require.Equal(t, "Bearer test-secret", req.Header.Get("Authorization"))
				require.Equal(t, "tenant", req.Header.Get("X-Scope-OrgID"))
				paths = append(paths, req.URL.Path)
				vals := req.URL.Query()
				switch req.URL.Path {
				case "/api/v1/query":
					require.Equal(t, "up", vals.Get("query"))
					require.Equal(t, "2", vals.Get("limit"))
					require.NotContains(t, vals.Get("time"), "now")
				case "/api/v1/query_range":
					require.Equal(t, "30", vals.Get("step"))
					require.Equal(t, "100", vals.Get("start"))
					require.Equal(t, "160", vals.Get("end"))
				case "/api/v1/rules":
					require.Equal(t, "alert", vals.Get("type"))
					require.Equal(t, []string{"Down"}, vals["rule_name[]"])
					require.Equal(t, "api", vals.Get("rule_group[]"))
					require.Equal(t, "next", vals.Get("group_next_token"))
				case "/api/v1/targets":
					require.Equal(t, "active", vals.Get("state"))
					require.Equal(t, "api", vals.Get("scrapePool"))
				case "/api/v1/series", "/api/v1/label/job/values":
					require.Equal(t, []string{"up"}, vals["match[]"])
				}
				payload := item.payload
				if item.name == "prometheus_search_metrics" {
					if strings.HasSuffix(req.URL.Path, "values") {
						payload = `["http_requests_total"]`
						require.Equal(t, []string{"{job=\"api\"}"}, vals["match[]"])
					} else {
						payload = `{"http_requests_total":[{"help":"Requests","type":"counter","unit":""}]}`
					}
				}
				_, _ = fmt.Fprintf(w, `{"status":"success","data":%s,"warnings":["partial data"],"infos":["query info"]}`, payload)
			})
			result, err := integration.Execute(context.Background(), item.name, item.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			require.Equal(t, item.paths, paths)
			require.Contains(t, result.Data, item.expected)
			require.Contains(t, result.Data, "partial data")
			require.Contains(t, result.Data, "query info")
			if item.name == "prometheus_list_alerts" {
				require.NotContains(t, result.Data, "Other")
			}
		})
	}
}

func TestInvalidArgumentsDoNotRequest(t *testing.T) {
	cases := []struct {
		tool mcp.ToolName
		args map[string]any
	}{
		{"prometheus_query", nil},
		{"prometheus_query", map[string]any{"query": "up", "limit": 0}},
		{"prometheus_query", map[string]any{"query": "up", "limit": 101}},
		{"prometheus_query", map[string]any{"query": "up", "time": "NaN"}},
		{"prometheus_query", map[string]any{"query": []any{"up"}}},
		{"prometheus_query_range", map[string]any{"query": "up", "start": "160", "end": "100"}},
		{"prometheus_query_range", map[string]any{"query": "up", "start": "0", "end": "10000", "step": "1s"}},
		{"prometheus_query_range", map[string]any{"query": "up", "start": "0", "end": "100", "step": "0"}},
		{"prometheus_query_range", map[string]any{"query": "up", "start": "0", "end": "100", "step": "NaN"}},
		{"prometheus_list_label_values", map[string]any{"label": "../../admin"}},
		{"prometheus_list_series", nil},
		{"prometheus_list_series", map[string]any{"match": []any{true}}},
		{"prometheus_list_targets", map[string]any{"state": "bad"}},
		{"prometheus_list_rules", map[string]any{"type": "delete"}},
		{"prometheus_list_rules", map[string]any{"group_limit": 0}},
		{"prometheus_list_alerts", map[string]any{"state": "bad"}},
	}
	for index, item := range cases {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			integration := configured(t, func(http.ResponseWriter, *http.Request) { t.Fatal("invalid args sent request") })
			result, err := integration.Execute(context.Background(), item.tool, item.args)
			require.NoError(t, err)
			require.True(t, result.IsError)
		})
	}
}

func TestHTTPFailures(t *testing.T) {
	cases := []struct {
		name  string
		code  int
		body  string
		retry bool
	}{
		{"unauthorized", 401, `{"status":"error","error":"test-secret"}`, false},
		{"forbidden", 403, `{"status":"error","error":"denied"}`, false},
		{"bad_query", 422, `{"status":"error","errorType":"execution","error":"bad expression"}`, false},
		{"busy", 429, `{"status":"error","error":"busy"}`, true},
		{"timeout", 503, `{"status":"error","error":"timeout"}`, true},
		{"envelope_error", 200, `{"status":"error","errorType":"bad_data","error":"invalid"}`, false},
		{"html", 200, `<html>login</html>`, false},
		{"empty", 204, ``, false},
		{"missing_status", 200, `{"data":{}}`, false},
		{"oversize", 200, strings.Repeat("x", maxResponseSize+1), false},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			integration := configured(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", "2")
				w.WriteHeader(item.code)
				_, _ = w.Write([]byte(item.body))
			})
			result, err := integration.Execute(context.Background(), "prometheus_query", map[string]any{"query": "up"})
			if item.retry {
				require.Error(t, err)
				require.True(t, mcp.IsRetryable(err))
			} else {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.NotContains(t, result.Data, "test-secret")
			}
			require.False(t, integration.Healthy(context.Background()))
		})
	}
}

func TestRedirectNeverFollowed(t *testing.T) {
	destination := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("redirect followed") }))
	defer destination.Close()
	integration := configured(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, http.StatusFound) })
	result, err := integration.Execute(context.Background(), "prometheus_query", map[string]any{"query": "up"})
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestBasicAuthAndBasePath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/prometheus/api/v1/status/buildinfo", r.URL.Path)
		user, password, ok := r.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "user", user)
		require.Equal(t, "password", password)
		_, _ = w.Write([]byte(`{"status":"success","data":{"version":"3.0"}}`))
	}))
	defer srv.Close()
	integration := New()
	require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL + "/prometheus/", "username": "user", "password": "password"}))
	require.True(t, integration.Healthy(context.Background()))
}

func TestFieldCompactionSpecs_ParityAndShape(t *testing.T) {
	integration := New().(*prometheus)
	require.Len(t, fieldCompactionSpecs, len(dispatch))
	for tool := range dispatch {
		t.Run(string(tool), func(t *testing.T) {
			fields, ok := integration.CompactSpec(tool)
			require.True(t, ok)
			require.NotEmpty(t, fields)
			payload := `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"job":"api"},"value":[100,"1"]}]},"warnings":["partial"],"infos":["info"],"truncated":true}`
			output, err := mcp.CompactJSON([]byte(payload), fields)
			require.NoError(t, err)
			assert.Contains(t, string(output), "partial")
			assert.Contains(t, string(output), "api")
			assert.Contains(t, string(output), "truncated")
		})
	}
	for tool := range fieldCompactionSpecs {
		require.Contains(t, dispatch, tool)
	}
	_, ok := integration.CompactSpec("unknown")
	require.False(t, ok)
}

func TestBoundedProjection(t *testing.T) {
	cases := []struct {
		name    string
		tool    mcp.ToolName
		payload string
		args    map[string]any
		keep    string
		drop    string
	}{
		{"targets", "prometheus_list_targets", `{"activeTargets":[{"health":"down","scrapePool":"api","scrapeUrl":"SECRET_URL","discoveredLabels":{"secret":"PRIVATE"}},{"health":"up"}],"droppedTargets":[]}`, map[string]any{"limit": 1}, "down", "SECRET_URL"},
		{"rules", "prometheus_list_rules", `{"groups":[{"name":"api","file":"PRIVATE_PATH","rules":[{"name":"Down","query":"up == 0","alerts":[{"secret":"PRIVATE"}]},{"name":"Extra"}]}],"groupNextToken":"continue"}`, map[string]any{"limit": 1}, "continue", "PRIVATE"},
		{"values", "prometheus_list_label_values", `["api","extra"]`, map[string]any{"label": "job", "limit": 1}, "api", "extra"},
		{"series", "prometheus_list_series", `[{"job":"api"},{"job":"extra"}]`, map[string]any{"match": []string{"up"}, "limit": 1}, "api", "extra"},
		{"query", "prometheus_query", `{"resultType":"vector","result":[{"metric":{"job":"api"},"value":[100,"1"]},{"metric":{"job":"extra"},"value":[100,"0"]}]}`, map[string]any{"query": "up", "limit": 1}, "api", "extra"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			integration := configured(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprintf(w, `{"status":"success","data":%s}`, item.payload)
			})
			result, err := integration.Execute(context.Background(), item.tool, item.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			require.Contains(t, result.Data, item.keep)
			require.NotContains(t, result.Data, item.drop)
			require.Contains(t, result.Data, `"truncated":true`)
		})
	}
}

func TestQuerySampleTypes(t *testing.T) {
	cases := []struct {
		name     string
		payload  string
		expected string
	}{
		{"scalar", `{"resultType":"scalar","result":[100,"NaN"]}`, "NaN"},
		{"string", `{"resultType":"string","result":[100,"hello"]}`, "hello"},
		{"histogram", `{"resultType":"vector","result":[{"metric":{"job":"api"},"histogram":[100,{"count":"2","sum":"3","buckets":[]}]}]}`, "histogram"},
		{"matrix_histogram", `{"resultType":"matrix","result":[{"metric":{"job":"api"},"histograms":[[100,{"count":"2","sum":"3","buckets":[]}]]}]}`, "histograms"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			integration := configured(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprintf(w, `{"status":"success","data":%s}`, item.payload)
			})
			result, err := integration.Execute(context.Background(), "prometheus_query", map[string]any{"query": "up"})
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			require.Contains(t, result.Data, item.expected)
		})
	}
}

func TestTLSRequiresTrustedCA(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","data":{"version":"3.0"}}`))
	}))
	defer srv.Close()
	integration := New()
	require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL}))
	require.False(t, integration.Healthy(context.Background()))
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL, "ca_cert": string(certificate)}))
	require.True(t, integration.Healthy(context.Background()))
}

func TestRangeResponseBudget(t *testing.T) {
	integration := configured(t, func(w http.ResponseWriter, _ *http.Request) {
		values := make([][2]any, 5001)
		for index := range values {
			values[index] = [2]any{index, "1"}
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"resultType": "matrix", "result": []any{map[string]any{"metric": map[string]string{"job": "api"}, "values": values}}}}))
	})
	result, err := integration.Execute(context.Background(), "prometheus_query_range", map[string]any{"query": "up", "start": "100", "end": "160", "step": "30"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Data, "sample")
}
