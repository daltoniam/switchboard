package posthog

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	// PostHog list endpoints return {"count": N, "results": [...], "next": "..."}.
	// Get endpoints return the object directly (no wrapper).

	// ── Projects ────────────────────────────────────────────────────
	// Handler: rawResult(API response) — list returns {"results": [...]}
	"posthog_list_projects": {"results[].id", "results[].name", "results[].created_at", "results[].updated_at"},
	// Handler: rawResult(API response) — get returns flat object
	"posthog_get_project": {"id", "name", "created_at", "updated_at", "timezone", "completed_snippet_onboarding"},

	// ── Feature Flags ───────────────────────────────────────────────
	"posthog_list_feature_flags":    {"results[].id", "results[].key", "results[].name", "results[].active", "results[].created_at", "results[].created_by.email", "results[].rollout_percentage", "results[].filters"},
	"posthog_get_feature_flag":      {"id", "key", "name", "active", "created_at", "created_by.email", "rollout_percentage", "filters", "ensure_experience_continuity"},
	"posthog_feature_flag_activity": {"results[].id", "results[].activity", "results[].created_at", "results[].user.email", "results[].detail"},

	// ── Cohorts ─────────────────────────────────────────────────────
	"posthog_list_cohorts":        {"results[].id", "results[].name", "results[].count", "results[].is_static", "results[].created_at", "results[].created_by.email"},
	"posthog_get_cohort":          {"id", "name", "description", "count", "is_static", "filters", "created_at", "created_by.email"},
	"posthog_list_cohort_persons": {"results[].id", "results[].distinct_ids", "results[].properties", "results[].created_at"},

	// ── Insights ────────────────────────────────────────────────────
	"posthog_list_insights": {"results[].id", "results[].name", "results[].short_id", "results[].filters", "results[].created_at", "results[].created_by.email", "results[].last_modified_at"},
	"posthog_get_insight":   {"id", "name", "short_id", "description", "filters", "query", "result", "created_at", "created_by.email", "last_modified_at"},

	// ── Persons ─────────────────────────────────────────────────────
	"posthog_list_persons": {"results[].id", "results[].distinct_ids", "results[].properties.email", "results[].properties.name", "results[].created_at"},
	"posthog_get_person":   {"id", "distinct_ids", "properties", "created_at"},

	// ── Groups ──────────────────────────────────────────────────────
	"posthog_list_groups": {"results[].group_type_index", "results[].group_key", "results[].group_properties", "results[].created_at"},
	// Handler: rawResult(API response) — find returns single object
	"posthog_find_group": {"group_type_index", "group_key", "group_properties", "created_at"},

	// ── Annotations ─────────────────────────────────────────────────
	"posthog_list_annotations": {"results[].id", "results[].content", "results[].date_marker", "results[].scope", "results[].created_at", "results[].created_by.email"},
	"posthog_get_annotation":   {"id", "content", "date_marker", "scope", "created_at", "created_by.email"},

	// ── Dashboards ──────────────────────────────────────────────────
	"posthog_list_dashboards": {"results[].id", "results[].name", "results[].description", "results[].pinned", "results[].created_at", "results[].created_by.email", "results[].tags"},
	"posthog_get_dashboard":   {"id", "name", "description", "pinned", "tiles", "created_at", "created_by.email", "tags"},

	// ── Actions ─────────────────────────────────────────────────────
	"posthog_list_actions": {"results[].id", "results[].name", "results[].description", "results[].tags", "results[].created_at", "results[].created_by.email"},
	"posthog_get_action":   {"id", "name", "description", "steps", "tags", "created_at", "created_by.email"},

	// ── Events ──────────────────────────────────────────────────────
	"posthog_list_events": {"results[].id", "results[].event", "results[].distinct_id", "results[].timestamp", "results[].properties"},
	"posthog_get_event":   {"id", "event", "distinct_id", "timestamp", "properties"},

	// ── Experiments ─────────────────────────────────────────────────
	"posthog_list_experiments": {"results[].id", "results[].name", "results[].start_date", "results[].end_date", "results[].feature_flag_key", "results[].created_at"},
	"posthog_get_experiment":   {"id", "name", "description", "start_date", "end_date", "feature_flag_key", "filters", "created_at"},

	// ── Surveys ─────────────────────────────────────────────────────
	"posthog_list_surveys": {"results[].id", "results[].name", "results[].type", "results[].created_at"},
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
