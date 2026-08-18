package server

import (
	"context"
	"strings"

	"github.com/daltoniam/switchboard/awm"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// AttachAWM registers minimal Agent Work Model tools on the main MCP server.
func AttachAWM(mcpSrv *mcpsdk.Server, store *awm.Store) {
	if mcpSrv == nil || store == nil {
		return
	}
	h := &awmHandlers{store: store}

	destructive := true
	notDestructive := false
	closed := false

	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "awm.work_profile.list",
		Description: "List WorkProfile blueprints (session blueprints / session profiles). Start here to discover reusable work episode templates.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, h.listWorkProfiles)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "awm.work_profile.get",
		Description: "Get a WorkProfile by work_profile_id.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, h.getWorkProfile)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "awm.work_profile.put",
		Description: "Create or replace a WorkProfile (session blueprint).",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &notDestructive, OpenWorldHint: &closed},
	}, h.putWorkProfile)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "awm.work_profile.delete",
		Description: "Delete a WorkProfile by id.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &destructive, OpenWorldHint: &closed},
	}, h.deleteWorkProfile)

	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "awm.agent_profile.list",
		Description: "List AgentProfile kinds (eligible agent types, not running instances).",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, h.listAgentProfiles)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "awm.agent_profile.get",
		Description: "Get an AgentProfile by agent_profile_id.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, h.getAgentProfile)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "awm.agent_profile.put",
		Description: "Create or replace an AgentProfile.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &notDestructive, OpenWorldHint: &closed},
	}, h.putAgentProfile)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "awm.agent_profile.delete",
		Description: "Delete an AgentProfile by id.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &destructive, OpenWorldHint: &closed},
	}, h.deleteAgentProfile)

	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "awm.work_session.list",
		Description: "List WorkSessions (bounded work episodes). Optional filters: state, project_id. Not MCP transport sessions.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, h.listWorkSessions)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "awm.work_session.get",
		Description: "Get a WorkSession by work_session_id.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, h.getWorkSession)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "awm.work_session.create",
		Description: "Create a WorkSession. May reference project_id, project_revision, work_profile_id, agent_profile_ids.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &notDestructive, OpenWorldHint: &closed},
	}, h.createWorkSession)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "awm.work_session.transition",
		Description: "Transition a WorkSession lifecycle state (proposed|open|paused|closed|aborted).",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &destructive, OpenWorldHint: &closed},
	}, h.transitionWorkSession)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "awm.work_session.patch",
		Description: "Patch mutable WorkSession fields (display_name, agent_profile_ids, policy). Cannot patch terminal sessions.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &notDestructive, OpenWorldHint: &closed},
	}, h.patchWorkSession)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "awm.work_session.delete",
		Description: "Delete a WorkSession record. Does not delete projects or profiles.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &destructive, OpenWorldHint: &closed},
	}, h.deleteWorkSession)

}

type awmHandlers struct {
	store *awm.Store
}

type idIn struct {
	ID string `json:"id"`
}

