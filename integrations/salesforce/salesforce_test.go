package salesforce

import (
	"context"
	"encoding/json"
	"fmt"
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
	assert.Equal(t, "salesforce", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"access_token": "test_token_123",
		"instance_url": "https://myorg.my.salesforce.com",
	})
	assert.NoError(t, err)
}

func TestConfigure_MissingAccessToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"access_token": "",
		"instance_url": "https://myorg.my.salesforce.com",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_token is required")
}

func TestConfigure_MissingInstanceURL(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"access_token": "test_token_123",
		"instance_url": "",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "instance_url is required")
}

func TestConfigure_TrailingSlash(t *testing.T) {
	s := &salesforce{client: &http.Client{}, apiVersion: "v62.0"}
	err := s.Configure(context.Background(), mcp.Credentials{
		"access_token": "test",
		"instance_url": "https://myorg.my.salesforce.com/",
	})
	assert.NoError(t, err)
	assert.Equal(t, "https://myorg.my.salesforce.com", s.instanceURL)
}

func TestConfigure_CustomAPIVersion(t *testing.T) {
	s := &salesforce{client: &http.Client{}, apiVersion: "v62.0"}
	err := s.Configure(context.Background(), mcp.Credentials{
		"access_token": "test",
		"instance_url": "https://myorg.my.salesforce.com",
		"api_version":  "v59.0",
	})
	assert.NoError(t, err)
	assert.Equal(t, "v59.0", s.apiVersion)
}

