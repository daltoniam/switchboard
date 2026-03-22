package jira

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	// ── Issues ───────────────────────────────────────────────────────
	"jira_search_issues": {
		"issues[].key", "issues[].fields.summary", "issues[].fields.status.name",
		"issues[].fields.assignee.displayName", "issues[].fields.priority.name",
		"issues[].fields.issuetype.name", "issues[].fields.created", "issues[].fields.updated",
	},
	"jira_get_issue": {
		"key", "fields.summary", "fields.status.name", "fields.assignee.displayName",
		"fields.assignee.accountId", "fields.reporter.displayName", "fields.priority.name",
		"fields.issuetype.name", "fields.description", "fields.created", "fields.updated",
		"fields.labels", "fields.components[].name", "fields.fixVersions[].name",
	},
	"jira_get_transitions": {"transitions[].id", "transitions[].name", "transitions[].to.name"},
	"jira_list_comments": {
		"comments[].id", "comments[].body", "comments[].author.displayName",
		"comments[].created", "comments[].updated",
	},
	"jira_list_issue_links": {
		"id", "type.name", "type.inward", "type.outward",
		"inwardIssue.key", "inwardIssue.fields.summary",
		"outwardIssue.key", "outwardIssue.fields.summary",
	},

	// ── Projects ─────────────────────────────────────────────────────
	"jira_list_projects":           {"values[].key", "values[].name", "values[].projectTypeKey", "values[].style"},
	"jira_get_project":             {"key", "name", "projectTypeKey", "description", "lead.displayName", "components", "versions"},
	"jira_list_project_components": {"id", "name", "description", "lead.displayName", "assigneeType"},
	"jira_list_project_versions":   {"id", "name", "description", "released", "releaseDate", "archived"},
	"jira_list_project_statuses":   {"id", "name", "statuses[].id", "statuses[].name"},

	// ── Boards & Sprints (Agile API) ────────────────────────────────
	"jira_list_boards":  {"values[].id", "values[].name", "values[].type", "values[].location.projectKey"},
	"jira_get_board":    {"id", "name", "type", "location.projectKey"},
	"jira_list_sprints": {"values[].id", "values[].name", "values[].state", "values[].startDate", "values[].endDate", "values[].goal"},
	"jira_get_sprint":   {"id", "name", "state", "startDate", "endDate", "goal", "originBoardId"},
	"jira_get_sprint_issues": {
		"issues[].key", "issues[].fields.summary", "issues[].fields.status.name",
		"issues[].fields.assignee.displayName", "issues[].fields.priority.name",
	},
	"jira_list_board_backlog": {
		"issues[].key", "issues[].fields.summary", "issues[].fields.status.name",
		"issues[].fields.assignee.displayName", "issues[].fields.priority.name",
	},
	"jira_get_board_config": {"id", "name", "columnConfig.columns[].name", "columnConfig.columns[].statuses[].id"},

	// ── Users ────────────────────────────────────────────────────────
	"jira_get_myself":   {"accountId", "displayName", "emailAddress", "active", "timeZone"},
	"jira_search_users": {"accountId", "displayName", "emailAddress", "active"},
	"jira_get_user":     {"accountId", "displayName", "emailAddress", "active", "timeZone"},

	// ── Metadata ─────────────────────────────────────────────────────
	"jira_list_issue_types": {"id", "name", "subtask", "description"},
	"jira_list_priorities":  {"id", "name", "description"},
	"jira_list_statuses":    {"id", "name", "statusCategory.name"},
	"jira_list_labels":      {"values"},
	"jira_list_fields":      {"id", "name", "custom", "schema.type"},
	"jira_list_filters":     {"values[].id", "values[].name", "values[].jql", "values[].owner.displayName"},
	"jira_get_filter":       {"id", "name", "jql", "owner.displayName", "description"},

	// ── Worklogs & Info ──────────────────────────────────────────────
	"jira_list_worklogs":   {"worklogs[].id", "worklogs[].author.displayName", "worklogs[].timeSpent", "worklogs[].started", "worklogs[].comment"},
	"jira_get_server_info": {"baseUrl", "version", "deploymentType", "serverTitle"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("jira: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
