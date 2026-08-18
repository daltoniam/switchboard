package server

import (
	"context"
	"strings"

	"github.com/daltoniam/switchboard/awm"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// AttachProjectWorkModel registers WorkProfile, WorkSession, and AgentProfile tools
// under the project_* namespace on the main MCP server.
// writesEnabled gates mutation tools with the same project_catalog.writes_enabled flag.
func AttachProjectWorkModel(mcpSrv *mcpsdk.Server, store *awm.Store, writesEnabled bool) {
	if mcpSrv == nil || store == nil {
		return
	}
	h := &projectWorkHandlers{store: store, writesEnabled: writesEnabled}

	destructive := true
	notDestructive := false
	closed := false

	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_work_profile_list",
		Description: "List WorkProfile blueprints (session blueprints / session profiles). Start here to discover reusable work episode templates.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, h.listWorkProfiles)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_work_profile_get",
		Description: "Get a WorkProfile by work_profile_id.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, h.getWorkProfile)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_work_profile_put",
		Description: "Create or replace a WorkProfile (session blueprint).",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &notDestructive, OpenWorldHint: &closed},
	}, h.putWorkProfile)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_work_profile_delete",
		Description: "Delete a WorkProfile by id.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &destructive, OpenWorldHint: &closed},
	}, h.deleteWorkProfile)

	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_agent_profile_list",
		Description: "List AgentProfile kinds (eligible agent types, not running instances).",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, h.listAgentProfiles)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_agent_profile_get",
		Description: "Get an AgentProfile by agent_profile_id.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, h.getAgentProfile)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_agent_profile_put",
		Description: "Create or replace an AgentProfile.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &notDestructive, OpenWorldHint: &closed},
	}, h.putAgentProfile)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_agent_profile_delete",
		Description: "Delete an AgentProfile by id.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &destructive, OpenWorldHint: &closed},
	}, h.deleteAgentProfile)

	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_work_session_list",
		Description: "List WorkSessions (bounded work episodes). Optional filters: state, project_id. Not MCP transport sessions.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, h.listWorkSessions)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_work_session_get",
		Description: "Get a WorkSession by work_session_id.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, h.getWorkSession)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_work_session_create",
		Description: "Create a WorkSession. May reference project_id, project_revision, work_profile_id, agent_profile_ids.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &notDestructive, OpenWorldHint: &closed},
	}, h.createWorkSession)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_work_session_transition",
		Description: "Transition a WorkSession lifecycle state (proposed|open|paused|closed|aborted).",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &destructive, OpenWorldHint: &closed},
	}, h.transitionWorkSession)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_work_session_patch",
		Description: "Patch mutable WorkSession fields (display_name, agent_profile_ids, policy). Cannot patch terminal sessions.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &notDestructive, OpenWorldHint: &closed},
	}, h.patchWorkSession)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_work_session_delete",
		Description: "Delete a WorkSession record. Does not delete projects or profiles.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &destructive, OpenWorldHint: &closed},
	}, h.deleteWorkSession)

}

type projectWorkHandlers struct {
	store         *awm.Store
	writesEnabled bool
}

func (h *projectWorkHandlers) writeDisabled() (*mcpsdk.CallToolResult, any, error) {
	return projectWorkErr(&awm.Error{
		Code:    awm.CodeInvalidInput,
		Message: "write_disabled: project_catalog.writes_enabled is false",
	})
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

func projectWorkOK(v any) (*mcpsdk.CallToolResult, any, error) {
	return nil, v, nil
}

func projectWorkErr(err error) (*mcpsdk.CallToolResult, any, error) {
	body := map[string]any{"message": err.Error()}
	if e, ok := awm.AsError(err); ok {
		body = map[string]any{
			"code":            e.Code,
			"message":         e.Message,
			"entity_kind":     e.EntityKind,
			"entity_id":       e.EntityID,
			"project_id":      e.ProjectID,
			"work_profile_id": e.WorkProfileID,
			"work_session_id": e.WorkSessionID,
		}
		if e.Expected != "" {
			body["expected"] = e.Expected
		}
		if e.Current != "" {
			body["current"] = e.Current
		}
	}
	msg := err.Error()
	return &mcpsdk.CallToolResult{
		IsError: true,
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: msg}},
		StructuredContent: map[string]any{
			"error": body,
		},
	}, nil, nil
}

func (h *projectWorkHandlers) listWorkProfiles(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
	list, err := h.store.ListWorkProfiles(ctx)
	if err != nil {
		return projectWorkErr(err)
	}
	if list == nil {
		list = []awm.WorkProfile{}
	}
	return projectWorkOK(map[string]any{"work_profiles": list})
}

func (h *projectWorkHandlers) getWorkProfile(ctx context.Context, _ *mcpsdk.CallToolRequest, in idIn) (*mcpsdk.CallToolResult, any, error) {
	p, err := h.store.GetWorkProfile(ctx, strings.TrimSpace(in.ID))
	if err != nil {
		return projectWorkErr(err)
	}
	return projectWorkOK(p)
}

