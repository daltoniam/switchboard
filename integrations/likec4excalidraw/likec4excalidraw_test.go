package likec4excalidraw

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeLikeC4Server(t *testing.T, expectedToken string) *httptest.Server {
	t.Helper()

	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "likec4-excalidraw", Version: "0.1.0"}, nil)
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "get_architecture",
		Description: "Return the current LikeC4 architecture model.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: `{"elements":[{"fqn":"shop.api","kind":"service","title":"API"}]}`}}}, nil, nil
	})
	type screenshotInput struct {
		Scope string `json:"scope"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "get_canvas_screenshot",
		Description: "Return a PNG screenshot of the diagram canvas.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, input screenshotInput) (*mcpsdk.CallToolResult, any, error) {
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{
			&mcpsdk.ImageContent{Data: []byte("png"), MIMEType: "image/png"},
			&mcpsdk.TextContent{Text: `{"width":10,"height":5,"scope":"` + input.Scope + `"}`},
		}}, nil, nil
	})

	stream := mcpsdk.NewStreamableHTTPHandler(func(r *http.Request) *mcpsdk.Server {
		assert.Equal(t, expectedToken, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		return server
	}, &mcpsdk.StreamableHTTPOptions{JSONResponse: true, Stateless: true})
	mux := http.NewServeMux()
	mux.Handle("/mcp", stream)
	return httptest.NewServer(mux)
}

func TestNew(t *testing.T) {
	integration := New()
	require.NotNil(t, integration)
	assert.Equal(t, "likec4excalidraw", integration.Name())
}

func TestConfigure(t *testing.T) {
	tests := []struct {
		name        string
		credentials mcp.Credentials
		wantError   string
	}{
		{name: "base URL only", credentials: mcp.Credentials{"base_url": "http://127.0.0.1:4242"}},
		{name: "optional token", credentials: mcp.Credentials{"base_url": "http://127.0.0.1:4242", "mcp_token": "secret"}},
		{name: "trailing MCP path", credentials: mcp.Credentials{"base_url": "http://127.0.0.1:4242/mcp"}},
		{name: "missing base URL", credentials: mcp.Credentials{}, wantError: "base_url is required"},
		{name: "invalid base URL", credentials: mcp.Credentials{"base_url": "://bad"}, wantError: "invalid base_url"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := New().Configure(context.Background(), test.credentials)
			if test.wantError == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantError)
		})
	}
}

func TestCredentialHints(t *testing.T) {
	integration := New()
	plainText := integration.(mcp.PlainTextCredentials)
	optional := integration.(mcp.OptionalCredentials)
	placeholders := integration.(mcp.PlaceholderHints)
	assert.Equal(t, []string{"base_url"}, plainText.PlainTextKeys())
	assert.Equal(t, []string{"mcp_token"}, optional.OptionalKeys())
	assert.Equal(t, "http://127.0.0.1:4242", placeholders.Placeholders()["base_url"])
}

func TestTools(t *testing.T) {
	integration := New()
	definitions := integration.Tools()
	require.Len(t, definitions, 10)

	seen := make(map[mcp.ToolName]bool, len(definitions))
	for index, tool := range definitions {
		assert.NotEmpty(t, tool.Name)
		assert.NotEmpty(t, tool.Description)
		assert.True(t, strings.HasPrefix(string(tool.Name), "likec4excalidraw_"))
		assert.False(t, seen[tool.Name], "duplicate tool %s", tool.Name)
		seen[tool.Name] = true
		if index == 0 {
			assert.Contains(t, tool.Description, "Start here")
		}
	}
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range New().Tools() {
		assert.Contains(t, supportedTools, tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	definitions := make(map[mcp.ToolName]bool)
	for _, tool := range New().Tools() {
		definitions[tool.Name] = true
	}
	for name := range supportedTools {
		assert.True(t, definitions[name], "supported tool %s has no definition", name)
	}
}

func TestExecuteUnknownTool(t *testing.T) {
	result, err := New().Execute(context.Background(), "likec4excalidraw_unknown", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestRemoteExecutionWithoutToken(t *testing.T) {
	server := fakeLikeC4Server(t, "")
	defer server.Close()
	integration := New()
	require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{"base_url": server.URL}))
	assert.True(t, integration.Healthy(context.Background()))

	result, err := integration.Execute(context.Background(), "likec4excalidraw_get_architecture", nil)
	require.NoError(t, err)
	require.False(t, result.IsError)
	var architecture map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &architecture))
	assert.NotEmpty(t, architecture["elements"])
}

func TestRemoteExecutionWithTokenAndImage(t *testing.T) {
	server := fakeLikeC4Server(t, "secret")
	defer server.Close()
	integration := New()
	require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{"base_url": server.URL, "mcp_token": "secret"}))

	result, err := integration.Execute(context.Background(), "likec4excalidraw_get_canvas_screenshot", map[string]any{"scope": "whole"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.JSONEq(t, `{"width":10,"height":5,"scope":"whole"}`, result.Data)
	require.Len(t, result.Media, 1)
	assert.Equal(t, []byte("png"), result.Media[0].Data)
	assert.Equal(t, "image/png", result.Media[0].MIMEType)
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		assert.Contains(t, supportedTools, name)
	}
}
