package aikido

import (
	"net/http"
	"net/url"
	"slices"

	mcp "github.com/daltoniam/switchboard"
)

var issueTypes = []string{"open_source", "leaked_secret", "cloud", "sast", "iac", "docker_container", "cloud_instance", "surface_monitoring", "malware", "eol", "mobile", "scm_security", "ai_pentest", "license"}

var severities = []string{"critical", "high", "medium", "low"}

func issueID() param { return integer("issue_id", "Aikido issue id (id from aikido_list_issues).") }
func issueGroupID() param {
	return integer("issue_group_id", "Aikido issue group id (id from aikido_list_issue_groups).")
}

func applyAllTags(action string) param {
	return boolean("apply_for_all_tags", "For container issues, "+action+" across every tag of the container (Aikido support must enable this).")
}

var issueEndpoints = []endpoint{
	{
		tool: "aikido_list_issue_groups", method: http.MethodGet, path: "/open-issue-groups", scope: "issues:read", access: mcp.AccessRead,
		desc: "List Aikido security issue groups (the prioritized feed of open security issues): grouped vulnerabilities, CVEs, SAST code findings, leaked secrets, dependency (open source) scanning, cloud posture misconfigurations, container, IaC, malware and license issues with severity, status, affected repositories/containers/clouds and how to fix. Start here for Aikido code security, finding open security findings, and triaging what to fix first. Sorted by priority.",
		params: ps(
			integer("filter_code_repo_id", "Only issues in this Aikido code repository id."),
			str("filter_external_code_repo_id", "Only issues in this repository, by the Git provider's repository id."),
			str("filter_code_repo_name", "Only issues in the code repository with this name."),
			integer("filter_container_repo_id", "Only issues in this container repository id."),
			integer("filter_team_id", "Only issues owned by this team id."),
			str("filter_issue_type", "Only this issue type.").oneOf(issueTypes...),
			str("filter_status", "Group status (default open).").oneOf("open", "closed", "snoozed", "ignored"),
		),
		page: pages(10, 100, 20),
	},
	{
		tool: "aikido_get_issue_group", method: http.MethodGet, path: "/issues/groups/{issue_group_id}", scope: "issues:read", access: mcp.AccessRead,
		desc:   "Get one Aikido issue group (grouped security finding or vulnerability): title, description, severity score, status, every affected location (repository, container, cloud, domain), related CVEs, time to fix and how to fix. Use after aikido_list_issue_groups.",
		params: ps(issueGroupID()),
	},
	{
		tool: "aikido_list_issues", method: http.MethodGet, path: "/issues/export", scope: "issues:read", access: mcp.AccessRead,
		desc: "List individual Aikido security issues (findings) across all statuses with filters: vulnerabilities by CVE id, severity, type (SAST, secrets, open source dependency, cloud, container, IaC, malware, license, pentest), repository, container, domain, virtual machine, team, language and SLA due date. Each issue has its affected package, installed and patched versions, file and lines, CWE classes and status. Use for detailed vulnerability and CVE searches; aikido_list_issue_groups is the prioritized summary.",
		params: ps(
			str("filter_status", "Issue status (default all).").oneOf("all", "open", "ignored", "snoozed", "closed"),
			integer("filter_team_id", "Only issues owned by this team id."),
			integer("filter_issue_group_id", "Only issues in this issue group id."),
			integer("filter_code_repo_id", "Only issues in this code repository id."),
			str("filter_code_repo_name", "Only issues in the code repository with exactly this name."),
			integer("filter_container_repo_id", "Only issues in this container repository id."),
			str("filter_container_repo_name", "Only issues in the container repository with exactly this name."),
			integer("filter_domain_id", "Only issues on this domain id."),
			integer("filter_virtual_machine_id", "Only issues on this virtual machine id."),
			str("filter_issue_type", "Only this issue type.").oneOf(slices.Concat(issueTypes, []string{"agentic_review", "app_level_open_source"})...),
			str("filter_assessment_id", "Only issues from this pentest or AI code analysis assessment id."),
			str("filter_cve_id", "Only open source issues for exactly this CVE id, for example CVE-2024-3094."),
			strs("filter_severities", "Only these severities (list or comma-separated).").oneOf(severities...),
			integer("sla_remediate_by_after", "Only issues whose SLA due date is after this Unix timestamp (seconds)."),
			integer("sla_remediate_by_before", "Only issues whose SLA due date is before this Unix timestamp (seconds); use for overdue SLA breaches."),
			str("filter_language", "Only issues in this language.").oneOf("JS", "TS", "PHP", "Java", "Scala", "GO", "PY", "Ruby", ".NET", "RUST", "Dart", "Swift", "Elixir", "C", "Clojure", "kotlin", "APEX"),
		),
		page:  pages(10, 500, 50),
		fixed: url.Values{"format": {"json"}},
	},
	{
		tool: "aikido_get_issue", method: http.MethodGet, path: "/issues/{issue_id}", scope: "issues:read", access: mcp.AccessRead,
		desc: "Get one Aikido security issue (finding): type, rule, severity, status, CVE and alias, CWE classes, affected package/file, location (repository, container, cloud, domain, VM), reachability status, snooze and ignore reasons, SLA due date and how to fix. Use after aikido_list_issues or aikido_get_issue_group.",
		params: ps(
			issueID(),
			boolean("include_epss_score", "Include the EPSS exploit probability score (Pro and Scale plans)."),
		),
	},
	{
		tool: "aikido_get_issue_counts", method: http.MethodGet, path: "/issues/counts", scope: "issues:read", access: mcp.AccessRead,
		desc: "Count open Aikido security issues and issue groups by severity (critical, high, medium, low), optionally for one repository, container or team. Use for a quick security posture overview or vulnerability totals.",
		params: ps(
			integer("filter_code_repo_id", "Only this code repository id."),
			str("filter_external_code_repo_id", "Only this repository, by the Git provider's repository id."),
			str("filter_code_repo_name", "Only the code repository with this name."),
			integer("filter_container_repo_id", "Only this container repository id."),
			integer("filter_team_id", "Only this team id."),
			integer("since_timestamp", "Only issues created after this Unix timestamp (seconds)."),
		),
	},
	{
		tool: "aikido_get_issue_reachability", method: http.MethodGet, path: "/issues/{issue_id}/reachability", scope: "issues:read", access: mcp.AccessRead,
		desc: "Get reachability of a vulnerable dependency for one Aikido issue: the call paths showing which files and lines reach the vulnerable package. Use after aikido_get_issue to decide whether a CVE is actually exploitable.",
		params: ps(
			issueID(),
			boolean("include_dev_deps", "Include paths through development dependencies."),
		),
	},
	{
		tool: "aikido_get_issue_cve_exploitability", method: http.MethodGet, path: "/issues/{issue_id}/cveExploitability", scope: "issues:read", access: mcp.AccessRead,
		desc:   "Get Aikido's AI CVE exploitability analysis for one issue: proposed action (ignore or adjust severity), whether it was executed, reasoning and severity change. Use after aikido_get_issue when triaging a CVE.",
		params: ps(issueID()),
	},
	{
		tool: "aikido_list_issue_group_notes", method: http.MethodGet, path: "/issues/groups/{issue_group_id}/notes", scope: "issues:read", access: mcp.AccessRead,
		desc: "List notes (comments) on an Aikido issue group with author and time. Use after aikido_get_issue_group.",
		params: ps(
			issueGroupID(),
			boolean("include_personal_notes", "Also include personal notes."),
		),
	},
	{
		tool: "aikido_list_issue_group_tasks", method: http.MethodGet, path: "/issues/groups/{issue_group_id}/tasks", scope: "issues:read", access: mcp.AccessRead,
		desc:   "List task tracker tickets (Jira, Linear, GitHub and others) linked to an Aikido issue group, with status, assignee and URL. Use after aikido_get_issue_group.",
		params: ps(issueGroupID()),
	},
	{
		tool: "aikido_ignore_issue", method: http.MethodPut, path: "/issues/{issue_id}/ignore", scope: "issues:write", access: mcp.AccessWrite,
		desc:   "Ignore one Aikido security issue (mark a finding as false positive or accepted risk) with a reason. Reversible with aikido_unignore_issue.",
		params: ps(issueID(), str("reason", "Why the issue is ignored."), applyAllTags("ignore")),
	},
	{
		tool: "aikido_unignore_issue", method: http.MethodPut, path: "/issues/{issue_id}/unignore", scope: "issues:write", access: mcp.AccessWrite,
		desc:   "Unignore (reopen) one ignored Aikido security issue. Use after aikido_ignore_issue.",
		params: ps(issueID(), str("reason", "Why the issue is reopened."), applyAllTags("unignore")),
	},
	{
		tool: "aikido_snooze_issue", method: http.MethodPut, path: "/issues/{issue_id}/snooze", scope: "issues:write", access: mcp.AccessWrite,
		desc: "Snooze one Aikido security issue until a date, hiding it from the feed until then. Reversible with aikido_unsnooze_issue.",
		params: ps(
			issueID(),
			integer("snooze_until", "Unix timestamp (seconds) when the issue comes back.").req(),
			str("reason", "Why the issue is snoozed."),
			applyAllTags("snooze"),
		),
	},
	{
		tool: "aikido_unsnooze_issue", method: http.MethodPut, path: "/issues/{issue_id}/unsnooze", scope: "issues:write", access: mcp.AccessWrite,
		desc:   "Unsnooze one snoozed Aikido security issue so it returns to the feed now.",
		params: ps(issueID(), applyAllTags("unsnooze")),
	},
	{
		tool: "aikido_solve_issue", method: http.MethodPut, path: "/issues/{issue_id}/solve", scope: "issues:write", access: mcp.AccessWrite,
		desc:   "Mark one Aikido security issue as solved (fixed). Aikido reopens it if the next scan still finds it.",
		params: ps(issueID()),
	},
	{
		tool: "aikido_adjust_issue_severity", method: http.MethodPost, path: "/issues/{issue_id}/severity/adjust", scope: "issues:write", access: mcp.AccessWrite,
		desc: "Change the severity of one Aikido security issue (critical, high, medium or low) with a reason, for example after a reachability or exploitability review.",
		params: ps(
			issueID(),
			str("adjusted_severity", "New severity.").req().oneOf(severities...),
			str("reason", "Why the severity changes.").req(),
		),
	},
	{
		tool: "aikido_ignore_issue_group", method: http.MethodPut, path: "/issues/groups/{issue_group_id}/ignore", scope: "issues:write", access: mcp.AccessWrite,
		desc:   "Ignore a whole Aikido issue group (every finding in it) as false positive or accepted risk, with a reason. Reversible with aikido_unignore_issue_group.",
		params: ps(issueGroupID(), str("reason", "Why the group is ignored.")),
	},
	{
		tool: "aikido_unignore_issue_group", method: http.MethodPut, path: "/issues/groups/{issue_group_id}/unignore", scope: "issues:write", access: mcp.AccessWrite,
		desc:   "Unignore (reopen) an ignored Aikido issue group.",
		params: ps(issueGroupID(), str("reason", "Why the group is reopened.")),
	},
	{
		tool: "aikido_snooze_issue_group", method: http.MethodPut, path: "/issues/groups/{issue_group_id}/snooze", scope: "issues:write", access: mcp.AccessWrite,
		desc: "Snooze a whole Aikido issue group until a date. Reversible with aikido_unsnooze_issue_group.",
		params: ps(
			issueGroupID(),
			integer("snooze_until", "Unix timestamp (seconds) when the group comes back.").req(),
			str("reason", "Why the group is snoozed."),
		),
	},
	{
		tool: "aikido_unsnooze_issue_group", method: http.MethodPut, path: "/issues/groups/{issue_group_id}/unsnooze", scope: "issues:write", access: mcp.AccessWrite,
		desc:   "Unsnooze a snoozed Aikido issue group so it returns to the feed now.",
		params: ps(issueGroupID()),
	},
	{
		tool: "aikido_adjust_issue_group_severity", method: http.MethodPost, path: "/issues/groups/{issue_group_id}/severity/adjust", scope: "issues:write", access: mcp.AccessWrite,
		desc: "Change the severity of a whole Aikido issue group (critical, high, medium or low) with a reason.",
		params: ps(
			issueGroupID(),
			str("adjusted_severity", "New severity.").req().oneOf(severities...),
			str("reason", "Why the severity changes.").req(),
		),
	},
	{
		tool: "aikido_add_issue_group_note", method: http.MethodPost, path: "/issues/groups/{issue_group_id}/notes", scope: "issues:write", access: mcp.AccessWrite,
		desc: "Add a note (comment) to an Aikido issue group, optionally scoped to one CVE, to record triage decisions.",
		params: ps(
			issueGroupID(),
			str("note", "Note text.").req(),
			str("cve_id", "Limit the note to this CVE id."),
		),
	},
	{
		tool: "aikido_link_task_to_issue_group", method: http.MethodPost, path: "/task_tracking/linkTaskToIssueGroup", scope: "task_tracking:write", access: mcp.AccessWrite,
		desc: "Link an existing task tracker ticket (Jira, Linear, GitHub issue) to an Aikido issue group. Find the task with aikido_list_task_tracking_tasks.",
		params: ps(
			integer("issue_group_id", "Aikido issue group id.").req(),
			str("task_id", "Task id in the task tracker.").req(),
			str("project_id", "Task tracker project id from aikido_list_task_tracking_projects.").req(),
			str("scope", "Which issues the task covers (default group).").oneOf("group", "repo", "cloud", "domain", "container", "single_issue"),
			integer("scope_id", "Id of the repo, cloud, domain, container or issue for a non-group scope."),
			integer("integration_id", "Task tracker integration id when several are connected."),
		),
	},
	{
		tool: "aikido_unlink_task_from_issue_group", method: http.MethodPost, path: "/task_tracking/unlinkTaskFromIssueGroup", scope: "task_tracking:write", access: mcp.AccessWrite,
		desc: "Remove the link between a task tracker ticket and an Aikido issue group. Use after aikido_list_issue_group_tasks.",
		params: ps(
			integer("issue_group_id", "Aikido issue group id.").req(),
			integer("task_id", "Aikido id of the linked task (id from aikido_list_issue_group_tasks).").req(),
		),
	},
}