func (h *projectWorkHandlers) putWorkProfile(ctx context.Context, _ *mcpsdk.CallToolRequest, in awm.WorkProfile) (*mcpsdk.CallToolResult, any, error) {
	if !h.writesEnabled {
		return h.writeDisabled()
	}
	p, err := h.store.PutWorkProfile(ctx, in)
	if err != nil {
		return projectWorkErr(err)
	}
	return projectWorkOK(p)
}

func (h *projectWorkHandlers) deleteWorkProfile(ctx context.Context, _ *mcpsdk.CallToolRequest, in idIn) (*mcpsdk.CallToolResult, any, error) {
	if !h.writesEnabled {
		return h.writeDisabled()
	}
	if err := h.store.DeleteWorkProfile(ctx, strings.TrimSpace(in.ID)); err != nil {
		return projectWorkErr(err)
	}
	return projectWorkOK(map[string]any{"id": in.ID, "deleted": true})
}

func (h *projectWorkHandlers) listAgentProfiles(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
	list, err := h.store.ListAgentProfiles(ctx)
	if err != nil {
		return projectWorkErr(err)
	}
	if list == nil {
		list = []awm.AgentProfile{}
	}
	return projectWorkOK(map[string]any{"agent_profiles": list})
}

func (h *projectWorkHandlers) getAgentProfile(ctx context.Context, _ *mcpsdk.CallToolRequest, in idIn) (*mcpsdk.CallToolResult, any, error) {
	p, err := h.store.GetAgentProfile(ctx, strings.TrimSpace(in.ID))
	if err != nil {
		return projectWorkErr(err)
	}
	return projectWorkOK(p)
}

func (h *projectWorkHandlers) putAgentProfile(ctx context.Context, _ *mcpsdk.CallToolRequest, in awm.AgentProfile) (*mcpsdk.CallToolResult, any, error) {
	if !h.writesEnabled {
		return h.writeDisabled()
	}
	p, err := h.store.PutAgentProfile(ctx, in)
	if err != nil {
		return projectWorkErr(err)
	}
	return projectWorkOK(p)
}

func (h *projectWorkHandlers) deleteAgentProfile(ctx context.Context, _ *mcpsdk.CallToolRequest, in idIn) (*mcpsdk.CallToolResult, any, error) {
	if !h.writesEnabled {
		return h.writeDisabled()
	}
	if err := h.store.DeleteAgentProfile(ctx, strings.TrimSpace(in.ID)); err != nil {
		return projectWorkErr(err)
	}
	return projectWorkOK(map[string]any{"id": in.ID, "deleted": true})
}

func (h *projectWorkHandlers) listWorkSessions(ctx context.Context, _ *mcpsdk.CallToolRequest, in stateFilterIn) (*mcpsdk.CallToolResult, any, error) {
	list, err := h.store.ListWorkSessions(ctx, strings.TrimSpace(in.State), strings.TrimSpace(in.ProjectID))
	if err != nil {
		return projectWorkErr(err)
	}
	if list == nil {
		list = []awm.WorkSession{}
	}
	return projectWorkOK(map[string]any{"work_sessions": list})
}

func (h *projectWorkHandlers) getWorkSession(ctx context.Context, _ *mcpsdk.CallToolRequest, in idIn) (*mcpsdk.CallToolResult, any, error) {
	s, err := h.store.GetWorkSession(ctx, strings.TrimSpace(in.ID))
	if err != nil {
		return projectWorkErr(err)
	}
	return projectWorkOK(s)
}

func (h *projectWorkHandlers) createWorkSession(ctx context.Context, _ *mcpsdk.CallToolRequest, in createSessionIn) (*mcpsdk.CallToolResult, any, error) {
	if !h.writesEnabled {
		return h.writeDisabled()
	}
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
		return projectWorkErr(err)
	}
	return projectWorkOK(s)
}

func (h *projectWorkHandlers) transitionWorkSession(ctx context.Context, _ *mcpsdk.CallToolRequest, in transitionIn) (*mcpsdk.CallToolResult, any, error) {
	if !h.writesEnabled {
		return h.writeDisabled()
	}
	s, err := h.store.TransitionWorkSession(ctx, strings.TrimSpace(in.ID), strings.TrimSpace(in.State))
	if err != nil {
		return projectWorkErr(err)
	}
	return projectWorkOK(s)
}

func (h *projectWorkHandlers) patchWorkSession(ctx context.Context, _ *mcpsdk.CallToolRequest, in patchSessionIn) (*mcpsdk.CallToolResult, any, error) {
	if !h.writesEnabled {
		return h.writeDisabled()
	}
	s, err := h.store.PatchWorkSession(ctx, strings.TrimSpace(in.ID), in.DisplayName, in.AgentProfileIDs, in.Policy)
	if err != nil {
		return projectWorkErr(err)
	}
	return projectWorkOK(s)
}

func (h *projectWorkHandlers) deleteWorkSession(ctx context.Context, _ *mcpsdk.CallToolRequest, in idIn) (*mcpsdk.CallToolResult, any, error) {
	if !h.writesEnabled {
		return h.writeDisabled()
	}
	if err := h.store.DeleteWorkSession(ctx, strings.TrimSpace(in.ID)); err != nil {
		return projectWorkErr(err)
	}
	return projectWorkOK(map[string]any{"id": in.ID, "deleted": true})
}
