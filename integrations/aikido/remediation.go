package aikido

import (
	"net/http"

	mcp "github.com/daltoniam/switchboard"
)

var remediationEndpoints = []endpoint{
	{
		tool: "aikido_list_autofix_history", method: http.MethodGet, path: "/autofix/history", scope: "autofix:read", access: mcp.AccessRead,
		desc: "List Aikido AutoFix tasks (automatic dependency upgrade, SAST, IaC and container fix pull requests) with status, target repository and file, trigger and resulting pull request.",
		page: pages(1, 20, 20),
	},
	{
		tool: "aikido_get_autofix_task", method: http.MethodGet, path: "/autofix/history/{autofix_task_id}", scope: "autofix:read", access: mcp.AccessRead,
		desc:   "Get one Aikido AutoFix task; poll its status to follow a fix and get the pull request URL. Use after aikido_create_dependency_autofix or aikido_list_autofix_history.",
		params: ps(integer("autofix_task_id", "AutoFix task id.")),
	},
	{
		tool: "aikido_list_autofix_task_issues", method: http.MethodGet, path: "/autofix/history/{autofix_task_id}/issues", scope: "autofix:read", access: mcp.AccessRead,
		desc:   "List the Aikido issue ids an AutoFix task tries to solve.",
		params: ps(integer("autofix_task_id", "AutoFix task id.")),
	},
	{
		tool: "aikido_get_autofix_settings", method: http.MethodGet, path: "/repositories/autofix/{kind}/settings", scope: "autofix:read", access: mcp.AccessRead,
		desc:   "Get the workspace Aikido AutoFix pull request settings for dependency, SAST and IaC, pentest, or container fixes: enabled, severity filter and repository or container scope.",
		params: ps(str("kind", "Which AutoFix settings.").req().oneOf("dependency", "sast", "pentest", "container")),
	},
	{
		tool: "aikido_create_dependency_autofix", method: http.MethodPost, path: "/autofix/dependency", scope: "autofix:write", access: mcp.AccessWrite,
		desc: "Open an Aikido AutoFix pull request that upgrades vulnerable dependencies in one lockfile or manifest to fix CVEs. Aikido picks patch or minor versions. Follow it with aikido_get_autofix_task.",
		params: ps(
			codeRepoID().req(),
			str("target_path", "Repository-relative lockfile or manifest path, as affected_file in aikido_get_issue.").req(),
			value("packages", "Packages to fix: [{\"name\": \"lodash\", \"cve_ids\": [\"CVE-2021-23337\"]}].").req(),
		),
	},
	{
		tool: "aikido_list_pentest_assessments", method: http.MethodGet, path: "/pentests/assessments", scope: "pentests:read", access: mcp.AccessRead,
		desc: "List Aikido AI pentest assessments (penetration tests), newest first, with status and creation time.",
		page: pages(10, 100, 20),
	},
	{
		tool: "aikido_list_code_audits", method: http.MethodGet, path: "/pentests/code-audits", scope: "pentests:read", access: mcp.AccessRead,
		desc: "List Aikido AI code analysis (code audit) assessments, newest first, with status and creation time.",
		page: pages(10, 100, 20),
	},
	{
		tool: "aikido_get_pentest_assessment", method: http.MethodGet, path: "/pentests/assessments/{assessment_id}/detail", scope: "pentests:read", access: mcp.AccessRead,
		desc:   "Get the status, status reason and issue count of one Aikido pentest assessment. List its findings with aikido_list_issues filter_assessment_id.",
		params: ps(str("assessment_id", "Assessment UUID from aikido_list_pentest_assessments.")),
	},
	{
		tool: "aikido_get_pentest_attack_analysis", method: http.MethodGet, path: "/pentests/issues/{issue_id}/attackAnalysis", scope: "pentests:read", access: mcp.AccessRead,
		desc:   "Get the attack analysis of one Aikido pentest finding: summary, risk, reproduction steps, root cause, remediation and retest results.",
		params: ps(integer("issue_id", "Pentest issue id from aikido_list_issues.")),
	},
}
