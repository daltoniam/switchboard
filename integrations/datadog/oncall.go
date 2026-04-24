package datadog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	mcp "github.com/daltoniam/switchboard"
	"github.com/google/uuid"
)

// ── Raw HTTP helper ──────────────────────────────────────────────────

// ddGet performs a raw GET against the Datadog API for endpoints not yet
// exposed by the Go SDK. It reuses the SDK client for auth, retry, and
// base-URL resolution.
func ddGet(ctx context.Context, d *dd, path string, query url.Values) (*mcp.ToolResult, error) {
	base, err := d.client.Cfg.ServerURLWithContext(ctx, "")
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("resolve base URL: %w", err))
	}
	headers := map[string]string{"Accept": "application/json"}
	req, err := d.client.PrepareRequest(ctx, base+path, http.MethodGet, nil, headers, query, url.Values{}, nil)
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("prepare request: %w", err))
	}
	resp, err := d.client.CallAPI(req)
	if err != nil {
		return mcp.ErrResult(err)
	}
	defer resp.Body.Close() //nolint:errcheck
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("read response: %w", err))
	}
	if resp.StatusCode >= 400 {
		return mcp.ErrResult(fmt.Errorf("HTTP %d: %s", resp.StatusCode, body))
	}
	return mcp.RawResult(body)
}

// ── On-Call Schedules ────────────────────────────────────────────────

func listOnCallSchedules(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := url.Values{}
	if v := args["include"]; v != nil {
		q.Set("include", fmt.Sprint(v))
	}
	return ddGet(ctx, d, "/api/v2/on-call/schedules", q)
}

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

func listOnCallEscalationPolicies(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := url.Values{}
	if v := args["include"]; v != nil {
		q.Set("include", fmt.Sprint(v))
	}
	return ddGet(ctx, d, "/api/v2/on-call/escalation-policies", q)
}

func getOnCallEscalationPolicy(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("policy_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewOnCallApi(d.client)
	opts := datadogV2.NewGetOnCallEscalationPolicyOptionalParameters()
	opts = opts.WithInclude("steps,steps.targets")
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
	opts = opts.WithInclude("rules,rules.policy")
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

func listOnCallPages(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := url.Values{}
	if v := args["include"]; v != nil {
		q.Set("include", fmt.Sprint(v))
	}
	if v := args["status"]; v != nil {
		q.Set("filter[status]", fmt.Sprint(v))
	}
	if v := args["urgency"]; v != nil {
		q.Set("filter[urgency]", fmt.Sprint(v))
	}
	return ddGet(ctx, d, "/api/v2/on-call/pages", q)
}

func getOnCallPage(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	pageID := r.Str("page_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := url.Values{}
	if v := args["include"]; v != nil {
		q.Set("include", fmt.Sprint(v))
	}
	return ddGet(ctx, d, "/api/v2/on-call/pages/"+url.PathEscape(pageID), q)
}

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

// ── Notification Channels ────────────────────────────────────────────

func listUserNotificationChannels(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewOnCallApi(d.client)
	resp, _, err := api.ListUserNotificationChannels(ctx, userID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func createUserNotificationChannel(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	bodyJSON := r.Str("body_json")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var body datadogV2.CreateUserNotificationChannelRequest
	if err := json.Unmarshal([]byte(bodyJSON), &body); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid body_json: %w", err))
	}
	api := datadogV2.NewOnCallApi(d.client)
	resp, _, err := api.CreateUserNotificationChannel(ctx, userID, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getUserNotificationChannel(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	channelID := r.Str("channel_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewOnCallApi(d.client)
	resp, _, err := api.GetUserNotificationChannel(ctx, userID, channelID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func deleteUserNotificationChannel(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	channelID := r.Str("channel_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewOnCallApi(d.client)
	_, err := api.DeleteUserNotificationChannel(ctx, userID, channelID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

// ── Notification Rules ───────────────────────────────────────────────

func listUserNotificationRules(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewOnCallApi(d.client)
	opts := datadogV2.NewListUserNotificationRulesOptionalParameters()
	if v := args["include"]; v != nil {
		include := fmt.Sprint(v)
		opts = opts.WithInclude(include)
	}
	resp, _, err := api.ListUserNotificationRules(ctx, userID, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func createUserNotificationRule(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	bodyJSON := r.Str("body_json")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var body datadogV2.CreateOnCallNotificationRuleRequest
	if err := json.Unmarshal([]byte(bodyJSON), &body); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid body_json: %w", err))
	}
	api := datadogV2.NewOnCallApi(d.client)
	resp, _, err := api.CreateUserNotificationRule(ctx, userID, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getUserNotificationRule(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	ruleID := r.Str("rule_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewOnCallApi(d.client)
	opts := datadogV2.NewGetUserNotificationRuleOptionalParameters()
	if v := args["include"]; v != nil {
		include := fmt.Sprint(v)
		opts = opts.WithInclude(include)
	}
	resp, _, err := api.GetUserNotificationRule(ctx, userID, ruleID, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func updateUserNotificationRule(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	ruleID := r.Str("rule_id")
	bodyJSON := r.Str("body_json")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var body datadogV2.UpdateOnCallNotificationRuleRequest
	if err := json.Unmarshal([]byte(bodyJSON), &body); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid body_json: %w", err))
	}
	api := datadogV2.NewOnCallApi(d.client)
	opts := datadogV2.NewUpdateUserNotificationRuleOptionalParameters()
	if v := args["include"]; v != nil {
		include := fmt.Sprint(v)
		opts = opts.WithInclude(include)
	}
	resp, _, err := api.UpdateUserNotificationRule(ctx, userID, ruleID, body, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func deleteUserNotificationRule(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	ruleID := r.Str("rule_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewOnCallApi(d.client)
	_, err := api.DeleteUserNotificationRule(ctx, userID, ruleID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}
