package posthog

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	// ── Projects ────────────────────────────────────────────────────
	"posthog_list_projects": {"id", "name", "created_at", "updated_at"},
	"posthog_get_project":   {"id", "name", "created_at", "updated_at", "timezone", "completed_snippet_onboarding"},

	// ── Feature Flags ───────────────────────────────────────────────
	"posthog_list_feature_flags":  {"id", "key", "name", "active", "created_at", "created_by.email", "rollout_percentage", "filters"},
	"posthog_get_feature_flag":    {"id", "key", "name", "active", "created_at", "created_by.email", "rollout_percentage", "filters", "ensure_experience_continuity"},
	"posthog_feature_flag_activity": {"id", "activity", "created_at", "user.email", "detail"},

	// ── Cohorts ─────────────────────────────────────────────────────
	"posthog_list_cohorts":       {"id", "name", "count", "is_static", "created_at", "created_by.email"},
	"posthog_get_cohort":         {"id", "name", "description", "count", "is_static", "filters", "created_at", "created_by.email"},
	"posthog_list_cohort_persons": {"id", "distinct_ids", "properties", "created_at"},

	// ── Insights ────────────────────────────────────────────────────
	"posthog_list_insights": {"id", "name", "short_id", "filters", "created_at", "created_by.email", "last_modified_at"},
	"posthog_get_insight":   {"id", "name", "short_id", "description", "filters", "query", "result", "created_at", "created_by.email", "last_modified_at"},

	// ── Persons ─────────────────────────────────────────────────────
	"posthog_list_persons": {"id", "distinct_ids", "properties.email", "properties.name", "created_at"},
	"posthog_get_person":   {"id", "distinct_ids", "properties", "created_at"},

	// ── Groups ──────────────────────────────────────────────────────
	"posthog_list_groups": {"group_type_index", "group_key", "group_properties", "created_at"},
	"posthog_find_group":  {"group_type_index", "group_key", "group_properties", "created_at"},

	// ── Annotations ─────────────────────────────────────────────────
	"posthog_list_annotations": {"id", "content", "date_marker", "scope", "created_at", "created_by.email"},
	"posthog_get_annotation":   {"id", "content", "date_marker", "scope", "created_at", "created_by.email"},

	// ── Dashboards ──────────────────────────────────────────────────
	"posthog_list_dashboards": {"id", "name", "description", "pinned", "created_at", "created_by.email", "tags"},
	"posthog_get_dashboard":   {"id", "name", "description", "pinned", "tiles", "created_at", "created_by.email", "tags"},

	// ── Actions ─────────────────────────────────────────────────────
	"posthog_list_actions": {"id", "name", "description", "tags", "created_at", "created_by.email"},
	"posthog_get_action":   {"id", "name", "description", "steps", "tags", "created_at", "created_by.email"},

	// ── Events ──────────────────────────────────────────────────────
	"posthog_list_events": {"id", "event", "distinct_id", "timestamp", "properties"},
	"posthog_get_event":   {"id", "event", "distinct_id", "timestamp", "properties"},

	// ── Experiments ─────────────────────────────────────────────────
	"posthog_list_experiments": {"id", "name", "start_date", "end_date", "feature_flag_key", "created_at"},
	"posthog_get_experiment":   {"id", "name", "description", "start_date", "end_date", "feature_flag_key", "filters", "created_at"},

	// ── Surveys ─────────────────────────────────────────────────────
	"posthog_list_surveys": {"id", "name", "type", "created_at"},
	"posthog_get_survey":   {"id", "name", "description", "type", "questions", "created_at", "targeting_flag_filters"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("posthog: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
