package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "cloudflare", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_token": "test-token"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAPIToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_token": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_token is required")
}

func TestConfigure_WithAccountID(t *testing.T) {
	c := &cloudflare{client: &http.Client{}, baseURL: "https://api.cloudflare.com/client/v4"}
	err := c.Configure(context.Background(), mcp.Credentials{
		"api_token":  "test",
		"account_id": "abc123",
	})
	assert.NoError(t, err)
	assert.Equal(t, "abc123", c.accountID)
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	c := &cloudflare{client: &http.Client{}, baseURL: "https://api.cloudflare.com/client/v4"}
	err := c.Configure(context.Background(), mcp.Credentials{
		"api_token": "test",
		"base_url":  "https://custom.cloudflare.com/v4/",
	})
	assert.NoError(t, err)
	assert.Equal(t, "https://custom.cloudflare.com/v4", c.baseURL)
}

func TestTools(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.NotEmpty(t, tools)

	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveCloudflarePrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "cloudflare_", "tool %s missing cloudflare_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	c := &cloudflare{apiToken: "test", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := c.Execute(context.Background(), "cloudflare_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	i := New()
	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

// --- HTTP helper tests ---

func TestDoRequest_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":{"id":"abc"}}`))
	}))
	defer ts.Close()

	c := &cloudflare{apiToken: "test-token", client: ts.Client(), baseURL: ts.URL}
	data, err := c.get(context.Background(), "/user/tokens/verify")
	require.NoError(t, err)
	assert.Contains(t, string(data), "abc")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":9109,"message":"Invalid access token"}]}`))
	}))
	defer ts.Close()

	c := &cloudflare{apiToken: "bad-token", client: ts.Client(), baseURL: ts.URL}
	_, err := c.get(context.Background(), "/zones")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cloudflare API error (403)")
}

func TestDoRequest_RateLimited(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":10000,"message":"Rate limited"}]}`))
	}))
	defer ts.Close()

	c := &cloudflare{apiToken: "token", client: ts.Client(), baseURL: ts.URL}
	_, err := c.get(context.Background(), "/zones")
	assert.Error(t, err)
	assert.True(t, mcp.IsRetryable(err))
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	c := &cloudflare{apiToken: "token", client: ts.Client(), baseURL: ts.URL}
	data, err := c.doRequest(context.Background(), "DELETE", "/test", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestPost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.NotEmpty(t, body)
		_, _ = w.Write([]byte(`{"success":true,"result":{"id":"new-zone"}}`))
	}))
	defer ts.Close()

	c := &cloudflare{apiToken: "token", client: ts.Client(), baseURL: ts.URL}
	data, err := c.post(context.Background(), "/zones", map[string]string{"name": "example.com"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "new-zone")
}

func TestPatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		_, _ = w.Write([]byte(`{"success":true,"result":{"paused":true}}`))
	}))
	defer ts.Close()

	c := &cloudflare{apiToken: "token", client: ts.Client(), baseURL: ts.URL}
	data, err := c.patch(context.Background(), "/zones/abc", map[string]any{"paused": true})
	require.NoError(t, err)
	assert.Contains(t, string(data), "paused")
}

// --- Helper tests ---

func TestQueryEncode(t *testing.T) {
	t.Run("with values", func(t *testing.T) {
		result := queryEncode(map[string]string{"name": "example.com", "empty": ""})
		assert.Contains(t, result, "name=example.com")
		assert.NotContains(t, result, "empty")
		assert.True(t, result[0] == '?')
	})

	t.Run("all empty", func(t *testing.T) {
		result := queryEncode(map[string]string{"empty": ""})
		assert.Empty(t, result)
	})
}

func TestAcctID(t *testing.T) {
	c := &cloudflare{accountID: "default-acct"}

	v, err := c.acctID(map[string]any{})
	assert.NoError(t, err)
	assert.Equal(t, "default-acct", v)

	v, err = c.acctID(map[string]any{"account_id": "override-acct"})
	assert.NoError(t, err)
	assert.Equal(t, "override-acct", v)
}

func TestAcctID_MissingReturnsError(t *testing.T) {
	c := &cloudflare{}
	_, err := c.acctID(map[string]any{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "account_id is required")
}

func TestSplitCSV(t *testing.T) {
	assert.Nil(t, splitCSV(""))
	assert.Equal(t, []string{"a", "b", "c"}, splitCSV("a,b,c"))
	assert.Equal(t, []string{"a", "b"}, splitCSV("a, b"))
}

func TestPlainTextKeys(t *testing.T) {
	c := &cloudflare{}
	assert.Equal(t, []string{"account_id"}, c.PlainTextKeys())
}

func TestOptionalKeys(t *testing.T) {
	c := &cloudflare{}
	assert.Equal(t, []string{"account_id", "base_url"}, c.OptionalKeys())
}

// --- Handler integration tests ---

func newTestClient(t *testing.T, handler http.HandlerFunc) (*cloudflare, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	c := &cloudflare{apiToken: "test-token", accountID: "test-acct", client: ts.Client(), baseURL: ts.URL}
	return c, ts
}

func TestListZones(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/zones")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"z1","name":"example.com"}]}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_list_zones", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "example.com")
}

