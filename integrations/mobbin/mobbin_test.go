package mobbin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type hostedCall struct {
	name string
	args map[string]any
}

// hostedMobbin is a fake of Mobbin's hosted MCP server at <url>/mcp. Each
// tool answers with its own name so tests can see which upstream tool ran.
type hostedMobbin struct {
	server *httptest.Server
	mu     sync.Mutex
	calls  []hostedCall
	auths  []string
}

func newHostedMobbin(t *testing.T, tokens ...string) *hostedMobbin {
	t.Helper()
	fixture := &hostedMobbin{}
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "fake-mobbin", Version: "1"}, nil)
	for _, tool := range upstreamToolNames() {
		server.AddTool(&mcpsdk.Tool{Name: tool, InputSchema: map[string]any{"type": "object"}}, func(_ context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			var args map[string]any
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return nil, err
			}
			fixture.mu.Lock()
			fixture.calls = append(fixture.calls, hostedCall{name: req.Params.Name, args: args})
			fixture.mu.Unlock()
			return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: req.Params.Name},
				&mcpsdk.ImageContent{Data: []byte("img"), MIMEType: "image/webp"},
			}}, nil
		})
	}
	handler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, &mcpsdk.StreamableHTTPOptions{JSONResponse: true})
	allowed := map[string]bool{}
	for _, token := range tokens {
		allowed["Bearer "+token] = true
	}
	fixture.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixture.mu.Lock()
		fixture.auths = append(fixture.auths, r.Header.Get("Authorization"))
		fixture.mu.Unlock()
		if r.URL.Path != "/mcp" {
			http.NotFound(w, r)
			return
		}
		if !allowed[r.Header.Get("Authorization")] {
			http.Error(w, `{"error":{"code":"unauthorized"}}`, http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(fixture.server.Close)
	return fixture
}

func (f *hostedMobbin) snapshot() ([]hostedCall, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]hostedCall(nil), f.calls...), append([]string(nil), f.auths...)
}

func upstreamToolNames() []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, strings.TrimPrefix(string(tool.Name), toolPrefix))
	}
	return names
}

func newIntegration(t *testing.T) mcp.Integration {
	t.Helper()
	integration := New()
	closer, ok := integration.(io.Closer)
	require.True(t, ok)
	t.Cleanup(func() { assert.NoError(t, closer.Close()) })
	return integration
}

func TestNew(t *testing.T) {
	integration := New()
	require.NotNil(t, integration)
	assert.Equal(t, "mobbin", integration.Name())
	assert.Equal(t, defaultBaseURL, MCPServerURL(integration))
}

func TestConfigure(t *testing.T) {
	for _, tc := range []struct {
		name    string
		creds   mcp.Credentials
		wantErr string
		wantURL string
	}{
		{name: "missing token", creds: mcp.Credentials{}, wantErr: "mcp_access_token is required"},
		{name: "blank token", creds: mcp.Credentials{"mcp_access_token": "  "}, wantErr: "mcp_access_token is required"},
		{name: "invalid base url", creds: mcp.Credentials{"mcp_access_token": "t", "base_url": "ftp://x"}, wantErr: "invalid base_url"},
		{name: "default url", creds: mcp.Credentials{"mcp_access_token": "t"}, wantURL: defaultBaseURL},
		{name: "full endpoint", creds: mcp.Credentials{"mcp_access_token": "t", "base_url": "https://proxy.example/mcp/"}, wantURL: "https://proxy.example"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			integration := newIntegration(t)
			err := integration.Configure(t.Context(), tc.creds)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantURL, MCPServerURL(integration))
		})
	}
}

func TestTools_Metadata(t *testing.T) {
	integration := New()
	seen := map[mcp.ToolName]bool{}
	for _, tool := range integration.Tools() {
		t.Run(string(tool.Name), func(t *testing.T) {
			assert.True(t, strings.HasPrefix(string(tool.Name), "mobbin_"))
			assert.NotEmpty(t, tool.Description)
			assert.True(t, tool.Access.Declared(), "access must be classified")
			assert.False(t, seen[tool.Name], "duplicate tool")
			seen[tool.Name] = true
			for _, required := range tool.Required {
				assert.Contains(t, tool.Parameters, required)
			}
		})
	}
	assert.Len(t, seen, 8)
	assert.Contains(t, toolByName("mobbin_search_screens").Description, "Start here")
}

