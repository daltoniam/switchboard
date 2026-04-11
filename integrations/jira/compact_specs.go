package jira

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	// ── Issues ───────────────────────────────────────────────────────
	mcp.ToolName("jira_search_issues"): {
		"issues[].key", "issues[].fields.summary", "issues[].fields.status.name",
		"issues[].fields.assignee.displayName", "issues[].fields.priority.name",
		"issues[].fields.issuetype.name", "issues[].fields.created", "issues[].fields.updated",
	},
	mcp.ToolName("jira_get_issue"): {
		"key", "fields.summary", "fields.status.name", "fields.assignee.displayName",
		"fields.assignee.accountId", "fields.reporter.displayName", "fields.priority.name",
		"fields.issuetype.name", "fields.description", "fields.created", "fields.updated",
		"fields.labels", "fields.components[].name", "fields.fixVersions[].name",
	},
	mcp.ToolName("jira_get_transitions"): {"transitions[].id", "transitions[].name", "transitions[].to.name"},
	mcp.ToolName("jira_list_comments"): {
		"comments[].id", "comments[].body", "comments[].author.displayName",
		"comments[].created", "comments[].updated",
	},
	mcp.ToolName("jira_list_issue_links"): {
		"id", "type.name", "type.inward", "type.outward",
		"inwardIssue.key", "inwardIssue.fields.summary",
		"outwardIssue.key", "outwardIssue.fields.summary",
	},

	// ── Projects ─────────────────────────────────────────────────────
	mcp.ToolName("jira_list_projects"):           {"values[].key", "values[].name", "values[].projectTypeKey", "values[].style"},
	mcp.ToolName("jira_get_project"):             {"key", "name", "projectTypeKey", "description", "lead.displayName", "components", "versions"},
	mcp.ToolName("jira_list_project_components"): {"id", "name", "description", "lead.displayName", "assigneeType"},
	mcp.ToolName("jira_list_project_versions"):   {"id", "name", "description", "released", "releaseDate", "archived"},
	mcp.ToolName("jira_list_project_statuses"):   {"id", "name", "statuses[].id", "statuses[].name"},

	// ── Boards & Sprints (Agile API) ────────────────────────────────
	mcp.ToolName("jira_list_boards"):  {"values[].id", "values[].name", "values[].type", "values[].location.projectKey"},
	mcp.ToolName("jira_get_board"):    {"id", "name", "type", "location.projectKey"},
	mcp.ToolName("jira_list_sprints"): {"values[].id", "values[].name", "values[].state", "values[].startDate", "values[].endDate", "values[].goal"},
	mcp.ToolName("jira_get_sprint"):   {"id", "name", "state", "startDate", "endDate", "goal", "originBoardId"},
	mcp.ToolName("jira_get_sprint_issues"): {
		"issues[].key", "issues[].fields.summary", "issues[].fields.status.name",
		"issues[].fields.assignee.displayName", "issues[].fields.priority.name",
	},
	mcp.ToolName("jira_list_board_backlog"): {
		"issues[].key", "issues[].fields.summary", "issues[].fields.status.name",
		"issues[].fields.assignee.displayName", "issues[].fields.priority.name",
	},
	mcp.ToolName("jira_get_board_config"): {"id", "name", "columnConfig.columns[].name", "columnConfig.columns[].statuses[].id"},

	// ── Users ────────────────────────────────────────────────────────
	mcp.ToolName("jira_get_myself"):   {"accountId", "displayName", "emailAddress", "active", "timeZone"},
	mcp.ToolName("jira_search_users"): {"accountId", "displayName", "emailAddress", "active"},
	mcp.ToolName("jira_get_user"):     {"accountId", "displayName", "emailAddress", "active", "timeZone"},

	// ── Metadata ─────────────────────────────────────────────────────
	mcp.ToolName("jira_list_issue_types"): {"id", "name", "subtask", "description"},
	mcp.ToolName("jira_list_priorities"):  {"id", "name", "description"},
	mcp.ToolName("jira_list_statuses"):    {"id", "name", "statusCategory.name"},
	mcp.ToolName("jira_list_labels"):      {"values"},
	mcp.ToolName("jira_list_fields"):      {"id", "name", "custom", "schema.type"},
	mcp.ToolName("jira_list_filters"):     {"values[].id", "values[].name", "values[].jql", "values[].owner.displayName"},
	mcp.ToolName("jira_get_filter"):       {"id", "name", "jql", "owner.displayName", "description"},

	// ── Worklogs & Info ──────────────────────────────────────────────
	mcp.ToolName("jira_list_worklogs"):   {"worklogs[].id", "worklogs[].author.displayName", "worklogs[].timeSpent", "worklogs[].started", "worklogs[].comment"},
	mcp.ToolName("jira_get_server_info"): {"baseUrl", "version", "deploymentType", "serverTitle"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("jira: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
