package notionmcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/notion"
	"github.com/daltoniam/switchboard/registry"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type hostedTool struct {
	name        string
	description string
	schema      any
	result      *mcpsdk.CallToolResult
	err         error
	handler     mcpsdk.ToolHandler
}

type hostedRequest struct {
	method string
	path   string
	auth   string
}

type hostedCall struct {
	name string
	args map[string]any
	auth string
}

type hostedMCP struct {
	server   *httptest.Server
	mu       sync.Mutex
	requests []hostedRequest
	calls    []hostedCall
}

func newHostedMCP(t *testing.T, path string, catalogs map[string][]hostedTool) *hostedMCP {
	t.Helper()
	fixture := &hostedMCP{}
	handlers := make(map[string]http.Handler, len(catalogs))
	for token, tools := range catalogs {
		auth := "Bearer " + token
		server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "fake-notion-mcp", Version: "1"}, nil)
		for _, tool := range tools {
			schema := tool.schema
			if schema == nil {
				schema = map[string]any{"type": "object"}
			}
			server.AddTool(&mcpsdk.Tool{Name: tool.name, Description: tool.description, InputSchema: schema}, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
				var args map[string]any
				if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
					return nil, err
				}
				fixture.mu.Lock()
				fixture.calls = append(fixture.calls, hostedCall{name: req.Params.Name, args: args, auth: auth})
				fixture.mu.Unlock()
				if tool.handler != nil {
					return tool.handler(ctx, req)
				}
				if tool.result != nil || tool.err != nil {
					return tool.result, tool.err
				}
				return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: tool.name}}}, nil
			})
		}
		handlers[auth] = mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, &mcpsdk.StreamableHTTPOptions{JSONResponse: true})
	}
	fixture.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixture.mu.Lock()
		fixture.requests = append(fixture.requests, hostedRequest{method: r.Method, path: r.URL.RequestURI(), auth: r.Header.Get("Authorization")})
		fixture.mu.Unlock()
		if r.URL.RequestURI() != path {
			http.NotFound(w, r)
			return
		}
		handler := handlers[r.Header.Get("Authorization")]
		if handler == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(fixture.server.Close)
	return fixture
}

func (f *hostedMCP) snapshot() ([]hostedRequest, []hostedCall) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]hostedRequest(nil), f.requests...), append([]hostedCall(nil), f.calls...)
}

func newIntegration(t *testing.T) mcp.Integration {
	t.Helper()
	integration := New()
	closer, ok := integration.(io.Closer)
	require.True(t, ok)
	t.Cleanup(func() { assert.NoError(t, closer.Close()) })
	return integration
}

func configure(t *testing.T, integration mcp.Integration, baseURL, token string) {
	t.Helper()
	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"base_url": baseURL, "mcp_access_token": token}))
}

func definitions(tools []mcp.ToolDefinition) map[mcp.ToolName]mcp.ToolDefinition {
	result := make(map[mcp.ToolName]mcp.ToolDefinition, len(tools))
	for _, tool := range tools {
		result[tool.Name] = tool
	}
	return result
}

func assertUnconfigured(t *testing.T, integration mcp.Integration) {
	t.Helper()
	assert.Empty(t, integration.Tools())
	assert.False(t, integration.Healthy(t.Context()))
	result, err := integration.Execute(t.Context(), "notion-mcp_notion-search", nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, mcp.ErrNotConfigured.Error())
}

func TestNew(t *testing.T) {
	integration := newIntegration(t)
	assert.Equal(t, "notion-mcp", integration.Name())
	assert.Equal(t, "https://mcp.notion.com", MCPServerURL(integration))
	assert.Equal(t, []string{"base_url"}, integration.(mcp.PlainTextCredentials).PlainTextKeys())
	assert.Equal(t, []string{"base_url"}, integration.(mcp.OptionalCredentials).OptionalKeys())
	assert.Empty(t, MCPServerURL(notion.New()))
	assert.Empty(t, MCPServerURL(nil))
	assertUnconfigured(t, integration)
}

