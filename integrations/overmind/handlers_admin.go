package overmind

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

// ── Agents ───────────────────────────────────────────────────────────────────

func listAgents(ctx context.Context, o *overmind, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := o.get(ctx, "/api/agents")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAgent(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := o.get(ctx, "/api/agents/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createAgent(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}
	body := buildBody(args, "name", "description", "model", "model_provider", "base_prompt", "mcp_role_id")
	data, err := o.post(ctx, "/api/agents", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateAgent(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	body := buildBody(args, "name", "description", "model", "model_provider", "base_prompt", "mcp_role_id")
	data, err := o.put(ctx, fmt.Sprintf("/api/agents/%s", url.PathEscape(id)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteAgent(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := o.del(ctx, "/api/agents/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Flows ────────────────────────────────────────────────────────────────────

func listFlows(ctx context.Context, o *overmind, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := o.get(ctx, "/api/flows")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getFlow(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	expand := r.Str("expand")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	path := fmt.Sprintf("/api/flows/%s", url.PathEscape(id))
	if expand != "" {
		path += "?expand=" + url.QueryEscape(expand)
	}
	data, err := o.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createFlow(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}
	body := buildFlowBody(args)
	data, err := o.post(ctx, "/api/flows", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateFlow(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	body := buildFlowBody(args)
	data, err := o.put(ctx, fmt.Sprintf("/api/flows/%s", url.PathEscape(id)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteFlow(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := o.del(ctx, "/api/flows/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func cloneFlow(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	data, err := o.post(ctx, fmt.Sprintf("/api/flows/%s/clone", url.PathEscape(id)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func runFlow(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	prompt := r.Str("prompt")
	params := r.Map("params")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	body := map[string]any{}
	if prompt != "" {
		body["prompt"] = prompt
	}
	if params != nil {
		body["params"] = params
	}
	data, err := o.post(ctx, fmt.Sprintf("/api/flows/%s/run", url.PathEscape(id)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func validateFlow(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	body := buildFlowBody(args)
	data, err := o.post(ctx, "/api/flows/validate", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Flow runs ────────────────────────────────────────────────────────────────

func listFlowRuns(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	flowID, err := mcp.ArgStr(args, "flow_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if flowID == "" {
		return mcp.ErrResult(fmt.Errorf("flow_id is required"))
	}
	data, err := o.get(ctx, "/api/flow_runs?flow_id=%s", url.QueryEscape(flowID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getFlowRun(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := o.get(ctx, "/api/flow_runs/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func cancelFlowRun(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := o.post(ctx, fmt.Sprintf("/api/flow_runs/%s/cancel", url.PathEscape(id)), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Agent runs ───────────────────────────────────────────────────────────────

func listAgentRuns(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	flowRunID, err := mcp.ArgStr(args, "flow_run_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if flowRunID == "" {
		return mcp.ErrResult(fmt.Errorf("flow_run_id is required"))
	}
	data, err := o.get(ctx, "/api/agent_runs?flow_run_id=%s", url.QueryEscape(flowRunID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAgentRun(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := o.get(ctx, "/api/agent_runs/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── MCP identities ──────────────────────────────────────────────────────────

func listMCPIdentities(ctx context.Context, o *overmind, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := o.get(ctx, "/api/mcp_identities")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getMCPIdentity(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := o.get(ctx, "/api/mcp_identities/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createMCPIdentity(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	integrationName := r.Str("integration_name")
	credentialsRaw := r.Str("credentials")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}
	if integrationName == "" {
		return mcp.ErrResult(fmt.Errorf("integration_name is required"))
	}
	if credentialsRaw == "" {
		return mcp.ErrResult(fmt.Errorf("credentials is required"))
	}
	var creds json.RawMessage
	if err := json.Unmarshal([]byte(credentialsRaw), &creds); err != nil {
		return mcp.ErrResult(fmt.Errorf("credentials must be valid JSON: %w", err))
	}
	body := map[string]any{
		"name":             name,
		"integration_name": integrationName,
		"credentials":      creds,
	}
	data, err := o.post(ctx, "/api/mcp_identities", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateMCPIdentity(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	name := r.Str("name")
	integrationName := r.Str("integration_name")
	credentialsRaw := r.Str("credentials")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	if integrationName != "" {
		body["integration_name"] = integrationName
	}
	if credentialsRaw != "" {
		var creds json.RawMessage
		if err := json.Unmarshal([]byte(credentialsRaw), &creds); err != nil {
			return mcp.ErrResult(fmt.Errorf("credentials must be valid JSON: %w", err))
		}
		body["credentials"] = creds
	}
	data, err := o.put(ctx, fmt.Sprintf("/api/mcp_identities/%s", url.PathEscape(id)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteMCPIdentity(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := o.del(ctx, "/api/mcp_identities/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── MCP roles ────────────────────────────────────────────────────────────────

func listMCPRoles(ctx context.Context, o *overmind, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := o.get(ctx, "/api/mcp_roles")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getMCPRole(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := o.get(ctx, "/api/mcp_roles/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createMCPRole(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}
	body := buildBody(args, "name", "description")
	data, err := o.post(ctx, "/api/mcp_roles", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateMCPRole(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	body := buildBody(args, "name", "description")
	data, err := o.put(ctx, fmt.Sprintf("/api/mcp_roles/%s", url.PathEscape(id)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteMCPRole(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := o.del(ctx, "/api/mcp_roles/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createMCPRoleEntry(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	roleID := r.Str("role_id")
	identityID := r.Str("mcp_identity_id")
	toolGlobs := r.StrSlice("tool_globs")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if roleID == "" {
		return mcp.ErrResult(fmt.Errorf("role_id is required"))
	}
	if identityID == "" {
		return mcp.ErrResult(fmt.Errorf("mcp_identity_id is required"))
	}
	body := map[string]any{
		"mcp_identity_id": identityID,
	}
	if len(toolGlobs) > 0 {
		body["tool_globs"] = toolGlobs
	}
	data, err := o.post(ctx, fmt.Sprintf("/api/mcp_roles/%s/entries", url.PathEscape(roleID)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateMCPRoleEntry(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	roleID := r.Str("role_id")
	entryID := r.Str("entry_id")
	identityID := r.Str("mcp_identity_id")
	toolGlobs := r.StrSlice("tool_globs")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if roleID == "" {
		return mcp.ErrResult(fmt.Errorf("role_id is required"))
	}
	if entryID == "" {
		return mcp.ErrResult(fmt.Errorf("entry_id is required"))
	}
	body := map[string]any{}
	if identityID != "" {
		body["mcp_identity_id"] = identityID
	}
	if len(toolGlobs) > 0 {
		body["tool_globs"] = toolGlobs
	}
	data, err := o.put(ctx, fmt.Sprintf("/api/mcp_roles/%s/entries/%s",
		url.PathEscape(roleID), url.PathEscape(entryID)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteMCPRoleEntry(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	roleID := r.Str("role_id")
	entryID := r.Str("entry_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if roleID == "" {
		return mcp.ErrResult(fmt.Errorf("role_id is required"))
	}
	if entryID == "" {
		return mcp.ErrResult(fmt.Errorf("entry_id is required"))
	}
	data, err := o.del(ctx, "/api/mcp_roles/%s/entries/%s",
		url.PathEscape(roleID), url.PathEscape(entryID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Pipelines ────────────────────────────────────────────────────────────────

func listPipelines(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	gcID, err := mcp.ArgStr(args, "global_context_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	path := "/api/pipelines"
	if gcID != "" {
		path += "?global_context_id=" + url.QueryEscape(gcID)
	}
	data, err := o.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getPipeline(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := o.get(ctx, "/api/pipelines/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createPipeline(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	gcID := r.Str("global_context_id")
	contextRaw := r.Str("context")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}
	if gcID == "" {
		return mcp.ErrResult(fmt.Errorf("global_context_id is required"))
	}
	body := map[string]any{
		"name":              name,
		"global_context_id": gcID,
	}
	if contextRaw != "" {
		var raw json.RawMessage
		if err := json.Unmarshal([]byte(contextRaw), &raw); err != nil {
			return mcp.ErrResult(fmt.Errorf("context must be valid JSON: %w", err))
		}
		body["context"] = raw
	}
	data, err := o.post(ctx, "/api/pipelines", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updatePipeline(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	name := r.Str("name")
	contextRaw := r.Str("context")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	if contextRaw != "" {
		var raw json.RawMessage
		if err := json.Unmarshal([]byte(contextRaw), &raw); err != nil {
			return mcp.ErrResult(fmt.Errorf("context must be valid JSON: %w", err))
		}
		body["context"] = raw
	}
	data, err := o.put(ctx, fmt.Sprintf("/api/pipelines/%s", url.PathEscape(id)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deletePipeline(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := o.del(ctx, "/api/pipelines/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Tasks ────────────────────────────────────────────────────────────────────

func listTasks(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	pipelineID, err := mcp.ArgStr(args, "pipeline_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if pipelineID == "" {
		return mcp.ErrResult(fmt.Errorf("pipeline_id is required"))
	}
	data, err := o.get(ctx, "/api/tasks?pipeline_id=%s", url.QueryEscape(pipelineID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTask(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := o.get(ctx, "/api/tasks/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createTask(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	pipelineID := r.Str("pipeline_id")
	contextRaw := r.Str("context")
	dependsOn := r.StrSlice("depends_on")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}
	if pipelineID == "" {
		return mcp.ErrResult(fmt.Errorf("pipeline_id is required"))
	}
	body := map[string]any{
		"name":        name,
		"pipeline_id": pipelineID,
	}
	if contextRaw != "" {
		var raw json.RawMessage
		if err := json.Unmarshal([]byte(contextRaw), &raw); err != nil {
			return mcp.ErrResult(fmt.Errorf("context must be valid JSON: %w", err))
		}
		body["context"] = raw
	}
	if len(dependsOn) > 0 {
		body["depends_on"] = dependsOn
	}
	data, err := o.post(ctx, "/api/tasks", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateTask(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	name := r.Str("name")
	contextRaw := r.Str("context")
	dependsOn := r.StrSlice("depends_on")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	if contextRaw != "" {
		var raw json.RawMessage
		if err := json.Unmarshal([]byte(contextRaw), &raw); err != nil {
			return mcp.ErrResult(fmt.Errorf("context must be valid JSON: %w", err))
		}
		body["context"] = raw
	}
	if len(dependsOn) > 0 {
		body["depends_on"] = dependsOn
	}
	data, err := o.put(ctx, fmt.Sprintf("/api/tasks/%s", url.PathEscape(id)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteTask(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := o.del(ctx, "/api/tasks/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func buildBody(args map[string]any, keys ...string) map[string]any {
	body := map[string]any{}
	for _, k := range keys {
		if v, ok := args[k]; ok && v != nil && v != "" {
			body[k] = v
		}
	}
	return body
}

func buildFlowBody(args map[string]any) map[string]any {
	body := buildBody(args,
		"name", "description", "prompt_template", "initial_agent_id",
		"repo_url", "repo_ref", "output_webhook_url", "output_webhook_template",
		"webhook_secret",
	)
	if v, err := mcp.ArgStrSlice(args, "available_agent_ids"); err == nil && len(v) > 0 {
		body["available_agent_ids"] = v
	}
	if v, err := mcp.ArgInt(args, "timeout_minutes"); err == nil && v > 0 {
		body["timeout_minutes"] = v
	}
	if v, err := mcp.ArgBool(args, "enabled"); err == nil {
		if _, ok := args["enabled"]; ok {
			body["enabled"] = v
		}
	}
	return body
}
