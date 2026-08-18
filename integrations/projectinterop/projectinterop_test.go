package projectinterop

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	integration := New()
	require.NotNil(t, integration)
	assert.Equal(t, "projectinterop", integration.Name())
}

func TestConfigure(t *testing.T) {
	root := t.TempDir()
	integration := New()

	require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{"config_root": root}))
	assert.True(t, integration.Healthy(context.Background()))
	assert.Equal(t, []string{"config_root"}, integration.(mcp.PlainTextCredentials).PlainTextKeys())
}

func TestConfigure_DefaultRoot(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	integration := New()

	require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{}))
	assert.True(t, integration.Healthy(context.Background()))
}

func TestTools(t *testing.T) {
	integration := New()
	seen := make(map[mcp.ToolName]bool)
	for _, tool := range integration.Tools() {
		assert.NotEmpty(t, tool.Name)
		assert.NotEmpty(t, tool.Description)
		assert.Contains(t, tool.Name, "projectinterop_")
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
	assert.Len(t, seen, 6)
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range New().Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range New().Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		_, ok := dispatch[name]
		assert.True(t, ok, "compaction spec %s has no dispatch handler", name)
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	result, err := New().Execute(context.Background(), mcp.ToolName("projectinterop_unknown"), nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestNewWithCatalog_DoesNotAllocateSecondStore(t *testing.T) {
	root := t.TempDir()
	store := project.NewStore(root)
	require.NoError(t, store.Load())
	_, err := store.Create(context.Background(), project.CreateRequest{Definition: project.Definition{
		Version:   "1",
		Name:      "shared",
		Resources: map[string]project.Resource{"main": {Type: project.ResourceTypeRepo, Path: "/tmp/shared", Branch: "main"}},
	}})
	require.NoError(t, err)

	integration := NewWithCatalog(store)
	require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{"config_root": t.TempDir()}))

	got := executeJSON(t, integration, "projectinterop_get_project", map[string]any{"name": "shared"})
	assert.Equal(t, "shared", got["name"])
	resources := got["resources"].(map[string]any)
	main := resources["main"].(map[string]any)
	assert.Equal(t, "main", main["branch"])

	executeJSON(t, integration, "projectinterop_update_project", map[string]any{
		"name": "shared",
		"patch": map[string]any{"resources": map[string]any{
			"main": map[string]any{"type": "repo", "path": "/tmp/shared", "branch": "from-interop"},
		}},
	})
	snap, err := store.Get(context.Background(), "shared")
	require.NoError(t, err)
	assert.Equal(t, "from-interop", snap.Definition.PrimaryBranch())
}

func TestCreate_ReturnsUserDefinitionNotOverlay(t *testing.T) {
	root := t.TempDir()
	repo := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".project.json"), []byte(
		`{"version":"1","name":"acme","resources":{"main":{"type":"repo","path":"`+repo+`","branch":"overlay"}}}`), 0600))
	integration := New()
	require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{"config_root": root}))

	created := executeJSON(t, integration, "projectinterop_create_project", map[string]any{
		"name": "acme", "repo": repo, "branch": "user",
	})
	assert.Equal(t, "acme", created["name"])
	resources := created["resources"].(map[string]any)
	main := resources["main"].(map[string]any)
	assert.Equal(t, "user", main["branch"])
	assert.Equal(t, repo, main["path"])
}

