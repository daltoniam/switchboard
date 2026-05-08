package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

// messageCounter generates unique message IDs.
var messageCounter uint64

func genMessageID() string {
	n := atomic.AddUint64(&messageCounter, 1)
	return fmt.Sprintf("swb-%016x", n)
}

// ---------------------------------------------------------------------------
// ProjectService handlers
// ---------------------------------------------------------------------------

func handleProjectList(ctx context.Context, grpc *grpcClient, _ *httpClient, _ map[string]any) (*mcp.ToolResult, error) {
	resp, err := grpc.call(ctx, "arp.v1.ProjectService", "ListProjects", map[string]any{})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(resp)}, nil
}

func handleProjectRegister(ctx context.Context, grpc *grpcClient, _ *httpClient, args map[string]any) (*mcp.ToolResult, error) {
	name, _ := mcp.ArgStr(args, "name")
	repo, _ := mcp.ArgStr(args, "repo")
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}
	if repo == "" {
		return mcp.ErrResult(fmt.Errorf("repo is required"))
	}

	req := map[string]any{
		"name": name,
		"repo": repo,
	}
	if branch, _ := mcp.ArgStr(args, "branch"); branch != "" {
		req["branch"] = branch
	}
	if agentsJSON, _ := mcp.ArgStr(args, "agents"); agentsJSON != "" {
		var agents []any
		if err := json.Unmarshal([]byte(agentsJSON), &agents); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid agents JSON: %w", err))
		}
		req["agents"] = agents
	}

	resp, err := grpc.call(ctx, "arp.v1.ProjectService", "RegisterProject", req)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(resp)}, nil
}

func handleProjectUnregister(ctx context.Context, grpc *grpcClient, _ *httpClient, args map[string]any) (*mcp.ToolResult, error) {
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}

	_, err := grpc.call(ctx, "arp.v1.ProjectService", "UnregisterProject", map[string]any{"name": name})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: `{"status":"ok"}`}, nil
}

// ---------------------------------------------------------------------------
// WorkspaceService handlers
// ---------------------------------------------------------------------------

func handleWorkspaceCreate(ctx context.Context, grpc *grpcClient, _ *httpClient, args map[string]any) (*mcp.ToolResult, error) {
	name, _ := mcp.ArgStr(args, "name")
	project, _ := mcp.ArgStr(args, "project")
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}
	if project == "" {
		return mcp.ErrResult(fmt.Errorf("project is required"))
	}

	req := map[string]any{
		"name":    name,
		"project": project,
	}
	if branch, _ := mcp.ArgStr(args, "branch"); branch != "" {
		req["branch"] = branch
	}
	if autoJSON, _ := mcp.ArgStr(args, "auto_agents"); autoJSON != "" {
		var agents []any
		if err := json.Unmarshal([]byte(autoJSON), &agents); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid auto_agents JSON: %w", err))
		}
		req["autoAgents"] = agents
	}

	resp, err := grpc.call(ctx, "arp.v1.WorkspaceService", "CreateWorkspace", req)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(resp)}, nil
}

func handleWorkspaceList(ctx context.Context, grpc *grpcClient, _ *httpClient, args map[string]any) (*mcp.ToolResult, error) {
	req := map[string]any{}
	if project, _ := mcp.ArgStr(args, "project"); project != "" {
		req["project"] = project
	}
	if status, _ := mcp.ArgStr(args, "status"); status != "" {
		switch status {
		case "active":
			req["status"] = 1
		case "inactive":
			req["status"] = 2
		}
	}

	resp, err := grpc.call(ctx, "arp.v1.WorkspaceService", "ListWorkspaces", req)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(resp)}, nil
}

func handleWorkspaceGet(ctx context.Context, grpc *grpcClient, _ *httpClient, args map[string]any) (*mcp.ToolResult, error) {
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}

	resp, err := grpc.call(ctx, "arp.v1.WorkspaceService", "GetWorkspace", map[string]any{"name": name})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(resp)}, nil
}

