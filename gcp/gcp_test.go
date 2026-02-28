package gcp

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
	assert.Equal(t, "gcp", i.Name())
}

func TestConfigure_MissingProjectID(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project_id is required")
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

func TestTools_AllHaveGCPPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "gcp_", "tool %s missing gcp_ prefix", tool.Name)
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
	a := &integration{projectID: "test-project"}
	result, err := a.Execute(context.Background(), "gcp_nonexistent", nil)
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

func TestToolCount(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.Len(t, tools, len(dispatch), "tool count should match dispatch map size")
}

// --- Result helper tests ---

func TestJsonResult(t *testing.T) {
	result, err := jsonResult(map[string]string{"key": "value"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"key"`)
	assert.Contains(t, result.Data, `"value"`)
}

func TestErrResult(t *testing.T) {
	result, err := errResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

// --- Argument helper tests ---

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

func TestArgInt32(t *testing.T) {
	assert.Equal(t, int32(42), argInt32(map[string]any{"n": float64(42)}, "n"))
	assert.Equal(t, int32(42), argInt32(map[string]any{"n": 42}, "n"))
	assert.Equal(t, int32(42), argInt32(map[string]any{"n": "42"}, "n"))
	assert.Equal(t, int32(0), argInt32(map[string]any{}, "n"))
}

func TestArgInt64(t *testing.T) {
	assert.Equal(t, int64(42), argInt64(map[string]any{"n": float64(42)}, "n"))
	assert.Equal(t, int64(42), argInt64(map[string]any{"n": 42}, "n"))
	assert.Equal(t, int64(42), argInt64(map[string]any{"n": int64(42)}, "n"))
	assert.Equal(t, int64(42), argInt64(map[string]any{"n": "42"}, "n"))
	assert.Equal(t, int64(0), argInt64(map[string]any{}, "n"))
}

func TestArgBool(t *testing.T) {
	assert.True(t, argBool(map[string]any{"b": true}, "b"))
	assert.False(t, argBool(map[string]any{"b": false}, "b"))
	assert.True(t, argBool(map[string]any{"b": "true"}, "b"))
	assert.False(t, argBool(map[string]any{}, "b"))
}

func TestArgStrSlice(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, argStrSlice(map[string]any{"s": []any{"a", "b"}}, "s"))
	assert.Equal(t, []string{"a", "b"}, argStrSlice(map[string]any{"s": []string{"a", "b"}}, "s"))
	assert.Equal(t, []string{"a", "b"}, argStrSlice(map[string]any{"s": "a,b"}, "s"))
	assert.Nil(t, argStrSlice(map[string]any{}, "s"))
	assert.Nil(t, argStrSlice(map[string]any{"s": ""}, "s"))
}

func TestHealthy_NilClient(t *testing.T) {
	a := &integration{}
	assert.False(t, a.Healthy(context.Background()))
}

func TestProjectName(t *testing.T) {
	a := &integration{projectID: "my-project"}
	assert.Equal(t, "projects/my-project", a.projectName())
}
