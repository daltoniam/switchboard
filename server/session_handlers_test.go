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

func sessionRequest(args map[string]any) *mcpsdk.CallToolRequest {
	data, _ := json.Marshal(args)
	return &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      "session",
			Arguments: json.RawMessage(data),
		},
	}
}

func historyRequest(args map[string]any) *mcpsdk.CallToolRequest {
	data, _ := json.Marshal(args)
	return &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      "history",
			Arguments: json.RawMessage(data),
		},
	}
}

func parseSessionResponse(t *testing.T, result *mcpsdk.CallToolResult) map[string]any {
	t.Helper()
	require.NotNil(t, result)
	require.False(t, result.IsError, "expected non-error result, got: %s", result.Content[0].(*mcpsdk.TextContent).Text)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].(*mcpsdk.TextContent).Text), &resp))
	return resp
}

func TestHandleSession_Set(t *testing.T) {
	s := setupTestServer(&mockIntegration{name: "test", healthy: true})
	ctx := context.Background()

	result, err := s.handleSession(ctx, sessionRequest(map[string]any{
		"action":  "set",
		"context": map[string]any{"owner": "daltoniam", "repo": "switchboard"},
	}))
	require.NoError(t, err)
	resp := parseSessionResponse(t, result)

	ctxMap := resp["context"].(map[string]any)
	assert.Equal(t, "daltoniam", ctxMap["owner"])
	assert.Equal(t, "switchboard", ctxMap["repo"])
}

func TestHandleSession_Get(t *testing.T) {
	s := setupTestServer(&mockIntegration{name: "test", healthy: true})
	ctx := context.Background()

	s.sessionStore.GetOrCreate("default").SetContext(map[string]any{"owner": "pre-set"})

	result, err := s.handleSession(ctx, sessionRequest(map[string]any{"action": "get"}))
	require.NoError(t, err)
	resp := parseSessionResponse(t, result)

	ctxMap := resp["context"].(map[string]any)
	assert.Equal(t, "pre-set", ctxMap["owner"])
}

func TestHandleSession_Clear(t *testing.T) {
	s := setupTestServer(&mockIntegration{name: "test", healthy: true})
	ctx := context.Background()

	sess := s.sessionStore.GetOrCreate("default")
	sess.SetContext(map[string]any{"owner": "daltoniam"})

	result, err := s.handleSession(ctx, sessionRequest(map[string]any{"action": "clear"}))
	require.NoError(t, err)
	resp := parseSessionResponse(t, result)

	ctxMap := resp["context"].(map[string]any)
	assert.Empty(t, ctxMap)
}

func TestHandleSession_SetRequiresContext(t *testing.T) {
	s := setupTestServer(&mockIntegration{name: "test", healthy: true})
	ctx := context.Background()

	result, err := s.handleSession(ctx, sessionRequest(map[string]any{"action": "set"}))
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestHandleSession_InvalidAction(t *testing.T) {
	s := setupTestServer(&mockIntegration{name: "test", healthy: true})
	ctx := context.Background()

	result, err := s.handleSession(ctx, sessionRequest(map[string]any{"action": "invalid"}))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(*mcpsdk.TextContent).Text, "unknown action")
}

func TestHandleExecute_SessionContextInjected(t *testing.T) {
	var capturedArgs map[string]any
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{
				Name:        "github_list_issues",
				Description: "List issues",
				Parameters:  map[string]string{"owner": "Repo owner", "repo": "Repo name", "state": "Issue state"},
				Required:    []string{"owner", "repo"},
			},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
			capturedArgs = args
			return &mcp.ToolResult{Data: `[]`}, nil
		},
	}
	s := setupTestServer(mi)

	sess := s.sessionStore.GetOrCreate("default")
	sess.SetContext(map[string]any{"owner": "daltoniam", "repo": "switchboard"})

	ctx := context.Background()
	result, err := s.handleExecute(ctx, executeRequest("github_list_issues", map[string]any{"state": "open"}))
	require.NoError(t, err)
	require.False(t, result.IsError)

	assert.Equal(t, "daltoniam", capturedArgs["owner"])
	assert.Equal(t, "switchboard", capturedArgs["repo"])
	assert.Equal(t, "open", capturedArgs["state"])
}

