package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/project"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupProjectRouter(t *testing.T, def *project.Definition, integrations ...*mockIntegration) (*ProjectRouter, *project.Store) {
	t.Helper()
	dir := t.TempDir()
	store := project.NewStore(dir)
	require.NoError(t, store.Create(def))

	reg := newMockRegistry()
	cfgIntegrations := make(map[string]*mcp.IntegrationConfig)

	for _, i := range integrations {
		reg.Register(i)
		cfgIntegrations[i.name] = &mcp.IntegrationConfig{
			Enabled:     true,
			Credentials: mcp.Credentials{"token": "test"},
		}
	}

	services := &mcp.Services{
		Config:   newMockConfigService(cfgIntegrations),
		Registry: reg,
	}

	// Build search index from test integrations so project-scoped search
	// gets TF-IDF + synonym scoring.
	sm := buildSynonymMap(synonymGroups)
	var tools []toolWithIntegration
	for _, i := range integrations {
		for _, tool := range i.Tools() {
			tools = append(tools, toolWithIntegration{Integration: i.Name(), Tool: tool})
		}
	}
	idf := computeIDF(tools)

	router := NewProjectRouter(services, store, "switchboard", SearchIndex{IDF: idf, SynMap: sm, AllTools: tools})
	return router, store
}

// setupProjectRouterWithIntegration mirrors setupProjectRouter but accepts an
// arbitrary mcp.Integration so tests can register wrapper types like
// mockIntegrationWithCap that aren't *mockIntegration directly.
func setupProjectRouterWithIntegration(t *testing.T, def *project.Definition, i mcp.Integration) (*ProjectRouter, *project.Store) {
	t.Helper()
	dir := t.TempDir()
	store := project.NewStore(dir)
	require.NoError(t, store.Create(def))

	reg := newMockRegistry()
	reg.Register(i)

	services := &mcp.Services{
		Config: newMockConfigService(map[string]*mcp.IntegrationConfig{
			i.Name(): {Enabled: true, Credentials: mcp.Credentials{"token": "test"}},
		}),
		Registry: reg,
	}

	sm := buildSynonymMap(synonymGroups)
	var tools []toolWithIntegration
	for _, tool := range i.Tools() {
		tools = append(tools, toolWithIntegration{Integration: i.Name(), Tool: tool})
	}
	idf := computeIDF(tools)

	router := NewProjectRouter(services, store, "switchboard", SearchIndex{IDF: idf, SynMap: sm, AllTools: tools})
	return router, store
}

func projectToolRequest(name string, args map[string]any) *mcpsdk.CallToolRequest {
	data, _ := json.Marshal(args)
	return &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      name,
			Arguments: json.RawMessage(data),
		},
	}
}

func TestProjectRouter_GetOrCreate(t *testing.T) {
	def := &project.Definition{Version: "1", Name: "test-project"}
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "github_list_issues", Description: "List issues"},
		},
	}
	router, _ := setupProjectRouter(t, def, mi)

	srv, err := router.getOrCreate("test-project")
	require.NoError(t, err)
	require.NotNil(t, srv)

	srv2, err := router.getOrCreate("test-project")
	require.NoError(t, err)
	assert.Same(t, srv, srv2)
}

func TestProjectRouter_GetOrCreate_NotFound(t *testing.T) {
	def := &project.Definition{Version: "1", Name: "test-project"}
	router, _ := setupProjectRouter(t, def)

	_, err := router.getOrCreate("nonexistent")
	assert.ErrorContains(t, err, "not found")
}

func TestProjectRouter_SearchFiltersTools(t *testing.T) {
	def := &project.Definition{
		Version: "1",
		Name:    "scoped",
		Tools: map[string]*project.ScopeRule{
			"switchboard": {
				Allow: []string{"github_*"},
				Deny:  []string{"github_delete_*"},
			},
		},
	}
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "github_list_issues", Description: "List issues"},
			{Name: "github_delete_repo", Description: "Delete repo"},
			{Name: "github_get_issue", Description: "Get issue"},
		},
	}
	router, _ := setupProjectRouter(t, def, mi)

	srv, err := router.getOrCreate("scoped")
	require.NoError(t, err)

	handler := router.makeSearchHandler(project.GetEffectiveRule(def, "switchboard", ""))

	result, err := handler(context.Background(), projectToolRequest("search", map[string]any{}))
	require.NoError(t, err)

	resp := parseSearchResponse(t, result)
	assert.Equal(t, 2, resp.Total)

	names := searchToolNames(t, resp)
	assert.Contains(t, names, "github_list_issues")
	assert.Contains(t, names, "github_get_issue")
	assert.NotContains(t, names, "github_delete_repo")

	_ = srv
}

