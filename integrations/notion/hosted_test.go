package notion

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
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeHostedTool struct {
	name        string
	description string
	schema      any
	result      *mcpsdk.CallToolResult
	err         error
	handler     mcpsdk.ToolHandler
}

type fakeHostedRequest struct {
	method string
	path   string
	auth   string
}

type fakeHostedCall struct {
	name string
	args map[string]any
	auth string
}

type fakeHostedMCP struct {
	server   *httptest.Server
	mu       sync.Mutex
	requests []fakeHostedRequest
	calls    []fakeHostedCall
}

func newFakeHostedMCP(t *testing.T, path string, catalogs map[string][]fakeHostedTool) *fakeHostedMCP {
	t.Helper()
	fixture := &fakeHostedMCP{}
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
				fixture.calls = append(fixture.calls, fakeHostedCall{name: req.Params.Name, args: args, auth: auth})
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
		fixture.requests = append(fixture.requests, fakeHostedRequest{method: r.Method, path: r.URL.RequestURI(), auth: r.Header.Get("Authorization")})
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

func (f *fakeHostedMCP) snapshot() ([]fakeHostedRequest, []fakeHostedCall) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeHostedRequest(nil), f.requests...), append([]fakeHostedCall(nil), f.calls...)
}

func newHostedIntegration(t *testing.T) mcp.Integration {
	t.Helper()
	integration := New()
	closer, ok := integration.(io.Closer)
	require.True(t, ok)
	t.Cleanup(func() { assert.NoError(t, closer.Close()) })
	return integration
}

func configureHosted(t *testing.T, integration mcp.Integration, baseURL, token string) {
	t.Helper()
	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"mcp_base_url": baseURL, "mcp_access_token": token}))
}

func hostedDefinitions(tools []mcp.ToolDefinition) map[mcp.ToolName]mcp.ToolDefinition {
	result := make(map[mcp.ToolName]mcp.ToolDefinition, len(tools))
	for _, tool := range tools {
		result[tool.Name] = tool
	}
	return result
}

func assertHostedInactive(t *testing.T, integration mcp.Integration) {
	t.Helper()
	assert.Equal(t, tools, integration.Tools())
	result, err := integration.Execute(t.Context(), "notion_notion-search", nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestHostedNewDefaults(t *testing.T) {
	integration := newHostedIntegration(t)
	assert.Equal(t, "notion", integration.Name())
	assert.Equal(t, "https://mcp.notion.com", MCPServerURL(integration))
	assert.Equal(t, []string{"mcp_base_url"}, integration.(mcp.PlainTextCredentials).PlainTextKeys())
	assert.Equal(t, []string{"mcp_base_url"}, integration.(mcp.OptionalCredentials).OptionalKeys())
	assert.Empty(t, MCPServerURL(nil))
	assertHostedInactive(t, integration)
}

func TestHostedConfigureRejectsInvalidBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{name: "relative URL", baseURL: "/mcp"},
		{name: "missing scheme", baseURL: "mcp.notion.com"},
		{name: "wrong scheme", baseURL: "ftp://example.test"},
		{name: "missing host", baseURL: "https:///mcp"},
		{name: "empty hostname", baseURL: "http://:8080/mcp"},
		{name: "malformed URL", baseURL: "https://bad host/mcp"},
		{name: "invalid escape", baseURL: "https://example.test/%zz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			integration := newHostedIntegration(t)
			err := integration.Configure(t.Context(), mcp.Credentials{"mcp_access_token": "secret", "mcp_base_url": tt.baseURL})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "mcp_base_url")
			assert.NotContains(t, err.Error(), "secret")
			assert.Equal(t, "https://mcp.notion.com", MCPServerURL(integration))
			assertHostedInactive(t, integration)
		})
	}
}

func TestHostedConfigureMissingCredentials(t *testing.T) {
	integration := newHostedIntegration(t)
	for _, creds := range []mcp.Credentials{nil, {"mcp_access_token": " \t\n "}} {
		err := integration.Configure(t.Context(), creds)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mcp_access_token")
	}
}

func TestHostedConfigureNormalizesBaseURL(t *testing.T) {
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
			integration := newHostedIntegration(t)
			configureHosted(t, integration, tt.url, "token")
			assert.Equal(t, tt.want, MCPServerURL(integration))
		})
	}
}