func TestTools(t *testing.T) {
	i := New()
	tls := i.Tools()
	assert.NotEmpty(t, tls)

	for _, tool := range tls {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveSalesforcePrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "salesforce_", "tool %s missing salesforce_ prefix", tool.Name)
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
	s := &salesforce{accessToken: "test", instanceURL: "http://localhost", apiVersion: "v62.0", client: &http.Client{}}
	result, err := s.Execute(context.Background(), "salesforce_nonexistent", nil)
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
		_, _ = w.Write([]byte(`{"id":"001xx"}`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "test-token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	data, err := s.get(context.Background(), "/test")
	require.NoError(t, err)
	assert.Contains(t, string(data), "001xx")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`[{"errorCode":"INSUFFICIENT_ACCESS","message":"forbidden"}]`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "bad", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	_, err := s.get(context.Background(), "/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "salesforce API error (403)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	data, err := s.doRequest(context.Background(), "DELETE", "/test", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestDoRequest_Retryable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`[{"errorCode":"REQUEST_LIMIT_EXCEEDED"}]`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	_, err := s.get(context.Background(), "/test")
	assert.Error(t, err)
	assert.True(t, mcp.IsRetryable(err))
}

func TestPost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Acme", body["Name"])
		_, _ = w.Write([]byte(`{"id":"001xx","success":true,"created":true}`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	data, err := s.post(context.Background(), "/test", map[string]string{"Name": "Acme"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "001xx")
}

func TestPatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		w.WriteHeader(204)
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	data, err := s.patch(context.Background(), "/test", map[string]string{"Name": "Updated"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestQueryEncode(t *testing.T) {
	t.Run("with values", func(t *testing.T) {
		result := queryEncode(map[string]string{"key": "val", "empty": ""})
		assert.Contains(t, result, "key=val")
		assert.NotContains(t, result, "empty")
		assert.True(t, result[0] == '?')
	})

	t.Run("all empty", func(t *testing.T) {
		result := queryEncode(map[string]string{"empty": ""})
		assert.Empty(t, result)
	})
}

// --- handler integration tests ---

func TestQuery(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/services/data/v62.0/query")
		assert.Contains(t, r.URL.RawQuery, "q=")
		_, _ = w.Write([]byte(`{"totalSize":1,"done":true,"records":[{"Id":"001xx","Name":"Acme"}]}`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	result, err := s.Execute(context.Background(), "salesforce_query", map[string]any{
		"q": "SELECT Id, Name FROM Account LIMIT 1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Acme")
}

func TestSearch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/services/data/v62.0/search")
		_, _ = w.Write([]byte(`{"searchRecords":[{"Id":"001xx","Name":"Acme"}]}`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	result, err := s.Execute(context.Background(), "salesforce_search", map[string]any{
		"q": "FIND {Acme} IN ALL FIELDS RETURNING Account",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Acme")
}

func TestGetRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/services/data/v62.0/sobjects/Account/001xx")
		_, _ = w.Write([]byte(`{"Id":"001xx","Name":"Acme","attributes":{"type":"Account"}}`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	result, err := s.Execute(context.Background(), "salesforce_get_record", map[string]any{
		"sobject": "Account",
		"id":      "001xx",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Acme")
}

func TestGetRecord_WithFields(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "fields=")
		_, _ = w.Write([]byte(`{"Id":"001xx","Name":"Acme"}`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	result, err := s.Execute(context.Background(), "salesforce_get_record", map[string]any{
		"sobject": "Account",
		"id":      "001xx",
		"fields":  "Id,Name",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCreateRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/services/data/v62.0/sobjects/Account/")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Acme", body["Name"])
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"001xx","success":true,"created":true}`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	result, err := s.Execute(context.Background(), "salesforce_create_record", map[string]any{
		"sobject": "Account",
		"data":    `{"Name":"Acme","Industry":"Technology"}`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "001xx")
}

func TestCreateRecord_InvalidJSON(t *testing.T) {
	s := &salesforce{accessToken: "token", instanceURL: "http://localhost", apiVersion: "v62.0", client: &http.Client{}}
	result, err := s.Execute(context.Background(), "salesforce_create_record", map[string]any{
		"sobject": "Account",
		"data":    "{bad json}",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid JSON for data")
}

func TestDeleteRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/services/data/v62.0/sobjects/Account/001xx")
		w.WriteHeader(204)
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	result, err := s.Execute(context.Background(), "salesforce_delete_record", map[string]any{
		"sobject": "Account",
		"id":      "001xx",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestDescribeGlobal(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/services/data/v62.0/sobjects/")
		_, _ = w.Write([]byte(`{"sobjects":[{"name":"Account","label":"Account"}]}`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	result, err := s.Execute(context.Background(), "salesforce_describe_global", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Account")
}

func TestGetLimits(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/services/data/v62.0/limits")
		_, _ = w.Write([]byte(`{"DailyApiRequests":{"Max":15000,"Remaining":14500}}`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	result, err := s.Execute(context.Background(), "salesforce_get_limits", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "DailyApiRequests")
}

func TestCompositeBatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/services/data/v62.0/composite/batch")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.NotNil(t, body["batchRequests"])
		_, _ = w.Write([]byte(`{"hasErrors":false,"results":[{"statusCode":200}]}`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	result, err := s.Execute(context.Background(), "salesforce_composite_batch", map[string]any{
		"requests": `[{"method":"GET","url":"/services/data/v62.0/sobjects/Account/001xx"}]`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "results")
}

func TestSObjectCollections_UnsupportedMethod(t *testing.T) {
	s := &salesforce{accessToken: "token", instanceURL: "http://localhost", apiVersion: "v62.0", client: &http.Client{}}
	result, err := s.Execute(context.Background(), "salesforce_sobject_collections", map[string]any{
		"method": "PUT",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unsupported method")
}

func TestVer(t *testing.T) {
	s := &salesforce{apiVersion: "v62.0"}
	assert.Equal(t, "/services/data/v62.0", s.ver())
}

func TestRawResult(t *testing.T) {
	data := json.RawMessage(`{"key":"value"}`)
	result, err := mcp.RawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"key":"value"}`, result.Data)
}

func TestErrResult(t *testing.T) {
	result, err := mcp.ErrResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

func TestQueryMore(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/services/data/v62.0/query/01gxx")
		_, _ = w.Write([]byte(`{"totalSize":500,"done":true,"records":[{"Id":"002xx"}]}`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	result, err := s.Execute(context.Background(), "salesforce_query_more", map[string]any{
		"next_url": "/services/data/v62.0/query/01gxx",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "002xx")
}

func TestQueryMore_InvalidPath(t *testing.T) {
	s := &salesforce{accessToken: "token", instanceURL: "http://localhost", apiVersion: "v62.0", client: &http.Client{}}
	result, err := s.Execute(context.Background(), "salesforce_query_more", map[string]any{
		"next_url": "https://evil.com/steal",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "must be a Salesforce-relative path")
}

func TestListRecentlyViewed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/services/data/v62.0/recent")
		_, _ = w.Write([]byte(`[{"attributes":{"type":"Account"},"Id":"001xx","Name":"Acme"}]`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	result, err := s.Execute(context.Background(), "salesforce_list_recently_viewed", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Acme")
}

func TestDescribeSObject(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/services/data/v62.0/sobjects/Account/describe")
		_, _ = w.Write([]byte(`{"name":"Account","fields":[{"name":"Id","type":"id"}]}`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	result, err := s.Execute(context.Background(), "salesforce_describe_sobject", map[string]any{
		"sobject": "Account",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Account")
}

func TestGetRecordByExternalID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/services/data/v62.0/sobjects/Account/External_Id__c/ext-123")
		_, _ = w.Write([]byte(`{"Id":"001xx","Name":"Acme","External_Id__c":"ext-123"}`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	result, err := s.Execute(context.Background(), "salesforce_get_record_by_external_id", map[string]any{
		"sobject": "Account",
		"field":   "External_Id__c",
		"value":   "ext-123",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "ext-123")
}

func TestUpsertByExternalID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Contains(t, r.URL.Path, "/services/data/v62.0/sobjects/Account/External_Id__c/ext-456")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Acme Updated", body["Name"])
		_, _ = w.Write([]byte(`{"id":"001xx","success":true,"created":false}`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	result, err := s.Execute(context.Background(), "salesforce_upsert_by_external_id", map[string]any{
		"sobject": "Account",
		"field":   "External_Id__c",
		"value":   "ext-456",
		"data":    `{"Name":"Acme Updated"}`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "001xx")
}

func TestListAPIVersions(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/services/data/", r.URL.Path)
		_, _ = w.Write([]byte(`[{"version":"62.0","label":"Winter '25","url":"/services/data/v62.0"}]`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	result, err := s.Execute(context.Background(), "salesforce_list_api_versions", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "62.0")
}

func TestUpdateRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Contains(t, r.URL.Path, "/services/data/v62.0/sobjects/Account/001xx")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "New Name", body["Name"])
		w.WriteHeader(204)
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	result, err := s.Execute(context.Background(), "salesforce_update_record", map[string]any{
		"sobject": "Account",
		"id":      "001xx",
		"data":    `{"Name":"New Name"}`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestSObjectCollections_Create(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/services/data/v62.0/composite/sobjects")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.NotNil(t, body["records"])
		_, _ = w.Write([]byte(`[{"id":"001xx","success":true},{"id":"002xx","success":true}]`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	result, err := s.Execute(context.Background(), "salesforce_sobject_collections", map[string]any{
		"method":  "POST",
		"records": `[{"attributes":{"type":"Account"},"Name":"Acme1"},{"attributes":{"type":"Account"},"Name":"Acme2"}]`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "001xx")
}

func TestSObjectCollections_Update(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		_, _ = w.Write([]byte(`[{"id":"001xx","success":true}]`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	result, err := s.Execute(context.Background(), "salesforce_sobject_collections", map[string]any{
		"method":  "PATCH",
		"records": `[{"attributes":{"type":"Account"},"Id":"001xx","Name":"Updated"}]`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestSObjectCollections_Delete(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.RawQuery, "ids=001xx%2C002xx")
		_, _ = w.Write([]byte(`[{"id":"001xx","success":true},{"id":"002xx","success":true}]`))
	}))
	defer ts.Close()

	s := &salesforce{accessToken: "token", instanceURL: ts.URL, apiVersion: "v62.0", client: ts.Client()}
	result, err := s.Execute(context.Background(), "salesforce_sobject_collections", map[string]any{
		"method": "DELETE",
		"ids":    "001xx,002xx",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}
