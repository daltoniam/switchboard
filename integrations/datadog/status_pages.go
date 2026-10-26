package datadog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	mcp "github.com/daltoniam/switchboard"
	"github.com/google/uuid"
)

// ── Status Pages ─────────────────────────────────────────────────────

var degradationStatuses = map[string]struct{}{
	"investigating": {}, "identified": {}, "monitoring": {}, "resolved": {},
}

var componentStatuses = map[string]struct{}{
	"operational": {}, "degraded": {}, "partial_outage": {}, "major_outage": {},
}

type affectedComponent struct {
	ID     uuid.UUID
	Name   string
	Status datadogV2.StatusPagesComponentDataAttributesStatus
}

func requireUUID(args map[string]any, key string) (uuid.UUID, error) {
	raw, err := mcp.ArgStr(args, key)
	if err != nil {
		return uuid.Nil, err
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.Nil, fmt.Errorf("parameter %q is required", key)
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parameter %q: %q is not a valid UUID", key, raw)
	}
	return id, nil
}

// parseAffectedComponents accepts a JSON array (already decoded or as a
// string) of {"id": "<uuid>", "status": "<component status>", "name"?: ""}.
func parseAffectedComponents(args map[string]any, key string) ([]affectedComponent, bool, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil, false, nil
	}
	var items []any
	switch v := raw.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, false, nil
		}
		if err := json.Unmarshal([]byte(v), &items); err != nil {
			return nil, true, fmt.Errorf("parameter %q: must be a JSON array of {id, status}: %w", key, err)
		}
	case []any:
		items = v
	case []map[string]any:
		for _, m := range v {
			items = append(items, m)
		}
	default:
		return nil, true, fmt.Errorf("parameter %q: cannot convert %T to an array", key, raw)
	}
	out := make([]affectedComponent, 0, len(items))
	for i, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, true, fmt.Errorf("parameter %q: element %d is %T, not an object", key, i, item)
		}
		id, err := requireUUID(m, "id")
		if err != nil {
			return nil, true, fmt.Errorf("parameter %q: element %d: %w", key, i, err)
		}
		status, err := mcp.ArgStr(m, "status")
		if err != nil {
			return nil, true, fmt.Errorf("parameter %q: element %d: %w", key, i, err)
		}
		if _, ok := componentStatuses[status]; !ok {
			return nil, true, fmt.Errorf("parameter %q: element %d: status %q must be one of operational, degraded, partial_outage, major_outage", key, i, status)
		}
		name, err := mcp.ArgStr(m, "name")
		if err != nil {
			return nil, true, fmt.Errorf("parameter %q: element %d: %w", key, i, err)
		}
		out = append(out, affectedComponent{ID: id, Name: name, Status: datadogV2.StatusPagesComponentDataAttributesStatus(status)})
	}
	return out, true, nil
}

func validDegradationStatus(status string) error {
	if _, ok := degradationStatuses[status]; !ok {
		return fmt.Errorf("status %q must be one of investigating, identified, monitoring, resolved", status)
	}
	return nil
}

