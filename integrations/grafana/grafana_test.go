package grafana

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testToken = "glsa_test_token"

func newTestClient(t *testing.T, handler http.HandlerFunc) *grafana {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+testToken {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"message":"Invalid API key","statusCode":401}`)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(ts.Close)
	g := New().(*grafana)
	require.NoError(t, g.Configure(context.Background(), mcp.Credentials{"base_url": ts.URL + "/", "service_account_token": testToken}))
	return g
}

// decodeBody reads a JSON request body into a map.
func decodeBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
	return body
}

func execute(t *testing.T, g *grafana, tool mcp.ToolName, args map[string]any) *mcp.ToolResult {
	t.Helper()
	result, err := g.Execute(context.Background(), tool, args)
	require.NoError(t, err)
	return result
}

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "grafana", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	g := New().(*grafana)
	require.NoError(t, g.Configure(context.Background(), mcp.Credentials{"base_url": " https://acme.grafana.net/ ", "service_account_token": " glsa_x "}))
	assert.Equal(t, "https://acme.grafana.net", g.baseURL)
	assert.Equal(t, "glsa_x", g.token)
}

func TestConfigure_KeepsSubpath(t *testing.T) {
	g := New().(*grafana)
	require.NoError(t, g.Configure(context.Background(), mcp.Credentials{"base_url": "http://homelab.lan:3000/grafana/", "service_account_token": "t"}))
	assert.Equal(t, "http://homelab.lan:3000/grafana", g.baseURL)
}

func TestConfigure_Rejects(t *testing.T) {
	tests := []struct {
		name  string
		creds mcp.Credentials
		want  string
	}{
		{"missing base_url", mcp.Credentials{"service_account_token": "t"}, "base_url is required"},
		{"missing token", mcp.Credentials{"base_url": "https://acme.grafana.net"}, "service_account_token is required"},
		{"no scheme", mcp.Credentials{"base_url": "acme.grafana.net", "service_account_token": "t"}, "base_url must be an http or https URL"},
		{"other scheme", mcp.Credentials{"base_url": "ftp://acme.grafana.net", "service_account_token": "t"}, "base_url must be an http or https URL"},
		{"userinfo", mcp.Credentials{"base_url": "https://u:p@acme.grafana.net", "service_account_token": "t"}, "must not contain a user name, query or fragment"},
		{"query", mcp.Credentials{"base_url": "https://acme.grafana.net/?orgId=1", "service_account_token": "t"}, "must not contain a user name, query or fragment"},
		{"token newline", mcp.Credentials{"base_url": "https://acme.grafana.net", "service_account_token": "a\nb"}, "service_account_token is invalid"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := New().(*grafana)
			err := g.Configure(context.Background(), tc.creds)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			assert.Nil(t, g.client, "a rejected configuration must leave the adapter unconfigured")
		})
	}
}

func TestTools_Metadata(t *testing.T) {
	seen := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
		assert.True(t, strings.HasPrefix(string(tool.Name), "grafana_"), "tool %s missing grafana_ prefix", tool.Name)
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		assert.True(t, tool.Access.Declared(), "tool %s has no access class", tool.Name)
		seen[tool.Name] = true
	}
}

func TestTools_EntryPointSaysStartHere(t *testing.T) {
	for _, tool := range New().Tools() {
		if tool.Name == "grafana_search_dashboards" {
			assert.Contains(t, tool.Description, "Start here")
			return
		}
	}
	t.Fatal("grafana_search_dashboards not defined")
}

func TestTools_OnlySaveDashboardWrites(t *testing.T) {
	for _, tool := range New().Tools() {
		if tool.Name == "grafana_save_dashboard" {
			assert.Equal(t, mcp.AccessWrite, tool.Access)
			assert.Contains(t, tool.Description, "WRITE:")
		} else {
			assert.Equal(t, mcp.AccessRead, tool.Access, "%s must be a read", tool.Name)
		}
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	g := New().(*grafana)
	require.NoError(t, g.Configure(context.Background(), mcp.Credentials{"base_url": "https://x.test", "service_account_token": "t"}))
	result := execute(t, g, "grafana_nonexistent", nil)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestExecute_NotConfigured(t *testing.T) {
	result, err := New().Execute(context.Background(), "grafana_list_datasources", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "not configured")
}

func TestHealthy(t *testing.T) {
	assert.False(t, New().(*grafana).Healthy(context.Background()), "unconfigured adapter is not healthy")
	g := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/org", r.URL.Path)
		_, _ = io.WriteString(w, `{"id":1,"name":"Main Org."}`)
	})
	assert.True(t, g.Healthy(context.Background()))
	g.token = "wrong"
	assert.False(t, g.Healthy(context.Background()))
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

func TestSearchDashboards(t *testing.T) {
	g := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/search", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "dash-db", q.Get("type"))
		assert.Equal(t, "service", q.Get("query"))
		assert.Equal(t, []string{"prod", "api"}, q["tag"])
		assert.Equal(t, []string{"ops"}, q["folderUIDs"])
		assert.Equal(t, "true", q.Get("starred"))
		assert.Equal(t, "10", q.Get("limit"))
		assert.Equal(t, "2", q.Get("page"))
		_, _ = io.WriteString(w, `[{"uid":"svc","title":"Service Overview","url":"/d/svc/service-overview","folderUid":"ops","tags":["prod"]}]`)
	})
	result := execute(t, g, "grafana_search_dashboards", map[string]any{
		"query": "service", "tags": []any{"prod", "api"}, "folder_uids": []any{"ops"}, "starred": true, "limit": 10, "page": 2,
	})
	require.False(t, result.IsError, result.Data)
	assert.Contains(t, result.Data, `"uid":"svc"`)
}

func TestSearchDashboards_DefaultLimit(t *testing.T) {
	g := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, "50", q.Get("limit"))
		assert.False(t, q.Has("page"))
		assert.False(t, q.Has("starred"))
		_, _ = io.WriteString(w, `[]`)
	})
	result := execute(t, g, "grafana_search_dashboards", nil)
	require.False(t, result.IsError, result.Data)
	assert.Equal(t, "[]", result.Data)
}

func TestSearchDashboards_BadLimit(t *testing.T) {
	g := newTestClient(t, func(http.ResponseWriter, *http.Request) { t.Fatal("must not call Grafana") })
	result := execute(t, g, "grafana_search_dashboards", map[string]any{"limit": 9000})
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "limit must be between 1 and 5000")
}

func TestGetDashboard(t *testing.T) {
	g := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/dashboards/uid/a%2Fb", r.URL.EscapedPath())
		_, _ = io.WriteString(w, `{"meta":{"folderUid":"ops","version":3},"dashboard":{"uid":"a/b","title":"T","id":622268405960704}}`)
	})
	result := execute(t, g, "grafana_get_dashboard", map[string]any{"uid": "a/b"})
	require.False(t, result.IsError, result.Data)
	assert.Contains(t, result.Data, `"id":622268405960704`, "large ids must survive the round trip unchanged")
}

func TestGetDashboard_RequiresUID(t *testing.T) {
	g := newTestClient(t, func(http.ResponseWriter, *http.Request) { t.Fatal("must not call Grafana") })
	result := execute(t, g, "grafana_get_dashboard", map[string]any{"uid": " "})
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "uid is required")
}

func TestGetDashboard_NotFound(t *testing.T) {
	g := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"Dashboard not found","title":"Not found"}`)
	})
	result := execute(t, g, "grafana_get_dashboard", map[string]any{"uid": "nope"})
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "grafana API error (404): Dashboard not found")
}

