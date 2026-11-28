package aikido

import (
	"net/http"

	mcp "github.com/daltoniam/switchboard"
)

func teamID() param { return integer("team_id", "Aikido team id (id from aikido_list_teams).") }

func teamResources(verb string) []param {
	return ps(
		integer("repo_id", "Code repository id to "+verb+"."),
		integer("cloud_id", "Cloud id to "+verb+"."),
		integer("image_id", "Container repository id to "+verb+"."),
		integer("domain_id", "Domain id to "+verb+"."),
		integer("zen_app_id", "Zen app id to "+verb+"."),
	)
}

var webhookEvents = []string{"issue.open.created", "issue.open.created.open_source", "issue.open.created.sast", "issue.snoozed", "issue.ignored.manual", "ci.gate.failed", "issue.closed", "ci.gate.passed", "issue.sla.breached", "zen.attack", "zen.attack_wave", "zen.outbound.discovered", "scan.image.finished", "scan.domain.finished", "issue.unignored", "issue.unignored.manual", "issue.severity.changed.manual", "configuration_issue.detected", "inbox.issues.request.created", "pentest.running", "pentest.paused", "pentest.finished", "pentest.failed", "pentest.cancelled"}

var orgEndpoints = []endpoint{
	{
		tool: "aikido_get_workspace", method: http.MethodGet, path: "/workspace", scope: "basics:read", access: mcp.AccessRead,
		desc: "Get the Aikido workspace these credentials belong to: id, name, linked Git provider and organization. Use to confirm which Aikido security workspace is connected.",
	},
	{
		tool: "aikido_get_workspace_config_errors", method: http.MethodGet, path: "/workspace/configurationErrors", scope: "basics:read", access: mcp.AccessRead,
		desc: "List Aikido workspace configuration errors (broken integrations, missing lockfiles, failing scans) with location, severity and detection time. Use to find why Aikido scans are incomplete.",
	},
	{
		tool: "aikido_get_sla_settings", method: http.MethodGet, path: "/workspace/slaSettings", scope: "basics:read", access: mcp.AccessRead,
		desc: "Get the Aikido remediation SLA settings: days allowed to fix critical, high, medium and low security issues.",
	},
	{
		tool: "aikido_list_teams", method: http.MethodGet, path: "/teams", scope: "teams:read", access: mcp.AccessRead,
		desc: "List Aikido teams with their responsibilities (owned repositories, containers, clouds, domains, path limits) and import source. Use to find who owns a security issue or to filter findings by team.",
		page: pages(10, 100, 20),
	},
	{
		tool: "aikido_create_team", method: http.MethodPost, path: "/teams", scope: "teams:write", access: mcp.AccessWrite,
		desc:   "Create an Aikido team. Link resources with aikido_link_resource_to_team and members with aikido_add_user_to_team.",
		params: ps(str("name", "Team name.").req()),
	},
	{
		tool: "aikido_update_team", method: http.MethodPut, path: "/teams/{team_id}", scope: "teams:write", access: mcp.AccessWrite,
		desc:   "Rename an Aikido team.",
		params: ps(teamID(), str("name", "New team name.").req()),
	},
	{
		tool: "aikido_delete_team", method: http.MethodDelete, path: "/teams/{team_id}", scope: "teams:write", access: mcp.AccessDestructive,
		desc:   "Delete an Aikido team that was created in Aikido (imported teams cannot be deleted).",
		params: ps(teamID()),
	},
	{
		tool: "aikido_link_resource_to_team", method: http.MethodPost, path: "/teams/{team_id}/linkResource", scope: "teams:write", access: mcp.AccessWrite,
		desc:   "Make an Aikido team responsible for a code repository, cloud, container, domain or Zen app, so its security issues route to that team. Pass exactly one resource id.",
		params: append(ps(teamID(), object("repo_path_limitation", "Optional repository path limits: {\"limitation_type\": \"include\" or \"exclude\", \"paths\": [...]}.")), teamResources("link")...),
	},
	{
		tool: "aikido_unlink_resource_from_team", method: http.MethodPost, path: "/teams/{team_id}/unlinkResource", scope: "teams:write", access: mcp.AccessWrite,
		desc:   "Remove an Aikido team's responsibility for a code repository, cloud, container, domain or Zen app. Pass exactly one resource id.",
		params: append(ps(teamID()), teamResources("unlink")...),
	},
	{
		tool: "aikido_add_user_to_team", method: http.MethodPost, path: "/teams/{team_id}/addUser", scope: "teams:write", access: mcp.AccessWrite,
		desc:   "Add an Aikido user to a team.",
		params: ps(teamID(), integer("user_id", "User id from aikido_list_users.").req()),
	},
	{
		tool: "aikido_remove_user_from_team", method: http.MethodPost, path: "/teams/{team_id}/removeUser", scope: "teams:write", access: mcp.AccessWrite,
		desc:   "Remove an Aikido user from a team.",
		params: ps(teamID(), integer("user_id", "User id from aikido_list_users.").req()),
	},
	{
		tool: "aikido_list_users", method: http.MethodGet, path: "/users", scope: "users:read", access: mcp.AccessRead,
		desc: "List Aikido workspace users (members) with name, email, role, auth type, active state and last login.",
		params: ps(
			integer("filter_team_id", "Only members of this team id."),
			boolean("include_inactive", "Include inactive users.").asInt(),
		),
	},
	{
		tool: "aikido_get_user", method: http.MethodGet, path: "/users/{user_id}", scope: "users:read", access: mcp.AccessRead,
		desc:   "Get one Aikido user with role, read-only flag, auth type, last login and fine-grained permissions (ignore, snooze, manage repos, clouds, teams and more).",
		params: ps(integer("user_id", "User id from aikido_list_users.")),
	},
	{
		tool: "aikido_list_user_login_history", method: http.MethodGet, path: "/users/login/history", scope: "users:read", access: mcp.AccessRead,
		desc:   "List Aikido workspace login events (user and time), newest first, for access audits.",
		params: ps(integer("filter_user_id", "Only logins by this user id.")),
		page:   pages(1, 100, 20),
	},
	{
		tool: "aikido_list_pending_invites", method: http.MethodGet, path: "/users/pending-invites", scope: "users:read", access: mcp.AccessRead,
		desc: "List unaccepted Aikido user invites (email and shareable link) with role and expiry.",
	},
	{
		tool: "aikido_list_ide_adoption", method: http.MethodGet, path: "/users/ide/adoption", scope: "users:read", access: mcp.AccessRead,
		desc:   "List Aikido users with an active IDE plugin token: installed plugins, last use, and whether Safe Chain and pre-commit secret scanning are on.",
		params: ps(str("ide", "Only this IDE (default all).").oneOf("all", "vscode", "jetbrains", "cursor", "windsurf", "visualstudio", "eclipse", "kiro", "antigravity", "mcp")),
	},
	{
		tool: "aikido_update_user_rights", method: http.MethodPut, path: "/users/{user_id}/rights", scope: "users:write", access: mcp.AccessWrite,
		desc: "Change an Aikido user's role (admin, default, team_only), read-only flag and fine-grained permissions.",
		params: ps(
			integer("user_id", "User id from aikido_list_users."),
			str("role", "Role.").req().oneOf("admin", "default", "team_only"),
			integer("read_only", "1 for read-only, 0 for full rights; omit to leave unchanged."),
			object("permissions", "Permission flags such as {\"can_ignore_issues\": 1, \"can_manage_repos\": 0}."),
		),
	},
	{
		tool: "aikido_disable_user", method: http.MethodPut, path: "/users/{user_id}/disable", scope: "users:write", access: mcp.AccessDestructive,
		desc:   "Disable an Aikido user, removing them from the workspace.",
		params: ps(integer("user_id", "User id from aikido_list_users.")),
	},
	{
		tool: "aikido_list_activity_log", method: http.MethodGet, path: "/report/activityLog", scope: "reports:read", access: mcp.AccessRead,
		desc: "List the Aikido audit activity log: who ignored, snoozed, solved or changed issues and settings, with time, user, action, issue and comment.",
		params: ps(
			integer("start", "Only items after this Unix timestamp (seconds)."),
			integer("end", "Only items before this Unix timestamp (seconds)."),
			integer("repository_id", "Only items for this code repository id."),
			integer("cloud_id", "Only items for this cloud id."),
			str("user_type_filter", "Who acted, for example all, user_only or ai."),
			str("action_filter", "Action, for example snoozed, ignored or severity_adjusted."),
			str("search", "Search term."),
		),
		page: pages(1, 100, 20),
	},
	{
		tool: "aikido_list_webhooks", method: http.MethodGet, path: "/webhooks", scope: "webhooks:read", access: mcp.AccessRead,
		desc: "List Aikido webhooks with target URL, event type and delivery health.",
	},
	{
		tool: "aikido_add_webhook", method: http.MethodPost, path: "/webhooks", scope: "webhooks:write", access: mcp.AccessWrite,
		desc: "Add an Aikido webhook that posts an event type (new issue, SLA breach, PR gate failed, Zen attack, scan finished and more) to a URL.",
		params: ps(
			str("target_url", "HTTPS URL that receives the webhook.").req(),
			str("event_type", "Event to send.").req().oneOf(webhookEvents...),
		),
	},
	{
		tool: "aikido_remove_webhook", method: http.MethodDelete, path: "/webhooks/{webhook_id}", scope: "webhooks:write", access: mcp.AccessDestructive,
		desc:   "Remove an Aikido webhook.",
		params: ps(integer("webhook_id", "Webhook id from aikido_list_webhooks.")),
	},
	{
		tool: "aikido_list_user_requests", method: http.MethodGet, path: "/request-inbox/requests", scope: "request_inbox:read", access: mcp.AccessRead,
		desc:   "List the Aikido request inbox: developer requests to ignore or snooze issues awaiting admin approval, with reason, issue and status.",
		params: ps(str("filter_status", "Only this status.").oneOf("pending", "approved", "rejected")),
	},
	{
		tool: "aikido_review_user_request", method: http.MethodPost, path: "/request-inbox/requests/{request_id}/review", scope: "request_inbox:write", access: mcp.AccessWrite,
		desc: "Approve or reject a request in the Aikido request inbox. Use after aikido_list_user_requests.",
		params: ps(
			integer("request_id", "Request id from aikido_list_user_requests."),
			str("status", "Decision.").req().oneOf("approved", "rejected"),
		),
	},
	{
		tool: "aikido_list_task_tracking_integrations", method: http.MethodGet, path: "/task_tracking/integrations", scope: "task_tracking:read", access: mcp.AccessRead,
		desc: "List task trackers (Jira, Linear, GitHub, GitLab, Asana and more) connected to Aikido.",
	},
	{
		tool: "aikido_list_task_tracking_projects", method: http.MethodGet, path: "/task_tracking/projects", scope: "task_tracking:read", access: mcp.AccessRead,
		desc:   "List task tracker projects connected to Aikido and the code repositories mapped to each.",
		params: ps(integer("integration_id", "Task tracker integration id when several are connected.")),
	},
	{
		tool: "aikido_list_task_tracking_tasks", method: http.MethodGet, path: "/task_tracking/projects/{project_id}/tasks", scope: "task_tracking:read", access: mcp.AccessRead,
		desc: "Search tickets live in a task tracker project connected to Aikido, to link one to an issue group.",
		params: ps(
			str("project_id", "Project id from aikido_list_task_tracking_projects."),
			str("search", "Search text."),
			integer("integration_id", "Task tracker integration id when several are connected."),
		),
	},
}