func TestProjectRouter_ExecuteInjectsDefaults(t *testing.T) {
	def := &project.Definition{
		Version: "1",
		Name:    "defaults-test",
		Tools: map[string]*project.ScopeRule{
			"switchboard": {
				Defaults: map[string]map[string]any{
					"github_*": {"owner": "myorg", "repo": "myrepo"},
				},
			},
		},
	}

	var capturedArgs map[string]any
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "github_list_issues", Description: "List issues"},
		},
		execFn: func(_ context.Context, _ string, args map[string]any) (*mcp.ToolResult, error) {
			capturedArgs = args
			return &mcp.ToolResult{Data: `[]`}, nil
		},
	}
	router, _ := setupProjectRouter(t, def, mi)

	scopeRule := project.GetEffectiveRule(def, "switchboard", "")
	handler := router.makeExecuteHandler(def, scopeRule)

	_, err := handler(context.Background(), projectToolRequest("execute", map[string]any{
		"tool_name": "github_list_issues",
		"arguments": map[string]any{"state": "open"},
	}))
	require.NoError(t, err)

	assert.Equal(t, "myorg", capturedArgs["owner"])
	assert.Equal(t, "myrepo", capturedArgs["repo"])
	assert.Equal(t, "open", capturedArgs["state"])
}

func TestProjectRouter_ExecuteAgentOverridesDefaults(t *testing.T) {
	def := &project.Definition{
		Version: "1",
		Name:    "override-test",
		Tools: map[string]*project.ScopeRule{
			"switchboard": {
				Defaults: map[string]map[string]any{
					"github_*": {"owner": "default-org"},
				},
			},
		},
	}

	var capturedArgs map[string]any
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "github_list_issues", Description: "List issues"},
		},
		execFn: func(_ context.Context, _ string, args map[string]any) (*mcp.ToolResult, error) {
			capturedArgs = args
			return &mcp.ToolResult{Data: `[]`}, nil
		},
	}
	router, _ := setupProjectRouter(t, def, mi)

	scopeRule := project.GetEffectiveRule(def, "switchboard", "")
	handler := router.makeExecuteHandler(def, scopeRule)

	_, err := handler(context.Background(), projectToolRequest("execute", map[string]any{
		"tool_name": "github_list_issues",
		"arguments": map[string]any{"owner": "override-org"},
	}))
	require.NoError(t, err)

	assert.Equal(t, "override-org", capturedArgs["owner"])
}

func TestProjectRouter_ExecuteDenied(t *testing.T) {
	def := &project.Definition{
		Version: "1",
		Name:    "deny-test",
		Tools: map[string]*project.ScopeRule{
			"switchboard": {
				Deny: []string{"github_delete_*"},
			},
		},
	}
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "github_delete_repo", Description: "Delete repo"},
		},
	}
	router, _ := setupProjectRouter(t, def, mi)

	scopeRule := project.GetEffectiveRule(def, "switchboard", "")
	handler := router.makeExecuteHandler(def, scopeRule)

	result, err := handler(context.Background(), projectToolRequest("execute", map[string]any{
		"tool_name": "github_delete_repo",
	}))
	require.NoError(t, err)
	assert.True(t, result.IsError)

	tc := result.Content[0].(*mcpsdk.TextContent)
	assert.Contains(t, tc.Text, "denied")
}

