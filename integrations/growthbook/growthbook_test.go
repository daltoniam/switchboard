package growthbook

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testKey = "secret_admin_unit"

type recordedRequest struct {
	Method string
	Path   string
	Query  map[string]string
	Body   map[string]any
}

// requestLog records requests from the server goroutine for the test
// goroutine to read.
type requestLog struct {
	mu       sync.Mutex
	requests []recordedRequest
}

func (l *requestLog) add(record recordedRequest) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.requests = append(l.requests, record)
}

func (l *requestLog) list() []recordedRequest {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]recordedRequest(nil), l.requests...)
}

func newTestServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) (*growthbook, *requestLog) {
	t.Helper()
	requests := &requestLog{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The handler runs outside the test goroutine, so it reports with
		// non-fatal assertions and answers with an error status instead.
		if !assert.Equal(t, "Bearer "+testKey, r.Header.Get("Authorization")) {
			http.Error(w, `{"message":"unexpected authorization"}`, http.StatusUnauthorized)
			return
		}
		record := recordedRequest{Method: r.Method, Path: r.URL.Path, Query: map[string]string{}}
		for key := range r.URL.Query() {
			record.Query[key] = r.URL.Query().Get(key)
		}
		if r.Body != nil {
			data, _ := io.ReadAll(r.Body)
			if len(data) > 0 {
				if !assert.Equal(t, "application/json", r.Header.Get("Content-Type")) || !assert.NoError(t, json.Unmarshal(data, &record.Body)) {
					http.Error(w, `{"message":"unexpected request body"}`, http.StatusBadRequest)
					return
				}
			}
		}
		requests.add(record)
		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	adapter := New().(*growthbook)
	t.Cleanup(func() { require.NoError(t, adapter.Close()) })
	require.NoError(t, adapter.Configure(context.Background(), mcp.Credentials{
		"api_key":  testKey,
		"base_url": server.URL,
		"ca_cert":  strings.ReplaceAll(string(cert), "\n", `\n`),
	}))
	return adapter, requests
}

func TestNew(t *testing.T) {
	adapter := New()
	require.NotNil(t, adapter)
	assert.Equal(t, "growthbook", adapter.Name())
}

func TestConfigure(t *testing.T) {
	tests := []struct {
		name    string
		creds   mcp.Credentials
		wantErr string
		wantURL string
	}{
		{name: "cloud default", creds: mcp.Credentials{"api_key": "secret_abc"}, wantURL: "https://api.growthbook.io/api"},
		{name: "personal access token", creds: mcp.Credentials{"api_key": "secret_user_abc"}, wantURL: "https://api.growthbook.io/api"},
		{name: "self-hosted host appends api", creds: mcp.Credentials{"api_key": "secret_abc", "base_url": "https://growthbook.example.com:3100/"}, wantURL: "https://growthbook.example.com:3100/api"},
		{name: "self-hosted api path kept", creds: mcp.Credentials{"api_key": "secret_abc", "base_url": "https://growthbook.example.com/api/"}, wantURL: "https://growthbook.example.com/api"},
		{name: "self-hosted prefix", creds: mcp.Credentials{"api_key": "secret_abc", "base_url": "https://example.com/growthbook"}, wantURL: "https://example.com/growthbook/api"},
		{name: "missing key", creds: mcp.Credentials{}, wantErr: "api_key is required"},
		{name: "whitespace key", creds: mcp.Credentials{"api_key": "secret abc"}, wantErr: "api_key is required"},
		{name: "sdk client key", creds: mcp.Credentials{"api_key": "sdk-abc123"}, wantErr: "SDK client keys"},
		{name: "plain http", creds: mcp.Credentials{"api_key": "secret_abc", "base_url": "http://growthbook.example.com"}, wantErr: "HTTPS"},
		{name: "userinfo", creds: mcp.Credentials{"api_key": "secret_abc", "base_url": "https://user:pass@growthbook.example.com"}, wantErr: "HTTPS"},
		{name: "query", creds: mcp.Credentials{"api_key": "secret_abc", "base_url": "https://growthbook.example.com/?x=1"}, wantErr: "HTTPS"},
		{name: "dot segment", creds: mcp.Credentials{"api_key": "secret_abc", "base_url": "https://growthbook.example.com/../api"}, wantErr: "HTTPS"},
		{name: "bad ca", creds: mcp.Credentials{"api_key": "secret_abc", "ca_cert": "not a cert"}, wantErr: "ca_cert"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := New().(*growthbook)
			err := adapter.Configure(context.Background(), tt.creds)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, adapter.client)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, adapter.baseURL)
		})
	}
}

