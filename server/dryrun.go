package server

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) handleDryRun(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcpsdk.CallToolResult, error) {
	integration, toolDef, err := s.findTool(toolName)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	if err := validateArgs(toolDef, args); err != nil {
		return errorResult(fmt.Sprintf("dry-run validation failed: %s", err)), nil
	}

	cb := s.getBreaker(integration.Name())
	if !cb.allow() {
		cb.recordSuccess()
		return errorResult(fmt.Sprintf(
			"dry-run: integration %q temporarily unavailable (circuit breaker open)",
			integration.Name(),
		)), nil
	}
	cb.recordSuccess()

	if !integration.Healthy(ctx) {
		return errorResult(fmt.Sprintf(
			"dry-run: integration %q is unhealthy — call would likely fail",
			integration.Name(),
		)), nil
	}

	if dri, ok := integration.(mcp.DryRunIntegration); ok {
		if result, handled := dri.DryRun(ctx, toolName, args); handled {
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: result.Data},
				},
				IsError: result.IsError,
			}, nil
		}
	}

	result, err := mcp.JSONResult(map[string]any{
		"dry_run":        true,
		"tool":           toolName,
		"integration":    integration.Name(),
		"validated_args": args,
		"status":         "ok",
		"note":           "Simulated dry-run: arguments are valid, integration is healthy. This tool does not support native dry-run preview.",
	})
	if err != nil {
		return errorResult(err.Error()), nil
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: result.Data},
		},
	}, nil
}