func TestSaveDashboard_Variants(t *testing.T) {
	tests := []struct {
		name      string
		args      map[string]any
		wantDash  map[string]any
		wantExtra map[string]any
	}{
		{
			name:      "object with folder, overwrite and message",
			args:      map[string]any{"dashboard": map[string]any{"uid": "svc", "title": "S", "id": float64(7)}, "folder_uid": "ops", "overwrite": true, "message": "tidy"},
			wantDash:  map[string]any{"uid": "svc", "title": "S", "id": nil},
			wantExtra: map[string]any{"folderUid": "ops", "overwrite": true, "message": "tidy"},
		},
		{
			name:      "JSON string",
			args:      map[string]any{"dashboard": `{"title":"New"}`},
			wantDash:  map[string]any{"title": "New"},
			wantExtra: map[string]any{"overwrite": false},
		},
		{
			name:      "get_dashboard result is unwrapped",
			args:      map[string]any{"dashboard": map[string]any{"meta": map[string]any{"version": float64(3)}, "dashboard": map[string]any{"uid": "svc", "title": "S", "version": float64(3)}}},
			wantDash:  map[string]any{"uid": "svc", "title": "S", "version": float64(3)},
			wantExtra: map[string]any{"overwrite": false},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/api/dashboards/db", r.URL.Path)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				body := decodeBody(t, r)
				dash, ok := body["dashboard"].(map[string]any)
				require.True(t, ok)
				for k, v := range tc.wantDash {
					if v == nil {
						assert.NotContains(t, dash, k)
					} else {
						assert.Equal(t, v, dash[k], k)
					}
				}
				assert.NotContains(t, dash, "meta")
				for k, v := range tc.wantExtra {
					assert.Equal(t, v, body[k], k)
				}
				_, _ = io.WriteString(w, `{"status":"success","uid":"svc","url":"/d/svc/s","version":4}`)
			})
			result := execute(t, g, "grafana_save_dashboard", tc.args)
			require.False(t, result.IsError, result.Data)
			assert.Contains(t, result.Data, `"status":"success"`)
		})
	}
}

