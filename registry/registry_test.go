package registry

import (
	"context"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockIntegration implements mcp.Integration for testing.
type mockIntegration struct {
	name  string
	tools []mcp.ToolDefinition
}

func (m *mockIntegration) Name() string                      { return m.name }
func (m *mockIntegration) Configure(_ mcp.Credentials) error { return nil }
func (m *mockIntegration) Tools() []mcp.ToolDefinition       { return m.tools }
func (m *mockIntegration) Execute(_ context.Context, _ string, _ map[string]any) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: "ok"}, nil
}
func (m *mockIntegration) Healthy(_ context.Context) bool { return true }

func TestNew(t *testing.T) {
	r := New()
	require.NotNil(t, r)
	assert.Empty(t, r.All())
	assert.Empty(t, r.Names())
}

func TestRegister(t *testing.T) {
	r := New()
	err := r.Register(&mockIntegration{name: "github"})
	require.NoError(t, err)
	assert.Len(t, r.All(), 1)
}

func TestRegister_Duplicate(t *testing.T) {
	r := New()
	require.NoError(t, r.Register(&mockIntegration{name: "github"}))

	err := r.Register(&mockIntegration{name: "github"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestGet(t *testing.T) {
	r := New()
	require.NoError(t, r.Register(&mockIntegration{name: "github"}))

	i, ok := r.Get("github")
	assert.True(t, ok)
	assert.Equal(t, "github", i.Name())

	_, ok = r.Get("nonexistent")
	assert.False(t, ok)
}

func TestAll(t *testing.T) {
	r := New()
	require.NoError(t, r.Register(&mockIntegration{name: "github"}))
	require.NoError(t, r.Register(&mockIntegration{name: "datadog"}))

	all := r.All()
	assert.Len(t, all, 2)

	names := make([]string, len(all))
	for i, integration := range all {
		names[i] = integration.Name()
	}
	assert.ElementsMatch(t, []string{"github", "datadog"}, names)
}

func TestNames(t *testing.T) {
	r := New()
	require.NoError(t, r.Register(&mockIntegration{name: "github"}))
	require.NoError(t, r.Register(&mockIntegration{name: "datadog"}))
	require.NoError(t, r.Register(&mockIntegration{name: "linear"}))

	names := r.Names()
	assert.Len(t, names, 3)
	assert.ElementsMatch(t, []string{"github", "datadog", "linear"}, names)
}

func TestRegisterMultiple(t *testing.T) {
	r := New()
	integrations := []string{"github", "datadog", "linear", "sentry", "slack", "metabase"}

	for _, name := range integrations {
		require.NoError(t, r.Register(&mockIntegration{name: name}))
	}

	assert.Len(t, r.All(), 6)
	assert.Len(t, r.Names(), 6)
	assert.ElementsMatch(t, integrations, r.Names())
}
