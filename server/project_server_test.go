package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

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
	require.NoError(t, store.CreateDefinition(def))

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
	require.NoError(t, store.CreateDefinition(def))

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

func TestProjectRouter_StaticToolListCapability(t *testing.T) {
	def := &project.Definition{Version: "1", Name: "test-project"}
	router, _ := setupProjectRouter(t, def, &mockIntegration{
		name:    "github",
		healthy: true,
		tools:   []mcp.ToolDefinition{{Name: mcp.ToolName("github_list_issues"), Description: "List issues"}},
	})
	srv, err := router.getOrCreate("test-project")
	require.NoError(t, err)

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ss, err := srv.mcpSrv.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	defer ss.Close() //nolint:errcheck

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "crush", Version: "0.89.0"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer cs.Close() //nolint:errcheck

	require.NotNil(t, cs.InitializeResult().Capabilities.Tools)
	assert.False(t, cs.InitializeResult().Capabilities.Tools.ListChanged)
}

func TestProjectRouter_GetOrCreate(t *testing.T) {
	def := &project.Definition{Version: "1", Name: "test-project"}
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("github_list_issues"), Description: "List issues"},
		},
	}
	router, _ := setupProjectRouter(t, def, mi)

	srv, err := router.getOrCreate("test-project")
	require.NoError(t, err)
	require.NotNil(t, srv)

	srv2, err := router.getOrCreate("test-project")
	require.NoError(t, err)
	require.NotNil(t, srv2)
	assert.Equal(t, srv.def.Name, srv2.def.Name)
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
			{Name: mcp.ToolName("github_list_issues"), Description: "List issues"},
			{Name: mcp.ToolName("github_delete_repo"), Description: "Delete repo"},
			{Name: mcp.ToolName("github_get_issue"), Description: "Get issue"},
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
			{Name: mcp.ToolName("github_list_issues"), Description: "List issues"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
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
			{Name: mcp.ToolName("github_list_issues"), Description: "List issues"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
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

func TestProjectRouter_ExecuteReturnsNativeMedia(t *testing.T) {
	def := &project.Definition{Version: "1", Name: "media-test"}
	mi := &mockIntegration{
		name:    "vision",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "vision_get_image", Description: "Get an image"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			return mcp.MediaResult(`{"document_id":7}`, []byte("image"), "image/webp", "document-7.webp")
		},
	}
	router, _ := setupProjectRouter(t, def, mi)
	handler := router.makeExecuteHandler(def, nil)

	result, err := handler(context.Background(), projectToolRequest("execute", map[string]any{
		"tool_name": "vision_get_image",
	}))
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Len(t, result.Content, 2)
	assert.Equal(t, `{"document_id":7}`, result.Content[0].(*mcpsdk.TextContent).Text)
	image := result.Content[1].(*mcpsdk.ImageContent)
	assert.Equal(t, []byte("image"), image.Data)
	assert.Equal(t, "image/webp", image.MIMEType)
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
			{Name: mcp.ToolName("github_delete_repo"), Description: "Delete repo"},
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
				execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
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

func TestProjectRouter_ExecutePerToolCap(t *testing.T) {
	// Pins the project router's tool-aware responseLimitFor lookup so a future
	// refactor can't silently drop the per-tool override branch.
	def := &project.Definition{Version: "1", Name: "per-tool-cap-test"}

	buildIntegration := func(toolName mcp.ToolName, payload string) *mockProjectIntegrationWithPerToolCap {
		_ = toolName // Reserved for future per-tool variations; keeps the call sites self-documenting.
		return &mockProjectIntegrationWithPerToolCap{
			mockIntegration: &mockIntegration{
				name:    "bigint",
				healthy: true,
				tools: []mcp.ToolDefinition{
					{Name: "bigint_get_diff", Description: "Returns raw diff"},
					{Name: "bigint_get_thing", Description: "Returns a normal payload"},
				},
				execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
					return &mcp.ToolResult{Data: payload}, nil
				},
			},
			perTool: map[mcp.ToolName]int{"bigint_get_diff": 1024 * 1024},
		}
	}

	execute := func(t *testing.T, mi *mockProjectIntegrationWithPerToolCap, toolName string) *mcpsdk.CallToolResult {
		t.Helper()
		router, _ := setupProjectRouterWithIntegration(t, def, mi)
		scopeRule := project.GetEffectiveRule(def, "switchboard", "")
		handler := router.makeExecuteHandler(def, scopeRule)
		result, err := handler(context.Background(), projectToolRequest("execute", map[string]any{
			"tool_name": toolName,
			"arguments": map[string]any{},
		}))
		require.NoError(t, err)
		return result
	}

	t.Run("per-tool override allows oversize for declared tool", func(t *testing.T) {
		payload := strings.Repeat("a", 600*1024) // 600KB plain text, above default
		result := execute(t, buildIntegration("bigint_get_diff", payload), "bigint_get_diff")

		assert.False(t, result.IsError, "response within per-tool cap should succeed")
		tc := result.Content[0].(*mcpsdk.TextContent)
		assert.Equal(t, payload, tc.Text)
	})

	t.Run("per-tool override does not leak to other tools", func(t *testing.T) {
		payload := fmt.Sprintf(`{"data":"%s"}`, strings.Repeat("y", 60*1024)) // 60KB, above default
		result := execute(t, buildIntegration("bigint_get_thing", payload), "bigint_get_thing")

		assert.True(t, result.IsError, "tools without an override must use the default cap")
		tc := result.Content[0].(*mcpsdk.TextContent)
		capKB := fmt.Sprintf("%dKB", defaultMaxResponseBytes/1024)
		assert.Contains(t, tc.Text, capKB, "error should report the default cap")
	})
}

// mockProjectIntegrationWithPerToolCap implements PerToolMaxResponseBytesIntegration
// on top of *mockIntegration so the project router tests can register a fake
// integration that raises the cap for one specific tool.
type mockProjectIntegrationWithPerToolCap struct {
	*mockIntegration
	perTool map[mcp.ToolName]int
}

func (m *mockProjectIntegrationWithPerToolCap) MaxResponseBytesForTool(name mcp.ToolName) (int, bool) {
	v, ok := m.perTool[name]
	return v, ok
}

func TestProjectRouter_ContextManifest(t *testing.T) {
	def := &project.Definition{Version: "1", Name: "ctx-test"}
	router, _ := setupProjectRouter(t, def)
	handler := router.makeContextHandler(def)
	result, err := handler(context.Background(), projectToolRequest("project_context", map[string]any{}))
	require.NoError(t, err)
	require.False(t, result.IsError)
	tc := result.Content[0].(*mcpsdk.TextContent)
	var entries []project.ContextEntry
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &entries))
	assert.Empty(t, entries)
}