func TestHostedCredentialPrecedence(t *testing.T) {
	fixture := newFakeHostedMCP(t, "/mcp", map[string][]fakeHostedTool{"token": {{name: "notion-search"}}})
	integration := newHostedIntegration(t)
	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{
		"mcp_access_token": "token",
		"mcp_base_url":     fixture.server.URL,
		"access_token":     "ignored-v1-token",
		"token_v2":         "ignored-cookie",
	}))
	assert.Equal(t, []mcp.ToolName{"notion_notion-search"}, toolNames(integration.Tools()))

	err := integration.Configure(t.Context(), mcp.Credentials{"mcp_base_url": fixture.server.URL})
	require.Error(t, err)
	assertHostedInactive(t, integration)
	requests, _ := fixture.snapshot()
	assert.Contains(t, requests, fakeHostedRequest{method: http.MethodDelete, path: "/mcp", auth: "Bearer token"})
}

func TestHostedModeDisablesNativeCompaction(t *testing.T) {
	fixture := newFakeHostedMCP(t, "/mcp", map[string][]fakeHostedTool{"token": {{name: "search"}, {name: "get_page_content"}}})
	integration := newHostedIntegration(t)
	configureHosted(t, integration, fixture.server.URL, "token")
	for _, name := range []mcp.ToolName{"notion_search", "notion_get_page_content"} {
		t.Run(string(name), func(t *testing.T) {
			_, ok := integration.(mcp.FieldCompactionIntegration).CompactSpec(name)
			assert.False(t, ok)
			_, ok = integration.(mcp.ToolMaxBytesIntegration).MaxBytes(name)
			assert.False(t, ok)
			_, ok = integration.(*notion).Views(name)
			assert.False(t, ok)
		})
	}
	require.NoError(t, integration.(io.Closer).Close())
	_, ok := integration.(mcp.FieldCompactionIntegration).CompactSpec("notion_search")
	assert.True(t, ok)
}

func toolNames(tools []mcp.ToolDefinition) []mcp.ToolName {
	names := make([]mcp.ToolName, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

func TestHostedToolsDynamicCatalogAndSchema(t *testing.T) {
	catalog := []fakeHostedTool{
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
	}
	fixture := newFakeHostedMCP(t, "/mcp", map[string][]fakeHostedTool{"token": catalog})
	integration := newHostedIntegration(t)
	configureHosted(t, integration, " "+fixture.server.URL+"/mcp/// ", " token \n")
	assert.Equal(t, fixture.server.URL, MCPServerURL(integration))
	assert.True(t, integration.Healthy(t.Context()))
	tools := integration.Tools()
	require.Len(t, tools, len(catalog))
	byName := hostedDefinitions(tools)
	for _, upstream := range catalog {
		name := mcp.ToolName("notion_" + upstream.name)
		require.Contains(t, byName, name)
		if upstream.name == "notion-search" {
			assert.Contains(t, byName[name].Description, "Start here")
			assert.Contains(t, byName[name].Description, upstream.description)
		} else {
			assert.Equal(t, upstream.description, byName[name].Description)
		}
	}
	search := byName["notion_notion-search"]
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

func TestHostedToolsDoesNotMutateCachedCatalog(t *testing.T) {
	fixture := newFakeHostedMCP(t, "/mcp", map[string][]fakeHostedTool{"token": {{name: "notion-search", description: "Original description", schema: map[string]any{
		"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string", "description": "Query"}}, "required": []string{"query"},
	}}}})
	integration := newHostedIntegration(t)
	configureHosted(t, integration, fixture.server.URL, "token")
	first := integration.Tools()
	require.Len(t, first, 1)
	first[0].Name = "notion_injected"
	first[0].Description = "changed"
	first[0].Parameters["query"] = "changed"
	first[0].Required[0] = "changed"
	second := integration.Tools()
	require.Len(t, second, 1)
	assert.Equal(t, mcp.ToolName("notion_notion-search"), second[0].Name)
	assert.Contains(t, second[0].Description, "Original description")
	assert.Equal(t, 1, strings.Count(second[0].Description, "Start here"))
	assert.Equal(t, "Query", second[0].Parameters["query"])
	assert.Equal(t, []string{"query"}, second[0].Required)
}

func TestHostedExecutePassesArgumentsAndResults(t *testing.T) {
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
			fixture := newFakeHostedMCP(t, "/mcp", map[string][]fakeHostedTool{"token": {{name: "notion-fetch", result: tt.result}}})
			integration := newHostedIntegration(t)
			configureHosted(t, integration, fixture.server.URL, "token")
			original, err := json.Marshal(tt.args)
			require.NoError(t, err)
			result, err := integration.Execute(t.Context(), "notion_notion-fetch", tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
			after, err := json.Marshal(tt.args)
			require.NoError(t, err)
			assert.Equal(t, original, after)
			_, calls := fixture.snapshot()
			require.Len(t, calls, 1)
			assert.Equal(t, "notion-fetch", calls[0].name)
			assert.Equal(t, "Bearer token", calls[0].auth)
		})
	}
}