func TestProjectRouter_ExecutePerIntegrationCap(t *testing.T) {
	// The project router's execute handler and server.handleExecute both call
	// responseLimitFor. These subtests pin the project router path so a future
	// refactor can't silently regress the per-integration cap behavior there.
	def := &project.Definition{Version: "1", Name: "cap-test"}

	buildIntegration := func(payload string) *mockIntegrationWithCap {
		return &mockIntegrationWithCap{
			mockIntegration: &mockIntegration{
				name:    "bigint",
				healthy: true,
				tools: []mcp.ToolDefinition{
					{Name: "bigint_get_page", Description: "Returns rich page content"},
				},
				execFn: func(_ context.Context, _ string, _ map[string]any) (*mcp.ToolResult, error) {
					return &mcp.ToolResult{Data: payload}, nil
				},
			},
			maxBytes: 256 * 1024,
		}
	}

	executeTool := func(t *testing.T, mi *mockIntegrationWithCap) *mcpsdk.CallToolResult {
		t.Helper()
		router, _ := setupProjectRouterWithIntegration(t, def, mi)
		scopeRule := project.GetEffectiveRule(def, "switchboard", "")
		handler := router.makeExecuteHandler(def, scopeRule)
		result, err := handler(context.Background(), projectToolRequest("execute", map[string]any{
			"tool_name": "bigint_get_page",
			"arguments": map[string]any{},
		}))
		require.NoError(t, err)
		return result
	}

	t.Run("honored above default under override", func(t *testing.T) {
		// 100KB payload is above the 50KB default but under the 256KB override —
		// a default-capped integration would reject this, the override must allow it.
		payload := fmt.Sprintf(`{"data":"%s"}`, strings.Repeat("x", 100*1024))
		result := executeTool(t, buildIntegration(payload))

		assert.False(t, result.IsError, "response within per-integration cap should succeed")
		tc := result.Content[0].(*mcpsdk.TextContent)
		assert.Equal(t, payload, tc.Text)
	})

	t.Run("still enforced above override", func(t *testing.T) {
		// 300KB payload exceeds even the raised 256KB cap — must still be rejected,
		// and the error must report the integration's cap, not the default.
		payload := fmt.Sprintf(`{"data":"%s"}`, strings.Repeat("x", 300*1024))
		result := executeTool(t, buildIntegration(payload))

		assert.True(t, result.IsError, "response above per-integration cap should be rejected")
		tc := result.Content[0].(*mcpsdk.TextContent)
		assert.Contains(t, tc.Text, "256KB", "error should report the integration's own cap")
	})
}

func TestProjectRouter_ContextManifest(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "AGENTS.md"), []byte("instructions"), 0600))

	configDir := filepath.Join(dir, "config")
	contextDir := filepath.Join(configDir, "context", "ctx-test")
	require.NoError(t, os.MkdirAll(contextDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(contextDir, "sprint.md"), []byte("goals"), 0600))

	def := &project.Definition{
		Version: "1",
		Name:    "ctx-test",
		Repo:    repoDir,
		Context: &project.ContextConfig{
			Files:        []string{"sprint.md"},
			RepoIncludes: []string{"AGENTS.md"},
		},
	}

	store := project.NewStore(configDir)
	require.NoError(t, store.Create(def))

	services := &mcp.Services{
		Config:   newMockConfigService(nil),
		Registry: newMockRegistry(),
	}
	router := NewProjectRouter(services, store, "switchboard", SearchIndex{})

	handler := router.makeContextHandler(def)

	t.Run("manifest with no args", func(t *testing.T) {
		result, err := handler(context.Background(), projectToolRequest("project_context", map[string]any{}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		tc := result.Content[0].(*mcpsdk.TextContent)
		var entries []project.ContextEntry
		require.NoError(t, json.Unmarshal([]byte(tc.Text), &entries))
		assert.Len(t, entries, 2)
	})

	t.Run("fetch specific file", func(t *testing.T) {
		result, err := handler(context.Background(), projectToolRequest("project_context", map[string]any{
			"path": "AGENTS.md",
		}))
		require.NoError(t, err)

		tc := result.Content[0].(*mcpsdk.TextContent)
		assert.Equal(t, "instructions", tc.Text)
	})

	t.Run("query filter", func(t *testing.T) {
		result, err := handler(context.Background(), projectToolRequest("project_context", map[string]any{
			"query": "sprint",
		}))
		require.NoError(t, err)

		tc := result.Content[0].(*mcpsdk.TextContent)
		var entries []project.ContextEntry
		require.NoError(t, json.Unmarshal([]byte(tc.Text), &entries))
		assert.Len(t, entries, 1)
		assert.Equal(t, "sprint.md", entries[0].Path)
	})
}

func TestProjectRouter_ProjectList(t *testing.T) {
	def := &project.Definition{Version: "1", Name: "p1", Repo: "~/work/p1"}
	router, store := setupProjectRouter(t, def)

	def2 := &project.Definition{Version: "1", Name: "p2"}
	require.NoError(t, store.Create(def2))

	result, err := router.handleProjectList(context.Background(), projectToolRequest("project_list", nil))
	require.NoError(t, err)

	tc := result.Content[0].(*mcpsdk.TextContent)
	var list []struct {
		Name string `json:"name"`
	}
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &list))
	assert.Len(t, list, 2)
}