func TestConfigureRejectsInvalidCredentials(t *testing.T) {
	tests := []struct {
		name  string
		creds mcp.Credentials
		want  string
	}{
		{name: "missing token", want: "mcp_access_token is required"},
		{name: "blank token", creds: mcp.Credentials{"mcp_access_token": " \t\n "}, want: "mcp_access_token is required"},
		{name: "legacy token", creds: mcp.Credentials{"access_token": "legacy-secret"}, want: "mcp_access_token is required"},
		{name: "relative URL", creds: mcp.Credentials{"mcp_access_token": "secret", "base_url": "/mcp"}, want: "base_url"},
		{name: "missing scheme", creds: mcp.Credentials{"mcp_access_token": "secret", "base_url": "mcp.notion.com"}, want: "base_url"},
		{name: "wrong scheme", creds: mcp.Credentials{"mcp_access_token": "secret", "base_url": "ftp://example.test"}, want: "base_url"},
		{name: "missing host", creds: mcp.Credentials{"mcp_access_token": "secret", "base_url": "https:///mcp"}, want: "base_url"},
		{name: "empty hostname", creds: mcp.Credentials{"mcp_access_token": "secret", "base_url": "http://:8080/mcp"}, want: "base_url"},
		{name: "malformed URL", creds: mcp.Credentials{"mcp_access_token": "secret", "base_url": "https://bad host/mcp"}, want: "base_url"},
		{name: "invalid escape", creds: mcp.Credentials{"mcp_access_token": "secret", "base_url": "https://example.test/%zz"}, want: "base_url"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			integration := newIntegration(t)
			err := integration.Configure(t.Context(), tt.creds)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
			assert.NotContains(t, err.Error(), "secret")
			assert.Equal(t, "https://mcp.notion.com", MCPServerURL(integration))
			assertUnconfigured(t, integration)
		})
	}
}

func TestConfigureNormalizesBaseURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "default", want: "https://mcp.notion.com"},
		{name: "blank", url: " \t ", want: "https://mcp.notion.com"},
		{name: "https", url: "https://example.test", want: "https://example.test"},
		{name: "http", url: "http://example.test:8080", want: "http://example.test:8080"},
		{name: "trailing slash", url: " https://example.test/// ", want: "https://example.test"},
		{name: "MCP endpoint", url: "https://example.test/mcp/", want: "https://example.test"},
		{name: "nested endpoint", url: "https://example.test/proxy/mcp///", want: "https://example.test/proxy"},
		{name: "query and fragment", url: "https://example.test/proxy/mcp?ignored=1#ignored", want: "https://example.test/proxy"},
		{name: "suffix only", url: "https://example.test/mcp-other", want: "https://example.test/mcp-other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			integration := newIntegration(t)
			configure(t, integration, tt.url, "token")
			assert.Equal(t, tt.want, MCPServerURL(integration))
		})
	}
}

func TestToolsDynamicCatalogAndSchema(t *testing.T) {
	catalog := []hostedTool{
		{name: "notion-search", description: "Search every accessible page.\nPreserve these upstream instructions.", schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":  map[string]any{"type": "string", "description": "Search query"},
				"filter": map[string]any{"type": "object", "description": "Nested filter", "properties": map[string]any{"type": map[string]any{"type": "string"}}},
				"limit":  map[string]any{"type": "integer"},
			},
			"required": []string{"query", "filter"},
		}},
		{name: "notion-fetch", description: "Fetch a page."},
		{name: "future_tool-v2", description: "An unanticipated upstream tool."},
		{name: "notion_search", description: "A differently named search tool."},
	}
	fixture := newHostedMCP(t, "/mcp", map[string][]hostedTool{"token": catalog})
	integration := newIntegration(t)
	configure(t, integration, " "+fixture.server.URL+"/mcp/// ", " token \n")
	assert.Equal(t, fixture.server.URL, MCPServerURL(integration))
	assert.True(t, integration.Healthy(t.Context()))
	tools := integration.Tools()
	require.Len(t, tools, len(catalog))
	byName := definitions(tools)
	require.Len(t, byName, len(catalog))
	for _, upstream := range catalog {
		name := mcp.ToolName("notion-mcp_" + upstream.name)
		require.Contains(t, byName, name)
		if upstream.name == "notion-search" {
			assert.Contains(t, byName[name].Description, "Start here")
			assert.Contains(t, byName[name].Description, upstream.description)
		} else {
			assert.Equal(t, upstream.description, byName[name].Description)
		}
	}
	search := byName["notion-mcp_notion-search"]
	assert.Equal(t, map[string]string{"query": "Search query", "filter": "Nested filter", "limit": ""}, search.Parameters)
	assert.Equal(t, []string{"query", "filter"}, search.Required)
	assert.Equal(t, tools, integration.Tools())
	requests, _ := fixture.snapshot()
	require.NotEmpty(t, requests)
	for _, request := range requests {
		assert.Equal(t, "/mcp", request.path)
		assert.Equal(t, "Bearer token", request.auth)
	}
}