func TestConfigure_FailureClearsPreviousState(t *testing.T) {
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"projects":[]}`)
	})
	require.True(t, adapter.Healthy(context.Background()))
	require.Error(t, adapter.Configure(context.Background(), mcp.Credentials{}))
	assert.Empty(t, adapter.apiKey)
	assert.False(t, adapter.Healthy(context.Background()))
	assert.Len(t, requests.list(), 1, "a failed reconfigure must not keep the old credentials")
}

func TestCloseIsIdempotent(t *testing.T) {
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"projects":[]}`)
	})
	require.True(t, adapter.Healthy(context.Background()))
	require.Len(t, requests.list(), 1)
	require.NoError(t, adapter.Close())
	require.NoError(t, adapter.Close())
	assert.False(t, adapter.Healthy(context.Background()))
	result, err := adapter.Execute(context.Background(), "growthbook_list_projects", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Len(t, requests.list(), 1, "a closed adapter must not reach GrowthBook")
}

func TestHealthy(t *testing.T) {
	assert.False(t, New().Healthy(context.Background()))
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"projects":[],"limit":1,"offset":0,"count":0,"total":0,"hasMore":false,"nextOffset":null}`)
	})
	assert.True(t, adapter.Healthy(context.Background()))
	require.Len(t, requests.list(), 1)
	assert.Equal(t, "/api/v1/projects", requests.list()[0].Path)
	assert.Equal(t, "1", requests.list()[0].Query["limit"])
}

func TestHealthy_Unauthorized(t *testing.T) {
	adapter, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"message":"Invalid API key"}`)
	})
	assert.False(t, adapter.Healthy(context.Background()))
}

func TestTools_Metadata(t *testing.T) {
	seen := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		assert.True(t, strings.HasPrefix(string(tool.Name), "growthbook_"), tool.Name)
		assert.NotEmpty(t, tool.Description, tool.Name)
		assert.Contains(t, tool.Description, "GrowthBook", tool.Name)
		assert.True(t, tool.Access.Declared(), "%s must declare Access", tool.Name)
		assert.False(t, seen[tool.Name], "duplicate tool %s", tool.Name)
		seen[tool.Name] = true
		for _, required := range tool.Required {
			_, ok := tool.Parameters[required]
			assert.True(t, ok, "%s requires undocumented parameter %s", tool.Name, required)
		}
	}
}

func TestTools_EntryPointsSayStartHere(t *testing.T) {
	byName := map[mcp.ToolName]mcp.ToolDefinition{}
	for _, tool := range New().Tools() {
		byName[tool.Name] = tool
	}
	for _, name := range []mcp.ToolName{"growthbook_list_features", "growthbook_list_experiments"} {
		assert.Contains(t, byName[name].Description, "Start here", name)
	}
}

