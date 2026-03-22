package overmind

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

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

// --- CRUD handlers ---

func listMCPRoles(ctx context.Context, o *overmind, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := o.get(ctx, "/api/mcp_roles")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createMCPRole(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	name := argStr(args, "name")
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}

	body := map[string]any{"name": name}
	if desc := argStr(args, "description"); desc != "" {
		body["description"] = desc
	}

	data, err := o.post(ctx, "/api/mcp_roles", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listAgents(ctx context.Context, o *overmind, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := o.get(ctx, "/api/agents")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createAgent(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	name := argStr(args, "name")
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}
	basePrompt := argStr(args, "base_prompt")
	if basePrompt == "" {
		return mcp.ErrResult(fmt.Errorf("base_prompt is required"))
	}
	model := argStr(args, "model")
	if model == "" {
		return mcp.ErrResult(fmt.Errorf("model is required"))
	}
	provider := argStr(args, "model_provider")
	if provider == "" {
		return mcp.ErrResult(fmt.Errorf("model_provider is required"))
	}
	roleID := argStr(args, "mcp_role_id")
	if roleID == "" {
		return mcp.ErrResult(fmt.Errorf("mcp_role_id is required"))
	}

	body := map[string]any{
		"name":           name,
		"base_prompt":    basePrompt,
		"model":          model,
		"model_provider": provider,
		"mcp_role_id":    roleID,
	}
	if desc := argStr(args, "description"); desc != "" {
		body["description"] = desc
	}

	data, err := o.post(ctx, "/api/agents", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listFlows(ctx context.Context, o *overmind, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := o.get(ctx, "/api/flows")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createFlow(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	name := argStr(args, "name")
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}
	agentID := argStr(args, "initial_agent_id")
	if agentID == "" {
		return mcp.ErrResult(fmt.Errorf("initial_agent_id is required"))
	}
	promptTemplate := argStr(args, "prompt_template")
	if promptTemplate == "" {
		return mcp.ErrResult(fmt.Errorf("prompt_template is required"))
	}

	body := map[string]any{
		"name":             name,
		"initial_agent_id": agentID,
		"prompt_template":  promptTemplate,
		"enabled":          true,
	}
	if desc := argStr(args, "description"); desc != "" {
		body["description"] = desc
	}

	if wp := argStr(args, "webhook_params"); wp != "" {
		var parsed json.RawMessage
		if json.Unmarshal([]byte(wp), &parsed) == nil {
			body["webhook_params"] = parsed
		}
	}
	if op := argStr(args, "output_params"); op != "" {
		var parsed json.RawMessage
		if json.Unmarshal([]byte(op), &parsed) == nil {
			body["output_params"] = parsed
		}
	}
	if e := argStr(args, "enabled"); e == "false" {
		body["enabled"] = false
	}
	if tm := argStr(args, "timeout_minutes"); tm != "" {
		if v, err := strconv.Atoi(tm); err == nil && v > 0 {
			body["timeout_minutes"] = v
		}
	}

	data, err := o.post(ctx, "/api/flows", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