func handleWorkspaceDestroy(ctx context.Context, grpc *grpcClient, _ *httpClient, args map[string]any) (*mcp.ToolResult, error) {
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}

	req := map[string]any{"name": name}
	if keepStr, _ := mcp.ArgStr(args, "keep_worktree"); keepStr == "true" {
		req["keepWorktree"] = true
	}

	_, err := grpc.call(ctx, "arp.v1.WorkspaceService", "DestroyWorkspace", req)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: `{"status":"ok"}`}, nil
}

// ---------------------------------------------------------------------------
// AgentService handlers — lifecycle
// ---------------------------------------------------------------------------

func handleAgentSpawn(ctx context.Context, grpc *grpcClient, _ *httpClient, args map[string]any) (*mcp.ToolResult, error) {
	workspace, _ := mcp.ArgStr(args, "workspace")
	template, _ := mcp.ArgStr(args, "template")
	if workspace == "" {
		return mcp.ErrResult(fmt.Errorf("workspace is required"))
	}
	if template == "" {
		return mcp.ErrResult(fmt.Errorf("template is required"))
	}

	req := map[string]any{
		"workspace": workspace,
		"template":  template,
	}
	if name, _ := mcp.ArgStr(args, "name"); name != "" {
		req["name"] = name
	}
	if prompt, _ := mcp.ArgStr(args, "prompt"); prompt != "" {
		req["prompt"] = prompt
	}
	if envJSON, _ := mcp.ArgStr(args, "env"); envJSON != "" {
		var env map[string]any
		if err := json.Unmarshal([]byte(envJSON), &env); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid env JSON: %w", err))
		}
		req["env"] = env
	}
	if scopeJSON, _ := mcp.ArgStr(args, "scope"); scopeJSON != "" {
		var scope map[string]any
		if err := json.Unmarshal([]byte(scopeJSON), &scope); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid scope JSON: %w", err))
		}
		req["scope"] = scope
	}
	if perm, _ := mcp.ArgStr(args, "permission"); perm != "" {
		switch perm {
		case "session", "PERMISSION_SESSION":
			req["permission"] = 1
		case "project", "PERMISSION_PROJECT":
			req["permission"] = 2
		case "admin", "PERMISSION_ADMIN":
			req["permission"] = 3
		}
	}

	resp, err := grpc.call(ctx, "arp.v1.AgentService", "SpawnAgent", req)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(resp)}, nil
}

func handleAgentList(ctx context.Context, grpc *grpcClient, _ *httpClient, args map[string]any) (*mcp.ToolResult, error) {
	req := map[string]any{}
	if workspace, _ := mcp.ArgStr(args, "workspace"); workspace != "" {
		req["workspace"] = workspace
	}
	if template, _ := mcp.ArgStr(args, "template"); template != "" {
		req["template"] = template
	}
	if status, _ := mcp.ArgStr(args, "status"); status != "" {
		switch status {
		case "starting":
			req["status"] = 1
		case "ready":
			req["status"] = 2
		case "busy":
			req["status"] = 3
		case "error":
			req["status"] = 4
		case "stopping":
			req["status"] = 5
		case "stopped":
			req["status"] = 6
		}
	}

	resp, err := grpc.call(ctx, "arp.v1.AgentService", "ListAgents", req)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(resp)}, nil
}

func handleAgentStatus(ctx context.Context, grpc *grpcClient, _ *httpClient, args map[string]any) (*mcp.ToolResult, error) {
	agentID, _ := mcp.ArgStr(args, "agent_id")
	if agentID == "" {
		return mcp.ErrResult(fmt.Errorf("agent_id is required"))
	}

	resp, err := grpc.call(ctx, "arp.v1.AgentService", "GetAgentStatus", map[string]any{"agentId": agentID})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(resp)}, nil
}

func handleAgentStop(ctx context.Context, grpc *grpcClient, _ *httpClient, args map[string]any) (*mcp.ToolResult, error) {
	agentID, _ := mcp.ArgStr(args, "agent_id")
	if agentID == "" {
		return mcp.ErrResult(fmt.Errorf("agent_id is required"))
	}

	req := map[string]any{"agentId": agentID}
	if grace, _ := mcp.ArgStr(args, "grace_period_ms"); grace != "" {
		if v, err := strconv.Atoi(grace); err == nil {
			req["gracePeriodMs"] = v
		}
	}

	resp, err := grpc.call(ctx, "arp.v1.AgentService", "StopAgent", req)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(resp)}, nil
}

