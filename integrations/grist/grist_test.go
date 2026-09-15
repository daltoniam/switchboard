package grist

import (
	"context"
	"encoding/json"
	"io"
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
	assert.Equal(t, "grist", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_key": "key"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAPIKey(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_key is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	g := &grist{client: &http.Client{}, baseURL: defaultBaseURL}
	err := g.Configure(context.Background(), mcp.Credentials{
		"api_key":  "key",
		"base_url": "https://grist.example.com/api/",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://grist.example.com", g.baseURL)
}

func TestTools(t *testing.T) {
	i := New()
	tls := i.Tools()
	assert.NotEmpty(t, tls)
	for _, tool := range tls {
		assert.NotEmpty(t, tool.Name)
		assert.NotEmpty(t, tool.Description)
	}
}

func TestTools_AllHaveGristPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "grist_")
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

func TestTools_EntryPointHasStartHere(t *testing.T) {
	i := New()
	found := false
	for _, tool := range i.Tools() {
		if tool.Name == "grist_list_orgs" {
			assert.Contains(t, tool.Description, "Start here")
			found = true
		}
	}
	assert.True(t, found)
}

func TestExecute_UnknownTool(t *testing.T) {
	g := &grist{apiKey: "key", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "grist_nonexistent", nil)
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

func configured(ts *httptest.Server) *grist {
	return &grist{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
}

func TestDoRequest_Bearer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer key", r.Header.Get("Authorization"))
		assert.Equal(t, "/api/orgs", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":1,"name":"Acme"}]`))
	}))
	defer ts.Close()

	data, err := configured(ts).get(context.Background(), "/api/orgs")
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":1`)
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"Unauthorized"}`))
	}))
	defer ts.Close()

	_, err := configured(ts).get(context.Background(), "/api/orgs")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "grist API error (401)")
}

func TestDoRequest_Retryable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`rate limited`))
	}))
	defer ts.Close()

	_, err := configured(ts).get(context.Background(), "/api/orgs")
	require.Error(t, err)
	var re *mcp.RetryableError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, 429, re.StatusCode)
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	data, err := configured(ts).doRequest(context.Background(), "DELETE", "/api/docs/doc1", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestHealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/profile/user", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":1,"email":"ada@acme.com"}`))
	}))
	defer ts.Close()

	g := configured(ts)
	assert.True(t, g.Healthy(context.Background()))

	g = &grist{}
	assert.False(t, g.Healthy(context.Background()))
}

func TestListOrgs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/orgs", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":1,"name":"Acme"}]`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "grist_list_orgs", nil)
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Acme")
}

func TestListWorkspaces(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/orgs/acme/workspaces", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":10,"name":"Ops"}]`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "grist_list_workspaces", map[string]any{"org_id": "acme"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Ops")
}

