package panther

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

// defaultAlertWindow bounds alert listings that name no detection or dates;
// Panther needs one or the other.
const defaultAlertWindow = 7 * 24 * time.Hour

var alertStatuses = []string{"OPEN", "TRIAGED", "RESOLVED", "CLOSED"}

var alertListFilters = []param{
	str("alert_type", "type", "ALERT (default) for detection alerts, DETECTION_ERROR for rule errors, or SYSTEM_ERROR for system health alerts."),
	strs("severity", "severity", "Severities to include: CRITICAL, HIGH, MEDIUM, LOW, INFO (list or comma-separated)."),
	strs("status", "status", "Statuses to include: OPEN, TRIAGED, RESOLVED, CLOSED (list or comma-separated)."),
	str("created_after", "created-after", "Only alerts created at or after this ISO-8601 time. Without detection_id or dates, the last 7 days are listed."),
	str("created_before", "created-before", "Only alerts created before this ISO-8601 time."),
	str("detection_id", "detection-id", "Only alerts from this detection (rule, policy, or scheduled rule id)."),
	str("name_contains", "name-contains", "Only alerts whose title contains this text."),
	str("assignee", "assignee", "Only alerts assigned to this Panther user id."),
	strs("sub_type", "sub-type", "Alert subtypes: RULE, SCHEDULED_RULE, POLICY for ALERT; RULE_ERROR, SCHEDULED_RULE_ERROR for DETECTION_ERROR."),
	strs("log_source", "log-source", "Only alerts from these log source ids."),
	strs("log_type", "log-type", "Only alerts on these log types, such as AWS.CloudTrail."),
	strs("resource_type", "resource-type", "Only policy alerts on these cloud resource types, such as AWS.S3.Bucket."),
	integer("event_count_min", "event-count-min", "Only alerts with at least this many events."),
	integer("event_count_max", "event-count-max", "Only alerts with at most this many events."),
	str("sort_dir", "sort-dir", "Sort by creation time: desc (default, newest first) or asc."),
}

var alertSpecs = []spec{
	{
		name:   "panther_list_alerts",
		desc:   "List Panther SIEM security alerts and incidents with severity, status, detection, event count, assignee, and created time. Start here for security alert triage, incident response, threat detection results, and finding open or critical alerts; filter by severity, status, detection rule, log type, or time range. Also lists detection errors and system errors with alert_type.",
		access: mcp.AccessRead,
		params: slices.Concat(alertListFilters, pageParams()),
		run:    listAlerts,
	},
	restGet("panther_get_alert", "Get one Panther security alert in full: title, severity, status, detection, runbook, event counts, first and last event times, assignee, context tags, and delivery attempts. Use after panther_list_alerts.", "/alerts", "alert_id", "Alert id from panther_list_alerts."),
	{
		name:   "panther_list_alert_events",
		desc:   "List the raw log events behind a Panther security alert (matched events with p_ fields) for investigation and threat hunting. Use after panther_get_alert; event order is not guaranteed.",
		access: mcp.AccessRead,
		params: slices.Concat([]param{str("alert_id", "", "Alert id from panther_list_alerts.").req()}, pageParams()),
		run:    listAlertEvents,
	},
	restList("panther_list_alert_comments", "List comments and analyst notes on a Panther security alert. Use after panther_get_alert.", "/alert-comments",
		str("alert_id", "alert-id", "Alert id whose comments to list.").req()),
	{
		name:   "panther_update_alert_status",
		desc:   "Update (change, set) the status of one or more Panther security alerts: OPEN, TRIAGED, RESOLVED, or CLOSED, to triage, acknowledge, resolve, or reopen them. Use after panther_list_alerts.",
		access: mcp.AccessWrite,
		params: []param{
			strs("alert_ids", "", "Alert ids to update (list or comma-separated).").req(),
			str("status", "", "New status: OPEN, TRIAGED, RESOLVED, or CLOSED.").req(),
		},
		run: updateAlertStatus,
	},
	{
		name:   "panther_assign_alerts",
		desc:   "Assign one or more Panther security alerts to a Panther user (set the assignee) for triage ownership. Use panther_list_users to find the user id.",
		access: mcp.AccessWrite,
		params: []param{
			strs("alert_ids", "", "Alert ids to assign (list or comma-separated).").req(),
			str("assignee_id", "", "Panther user id to assign; from panther_list_users.").req(),
		},
		run: assignAlerts,
	},
	{
		name:   "panther_add_alert_comment",
		desc:   "Add (post, create) a comment or analyst note to a Panther security alert to record triage findings. Use after panther_get_alert.",
		access: mcp.AccessWrite,
		params: []param{
			str("alert_id", "", "Alert id to comment on.").req(),
			str("body", "", "Comment text.").req(),
			str("format", "", "PLAIN_TEXT (default) or HTML."),
		},
		run: addAlertComment,
	},
}

