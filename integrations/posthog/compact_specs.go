package posthog

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	// PostHog list endpoints return {"count": N, "results": [...], "next": "..."}.
	// Get endpoints return the object directly (no wrapper).

	// ── Projects ────────────────────────────────────────────────────
	// Handler: rawResult(API response) — list returns {"results": [...]}
	mcp.ToolName("posthog_list_projects"): {"results[].id", "results[].name", "results[].created_at", "results[].updated_at"},
	// Handler: rawResult(API response) — get returns flat object
	mcp.ToolName("posthog_get_project"): {"id", "name", "created_at", "updated_at", "timezone", "completed_snippet_onboarding"},

	// ── Feature Flags ───────────────────────────────────────────────
	mcp.ToolName("posthog_list_feature_flags"):    {"results[].id", "results[].key", "results[].name", "results[].active", "results[].created_at", "results[].created_by.email", "results[].rollout_percentage", "results[].filters"},
	mcp.ToolName("posthog_get_feature_flag"):      {"id", "key", "name", "active", "created_at", "created_by.email", "rollout_percentage", "filters", "ensure_experience_continuity"},
	mcp.ToolName("posthog_feature_flag_activity"): {"results[].id", "results[].activity", "results[].created_at", "results[].user.email", "results[].detail"},

	// ── Cohorts ─────────────────────────────────────────────────────
	mcp.ToolName("posthog_list_cohorts"):        {"results[].id", "results[].name", "results[].count", "results[].is_static", "results[].created_at", "results[].created_by.email"},
	mcp.ToolName("posthog_get_cohort"):          {"id", "name", "description", "count", "is_static", "filters", "created_at", "created_by.email"},
	mcp.ToolName("posthog_list_cohort_persons"): {"results[].id", "results[].distinct_ids", "results[].properties", "results[].created_at"},

	// ── Insights ────────────────────────────────────────────────────
	// Note: result/hogql/is_cached/last_refresh/query_status expose the computed
	// query output. PostHog computes results lazily, so freshly-created insights
	// may have result: null until the first refresh (is_cached: true).
	mcp.ToolName("posthog_list_insights"): {"results[].id", "results[].name", "results[].short_id", "results[].filters", "results[].query", "results[].result", "results[].hogql", "results[].is_cached", "results[].last_refresh", "results[].query_status", "results[].created_at", "results[].created_by.email", "results[].last_modified_at"},
	mcp.ToolName("posthog_get_insight"):   {"id", "name", "short_id", "description", "filters", "query", "result", "hogql", "is_cached", "last_refresh", "query_status", "created_at", "created_by.email", "last_modified_at"},

	// ── Persons ─────────────────────────────────────────────────────
	mcp.ToolName("posthog_list_persons"): {"results[].id", "results[].distinct_ids", "results[].properties.email", "results[].properties.name", "results[].created_at"},
	mcp.ToolName("posthog_get_person"):   {"id", "distinct_ids", "properties", "created_at"},

	// ── Groups ──────────────────────────────────────────────────────
	mcp.ToolName("posthog_list_groups"): {"results[].group_type_index", "results[].group_key", "results[].group_properties", "results[].created_at"},
	// Handler: rawResult(API response) — find returns single object
	mcp.ToolName("posthog_find_group"): {"group_type_index", "group_key", "group_properties", "created_at"},

	// ── Annotations ─────────────────────────────────────────────────
	mcp.ToolName("posthog_list_annotations"): {"results[].id", "results[].content", "results[].date_marker", "results[].scope", "results[].created_at", "results[].created_by.email"},
	mcp.ToolName("posthog_get_annotation"):   {"id", "content", "date_marker", "scope", "created_at", "created_by.email"},

	// ── Dashboards ──────────────────────────────────────────────────
	mcp.ToolName("posthog_list_dashboards"): {"results[].id", "results[].name", "results[].description", "results[].pinned", "results[].created_at", "results[].created_by.email", "results[].tags"},
	mcp.ToolName("posthog_get_dashboard"):   {"id", "name", "description", "pinned", "tiles", "created_at", "created_by.email", "tags"},

	// ── Actions ─────────────────────────────────────────────────────
	mcp.ToolName("posthog_list_actions"): {"results[].id", "results[].name", "results[].description", "results[].tags", "results[].created_at", "results[].created_by.email"},
	mcp.ToolName("posthog_get_action"):   {"id", "name", "description", "steps", "tags", "created_at", "created_by.email"},

	// ── Events ──────────────────────────────────────────────────────
	mcp.ToolName("posthog_list_events"): {"results[].id", "results[].event", "results[].distinct_id", "results[].timestamp", "results[].properties"},
	mcp.ToolName("posthog_get_event"):   {"id", "event", "distinct_id", "timestamp", "properties"},

	// ── Experiments ─────────────────────────────────────────────────
	mcp.ToolName("posthog_list_experiments"): {"results[].id", "results[].name", "results[].start_date", "results[].end_date", "results[].feature_flag_key", "results[].created_at"},
	mcp.ToolName("posthog_get_experiment"):   {"id", "name", "description", "start_date", "end_date", "feature_flag_key", "filters", "created_at"},

	// ── Surveys ─────────────────────────────────────────────────────
	mcp.ToolName("posthog_list_surveys"): {"results[].id", "results[].name", "results[].type", "results[].created_at"},
	mcp.ToolName("posthog_get_survey"):   {"id", "name", "description", "type", "questions", "created_at", "targeting_flag_filters"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("posthog: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