func TestProjectCRUD(t *testing.T) {
	root := t.TempDir()
	integration := New()
	require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{"config_root": root}))

	created := executeJSON(t, integration, "projectinterop_create_project", map[string]any{
		"name": "acme-api", "repo": "/work/acme", "branch": "main",
	})
	assert.Equal(t, "acme-api", created["name"])

	listed := executeJSONArray(t, integration, "projectinterop_list_projects", nil)
	require.Len(t, listed, 1)
	assert.Equal(t, "acme-api", listed[0]["name"])

	updated := executeJSON(t, integration, "projectinterop_update_project", map[string]any{
		"name": "acme-api",
		"patch": map[string]any{
			"resources": map[string]any{"main": map[string]any{"type": "repo", "path": "/work/acme", "branch": "develop"}},
			"custom":    "preserved",
		},
	})
	resources := updated["resources"].(map[string]any)
	main := resources["main"].(map[string]any)
	assert.Equal(t, "develop", main["branch"])
	assert.Equal(t, "preserved", updated["custom"])

	got := executeJSON(t, integration, "projectinterop_get_project", map[string]any{"name": "acme-api"})
	resources = got["resources"].(map[string]any)
	main = resources["main"].(map[string]any)
	assert.Equal(t, "develop", main["branch"])
	assert.Equal(t, "preserved", got["custom"])

	result, err := integration.Execute(context.Background(), mcp.ToolName("projectinterop_delete_project"), map[string]any{"name": "acme-api"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "deleted")

	_, err = os.Stat(filepath.Join(root, "projects", "acme-api.project.json"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestProjectContext(t *testing.T) {
	root := t.TempDir()
	repo := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(repo, "AGENTS.md"), []byte("repo guidance"), 0600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "context", "acme"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "context", "acme", "sprint.md"), []byte("store guidance"), 0600))

	integration := New()
	require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{"config_root": root}))
	executeJSON(t, integration, "projectinterop_create_project", map[string]any{"name": "acme", "repo": repo})
	sprintPath := filepath.Join(root, "context", "acme", "sprint.md")
	executeJSON(t, integration, "projectinterop_update_project", map[string]any{
		"name": "acme",
		"patch": map[string]any{"resources": map[string]any{
			"main":   map[string]any{"type": "repo", "path": repo},
			"agents": map[string]any{"type": "file", "repo": "main", "path": "AGENTS.md"},
			"sprint": map[string]any{"type": "file", "path": sprintPath},
		}},
	})

	manifest := executeJSONArray(t, integration, "projectinterop_get_context", map[string]any{"name": "acme"})
	require.GreaterOrEqual(t, len(manifest), 2)
	paths := map[string]bool{}
	for _, e := range manifest {
		paths[e["path"].(string)] = true
	}
	assert.True(t, paths["AGENTS.md"])

	result, err := integration.Execute(context.Background(), mcp.ToolName("projectinterop_get_context"), map[string]any{"name": "acme", "path": "AGENTS.md"})
	require.NoError(t, err)
	assert.Equal(t, "repo guidance", result.Data)

	filtered := executeJSONArray(t, integration, "projectinterop_get_context", map[string]any{"name": "acme", "query": "agent"})
	require.Len(t, filtered, 1)
	assert.Equal(t, "AGENTS.md", filtered[0]["path"])
}

func TestErrorsAreToolResults(t *testing.T) {
	integration := New()
	require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{"config_root": t.TempDir()}))

	tests := []struct {
		name string
		tool string
		args map[string]any
		want string
	}{
		{name: "missing name", tool: "projectinterop_get_project", args: nil, want: "name is required"},
		{name: "not found", tool: "projectinterop_get_project", args: map[string]any{"name": "missing"}, want: "not found"},
		{name: "missing patch", tool: "projectinterop_update_project", args: map[string]any{"name": "missing"}, want: "patch is required"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := integration.Execute(context.Background(), mcp.ToolName(test.tool), test.args)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, test.want)
		})
	}
}

func executeJSON(t *testing.T, integration mcp.Integration, tool string, args map[string]any) map[string]any {
	t.Helper()
	result, err := integration.Execute(context.Background(), mcp.ToolName(tool), args)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	var value map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &value))
	return value
}

func executeJSONArray(t *testing.T, integration mcp.Integration, tool string, args map[string]any) []map[string]any {
	t.Helper()
	result, err := integration.Execute(context.Background(), mcp.ToolName(tool), args)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	var value []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &value))
	return value
}
