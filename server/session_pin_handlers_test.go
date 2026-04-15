package server

import (
	"context"
	"encoding/json"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pinRequest(args map[string]any) *mcpsdk.CallToolRequest {
	data, _ := json.Marshal(args)
	return &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      "pin",
			Arguments: json.RawMessage(data),
		},
	}
}

func TestHandleExecute_AutoPinsResult(t *testing.T) {
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "github_get_repo", Description: "Get repo"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: `{"id":1,"name":"switchboard"}`}, nil
		},
	}
	s := setupTestServer(mi)
	ctx := context.Background()

	result, err := s.handleExecute(ctx, executeRequest("github_get_repo", nil))
	require.NoError(t, err)
	require.False(t, result.IsError)

	require.Len(t, result.Content, 2)
	assert.Contains(t, result.Content[1].(*mcpsdk.TextContent).Text, "pinned as $1")

	sess := s.sessionStore.GetOrCreate("default")
	pr, ok := sess.GetPinned("$1")
	require.True(t, ok)
	assert.Equal(t, "github_get_repo", string(pr.Tool))
}

func TestHandleExecute_RefResolution(t *testing.T) {
	var capturedArgs map[string]any
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "github_get_repo", Description: "Get repo"},
			{Name: "github_get_issue", Description: "Get issue", Parameters: map[string]string{
				"owner":        "Repo owner",
				"issue_number": "Issue number",
			}},
		},
		execFn: func(_ context.Context, tn mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
			if tn == "github_get_repo" {
				return &mcp.ToolResult{Data: `{"owner":{"login":"daltoniam"},"name":"switchboard"}`}, nil
			}
			capturedArgs = args
			return &mcp.ToolResult{Data: `{"id":42}`}, nil
		},
	}
	s := setupTestServer(mi)
	ctx := context.Background()

	_, err := s.handleExecute(ctx, executeRequest("github_get_repo", nil))
	require.NoError(t, err)

	result, err := s.handleExecute(ctx, executeRequest("github_get_issue", map[string]any{
		"owner":        "$1.owner.login",
		"issue_number": 42,
	}))
	require.NoError(t, err)
	require.False(t, result.IsError)

	assert.Equal(t, "daltoniam", capturedArgs["owner"])
	assert.Equal(t, float64(42), capturedArgs["issue_number"])
}

func TestHandlePin_List(t *testing.T) {
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "github_get_repo", Description: "Get repo"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: `{"id":1}`}, nil
		},
	}
	s := setupTestServer(mi)
	ctx := context.Background()

	_, _ = s.handleExecute(ctx, executeRequest("github_get_repo", nil))

	result, err := s.handlePin(ctx, pinRequest(map[string]any{"action": "list"}))
	require.NoError(t, err)

	resp := parseSessionResponse(t, result)
	assert.Equal(t, float64(1), resp["pinned_count"])
}

func TestHandlePin_GetWithPath(t *testing.T) {
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "github_get_repo", Description: "Get repo"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: `{"owner":{"login":"daltoniam"}}`}, nil
		},
	}
	s := setupTestServer(mi)
	ctx := context.Background()

	_, _ = s.handleExecute(ctx, executeRequest("github_get_repo", nil))

	result, err := s.handlePin(ctx, pinRequest(map[string]any{
		"action": "get",
		"handle": "$1",
		"path":   "owner.login",
	}))
	require.NoError(t, err)

	text := result.Content[0].(*mcpsdk.TextContent).Text
	assert.Equal(t, `"daltoniam"`, text)
}

func TestHandlePin_Unpin(t *testing.T) {
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "github_get_repo", Description: "Get repo"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: `{"id":1}`}, nil
		},
	}
	s := setupTestServer(mi)
	ctx := context.Background()

	_, _ = s.handleExecute(ctx, executeRequest("github_get_repo", nil))

	result, err := s.handlePin(ctx, pinRequest(map[string]any{
		"action": "unpin",
		"handle": "$1",
	}))
	require.NoError(t, err)

	resp := parseSessionResponse(t, result)
	assert.Equal(t, true, resp["unpinned"])

	sess := s.sessionStore.GetOrCreate("default")
	assert.Equal(t, 0, sess.PinnedCount())
}

func TestHandleExecute_MetaToolsBlocked_IncludesPin(t *testing.T) {
	s := setupTestServer(&mockIntegration{name: "test", healthy: true})
	ctx := context.Background()

	result, err := s.handleExecute(ctx, executeRequest("pin", nil))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(*mcpsdk.TextContent).Text, "meta-tool")
}

func TestResolveRefs(t *testing.T) {
	s := newSession("test")
	s.PinResult("tool", `{"id":42,"name":"sb"}`)

	args := map[string]any{
		"id":    "$1.id",
		"plain": "not-a-ref",
		"num":   99,
	}
	resolveRefs(s, args)

	assert.Equal(t, float64(42), args["id"])
	assert.Equal(t, "not-a-ref", args["plain"])
	assert.Equal(t, 99, args["num"])
}

func TestResolveRefs_SkipsInvalidRefs(t *testing.T) {
	s := newSession("test")

	args := map[string]any{
		"dollar":  "$env_var",
		"handle":  "$99",
		"short":   "$",
		"regular": "value",
	}
	resolveRefs(s, args)

	assert.Equal(t, "$env_var", args["dollar"])
	assert.Equal(t, "$99", args["handle"])
	assert.Equal(t, "$", args["short"])
}
