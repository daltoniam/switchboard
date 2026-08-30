package server

import (
	"context"
	"strings"

	"github.com/daltoniam/switchboard/awm"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func attachProjectResources(mcpSrv *mcpsdk.Server, h *projectWorkHandlers) {
	destructive := true
	notDestructive := false
	closed := false

	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_resource_list",
		Description: "List independently addressable Resources known to AWM. Start here to discover repositories, workspace roots, datasets, and other bindable resources.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, h.listResources)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_resource_get",
		Description: "Get a Resource by resource_id. Resource identity is descriptive and does not grant access.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, h.getResource)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_resource_put",
		Description: "Create or replace an independently addressable Resource.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &notDestructive, OpenWorldHint: &closed},
	}, h.putResource)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_resource_delete",
		Description: "Delete a Resource. Fails while any retained ResourceBinding references it.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &destructive, OpenWorldHint: &closed},
	}, h.deleteResource)

	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_resource_binding_list",
		Description: "List session-specific ResourceBindings. Start here to discover which resources are attached to WorkSessions; optional filters are state, work_session_id, and resource_id.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, h.listResourceBindings)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_resource_binding_get",
		Description: "Get a ResourceBinding by resource_binding_id.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, h.getResourceBinding)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_resource_binding_create",
		Description: "Create a WorkSession-specific binding to one Resource with a resolved locator and narrowed capability grant.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &notDestructive, OpenWorldHint: &closed},
	}, h.createResourceBinding)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_resource_binding_transition",
		Description: "Transition ResourceBinding state (proposed|bound|revoked).",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &destructive, OpenWorldHint: &closed},
	}, h.transitionResourceBinding)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_resource_binding_patch",
		Description: "Patch a non-revoked ResourceBinding's resolved_locator and/or narrowed grant.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &notDestructive, OpenWorldHint: &closed},
	}, h.patchResourceBinding)
	mcpsdk.AddTool(mcpSrv, &mcpsdk.Tool{
		Name:        "project_resource_binding_delete",
		Description: "Delete a ResourceBinding record. Does not delete its WorkSession or Resource.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &destructive, OpenWorldHint: &closed},
	}, h.deleteResourceBinding)
}

type resourceIDIn struct {
	ResourceID string `json:"resource_id"`
}

type resourceBindingIDIn struct {
	ResourceBindingID string `json:"resource_binding_id"`
}

type resourceBindingFilterIn struct {
	State         string `json:"state,omitempty"`
	WorkSessionID string `json:"work_session_id,omitempty"`
	ResourceID    string `json:"resource_id,omitempty"`
}

type createResourceBindingIn struct {
	Version           string             `json:"version"`
	ResourceBindingID string             `json:"resource_binding_id"`
	WorkSessionID     string             `json:"work_session_id"`
	ResourceID        string             `json:"resource_id"`
	Grant             awm.PolicyDocument `json:"grant,omitempty"`
	ResolvedLocator   string             `json:"resolved_locator,omitempty"`
	State             string             `json:"state,omitempty"`
}

type transitionResourceBindingIn struct {
	ResourceBindingID string `json:"resource_binding_id"`
	State             string `json:"state"`
}

type patchResourceBindingIn struct {
	ResourceBindingID string             `json:"resource_binding_id"`
	ResolvedLocator   *string            `json:"resolved_locator,omitempty"`
	Grant             awm.PolicyDocument `json:"grant,omitempty"`
}

func (h *projectWorkHandlers) listResources(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
	resources, err := h.store.ListResources(ctx)
	if err != nil {
		return projectWorkErr(err)
	}
	if resources == nil {
		resources = []awm.Resource{}
	}
	return projectWorkOK(map[string]any{"resources": resources})
}

func (h *projectWorkHandlers) getResource(ctx context.Context, _ *mcpsdk.CallToolRequest, in resourceIDIn) (*mcpsdk.CallToolResult, any, error) {
	resource, err := h.store.GetResource(ctx, strings.TrimSpace(in.ResourceID))
	if err != nil {
		return projectWorkErr(err)
	}
	return projectWorkOK(resource)
}

