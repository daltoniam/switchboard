package projectinterop

import (
	"context"
	"encoding/json"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range tools {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "missing handler for %s", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	names := map[mcp.ToolName]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, names[name], "orphan handler %s", name)
	}
}

func TestCRUD(t *testing.T) {
	root := t.TempDir()
	store := project.NewStore(root)
	require.NoError(t, store.Load())
	integration := NewWithCatalog(store)
	require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{"config_root": root}))

	res, err := integration.Execute(context.Background(), "projectinterop_create_project", map[string]any{
		"name": "demo", "description": "hello",
	})
	require.NoError(t, err)
	require.False(t, res.IsError)

	res, err = integration.Execute(context.Background(), "projectinterop_list_projects", nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	var list []map[string]any
	require.NoError(t, json.Unmarshal([]byte(res.Data), &list))
	require.Len(t, list, 1)
	assert.Equal(t, "demo", list[0]["name"])

	res, err = integration.Execute(context.Background(), "projectinterop_get_project", map[string]any{"name": "demo"})
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(res.Data), &got))
	assert.Equal(t, "hello", got["description"])

	res, err = integration.Execute(context.Background(), "projectinterop_update_project", map[string]any{
		"name": "demo", "patch": map[string]any{"description": "updated"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)

	res, err = integration.Execute(context.Background(), "projectinterop_delete_project", map[string]any{"name": "demo"})
	require.NoError(t, err)
	require.False(t, res.IsError)
}

func TestNewWithCatalog_DoesNotAllocateSecondStore(t *testing.T) {
	store := project.NewStore(t.TempDir())
	require.NoError(t, store.Load())
	integration := NewWithCatalog(store)
	require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{"config_root": t.TempDir()}))
	// Configure must not replace injected store.
	res, err := integration.Execute(context.Background(), "projectinterop_list_projects", nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
}