func listAlerts(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
	params := slices.Concat(alertListFilters, pageParams())
	now := time.Now().UTC()
	return listREST(ctx, c, "/alerts", params, args, func(q url.Values) {
		if q.Get("type") == "" {
			q.Set("type", "ALERT")
		}
		if q.Get("sort-dir") == "" {
			q.Set("sort-dir", "desc")
		}
		if q.Get("detection-id") == "" && q.Get("created-after") == "" && q.Get("created-before") == "" {
			q.Set("created-after", now.Add(-defaultAlertWindow).Format(time.RFC3339))
			q.Set("created-before", now.Format(time.RFC3339))
		}
	})
}

func listAlertEvents(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "alert_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	escaped, err := pathID("alert_id", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return listREST(ctx, c, "/alerts/"+escaped+"/events", pageParams(), args, nil)
}

func alertIDs(args map[string]any) ([]string, error) {
	raw, err := mcp.ArgStrSlice(args, "alert_ids")
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, id := range raw {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("alert_ids is required")
	}
	return ids, nil
}

// patchAlerts sends one bulk PATCH /alerts, which answers 204, and reports
// what changed so the caller has a confirmation to show.
func patchAlerts(ctx context.Context, c *client, ids []string, change map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{"ids": ids}
	for k, v := range change {
		body[k] = v
	}
	if _, err := c.rest(ctx, http.MethodPatch, "/alerts", nil, body); err != nil {
		return mcp.ErrResult(err)
	}
	out := map[string]any{"updated": ids}
	for k, v := range change {
		out[k] = v
	}
	return mcp.JSONResult(out)
}

func updateAlertStatus(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
	ids, err := alertIDs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	status, err := mcp.ArgStr(args, "status")
	if err != nil {
		return mcp.ErrResult(err)
	}
	status = strings.ToUpper(strings.TrimSpace(status))
	if !slices.Contains(alertStatuses, status) {
		return mcp.ErrResult(fmt.Errorf("status must be one of %s", strings.Join(alertStatuses, ", ")))
	}
	return patchAlerts(ctx, c, ids, map[string]any{"status": status})
}

func assignAlerts(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
	ids, err := alertIDs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	assignee, err := mcp.ArgStr(args, "assignee_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if assignee = strings.TrimSpace(assignee); assignee == "" {
		return mcp.ErrResult(fmt.Errorf("assignee_id is required"))
	}
	return patchAlerts(ctx, c, ids, map[string]any{"assignee": assignee})
}

func addAlertComment(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	alertID := strings.TrimSpace(r.Str("alert_id"))
	body := r.Str("body")
	format := strings.ToUpper(strings.TrimSpace(r.Str("format")))
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if alertID == "" || strings.TrimSpace(body) == "" {
		return mcp.ErrResult(fmt.Errorf("alert_id and body are required"))
	}
	if format == "" {
		format = "PLAIN_TEXT"
	}
	if format != "PLAIN_TEXT" && format != "HTML" {
		return mcp.ErrResult(fmt.Errorf("format must be PLAIN_TEXT or HTML"))
	}
	data, err := c.rest(ctx, http.MethodPost, "/alert-comments", nil, map[string]any{"alertId": alertID, "body": body, "format": format})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
