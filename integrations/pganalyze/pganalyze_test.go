package pganalyze

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
	assert.Equal(t, "pganalyze", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_key": "test-token-123"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAPIKey(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_key": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_key is required")
}

func TestConfigure_EmptyCredentials(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.Error(t, err)
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	p := &pganalyze{client: &http.Client{}}
	err := p.Configure(context.Background(), mcp.Credentials{"api_key": "key", "base_url": "https://pganalyze.example.com/"})
	assert.NoError(t, err)
	assert.Equal(t, "https://pganalyze.example.com/graphql", p.graphqlURL)
}

func TestConfigure_DefaultBaseURL(t *testing.T) {
	p := &pganalyze{client: &http.Client{}}
	err := p.Configure(context.Background(), mcp.Credentials{"api_key": "key"})
	assert.NoError(t, err)
	assert.Equal(t, defaultGraphQLURL, p.graphqlURL)
}

func TestConfigure_BaseURLWithGraphQLSuffix(t *testing.T) {
	p := &pganalyze{client: &http.Client{}}
	err := p.Configure(context.Background(), mcp.Credentials{"api_key": "key", "base_url": "https://app.pganalyze.com/graphql"})
	assert.NoError(t, err)
	assert.Equal(t, "https://app.pganalyze.com/graphql", p.graphqlURL)
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

func TestTools_AllHavePganalyzePrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "pganalyze_", "tool %s missing pganalyze_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[string]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	p := &pganalyze{apiKey: "key", graphqlURL: "http://localhost", client: &http.Client{}}
	result, err := p.Execute(context.Background(), "pganalyze_nonexistent", nil)
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
	toolNames := make(map[string]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

// --- GQL helper tests ---

func TestGQL_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Token test-key", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"getServers": []map[string]string{{"id": "srv-123"}}},
		})
	}))
	defer ts.Close()

	p := &pganalyze{apiKey: "test-key", graphqlURL: ts.URL, client: ts.Client()}
	data, err := p.gql(context.Background(), `{ getServers { id } }`, nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "srv-123")
}

func TestGQL_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer ts.Close()

	p := &pganalyze{apiKey: "bad-key", graphqlURL: ts.URL, client: ts.Client()}
	_, err := p.gql(context.Background(), `{ bad }`, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pganalyze API error (401)")
}

func TestGQL_GraphQLErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"message": "field not found"}},
		})
	}))
	defer ts.Close()

	p := &pganalyze{apiKey: "test-key", graphqlURL: ts.URL, client: ts.Client()}
	_, err := p.gql(context.Background(), `{ bad }`, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "field not found")
}

// --- Arg helper tests ---

func TestArgStr(t *testing.T) {
	assert.Equal(t, "val", argStr(map[string]any{"k": "val"}, "k"))
	assert.Empty(t, argStr(map[string]any{}, "k"))
}

func TestArgInt(t *testing.T) {
	assert.Equal(t, 42, argInt(map[string]any{"n": float64(42)}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": 42}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": "42"}, "n"))
	assert.Equal(t, 0, argInt(map[string]any{}, "n"))
}

func TestArgBool(t *testing.T) {
	assert.True(t, argBool(map[string]any{"b": true}, "b"))
	assert.False(t, argBool(map[string]any{"b": false}, "b"))
	assert.True(t, argBool(map[string]any{"b": "true"}, "b"))
	assert.False(t, argBool(map[string]any{}, "b"))
}

// --- Result helper tests ---

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

func TestJsonResult(t *testing.T) {
	result, err := mcp.JSONResult(map[string]string{"key": "value"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"key":"value"`)
}

func TestPlainTextKeys(t *testing.T) {
	p := New()
	ptc, ok := p.(mcp.PlainTextCredentials)
	require.True(t, ok, "pganalyze must implement PlainTextCredentials")
	keys := ptc.PlainTextKeys()
	assert.Contains(t, keys, "base_url")
	assert.Contains(t, keys, "organization_slug")
}

func TestConfigure_OrganizationSlug(t *testing.T) {
	p := &pganalyze{client: &http.Client{}}
	err := p.Configure(context.Background(), mcp.Credentials{"api_key": "key", "organization_slug": "my-org"})
	assert.NoError(t, err)
	assert.Equal(t, "my-org", p.organizationSlug)
}

func TestOrgSlug_ArgOverridesConfig(t *testing.T) {
	p := &pganalyze{organizationSlug: "default-org"}
	assert.Equal(t, "override-org", p.orgSlug(map[string]any{"organization_slug": "override-org"}))
}

func TestOrgSlug_FallsBackToConfig(t *testing.T) {
	p := &pganalyze{organizationSlug: "default-org"}
	assert.Equal(t, "default-org", p.orgSlug(map[string]any{}))
}

func TestOrgSlug_EmptyWhenNeitherSet(t *testing.T) {
	p := &pganalyze{}
	assert.Empty(t, p.orgSlug(map[string]any{}))
}
