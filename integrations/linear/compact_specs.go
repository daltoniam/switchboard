package linear

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	// ── Issues ────────────────────────────────────────────────────────
	"linear_list_issues":          {"id", "identifier", "title", "state.name", "state.type", "priority", "priorityLabel", "assignee.name", "labels[].name", "createdAt", "updatedAt", "dueDate", "estimate", "project.name", "cycle.name"},
	"linear_search_issues":        {"id", "identifier", "title", "state.name", "state.type", "priority", "priorityLabel", "assignee.name", "labels[].name", "createdAt", "updatedAt"},
	"linear_get_issue":            {"id", "identifier", "title", "description", "state.name", "state.type", "priority", "priorityLabel", "assignee.name", "assignee.email", "labels[].name", "createdAt", "updatedAt", "dueDate", "estimate", "project.name", "cycle.name", "parent.identifier", "url"},
	"linear_list_issue_comments":  {"id", "body", "user.name", "createdAt", "updatedAt"},
	"linear_list_issue_relations": {"id", "type", "issue.identifier", "issue.title", "relatedIssue.identifier", "relatedIssue.title"},
	"linear_list_issue_labels":    {"id", "name", "color"},
	"linear_list_attachments":     {"id", "title", "url", "subtitle", "createdAt"},

	// ── Projects ──────────────────────────────────────────────────────
	"linear_list_projects":          {"id", "name", "slugId", "state", "progress", "lead.name", "startDate", "targetDate", "createdAt", "updatedAt"},
	"linear_search_projects":        {"id", "name", "slugId", "state", "progress", "lead.name", "startDate", "targetDate"},
	"linear_get_project":            {"id", "name", "slugId", "description", "state", "progress", "lead.name", "lead.email", "startDate", "targetDate", "createdAt", "updatedAt", "url"},
	"linear_list_project_updates":   {"id", "body", "health", "user.name", "createdAt"},
	"linear_list_project_milestones": {"id", "name", "description", "targetDate", "sortOrder"},

	// ── Cycles ────────────────────────────────────────────────────────
	"linear_list_cycles": {"id", "name", "number", "startsAt", "endsAt", "progress", "completedScopeCount", "scopeCount"},
	"linear_get_cycle":   {"id", "name", "number", "description", "startsAt", "endsAt", "progress", "completedScopeCount", "scopeCount"},

	// ── Teams ─────────────────────────────────────────────────────────
	"linear_list_teams": {"id", "name", "key", "description", "issueCount"},
	"linear_get_team":   {"id", "name", "key", "description", "issueCount", "timezone", "cyclesEnabled", "triageEnabled"},

	// ── Users ─────────────────────────────────────────────────────────
	"linear_viewer":     {"id", "name", "email", "displayName", "admin", "active"},
	"linear_list_users": {"id", "name", "email", "displayName", "admin", "active"},
	"linear_get_user":   {"id", "name", "email", "displayName", "admin", "active", "createdAt", "statusLabel", "statusEmoji"},

	// ── Labels ────────────────────────────────────────────────────────
	"linear_list_labels": {"id", "name", "color", "description", "parent.name"},

	// ── Workflow States ───────────────────────────────────────────────
	"linear_list_workflow_states": {"id", "name", "type", "color", "position"},

	// ── Documents ─────────────────────────────────────────────────────
	"linear_list_documents":   {"id", "title", "icon", "project.name", "creator.name", "createdAt", "updatedAt"},
	"linear_search_documents": {"id", "title", "icon", "project.name", "creator.name", "createdAt", "updatedAt"},
	"linear_get_document":     {"id", "title", "icon", "content", "project.name", "creator.name", "createdAt", "updatedAt"},

	// ── Initiatives ───────────────────────────────────────────────────
	"linear_list_initiatives": {"id", "name", "status", "targetDate", "createdAt"},
	"linear_get_initiative":   {"id", "name", "description", "status", "targetDate", "createdAt", "updatedAt"},

	// ── Misc ──────────────────────────────────────────────────────────
	"linear_list_favorites":     {"id", "type", "issue.identifier", "project.name", "cycle.name", "customView.name"},
	"linear_list_webhooks":      {"id", "url", "enabled", "resourceTypes", "createdAt"},
	"linear_list_notifications": {"id", "type", "readAt", "createdAt", "issue.identifier", "issue.title"},
	"linear_list_templates":     {"id", "name", "type", "description"},
	"linear_get_organization":   {"id", "name", "urlKey", "createdAt", "userCount"},
	"linear_list_custom_views":  {"id", "name", "description", "filters", "createdAt"},
	"linear_rate_limit":         {"cost", "remaining", "resetAt"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("linear: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