func TestSaveDashboard_Import(t *testing.T) {
	g := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/dashboards/import", r.URL.Path)
		body := decodeBody(t, r)
		assert.Equal(t, "ops", body["folderUid"])
		assert.Equal(t, true, body["overwrite"])
		assert.NotContains(t, body, "message", "the import API has no version message")
		assert.Equal(t, []any{
			map[string]any{"name": "DS_PROMETHEUS", "type": "datasource", "pluginId": "prometheus", "value": "prom-uid"},
		}, body["inputs"])
		_, _ = io.WriteString(w, `{"uid":"rb","imported":true,"importedUrl":"/d/rb/x"}`)
	})
	dash := `{"__inputs":[{"name":"DS_PROMETHEUS","type":"datasource","pluginId":"prometheus"}],"uid":"rb","title":"RB"}`
	result := execute(t, g, "grafana_save_dashboard", map[string]any{
		"dashboard": dash, "folder_uid": "ops", "overwrite": true, "inputs": map[string]any{"DS_PROMETHEUS": "prom-uid"},
	})
	require.False(t, result.IsError, result.Data)
	assert.Contains(t, result.Data, `"imported":true`)
}

func TestSaveDashboard_Rejects(t *testing.T) {
	inputsDash := map[string]any{"__inputs": []any{
		map[string]any{"name": "DS_PROMETHEUS", "type": "datasource", "pluginId": "prometheus"},
		map[string]any{"name": "DS_LOKI", "type": "datasource", "pluginId": "loki"},
	}, "title": "RB"}
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{"missing dashboard", map[string]any{}, "dashboard is required"},
		{"invalid JSON string", map[string]any{"dashboard": "{nope"}, "dashboard must be a JSON object"},
		{"array", map[string]any{"dashboard": []any{}}, "dashboard must be a JSON object"},
		{"no title", map[string]any{"dashboard": map[string]any{"uid": "x"}}, "dashboard needs a title"},
		{"exported without inputs", map[string]any{"dashboard": inputsDash}, "pass inputs with a value for DS_LOKI, DS_PROMETHEUS"},
		{"missing one input", map[string]any{"dashboard": inputsDash, "inputs": map[string]any{"DS_PROMETHEUS": "p"}}, "pass inputs with a value for DS_LOKI"},
		{"unknown input", map[string]any{"dashboard": inputsDash, "inputs": map[string]any{"DS_PROMETHEUS": "p", "DS_LOKI": "l", "DS_X": "x"}}, "DS_X is not one of the dashboard's __inputs"},
		{"inputs without __inputs", map[string]any{"dashboard": map[string]any{"title": "T"}, "inputs": map[string]any{"DS_X": "x"}}, "DS_X is not one of the dashboard's __inputs"},
		{"non-string input", map[string]any{"dashboard": inputsDash, "inputs": map[string]any{"DS_PROMETHEUS": 1, "DS_LOKI": "l"}}, "input DS_PROMETHEUS must be a string"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := newTestClient(t, func(http.ResponseWriter, *http.Request) { t.Fatal("must not call Grafana") })
			result := execute(t, g, "grafana_save_dashboard", tc.args)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tc.want)
		})
	}
}

func TestSaveDashboard_VersionConflictIsNotRetried(t *testing.T) {
	calls := 0
	g := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusPreconditionFailed)
		_, _ = io.WriteString(w, `{"message":"The dashboard has been changed by someone else","status":"version-mismatch"}`)
	})
	result := execute(t, g, "grafana_save_dashboard", map[string]any{"dashboard": map[string]any{"uid": "svc", "title": "S", "version": 1}})
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "grafana API error (412): The dashboard has been changed by someone else")
	assert.Equal(t, 1, calls)
}

func TestSaveDashboard_ServerErrorIsNotRetryable(t *testing.T) {
	g := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	_, err := g.doRequest(context.Background(), http.MethodPost, "/api/dashboards/db", nil, map[string]any{}, false)
	require.Error(t, err)
	assert.False(t, mcp.IsRetryable(err), "a failed save may already have been stored")
}