func TestCreateDoc(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/workspaces/10/docs", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), `"name":"CRM"`)
		_, _ = w.Write([]byte(`"doc1"`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "grist_create_doc", map[string]any{
		"workspace_id": 10,
		"name":         "CRM",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "doc1")
}

func TestListTables_Unwraps(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/docs/doc1/tables", r.URL.Path)
		assert.Equal(t, "columns", r.URL.Query().Get("expand"))
		_, _ = w.Write([]byte(`{"tables":[{"id":"People","fields":{"tableRef":1}}]}`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "grist_list_tables", map[string]any{
		"doc_id": "doc1",
		"expand": true,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, `"id":"People"`)
	assert.NotContains(t, result.Data, `"tables"`)
}

func TestListRecords_DefaultLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/docs/doc1/tables/People/records", r.URL.Path)
		assert.Equal(t, "50", r.URL.Query().Get("limit"))
		assert.Equal(t, `{"Status":["Open"]}`, r.URL.Query().Get("filter"))
		assert.Equal(t, "Name,-Age", r.URL.Query().Get("sort"))
		_, _ = w.Write([]byte(`{"records":[{"id":1,"fields":{"Name":"Ada"}}]}`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "grist_list_records", map[string]any{
		"doc_id":   "doc1",
		"table_id": "People",
		"filter":   `{"Status":["Open"]}`,
		"sort":     "Name,-Age",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, `"id":1`)
	assert.NotContains(t, result.Data, `"records"`)
}

func TestListRecords_NativeFilter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, `{"Status":["Open"]}`, r.URL.Query().Get("filter"))
		_, _ = w.Write([]byte(`{"records":[{"id":1,"fields":{"Name":"Ada"}}]}`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "grist_list_records", map[string]any{
		"doc_id":   "doc1",
		"table_id": "People",
		"filter":   map[string]any{"Status": []any{"Open"}},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestDeleteRecords_JSONArrayString(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ids []int
		require.NoError(t, json.NewDecoder(r.Body).Decode(&ids))
		assert.Equal(t, []int{1, 2, 3}, ids)
		w.WriteHeader(200)
		_, _ = w.Write(nil)
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "grist_delete_records", map[string]any{
		"doc_id":     "doc1",
		"table_id":   "People",
		"record_ids": "[1,2,3]",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestListRecords_NegativeLimitDefaults(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "50", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"records":[{"id":1,"fields":{"Name":"Ada"}}]}`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "grist_list_records", map[string]any{
		"doc_id": "doc1", "table_id": "People", "limit": -1,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestDeleteRecords_NativeIDs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ids []int
		require.NoError(t, json.NewDecoder(r.Body).Decode(&ids))
		assert.Equal(t, []int{1, 2, 3}, ids)
		w.WriteHeader(200)
		_, _ = w.Write(nil)
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "grist_delete_records", map[string]any{
		"doc_id":     "doc1",
		"table_id":   "People",
		"record_ids": []any{1, 2, 3},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestAddRecords_NativeArray(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		records, ok := body["records"].([]any)
		require.True(t, ok)
		require.Len(t, records, 1)
		_, _ = w.Write([]byte(`{"records":[{"id":8}]}`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "grist_add_records", map[string]any{
		"doc_id":   "doc1",
		"table_id": "People",
		"records":  []any{map[string]any{"fields": map[string]any{"Name": "Ada"}}},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, `"id":8`)
}

func TestListRecords_RejectsZeroWithoutUnlimited(t *testing.T) {
	g := &grist{apiKey: "key", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "grist_list_records", map[string]any{
		"doc_id": "doc1", "table_id": "People", "limit": 0,
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unlimited")
}

func TestListRecords_ZeroWithUnlimited(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "0", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"records":[{"id":1,"fields":{"Name":"Ada"}}]}`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "grist_list_records", map[string]any{
		"doc_id": "doc1", "table_id": "People", "limit": 0, "unlimited": true,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestListRecords_ClampsHugeLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "500", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"records":[{"id":1,"fields":{"Name":"Ada"}}]}`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "grist_list_records", map[string]any{
		"doc_id": "doc1", "table_id": "People", "limit": 5000,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestAddRecords(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/docs/doc1/tables/People/records", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Contains(t, body, "records")
		_, _ = w.Write([]byte(`{"records":[{"id":7}]}`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "grist_add_records", map[string]any{
		"doc_id":   "doc1",
		"table_id": "People",
		"records":  `[{"fields":{"Name":"Ada"}}]`,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, `"id":7`)
}

func TestDeleteRecords(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/docs/doc1/tables/People/records/delete", r.URL.Path)
		var ids []int
		require.NoError(t, json.NewDecoder(r.Body).Decode(&ids))
		assert.Equal(t, []int{1, 2, 3}, ids)
		w.WriteHeader(200)
		_, _ = w.Write(nil)
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "grist_delete_records", map[string]any{
		"doc_id":     "doc1",
		"table_id":   "People",
		"record_ids": "1,2,3",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestQuerySQL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/docs/doc1/sql", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "SELECT * FROM People WHERE Age >= ?", body["sql"])
		_, _ = w.Write([]byte(`{"statement":"SELECT * FROM People WHERE Age >= ?","records":[{"fields":{"id":1,"Name":"Ada"}}]}`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "grist_query_sql", map[string]any{
		"doc_id": "doc1",
		"sql":    "SELECT * FROM People WHERE Age >= ?",
		"args":   `[30]`,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Ada")
	assert.NotContains(t, result.Data, `"statement"`)
}

func TestUpsertRecords_QueryFlags(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "all", r.URL.Query().Get("onmany"))
		assert.Equal(t, "true", r.URL.Query().Get("noadd"))
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer ts.Close()

	result, err := configured(ts).Execute(context.Background(), "grist_upsert_records", map[string]any{
		"doc_id":   "doc1",
		"table_id": "People",
		"records":  `[{"require":{"Email":"a@b.com"},"fields":{"Age":37}}]`,
		"onmany":   "all",
		"noadd":    true,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestListRecords_MissingTable(t *testing.T) {
	g := &grist{apiKey: "key", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "grist_list_records", map[string]any{"doc_id": "doc1"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "table_id is required")
}

func TestAddRecords_InvalidJSON(t *testing.T) {
	g := &grist{apiKey: "key", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "grist_add_records", map[string]any{
		"doc_id": "doc1", "table_id": "People", "records": "not-json",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid JSON")
}

func TestMaxBytes_Unknown(t *testing.T) {
	g := &grist{}
	_, ok := g.MaxBytes("grist_nonexistent")
	assert.False(t, ok)
}

func TestPlainTextAndPlaceholders(t *testing.T) {
	g := New().(*grist)
	assert.Equal(t, []string{"base_url"}, g.PlainTextKeys())
	assert.Equal(t, []string{"base_url"}, g.OptionalKeys())
	assert.Contains(t, g.Placeholders()["base_url"], "docs.getgrist.com")
}