func listStatusPages(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	opts := datadogV2.NewListStatusPagesOptionalParameters()
	if v := r.Int32("page_limit"); v > 0 {
		opts = opts.WithPageLimit(v)
	}
	if v := r.Int32("page_offset"); v > 0 {
		opts = opts.WithPageOffset(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := datadogV2.NewStatusPagesApi(d.client).ListStatusPages(ctx, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getStatusPage(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	pageID, err := requireUUID(args, "page_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := datadogV2.NewStatusPagesApi(d.client).GetStatusPage(ctx, pageID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func listStatusPageComponents(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	pageID, err := requireUUID(args, "page_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := datadogV2.NewStatusPagesApi(d.client).ListComponents(ctx, pageID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getStatusPageComponent(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	pageID, err := requireUUID(args, "page_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	componentID, err := requireUUID(args, "component_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := datadogV2.NewStatusPagesApi(d.client).GetComponent(ctx, pageID, componentID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func listStatusPageDegradations(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	opts := datadogV2.NewListDegradationsOptionalParameters()
	pageID := r.Str("page_id")
	status := r.Str("status")
	limit := r.Int32("page_limit")
	offset := r.Int32("page_offset")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if pageID != "" {
		if _, err := uuid.Parse(pageID); err != nil {
			return mcp.ErrResult(fmt.Errorf("parameter \"page_id\": %q is not a valid UUID", pageID))
		}
		opts = opts.WithFilterPageId(pageID)
	}
	if status != "" {
		for _, s := range strings.Split(status, ",") {
			if err := validDegradationStatus(strings.TrimSpace(s)); err != nil {
				return mcp.ErrResult(err)
			}
		}
		opts = opts.WithFilterStatus(status)
	}
	if limit > 0 {
		opts = opts.WithPageLimit(limit)
	}
	if offset > 0 {
		opts = opts.WithPageOffset(offset)
	}
	resp, _, err := datadogV2.NewStatusPagesApi(d.client).ListDegradations(ctx, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getStatusPageDegradation(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	pageID, err := requireUUID(args, "page_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	degradationID, err := requireUUID(args, "degradation_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := datadogV2.NewStatusPagesApi(d.client).GetDegradation(ctx, pageID, degradationID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func createStatusPageDegradation(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	pageID, err := requireUUID(args, "page_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	title := r.Str("title")
	status := r.Str("status")
	description := r.Str("description")
	if _, ok := args["notify_subscribers"]; !ok {
		return mcp.ErrResult(fmt.Errorf("parameter \"notify_subscribers\" is required (true or false)"))
	}
	notify := r.Bool("notify_subscribers")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if strings.TrimSpace(title) == "" {
		return mcp.ErrResult(fmt.Errorf("parameter \"title\" is required"))
	}
	if err := validDegradationStatus(status); err != nil {
		return mcp.ErrResult(err)
	}
	components, present, err := parseAffectedComponents(args, "components_affected")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if !present || len(components) == 0 {
		return mcp.ErrResult(fmt.Errorf("parameter \"components_affected\" must list at least one component"))
	}
	items := make([]datadogV2.CreateDegradationRequestDataAttributesComponentsAffectedItems, 0, len(components))
	for _, c := range components {
		item := datadogV2.CreateDegradationRequestDataAttributesComponentsAffectedItems{Id: c.ID, Status: c.Status}
		if c.Name != "" {
			item.Name = datadog.PtrString(c.Name)
		}
		items = append(items, item)
	}
	attrs := datadogV2.CreateDegradationRequestDataAttributes{
		ComponentsAffected: items,
		Status:             datadogV2.CreateDegradationRequestDataAttributesStatus(status),
		Title:              title,
	}
	if description != "" {
		attrs.Description = datadog.PtrString(description)
	}
	body := datadogV2.CreateDegradationRequest{Data: &datadogV2.CreateDegradationRequestData{
		Attributes: attrs,
		Type:       datadogV2.PATCHDEGRADATIONREQUESTDATATYPE_DEGRADATIONS,
	}}
	opts := datadogV2.NewCreateDegradationOptionalParameters().WithNotifySubscribers(notify)
	resp, _, err := datadogV2.NewStatusPagesApi(d.client).CreateDegradation(ctx, pageID, body, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func updateStatusPageDegradation(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	pageID, err := requireUUID(args, "page_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	degradationID, err := requireUUID(args, "degradation_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	title := r.Str("title")
	status := r.Str("status")
	description := r.Str("description")
	_, notifySet := args["notify_subscribers"]
	notify := r.Bool("notify_subscribers")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	attrs := datadogV2.PatchDegradationRequestDataAttributes{}
	changed := false
	if title != "" {
		attrs.Title = datadog.PtrString(title)
		changed = true
	}
	if status != "" {
		if err := validDegradationStatus(status); err != nil {
			return mcp.ErrResult(err)
		}
		s := datadogV2.PatchDegradationRequestDataAttributesStatus(status)
		attrs.Status = &s
		changed = true
	}
	if description != "" {
		attrs.Description = datadog.PtrString(description)
		changed = true
	}
	components, present, err := parseAffectedComponents(args, "components_affected")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if present {
		items := make([]datadogV2.PatchDegradationRequestDataAttributesComponentsAffectedItems, 0, len(components))
		for _, c := range components {
			item := datadogV2.PatchDegradationRequestDataAttributesComponentsAffectedItems{Id: c.ID, Status: c.Status}
			if c.Name != "" {
				item.Name = datadog.PtrString(c.Name)
			}
			items = append(items, item)
		}
		attrs.ComponentsAffected = items
		changed = true
	}
	if !changed {
		return mcp.ErrResult(fmt.Errorf("nothing to update: set at least one of title, status, description, components_affected"))
	}
	body := datadogV2.PatchDegradationRequest{Data: &datadogV2.PatchDegradationRequestData{
		Attributes: attrs,
		Id:         degradationID,
		Type:       datadogV2.PATCHDEGRADATIONREQUESTDATATYPE_DEGRADATIONS,
	}}
	opts := datadogV2.NewUpdateDegradationOptionalParameters()
	if notifySet {
		opts = opts.WithNotifySubscribers(notify)
	}
	resp, _, err := datadogV2.NewStatusPagesApi(d.client).UpdateDegradation(ctx, pageID, degradationID, body, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func deleteStatusPageDegradation(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	pageID, err := requireUUID(args, "page_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	degradationID, err := requireUUID(args, "degradation_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if _, err := datadogV2.NewStatusPagesApi(d.client).DeleteDegradation(ctx, pageID, degradationID); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted", "degradation_id": degradationID.String()})
}
