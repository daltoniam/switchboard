package elasticsearch

import (
	"context"
	"fmt"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "elasticsearch", i.Name())
}

func TestConfigure_Defaults(t *testing.T) {
	e := &esInt{}
	err := e.Configure(context.Background(), mcp.Credentials{})
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9200", e.baseURL)
	assert.NotNil(t, e.client)
}

func TestConfigure_CustomURL(t *testing.T) {
	e := &esInt{}
	err := e.Configure(context.Background(), mcp.Credentials{"base_url": "https://es.example.com:9243/"})
	require.NoError(t, err)
	assert.Equal(t, "https://es.example.com:9243", e.baseURL)
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

func TestTools_AllHaveElasticsearchPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "elasticsearch_", "tool %s missing elasticsearch_ prefix", tool.Name)
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
	e := &esInt{}
	err := e.Configure(context.Background(), mcp.Credentials{})
	require.NoError(t, err)
	result, execErr := e.Execute(context.Background(), "elasticsearch_nonexistent", nil)
	require.NoError(t, execErr)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestExecute_NilClient(t *testing.T) {
	e := &esInt{}
	result, err := e.Execute(context.Background(), "elasticsearch_cluster_health", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "not configured")
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

func TestHealthy_NilClient(t *testing.T) {
	e := &esInt{}
	assert.False(t, e.Healthy(context.Background()))
}

func TestRawResult(t *testing.T) {
	data := []byte(`{"key":"value"}`)
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