func TestToolsDoesNotInventSearch(t *testing.T) {
	for _, catalog := range [][]hostedTool{nil, {{name: "future_tool", description: "Only available tool."}}} {
		name := "empty"
		if len(catalog) > 0 {
			name = "no search"
		}
		t.Run(name, func(t *testing.T) {
			fixture := newHostedMCP(t, "/mcp", map[string][]hostedTool{"token": catalog})
			integration := newIntegration(t)
			configure(t, integration, fixture.server.URL, "token")
			tools := integration.Tools()
			assert.Len(t, tools, len(catalog))
			for _, tool := range tools {
				assert.NotContains(t, tool.Description, "Start here")
			}
			result, err := integration.Execute(t.Context(), "notion-mcp_notion-search", nil)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, "unknown tool")
			_, calls := fixture.snapshot()
			assert.Empty(t, calls)
		})
	}
}

func TestToolsDoesNotMutateCachedCatalog(t *testing.T) {
	fixture := newHostedMCP(t, "/mcp", map[string][]hostedTool{"token": {{name: "notion-search", description: "Original description", schema: map[string]any{
		"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string", "description": "Query"}}, "required": []string{"query"},
	}}}})
	integration := newIntegration(t)
	configure(t, integration, fixture.server.URL, "token")
	first := integration.Tools()
	require.Len(t, first, 1)
	first[0].Name = "notion-mcp_injected"
	first[0].Description = "changed"
	first[0].Parameters["query"] = "changed"
	first[0].Required[0] = "changed"
	second := integration.Tools()
	require.Len(t, second, 1)
	assert.Equal(t, mcp.ToolName("notion-mcp_notion-search"), second[0].Name)
	assert.Contains(t, second[0].Description, "Original description")
	assert.Equal(t, 1, strings.Count(second[0].Description, "Start here"))
	assert.Equal(t, "Query", second[0].Parameters["query"])
	assert.Equal(t, []string{"query"}, second[0].Required)
	result, err := integration.Execute(t.Context(), "notion-mcp_injected", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	_, calls := fixture.snapshot()
	assert.Empty(t, calls)
}

