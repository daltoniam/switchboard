package overmind

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func launchAgent(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	agentID := argStr(args, "agent_id")
	prompt := argStr(args, "prompt")
	agentContext := argStr(args, "context")

	body := map[string]any{
		"agent_id":      agentID,
		"prompt":        prompt,
		"parent_run_id": o.agentRunID,
	}
	if agentContext != "" {
		body["context"] = agentContext
	}

	data, err := o.post(ctx, fmt.Sprintf("/api/flow_runs/%s/launch_agent", url.PathEscape(o.flowRunID)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAgentStatus(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	agentRunID := argStr(args, "agent_run_id")

	data, err := o.get(ctx, "/api/agent_runs/%s/status", url.PathEscape(agentRunID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAgentResult(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	agentRunID := argStr(args, "agent_run_id")

	data, err := o.get(ctx, "/api/agent_runs/%s/result", url.PathEscape(agentRunID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func completeFlow(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	summary := argStr(args, "summary")
	status := argStr(args, "status")
	if status == "" {
		status = "success"
	}
	if status != "success" && status != "failure" {
		return mcp.ErrResult(fmt.Errorf("status must be 'success' or 'failure', got %q", status))
	}

	body := map[string]any{
		"summary":      summary,
		"status":       status,
		"agent_run_id": o.agentRunID,
	}

	data, err := o.post(ctx, fmt.Sprintf("/api/flow_runs/%s/complete", url.PathEscape(o.flowRunID)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