func TestGetZone(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/zones/z1")
		_, _ = w.Write([]byte(`{"success":true,"result":{"id":"z1","name":"example.com","status":"active"}}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_get_zone", map[string]any{"zone_id": "z1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "active")
}

func TestCreateDNSRecord(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/zones/z1/dns_records")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "A", body["type"])
		assert.Equal(t, "sub.example.com", body["name"])
		_, _ = w.Write([]byte(`{"success":true,"result":{"id":"r1","type":"A","name":"sub.example.com"}}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_create_dns_record", map[string]any{
		"zone_id": "z1", "type": "A", "name": "sub.example.com", "content": "1.2.3.4",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "sub.example.com")
}

func TestDeleteDNSRecord(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/zones/z1/dns_records/r1")
		_, _ = w.Write([]byte(`{"success":true,"result":{"id":"r1"}}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_delete_dns_record", map[string]any{
		"zone_id": "z1", "record_id": "r1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListWorkers(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/workers/scripts")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"my-worker"}]}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_list_workers", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "my-worker")
}

func TestListPagesProjects(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/pages/projects")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"name":"my-site"}]}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_list_pages_projects", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "my-site")
}

func TestListR2Buckets(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/r2/buckets")
		_, _ = w.Write([]byte(`{"success":true,"buckets":[{"name":"my-bucket"}]}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_list_r2_buckets", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "my-bucket")
}

func TestListKVNamespaces(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/storage/kv/namespaces")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"ns1","title":"MY_KV"}]}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_list_kv_namespaces", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "MY_KV")
}

func TestGetKVValue_WrapsRawResponse(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/storage/kv/namespaces/ns1/values/mykey")
		_, _ = w.Write([]byte(`hello world`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_get_kv_value", map[string]any{
		"namespace_id": "ns1", "key_name": "mykey",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"value":"hello world"`)
}

func TestPutKVValue_SendsRawBody(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "text/plain", r.Header.Get("Content-Type"))
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/storage/kv/namespaces/ns1/values/mykey")
		body := make([]byte, 256)
		n, _ := r.Body.Read(body)
		assert.Equal(t, "hello world", string(body[:n]))
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_put_kv_value", map[string]any{
		"namespace_id": "ns1", "key_name": "mykey", "value": "hello world",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestAccountScopedTool_MissingAccountID(t *testing.T) {
	c := &cloudflare{apiToken: "test", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := c.Execute(context.Background(), "cloudflare_list_workers", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "account_id is required")
}

func TestQueryD1Database(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/d1/database/db1/query")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "SELECT * FROM users", body["sql"])
		_, _ = w.Write([]byte(`{"success":true,"result":[{"results":[{"id":1}]}]}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_query_d1_database", map[string]any{
		"database_id": "db1", "sql": "SELECT * FROM users",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestPurgeCache(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/zones/z1/purge_cache")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, true, body["purge_everything"])
		_, _ = w.Write([]byte(`{"success":true,"result":{"id":"z1"}}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_purge_cache", map[string]any{
		"zone_id": "z1", "purge_everything": true,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestPurgeCache_EmptyBodyReturnsError(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach server")
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_purge_cache", map[string]any{
		"zone_id": "z1",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "at least one of")
}

func TestListLoadBalancers(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/zones/z1/load_balancers")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"lb1","name":"my-lb"}]}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_list_load_balancers", map[string]any{
		"zone_id": "z1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "my-lb")
}

func TestGetZoneAnalytics(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/zones/z1/analytics/dashboard")
		_, _ = w.Write([]byte(`{"success":true,"result":{"totals":{"requests":{"all":1000}}}}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_get_zone_analytics", map[string]any{
		"zone_id": "z1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "1000")
}

func TestListAccounts(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"a1","name":"My Account"}]}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_list_accounts", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "My Account")
}

func TestAccountIDOverride(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/accounts/override-acct/workers/scripts")
		_, _ = w.Write([]byte(`{"success":true,"result":[]}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_list_workers", map[string]any{
		"account_id": "override-acct",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestRollbackPagesDeployment(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/pages/projects/my-site/deployments/dep1/rollback")
		_, _ = w.Write([]byte(`{"success":true,"result":{"id":"dep1"}}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_rollback_pages_deployment", map[string]any{
		"project_name": "my-site", "deployment_id": "dep1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}