func handleAgentRestart(ctx context.Context, grpc *grpcClient, _ *httpClient, args map[string]any) (*mcp.ToolResult, error) {
	agentID, _ := mcp.ArgStr(args, "agent_id")
	if agentID == "" {
		return mcp.ErrResult(fmt.Errorf("agent_id is required"))
	}

	resp, err := grpc.call(ctx, "arp.v1.AgentService", "RestartAgent", map[string]any{"agentId": agentID})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(resp)}, nil
}

// ---------------------------------------------------------------------------
// AgentService handlers — messaging
// ---------------------------------------------------------------------------

const maxPollAttempts = 600

func isTerminalStatus(status string) bool {
	switch status {
	case "completed", "failed", "canceled":
		return true
	}
	return false
}

func handleAgentMessage(ctx context.Context, grpc *grpcClient, _ *httpClient, args map[string]any) (*mcp.ToolResult, error) {
	agentID, _ := mcp.ArgStr(args, "agent_id")
	message, _ := mcp.ArgStr(args, "message")
	if agentID == "" {
		return mcp.ErrResult(fmt.Errorf("agent_id is required"))
	}
	if message == "" {
		return mcp.ErrResult(fmt.Errorf("message is required"))
	}

	blockingStr, _ := mcp.ArgStr(args, "blocking")
	blocking := blockingStr != "false" // default true

	contextID, _ := mcp.ArgStr(args, "context_id")

	// Always send non-blocking to avoid gRPC timeout, then poll if blocking.
	req := map[string]any{
		"agentId":  agentID,
		"message":  message,
		"blocking": false,
	}
	if contextID != "" {
		req["contextId"] = contextID
	}

	resp, err := grpc.call(ctx, "arp.v1.AgentService", "SendAgentMessage", req)
	if err != nil {
		return mcp.ErrResult(err)
	}

	// Parse response to check for direct message or task.
	var result map[string]json.RawMessage
	if err := json.Unmarshal(resp, &result); err != nil {
		return &mcp.ToolResult{Data: string(resp)}, nil
	}

	// If we got a direct message, return it.
	if _, ok := result["message"]; ok {
		return &mcp.ToolResult{Data: string(resp)}, nil
	}

	// Extract task.
	taskRaw, hasTask := result["task"]
	if !hasTask {
		return &mcp.ToolResult{Data: string(resp)}, nil
	}

	// Non-blocking mode: return task immediately.
	if !blocking {
		return &mcp.ToolResult{Data: string(resp)}, nil
	}

	// Extract task ID for polling.
	var task map[string]any
	if err := json.Unmarshal(taskRaw, &task); err != nil {
		return &mcp.ToolResult{Data: string(resp)}, nil
	}
	taskID, _ := task["id"].(string)
	if taskID == "" {
		return &mcp.ToolResult{Data: string(resp)}, nil
	}

	// Poll until terminal state.
	return pollTaskUntilDone(ctx, grpc, agentID, taskID)
}

func pollTaskUntilDone(ctx context.Context, grpc *grpcClient, agentID, taskID string) (*mcp.ToolResult, error) {
	pollReq := map[string]any{
		"agentId":       agentID,
		"taskId":        taskID,
		"historyLength": 10,
	}

	for range maxPollAttempts {
		select {
		case <-ctx.Done():
			return mcp.ErrResult(ctx.Err())
		default:
		}

		resp, err := grpc.call(ctx, "arp.v1.AgentService", "GetAgentTaskStatus", pollReq)
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("poll error: %w", err))
		}

		// Check if terminal.
		var task map[string]any
		if err := json.Unmarshal(resp, &task); err == nil {
			// Check nested status.state.
			if statusObj, ok := task["status"].(map[string]any); ok {
				if state, ok := statusObj["state"].(string); ok && isTerminalStatus(state) {
					return &mcp.ToolResult{Data: string(resp)}, nil
				}
			}
			// Check flat state field.
			if state, ok := task["state"].(string); ok && isTerminalStatus(state) {
				return &mcp.ToolResult{Data: string(resp)}, nil
			}
		}

		// Brief pause before next poll to avoid tight loop.
		time.Sleep(100 * time.Millisecond)
	}

	// Timed out — return last status with warning.
	resp, _ := grpc.call(ctx, "arp.v1.AgentService", "GetAgentTaskStatus", pollReq)
	if resp != nil {
		return &mcp.ToolResult{Data: fmt.Sprintf(`{"task":%s,"_warning":"polling timeout — task may still be running"}`, string(resp))}, nil
	}
	return mcp.ErrResult(fmt.Errorf("polling timeout for task %s", taskID))
}