func TestTools_AccessClassification(t *testing.T) {
	writes := map[mcp.ToolName]bool{
		"growthbook_toggle_feature":             true,
		"growthbook_create_feature":             true,
		"growthbook_refresh_experiment_results": true,
	}
	for _, tool := range New().Tools() {
		if writes[tool.Name] {
			assert.Equal(t, mcp.AccessWrite, tool.Access, tool.Name)
			continue
		}
		assert.Equal(t, mcp.AccessRead, tool.Access, tool.Name)
	}
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

func TestExecute_UnknownTool(t *testing.T) {
	adapter, _ := newTestServer(t, func(http.ResponseWriter, *http.Request) {})
	result, err := adapter.Execute(context.Background(), "growthbook_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestExecute_Unconfigured(t *testing.T) {
	result, err := New().Execute(context.Background(), "growthbook_list_projects", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "not configured")
}

func TestReadTools_RequestMapping(t *testing.T) {
	tests := []struct {
		tool      mcp.ToolName
		args      map[string]any
		wantPath  string
		wantQuery map[string]string
	}{
		{"growthbook_list_features", map[string]any{"project_id": "prj_1", "archived": true, "limit": 500, "offset": 20}, "/api/v2/features", map[string]string{"projectId": "prj_1", "archived": "true", "limit": "100", "offset": "20"}},
		{"growthbook_list_features", nil, "/api/v2/features", map[string]string{"limit": "20", "offset": "0"}},
		{"growthbook_get_feature", map[string]any{"feature_id": "dark mode:v2"}, "/api/v2/features/dark mode:v2", map[string]string{}},
		{"growthbook_get_feature_staleness", map[string]any{"feature_ids": "a, b"}, "/api/v2/stale-features", map[string]string{"ids": "a,b"}},
		{"growthbook_list_feature_revisions", map[string]any{"feature_id": "f1", "status": "published"}, "/api/v2/features/f1/revisions", map[string]string{"status": "published", "limit": "20", "offset": "0"}},
		{"growthbook_list_experiments", map[string]any{"project_id": "prj_1", "status": "running", "search": "checkout", "tag": "web", "archived": false}, "/api/v1/experiments", map[string]string{"projectId": "prj_1", "status": "running", "q": "checkout", "tag": "web", "archived": "false", "limit": "20", "offset": "0"}},
		{"growthbook_get_experiment", map[string]any{"experiment_id": "exp_1"}, "/api/v1/experiments/exp_1", map[string]string{}},
		{"growthbook_get_experiment_results", map[string]any{"experiment_id": "exp_1", "phase": 0, "dimension": "exp:country"}, "/api/v1/experiments/exp_1/results", map[string]string{"phase": "0", "dimension": "exp:country"}},
		{"growthbook_get_experiment_results", map[string]any{"experiment_id": "exp_1"}, "/api/v1/experiments/exp_1/results", map[string]string{}},
		{"growthbook_get_snapshot", map[string]any{"snapshot_id": "snp_1"}, "/api/v1/snapshots/snp_1", map[string]string{}},
		{"growthbook_list_fact_metrics", map[string]any{"project_id": "prj_1", "datasource_id": "ds_1", "fact_table_id": "ftb_1"}, "/api/v1/fact-metrics", map[string]string{"projectId": "prj_1", "datasourceId": "ds_1", "factTableId": "ftb_1", "limit": "20", "offset": "0"}},
		{"growthbook_get_fact_metric", map[string]any{"metric_id": "fact__1"}, "/api/v1/fact-metrics/fact__1", map[string]string{}},
		{"growthbook_list_metrics", map[string]any{"project_id": "prj_1", "datasource_id": "ds_1"}, "/api/v1/metrics", map[string]string{"projectId": "prj_1", "datasourceId": "ds_1", "limit": "20", "offset": "0"}},
		{"growthbook_list_fact_tables", map[string]any{"datasource_id": "ds_1"}, "/api/v1/fact-tables", map[string]string{"datasourceId": "ds_1", "limit": "20", "offset": "0"}},
		{"growthbook_list_projects", nil, "/api/v1/projects", map[string]string{"limit": "20", "offset": "0"}},
		{"growthbook_list_environments", nil, "/api/v1/environments", map[string]string{}},
		{"growthbook_list_sdk_connections", map[string]any{"project_id": "prj_1"}, "/api/v1/sdk-connections", map[string]string{"projectId": "prj_1", "limit": "20", "offset": "0"}},
		{"growthbook_list_saved_groups", nil, "/api/v1/saved-groups", map[string]string{"limit": "20", "offset": "0"}},
		{"growthbook_get_saved_group", map[string]any{"saved_group_id": "grp_1"}, "/api/v1/saved-groups/grp_1", map[string]string{}},
		{"growthbook_list_attributes", nil, "/api/v1/attributes", map[string]string{}},
		{"growthbook_list_data_sources", map[string]any{"project_id": "prj_1"}, "/api/v1/data-sources", map[string]string{"projectId": "prj_1", "limit": "20", "offset": "0"}},
		{"growthbook_list_segments", map[string]any{"datasource_id": "ds_1"}, "/api/v1/segments", map[string]string{"datasourceId": "ds_1", "limit": "20", "offset": "0"}},
		{"growthbook_list_audit_logs", map[string]any{"entity_type": "feature", "entity_id": "f1", "limit": 5, "before": "2026-01-01T00:00:00Z"}, "/api/v1/audits", map[string]string{"entityType": "feature", "entityId": "f1", "limit": "5", "before": "2026-01-01T00:00:00Z"}},
		{"growthbook_list_code_refs", nil, "/api/v1/code-refs", map[string]string{"limit": "20", "offset": "0"}},
		{"growthbook_get_feature_code_refs", map[string]any{"feature_id": "f1"}, "/api/v1/code-refs/f1", map[string]string{}},
		{"growthbook_get_settings", nil, "/api/v1/settings", map[string]string{}},
		{"growthbook_list_members", map[string]any{"limit": 5}, "/api/v1/members", map[string]string{"limit": "5", "offset": "0"}},
	}
	covered := map[mcp.ToolName]bool{}
	for _, tt := range tests {
		covered[tt.tool] = true
		t.Run(string(tt.tool), func(t *testing.T) {
			adapter, requests := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, `{"ok":true}`)
			})
			result, err := adapter.Execute(context.Background(), tt.tool, tt.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			require.Len(t, requests.list(), 1)
			got := requests.list()[0]
			assert.Equal(t, http.MethodGet, got.Method)
			assert.Equal(t, tt.wantPath, got.Path)
			assert.Equal(t, tt.wantQuery, got.Query)
		})
	}
	for _, tool := range New().Tools() {
		if tool.Access == mcp.AccessRead {
			assert.True(t, covered[tool.Name], "read tool %s has no request mapping case", tool.Name)
		}
	}
}