func TestExecutePassesArgumentsAndResults(t *testing.T) {
	tests := []struct {
		name   string
		args   map[string]any
		result *mcpsdk.CallToolResult
		want   *mcp.ToolResult
	}{
		{name: "nested JSON", args: map[string]any{"query": "design", "filter": map[string]any{"ids": []any{"a", "b"}, "archived": false}, "limit": float64(3), "nullable": nil}, result: &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: `{"pages":[{"id":"a"}]}`}}}, want: &mcp.ToolResult{Data: `{"pages":[{"id":"a"}]}`}},
		{name: "nil args and text", result: &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "first"}, &mcpsdk.TextContent{Text: "second"}}}, want: &mcp.ToolResult{Data: "first\nsecond"}},
		{name: "tool error", args: map[string]any{}, result: &mcpsdk.CallToolResult{IsError: true, Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "permission denied"}}}, want: &mcp.ToolResult{Data: "permission denied", IsError: true}},
		{name: "media", result: &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "preview"}, &mcpsdk.ImageContent{Data: []byte("image"), MIMEType: "image/png"}}}, want: &mcp.ToolResult{Data: "preview", Media: []mcp.MediaContent{{Data: []byte("image"), MIMEType: "image/png"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newHostedMCP(t, "/mcp", map[string][]hostedTool{"token": {{name: "notion-fetch", result: tt.result}}})
			integration := newIntegration(t)
			configure(t, integration, fixture.server.URL, "token")
			original, err := json.Marshal(tt.args)
			require.NoError(t, err)
			result, err := integration.Execute(t.Context(), "notion-mcp_notion-fetch", tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
			after, err := json.Marshal(tt.args)
			require.NoError(t, err)
			assert.Equal(t, original, after)
			_, calls := fixture.snapshot()
			require.Len(t, calls, 1)
			assert.Equal(t, "notion-fetch", calls[0].name)
			assert.Equal(t, "Bearer token", calls[0].auth)
			if tt.args == nil {
				assert.Equal(t, map[string]any{}, calls[0].args)
			} else {
				assert.Equal(t, tt.args, calls[0].args)
			}
		})
	}
}

func TestExecuteProtocolError(t *testing.T) {
	fixture := newHostedMCP(t, "/mcp", map[string][]hostedTool{"token": {{name: "notion-fetch", err: errors.New("upstream protocol failure")}}})
	integration := newIntegration(t)
	configure(t, integration, fixture.server.URL, "token")
	result, err := integration.Execute(t.Context(), "notion-mcp_notion-fetch", nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "upstream protocol failure")
}