func TestHandleExecute_ExplicitArgsOverrideSession(t *testing.T) {
	var capturedArgs map[string]any
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{
				Name:        "github_list_issues",
				Description: "List issues",
				Parameters:  map[string]string{"owner": "Repo owner", "repo": "Repo name"},
				Required:    []string{"owner", "repo"},
			},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
			capturedArgs = args
			return &mcp.ToolResult{Data: `[]`}, nil
		},
	}
	s := setupTestServer(mi)

	sess := s.sessionStore.GetOrCreate("default")
	sess.SetContext(map[string]any{"owner": "session-owner", "repo": "session-repo"})

	ctx := context.Background()
	result, err := s.handleExecute(ctx, executeRequest("github_list_issues", map[string]any{
		"owner": "explicit-owner",
		"repo":  "explicit-repo",
	}))
	require.NoError(t, err)
	require.False(t, result.IsError)

	assert.Equal(t, "explicit-owner", capturedArgs["owner"])
	assert.Equal(t, "explicit-repo", capturedArgs["repo"])
}

func TestHandleExecute_RecordsBreadcrumb(t *testing.T) {
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

	_, err := s.handleExecute(ctx, executeRequest("github_get_repo", nil))
	require.NoError(t, err)

	sess := s.sessionStore.GetOrCreate("default")
	require.Len(t, sess.Breadcrumbs, 1)
	assert.Equal(t, mcp.ToolName("github_get_repo"), sess.Breadcrumbs[0].Tool)
	assert.False(t, sess.Breadcrumbs[0].IsError)
}

func TestHandleHistory_ReturnsBreadcrumbs(t *testing.T) {
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "github_get_repo", Description: "Get repo"},
			{Name: "github_list_issues", Description: "List issues"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: `{"id":1}`}, nil
		},
	}
	s := setupTestServer(mi)
	ctx := context.Background()

	_, _ = s.handleExecute(ctx, executeRequest("github_get_repo", nil))
	_, _ = s.handleExecute(ctx, executeRequest("github_list_issues", nil))

	result, err := s.handleHistory(ctx, historyRequest(map[string]any{"last_n": 10}))
	require.NoError(t, err)

	resp := parseSessionResponse(t, result)
	total := resp["total_in_session"].(float64)
	assert.Equal(t, float64(2), total)
}

func TestHandleHistory_FilterByTool(t *testing.T) {
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "github_get_repo", Description: "Get repo"},
			{Name: "github_list_issues", Description: "List issues"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: `{"id":1}`}, nil
		},
	}
	s := setupTestServer(mi)
	ctx := context.Background()

	_, _ = s.handleExecute(ctx, executeRequest("github_get_repo", nil))
	_, _ = s.handleExecute(ctx, executeRequest("github_list_issues", nil))
	_, _ = s.handleExecute(ctx, executeRequest("github_get_repo", nil))

	result, err := s.handleHistory(ctx, historyRequest(map[string]any{
		"last_n": 10,
		"tool":   "github_get_repo",
	}))
	require.NoError(t, err)

	resp := parseSessionResponse(t, result)
	bcs := resp["breadcrumbs"].([]any)
	assert.Len(t, bcs, 2)
}

func TestHandleExecute_MetaToolsBlocked(t *testing.T) {
	s := setupTestServer(&mockIntegration{name: "test", healthy: true})
	ctx := context.Background()

	for _, name := range []string{"search", "execute", "session", "history"} {
		t.Run(name, func(t *testing.T) {
			result, err := s.handleExecute(ctx, executeRequest(name, nil))
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Content[0].(*mcpsdk.TextContent).Text, "meta-tool")
		})
	}
}
