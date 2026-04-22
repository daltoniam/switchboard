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

type mockDryRunIntegration struct {
	mockIntegration
	dryRunFn func(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, bool)
}

func (m *mockDryRunIntegration) DryRun(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, bool) {
	if m.dryRunFn != nil {
		return m.dryRunFn(ctx, toolName, args)
	}
	return nil, false
}

func dryRunRequest(toolName string, args map[string]any) *mcpsdk.CallToolRequest {
	data, _ := json.Marshal(map[string]any{
		"tool_name": toolName,
		"arguments": args,
		"dry_run":   true,
	})
	return &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      "execute",
			Arguments: json.RawMessage(data),
		},
	}
}

func TestDryRun_SimulatedFallback(t *testing.T) {
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{
				Name:        "github_create_issue",
				Description: "Create issue",
				Parameters:  map[string]string{"owner": "Owner", "repo": "Repo", "title": "Title"},
				Required:    []string{"owner", "repo", "title"},
			},
		},
	}
	s := setupTestServer(mi)
	ctx := context.Background()

	result, err := s.handleExecute(ctx, dryRunRequest("github_create_issue", map[string]any{
		"owner": "daltoniam",
		"repo":  "switchboard",
		"title": "Test issue",
	}))
	require.NoError(t, err)
	require.False(t, result.IsError)

	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].(*mcpsdk.TextContent).Text), &resp))
	assert.Equal(t, true, resp["dry_run"])
	assert.Equal(t, "github_create_issue", resp["tool"])
	assert.Equal(t, "github", resp["integration"])
	assert.Equal(t, "ok", resp["status"])
	args := resp["validated_args"].(map[string]any)
	assert.Equal(t, "daltoniam", args["owner"])
}

func TestDryRun_ValidationFails(t *testing.T) {
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{
				Name:       "github_create_issue",
				Parameters: map[string]string{"owner": "Owner", "repo": "Repo", "title": "Title"},
				Required:   []string{"owner", "repo", "title"},
			},
		},
	}
	s := setupTestServer(mi)
	ctx := context.Background()

	result, err := s.handleExecute(ctx, dryRunRequest("github_create_issue", map[string]any{
		"owner": "daltoniam",
	}))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(*mcpsdk.TextContent).Text, "dry-run validation failed")
	assert.Contains(t, result.Content[0].(*mcpsdk.TextContent).Text, "repo")
}

func TestDryRun_NativeIntegration(t *testing.T) {
	mi := &mockDryRunIntegration{
		mockIntegration: mockIntegration{
			name:    "aws",
			healthy: true,
			tools: []mcp.ToolDefinition{
				{
					Name:       "aws_lambda_invoke",
					Parameters: map[string]string{"function_name": "Function"},
					Required:   []string{"function_name"},
				},
			},
		},
		dryRunFn: func(_ context.Context, _ mcp.ToolName, args map[string]any) (*mcp.ToolResult, bool) {
			return &mcp.ToolResult{
				Data: `{"dry_run":true,"native":true,"would_invoke":"` + args["function_name"].(string) + `"}`,
			}, true
		},
	}
	s := setupTestServerWithIntegration(mi)
	ctx := context.Background()

	result, err := s.handleExecute(ctx, dryRunRequest("aws_lambda_invoke", map[string]any{
		"function_name": "my-func",
	}))
	require.NoError(t, err)
	require.False(t, result.IsError)

	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].(*mcpsdk.TextContent).Text), &resp))
	assert.Equal(t, true, resp["native"])
	assert.Equal(t, "my-func", resp["would_invoke"])
}

func TestDryRun_NativeDeclines_FallsBackToSimulated(t *testing.T) {
	mi := &mockDryRunIntegration{
		mockIntegration: mockIntegration{
			name:    "aws",
			healthy: true,
			tools: []mcp.ToolDefinition{
				{
					Name:       "aws_s3_list",
					Parameters: map[string]string{"bucket": "Bucket"},
					Required:   []string{"bucket"},
				},
			},
		},
		dryRunFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, bool) {
			return nil, false
		},
	}
	s := setupTestServerWithIntegration(mi)
	ctx := context.Background()

	result, err := s.handleExecute(ctx, dryRunRequest("aws_s3_list", map[string]any{
		"bucket": "my-bucket",
	}))
	require.NoError(t, err)
	require.False(t, result.IsError)

	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].(*mcpsdk.TextContent).Text), &resp))
	assert.Equal(t, true, resp["dry_run"])
	assert.Equal(t, "ok", resp["status"])
}

func TestDryRun_UnhealthyIntegration(t *testing.T) {
	mi := &mockIntegration{
		name:    "github",
		healthy: false,
		tools: []mcp.ToolDefinition{
			{Name: "github_create_issue", Parameters: map[string]string{"title": "Title"}, Required: []string{"title"}},
		},
	}
	s := setupTestServer(mi)
	ctx := context.Background()

	result, err := s.handleExecute(ctx, dryRunRequest("github_create_issue", map[string]any{
		"title": "test",
	}))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(*mcpsdk.TextContent).Text, "unhealthy")
}

func TestDryRun_ToolNotFound(t *testing.T) {
	s := setupTestServer(&mockIntegration{name: "test", healthy: true})
	ctx := context.Background()

	result, err := s.handleExecute(ctx, dryRunRequest("nonexistent_tool", nil))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(*mcpsdk.TextContent).Text, "not found")
}

func TestDryRun_DoesNotExecute(t *testing.T) {
	executed := false
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "github_create_issue", Parameters: map[string]string{"title": "Title"}, Required: []string{"title"}},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			executed = true
			return &mcp.ToolResult{Data: `{"id":1}`}, nil
		},
	}
	s := setupTestServer(mi)
	ctx := context.Background()

	_, err := s.handleExecute(ctx, dryRunRequest("github_create_issue", map[string]any{
		"title": "test",
	}))
	require.NoError(t, err)
	assert.False(t, executed, "dry-run should not call Execute")
}

func TestDryRun_NoPinning(t *testing.T) {
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "github_create_issue", Parameters: map[string]string{"title": "Title"}, Required: []string{"title"}},
		},
	}
	s := setupTestServer(mi)
	ctx := context.Background()

	_, err := s.handleExecute(ctx, dryRunRequest("github_create_issue", map[string]any{
		"title": "test",
	}))
	require.NoError(t, err)

	sess := s.sessionStore.GetOrCreate("default")
	assert.Equal(t, 0, sess.PinnedCount(), "dry-run should not pin results")
}