func handleAgentTask(ctx context.Context, grpc *grpcClient, _ *httpClient, args map[string]any) (*mcp.ToolResult, error) {
	agentID, _ := mcp.ArgStr(args, "agent_id")
	message, _ := mcp.ArgStr(args, "message")
	if agentID == "" {
		return mcp.ErrResult(fmt.Errorf("agent_id is required"))
	}
	if message == "" {
		return mcp.ErrResult(fmt.Errorf("message is required"))
	}

	req := map[string]any{
		"agentId": agentID,
		"message": message,
	}
	if contextID, _ := mcp.ArgStr(args, "context_id"); contextID != "" {
		req["contextId"] = contextID
	}

	resp, err := grpc.call(ctx, "arp.v1.AgentService", "CreateAgentTask", req)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(resp)}, nil
}

func handleAgentTaskStatus(ctx context.Context, grpc *grpcClient, _ *httpClient, args map[string]any) (*mcp.ToolResult, error) {
	agentID, _ := mcp.ArgStr(args, "agent_id")
	taskID, _ := mcp.ArgStr(args, "task_id")
	if agentID == "" {
		return mcp.ErrResult(fmt.Errorf("agent_id is required"))
	}
	if taskID == "" {
		return mcp.ErrResult(fmt.Errorf("task_id is required"))
	}

	req := map[string]any{
		"agentId": agentID,
		"taskId":  taskID,
	}
	if hl, _ := mcp.ArgStr(args, "history_length"); hl != "" {
		if v, err := strconv.Atoi(hl); err == nil {
			req["historyLength"] = v
		}
	}

	resp, err := grpc.call(ctx, "arp.v1.AgentService", "GetAgentTaskStatus", req)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(resp)}, nil
}

// ---------------------------------------------------------------------------
// DiscoveryService handler
// ---------------------------------------------------------------------------

func handleDiscover(ctx context.Context, grpc *grpcClient, _ *httpClient, args map[string]any) (*mcp.ToolResult, error) {
	req := map[string]any{}
	if scope, _ := mcp.ArgStr(args, "scope"); scope != "" {
		switch scope {
		case "local", "DISCOVERY_SCOPE_LOCAL":
			req["scope"] = 1
		case "network", "DISCOVERY_SCOPE_NETWORK":
			req["scope"] = 2
		}
	}
	if capability, _ := mcp.ArgStr(args, "capability"); capability != "" {
		req["capability"] = capability
	}
	if urlsJSON, _ := mcp.ArgStr(args, "urls"); urlsJSON != "" {
		var urls []any
		if err := json.Unmarshal([]byte(urlsJSON), &urls); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid urls JSON: %w", err))
		}
		req["urls"] = urls
	}

	resp, err := grpc.call(ctx, "arp.v1.DiscoveryService", "DiscoverAgents", req)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(resp)}, nil
}

// ---------------------------------------------------------------------------
// A2A proxy handlers (HTTP)
// ---------------------------------------------------------------------------

func handleProxyList(ctx context.Context, _ *grpcClient, h *httpClient, _ map[string]any) (*mcp.ToolResult, error) {
	resp, err := h.get(ctx, "/a2a/agents")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(resp)}, nil
}