func TestListFolders(t *testing.T) {
	g := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/folders", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "100", q.Get("limit"))
		assert.Equal(t, "ops", q.Get("parentUid"))
		_, _ = io.WriteString(w, `[{"id":3,"uid":"k8s","title":"Kubernetes"}]`)
	})
	result := execute(t, g, "grafana_list_folders", map[string]any{"parent_uid": "ops"})
	require.False(t, result.IsError, result.Data)
	assert.Contains(t, result.Data, "Kubernetes")
}

func TestListDatasources(t *testing.T) {
	g := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/datasources", r.URL.Path)
		_, _ = io.WriteString(w, `[{"uid":"prom","name":"Prometheus","type":"prometheus","isDefault":true}]`)
	})
	result := execute(t, g, "grafana_list_datasources", nil)
	require.False(t, result.IsError, result.Data)
	assert.Contains(t, result.Data, `"uid":"prom"`)
}

func TestQueryDatasource_Body(t *testing.T) {
	tests := []struct {
		name      string
		args      map[string]any
		wantTop   map[string]any
		wantQuery map[string]any
	}{
		{
			name:      "range PromQL with defaults",
			args:      map[string]any{"datasource_uid": "prom", "expr": "up"},
			wantTop:   map[string]any{"from": "now-1h", "to": "now"},
			wantQuery: map[string]any{"refId": "A", "datasource": map[string]any{"uid": "prom"}, "expr": "up", "instant": false, "range": true, "maxDataPoints": float64(300), "intervalMs": float64(12000)},
		},
		{
			name:      "instant with explicit window and step",
			args:      map[string]any{"datasource_uid": "prom", "expr": "up", "instant": true, "from": "now-5m", "to": "now-1m", "max_data_points": 50, "interval_ms": 15000},
			wantTop:   map[string]any{"from": "now-5m", "to": "now-1m"},
			wantQuery: map[string]any{"instant": true, "range": false, "maxDataPoints": float64(50), "intervalMs": float64(15000)},
		},
		{
			name:      "extra query fields merge but cannot replace refId or datasource",
			args:      map[string]any{"datasource_uid": "pg", "query": map[string]any{"rawSql": "select 1", "format": "table", "refId": "Z", "datasource": map[string]any{"uid": "other"}}},
			wantQuery: map[string]any{"refId": "A", "datasource": map[string]any{"uid": "pg"}, "rawSql": "select 1", "format": "table"},
		},
		{
			name:      "extra query fields as JSON string",
			args:      map[string]any{"datasource_uid": "loki", "expr": `{app="api"}`, "query": `{"queryType":"range"}`},
			wantQuery: map[string]any{"expr": `{app="api"}`, "queryType": "range"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/api/ds/query", r.URL.Path)
				body := decodeBody(t, r)
				for k, v := range tc.wantTop {
					assert.Equal(t, v, body[k], k)
				}
				queries, ok := body["queries"].([]any)
				require.True(t, ok)
				require.Len(t, queries, 1)
				q := queries[0].(map[string]any)
				for k, v := range tc.wantQuery {
					assert.Equal(t, v, q[k], k)
				}
				_, _ = io.WriteString(w, `{"results":{"A":{"status":200,"frames":[]}}}`)
			})
			result := execute(t, g, "grafana_query_datasource", tc.args)
			require.False(t, result.IsError, result.Data)
		})
	}
}

// Grafana's /api/ds/query does not derive the step from maxDataPoints (its
// frontend does), so the adapter spreads max_data_points over the range.
func TestQueryInterval(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		from, to string
		points   int
		want     int
	}{
		{"now-1h", "now", 300, 12000},
		{"now-6h", "now-1h", 100, 180000},
		{"now-30d", "now", 2000, 1296000},
		{"now-2w", "now", 1000, 1209600},
		{"now-90s", "now", 1000, 1000}, // never below one second
		{"1794700000000", "1794703600000", 60, 60000},
		{"2026-11-15T11:00:00Z", "2026-11-15T12:00:00Z", 360, 10000},
		{"now/d", "now", 300, 0},  // rounding is left to Grafana
		{"now", "now-1h", 300, 0}, // reversed range is left to Grafana
		{"yesterday", "now", 300, 0},
	}
	for _, tc := range tests {
		t.Run(tc.from+".."+tc.to, func(t *testing.T) {
			assert.Equal(t, tc.want, queryInterval(tc.from, tc.to, tc.points, now))
		})
	}
}

