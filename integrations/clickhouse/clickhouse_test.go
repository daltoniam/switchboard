package clickhouse

import (
	"context"
	"fmt"
	"testing"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "clickhouse", i.Name())
}

func TestConfigure_Defaults(t *testing.T) {
	c := &clickhouseInt{}
	err := c.Configure(context.Background(), mcp.Credentials{"host": "localhost", "port": "9000"})
	if err != nil {
		t.Skipf("ClickHouse not available: %v", err)
	}
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

func TestTools_AllHaveClickhousePrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "clickhouse_", "tool %s missing clickhouse_ prefix", tool.Name)
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
	conn, err := ch.Open(&ch.Options{Addr: []string{"localhost:9000"}})
	require.NoError(t, err)
	c := &clickhouseInt{conn: conn}
	result, err := c.Execute(context.Background(), "clickhouse_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestExecute_NilConn(t *testing.T) {
	c := &clickhouseInt{}
	result, err := c.Execute(context.Background(), "clickhouse_query", map[string]any{"sql": "SELECT 1"})
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

func TestHealthy_NilConn(t *testing.T) {
	c := &clickhouseInt{}
	assert.False(t, c.Healthy(context.Background()))
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

// Argument helper tests removed — shared helpers are tested in args_test.go.

func TestEscapeIdentifier(t *testing.T) {
	assert.Equal(t, "`my_table`", escapeIdentifier("my_table"))
	assert.Equal(t, "`my``table`", escapeIdentifier("my`table"))
}
