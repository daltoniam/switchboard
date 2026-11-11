package influxdb

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

const testOrg = "0123456789abcdef"
const testTask = "1111111111111111"
const testRun = "2222222222222222"

func fixture(t *testing.T, handler http.HandlerFunc) (*influxdb, *httptest.Server) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	adapter := New().(*influxdb)
	ca := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials{"base_url": server.URL, "api_token": "fixture-token", "org_id": testOrg, "ca_cert": ca}))
	t.Cleanup(adapter.client.CloseIdleConnections)
	return adapter, server
}

func execute(t *testing.T, adapter *influxdb, tool mcp.ToolName, args map[string]any) *mcp.ToolResult {
	t.Helper()
	result, err := adapter.Execute(t.Context(), tool, args)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func TestConfigure(t *testing.T) {
	valid := mcp.Credentials{"base_url": "https://influx.example/prefix", "api_token": "token", "org_id": testOrg}
	tests := []struct{ name, key, value string }{
		{"missing URL", "base_url", ""}, {"HTTP", "base_url", "http://localhost:8086"}, {"userinfo", "base_url", "https://user:secret@influx.example"}, {"query", "base_url", "https://influx.example?x=y"}, {"fragment", "base_url", "https://influx.example/#x"}, {"missing token", "api_token", ""}, {"header injection", "api_token", "token\nsecret"}, {"missing org", "org_id", ""}, {"invalid org", "org_id", "../org"}, {"invalid CA", "ca_cert", "not PEM"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			creds := mcp.Credentials{}
			for key, value := range valid {
				creds[key] = value
			}
			creds[test.key] = test.value
			require.Error(t, New().Configure(t.Context(), creds))
		})
	}
	require.NoError(t, New().Configure(t.Context(), valid))
	adapter := New().(*influxdb)
	require.False(t, adapter.Healthy(t.Context()))
	require.True(t, execute(t, adapter, "influxdb_list_tasks", nil).IsError)
	require.True(t, execute(t, adapter, "unknown", nil).IsError)
	require.Equal(t, "influxdb", adapter.Name())
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range New().Tools() {
		t.Run(string(tool.Name), func(t *testing.T) {
			require.Contains(t, dispatch, tool.Name)
			require.NotEmpty(t, tool.Description)
			require.True(t, strings.HasPrefix(string(tool.Name), "influxdb_"))
			require.Equal(t, mcp.AccessRead, tool.Access)
		})
	}
	require.Len(t, New().Tools(), 5)
	require.Contains(t, New().Tools()[0].Description, "Start here")
}
func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	names := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		require.False(t, names[tool.Name])
		names[tool.Name] = true
	}
	for name := range dispatch {
		require.True(t, names[name], string(name))
	}
}

func TestHealthy(t *testing.T) {
	for _, version := range []string{"v2.8.0", "2.8.1", "v2.7.12", "v3.0.0", ""} {
		t.Run(version, func(t *testing.T) {
			adapter, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "GET", r.Method)
				require.Equal(t, "Token fixture-token", r.Header.Get("Authorization"))
				if r.URL.Path == "/health" {
					_ = json.NewEncoder(w).Encode(map[string]string{"status": "pass", "version": version})
					return
				}
				require.Equal(t, "/api/v2/tasks", r.URL.Path)
				require.Equal(t, testOrg, r.URL.Query().Get("orgID"))
				fmt.Fprint(w, `{"tasks":[]}`)
			})
			require.Equal(t, version == "v2.8.0" || version == "2.8.1", adapter.Healthy(t.Context()))
		})
	}
}

