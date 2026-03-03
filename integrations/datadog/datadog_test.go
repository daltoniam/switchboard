package datadog

import (
	"fmt"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "datadog", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{"api_key": "key123", "app_key": "app456"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAPIKey(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{"api_key": "", "app_key": "app456"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_key and app_key are required")
}

func TestConfigure_MissingAppKey(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{"api_key": "key123", "app_key": ""})
	assert.Error(t, err)
}

func TestConfigure_EmptyCredentials(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{})
	assert.Error(t, err)
}

func TestConfigure_WithSite(t *testing.T) {
	d := &dd{}
	err := d.Configure(mcp.Credentials{"api_key": "key", "app_key": "app", "site": "datadoghq.eu"})
	assert.NoError(t, err)
	assert.Equal(t, "datadoghq.eu", d.site)
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

func TestTools_AllHaveDatadogPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "datadog_", "tool %s missing datadog_ prefix", tool.Name)
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
	d := &dd{apiKey: "key", appKey: "app"}
	d.Configure(mcp.Credentials{"api_key": "key", "app_key": "app"})
	result, err := d.Execute(t.Context(), "datadog_nonexistent", nil)
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
	args := map[string]any{"key": "value"}
	assert.Equal(t, "value", argStr(args, "key"))
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
	assert.True(t, argBool(map[string]any{"b": true}, "b"))
	assert.False(t, argBool(map[string]any{"b": false}, "b"))
	assert.True(t, argBool(map[string]any{"b": "true"}, "b"))
	assert.False(t, argBool(map[string]any{}, "b"))
}

func TestArgStrSlice(t *testing.T) {
	t.Run("[]any", func(t *testing.T) {
		args := map[string]any{"tags": []any{"a", "b"}}
		assert.Equal(t, []string{"a", "b"}, argStrSlice(args, "tags"))
	})

	t.Run("[]string", func(t *testing.T) {
		args := map[string]any{"tags": []string{"a"}}
		assert.Equal(t, []string{"a"}, argStrSlice(args, "tags"))
	})

	t.Run("csv string", func(t *testing.T) {
		args := map[string]any{"tags": "a,b"}
		assert.Equal(t, []string{"a", "b"}, argStrSlice(args, "tags"))
	})

	t.Run("empty string", func(t *testing.T) {
		assert.Nil(t, argStrSlice(map[string]any{"tags": ""}, "tags"))
	})

	t.Run("missing", func(t *testing.T) {
		assert.Nil(t, argStrSlice(map[string]any{}, "tags"))
	})
}

func TestOptInt(t *testing.T) {
	assert.Equal(t, 42, optInt(map[string]any{"n": float64(42)}, "n", 10))
	assert.Equal(t, 10, optInt(map[string]any{}, "n", 10))
	assert.Equal(t, 10, optInt(map[string]any{"n": float64(0)}, "n", 10))
}

func TestOptInt64(t *testing.T) {
	assert.Equal(t, int64(42), optInt64(map[string]any{"n": float64(42)}, "n", 10))
	assert.Equal(t, int64(10), optInt64(map[string]any{}, "n", 10))
}

func TestJsonResult(t *testing.T) {
	result, err := jsonResult(map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"key"`)
}

func TestErrResult(t *testing.T) {
	result, err := errResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

func TestParseTime(t *testing.T) {
	t.Run("empty defaults to fallback", func(t *testing.T) {
		result := parseTime("", -1*time.Hour)
		assert.WithinDuration(t, time.Now().Add(-1*time.Hour), result, 2*time.Second)
	})

	t.Run("now", func(t *testing.T) {
		result := parseTime("now", 0)
		assert.WithinDuration(t, time.Now(), result, 2*time.Second)
	})

	t.Run("relative now-1h", func(t *testing.T) {
		result := parseTime("now-1h", 0)
		assert.WithinDuration(t, time.Now().Add(-1*time.Hour), result, 2*time.Second)
	})

	t.Run("relative now-30m", func(t *testing.T) {
		result := parseTime("now-30m", 0)
		assert.WithinDuration(t, time.Now().Add(-30*time.Minute), result, 2*time.Second)
	})

	t.Run("ISO 8601", func(t *testing.T) {
		result := parseTime("2024-01-15T10:00:00Z", 0)
		expected, _ := time.Parse(time.RFC3339, "2024-01-15T10:00:00Z")
		assert.Equal(t, expected, result)
	})

	t.Run("epoch seconds", func(t *testing.T) {
		result := parseTime("1705312800", 0)
		assert.Equal(t, time.Unix(1705312800, 0), result)
	})

	t.Run("invalid falls back", func(t *testing.T) {
		result := parseTime("invalid-time", -1*time.Hour)
		assert.WithinDuration(t, time.Now().Add(-1*time.Hour), result, 2*time.Second)
	})
}
