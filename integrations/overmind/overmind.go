package overmind

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

type overmind struct {
	baseURL    string
	token      string
	agentRunID string
	flowRunID  string
	client     *http.Client
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

func New() mcp.Integration {
	return &overmind{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (o *overmind) Name() string { return "overmind" }

func (o *overmind) Configure(_ context.Context, creds mcp.Credentials) error {
	o.baseURL = strings.TrimRight(creds["base_url"], "/")
	if o.baseURL == "" {
		return fmt.Errorf("overmind: base_url is required")
	}
	o.token = creds["token"]
	if o.token == "" {
		return fmt.Errorf("overmind: token is required")
	}
	o.agentRunID = creds["agent_run_id"]
	if o.agentRunID == "" {
		return fmt.Errorf("overmind: agent_run_id is required")
	}
	o.flowRunID = creds["flow_run_id"]
	if o.flowRunID == "" {
		return fmt.Errorf("overmind: flow_run_id is required")
	}
	return nil
}

func (o *overmind) Healthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", o.baseURL+"/api/health", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+o.token)
	resp, err := o.client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode < 400
}

func (o *overmind) Tools() []mcp.ToolDefinition {
	return tools
}

func (o *overmind) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, o, args)
}

// --- HTTP helpers ---

func (o *overmind) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, o.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+o.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{
			StatusCode: resp.StatusCode,
			Err:        fmt.Errorf("overmind API error (%d): %s", resp.StatusCode, string(data)),
		}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("overmind API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (o *overmind) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return o.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (o *overmind) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return o.doRequest(ctx, "POST", path, body)
}

func (o *overmind) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return o.doRequest(ctx, "PUT", path, body)
}

func (o *overmind) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return o.doRequest(ctx, "DELETE", fmt.Sprintf(pathFmt, args...), nil)
}

type handlerFunc func(ctx context.Context, o *overmind, args map[string]any) (*mcp.ToolResult, error)

// --- Dispatch map ---

var dispatch = map[string]handlerFunc{
	// Flow tools (agent-facing)
	"overmind_launch_agent":     launchAgent,
	"overmind_get_agent_status": getAgentStatus,
	"overmind_get_agent_result": getAgentResult,
	"overmind_complete_flow":    completeFlow,

	// Agents admin
	"overmind_list_agents":  listAgents,
	"overmind_get_agent":    getAgent,
	"overmind_create_agent": createAgent,
	"overmind_update_agent": updateAgent,
	"overmind_delete_agent": deleteAgent,

	// Flows admin
	"overmind_list_flows":    listFlows,
	"overmind_get_flow":      getFlow,
	"overmind_create_flow":   createFlow,
	"overmind_update_flow":   updateFlow,
	"overmind_delete_flow":   deleteFlow,
	"overmind_clone_flow":    cloneFlow,
	"overmind_run_flow":      runFlow,
	"overmind_validate_flow": validateFlow,

	// Flow runs admin
	"overmind_list_flow_runs":  listFlowRuns,
	"overmind_get_flow_run":    getFlowRun,
	"overmind_cancel_flow_run": cancelFlowRun,

	// Agent runs admin
	"overmind_list_agent_runs": listAgentRuns,
	"overmind_get_agent_run":   getAgentRun,

	// MCP identities admin
	"overmind_list_mcp_identities": listMCPIdentities,
	"overmind_get_mcp_identity":    getMCPIdentity,
	"overmind_create_mcp_identity": createMCPIdentity,
	"overmind_update_mcp_identity": updateMCPIdentity,
	"overmind_delete_mcp_identity": deleteMCPIdentity,

	// MCP roles admin
	"overmind_list_mcp_roles":        listMCPRoles,
	"overmind_get_mcp_role":          getMCPRole,
	"overmind_create_mcp_role":       createMCPRole,
	"overmind_update_mcp_role":       updateMCPRole,
	"overmind_delete_mcp_role":       deleteMCPRole,
	"overmind_create_mcp_role_entry": createMCPRoleEntry,
	"overmind_update_mcp_role_entry": updateMCPRoleEntry,
	"overmind_delete_mcp_role_entry": deleteMCPRoleEntry,

	// Pipelines admin
	"overmind_list_pipelines":  listPipelines,
	"overmind_get_pipeline":    getPipeline,
	"overmind_create_pipeline": createPipeline,
	"overmind_update_pipeline": updatePipeline,
	"overmind_delete_pipeline": deletePipeline,

	// Tasks admin
	"overmind_list_tasks":  listTasks,
	"overmind_get_task":    getTask,
	"overmind_create_task": createTask,
	"overmind_update_task": updateTask,
	"overmind_delete_task": deleteTask,
}