func TestQueryDatasource_ResultsBecomeList(t *testing.T) {
	g := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"results":{"A":{"status":200,"frames":[{"schema":{"refId":"A","fields":[{"name":"Time","type":"time"},{"name":"Value","type":"number","labels":{"job":"api"}}]},"data":{"values":[[1700000000000],[1]]}}]}}}`)
	})
	result := execute(t, g, "grafana_query_datasource", map[string]any{"datasource_uid": "prom", "expr": "up"})
	require.False(t, result.IsError, result.Data)
	var out []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &out))
	require.Len(t, out, 1)
	assert.Equal(t, "A", out[0]["ref_id"])
	assert.Contains(t, result.Data, `"job":"api"`)
	assert.Contains(t, result.Data, `1700000000000`)
}

func TestQueryDatasource_QueryErrorIsReported(t *testing.T) {
	g := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"results":{"A":{"error":"bad_data: parse error at char 3","errorSource":"downstream","status":400}}}`)
	})
	result := execute(t, g, "grafana_query_datasource", map[string]any{"datasource_uid": "prom", "expr": "up{"})
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "bad_data: parse error at char 3")
}

func TestQueryDatasource_Rejects(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{"no datasource", map[string]any{"expr": "up"}, "datasource_uid is required"},
		{"no expr or query", map[string]any{"datasource_uid": "prom"}, "expr or query is required"},
		{"bad query JSON", map[string]any{"datasource_uid": "prom", "query": "{x"}, "query must be a JSON object"},
		{"too many points", map[string]any{"datasource_uid": "prom", "expr": "up", "max_data_points": 5000}, "max_data_points must be between 1 and 2000"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := newTestClient(t, func(http.ResponseWriter, *http.Request) { t.Fatal("must not call Grafana") })
			result := execute(t, g, "grafana_query_datasource", tc.args)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tc.want)
		})
	}
}

func TestQueryDatasource_ServerErrorIsRetryable(t *testing.T) {
	g := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	_, err := g.doRequest(context.Background(), http.MethodPost, "/api/ds/query", nil, map[string]any{}, true)
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err), "a query only reads, so a 5xx can be retried")
}

func TestGetHealth(t *testing.T) {
	g := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			_, _ = io.WriteString(w, `{"database":"ok","version":"13.3.1","commit":"abc"}`)
		case "/api/org":
			_, _ = io.WriteString(w, `{"id":1,"name":"Main Org."}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	result := execute(t, g, "grafana_get_health", nil)
	require.False(t, result.IsError, result.Data)
	assert.Contains(t, result.Data, `"version":"13.3.1"`)
	assert.Contains(t, result.Data, `"name":"Main Org."`)
}

func TestErrors_AuthenticationFailure(t *testing.T) {
	g := newTestClient(t, nil)
	g.token = "wrong"
	result := execute(t, g, "grafana_list_datasources", nil)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "grafana API error (401): Invalid API key")
	assert.Contains(t, result.Data, "service account token")
}

// A Grafana Cloud free stack that is asleep answers with a redirect or an
// HTML page instead of the API. Both must explain what to do.
func TestErrors_SleepingStack(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"redirect", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://grafana.com/auth/sign-in?wake=acme", http.StatusFound)
		}},
		{"HTML page", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, `<!doctype html><html><body>Your stack is waking up</body></html>`)
		}},
		{"HTML error page", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `<html><body>paused</body></html>`)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := newTestClient(t, tc.handler)
			result := execute(t, g, "grafana_list_datasources", nil)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, "asleep")
			assert.Contains(t, result.Data, "open the stack in a browser")
			assert.NotContains(t, result.Data, testToken)
		})
	}
}

func TestErrors_RedirectIsNotFollowed(t *testing.T) {
	var leaked bool
	other := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		leaked = r.Header.Get("Authorization") != ""
	}))
	t.Cleanup(other.Close)
	g := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/api/datasources", http.StatusTemporaryRedirect)
	})
	result := execute(t, g, "grafana_list_datasources", nil)
	assert.True(t, result.IsError)
	assert.False(t, leaked, "the token must never follow a redirect")
}

func TestErrors_OversizedResponse(t *testing.T) {
	g := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `["`+strings.Repeat("x", maxResponseSize)+`"]`)
	})
	result := execute(t, g, "grafana_list_datasources", nil)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "response is larger than")
}

func TestErrors_RateLimitIsRetryable(t *testing.T) {
	g := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	_, err := g.doRequest(context.Background(), http.MethodPost, "/api/dashboards/db", nil, map[string]any{}, false)
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err), "a rate-limited request was not processed, so even a save can retry")
}