func TestReadTools_RequiredArgs(t *testing.T) {
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	})
	for _, tool := range New().Tools() {
		if len(tool.Required) == 0 {
			continue
		}
		t.Run(string(tool.Name), func(t *testing.T) {
			result, err := adapter.Execute(context.Background(), tool.Name, map[string]any{})
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tool.Required[0])
		})
	}
	assert.Empty(t, requests.list(), "missing required args must not reach GrowthBook")
}

func TestPathArgsRejectTraversal(t *testing.T) {
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	})
	for _, id := range []string{"..", ".", "a/../../v1/members", "x\ny"} {
		result, err := adapter.Execute(context.Background(), "growthbook_get_feature", map[string]any{"feature_id": id})
		require.NoError(t, err)
		assert.True(t, result.IsError, id)
	}
	assert.Empty(t, requests.list())
}

func TestInvalidArgsAreRejected(t *testing.T) {
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	})
	tests := []struct {
		tool mcp.ToolName
		args map[string]any
		want string
	}{
		{"growthbook_list_features", map[string]any{"limit": "many"}, "limit"},
		{"growthbook_list_features", map[string]any{"offset": -1}, "offset"},
		{"growthbook_list_experiments", map[string]any{"status": "paused"}, "status"},
		{"growthbook_get_experiment_results", map[string]any{"experiment_id": "exp_1", "phase": -1}, "phase"},
		{"growthbook_list_audit_logs", map[string]any{"entity_type": "feature", "limit": 0}, "limit"},
		{"growthbook_get_feature_staleness", map[string]any{"feature_ids": " , "}, "feature_ids"},
	}
	for _, tt := range tests {
		result, err := adapter.Execute(context.Background(), tt.tool, tt.args)
		require.NoError(t, err)
		assert.True(t, result.IsError, "%s %v", tt.tool, tt.args)
		assert.Contains(t, result.Data, tt.want)
	}
	assert.Empty(t, requests.list())
}