func TestProjectRouter_ProjectGet(t *testing.T) {
	def := &project.Definition{Version: "1", Name: "myproj", Repo: "~/work/myproj"}
	router, _ := setupProjectRouter(t, def)

	handler := router.makeProjectGetHandler(def)
	result, err := handler(context.Background(), projectToolRequest("project_get", map[string]any{}))
	require.NoError(t, err)

	tc := result.Content[0].(*mcpsdk.TextContent)
	var got project.Definition
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &got))
	assert.Equal(t, "myproj", got.Name)
}

func TestProjectRouter_ProjectCreate(t *testing.T) {
	def := &project.Definition{Version: "1", Name: "existing"}
	router, _ := setupProjectRouter(t, def)

	result, err := router.handleProjectCreate(context.Background(), projectToolRequest("project_create", map[string]any{
		"name": "new-project",
		"repo": "~/work/new",
	}))
	require.NoError(t, err)
	require.False(t, result.IsError)

	got, ok := router.store.Get("new-project")
	require.True(t, ok)
	assert.Equal(t, "~/work/new", got.Repo)
}

func TestProjectRouter_ProjectCreateDuplicate(t *testing.T) {
	def := &project.Definition{Version: "1", Name: "existing"}
	router, _ := setupProjectRouter(t, def)

	result, err := router.handleProjectCreate(context.Background(), projectToolRequest("project_create", map[string]any{
		"name": "existing",
	}))
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestProjectRouter_ProjectUpdate(t *testing.T) {
	def := &project.Definition{Version: "1", Name: "updatable"}
	router, _ := setupProjectRouter(t, def)

	handler := router.makeProjectUpdateHandler(def)
	result, err := handler(context.Background(), projectToolRequest("project_update", map[string]any{
		"patch": map[string]any{"branch": "develop"},
	}))
	require.NoError(t, err)
	require.False(t, result.IsError)

	got, _ := router.store.Get("updatable")
	assert.Equal(t, "develop", got.Branch)
}

func TestProjectRouter_ProjectDelete(t *testing.T) {
	def := &project.Definition{Version: "1", Name: "deletable"}
	router, _ := setupProjectRouter(t, def)

	handler := router.makeProjectDeleteHandler(def)
	result, err := handler(context.Background(), projectToolRequest("project_delete", map[string]any{}))
	require.NoError(t, err)
	require.False(t, result.IsError)

	_, ok := router.store.Get("deletable")
	assert.False(t, ok)
}

func TestProjectRouter_ProjectTools(t *testing.T) {
	def := &project.Definition{
		Version: "1",
		Name:    "tools-test",
		Tools: map[string]*project.ScopeRule{
			"switchboard": {
				Allow: []string{"github_list_*"},
			},
		},
	}
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "github_list_issues", Description: "List issues"},
			{Name: "github_get_issue", Description: "Get issue"},
			{Name: "github_delete_repo", Description: "Delete repo"},
		},
	}
	router, _ := setupProjectRouter(t, def, mi)

	handler := router.makeProjectToolsHandler(def)
	result, err := handler(context.Background(), projectToolRequest("project_tools", map[string]any{}))
	require.NoError(t, err)

	tc := result.Content[0].(*mcpsdk.TextContent)
	var tools []string
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &tools))
	assert.Equal(t, []string{"github_list_issues"}, tools)
}

func TestProjectRouter_ProjectDefaults(t *testing.T) {
	def := &project.Definition{
		Version: "1",
		Name:    "defaults-test",
		Tools: map[string]*project.ScopeRule{
			"switchboard": {
				Defaults: map[string]map[string]any{
					"github_*": {"owner": "myorg"},
				},
			},
		},
	}
	router, _ := setupProjectRouter(t, def)

	handler := router.makeProjectDefaultsHandler(def)
	result, err := handler(context.Background(), projectToolRequest("project_defaults", map[string]any{
		"tool_name": "github_list_issues",
	}))
	require.NoError(t, err)

	tc := result.Content[0].(*mcpsdk.TextContent)
	var defaults map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &defaults))
	assert.Equal(t, "myorg", defaults["owner"])
}

func TestProjectRouter_Handler(t *testing.T) {
	def := &project.Definition{Version: "1", Name: "handler-test"}
	router, _ := setupProjectRouter(t, def)
	handler := router.Handler()
	assert.NotNil(t, handler)
}
