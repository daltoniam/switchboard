package netsuite

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "netsuite", i.Name())
}

func TestConfigure_OAuth2(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"account_id":   "1234567_SB1",
		"access_token": "tok",
	})
	require.NoError(t, err)
	n := i.(*netsuite)
	assert.Equal(t, "https://1234567-sb1.suitetalk.api.netsuite.com", n.baseURL)
}

func TestConfigure_TBA(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"account_id":      "1234567",
		"consumer_key":    "ck",
		"consumer_secret": "cs",
		"token_id":        "ti",
		"token_secret":    "ts",
	})
	assert.NoError(t, err)
}

func TestConfigure_MissingAccountID(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"access_token": "tok"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "account_id is required")
}

func TestConfigure_MissingAuth(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"account_id": "123"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_token")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	n := &netsuite{client: &http.Client{}}
	err := n.Configure(context.Background(), mcp.Credentials{
		"account_id":   "123",
		"access_token": "tok",
		"base_url":     "https://custom.example.com/",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://custom.example.com", n.baseURL)
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

func TestTools_AllHaveNetsuitePrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "netsuite_")
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
		if tool.Name == "netsuite_suiteql" {
			assert.Contains(t, tool.Description, "Start here")
			found = true
		}
	}
	assert.True(t, found)
}

func TestExecute_UnknownTool(t *testing.T) {
	n := &netsuite{accessToken: "t", accountID: "1", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := n.Execute(context.Background(), "netsuite_nonexistent", nil)
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

func TestDoRequest_OAuth2Bearer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "/services/rest/record/v1/customer", r.URL.Path)
		_, _ = w.Write([]byte(`{"items":[{"id":"1"}]}`))
	}))
	defer ts.Close()

	n := &netsuite{accessToken: "test-token", accountID: "1", client: ts.Client(), baseURL: ts.URL}
	data, err := n.get(context.Background(), "/services/rest/record/v1/customer")
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"1"`)
}

func TestDoRequest_TBAHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		assert.True(t, strings.HasPrefix(auth, "OAuth "))
		assert.Contains(t, auth, `realm="1234567"`)
		assert.Contains(t, auth, "oauth_signature_method=\"HMAC-SHA256\"")
		assert.Contains(t, auth, "oauth_consumer_key=")
		assert.Contains(t, auth, "oauth_token=")
		assert.Contains(t, auth, "oauth_signature=")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	n := &netsuite{
		accountID:      "1234567",
		consumerKey:    "ck",
		consumerSecret: "cs",
		tokenID:        "ti",
		tokenSecret:    "ts",
		client:         ts.Client(),
		baseURL:        ts.URL,
	}
	data, err := n.get(context.Background(), "/services/rest/record/v1/customer")
	require.NoError(t, err)
	assert.Contains(t, string(data), "ok")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"title":"Unauthorized"}`))
	}))
	defer ts.Close()

	n := &netsuite{accessToken: "bad", accountID: "1", client: ts.Client(), baseURL: ts.URL}
	_, err := n.get(context.Background(), "/services/rest/record/v1/customer")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "netsuite API error (401)")
}

func TestDoRequest_Retryable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "3")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`concurrency`))
	}))
	defer ts.Close()

	n := &netsuite{accessToken: "t", accountID: "1", client: ts.Client(), baseURL: ts.URL}
	_, err := n.get(context.Background(), "/x")
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

	n := &netsuite{accessToken: "t", accountID: "1", client: ts.Client(), baseURL: ts.URL}
	data, err := n.doRequest(context.Background(), "DELETE", "/x", nil, nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestSuiteQL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/services/rest/query/v1/suiteql", r.URL.Path)
		assert.Equal(t, "transient", r.Header.Get("Prefer"))
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Contains(t, body["q"], "customer")
		_, _ = w.Write([]byte(`{"items":[{"id":"9"}],"hasMore":false}`))
	}))
	defer ts.Close()

	n := &netsuite{accessToken: "t", accountID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := n.Execute(context.Background(), "netsuite_suiteql", map[string]any{
		"q": "SELECT id FROM customer",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, `"id":"9"`)
}

func TestListCustomers(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/services/rest/record/v1/customer", r.URL.Path)
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"items":[{"id":"1","companyName":"Acme"}],"hasMore":false}`))
	}))
	defer ts.Close()

	n := &netsuite{accessToken: "t", accountID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := n.Execute(context.Background(), "netsuite_list_customers", map[string]any{
		"limit": "10",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Acme")
}

func TestGetCustomer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/services/rest/record/v1/customer/42", r.URL.Path)
		assert.Equal(t, "true", r.URL.Query().Get("expandSubResources"))
		_, _ = w.Write([]byte(`{"id":"42","companyName":"Acme"}`))
	}))
	defer ts.Close()

	n := &netsuite{accessToken: "t", accountID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := n.Execute(context.Background(), "netsuite_get_customer", map[string]any{
		"id":     "42",
		"expand": "true",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Acme")
}

func TestCreateRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/services/rest/record/v1/customer", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Acme", body["companyName"])
		w.WriteHeader(204)
	}))
	defer ts.Close()

	n := &netsuite{accessToken: "t", accountID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := n.Execute(context.Background(), "netsuite_create_record", map[string]any{
		"record_type": "customer",
		"data":        `{"companyName":"Acme"}`,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestMetadataCatalog_SelectPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/services/rest/record/v1/metadata-catalog/customer", r.URL.Path)
		assert.Equal(t, "application/schema+json", r.Header.Get("Accept"))
		_, _ = w.Write([]byte(`{"title":"customer"}`))
	}))
	defer ts.Close()

	n := &netsuite{accessToken: "t", accountID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := n.Execute(context.Background(), "netsuite_metadata_catalog", map[string]any{
		"select": "customer",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "customer")
}

func TestInvalidRecordType(t *testing.T) {
	n := &netsuite{accessToken: "t", accountID: "1", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := n.Execute(context.Background(), "netsuite_list_records", map[string]any{
		"record_type": "../evil",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid record_type")
}

func TestPctEncode(t *testing.T) {
	assert.Equal(t, "abc-._~XYZ", pctEncode("abc-._~XYZ"))
	assert.Equal(t, "a%20b", pctEncode("a b"))
	assert.Equal(t, "%2B", pctEncode("+"))
}

func TestHealthy_Unconfigured(t *testing.T) {
	n := &netsuite{client: &http.Client{}, baseURL: "http://127.0.0.1:1"}
	assert.False(t, n.Healthy(context.Background()))
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