func TestListSDKConnections_StripsSecrets(t *testing.T) {
	adapter, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"connections":[{"id":"sdk_1","name":"Web","key":"sdk-public","encryptionKey":"SECRET_ENCRYPTION","proxySigningKey":"SECRET_PROXY","proxy":{"signingKey":"SECRET_NESTED","host":"proxy.example"}}],"limit":20,"offset":0,"count":1,"total":1,"hasMore":false,"nextOffset":null}`)
	})
	result, err := adapter.Execute(context.Background(), "growthbook_list_sdk_connections", nil)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.NotContains(t, result.Data, "SECRET_")
	assert.Contains(t, result.Data, "sdk-public")
	assert.Contains(t, result.Data, "proxy.example")
}

func TestListDataSources_StripsConnectionParams(t *testing.T) {
	adapter, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"dataSources":[{"id":"ds_1","name":"Warehouse","type":"postgres","params":{"password":"SECRET_PASSWORD"},"connection":{"password":"SECRET_CONN"}}]}`)
	})
	result, err := adapter.Execute(context.Background(), "growthbook_list_data_sources", nil)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.NotContains(t, result.Data, "SECRET_")
	assert.Contains(t, result.Data, "Warehouse")
}

func TestAPIErrors(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		retryable bool
		want      string
	}{
		{"not found", http.StatusNotFound, `{"message":"Could not find a feature with that key"}`, false, "Could not find a feature"},
		{"forbidden", http.StatusForbidden, `{"message":"You do not have permission"}`, false, "403"},
		{"gate", http.StatusUnprocessableEntity, `{"message":"blocked","gates":[{"type":"approval","override":"ignoreWarnings"}]}`, false, "gates"},
		{"conflict", http.StatusConflict, `{"message":"changed","code":"checklist_incomplete"}`, false, "checklist_incomplete"},
		{"rate limit", http.StatusTooManyRequests, `{"message":"slow down"}`, true, "429"},
		{"server", http.StatusBadGateway, `bad gateway`, true, "502"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", "2")
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			})
			result, err := adapter.Execute(context.Background(), "growthbook_get_feature", map[string]any{"feature_id": "f1"})
			if tt.retryable {
				require.Error(t, err)
				assert.True(t, mcp.IsRetryable(err))
				assert.Contains(t, err.Error(), tt.want)
				return
			}
			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Contains(t, result.Data, tt.want)
			assert.NotContains(t, result.Data, testKey)
		})
	}
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/redirected" {
			_, _ = io.WriteString(w, `{"projects":[{"id":"prj_redirected"}]}`)
			return
		}
		http.Redirect(w, r, "/api/v1/redirected", http.StatusFound)
	})
	result, err := adapter.Execute(context.Background(), "growthbook_list_projects", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "redirect")
	assert.NotContains(t, result.Data, "prj_redirected")
	require.Len(t, requests.list(), 1, "the redirect target must not be requested")
	assert.Equal(t, "/api/v1/projects", requests.list()[0].Path)
}

func TestNonJSONSuccessIsError(t *testing.T) {
	adapter, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `<html>login</html>`)
	})
	result, err := adapter.Execute(context.Background(), "growthbook_list_projects", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "not JSON")
}

func TestToggleFeature(t *testing.T) {
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"feature":{"id":"f1","environments":{"production":{"enabled":true,"defaultValue":"false"},"staging":{"enabled":false}},"revision":{"version":3},"dateUpdated":"2026-11-19T00:00:00Z"}}`)
	})
	result, err := adapter.Execute(context.Background(), "growthbook_toggle_feature", map[string]any{"feature_id": "f1", "environment": "production", "enabled": true, "reason": "launch"})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	require.Len(t, requests.list(), 1)
	got := requests.list()[0]
	assert.Equal(t, http.MethodPost, got.Method)
	assert.Equal(t, "/api/v2/features/f1/toggle", got.Path)
	assert.Equal(t, map[string]any{"environments": map[string]any{"production": true}, "reason": "launch"}, got.Body)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &out))
	assert.Equal(t, "updated", out["status"])
	assert.Equal(t, "f1", out["feature_id"])
	assert.Equal(t, "production", out["environment"])
	assert.Equal(t, true, out["enabled"])
	assert.EqualValues(t, 3, out["version"])
}

func TestToggleFeature_Validation(t *testing.T) {
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	})
	for _, args := range []map[string]any{
		{"feature_id": "f1", "environment": "production"},
		{"feature_id": "f1", "enabled": true},
		{"environment": "production", "enabled": true},
		{"feature_id": "f1", "environment": "production", "enabled": "maybe"},
	} {
		result, err := adapter.Execute(context.Background(), "growthbook_toggle_feature", args)
		require.NoError(t, err)
		assert.True(t, result.IsError, "%v", args)
	}
	assert.Empty(t, requests.list())
}

func TestWrite_ConfirmationRequired(t *testing.T) {
	adapter, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "/api/v1/confirmations/cnf_1")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"confirmation":{"id":"cnf_1","status":"pending","url":"https://app.growthbook.io/confirm/cnf_1","actions":[{"action":"feature.toggle","environments":["production"]}]}}`)
	})
	result, err := adapter.Execute(context.Background(), "growthbook_toggle_feature", map[string]any{"feature_id": "f1", "environment": "production", "enabled": false})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &out))
	assert.Equal(t, "pending_confirmation", out["status"])
	assert.Contains(t, out["message"], "not been applied")
	assert.Contains(t, result.Data, "https://app.growthbook.io/confirm/cnf_1")
}

