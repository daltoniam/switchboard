package panther

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testToken = "panther-test-token"

// recorded is one request the fake Panther API received.
type recorded struct {
	Method string
	Path   string
	Query  url.Values
	Body   map[string]any
	Key    string
}

type fakePanther struct {
	mu       sync.Mutex
	requests []recorded
}

func (f *fakePanther) record(r *http.Request) recorded {
	rec := recorded{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Key: r.Header.Get("X-API-Key")}
	if data, _ := io.ReadAll(r.Body); len(data) > 0 {
		_ = json.Unmarshal(data, &rec.Body)
	}
	f.mu.Lock()
	f.requests = append(f.requests, rec)
	f.mu.Unlock()
	return rec
}

func (f *fakePanther) all() []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recorded(nil), f.requests...)
}

func (f *fakePanther) last(t *testing.T) recorded {
	t.Helper()
	all := f.all()
	require.NotEmpty(t, all, "no request reached the fake Panther API")
	return all[len(all)-1]
}

// newNativeAdapter configures API token mode against handler, which sees
// each request after it is recorded.
func newNativeAdapter(t *testing.T, handler func(w http.ResponseWriter, rec recorded)) (*panther, *fakePanther) {
	t.Helper()
	fake := &fakePanther{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := fake.record(r)
		w.Header().Set("Content-Type", "application/json")
		handler(w, rec)
	}))
	t.Cleanup(srv.Close)
	p := New().(*panther)
	require.NoError(t, p.Configure(context.Background(), mcp.Credentials{"instance_url": srv.URL, "api_token": testToken}))
	p.native.pollInterval = time.Millisecond
	p.native.maxPollInterval = 2 * time.Millisecond
	t.Cleanup(func() { _ = p.Close() })
	return p, fake
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func run(t *testing.T, p *panther, tool mcp.ToolName, args map[string]any) *mcp.ToolResult {
	t.Helper()
	result, err := p.Execute(context.Background(), tool, args)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func decode(t *testing.T, result *mcp.ToolResult) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &out), result.Data)
	return out
}