func TestReadTools(t *testing.T) {
	task := `{"id":"` + testTask + `","orgID":"` + testOrg + `","name":"downsample","status":"active","every":"1h","latestCompleted":"2026-11-10T01:00:00Z","flux":"SECRET_SENTINEL","lastRunError":"SECRET_SENTINEL","authorizationID":"SECRET_SENTINEL","labels":["SECRET_SENTINEL"]}`
	run := `{"id":"` + testRun + `","taskID":"` + testTask + `","status":"failed","scheduledFor":"2026-11-10T01:00:00Z","log":["SECRET_SENTINEL"],"links":{"self":"SECRET_SENTINEL"}}`
	tests := []struct {
		name             string
		tool             mcp.ToolName
		args             map[string]any
		path, body, want string
	}{
		{"buckets", "influxdb_list_buckets", nil, "/api/v2/buckets", `{"buckets":[{"id":"3333333333333333","orgID":"` + testOrg + `","name":"metrics","retentionRules":[{"type":"expire","everySeconds":86400,"shardGroupDurationSeconds":3600}],"description":"SECRET_SENTINEL"}]}`, "metrics"},
		{"tasks", "influxdb_list_tasks", map[string]any{"name": "downsample", "status": "active"}, "/api/v2/tasks", `{"tasks":[` + task + `]}`, "downsample"},
		{"task", "influxdb_get_task", map[string]any{"task_id": testTask}, "/api/v2/tasks/" + testTask, task, "downsample"},
		{"runs", "influxdb_list_task_runs", map[string]any{"task_id": testTask, "status": "failed"}, "/api/v2/tasks/" + testTask + "/runs", `{"runs":[` + run + `]}`, "failed"},
		{"run", "influxdb_get_task_run", map[string]any{"task_id": testTask, "run_id": testRun}, "/api/v2/tasks/" + testTask + "/runs/" + testRun, run, "failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapter, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "Token fixture-token", r.Header.Get("Authorization"))
				require.Equal(t, "application/json", r.Header.Get("Accept"))
				if r.URL.Path == "/api/v2/tasks/"+testTask && test.tool != "influxdb_get_task" {
					fmt.Fprint(w, task)
					return
				}
				require.Equal(t, test.path, r.URL.Path)
				query := r.URL.Query()
				switch test.tool {
				case "influxdb_list_buckets":
					require.Equal(t, testOrg, query.Get("orgID"))
					require.Equal(t, "50", query.Get("limit"))
					require.Equal(t, "0", query.Get("offset"))
				case "influxdb_list_tasks":
					require.Equal(t, "basic", query.Get("type"))
					require.Equal(t, testOrg, query.Get("orgID"))
					require.Equal(t, "downsample", query.Get("name"))
				case "influxdb_list_task_runs":
					require.Empty(t, query.Get("status"))
					after, err := time.Parse(time.RFC3339, query.Get("afterTime"))
					require.NoError(t, err)
					before, err := time.Parse(time.RFC3339, query.Get("beforeTime"))
					require.NoError(t, err)
					require.Equal(t, 24*time.Hour, before.Sub(after))
				}
				fmt.Fprint(w, test.body)
			})
			result := execute(t, adapter, test.tool, test.args)
			require.False(t, result.IsError, result.Data)
			require.Contains(t, result.Data, test.want)
			require.NotContains(t, result.Data, "SECRET_SENTINEL")
			fields, ok := adapter.CompactSpec(test.tool)
			require.True(t, ok)
			var raw any
			require.NoError(t, json.Unmarshal([]byte(result.Data), &raw))
			projected := mcp.CompactAny(raw, fields)
			encoded, err := json.Marshal(projected)
			require.NoError(t, err)
			require.Contains(t, string(encoded), test.want)
			require.NotContains(t, string(encoded), "SECRET_SENTINEL")
		})
	}
}

func TestInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		tool mcp.ToolName
		args map[string]any
	}{
		{"limit zero", "influxdb_list_tasks", map[string]any{"limit": 0}}, {"limit large", "influxdb_list_tasks", map[string]any{"limit": 101}}, {"limit type", "influxdb_list_tasks", map[string]any{"limit": true}}, {"offset negative", "influxdb_list_buckets", map[string]any{"offset": -1}}, {"bad cursor", "influxdb_list_tasks", map[string]any{"after": "../x"}}, {"bad status", "influxdb_list_tasks", map[string]any{"status": "failed"}}, {"missing task", "influxdb_get_task", nil}, {"bad task", "influxdb_get_task", map[string]any{"task_id": "../x"}}, {"missing run", "influxdb_get_task_run", map[string]any{"task_id": testTask}}, {"invalid time", "influxdb_list_task_runs", map[string]any{"task_id": testTask, "after_time": "yesterday"}}, {"wide window", "influxdb_list_task_runs", map[string]any{"task_id": testTask, "after_time": "2026-01-01T00:00:00Z", "before_time": "2026-02-01T00:00:00Z"}}, {"reverse window", "influxdb_list_task_runs", map[string]any{"task_id": testTask, "after_time": "2026-02-01T00:00:00Z", "before_time": "2026-01-01T00:00:00Z"}}, {"unknown argument", "influxdb_list_tasks", map[string]any{"org_id": "other"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapter, _ := fixture(t, func(http.ResponseWriter, *http.Request) { t.Error("invalid arguments must not reach upstream") })
			require.True(t, execute(t, adapter, test.tool, test.args).IsError)
		})
	}
}

func TestOrgBoundary(t *testing.T) {
	for _, org := range []string{"", "ffffffffffffffff"} {
		t.Run(org, func(t *testing.T) {
			var requests atomic.Int32
			adapter, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				require.Equal(t, "/api/v2/tasks/"+testTask, r.URL.Path)
				fmt.Fprintf(w, `{"id":"%s","orgID":"%s"}`, testTask, org)
			})
			result := execute(t, adapter, "influxdb_list_task_runs", map[string]any{"task_id": testTask})
			require.True(t, result.IsError)
			require.Contains(t, result.Data, "org")
			require.Equal(t, int32(1), requests.Load())
		})
	}
}