func TestUnavailableUpstream(t *testing.T) {
	fixture := newHostedMCP(t, "/mcp", nil)
	integration := newIntegration(t)
	configure(t, integration, fixture.server.URL, "unauthorized-token")
	assert.False(t, integration.Healthy(t.Context()))
	assert.Empty(t, integration.Tools())
	result, err := integration.Execute(t.Context(), "notion-mcp_notion-search", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	_, calls := fixture.snapshot()
	assert.Empty(t, calls)
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	catalog := []hostedTool{{name: "notion-search"}, {name: "notion-fetch"}, {name: "future_tool-v2"}, {name: "notion-mcp_already-prefixed"}}
	fixture := newHostedMCP(t, "/mcp", map[string][]hostedTool{"token": catalog})
	integration := newIntegration(t)
	configure(t, integration, fixture.server.URL, "token")
	tools := integration.Tools()
	require.Len(t, tools, len(catalog))
	for _, tool := range tools {
		t.Run(string(tool.Name), func(t *testing.T) {
			result, err := integration.Execute(t.Context(), tool.Name, nil)
			require.NoError(t, err)
			assert.False(t, result.IsError)
			assert.Equal(t, strings.TrimPrefix(string(tool.Name), "notion-mcp_"), result.Data)
		})
	}
	_, calls := fixture.snapshot()
	require.Len(t, calls, len(catalog))
	byName := definitions(tools)
	for _, call := range calls {
		assert.Contains(t, byName, mcp.ToolName("notion-mcp_"+call.name))
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	fixture := newHostedMCP(t, "/mcp", map[string][]hostedTool{"token": {{name: "notion-fetch"}}})
	integration := newIntegration(t)
	configure(t, integration, fixture.server.URL, "token")
	for _, name := range []mcp.ToolName{"", "notion-fetch", "notion_retrieve_page", "figma_notion-fetch", "notionmcp_notion-fetch", "notion-mcp", "notion-mcp_", "notion-mcp_notion-search", "notion-mcp_unknown", "notion-mcp_notion-mcp_notion-fetch"} {
		t.Run(string(name), func(t *testing.T) {
			result, err := integration.Execute(t.Context(), name, nil)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, "unknown tool")
		})
	}
	_, calls := fixture.snapshot()
	assert.Empty(t, calls)
}

func TestConfigureRotatesTokenAndCatalog(t *testing.T) {
	fixture := newHostedMCP(t, "/mcp", map[string][]hostedTool{
		"first":  {{name: "notion-search"}},
		"second": {{name: "future_tool-v2"}},
	})
	integration := newIntegration(t)
	configure(t, integration, fixture.server.URL, "first")
	require.Contains(t, definitions(integration.Tools()), mcp.ToolName("notion-mcp_notion-search"))
	remote := integration.(*notionmcp).remote
	configure(t, integration, fixture.server.URL+"/mcp/", "first")
	assert.Same(t, remote, integration.(*notionmcp).remote)
	requests, _ := fixture.snapshot()
	for _, request := range requests {
		assert.NotEqual(t, http.MethodDelete, request.method)
	}
	configure(t, integration, fixture.server.URL, "second")
	assert.Same(t, remote, integration.(*notionmcp).remote)
	assert.Equal(t, []mcp.ToolDefinition{{Name: "notion-mcp_future_tool-v2", Parameters: map[string]string{}}}, integration.Tools())
	result, err := integration.Execute(t.Context(), "notion-mcp_notion-search", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	result, err = integration.Execute(t.Context(), "notion-mcp_future_tool-v2", nil)
	require.NoError(t, err)
	assert.Equal(t, "future_tool-v2", result.Data)
	requests, calls := fixture.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, "Bearer second", calls[0].auth)
	assert.Contains(t, requests, hostedRequest{method: http.MethodDelete, path: "/mcp", auth: "Bearer first"})
}

func TestConfigureReplacesEndpointAndClose(t *testing.T) {
	first := newHostedMCP(t, "/mcp", map[string][]hostedTool{"token": {{name: "notion-search"}}})
	second := newHostedMCP(t, "/proxy/mcp", map[string][]hostedTool{"token": {{name: "notion-fetch"}}})
	integration := newIntegration(t)
	configure(t, integration, first.server.URL, "token")
	require.Len(t, integration.Tools(), 1)
	configure(t, integration, second.server.URL+"/proxy/mcp/", "token")
	assert.Equal(t, second.server.URL+"/proxy", MCPServerURL(integration))
	requests, _ := first.snapshot()
	assert.Contains(t, requests, hostedRequest{method: http.MethodDelete, path: "/mcp", auth: "Bearer token"})
	result, err := integration.Execute(t.Context(), "notion-mcp_notion-search", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	result, err = integration.Execute(t.Context(), "notion-mcp_notion-fetch", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "notion-fetch", result.Data)
	closer := integration.(io.Closer)
	require.NoError(t, closer.Close())
	require.NoError(t, closer.Close())
	assertUnconfigured(t, integration)
	requests, _ = second.snapshot()
	assert.Contains(t, requests, hostedRequest{method: http.MethodDelete, path: "/proxy/mcp", auth: "Bearer token"})
	deletes := 0
	for _, request := range requests {
		assert.Equal(t, "/proxy/mcp", request.path)
		if request.method == http.MethodDelete {
			deletes++
		}
	}
	assert.Equal(t, 1, deletes)
	configure(t, integration, first.server.URL, "token")
	assert.True(t, integration.Healthy(t.Context()))
	require.Contains(t, definitions(integration.Tools()), mcp.ToolName("notion-mcp_notion-search"))
}

func TestInvalidReconfigurePreservesConfiguration(t *testing.T) {
	fixture := newHostedMCP(t, "/mcp", map[string][]hostedTool{"token": {{name: "notion-search"}}})
	integration := newIntegration(t)
	configure(t, integration, fixture.server.URL, "token")
	require.Len(t, integration.Tools(), 1)
	for _, creds := range []mcp.Credentials{nil, {"mcp_access_token": "other", "base_url": "invalid"}} {
		require.Error(t, integration.Configure(t.Context(), creds))
		assert.Equal(t, fixture.server.URL, MCPServerURL(integration))
		result, err := integration.Execute(t.Context(), "notion-mcp_notion-search", nil)
		require.NoError(t, err)
		assert.False(t, result.IsError)
	}
}

func TestRegistryCoexistsWithNotion(t *testing.T) {
	fixture := newHostedMCP(t, "/mcp", map[string][]hostedTool{"token": {{name: "notion-search"}}})
	hosted := newIntegration(t)
	configure(t, hosted, fixture.server.URL, "token")
	legacy := notion.New()
	reg := registry.New()
	require.NoError(t, reg.Register(legacy))
	require.NoError(t, reg.Register(hosted))
	assert.Equal(t, []string{"notion", "notion-mcp"}, reg.Names())
	seen := make(map[mcp.ToolName]bool)
	for _, integration := range reg.All() {
		registered, ok := reg.Get(integration.Name())
		require.True(t, ok)
		assert.Same(t, integration, registered)
		for _, tool := range integration.Tools() {
			assert.False(t, seen[tool.Name], "duplicate tool %s", tool.Name)
			seen[tool.Name] = true
			prefix, _, ok := strings.Cut(string(tool.Name), "_")
			require.True(t, ok)
			owner, ok := reg.Get(prefix)
			require.True(t, ok)
			assert.Same(t, integration, owner)
		}
	}
	assert.Contains(t, seen, mcp.ToolName("notion_search"))
	assert.Contains(t, seen, mcp.ToolName("notion-mcp_notion-search"))
	result, err := hosted.Execute(t.Context(), "notion_search", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	_, calls := fixture.snapshot()
	assert.Empty(t, calls)
}

func TestExecuteConcurrentCalls(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	fixture := newHostedMCP(t, "/mcp", map[string][]hostedTool{"token": {{name: "notion-fetch", handler: func(ctx context.Context, _ *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		entered <- struct{}{}
		select {
		case <-release:
			return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "done"}}}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}}})
	integration := newIntegration(t)
	configure(t, integration, fixture.server.URL, "token")
	require.Len(t, integration.Tools(), 1)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	var workers sync.WaitGroup
	defer workers.Wait()
	defer close(release)
	for range 2 {
		workers.Go(func() {
			result, err := integration.Execute(ctx, "notion-mcp_notion-fetch", nil)
			if assert.NoError(t, err) && assert.NotNil(t, result) {
				assert.False(t, result.IsError)
				assert.Equal(t, "done", result.Data)
			}
		})
	}
	for range 2 {
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal("tool calls did not reach upstream concurrently")
		}
	}
}

func TestConcurrentLifecycle(t *testing.T) {
	catalog := []hostedTool{{name: "notion-search"}, {name: "future_tool-v2"}}
	first := newHostedMCP(t, "/mcp", map[string][]hostedTool{"first": catalog, "second": catalog})
	second := newHostedMCP(t, "/mcp", map[string][]hostedTool{"first": catalog, "second": catalog})
	integration := newIntegration(t)
	configure(t, integration, first.server.URL, "first")
	var workers sync.WaitGroup
	for worker := range 6 {
		workers.Go(func() {
			for iteration := range 12 {
				switch worker {
				case 0:
					baseURL, token := first.server.URL, "first"
					if iteration%2 == 0 {
						baseURL, token = second.server.URL, "second"
					}
					assert.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"base_url": baseURL, "mcp_access_token": token}))
				case 1:
					assert.NoError(t, integration.(io.Closer).Close())
				case 2:
					for _, tool := range integration.Tools() {
						assert.Contains(t, []mcp.ToolName{"notion-mcp_notion-search", "notion-mcp_future_tool-v2"}, tool.Name)
					}
				case 3:
					integration.Healthy(t.Context())
				case 4:
					result, err := integration.Execute(t.Context(), "notion-mcp_future_tool-v2", nil)
					if assert.NoError(t, err) && assert.NotNil(t, result) {
						if result.IsError {
							assert.Contains(t, result.Data, mcp.ErrNotConfigured.Error())
						} else {
							assert.Equal(t, "future_tool-v2", result.Data)
						}
					}
				case 5:
					assert.Contains(t, []string{first.server.URL, second.server.URL}, MCPServerURL(integration))
				}
			}
		})
	}
	workers.Wait()
	configure(t, integration, first.server.URL, "first")
	assert.True(t, integration.Healthy(t.Context()))
	assert.Len(t, integration.Tools(), len(catalog))
}