func TestHostedExecuteProtocolError(t *testing.T) {
	fixture := newFakeHostedMCP(t, "/mcp", map[string][]fakeHostedTool{"token": {{name: "notion-fetch", err: errors.New("upstream protocol failure")}}})
	integration := newHostedIntegration(t)
	configureHosted(t, integration, fixture.server.URL, "token")
	result, err := integration.Execute(t.Context(), "notion_notion-fetch", nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "upstream protocol failure")
}

func TestHostedUnavailableUpstream(t *testing.T) {
	fixture := newFakeHostedMCP(t, "/mcp", nil)
	integration := newHostedIntegration(t)
	configureHosted(t, integration, fixture.server.URL, "unauthorized-token")
	assert.False(t, integration.Healthy(t.Context()))
	assert.Empty(t, integration.Tools())
	result, err := integration.Execute(t.Context(), "notion_notion-search", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	_, calls := fixture.snapshot()
	assert.Empty(t, calls)
}

func TestHostedDispatch_AllToolsCovered(t *testing.T) {
	catalog := []fakeHostedTool{{name: "notion-search"}, {name: "notion-fetch"}, {name: "future_tool-v2"}}
	fixture := newFakeHostedMCP(t, "/mcp", map[string][]fakeHostedTool{"token": catalog})
	integration := newHostedIntegration(t)
	configureHosted(t, integration, fixture.server.URL, "token")
	tools := integration.Tools()
	require.Len(t, tools, len(catalog))
	for _, tool := range tools {
		t.Run(string(tool.Name), func(t *testing.T) {
			result, err := integration.Execute(t.Context(), tool.Name, nil)
			require.NoError(t, err)
			assert.False(t, result.IsError)
			assert.Equal(t, strings.TrimPrefix(string(tool.Name), "notion_"), result.Data)
		})
	}
	_, calls := fixture.snapshot()
	require.Len(t, calls, len(catalog))
}

func TestHostedDispatch_NoOrphanHandlers(t *testing.T) {
	fixture := newFakeHostedMCP(t, "/mcp", map[string][]fakeHostedTool{"token": {{name: "notion-fetch"}}})
	integration := newHostedIntegration(t)
	configureHosted(t, integration, fixture.server.URL, "token")
	for _, name := range []mcp.ToolName{"", "notion-fetch", "notion_retrieve_page", "notion_search", "figma_notion-fetch", "notion-mcp_notion-fetch", "notion_", "notion_notion-search", "notion_notion_notion-fetch"} {
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

func TestHostedConfigureRotatesTokenAndCatalog(t *testing.T) {
	fixture := newFakeHostedMCP(t, "/mcp", map[string][]fakeHostedTool{
		"first":  {{name: "notion-search"}},
		"second": {{name: "future_tool-v2"}},
	})
	integration := newHostedIntegration(t)
	configureHosted(t, integration, fixture.server.URL, "first")
	require.Contains(t, hostedDefinitions(integration.Tools()), mcp.ToolName("notion_notion-search"))
	remote := integration.(*notion).hosted.remote
	configureHosted(t, integration, fixture.server.URL+"/mcp/", "first")
	assert.Same(t, remote, integration.(*notion).hosted.remote)
	configureHosted(t, integration, fixture.server.URL, "second")
	assert.Same(t, remote, integration.(*notion).hosted.remote)
	assert.Equal(t, []mcp.ToolDefinition{{Name: "notion_future_tool-v2", Parameters: map[string]string{}}}, integration.Tools())
	result, err := integration.Execute(t.Context(), "notion_future_tool-v2", nil)
	require.NoError(t, err)
	assert.Equal(t, "future_tool-v2", result.Data)
	requests, calls := fixture.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, "Bearer second", calls[0].auth)
	assert.Contains(t, requests, fakeHostedRequest{method: http.MethodDelete, path: "/mcp", auth: "Bearer first"})
}

func TestHostedConfigureReplacesEndpointAndClose(t *testing.T) {
	first := newFakeHostedMCP(t, "/mcp", map[string][]fakeHostedTool{"token": {{name: "notion-search"}}})
	second := newFakeHostedMCP(t, "/proxy/mcp", map[string][]fakeHostedTool{"token": {{name: "notion-fetch"}}})
	integration := newHostedIntegration(t)
	configureHosted(t, integration, first.server.URL, "token")
	require.Len(t, integration.Tools(), 1)
	configureHosted(t, integration, second.server.URL+"/proxy/mcp/", "token")
	assert.Equal(t, second.server.URL+"/proxy", MCPServerURL(integration))
	requests, _ := first.snapshot()
	assert.Contains(t, requests, fakeHostedRequest{method: http.MethodDelete, path: "/mcp", auth: "Bearer token"})
	result, err := integration.Execute(t.Context(), "notion_notion-fetch", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	closer := integration.(io.Closer)
	require.NoError(t, closer.Close())
	require.NoError(t, closer.Close())
	assertHostedInactive(t, integration)
	requests, _ = second.snapshot()
	deletes := 0
	for _, request := range requests {
		if request.method == http.MethodDelete {
			deletes++
		}
	}
	assert.Equal(t, 1, deletes)
	configureHosted(t, integration, first.server.URL, "token")
	assert.True(t, integration.Healthy(t.Context()))
	require.Contains(t, hostedDefinitions(integration.Tools()), mcp.ToolName("notion_notion-search"))
}

func TestHostedInvalidReconfigurePreservesSession(t *testing.T) {
	fixture := newFakeHostedMCP(t, "/mcp", map[string][]fakeHostedTool{"token": {{name: "notion-search"}}})
	integration := newHostedIntegration(t)
	configureHosted(t, integration, fixture.server.URL, "token")
	require.Len(t, integration.Tools(), 1)
	require.Error(t, integration.Configure(t.Context(), mcp.Credentials{"mcp_access_token": "other", "mcp_base_url": "invalid"}))
	assert.Equal(t, fixture.server.URL, MCPServerURL(integration))
	result, err := integration.Execute(t.Context(), "notion_notion-search", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestHostedExecuteConcurrentCalls(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	fixture := newFakeHostedMCP(t, "/mcp", map[string][]fakeHostedTool{"token": {{name: "notion-fetch", handler: func(ctx context.Context, _ *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		entered <- struct{}{}
		select {
		case <-release:
			return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "done"}}}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}}})
	integration := newHostedIntegration(t)
	configureHosted(t, integration, fixture.server.URL, "token")
	require.Len(t, integration.Tools(), 1)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	var workers sync.WaitGroup
	defer workers.Wait()
	defer close(release)
	for range 2 {
		workers.Go(func() {
			result, err := integration.Execute(ctx, "notion_notion-fetch", nil)
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

func TestHostedConcurrentLifecycle(t *testing.T) {
	catalog := []fakeHostedTool{{name: "notion-search"}, {name: "future_tool-v2"}}
	first := newFakeHostedMCP(t, "/mcp", map[string][]fakeHostedTool{"first": catalog, "second": catalog})
	second := newFakeHostedMCP(t, "/mcp", map[string][]fakeHostedTool{"first": catalog, "second": catalog})
	integration := newHostedIntegration(t)
	configureHosted(t, integration, first.server.URL, "first")
	var workers sync.WaitGroup
	for worker := range 5 {
		workers.Go(func() {
			for iteration := range 12 {
				switch worker {
				case 0:
					baseURL, token := first.server.URL, "first"
					if iteration%2 == 0 {
						baseURL, token = second.server.URL, "second"
					}
					assert.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"mcp_base_url": baseURL, "mcp_access_token": token}))
				case 1:
					assert.NoError(t, integration.(io.Closer).Close())
				case 2:
					integration.Healthy(t.Context())
				case 3:
					result, err := integration.Execute(t.Context(), "notion_future_tool-v2", nil)
					if assert.NoError(t, err) && assert.NotNil(t, result) && !result.IsError {
						assert.Equal(t, "future_tool-v2", result.Data)
					}
				case 4:
					assert.Contains(t, []string{first.server.URL, second.server.URL}, MCPServerURL(integration))
				}
			}
		})
	}
	workers.Wait()
	configureHosted(t, integration, first.server.URL, "first")
	assert.True(t, integration.Healthy(t.Context()))
	assert.Len(t, integration.Tools(), len(catalog))
}
