package sentry

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	// ── Organizations ────────────────────────────────────────────────
	mcp.ToolName("sentry_get_organization"):  {"id", "slug", "name", "status.id", "dateCreated", "features"},
	mcp.ToolName("sentry_list_org_projects"): {"id", "slug", "name", "platform", "dateCreated", "status"},
	mcp.ToolName("sentry_list_org_teams"):    {"id", "slug", "name", "memberCount", "dateCreated"},
	mcp.ToolName("sentry_list_org_members"):  {"id", "email", "name", "role", "pending", "dateCreated"},
	mcp.ToolName("sentry_get_org_member"):    {"id", "email", "name", "role", "pending", "dateCreated", "teams"},
	mcp.ToolName("sentry_list_org_repos"):    {"id", "name", "provider.id", "status", "dateCreated", "url"},
	mcp.ToolName("sentry_resolve_short_id"):  {"group.id", "group.title", "group.status", "group.level", "group.count", "shortId"},

	// ── Projects ─────────────────────────────────────────────────────
	mcp.ToolName("sentry_list_projects"):      {"id", "slug", "name", "platform", "dateCreated", "status"},
	mcp.ToolName("sentry_get_project"):        {"id", "slug", "name", "platform", "dateCreated", "status", "features", "team.slug"},
	mcp.ToolName("sentry_list_project_keys"):  {"id", "name", "label", "dsn.public", "dsn.secret", "dateCreated", "isActive"},
	mcp.ToolName("sentry_list_project_envs"):  {"id", "name"},
	mcp.ToolName("sentry_list_project_tags"):  {"key", "name", "totalValues"},
	mcp.ToolName("sentry_list_project_hooks"): {"id", "url", "status", "events", "dateCreated"},

	// ── Teams ────────────────────────────────────────────────────────
	mcp.ToolName("sentry_get_team"):           {"id", "slug", "name", "memberCount", "dateCreated"},
	mcp.ToolName("sentry_list_team_projects"): {"id", "slug", "name", "platform", "dateCreated"},

	// ── Issues & Events ──────────────────────────────────────────────
	mcp.ToolName("sentry_list_issues"):          {"id", "shortId", "title", "level", "status", "count", "userCount", "firstSeen", "lastSeen", "assignedTo.name", "project.slug"},
	mcp.ToolName("sentry_get_issue"):            {"id", "shortId", "title", "level", "status", "count", "userCount", "firstSeen", "lastSeen", "assignedTo.name", "project.slug", "metadata", "type"},
	mcp.ToolName("sentry_list_issue_events"):    {"eventID", "title", "message", "dateCreated", "tags"},
	mcp.ToolName("sentry_list_issue_hashes"):    {"id", "latestEvent.eventID"},
	mcp.ToolName("sentry_get_issue_tag_values"): {"key", "name", "value", "count", "lastSeen", "firstSeen"},
	mcp.ToolName("sentry_list_project_events"):  {"eventID", "title", "message", "dateCreated", "tags"},
	mcp.ToolName("sentry_get_event"):            {"eventID", "title", "message", "dateCreated", "tags", "entries", "context", "contexts", "user"},
	mcp.ToolName("sentry_list_org_issues"):      {"id", "shortId", "title", "level", "status", "count", "userCount", "firstSeen", "lastSeen", "project.slug"},

	// ── Releases ─────────────────────────────────────────────────────
	mcp.ToolName("sentry_list_releases"):        {"version", "shortVersion", "dateCreated", "dateReleased", "newGroups", "projects[].slug"},
	mcp.ToolName("sentry_get_release"):          {"version", "shortVersion", "dateCreated", "dateReleased", "newGroups", "projects[].slug", "firstEvent", "lastEvent", "authors"},
	mcp.ToolName("sentry_list_release_commits"): {"id", "message", "author.name", "dateCreated"},
	mcp.ToolName("sentry_list_release_deploys"): {"id", "environment", "name", "dateStarted", "dateFinished"},
	mcp.ToolName("sentry_list_release_files"):   {"id", "name", "size", "sha1", "dateCreated"},

	// ── Alerts ───────────────────────────────────────────────────────
	mcp.ToolName("sentry_list_metric_alerts"): {"id", "name", "status", "dataset", "query", "aggregate", "dateCreated"},
	mcp.ToolName("sentry_get_metric_alert"):   {"id", "name", "status", "dataset", "query", "aggregate", "triggers", "dateCreated"},
	mcp.ToolName("sentry_list_issue_alerts"):  {"id", "name", "status", "conditions", "actions", "dateCreated"},
	mcp.ToolName("sentry_get_issue_alert"):    {"id", "name", "status", "conditions", "actions", "filters", "dateCreated"},

	// ── Monitors (Cron) ──────────────────────────────────────────────
	mcp.ToolName("sentry_list_monitors"): {"id", "slug", "name", "status", "type", "schedule", "dateCreated", "project.slug"},
	mcp.ToolName("sentry_get_monitor"):   {"id", "slug", "name", "status", "type", "schedule", "config", "dateCreated", "project.slug"},

	// ── Discover ─────────────────────────────────────────────────────
	mcp.ToolName("sentry_list_saved_queries"): {"id", "name", "query", "fields", "dateCreated", "dateUpdated", "createdBy"},
	mcp.ToolName("sentry_get_saved_query"):    {"id", "name", "query", "fields", "dateCreated", "dateUpdated", "createdBy"},

	// ── Replays ──────────────────────────────────────────────────────
	mcp.ToolName("sentry_list_replays"): {"id", "title", "duration", "countErrors", "startedAt", "finishedAt", "urls"},
	mcp.ToolName("sentry_get_replay"):   {"id", "title", "duration", "countErrors", "startedAt", "finishedAt", "urls", "tags", "user"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("sentry: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