type stateFilterIn struct {
	State     string `json:"state,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
}

type transitionIn struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

type createSessionIn struct {
	Version           string         `json:"version"`
	WorkSessionID     string         `json:"work_session_id"`
	DisplayName       string         `json:"display_name,omitempty"`
	ProjectID         string         `json:"project_id,omitempty"`
	ProjectSnapshotID string         `json:"project_snapshot_id,omitempty"`
	ProjectRevision   string         `json:"project_revision,omitempty"`
	WorkProfileID     string         `json:"work_profile_id,omitempty"`
	AgentProfileIDs   []string       `json:"agent_profile_ids,omitempty"`
	State             string         `json:"state,omitempty"`
	Policy            map[string]any `json:"policy,omitempty"`
}

type patchSessionIn struct {
	ID              string         `json:"id"`
	DisplayName     *string        `json:"display_name,omitempty"`
	AgentProfileIDs *[]string      `json:"agent_profile_ids,omitempty"`
	Policy          map[string]any `json:"policy,omitempty"`
}

func awmOK(v any) (*mcpsdk.CallToolResult, any, error) {
	return nil, v, nil
}

func awmErr(err error) (*mcpsdk.CallToolResult, any, error) {
	msg := err.Error()
	return &mcpsdk.CallToolResult{
		IsError: true,
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: msg}},
		StructuredContent: map[string]any{
			"error": map[string]any{"message": msg},
		},
	}, nil, nil
}

func (h *awmHandlers) listWorkProfiles(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
	list, err := h.store.ListWorkProfiles(ctx)
	if err != nil {
		return awmErr(err)
	}
	if list == nil {
		list = []awm.WorkProfile{}
	}
	return awmOK(map[string]any{"work_profiles": list})
}

func (h *awmHandlers) getWorkProfile(ctx context.Context, _ *mcpsdk.CallToolRequest, in idIn) (*mcpsdk.CallToolResult, any, error) {
	p, err := h.store.GetWorkProfile(ctx, strings.TrimSpace(in.ID))
	if err != nil {
		return awmErr(err)
	}
	return awmOK(p)
}

func (h *awmHandlers) putWorkProfile(ctx context.Context, _ *mcpsdk.CallToolRequest, in awm.WorkProfile) (*mcpsdk.CallToolResult, any, error) {
	p, err := h.store.PutWorkProfile(ctx, in)
	if err != nil {
		return awmErr(err)
	}
	return awmOK(p)
}

func (h *awmHandlers) deleteWorkProfile(ctx context.Context, _ *mcpsdk.CallToolRequest, in idIn) (*mcpsdk.CallToolResult, any, error) {
	if err := h.store.DeleteWorkProfile(ctx, strings.TrimSpace(in.ID)); err != nil {
		return awmErr(err)
	}
	return awmOK(map[string]any{"id": in.ID, "deleted": true})
}

func (h *awmHandlers) listAgentProfiles(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
	list, err := h.store.ListAgentProfiles(ctx)
	if err != nil {
		return awmErr(err)
	}
	if list == nil {
		list = []awm.AgentProfile{}
	}
	return awmOK(map[string]any{"agent_profiles": list})
}

func (h *awmHandlers) getAgentProfile(ctx context.Context, _ *mcpsdk.CallToolRequest, in idIn) (*mcpsdk.CallToolResult, any, error) {
	p, err := h.store.GetAgentProfile(ctx, strings.TrimSpace(in.ID))
	if err != nil {
		return awmErr(err)
	}
	return awmOK(p)
}

func (h *awmHandlers) putAgentProfile(ctx context.Context, _ *mcpsdk.CallToolRequest, in awm.AgentProfile) (*mcpsdk.CallToolResult, any, error) {
	p, err := h.store.PutAgentProfile(ctx, in)
	if err != nil {
		return awmErr(err)
	}
	return awmOK(p)
}

func (h *awmHandlers) deleteAgentProfile(ctx context.Context, _ *mcpsdk.CallToolRequest, in idIn) (*mcpsdk.CallToolResult, any, error) {
	if err := h.store.DeleteAgentProfile(ctx, strings.TrimSpace(in.ID)); err != nil {
		return awmErr(err)
	}
	return awmOK(map[string]any{"id": in.ID, "deleted": true})
}

func (h *awmHandlers) listWorkSessions(ctx context.Context, _ *mcpsdk.CallToolRequest, in stateFilterIn) (*mcpsdk.CallToolResult, any, error) {
	list, err := h.store.ListWorkSessions(ctx, strings.TrimSpace(in.State), strings.TrimSpace(in.ProjectID))
	if err != nil {
		return awmErr(err)
	}
	if list == nil {
		list = []awm.WorkSession{}
	}
	return awmOK(map[string]any{"work_sessions": list})
}

func (h *awmHandlers) getWorkSession(ctx context.Context, _ *mcpsdk.CallToolRequest, in idIn) (*mcpsdk.CallToolResult, any, error) {
	s, err := h.store.GetWorkSession(ctx, strings.TrimSpace(in.ID))
	if err != nil {
		return awmErr(err)
	}
	return awmOK(s)
}

func (h *awmHandlers) createWorkSession(ctx context.Context, _ *mcpsdk.CallToolRequest, in createSessionIn) (*mcpsdk.CallToolResult, any, error) {
	s, err := h.store.CreateWorkSession(ctx, awm.WorkSession{
		Version:           in.Version,
		WorkSessionID:     in.WorkSessionID,
		DisplayName:       in.DisplayName,
		ProjectID:         in.ProjectID,
		ProjectSnapshotID: in.ProjectSnapshotID,
		ProjectRevision:   in.ProjectRevision,
		WorkProfileID:     in.WorkProfileID,
		AgentProfileIDs:   in.AgentProfileIDs,
		State:             in.State,
		Policy:            in.Policy,
	})
	if err != nil {
		return awmErr(err)
	}
	return awmOK(s)
}

func (h *awmHandlers) transitionWorkSession(ctx context.Context, _ *mcpsdk.CallToolRequest, in transitionIn) (*mcpsdk.CallToolResult, any, error) {
	s, err := h.store.TransitionWorkSession(ctx, strings.TrimSpace(in.ID), strings.TrimSpace(in.State))
	if err != nil {
		return awmErr(err)
	}
	return awmOK(s)
}

func (h *awmHandlers) patchWorkSession(ctx context.Context, _ *mcpsdk.CallToolRequest, in patchSessionIn) (*mcpsdk.CallToolResult, any, error) {
	s, err := h.store.PatchWorkSession(ctx, strings.TrimSpace(in.ID), in.DisplayName, in.AgentProfileIDs, in.Policy)
	if err != nil {
		return awmErr(err)
	}
	return awmOK(s)
}

func (h *awmHandlers) deleteWorkSession(ctx context.Context, _ *mcpsdk.CallToolRequest, in idIn) (*mcpsdk.CallToolResult, any, error) {
	if err := h.store.DeleteWorkSession(ctx, strings.TrimSpace(in.ID)); err != nil {
		return awmErr(err)
	}
	return awmOK(map[string]any{"id": in.ID, "deleted": true})
}