func TestCreateFeature_StartsDisabledEverywhere(t *testing.T) {
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `{"environments":[{"id":"production"},{"id":"staging"}]}`)
			return
		}
		_, _ = io.WriteString(w, `{"feature":{"id":"new-flag","valueType":"boolean","defaultValue":"false","environments":{"production":{"enabled":false},"staging":{"enabled":false}},"revision":{"version":1}}}`)
	})
	result, err := adapter.Execute(context.Background(), "growthbook_create_feature", map[string]any{
		"feature_id": "new-flag", "value_type": "boolean", "default_value": "false", "description": "Gates the new flow",
		"project_id": "prj_1", "owner": "dev@example.com", "tags": "checkout, web",
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	require.Len(t, requests.list(), 2)
	assert.Equal(t, "/api/v1/environments", requests.list()[0].Path)
	got := requests.list()[1]
	assert.Equal(t, http.MethodPost, got.Method)
	assert.Equal(t, "/api/v2/features", got.Path)
	assert.Equal(t, map[string]any{
		"id": "new-flag", "valueType": "boolean", "defaultValue": "false", "description": "Gates the new flow",
		"project": "prj_1", "owner": "dev@example.com", "tags": []any{"checkout", "web"},
		"environments": map[string]any{"production": map[string]any{"enabled": false}, "staging": map[string]any{"enabled": false}},
	}, got.Body)
	assert.Contains(t, result.Data, "new-flag")
}

func TestCreateFeature_Validation(t *testing.T) {
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	})
	for _, args := range []map[string]any{
		{"value_type": "boolean", "default_value": "false"},
		{"feature_id": "f", "default_value": "false"},
		{"feature_id": "f", "value_type": "boolean"},
		{"feature_id": "f", "value_type": "xml", "default_value": "x"},
		{"feature_id": "bad key!", "value_type": "boolean", "default_value": "false"},
	} {
		result, err := adapter.Execute(context.Background(), "growthbook_create_feature", args)
		require.NoError(t, err)
		assert.True(t, result.IsError, "%v", args)
	}
	assert.Empty(t, requests.list())
}

func TestRefreshExperimentResults(t *testing.T) {
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"snapshot":{"id":"snp_1","experiment":"exp_1","status":"running"}}`)
	})
	result, err := adapter.Execute(context.Background(), "growthbook_refresh_experiment_results", map[string]any{"experiment_id": "exp_1", "phase": 1, "dimension": "exp:country"})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	got := requests.list()[0]
	assert.Equal(t, http.MethodPost, got.Method)
	assert.Equal(t, "/api/v1/experiments/exp_1/snapshot", got.Path)
	assert.Equal(t, map[string]any{"phase": float64(1), "dimension": "exp:country"}, got.Body)
	assert.Contains(t, result.Data, "snp_1")
}

func TestRefreshExperimentResults_EmptyBody(t *testing.T) {
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"snapshot":{"id":"snp_1","status":"running"}}`)
	})
	result, err := adapter.Execute(context.Background(), "growthbook_refresh_experiment_results", map[string]any{"experiment_id": "exp_1"})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Equal(t, map[string]any{}, requests.list()[0].Body)
}