func handleAgentCard(ctx context.Context, _ *grpcClient, h *httpClient, args map[string]any) (*mcp.ToolResult, error) {
	agentID, _ := mcp.ArgStr(args, "agent_id")
	if agentID == "" {
		return mcp.ErrResult(fmt.Errorf("agent_id is required"))
	}

	path := fmt.Sprintf("/a2a/agents/%s/.well-known/agent-card.json", encodePath(agentID))
	resp, err := h.get(ctx, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(resp)}, nil
}

func handleProxySendMessage(ctx context.Context, _ *grpcClient, h *httpClient, args map[string]any) (*mcp.ToolResult, error) {
	agentID, _ := mcp.ArgStr(args, "agent_id")
	message, _ := mcp.ArgStr(args, "message")
	if agentID == "" {
		return mcp.ErrResult(fmt.Errorf("agent_id is required"))
	}
	if message == "" {
		return mcp.ErrResult(fmt.Errorf("message is required"))
	}

	msgID, _ := mcp.ArgStr(args, "message_id")
	if msgID == "" {
		msgID = genMessageID()
	}
	contextID, _ := mcp.ArgStr(args, "context_id")

	msg := map[string]any{
		"role":       "ROLE_USER",
		"parts":      []map[string]any{{"text": message}},
		"message_id": msgID,
	}
	if contextID != "" {
		msg["context_id"] = contextID
	}

	payload := map[string]any{"message": msg}

	path := fmt.Sprintf("/a2a/agents/%s/message:send", encodePath(agentID))
	resp, err := h.post(ctx, path, payload)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(resp)}, nil
}

func handleProxyGetTask(ctx context.Context, _ *grpcClient, h *httpClient, args map[string]any) (*mcp.ToolResult, error) {
	agentID, _ := mcp.ArgStr(args, "agent_id")
	taskID, _ := mcp.ArgStr(args, "task_id")
	if agentID == "" {
		return mcp.ErrResult(fmt.Errorf("agent_id is required"))
	}
	if taskID == "" {
		return mcp.ErrResult(fmt.Errorf("task_id is required"))
	}

	path := fmt.Sprintf("/a2a/agents/%s/tasks/%s", encodePath(agentID), encodePath(taskID))
	resp, err := h.get(ctx, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(resp)}, nil
}

func handleProxyCancelTask(ctx context.Context, _ *grpcClient, h *httpClient, args map[string]any) (*mcp.ToolResult, error) {
	agentID, _ := mcp.ArgStr(args, "agent_id")
	taskID, _ := mcp.ArgStr(args, "task_id")
	if agentID == "" {
		return mcp.ErrResult(fmt.Errorf("agent_id is required"))
	}
	if taskID == "" {
		return mcp.ErrResult(fmt.Errorf("task_id is required"))
	}

	path := fmt.Sprintf("/a2a/agents/%s/tasks/%s:cancel", encodePath(agentID), encodePath(taskID))
	resp, err := h.post(ctx, path, map[string]any{})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(resp)}, nil
}

func handleRouteMessage(ctx context.Context, _ *grpcClient, h *httpClient, args map[string]any) (*mcp.ToolResult, error) {
	message, _ := mcp.ArgStr(args, "message")
	tagsJSON, _ := mcp.ArgStr(args, "tags")
	if message == "" {
		return mcp.ErrResult(fmt.Errorf("message is required"))
	}
	if tagsJSON == "" {
		return mcp.ErrResult(fmt.Errorf("tags is required"))
	}

	var tags any
	if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid tags JSON: %w", err))
	}

	msgID, _ := mcp.ArgStr(args, "message_id")
	if msgID == "" {
		msgID = genMessageID()
	}
	contextID, _ := mcp.ArgStr(args, "context_id")

	msg := map[string]any{
		"role":       "ROLE_USER",
		"parts":      []map[string]any{{"text": message}},
		"message_id": msgID,
	}
	if contextID != "" {
		msg["context_id"] = contextID
	}

	payload := map[string]any{
		"message": msg,
		"routing": map[string]any{"tags": tags},
	}

	resp, err := h.post(ctx, "/a2a/route/message:send", payload)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(resp)}, nil
}
