package datadog

import (
	"context"
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
	err := i.Configure(context.Background(), mcp.Credentials{"api_key": "key123", "app_key": "app456"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAPIKey(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_key": "", "app_key": "app456"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_key and app_key are required")
}

func TestConfigure_MissingAppKey(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_key": "key123", "app_key": ""})
	assert.Error(t, err)
}

func TestConfigure_EmptyCredentials(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.Error(t, err)
}

func TestConfigure_WithSite(t *testing.T) {
	d := &dd{}
	err := d.Configure(context.Background(), mcp.Credentials{"api_key": "key", "app_key": "app", "site": "datadoghq.eu"})
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
	d.Configure(context.Background(), mcp.Credentials{"api_key": "key", "app_key": "app"})
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

// Argument helper tests removed — shared helpers are tested in args_test.go.

func TestJsonResult(t *testing.T) {
	result, err := mcp.JSONResult(map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"key"`)
}

func TestErrResult(t *testing.T) {
	result, err := mcp.ErrResult(fmt.Errorf("test error"))
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
