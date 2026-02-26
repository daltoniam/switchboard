package github

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
	assert.Equal(t, "github", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{"token": "ghp_test123"})
	assert.NoError(t, err)
}

func TestConfigure_MissingToken(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{"token": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token is required")
}

func TestConfigure_EmptyCredentials(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{})
	assert.Error(t, err)
}

func TestTools(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.NotEmpty(t, tools)

	// Verify all tools have names and descriptions.
	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveGitHubPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "github_", "tool %s missing github_ prefix", tool.Name)
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
	g := &integration{token: "test", client: nil}
	result, err := g.Execute(context.Background(), "github_nonexistent", nil)
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

// --- helper function tests ---

func TestArgStr(t *testing.T) {
	args := map[string]any{"key": "value", "num": 42}
	assert.Equal(t, "value", argStr(args, "key"))
	assert.Empty(t, argStr(args, "num"))
	assert.Empty(t, argStr(args, "missing"))
}

func TestArgInt(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		key  string
		want int
	}{
		{"float64", map[string]any{"n": float64(42)}, "n", 42},
		{"int", map[string]any{"n": 42}, "n", 42},
		{"string", map[string]any{"n": "42"}, "n", 42},
		{"missing", map[string]any{}, "n", 0},
		{"invalid string", map[string]any{"n": "abc"}, "n", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, argInt(tt.args, tt.key))
		})
	}
}

func TestArgInt64(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		key  string
		want int64
	}{
		{"float64", map[string]any{"n": float64(100)}, "n", 100},
		{"int", map[string]any{"n": 100}, "n", 100},
		{"int64", map[string]any{"n": int64(100)}, "n", 100},
		{"string", map[string]any{"n": "100"}, "n", 100},
		{"missing", map[string]any{}, "n", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, argInt64(tt.args, tt.key))
		})
	}
}

func TestArgBool(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		key  string
		want bool
	}{
		{"true bool", map[string]any{"b": true}, "b", true},
		{"false bool", map[string]any{"b": false}, "b", false},
		{"true string", map[string]any{"b": "true"}, "b", true},
		{"false string", map[string]any{"b": "false"}, "b", false},
		{"missing", map[string]any{}, "b", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, argBool(tt.args, tt.key))
		})
	}
}

func TestArgStrSlice(t *testing.T) {
	t.Run("[]any", func(t *testing.T) {
		args := map[string]any{"tags": []any{"a", "b", "c"}}
		assert.Equal(t, []string{"a", "b", "c"}, argStrSlice(args, "tags"))
	})

	t.Run("[]string", func(t *testing.T) {
		args := map[string]any{"tags": []string{"a", "b"}}
		assert.Equal(t, []string{"a", "b"}, argStrSlice(args, "tags"))
	})

	t.Run("comma-separated string", func(t *testing.T) {
		args := map[string]any{"tags": "a,b,c"}
		assert.Equal(t, []string{"a", "b", "c"}, argStrSlice(args, "tags"))
	})

	t.Run("empty string", func(t *testing.T) {
		args := map[string]any{"tags": ""}
		assert.Nil(t, argStrSlice(args, "tags"))
	})

	t.Run("missing", func(t *testing.T) {
		assert.Nil(t, argStrSlice(map[string]any{}, "tags"))
	})
}

func TestJsonResult(t *testing.T) {
	result, err := jsonResult(map[string]string{"key": "value"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"key"`)
	assert.Contains(t, result.Data, `"value"`)
}

func TestJsonResult_MarshalError(t *testing.T) {
	result, err := jsonResult(make(chan int))
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestErrResult(t *testing.T) {
	result, err := errResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

func TestListOpts(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		opts := listOpts(map[string]any{})
		assert.Equal(t, 1, opts.Page)
		assert.Equal(t, 30, opts.PerPage)
	})

	t.Run("custom", func(t *testing.T) {
		opts := listOpts(map[string]any{"page": float64(3), "per_page": float64(50)})
		assert.Equal(t, 3, opts.Page)
		assert.Equal(t, 50, opts.PerPage)
	})
}
