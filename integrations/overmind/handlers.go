package overmind

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listAvailableAgents(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	data, err := o.get(ctx, "/api/flow_runs/%s/available_agents", url.PathEscape(o.flowRunID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func launchAgent(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	agentID := argStr(args, "agent_id")
	if agentID == "" {
		return mcp.ErrResult(fmt.Errorf("agent_id is required"))
	}
	prompt := argStr(args, "prompt")
	if prompt == "" {
		return mcp.ErrResult(fmt.Errorf("prompt is required"))
	}
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
	if agentRunID == "" {
		return mcp.ErrResult(fmt.Errorf("agent_run_id is required"))
	}

	data, err := o.get(ctx, "/api/agent_runs/%s/status", url.PathEscape(agentRunID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAgentResult(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	agentRunID := argStr(args, "agent_run_id")
	if agentRunID == "" {
		return mcp.ErrResult(fmt.Errorf("agent_run_id is required"))
	}

	data, err := o.get(ctx, "/api/agent_runs/%s/result", url.PathEscape(agentRunID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func completeFlow(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	summary := argStr(args, "summary")
	if summary == "" {
		return mcp.ErrResult(fmt.Errorf("summary is required"))
	}
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