func TestTools_Access(t *testing.T) {
	for name, want := range map[mcp.ToolName]mcp.ToolAccess{
		"mobbin_search_screens":          mcp.AccessRead,
		"mobbin_search_flows":            mcp.AccessRead,
		"mobbin_search_sections":         mcp.AccessRead,
		"mobbin_list_collections":        mcp.AccessRead,
		"mobbin_get_collection_contents": mcp.AccessRead,
		"mobbin_create_collection":       mcp.AccessWrite,
		"mobbin_save_content":            mcp.AccessWrite,
		"mobbin_add_comment":             mcp.AccessWrite,
	} {
		t.Run(string(name), func(t *testing.T) {
			assert.Equal(t, want, toolByName(name).Access)
		})
	}
}

func toolByName(name mcp.ToolName) mcp.ToolDefinition {
	for _, tool := range tools {
		if tool.Name == name {
			return tool
		}
	}
	return mcp.ToolDefinition{}
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range tools {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch entry", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	defined := map[mcp.ToolName]bool{}
	for _, tool := range tools {
		defined[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, defined[name], "dispatch entry %s has no tool definition", name)
	}
}

func TestExecute_ForwardsEveryToolUpstream(t *testing.T) {
	fixture := newHostedMobbin(t, "token")
	integration := newIntegration(t)
	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"mcp_access_token": "token", "base_url": fixture.server.URL}))
	for _, tool := range integration.Tools() {
		t.Run(string(tool.Name), func(t *testing.T) {
			result, err := integration.Execute(t.Context(), tool.Name, map[string]any{"query": "paywall"})
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			assert.Equal(t, strings.TrimPrefix(string(tool.Name), toolPrefix), result.Data)
			require.Len(t, result.Media, 1)
			assert.Equal(t, "image/webp", result.Media[0].MIMEType)
		})
	}
	calls, auths := fixture.snapshot()
	assert.Len(t, calls, len(tools))
	for _, call := range calls {
		assert.Equal(t, "paywall", call.args["query"])
	}
	for _, auth := range auths {
		assert.Equal(t, "Bearer token", auth)
	}
}

func TestExecute_UnknownAndUnconfigured(t *testing.T) {
	fixture := newHostedMobbin(t, "token")
	unconfigured := newIntegration(t)
	result, err := unconfigured.Execute(t.Context(), "mobbin_search_screens", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, mcp.ErrNotConfigured.Error())
	assert.False(t, unconfigured.Healthy(t.Context()))

	integration := newIntegration(t)
	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"mcp_access_token": "token", "base_url": fixture.server.URL}))
	for _, name := range []mcp.ToolName{"", "mobbin_", "search_screens", "mobbin_delete_collection", "mobbin_mobbin_search_screens", "notion-mcp_search"} {
		t.Run(string(name), func(t *testing.T) {
			result, err := integration.Execute(t.Context(), name, nil)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, "unknown tool")
		})
	}
	calls, _ := fixture.snapshot()
	assert.Empty(t, calls)
}

func TestHealthy(t *testing.T) {
	fixture := newHostedMobbin(t, "good")
	for _, tc := range []struct {
		token string
		want  bool
	}{{token: "good", want: true}, {token: "revoked", want: false}} {
		t.Run(tc.token, func(t *testing.T) {
			integration := newIntegration(t)
			require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"mcp_access_token": tc.token, "base_url": fixture.server.URL}))
			assert.Equal(t, tc.want, integration.Healthy(t.Context()))
		})
	}
}

func TestMaxResponseBytesForTool_RaisesCapForImageResults(t *testing.T) {
	integration, ok := New().(mcp.PerToolMaxResponseBytesIntegration)
	require.True(t, ok)
	for _, tool := range tools {
		t.Run(string(tool.Name), func(t *testing.T) {
			limit, ok := integration.MaxResponseBytesForTool(tool.Name)
			if tool.Access == mcp.AccessRead && tool.Name != "mobbin_list_collections" {
				require.True(t, ok)
				assert.Equal(t, imageResultMaxBytes, limit)
				return
			}
			assert.False(t, ok)
		})
	}
}

func TestConfigure_RotatesTokenAndCloseIsIdempotent(t *testing.T) {
	fixture := newHostedMobbin(t, "first", "second")
	integration := newIntegration(t)
	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"mcp_access_token": "first", "base_url": fixture.server.URL}))
	require.True(t, integration.Healthy(t.Context()))
	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"mcp_access_token": "second", "base_url": fixture.server.URL}))
	result, err := integration.Execute(t.Context(), "mobbin_list_collections", nil)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	_, auths := fixture.snapshot()
	assert.Equal(t, "Bearer second", auths[len(auths)-1])

	closer := integration.(io.Closer)
	require.NoError(t, closer.Close())
	require.NoError(t, closer.Close())
	result, err = integration.Execute(t.Context(), "mobbin_list_collections", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Len(t, integration.Tools(), len(tools), "static catalog stays searchable while unconfigured")
}