func TestProjectRouter_NoCrossProjectAdminTools(t *testing.T) {
	def := &project.Definition{Version: "1", Name: "p1"}
	router, _ := setupProjectRouter(t, def)
	srv, err := router.getOrCreate("p1")
	require.NoError(t, err)

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ss, err := srv.mcpSrv.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	defer ss.Close() //nolint:errcheck
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "t", Version: "0"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer cs.Close() //nolint:errcheck
	tools, err := cs.ListTools(ctx, nil)
	require.NoError(t, err)
	names := map[string]bool{}
	for _, tool := range tools.Tools {
		names[tool.Name] = true
	}
	assert.False(t, names["project_create"])
	assert.False(t, names["project_update"])
	assert.False(t, names["project_delete"])
	assert.False(t, names["project_list"])
	assert.True(t, names["project_get"])
}

func TestProjectRouter_ProjectGet(t *testing.T) {
	def := &project.Definition{Version: "1", Name: "myproj"}
	router, _ := setupProjectRouter(t, def)

	handler := router.makeProjectGetHandler(def)
	result, err := handler(context.Background(), projectToolRequest("project_get", map[string]any{}))
	require.NoError(t, err)

	tc := result.Content[0].(*mcpsdk.TextContent)
	var got project.Definition
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &got))
	assert.Equal(t, "myproj", got.Name)
}

func TestProjectRouter_FreshSnapshotAfterDelete(t *testing.T) {
	def := &project.Definition{Version: "1", Name: "deletable"}
	router, store := setupProjectRouter(t, def)
	_, err := router.getOrCreate("deletable")
	require.NoError(t, err)
	require.NoError(t, store.DeleteDefinition("deletable"))
	_, err = router.getOrCreate("deletable")
	assert.ErrorContains(t, err, "not found")
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
			{Name: mcp.ToolName("github_list_issues"), Description: "List issues"},
			{Name: mcp.ToolName("github_get_issue"), Description: "Get issue"},
			{Name: mcp.ToolName("github_delete_repo"), Description: "Delete repo"},
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

func TestProjectRouter_GlobalToolGlobsBoundSearch(t *testing.T) {
	def := &project.Definition{Version: "1", Name: "scoped"}
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("github_list_issues"), Description: "List issues"},
			{Name: mcp.ToolName("github_delete_repo"), Description: "Delete repo"},
		},
	}
	router, _ := setupProjectRouter(t, def, mi)
	router.services.Config = newMockConfigService(map[string]*mcp.IntegrationConfig{
		"github": {Enabled: true, ToolGlobs: []string{"github_list_*"}},
	})
	handler := router.makeSearchHandler(nil)
	result, err := handler(context.Background(), projectToolRequest("search", map[string]any{}))
	require.NoError(t, err)
	tc := result.Content[0].(*mcpsdk.TextContent)
	assert.Contains(t, tc.Text, "github_list_issues")
	assert.NotContains(t, tc.Text, "github_delete_repo")
}

func TestProjectRouter_Handler(t *testing.T) {
	def := &project.Definition{Version: "1", Name: "handler-test"}
	router, _ := setupProjectRouter(t, def)
	handler := router.Handler()
	assert.NotNil(t, handler)
}
