package panther

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// detectionPaths maps each detection_type to its REST collection.
var detectionPaths = map[string]string{
	"rule":             "/rules",
	"scheduled_rule":   "/scheduled-rules",
	"simple_rule":      "/simple-rules",
	"policy":           "/policies",
	"correlation_rule": "/correlation-rules",
}

func detectionFilters(extra ...param) []param {
	return slices.Concat([]param{
		str("name_contains", "name-contains", "Only detections whose name or id contains this text."),
		str("state", "state", "enabled or disabled."),
		strs("severity", "severity", "Severities to include: CRITICAL, HIGH, MEDIUM, LOW, INFO."),
		strs("tag", "tag", "Only detections with these tags (for example MITRE ATT&CK tactics)."),
		str("created_by", "created-by", "Only detections created by this user or API token id."),
		str("last_modified_by", "last-modified-by", "Only detections last modified by this user or API token id."),
	}, extra)
}

var detectionSpecs = []spec{
	restList("panther_list_rules", "List Panther detection rules and detections (Python streaming rules on incoming logs): id, name, severity, enabled state, log types, and tags. Start here for detection engineering, detection rule coverage, and finding which rules watch a log type.", "/rules",
		detectionFilters(strs("log_type", "log-type", "Only rules on these log types, such as AWS.CloudTrail."))...),
	restGet("panther_get_rule", "Get one Panther rule in full: Python body, description, runbook, severity, log types, dedup period, threshold, tags, reports, and unit tests. Use after panther_list_rules.", "/rules", "rule_id", "Rule id, such as AWS.CloudTrail.Stopped."),
	restList("panther_list_scheduled_rules", "List Panther scheduled rules (detections that run on scheduled SQL query results): id, name, severity, enabled state, scheduled queries, and tags.", "/scheduled-rules",
		detectionFilters(str("scheduled_query", "scheduled-query", "Only scheduled rules that use this scheduled query name."))...),
	restGet("panther_get_scheduled_rule", "Get a Panther scheduled rule in full: Python body, scheduled queries, severity, runbook, tags, and unit tests. Use after panther_list_scheduled_rules.", "/scheduled-rules", "rule_id", "Scheduled rule id."),
	restList("panther_list_simple_rules", "List Panther simple rules (YAML no-code detections): id, name, severity, enabled state, log types, and tags.", "/simple-rules",
		detectionFilters(strs("log_type", "log-type", "Only simple rules on these log types."))...),
	restGet("panther_get_simple_rule", "Get a Panther simple (YAML) rule in full: rule YAML, alert title and context, group by, severity, log types, and unit tests. Use after panther_list_simple_rules.", "/simple-rules", "rule_id", "Simple rule id."),
	restList("panther_list_policies", "List Panther cloud security policies (posture and compliance checks on cloud resources): id, name, severity, enabled state, resource types, and tags.", "/policies",
		detectionFilters(
			strs("resource_type", "resource-type", "Only policies on these resource types, such as AWS.S3.Bucket."),
			str("compliance_status", "compliance-status", "PASS, FAIL, or ERROR."),
		)...),
	restGet("panther_get_policy", "Get a Panther cloud security policy in full: Python body, resource types, severity, suppressions, runbook, and unit tests. Use after panther_list_policies.", "/policies", "policy_id", "Policy id."),
	restList("panther_list_correlation_rules", "List Panther correlation rules (multi-signal detections that join rule matches over time): id, name, severity, enabled state, and tags.", "/correlation-rules",
		detectionFilters()...),
	restGet("panther_get_correlation_rule", "Get a Panther correlation rule in full: rule YAML (sequence or group), severity, runbook, and tests. Use after panther_list_correlation_rules.", "/correlation-rules", "rule_id", "Correlation rule id."),
	{
		name:   "panther_set_detection_enabled",
		desc:   "Enable or disable (turn on, turn off) a Panther detection: rule, scheduled rule, simple rule, policy, or correlation rule. Reads the detection and writes it back with only enabled changed, skipping unit tests. Use after the matching list or get tool.",
		access: mcp.AccessWrite,
		params: []param{
			str("detection_type", "", "rule, scheduled_rule, simple_rule, policy, or correlation_rule.").req(),
			str("detection_id", "", "Detection id.").req(),
			boolean("enabled", "", "true to enable, false to disable.").req(),
		},
		run: setDetectionEnabled,
	},
}

func setDetectionEnabled(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	kind := strings.ToLower(strings.TrimSpace(r.Str("detection_type")))
	id := r.Str("detection_id")
	enabled := r.Bool("enabled")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if _, ok := args["enabled"]; !ok {
		return mcp.ErrResult(fmt.Errorf("enabled is required"))
	}
	collection, ok := detectionPaths[kind]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("detection_type must be one of rule, scheduled_rule, simple_rule, policy, correlation_rule"))
	}
	escaped, err := pathID("detection_id", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	path := collection + "/" + escaped
	current, err := c.rest(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	// The detection is decoded as a generic object so every field Panther
	// returned goes back unchanged; PUT replaces the whole detection.
	// UseNumber keeps large integers exact on the way back.
	decoder := json.NewDecoder(bytes.NewReader(current))
	decoder.UseNumber()
	var detection map[string]any
	if err := decoder.Decode(&detection); err != nil || detection == nil {
		return mcp.ErrResult(fmt.Errorf("panther: %s %s is not a JSON object", kind, id))
	}
	detection["enabled"] = enabled
	// Every detection PUT runs its unit tests first unless told not to;
	// toggling enabled must not fail on an unrelated failing test.
	updated, err := c.rest(ctx, http.MethodPut, path, url.Values{"run-tests-first": {"false"}}, detection)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(updated)
}
