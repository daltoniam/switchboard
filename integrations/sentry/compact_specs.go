package sentry

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	// ── Organizations ────────────────────────────────────────────────
	"sentry_get_organization": {"id", "slug", "name", "status.id", "dateCreated", "features"},
	"sentry_list_org_projects": {"id", "slug", "name", "platform", "dateCreated", "status"},
	"sentry_list_org_teams":    {"id", "slug", "name", "memberCount", "dateCreated"},
	"sentry_list_org_members":  {"id", "email", "name", "role", "pending", "dateCreated"},
	"sentry_get_org_member":    {"id", "email", "name", "role", "pending", "dateCreated", "teams"},
	"sentry_list_org_repos":    {"id", "name", "provider.id", "status", "dateCreated", "url"},
	"sentry_resolve_short_id":  {"group.id", "group.title", "group.status", "group.level", "group.count", "shortId"},

	// ── Projects ─────────────────────────────────────────────────────
	"sentry_list_projects":      {"id", "slug", "name", "platform", "dateCreated", "status"},
	"sentry_get_project":        {"id", "slug", "name", "platform", "dateCreated", "status", "features", "team.slug"},
	"sentry_list_project_keys":  {"id", "name", "label", "dsn.public", "dsn.secret", "dateCreated", "isActive"},
	"sentry_list_project_envs":  {"id", "name"},
	"sentry_list_project_tags":  {"key", "name", "totalValues"},
	"sentry_list_project_hooks": {"id", "url", "status", "events", "dateCreated"},

	// ── Teams ────────────────────────────────────────────────────────
	"sentry_get_team":           {"id", "slug", "name", "memberCount", "dateCreated"},
	"sentry_list_team_projects": {"id", "slug", "name", "platform", "dateCreated"},

	// ── Issues & Events ──────────────────────────────────────────────
	"sentry_list_issues":          {"id", "shortId", "title", "level", "status", "count", "userCount", "firstSeen", "lastSeen", "assignedTo.name", "project.slug"},
	"sentry_get_issue":            {"id", "shortId", "title", "level", "status", "count", "userCount", "firstSeen", "lastSeen", "assignedTo.name", "project.slug", "metadata", "type"},
	"sentry_list_issue_events":    {"eventID", "title", "message", "dateCreated", "tags"},
	"sentry_list_issue_hashes":    {"id", "latestEvent.eventID"},
	"sentry_get_issue_tag_values": {"key", "name", "value", "count", "lastSeen", "firstSeen"},
	"sentry_list_project_events":  {"eventID", "title", "message", "dateCreated", "tags"},
	"sentry_get_event":            {"eventID", "title", "message", "dateCreated", "tags", "entries", "context", "contexts", "user"},
	"sentry_list_org_issues":      {"id", "shortId", "title", "level", "status", "count", "userCount", "firstSeen", "lastSeen", "project.slug"},

	// ── Releases ─────────────────────────────────────────────────────
	"sentry_list_releases":        {"version", "shortVersion", "dateCreated", "dateReleased", "newGroups", "projects[].slug"},
	"sentry_get_release":          {"version", "shortVersion", "dateCreated", "dateReleased", "newGroups", "projects[].slug", "firstEvent", "lastEvent", "authors"},
	"sentry_list_release_commits": {"id", "message", "author.name", "dateCreated"},
	"sentry_list_release_deploys": {"id", "environment", "name", "dateStarted", "dateFinished"},
	"sentry_list_release_files":   {"id", "name", "size", "sha1", "dateCreated"},

	// ── Alerts ───────────────────────────────────────────────────────
	"sentry_list_metric_alerts": {"id", "name", "status", "dataset", "query", "aggregate", "dateCreated"},
	"sentry_get_metric_alert":   {"id", "name", "status", "dataset", "query", "aggregate", "triggers", "dateCreated"},
	"sentry_list_issue_alerts":  {"id", "name", "status", "conditions", "actions", "dateCreated"},
	"sentry_get_issue_alert":    {"id", "name", "status", "conditions", "actions", "filters", "dateCreated"},

	// ── Monitors (Cron) ──────────────────────────────────────────────
	"sentry_list_monitors": {"id", "slug", "name", "status", "type", "schedule", "dateCreated", "project.slug"},
	"sentry_get_monitor":   {"id", "slug", "name", "status", "type", "schedule", "config", "dateCreated", "project.slug"},

	// ── Discover ─────────────────────────────────────────────────────
	"sentry_list_saved_queries": {"id", "name", "query", "fields", "dateCreated", "dateUpdated", "createdBy"},
	"sentry_get_saved_query":    {"id", "name", "query", "fields", "dateCreated", "dateUpdated", "createdBy"},

	// ── Replays ──────────────────────────────────────────────────────
	"sentry_list_replays": {"id", "title", "duration", "countErrors", "startedAt", "finishedAt", "urls"},
	"sentry_get_replay":   {"id", "title", "duration", "countErrors", "startedAt", "finishedAt", "urls", "tags", "user"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("sentry: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
