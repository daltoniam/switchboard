package ramp

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
	assert.Equal(t, "ramp", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"access_token": "test-token"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAccessToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"access_token": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_token is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	r := &ramp{client: &http.Client{}, baseURL: "https://api.ramp.com"}
	err := r.Configure(context.Background(), mcp.Credentials{
		"access_token": "test",
		"base_url":     "https://demo-api.ramp.com/",
	})
	assert.NoError(t, err)
	assert.Equal(t, "https://demo-api.ramp.com", r.baseURL)
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

func TestTools_AllHaveRampPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "ramp_", "tool %s missing ramp_ prefix", tool.Name)
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
		if tool.Name == "ramp_list_transactions" {
			assert.Contains(t, tool.Description, "Start here")
			found = true
		}
	}
	assert.True(t, found)
}

func TestExecute_UnknownTool(t *testing.T) {
	r := &ramp{accessToken: "test", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := r.Execute(context.Background(), "ramp_nonexistent", nil)
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

func TestDoRequest_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "/developer/v1/users", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"u1"}]}`))
	}))
	defer ts.Close()

	r := &ramp{accessToken: "test-token", client: ts.Client(), baseURL: ts.URL}
	data, err := r.get(context.Background(), "/developer/v1/users")
	require.NoError(t, err)
	assert.Contains(t, string(data), "u1")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer ts.Close()

	r := &ramp{accessToken: "bad-token", client: ts.Client(), baseURL: ts.URL}
	_, err := r.get(context.Background(), "/developer/v1/users")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ramp API error (401)")
}

func TestDoRequest_Retryable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`rate limited`))
	}))
	defer ts.Close()

	r := &ramp{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	_, err := r.get(context.Background(), "/developer/v1/users")
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

	r := &ramp{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	data, err := r.doRequest(context.Background(), "DELETE", "/test", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestListTransactions(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/developer/v1/transactions", r.URL.Path)
		assert.Equal(t, "u1", r.URL.Query().Get("user_id"))
		assert.Equal(t, "20", r.URL.Query().Get("page_size"))
		_, _ = w.Write([]byte(`{"data":[{"id":"t1","amount":12.5}],"page":{"next":null}}`))
	}))
	defer ts.Close()

	r := &ramp{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := r.Execute(context.Background(), "ramp_list_transactions", map[string]any{
		"user_id":   "u1",
		"page_size": "20",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "t1")
}

func TestGetTransaction(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/developer/v1/transactions/tx-1", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"tx-1","merchant_name":"Acme"}`))
	}))
	defer ts.Close()

	r := &ramp{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := r.Execute(context.Background(), "ramp_get_transaction", map[string]any{
		"transaction_id": "tx-1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Acme")
}

func TestSetTransactionMemo(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/developer/v1/memos/tx-1", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Client dinner", body["memo"])
		_, _ = w.Write([]byte(`{"id":"tx-1","memo":"Client dinner"}`))
	}))
	defer ts.Close()

	r := &ramp{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := r.Execute(context.Background(), "ramp_set_transaction_memo", map[string]any{
		"transaction_id": "tx-1",
		"memo":           "Client dinner",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Client dinner")
}

func TestUpdateTransactionSplits(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/developer/v1/transactions/tx-1", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		items, ok := body["line_items"].([]any)
		require.True(t, ok)
		assert.Len(t, items, 1)
		_, _ = w.Write([]byte(`{"id":"tx-1"}`))
	}))
	defer ts.Close()

	r := &ramp{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := r.Execute(context.Background(), "ramp_update_transaction_splits", map[string]any{
		"transaction_id": "tx-1",
		"line_items":     `[{"amount":4000,"memo":"Case-1"}]`,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestListTransactions_NextURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "cursor-abc", r.URL.Query().Get("start"))
		assert.Equal(t, "50", r.URL.Query().Get("page_size"))
		_, _ = w.Write([]byte(`{"data":[],"page":{"next":null}}`))
	}))
	defer ts.Close()

	r := &ramp{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := r.Execute(context.Background(), "ramp_list_transactions", map[string]any{
		"next": ts.URL + "/developer/v1/transactions?start=cursor-abc&page_size=50",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestHealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/developer/v1/users")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	r := &ramp{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	assert.True(t, r.Healthy(context.Background()))
}

func TestHealthy_Unconfigured(t *testing.T) {
	r := &ramp{client: &http.Client{}, baseURL: "http://127.0.0.1:1"}
	assert.False(t, r.Healthy(context.Background()))
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