func TestRESTTools_RequestMapping(t *testing.T) {
	tests := []struct {
		tool      mcp.ToolName
		args      map[string]any
		method    string
		path      string
		query     url.Values
		body      map[string]any
		response  string
		wantInOut string
	}{
		{tool: "panther_get_alert", args: map[string]any{"alert_id": "a1"}, method: "GET", path: "/alerts/a1", response: `{"id":"a1"}`, wantInOut: `"a1"`},
		{tool: "panther_list_alert_events", args: map[string]any{"alert_id": "a1", "limit": 5}, method: "GET", path: "/alerts/a1/events", query: url.Values{"limit": {"5"}}, response: `{"results":[{"p_event_time":"x"}]}`, wantInOut: `p_event_time`},
		{tool: "panther_list_alert_comments", args: map[string]any{"alert_id": "a1"}, method: "GET", path: "/alert-comments", query: url.Values{"alert-id": {"a1"}, "limit": {"25"}}, response: `{"results":[]}`},
		{tool: "panther_list_rules", args: map[string]any{"state": "enabled", "severity": []any{"HIGH", "CRITICAL"}, "log_type": "AWS.CloudTrail", "cursor": "c1", "limit": 10}, method: "GET", path: "/rules", query: url.Values{"state": {"enabled"}, "severity": {"HIGH", "CRITICAL"}, "log-type": {"AWS.CloudTrail"}, "cursor": {"c1"}, "limit": {"10"}}, response: `{"results":[{"id":"r1"}],"next":"c2"}`, wantInOut: `"next":"c2"`},
		{tool: "panther_get_rule", args: map[string]any{"rule_id": "AWS.CloudTrail.Stopped"}, method: "GET", path: "/rules/AWS.CloudTrail.Stopped", response: `{"id":"AWS.CloudTrail.Stopped"}`},
		{tool: "panther_list_scheduled_rules", args: map[string]any{"scheduled_query": "q"}, method: "GET", path: "/scheduled-rules", query: url.Values{"scheduled-query": {"q"}, "limit": {"25"}}, response: `{"results":[]}`},
		{tool: "panther_get_scheduled_rule", args: map[string]any{"rule_id": "s1"}, method: "GET", path: "/scheduled-rules/s1", response: `{"id":"s1"}`},
		{tool: "panther_list_simple_rules", args: map[string]any{"tag": "MITRE"}, method: "GET", path: "/simple-rules", query: url.Values{"tag": {"MITRE"}, "limit": {"25"}}, response: `{"results":[]}`},
		{tool: "panther_get_simple_rule", args: map[string]any{"rule_id": "y1"}, method: "GET", path: "/simple-rules/y1", response: `{"id":"y1"}`},
		{tool: "panther_list_policies", args: map[string]any{"resource_type": "AWS.S3.Bucket", "compliance_status": "FAIL"}, method: "GET", path: "/policies", query: url.Values{"resource-type": {"AWS.S3.Bucket"}, "compliance-status": {"FAIL"}, "limit": {"25"}}, response: `{"results":[]}`},
		{tool: "panther_get_policy", args: map[string]any{"policy_id": "p1"}, method: "GET", path: "/policies/p1", response: `{"id":"p1"}`},
		{tool: "panther_list_correlation_rules", args: map[string]any{"name_contains": "brute"}, method: "GET", path: "/correlation-rules", query: url.Values{"name-contains": {"brute"}, "limit": {"25"}}, response: `{"results":[]}`},
		{tool: "panther_get_correlation_rule", args: map[string]any{"rule_id": "c1"}, method: "GET", path: "/correlation-rules/c1", response: `{"id":"c1"}`},
		{tool: "panther_list_data_models", args: map[string]any{}, method: "GET", path: "/data-models", query: url.Values{"limit": {"25"}}, response: `{"results":[]}`},
		{tool: "panther_get_data_model", args: map[string]any{"data_model_id": "dm"}, method: "GET", path: "/data-models/dm", response: `{"id":"dm"}`},
		{tool: "panther_list_global_helpers", args: map[string]any{"name_contains": "aws"}, method: "GET", path: "/globals", query: url.Values{"name-contains": {"aws"}, "limit": {"25"}}, response: `{"results":[]}`},
		{tool: "panther_get_global_helper", args: map[string]any{"global_id": "panther_base_helpers"}, method: "GET", path: "/globals/panther_base_helpers", response: `{"id":"panther_base_helpers"}`},
		{tool: "panther_list_scheduled_queries", args: map[string]any{"limit": 3}, method: "GET", path: "/queries", query: url.Values{"limit": {"3"}}, response: `{"results":[]}`},
		{tool: "panther_get_scheduled_query", args: map[string]any{"query_id": "q1"}, method: "GET", path: "/queries/q1", response: `{"id":"q1"}`},
		{tool: "panther_list_users", args: map[string]any{"contains": "jo", "include_deactivated": true}, method: "GET", path: "/users", query: url.Values{"contains": {"jo"}, "include-deactivated": {"true"}, "limit": {"25"}}, response: `{"results":[]}`},
		{tool: "panther_get_user", args: map[string]any{"user_id": "u1"}, method: "GET", path: "/users/u1", response: `{"id":"u1"}`},
		{tool: "panther_list_roles", args: map[string]any{"name_contains": "Analyst"}, method: "GET", path: "/roles", query: url.Values{"name-contains": {"Analyst"}}, response: `{"results":[{"id":"r"}]}`},
		{tool: "panther_get_role", args: map[string]any{"role_id": "r1"}, method: "GET", path: "/roles/r1", response: `{"id":"r1"}`},
		{tool: "panther_update_alert_status", args: map[string]any{"alert_ids": "a1, a2", "status": "resolved"}, method: "PATCH", path: "/alerts", body: map[string]any{"ids": []any{"a1", "a2"}, "status": "RESOLVED"}, wantInOut: `"RESOLVED"`},
		{tool: "panther_assign_alerts", args: map[string]any{"alert_ids": []any{"a1"}, "assignee_id": "u1"}, method: "PATCH", path: "/alerts", body: map[string]any{"ids": []any{"a1"}, "assignee": "u1"}, wantInOut: `"assignee":"u1"`},
		{tool: "panther_add_alert_comment", args: map[string]any{"alert_id": "a1", "body": "Benign scanner"}, method: "POST", path: "/alert-comments", body: map[string]any{"alertId": "a1", "body": "Benign scanner", "format": "PLAIN_TEXT"}, response: `{"id":"cm1"}`, wantInOut: `"cm1"`},
	}
	for _, tt := range tests {
		t.Run(string(tt.tool), func(t *testing.T) {
			p, fake := newNativeAdapter(t, func(w http.ResponseWriter, _ recorded) {
				if tt.response == "" {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				_, _ = io.WriteString(w, tt.response)
			})
			result := run(t, p, tt.tool, tt.args)
			require.False(t, result.IsError, result.Data)
			got := fake.last(t)
			assert.Equal(t, tt.method, got.Method)
			assert.Equal(t, tt.path, got.Path)
			assert.Equal(t, testToken, got.Key)
			if tt.query != nil {
				assert.Equal(t, tt.query, got.Query)
			}
			if tt.body != nil {
				assert.Equal(t, tt.body, got.Body)
			}
			if tt.wantInOut != "" {
				assert.Contains(t, result.Data, tt.wantInOut)
			}
		})
	}
}

func TestListREST_AlwaysReturnsResultsAndNext(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     string
	}{
		{name: "last page without next", response: `{"results":[{"id":"r1"}]}`, want: `{"next":null,"results":[{"id":"r1"}]}`},
		{name: "empty next", response: `{"results":[],"next":""}`, want: `{"next":null,"results":[]}`},
		{name: "null results", response: `{"results":null}`, want: `{"next":null,"results":[]}`},
		{name: "more pages", response: `{"results":[{"id":"r1"}],"next":"abc"}`, want: `{"next":"abc","results":[{"id":"r1"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := newNativeAdapter(t, func(w http.ResponseWriter, _ recorded) { _, _ = io.WriteString(w, tt.response) })
			result := run(t, p, "panther_list_rules", nil)
			require.False(t, result.IsError, result.Data)
			assert.JSONEq(t, tt.want, result.Data)
		})
	}
}

func TestListAlerts_Defaults(t *testing.T) {
	p, fake := newNativeAdapter(t, func(w http.ResponseWriter, _ recorded) { _, _ = io.WriteString(w, `{"results":[]}`) })

	result := run(t, p, "panther_list_alerts", map[string]any{"severity": "CRITICAL,HIGH", "status": []any{"OPEN"}})
	require.False(t, result.IsError, result.Data)
	q := fake.last(t).Query
	assert.Equal(t, "ALERT", q.Get("type"))
	assert.Equal(t, "desc", q.Get("sort-dir"))
	assert.Equal(t, "25", q.Get("limit"))
	assert.Equal(t, []string{"CRITICAL", "HIGH"}, q["severity"])
	assert.Equal(t, []string{"OPEN"}, q["status"])
	after, err := time.Parse(time.RFC3339, q.Get("created-after"))
	require.NoError(t, err, "without a detection or dates the last 7 days are listed")
	before, err := time.Parse(time.RFC3339, q.Get("created-before"))
	require.NoError(t, err)
	assert.InDelta(t, defaultAlertWindow.Seconds(), before.Sub(after).Seconds(), 1)

	run(t, p, "panther_list_alerts", map[string]any{"detection_id": "AWS.Root.Login", "alert_type": "DETECTION_ERROR"})
	q = fake.last(t).Query
	assert.Equal(t, "AWS.Root.Login", q.Get("detection-id"))
	assert.Equal(t, "DETECTION_ERROR", q.Get("type"))
	assert.False(t, q.Has("created-after"), "a detection filter needs no default window")

	run(t, p, "panther_list_alerts", map[string]any{"created_after": "2026-01-01T00:00:00Z"})
	q = fake.last(t).Query
	assert.Equal(t, "2026-01-01T00:00:00Z", q.Get("created-after"))
	assert.False(t, q.Has("created-before"))
}

func TestArgumentValidation(t *testing.T) {
	tests := []struct {
		tool mcp.ToolName
		args map[string]any
		want string
	}{
		{tool: "panther_get_alert", args: map[string]any{}, want: "alert_id is required"},
		{tool: "panther_get_rule", args: map[string]any{"rule_id": ".."}, want: "not a valid id"},
		{tool: "panther_list_rules", args: map[string]any{"limit": 0}, want: "limit must be between"},
		{tool: "panther_list_rules", args: map[string]any{"limit": "many"}, want: "limit"},
		{tool: "panther_list_rules", args: map[string]any{"severity": 3}, want: "severity"},
		{tool: "panther_update_alert_status", args: map[string]any{"alert_ids": "a1", "status": "DONE"}, want: "status must be one of"},
		{tool: "panther_update_alert_status", args: map[string]any{"alert_ids": " , ", "status": "OPEN"}, want: "alert_ids is required"},
		{tool: "panther_assign_alerts", args: map[string]any{"alert_ids": "a1"}, want: "assignee_id is required"},
		{tool: "panther_add_alert_comment", args: map[string]any{"alert_id": "a1", "body": " "}, want: "alert_id and body are required"},
		{tool: "panther_add_alert_comment", args: map[string]any{"alert_id": "a1", "body": "x", "format": "MARKDOWN"}, want: "format must be"},
		{tool: "panther_set_detection_enabled", args: map[string]any{"detection_type": "rule", "detection_id": "r1"}, want: "enabled is required"},
		{tool: "panther_set_detection_enabled", args: map[string]any{"detection_type": "monitor", "detection_id": "r1", "enabled": true}, want: "detection_type must be one of"},
		{tool: "panther_query_data_lake", args: map[string]any{"sql": " "}, want: "sql is required"},
		{tool: "panther_query_data_lake", args: map[string]any{"sql": "select 1", "timeout_seconds": 999}, want: "timeout_seconds must be between"},
		{tool: "panther_get_data_lake_query", args: map[string]any{"query_id": "q", "page_size": 5000}, want: "page_size must be between"},
		{tool: "panther_get_table_schema", args: map[string]any{"database_name": "panther_logs"}, want: "table_name is required"},
		{tool: "panther_get_alert_metrics", args: map[string]any{"from": "yesterday"}, want: "from must be an ISO-8601"},
		{tool: "panther_get_alert_metrics", args: map[string]any{"from": "2026-02-01T00:00:00Z", "to": "2026-01-01T00:00:00Z"}, want: "from must be before to"},
	}
	for _, tt := range tests {
		t.Run(string(tt.tool)+" "+tt.want, func(t *testing.T) {
			p, fake := newNativeAdapter(t, func(w http.ResponseWriter, _ recorded) { _, _ = io.WriteString(w, `{}`) })
			result := run(t, p, tt.tool, tt.args)
			require.True(t, result.IsError, result.Data)
			assert.Contains(t, result.Data, tt.want)
			assert.Empty(t, fake.all(), "invalid arguments must not reach Panther")
		})
	}
}

func TestAPIErrors(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		retryable bool
		want      string
	}{
		{name: "unauthorized", status: 401, body: `{"message":"invalid token"}`, want: "invalid token; the API token is missing, expired"},
		{name: "forbidden", status: 403, body: `{"message":"missing permission RuleRead"}`, want: "panther_get_permissions"},
		{name: "not found", status: 404, body: `{"message":"rule not found"}`, want: "(404) on GET /rules/r1: rule not found"},
		{name: "redirect is not followed", status: 302, body: ``, want: "redirects are not followed"},
		{name: "throttled", status: 429, body: `{"message":"slow down"}`, retryable: true},
		{name: "server error", status: 502, body: `bad gateway`, retryable: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := newNativeAdapter(t, func(w http.ResponseWriter, _ recorded) {
				if tt.status == 302 {
					w.Header().Set("Location", "https://elsewhere.example/steal")
				}
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			})
			result, err := p.Execute(context.Background(), "panther_get_rule", map[string]any{"rule_id": "r1"})
			if tt.retryable {
				require.Error(t, err)
				assert.True(t, mcp.IsRetryable(err))
				return
			}
			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Contains(t, result.Data, tt.want)
			assert.NotContains(t, result.Data, testToken)
		})
	}
}

func TestNonJSONSuccessIsError(t *testing.T) {
	p, _ := newNativeAdapter(t, func(w http.ResponseWriter, _ recorded) { _, _ = io.WriteString(w, `<html>login</html>`) })
	result := run(t, p, "panther_get_rule", map[string]any{"rule_id": "r1"})
	require.True(t, result.IsError)
	assert.Contains(t, result.Data, "not JSON")
}

func TestSetDetectionEnabled_ReadModifyWrite(t *testing.T) {
	tests := []struct {
		kind string
		path string
	}{
		{kind: "rule", path: "/rules/r1"},
		{kind: "scheduled_rule", path: "/scheduled-rules/r1"},
		{kind: "simple_rule", path: "/simple-rules/r1"},
		{kind: "policy", path: "/policies/r1"},
		{kind: "correlation_rule", path: "/correlation-rules/r1"},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			p, fake := newNativeAdapter(t, func(w http.ResponseWriter, rec recorded) {
				if rec.Method == http.MethodGet {
					_, _ = io.WriteString(w, `{"id":"r1","enabled":true,"severity":"HIGH","body":"def rule(e): return True","tests":[{"name":"t"}]}`)
					return
				}
				_ = json.NewEncoder(w).Encode(rec.Body)
			})
			result := run(t, p, "panther_set_detection_enabled", map[string]any{"detection_type": tt.kind, "detection_id": "r1", "enabled": false})
			require.False(t, result.IsError, result.Data)
			reqs := fake.all()
			require.Len(t, reqs, 2)
			assert.Equal(t, http.MethodGet, reqs[0].Method)
			assert.Equal(t, tt.path, reqs[0].Path)
			put := reqs[1]
			assert.Equal(t, http.MethodPut, put.Method)
			assert.Equal(t, tt.path, put.Path)
			assert.Equal(t, "false", put.Query.Get("run-tests-first"))
			assert.Equal(t, map[string]any{"id": "r1", "enabled": false, "severity": "HIGH", "body": "def rule(e): return True", "tests": []any{map[string]any{"name": "t"}}}, put.Body, "every other field goes back unchanged")
		})
	}
}

func TestGetPermissions_NeverReturnsTokenValue(t *testing.T) {
	p, fake := newNativeAdapter(t, func(w http.ResponseWriter, _ recorded) {
		_, _ = io.WriteString(w, `{"id":"tok1","name":"switchboard","value":"SECRET-VALUE","permissions":["AlertRead","RuleRead"],"createdAt":"2026-01-01T00:00:00Z","createdBy":{"id":"u"}}`)
	})
	result := run(t, p, "panther_get_permissions", nil)
	require.False(t, result.IsError, result.Data)
	assert.Equal(t, "/api-tokens/self", fake.last(t).Path)
	assert.NotContains(t, result.Data, "SECRET-VALUE")
	out := decode(t, result)
	assert.Equal(t, []any{"AlertRead", "RuleRead"}, out["permissions"])
	assert.Equal(t, "switchboard", out["name"])
}

func TestHealthy(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   bool
	}{
		{name: "token accepted", status: 200, want: true},
		{name: "token rejected", status: 401, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, fake := newNativeAdapter(t, func(w http.ResponseWriter, _ recorded) { writeJSON(w, tt.status, map[string]any{"id": "self"}) })
			assert.Equal(t, tt.want, p.Healthy(context.Background()))
			assert.Equal(t, "/api-tokens/self", fake.last(t).Path)
		})
	}
	assert.False(t, New().Healthy(context.Background()), "an unconfigured adapter is not healthy")
}

func TestExecuteErrors(t *testing.T) {
	p, _ := newNativeAdapter(t, func(w http.ResponseWriter, _ recorded) {})
	result := run(t, p, "panther_nope", nil)
	require.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")

	unconfigured, err := New().Execute(context.Background(), "panther_list_alerts", nil)
	require.NoError(t, err)
	require.True(t, unconfigured.IsError)
	assert.Contains(t, unconfigured.Data, mcp.ErrNotConfigured.Error())
}

func TestGraphQLErrorsFailTheCall(t *testing.T) {
	p, _ := newNativeAdapter(t, func(w http.ResponseWriter, _ recorded) {
		_, _ = io.WriteString(w, `{"data":null,"errors":[{"message":"You do not have permission to perform this action"}]}`)
	})
	result := run(t, p, "panther_list_databases", nil)
	require.True(t, result.IsError)
	assert.Contains(t, result.Data, "do not have permission")
	assert.Contains(t, result.Data, "Settings > Developer Tools > API Tokens")
}

func TestGraphQLRequests(t *testing.T) {
	tests := []struct {
		tool      mcp.ToolName
		args      map[string]any
		operation string
		vars      map[string]any
		response  string
		want      string
	}{
		{tool: "panther_list_databases", operation: "ListDatabases", response: `{"data":{"dataLakeDatabases":[{"name":"panther_logs","description":"Logs"}]}}`, want: `{"results":[{"name":"panther_logs","description":"Logs"}]}`},
		{
			tool: "panther_list_database_tables", args: map[string]any{"database_name": "panther_logs", "limit": 2, "cursor": "t1"}, operation: "ListTables",
			vars:     map[string]any{"input": map[string]any{"databaseName": "panther_logs", "pageSize": float64(2), "cursor": "t1"}},
			response: `{"data":{"dataLakeDatabaseTables":{"edges":[{"node":{"name":"aws_cloudtrail","logType":"AWS.CloudTrail"}}],"pageInfo":{"hasNextPage":true,"endCursor":"t2"}}}}`,
			want:     `{"next":"t2","results":[{"name":"aws_cloudtrail","logType":"AWS.CloudTrail"}]}`,
		},
		{
			tool: "panther_get_table_schema", args: map[string]any{"database_name": "panther_logs", "table_name": "aws_cloudtrail"}, operation: "GetTable",
			vars:     map[string]any{"input": map[string]any{"databaseName": "panther_logs", "tableName": "aws_cloudtrail"}},
			response: `{"data":{"dataLakeDatabaseTable":{"name":"aws_cloudtrail","columns":[{"name":"eventName","type":"string"}]}}}`,
			want:     `{"name":"aws_cloudtrail","columns":[{"name":"eventName","type":"string"}]}`,
		},
		{
			tool: "panther_list_data_lake_queries", args: map[string]any{"contains": "cloudtrail", "status": "RUNNING", "is_scheduled": false, "started_after": "2026-01-01T00:00:00Z"}, operation: "ListQueries",
			vars:     map[string]any{"input": map[string]any{"contains": "cloudtrail", "status": []any{"running"}, "isScheduled": false, "startedAtAfter": "2026-01-01T00:00:00Z", "pageSize": float64(25), "cursor": nil}},
			response: `{"data":{"dataLakeQueries":{"edges":[{"node":{"id":"q1","status":"running"}}],"pageInfo":{"hasNextPage":false,"endCursor":"z"}}}}`,
			want:     `{"next":null,"results":[{"id":"q1","status":"running"}]}`,
		},
		{
			tool: "panther_cancel_data_lake_query", args: map[string]any{"query_id": "q1"}, operation: "CancelQuery",
			vars:     map[string]any{"input": map[string]any{"id": "q1"}},
			response: `{"data":{"cancelDataLakeQuery":{"id":"q1"}}}`,
			want:     `{"cancelled":"q1"}`,
		},
		{
			tool: "panther_list_log_sources", args: map[string]any{"cursor": "s1"}, operation: "ListSources",
			vars:     map[string]any{"input": map[string]any{"cursor": "s1"}},
			response: `{"data":{"sources":{"edges":[{"node":{"integrationId":"src","isHealthy":true}}],"pageInfo":{"hasNextPage":false,"endCursor":null}}}}`,
			want:     `{"next":null,"results":[{"integrationId":"src","isHealthy":true}]}`,
		},
		{
			tool: "panther_get_log_source", args: map[string]any{"source_id": "src"}, operation: "GetSource",
			vars:     map[string]any{"id": "src"},
			response: `{"data":{"source":{"integrationId":"src","s3Bucket":"logs"}}}`,
			want:     `{"integrationId":"src","s3Bucket":"logs"}`,
		},
		{
			tool: "panther_list_schemas", args: map[string]any{"contains": "AWS", "is_managed": true}, operation: "ListSchemas",
			vars:     map[string]any{"input": map[string]any{"contains": "AWS", "isManaged": true}},
			response: `{"data":{"schemas":{"edges":[{"node":{"name":"AWS.CloudTrail"}}]}}}`,
			want:     `{"next":null,"results":[{"name":"AWS.CloudTrail"}]}`,
		},
		{
			tool: "panther_get_schema", args: map[string]any{"name": "AWS.ALB"}, operation: "GetSchema",
			vars:     map[string]any{"input": map[string]any{"contains": "AWS.ALB"}},
			response: `{"data":{"schemas":{"edges":[{"node":{"name":"AWS.ALBExtra","spec":"x"}},{"node":{"name":"AWS.ALB","spec":"fields: []"}}]}}}`,
			want:     `{"name":"AWS.ALB","spec":"fields: []"}`,
		},
	}
	for _, tt := range tests {
		t.Run(string(tt.tool), func(t *testing.T) {
			p, fake := newNativeAdapter(t, func(w http.ResponseWriter, _ recorded) { _, _ = io.WriteString(w, tt.response) })
			result := run(t, p, tt.tool, tt.args)
			require.False(t, result.IsError, result.Data)
			assert.JSONEq(t, tt.want, result.Data)
			got := fake.last(t)
			assert.Equal(t, http.MethodPost, got.Method)
			assert.Equal(t, graphqlPath, got.Path)
			assert.Equal(t, testToken, got.Key)
			assert.Equal(t, tt.operation, got.Body["operationName"])
			if tt.vars != nil {
				assert.Equal(t, tt.vars, got.Body["variables"])
			}
		})
	}
}

func TestGetSchema_NoExactMatch(t *testing.T) {
	p, _ := newNativeAdapter(t, func(w http.ResponseWriter, _ recorded) {
		_, _ = io.WriteString(w, `{"data":{"schemas":{"edges":[{"node":{"name":"AWS.ALBExtra"}}]}}}`)
	})
	result := run(t, p, "panther_get_schema", map[string]any{"name": "AWS.ALB"})
	require.True(t, result.IsError)
	assert.Contains(t, result.Data, "panther_list_schemas")
}

func TestGetLogSource_Missing(t *testing.T) {
	p, _ := newNativeAdapter(t, func(w http.ResponseWriter, _ recorded) { _, _ = io.WriteString(w, `{"data":{"source":null}}`) })
	result := run(t, p, "panther_get_log_source", map[string]any{"source_id": "nope"})
	require.True(t, result.IsError)
	assert.Contains(t, result.Data, "no log source with id nope")
}

func TestMetrics(t *testing.T) {
	tests := []struct {
		tool   mcp.ToolName
		fields []string
	}{
		{tool: "panther_get_alert_metrics", fields: []string{"alertsPerSeverity", "alertsPerRule", "totalAlerts"}},
		{tool: "panther_get_data_metrics", fields: []string{"eventsProcessedPerLogType", "bytesProcessedPerSource", "totalBytesQueried"}},
	}
	for _, tt := range tests {
		t.Run(string(tt.tool), func(t *testing.T) {
			p, fake := newNativeAdapter(t, func(w http.ResponseWriter, _ recorded) {
				_, _ = io.WriteString(w, `{"data":{"metrics":{"totalAlerts":3}}}`)
			})
			result := run(t, p, tt.tool, map[string]any{"from": "2026-01-01T00:00:00Z", "to": "2026-01-08T00:00:00Z", "interval_minutes": 60})
			require.False(t, result.IsError, result.Data)
			got := fake.last(t)
			assert.Equal(t, map[string]any{"input": map[string]any{"fromDate": "2026-01-01T00:00:00Z", "toDate": "2026-01-08T00:00:00Z", "intervalInMinutes": float64(60)}}, got.Body["variables"])
			for _, field := range tt.fields {
				assert.Contains(t, got.Body["query"], field)
			}
			assert.JSONEq(t, `{"fromDate":"2026-01-01T00:00:00Z","toDate":"2026-01-08T00:00:00Z","metrics":{"totalAlerts":3}}`, result.Data)
		})
	}

	p, fake := newNativeAdapter(t, func(w http.ResponseWriter, _ recorded) { _, _ = io.WriteString(w, `{"data":{"metrics":{}}}`) })
	run(t, p, "panther_get_alert_metrics", nil)
	input := fake.last(t).Body["variables"].(map[string]any)["input"].(map[string]any)
	from, err := time.Parse(time.RFC3339, input["fromDate"].(string))
	require.NoError(t, err)
	to, err := time.Parse(time.RFC3339, input["toDate"].(string))
	require.NoError(t, err)
	assert.InDelta(t, defaultMetricsWindow.Seconds(), to.Sub(from).Seconds(), 1, "metrics default to the last 7 days")
	assert.NotContains(t, input, "intervalInMinutes")
}

// dataLakeFake answers the execute mutation and then reports the query as
// running for runningPolls status checks before it finishes with status.
func dataLakeFake(runningPolls int, final string) func(w http.ResponseWriter, rec recorded) {
	var mu sync.Mutex
	polls := 0
	return func(w http.ResponseWriter, rec recorded) {
		switch rec.Body["operationName"] {
		case "ExecuteQuery":
			_, _ = io.WriteString(w, `{"data":{"executeDataLakeQuery":{"id":"q-123"}}}`)
		case "GetQuery":
			mu.Lock()
			polls++
			n := polls
			mu.Unlock()
			if n <= runningPolls {
				_, _ = io.WriteString(w, `{"data":{"dataLakeQuery":{"id":"q-123","status":"running","message":"","sql":"select 1","startedAt":"t","results":null}}}`)
				return
			}
			_, _ = io.WriteString(w, final)
		default:
			http.Error(w, "unexpected operation", http.StatusBadRequest)
		}
	}
}

const succeededQuery = `{"data":{"dataLakeQuery":{"id":"q-123","status":"succeeded","message":"ok","sql":"select 1","startedAt":"t0","completedAt":"t1",
  "results":{"edges":[{"node":{"eventName":"ConsoleLogin","n":2}}],"pageInfo":{"hasNextPage":true,"endCursor":"page2"},
  "columnInfo":{"order":["eventName","n"],"types":{"eventName":"string","n":"number"}},"stats":{"bytesScanned":10,"executionTime":5,"rowCount":1},"warningMessage":null}}}}`

func TestQueryDataLake_PollsUntilDone(t *testing.T) {
	p, fake := newNativeAdapter(t, dataLakeFake(2, succeededQuery))
	result := run(t, p, "panther_query_data_lake", map[string]any{"sql": "SELECT eventName FROM panther_logs.public.aws_cloudtrail WHERE p_occurs_since('1 d')", "database_name": "panther_logs.public", "page_size": 50})
	require.False(t, result.IsError, result.Data)

	reqs := fake.all()
	require.Len(t, reqs, 4, "one execute and three status checks")
	assert.Equal(t, map[string]any{"input": map[string]any{"sql": "SELECT eventName FROM panther_logs.public.aws_cloudtrail WHERE p_occurs_since('1 d')", "databaseName": "panther_logs.public"}}, reqs[0].Body["variables"])
	assert.Equal(t, map[string]any{"id": "q-123", "results": map[string]any{"pageSize": float64(50), "cursor": nil}}, reqs[1].Body["variables"])

	out := decode(t, result)
	assert.Equal(t, "q-123", out["queryId"])
	assert.Equal(t, "succeeded", out["status"])
	assert.Equal(t, []any{map[string]any{"eventName": "ConsoleLogin", "n": float64(2)}}, out["rows"])
	assert.Equal(t, []any{"eventName", "n"}, out["columns"])
	assert.Equal(t, "page2", out["next"])
	assert.NotContains(t, out, "warningMessage")
}

func TestQueryDataLake_TimeoutReturnsRunningQuery(t *testing.T) {
	p, fake := newNativeAdapter(t, dataLakeFake(1_000_000, succeededQuery))
	p.native.pollInterval = 300 * time.Millisecond
	p.native.maxPollInterval = 300 * time.Millisecond
	result := run(t, p, "panther_query_data_lake", map[string]any{"sql": "select 1", "timeout_seconds": 1})
	require.False(t, result.IsError, result.Data)
	out := decode(t, result)
	assert.Equal(t, "running", out["status"])
	assert.Equal(t, "q-123", out["queryId"])
	assert.Contains(t, out["message"], "panther_get_data_lake_query")
	for _, req := range fake.all() {
		assert.NotEqual(t, "CancelQuery", req.Body["operationName"], "a slow query is left running for the caller to poll")
	}
}

func TestQueryDataLake_FailedQueryIsError(t *testing.T) {
	failed := `{"data":{"dataLakeQuery":{"id":"q-123","status":"failed","message":"SQL compilation error: invalid identifier 'NOPE'","sql":"x","startedAt":"t","results":null}}}`
	p, _ := newNativeAdapter(t, dataLakeFake(0, failed))
	result := run(t, p, "panther_query_data_lake", map[string]any{"sql": "select nope"})
	require.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid identifier")
	assert.Contains(t, result.Data, "q-123")
}

func TestQueryDataLake_ContextCanceled(t *testing.T) {
	p, _ := newNativeAdapter(t, dataLakeFake(1_000_000, succeededQuery))
	p.native.pollInterval = time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	result, err := p.Execute(ctx, "panther_query_data_lake", map[string]any{"sql": "select 1", "timeout_seconds": 60})
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Contains(t, result.Data, "stopped waiting for query q-123")
}

func TestGetDataLakeQuery_NextPage(t *testing.T) {
	p, fake := newNativeAdapter(t, func(w http.ResponseWriter, _ recorded) {
		_, _ = io.WriteString(w, strings.ReplaceAll(succeededQuery, `"hasNextPage":true`, `"hasNextPage":false`))
	})
	result := run(t, p, "panther_get_data_lake_query", map[string]any{"query_id": "q-123", "cursor": "page2"})
	require.False(t, result.IsError, result.Data)
	assert.Equal(t, map[string]any{"id": "q-123", "results": map[string]any{"pageSize": float64(defaultResultRows), "cursor": "page2"}}, fake.last(t).Body["variables"])
	out := decode(t, result)
	assert.Contains(t, out, "next")
	assert.Nil(t, out["next"])
}
