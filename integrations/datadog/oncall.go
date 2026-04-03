package datadog

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	mcp "github.com/daltoniam/switchboard"
	"github.com/google/uuid"
)

// ── On-Call Schedules ────────────────────────────────────────────────

func getOnCallSchedule(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("schedule_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewOnCallApi(d.client)
	opts := datadogV2.NewGetOnCallScheduleOptionalParameters()
	opts = opts.WithInclude("layers,layers.members,layers.members.user")
	resp, _, err := api.GetOnCallSchedule(ctx, id, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getScheduleOnCallUser(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("schedule_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewOnCallApi(d.client)
	resp, _, err := api.GetScheduleOnCallUser(ctx, id, *datadogV2.NewGetScheduleOnCallUserOptionalParameters())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

// ── On-Call Escalation Policies ──────────────────────────────────────

func getOnCallEscalationPolicy(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("policy_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewOnCallApi(d.client)
	opts := datadogV2.NewGetOnCallEscalationPolicyOptionalParameters()
	opts = opts.WithInclude("rules,rules.members,rules.members.user,rules.members.schedule")
	resp, _, err := api.GetOnCallEscalationPolicy(ctx, id, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

// ── On-Call Team Routing ─────────────────────────────────────────────

func getOnCallTeamRoutingRules(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewOnCallApi(d.client)
	opts := datadogV2.NewGetOnCallTeamRoutingRulesOptionalParameters()
	opts = opts.WithInclude("escalation_policy,schedule")
	resp, _, err := api.GetOnCallTeamRoutingRules(ctx, teamID, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getTeamOnCallUsers(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewOnCallApi(d.client)
	resp, _, err := api.GetTeamOnCallUsers(ctx, teamID, *datadogV2.NewGetTeamOnCallUsersOptionalParameters())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

// ── On-Call Paging ───────────────────────────────────────────────────

func createOnCallPage(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	title := r.Str("title")
	urgency := r.Str("urgency")
	targetID := r.Str("target_id")
	targetType := r.Str("target_type")

	if urgency == "" {
		urgency = "high"
	}
	if targetType == "" {
		targetType = "team_handle"
	}

	attrs := datadogV2.CreatePageRequestDataAttributes{
		Title:   title,
		Urgency: datadogV2.PageUrgency(urgency),
		Target: datadogV2.CreatePageRequestDataAttributesTarget{
			Identifier: &targetID,
			Type:       (*datadogV2.OnCallPageTargetType)(&targetType),
		},
	}

	if v := r.Str("description"); v != "" {
		attrs.Description = &v
	}
	if tags := r.StrSlice("tags"); len(tags) > 0 {
		attrs.Tags = tags
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := datadogV2.CreatePageRequest{
		Data: &datadogV2.CreatePageRequestData{
			Attributes: &attrs,
			Type:       datadogV2.CREATEPAGEREQUESTDATATYPE_PAGES,
		},
	}

	api := datadogV2.NewOnCallPagingApi(d.client)
	resp, _, err := api.CreateOnCallPage(ctx, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func acknowledgeOnCallPage(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	pageID := r.Str("page_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	id, err := uuid.Parse(pageID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewOnCallPagingApi(d.client)
	_, err = api.AcknowledgeOnCallPage(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "acknowledged"})
}

func escalateOnCallPage(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	pageID := r.Str("page_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	id, err := uuid.Parse(pageID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewOnCallPagingApi(d.client)
	_, err = api.EscalateOnCallPage(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "escalated"})
}

func resolveOnCallPage(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	pageID := r.Str("page_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	id, err := uuid.Parse(pageID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewOnCallPagingApi(d.client)
	_, err = api.ResolveOnCallPage(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "resolved"})
}

// ── On-Call Schedule CRUD ────────────────────────────────────────────

func createOnCallSchedule(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	bodyJSON := r.Str("body_json")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var body datadogV2.ScheduleCreateRequest
	if err := json.Unmarshal([]byte(bodyJSON), &body); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid body_json: %w", err))
	}
	api := datadogV2.NewOnCallApi(d.client)
	resp, _, err := api.CreateOnCallSchedule(ctx, body, *datadogV2.NewCreateOnCallScheduleOptionalParameters())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func updateOnCallSchedule(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	scheduleID := r.Str("schedule_id")
	bodyJSON := r.Str("body_json")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var body datadogV2.ScheduleUpdateRequest
	if err := json.Unmarshal([]byte(bodyJSON), &body); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid body_json: %w", err))
	}
	api := datadogV2.NewOnCallApi(d.client)
	resp, _, err := api.UpdateOnCallSchedule(ctx, scheduleID, body, *datadogV2.NewUpdateOnCallScheduleOptionalParameters())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func deleteOnCallSchedule(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("schedule_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewOnCallApi(d.client)
	_, err := api.DeleteOnCallSchedule(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

// ── On-Call Escalation Policy CRUD ───────────────────────────────────

func createOnCallEscalationPolicy(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	bodyJSON := r.Str("body_json")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var body datadogV2.EscalationPolicyCreateRequest
	if err := json.Unmarshal([]byte(bodyJSON), &body); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid body_json: %w", err))
	}
	api := datadogV2.NewOnCallApi(d.client)
	resp, _, err := api.CreateOnCallEscalationPolicy(ctx, body, *datadogV2.NewCreateOnCallEscalationPolicyOptionalParameters())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func updateOnCallEscalationPolicy(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	policyID := r.Str("policy_id")
	bodyJSON := r.Str("body_json")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var body datadogV2.EscalationPolicyUpdateRequest
	if err := json.Unmarshal([]byte(bodyJSON), &body); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid body_json: %w", err))
	}
	api := datadogV2.NewOnCallApi(d.client)
	resp, _, err := api.UpdateOnCallEscalationPolicy(ctx, policyID, body, *datadogV2.NewUpdateOnCallEscalationPolicyOptionalParameters())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func deleteOnCallEscalationPolicy(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("policy_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewOnCallApi(d.client)
	_, err := api.DeleteOnCallEscalationPolicy(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

// ── On-Call Team Routing Rules CRUD ──────────────────────────────────

func setOnCallTeamRoutingRules(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	bodyJSON := r.Str("body_json")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var body datadogV2.TeamRoutingRulesRequest
	if err := json.Unmarshal([]byte(bodyJSON), &body); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid body_json: %w", err))
	}
	api := datadogV2.NewOnCallApi(d.client)
	resp, _, err := api.SetOnCallTeamRoutingRules(ctx, teamID, body, *datadogV2.NewSetOnCallTeamRoutingRulesOptionalParameters())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}