func (h *projectWorkHandlers) putResource(ctx context.Context, _ *mcpsdk.CallToolRequest, in awm.Resource) (*mcpsdk.CallToolResult, any, error) {
	if !h.writesEnabled {
		return h.writeDisabled()
	}
	resource, err := h.store.PutResource(ctx, in)
	if err != nil {
		return projectWorkErr(err)
	}
	return projectWorkOK(resource)
}

func (h *projectWorkHandlers) deleteResource(ctx context.Context, _ *mcpsdk.CallToolRequest, in resourceIDIn) (*mcpsdk.CallToolResult, any, error) {
	if !h.writesEnabled {
		return h.writeDisabled()
	}
	id := strings.TrimSpace(in.ResourceID)
	if err := h.store.DeleteResource(ctx, id); err != nil {
		return projectWorkErr(err)
	}
	return projectWorkOK(map[string]any{"resource_id": id, "deleted": true})
}

func (h *projectWorkHandlers) listResourceBindings(ctx context.Context, _ *mcpsdk.CallToolRequest, in resourceBindingFilterIn) (*mcpsdk.CallToolResult, any, error) {
	bindings, err := h.store.ListResourceBindings(ctx, strings.TrimSpace(in.State), strings.TrimSpace(in.WorkSessionID), strings.TrimSpace(in.ResourceID))
	if err != nil {
		return projectWorkErr(err)
	}
	if bindings == nil {
		bindings = []awm.ResourceBinding{}
	}
	return projectWorkOK(map[string]any{"resource_bindings": bindings})
}

func (h *projectWorkHandlers) getResourceBinding(ctx context.Context, _ *mcpsdk.CallToolRequest, in resourceBindingIDIn) (*mcpsdk.CallToolResult, any, error) {
	binding, err := h.store.GetResourceBinding(ctx, strings.TrimSpace(in.ResourceBindingID))
	if err != nil {
		return projectWorkErr(err)
	}
	return projectWorkOK(binding)
}

func (h *projectWorkHandlers) createResourceBinding(ctx context.Context, _ *mcpsdk.CallToolRequest, in createResourceBindingIn) (*mcpsdk.CallToolResult, any, error) {
	if !h.writesEnabled {
		return h.writeDisabled()
	}
	binding, err := h.store.CreateResourceBinding(ctx, awm.ResourceBinding{
		Version: in.Version, ResourceBindingID: in.ResourceBindingID,
		WorkSessionID: in.WorkSessionID, ResourceID: in.ResourceID,
		Grant: in.Grant, ResolvedLocator: in.ResolvedLocator, State: in.State,
	})
	if err != nil {
		return projectWorkErr(err)
	}
	return projectWorkOK(binding)
}

func (h *projectWorkHandlers) transitionResourceBinding(ctx context.Context, _ *mcpsdk.CallToolRequest, in transitionResourceBindingIn) (*mcpsdk.CallToolResult, any, error) {
	if !h.writesEnabled {
		return h.writeDisabled()
	}
	binding, err := h.store.TransitionResourceBinding(ctx, strings.TrimSpace(in.ResourceBindingID), strings.TrimSpace(in.State))
	if err != nil {
		return projectWorkErr(err)
	}
	return projectWorkOK(binding)
}

func (h *projectWorkHandlers) patchResourceBinding(ctx context.Context, _ *mcpsdk.CallToolRequest, in patchResourceBindingIn) (*mcpsdk.CallToolResult, any, error) {
	if !h.writesEnabled {
		return h.writeDisabled()
	}
	binding, err := h.store.PatchResourceBinding(ctx, strings.TrimSpace(in.ResourceBindingID), in.ResolvedLocator, in.Grant)
	if err != nil {
		return projectWorkErr(err)
	}
	return projectWorkOK(binding)
}

func (h *projectWorkHandlers) deleteResourceBinding(ctx context.Context, _ *mcpsdk.CallToolRequest, in resourceBindingIDIn) (*mcpsdk.CallToolResult, any, error) {
	if !h.writesEnabled {
		return h.writeDisabled()
	}
	id := strings.TrimSpace(in.ResourceBindingID)
	if err := h.store.DeleteResourceBinding(ctx, id); err != nil {
		return projectWorkErr(err)
	}
	return projectWorkOK(map[string]any{"resource_binding_id": id, "deleted": true})
}
