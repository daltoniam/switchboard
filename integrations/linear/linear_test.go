package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	assert.Equal(t, "linear", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_key": "lin_api_test123"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAPIKey(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_key": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_key or mcp_access_token is required")
}

func TestConfigure_EmptyCredentials(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.Error(t, err)
}

func TestNew_WithMCPServerURL(t *testing.T) {
	i := New("https://mcp.linear.app")
	require.NotNil(t, i)
	assert.Equal(t, "linear", i.Name())
	assert.Equal(t, "https://mcp.linear.app", MCPServerURL(i))
}

func TestNew_WithoutMCPServerURL(t *testing.T) {
	i := New()
	assert.Equal(t, "", MCPServerURL(i))
}

func TestIsRemoteMCP_APIKeyMode(t *testing.T) {
	i := New("https://mcp.linear.app")
	_ = i.Configure(context.Background(), mcp.Credentials{"api_key": "lin_api_test"})
	assert.False(t, IsRemoteMCP(i))
}

func TestIsRemoteMCP_NonLinear(t *testing.T) {
	assert.False(t, IsRemoteMCP(&mockIntegration{}))
	assert.Equal(t, "", MCPServerURL(&mockIntegration{}))
}

type mockIntegration struct{}

func (m *mockIntegration) Name() string                                         { return "mock" }
func (m *mockIntegration) Configure(_ context.Context, _ mcp.Credentials) error { return nil }
func (m *mockIntegration) Tools() []mcp.ToolDefinition                          { return nil }
func (m *mockIntegration) Execute(context.Context, string, map[string]any) (*mcp.ToolResult, error) {
	return nil, nil
}
func (m *mockIntegration) Healthy(context.Context) bool { return false }

func TestTools(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.NotEmpty(t, tools)

	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveLinearPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "linear_", "tool %s missing linear_ prefix", tool.Name)
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
	l := &linear{apiKey: "test", client: &http.Client{}}
	result, err := l.Execute(context.Background(), "linear_nonexistent", nil)
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

// --- GQL helper test ---

// testGQL calls the Linear GraphQL endpoint against a custom URL.
func testGQL(url string, l *linear, query string, variables map[string]any) (json.RawMessage, error) {
	body := map[string]any{"query": query}
	if variables != nil {
		body["variables"] = variables
	}
	jsonBody, _ := json.Marshal(body)

	resp, err := l.client.Post(url, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("linear API error (%d): %s", resp.StatusCode, string(data))
	}

	var gqlResp gqlResponse
	if err := json.Unmarshal(data, &gqlResp); err != nil {
		return data, nil
	}
	if len(gqlResp.Errors) > 0 {
		msgs := make([]string, len(gqlResp.Errors))
		for i, e := range gqlResp.Errors {
			msgs[i] = e.String()
		}
		return nil, fmt.Errorf("graphql errors: %s", strings.Join(msgs, "; "))
	}
	return gqlResp.Data, nil
}

func TestGQL_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"viewer": map[string]string{"id": "user-123"}},
		})
	}))
	defer ts.Close()

	l := &linear{apiKey: "test-key", client: ts.Client()}
	data, err := testGQL(ts.URL, l, `{ viewer { id } }`, nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "user-123")
}

func TestGQL_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer ts.Close()

	l := &linear{apiKey: "test-key", client: ts.Client()}
	_, err := testGQL(ts.URL, l, `{ bad }`, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "linear API error (400)")
}

func TestGQL_GraphQLErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"message": "field not found"}},
		})
	}))
	defer ts.Close()

	l := &linear{apiKey: "test-key", client: ts.Client()}
	_, err := testGQL(ts.URL, l, `{ bad }`, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "field not found")
}

func TestGQL_GraphQLErrorsWithPathAndExtensions(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]any{
				{
					"message":    "Argument Validation Error",
					"path":       []string{"searchIssues"},
					"extensions": map[string]any{"code": "INVALID_INPUT", "field": "term"},
				},
			},
		})
	}))
	defer ts.Close()

	l := &linear{apiKey: "test-key", client: ts.Client()}
	_, err := testGQL(ts.URL, l, `{ searchIssues(term: "") { nodes { id } } }`, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Argument Validation Error")
	assert.Contains(t, err.Error(), "searchIssues")
	assert.Contains(t, err.Error(), "INVALID_INPUT")
}

func TestGQL_GraphQLErrorsWithoutExtensions(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]any{
				{"message": "Not authorized"},
			},
		})
	}))
	defer ts.Close()

	l := &linear{apiKey: "test-key", client: ts.Client()}
	_, err := testGQL(ts.URL, l, `{ viewer { id } }`, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Not authorized")
}

// --- arg helper tests (now using shared mcp.Arg* helpers) ---

func TestArgStr(t *testing.T) {
	v, err := mcp.ArgStr(map[string]any{"k": "val"}, "k")
	require.NoError(t, err)
	assert.Equal(t, "val", v)

	v, err = mcp.ArgStr(map[string]any{}, "k")
	require.NoError(t, err)
	assert.Empty(t, v)
}

func TestArgInt(t *testing.T) {
	v, err := mcp.ArgInt(map[string]any{"n": float64(42)}, "n")
	require.NoError(t, err)
	assert.Equal(t, 42, v)

	v, err = mcp.ArgInt(map[string]any{"n": 42}, "n")
	require.NoError(t, err)
	assert.Equal(t, 42, v)

	v, err = mcp.ArgInt(map[string]any{"n": "42"}, "n")
	require.NoError(t, err)
	assert.Equal(t, 42, v)

	v, err = mcp.ArgInt(map[string]any{}, "n")
	require.NoError(t, err)
	assert.Equal(t, 0, v)
}

func TestArgBool(t *testing.T) {
	v, err := mcp.ArgBool(map[string]any{"b": true}, "b")
	require.NoError(t, err)
	assert.True(t, v)

	v, err = mcp.ArgBool(map[string]any{"b": false}, "b")
	require.NoError(t, err)
	assert.False(t, v)

	v, err = mcp.ArgBool(map[string]any{"b": "true"}, "b")
	require.NoError(t, err)
	assert.True(t, v)

	v, err = mcp.ArgBool(map[string]any{}, "b")
	require.NoError(t, err)
	assert.False(t, v)
}

func TestOptInt(t *testing.T) {
	assert.Equal(t, 42, mcp.OptInt(map[string]any{"n": float64(42)}, "n", 10))
	assert.Equal(t, 10, mcp.OptInt(map[string]any{}, "n", 10))
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