func TestHTTPFailures(t *testing.T) {
	for _, status := range []int{301, 401, 403, 404, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			adapter, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "https://other.example/secret")
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(status)
				fmt.Fprint(w, "SECRET_SENTINEL")
			})
			result := execute(t, adapter, "influxdb_list_tasks", nil)
			require.True(t, result.IsError)
			require.Contains(t, result.Data, fmt.Sprint(status))
			require.NotContains(t, result.Data, "SECRET_SENTINEL")
		})
	}
}

func TestMalformedResponses(t *testing.T) {
	for _, body := range []string{`not json`, `{}null`, `null`, `{}`, `{"tasks":null}`, `{"tasks":[{"orgID":"other"}]}`, strings.Repeat(" ", maxResponseBytes+1)} {
		t.Run(fmt.Sprint(len(body)), func(t *testing.T) {
			adapter, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) })
			require.True(t, execute(t, adapter, "influxdb_list_tasks", nil).IsError)
		})
	}
}

func TestPaginationAndLocalFilter(t *testing.T) {
	adapter, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/tasks/"+testTask {
			fmt.Fprintf(w, `{"id":"%s","orgID":"%s"}`, testTask, testOrg)
			return
		}
		fmt.Fprintf(w, `{"runs":[{"id":"%s","taskID":"%s","status":"success"}]}`, testRun, testTask)
	})
	result := execute(t, adapter, "influxdb_list_task_runs", map[string]any{"task_id": testTask, "limit": 1, "status": "failed"})
	require.False(t, result.IsError, result.Data)
	var page map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &page))
	require.Equal(t, []any{}, page["runs"])
	require.Equal(t, testRun, page["next_after"])
	require.Equal(t, true, page["has_more"])
}

func TestDeadline(t *testing.T) {
	adapter, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	result, err := adapter.Execute(ctx, "influxdb_list_tasks", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestRetryAndPrefix(t *testing.T) {
	var attempts atomic.Int32
	adapter, server := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/influx/api/v2/tasks", r.URL.Path)
		require.Equal(t, testTask, r.URL.Query().Get("after"))
		if attempts.Add(1) < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, `{"tasks":[],"links":{"next":"https://untrusted.example/SECRET_SENTINEL"}}`)
	})
	adapter.baseURL = server.URL + "/influx"
	result := execute(t, adapter, "influxdb_list_tasks", map[string]any{"after": testTask})
	require.False(t, result.IsError, result.Data)
	require.Equal(t, int32(3), attempts.Load())
	require.NotContains(t, result.Data, "SECRET_SENTINEL")
}

func TestLongRetryAfterDoesNotSleep(t *testing.T) {
	var attempts atomic.Int32
	adapter, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	started := time.Now()
	require.True(t, execute(t, adapter, "influxdb_list_tasks", nil).IsError)
	require.Less(t, time.Since(started), time.Second)
	require.Equal(t, int32(1), attempts.Load())
}

func TestRedirectDoesNotForwardCredentials(t *testing.T) {
	var forwarded atomic.Int32
	destination := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded.Add(1) }))
	t.Cleanup(destination.Close)
	adapter, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	})
	require.True(t, execute(t, adapter, "influxdb_list_tasks", nil).IsError)
	require.Zero(t, forwarded.Load())
}

func TestReconfigureClearsInvalidState(t *testing.T) {
	adapter, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid reconfiguration must disable requests") })
	require.Error(t, adapter.Configure(t.Context(), mcp.Credentials{}))
	require.False(t, adapter.Healthy(t.Context()))
	require.True(t, execute(t, adapter, "influxdb_list_tasks", nil).IsError)
}

func TestFractionalPagination(t *testing.T) {
	for _, args := range []map[string]any{{"limit": 1.5}, {"offset": 1.5}} {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			adapter, _ := fixture(t, func(http.ResponseWriter, *http.Request) { t.Error("fractional pagination reached upstream") })
			require.True(t, execute(t, adapter, "influxdb_list_buckets", args).IsError)
		})
	}
}

func TestAbsentFieldsRemainNull(t *testing.T) {
	adapter, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"id":"%s","orgID":"%s"}`, testTask, testOrg)
	})
	result := execute(t, adapter, "influxdb_get_task", map[string]any{"task_id": testTask})
	require.False(t, result.IsError)
	var value map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &value))
	require.Contains(t, value, "status")
	require.Nil(t, value["status"])
	require.Contains(t, value, "cron")
	require.Nil(t, value["cron"])
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	require.Len(t, fieldCompactionSpecs, 5)
	for name := range fieldCompactionSpecs {
		require.Contains(t, dispatch, name)
	}
}
