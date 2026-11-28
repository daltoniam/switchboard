package aikido

import (
	"net/http"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

var sensitivityLevels = []string{"extreme", "sensitive", "normal", "not_sensitive", "no_data"}

var connectivityLevels = []string{"connected", "not_connected", "unknown"}

var ruleLanguages = []string{"JS", "TS", "PHP", "Java", "Scala", "GO", "PY", "Ruby", ".NET", "RUST", "Dart", "Swift", "Elixir", "C", "Clojure", "kotlin", "terraform", "generic"}

func codeRepoID() param {
	return integer("code_repo_id", "Aikido code repository id (id from aikido_list_code_repos).")
}

func sbomFormat() param {
	return str("format", "Export format (default sbom, CycloneDX JSON).").oneOf("sbom", "sbom_spdx", "csv")
}

func customRule() []param {
	return ps(
		str("semgrep_rule", "The Semgrep rule in YAML.").req(),
		str("issue_title", "Title of issues this rule raises.").req(),
		str("tldr", "Short summary of the problem the rule finds.").req(),
		str("how_to_fix", "How to fix a finding.").req(),
		integer("priority", "Severity score from 1 to 100.").req(),
		str("language", "Language the rule targets.").req().oneOf(ruleLanguages...),
	)
}

func prCheckSettings() []param {
	return ps(
		str("minimum_severity", "Lowest severity of new issues that fails the PR check.").req().oneOf("low", "medium", "high", "critical", "always_pass_check"),
		boolean("fail_on_dependency_scan", "Fail on new open source dependency vulnerabilities.").req(),
		boolean("fail_on_sast_scan", "Fail on new SAST code issues.").req(),
		boolean("fail_on_iac_scan", "Fail on new infrastructure as code issues.").req(),
		boolean("fail_on_secrets_scan", "Fail on new leaked secrets.").req(),
		boolean("fail_on_malware_scan", "Fail on new malware.").req(),
		str("minimum_license_severity", "Lowest license issue severity that fails the check.").req().oneOf("none", "high", "critical"),
		boolean("enable_code_quality_scan", "Run the code quality scan.").req(),
		boolean("fail_on_code_quality_scan", "Fail on new code quality issues.").req(),
		str("post_code_quality_inline_comments_min_severity", "Lowest code quality severity that gets inline comments.").req().oneOf("low", "medium", "high", "critical"),
		str("post_inline_comments_min_severity", "Lowest security severity that gets inline PR comments (default none).").oneOf("none", "low", "medium", "high", "critical"),
		boolean("run_deep_audit_pr_scan", "Run Deep Review on pull requests."),
	)
}

var codeEndpoints = []endpoint{
	{
		tool: "aikido_list_code_repos", method: http.MethodGet, path: "/repositories/code", scope: "repositories:read", access: mcp.AccessRead,
		desc: "List Git code repositories monitored by Aikido for code security (SAST, secrets, dependency scanning), with provider, branch, last scan time, sensitivity, connectivity and optional labels and lockfile or scan errors. Start here to find a repository id for Aikido findings, SBOMs and scans.",
		params: ps(
			boolean("include_inactive", "Include repositories that are not scanned."),
			str("filter_name", "Only repositories whose name matches."),
			str("filter_branch", "Only repositories scanning this branch."),
			str("filter_label", "Only repositories with this label."),
			boolean("include_labels", "Include labels."),
			boolean("include_config_issues_info", "Include missing or faulty lockfile flags and the last scan error."),
		),
		page: pages(10, 200, 20),
	},
	{
		tool: "aikido_get_code_repo", method: http.MethodGet, path: "/repositories/code/{code_repo_id}", scope: "repositories:read", access: mcp.AccessRead,
		desc:   "Get one Aikido code repository: provider, URL, scanned branch, active state, last scan time, linked teams, excluded paths, sensitivity, connectivity and labels. Use after aikido_list_code_repos.",
		params: ps(codeRepoID()),
	},
	{
		tool: "aikido_export_code_repo_sbom", method: http.MethodGet, path: "/repositories/code/{code_repo_id}/licenses/export", scope: "repositories:read", access: mcp.AccessRead,
		desc: "Export the SBOM (software bill of materials) of one Aikido code repository: every open source package, version and license, as CycloneDX JSON (default), SPDX or CSV. Use for dependency inventories and license compliance after aikido_list_code_repos.",
		params: ps(
			codeRepoID(),
			sbomFormat(),
			str("location", "Only packages from a lockfile path ending with this value, for example client/package-lock.json."),
			str("language", "Only this ecosystem.").oneOf("dotnet", "java", "js", "go", "rust", "dart", "swift", "ruby", "R", "php", "python", "elixir", "c++", "lua"),
			boolean("include_vex", "Include VEX exploitability analysis (CycloneDX only).").asInt(),
			boolean("include_risk_reason", "Include the license risk reason (CSV only).").asInt(),
			str("scope", "Dependency scope (default all).").oneOf("all", "only_dev_deps", "exclude_dev_deps"),
		),
		fixed: url.Values{"format": {"sbom"}},
	},
	{
		tool: "aikido_scan_code_repo", method: http.MethodPost, path: "/repositories/code/{code_repo_id}/scan", scope: "repositories:write", access: mcp.AccessWrite,
		desc: "Start a new Aikido scan of one code repository (dependency scan, plus optional SAST, IaC and secrets scans). Findings appear in aikido_list_issues when it finishes.",
		params: ps(
			codeRepoID(),
			boolean("include_sast_scan", "Also run the SAST code scan.").query(),
			boolean("include_iac_scan", "Also run the infrastructure as code scan.").query(),
			boolean("include_secrets_scan", "Also run the secrets scan.").query(),
		),
	},
	{
		tool: "aikido_sync_code_repos", method: http.MethodPost, path: "/repositories/import", scope: "repositories:write", access: mcp.AccessWrite,
		desc: "Sync code repositories from the connected Git provider (GitHub, GitLab, Bitbucket, Azure DevOps) so new repositories appear in Aikido.",
	},
	{
		tool: "aikido_activate_code_repo", method: http.MethodPost, path: "/repositories/code/activate", scope: "repositories:write", access: mcp.AccessWrite,
		desc:   "Activate an Aikido code repository so it is scanned. Reversible with aikido_deactivate_code_repo.",
		params: ps(codeRepoID().req()),
	},
	{
		tool: "aikido_deactivate_code_repo", method: http.MethodPost, path: "/repositories/code/deactivate", scope: "repositories:write", access: mcp.AccessWrite,
		desc:   "Deactivate an Aikido code repository so it is no longer scanned; its findings close. Reversible with aikido_activate_code_repo.",
		params: ps(codeRepoID().req()),
	},
	{
		tool: "aikido_update_code_repo_sensitivity", method: http.MethodPut, path: "/repositories/code/{code_repo_id}/sensitivity", scope: "repositories:write", access: mcp.AccessWrite,
		desc:   "Set how sensitive the data in an Aikido code repository is, which weighs issue severity.",
		params: ps(codeRepoID(), str("sensitivity", "Sensitivity level.").req().oneOf(sensitivityLevels...)),
	},
	{
		tool: "aikido_update_code_repo_connectivity", method: http.MethodPut, path: "/repositories/code/{code_repo_id}/connectivity", scope: "repositories:write", access: mcp.AccessWrite,
		desc:   "Set whether an Aikido code repository's application is reachable from the internet, which weighs issue severity.",
		params: ps(codeRepoID(), str("connectivity", "Internet connectivity.").req().oneOf(connectivityLevels...)),
	},
	{
		tool: "aikido_update_code_repo_dev_dep_scanning", method: http.MethodPut, path: "/repositories/code/{code_repo_id}/devdep-scan", scope: "repositories:write", access: mcp.AccessWrite,
		desc:   "Turn scanning of development dependencies on or off for an Aikido code repository.",
		params: ps(codeRepoID(), boolean("dev_dep_scanning_enabled", "Whether development dependencies are scanned.").req()),
	},
	{
		tool: "aikido_add_code_repo_exclude_path", method: http.MethodPost, path: "/repositories/code/{code_repo_id}/exclude-path", scope: "repositories:write", access: mcp.AccessWrite,
		desc:   "Exclude a path (for example test fixtures or vendored code) from Aikido scans of a code repository. Reversible with aikido_remove_code_repo_exclude_path.",
		params: ps(codeRepoID(), str("path", "Path to exclude.").req(), str("reason", "Why the path is excluded.")),
	},
	{
		tool: "aikido_remove_code_repo_exclude_path", method: http.MethodPost, path: "/repositories/code/{code_repo_id}/exclude-path/remove", scope: "repositories:write", access: mcp.AccessWrite,
		desc:   "Remove an excluded path so Aikido scans it again. Use after aikido_get_code_repo lists excluded paths.",
		params: ps(codeRepoID(), str("path", "Excluded path to remove.").req()),
	},
	{
		tool: "aikido_add_code_repo_label", method: http.MethodPost, path: "/repositories/code/{code_repo_id}/labels", scope: "repositories:write", access: mcp.AccessWrite,
		desc:   "Add a label (tag) to an Aikido code repository.",
		params: ps(codeRepoID(), str("name", "Label name.").req()),
	},
	{
		tool: "aikido_remove_code_repo_label", method: http.MethodDelete, path: "/repositories/code/{code_repo_id}/labels/{label_id}", scope: "repositories:write", access: mcp.AccessWrite,
		desc:   "Remove a label (tag) from an Aikido code repository. Label ids come from aikido_get_code_repo.",
		params: ps(codeRepoID(), integer("label_id", "Label id.")),
	},
	{
		tool: "aikido_delete_code_repo", method: http.MethodDelete, path: "/repositories/code/{code_repo_id}", scope: "repositories:write", access: mcp.AccessDestructive,
		desc:   "Delete an Aikido code repository and its findings. Only self-scanned or cloned repositories can be deleted; use aikido_deactivate_code_repo for synced ones.",
		params: ps(codeRepoID()),
	},
	{
		tool: "aikido_list_pr_checks", method: http.MethodGet, path: "/report/ciScans", scope: "reports:read", access: mcp.AccessRead,
		desc: "List Aikido PR checks (pull request and merge request security gating scans in CI/CD) with gate status (passed, failed, bypassed), new and solved issue counts, repository, branch and PR link. Start here to see why an Aikido PR check failed.",
		params: ps(
			integer("filter_code_repo_id", "Only PR checks for this code repository id."),
			str("filter_gate_status", "Only this gate status (default all).").oneOf("all", "timed_out", "unknown", "passed", "failed", "bypassed", "pending"),
			str("search", "Search PR titles and branches."),
		),
		page: pages(1, 50, 20),
	},
	{
		tool: "aikido_get_pr_check", method: http.MethodGet, path: "/report/ciScans/{scan_id}/detail", scope: "reports:read", access: mcp.AccessRead,
		desc:   "Get one Aikido PR check: PR title and URL, branch, commits, changed files, completed scans, and the security issues it added or removed with severity, CVE, package, file and lines. Use after aikido_list_pr_checks.",
		params: ps(integer("scan_id", "PR check scan_id from aikido_list_pr_checks.")),
	},
	{
		tool: "aikido_list_pr_check_actions", method: http.MethodGet, path: "/report/ciScans/issueActions", scope: "reports:read", access: mcp.AccessRead,
		desc: "List manual ignore and snooze actions people took on issues flagged in Aikido PR checks, with reason, user and the affected package, file or secret.",
		page: pages(1, 100, 20),
	},
	{
		tool: "aikido_list_pr_check_configs", method: http.MethodGet, path: "/repositories/code/continuous_integration/checks", scope: "repositories:read", access: mcp.AccessRead,
		desc:   "List Aikido PR check (CI gating) configurations per repository: minimum severity and which scans (dependency, SAST, IaC, secrets, malware, license, code quality) fail the check.",
		params: ps(integer("filter_code_repo_id", "Only the configuration for this code repository id.")),
		page:   pages(10, 100, 20),
	},
	{
		tool: "aikido_get_default_pr_check_config", method: http.MethodGet, path: "/repositories/code/continuous_integration/checks/default", scope: "repositories:read", access: mcp.AccessRead,
		desc: "Get the workspace default Aikido PR check configuration applied to newly activated repositories.",
	},
	{
		tool: "aikido_configure_pr_checks", method: http.MethodPost, path: "/repositories/code/continuous_integration/checks", scope: "repositories:write", access: mcp.AccessWrite,
		desc:   "Configure Aikido PR checks (CI security gating) for one code repository: minimum severity and which scan types fail the pull request check. Read the current values with aikido_list_pr_check_configs first.",
		params: append(ps(codeRepoID().req()), prCheckSettings()...),
	},
	{
		tool: "aikido_list_code_quality_findings", method: http.MethodGet, path: "/code-quality/repo-findings", scope: "code_quality:read", access: mcp.AccessRead,
		desc: "List Aikido code quality findings for a repository (full-repository scans) with rule, severity, file, lines and state.",
		params: ps(
			codeRepoID().req(),
			str("filter_state", "Finding state (default open).").oneOf("open", "ignored", "closed"),
		),
		unwrap: "findings",
		page:   pages(10, 100, 20),
	},
	{
		tool: "aikido_list_pr_code_quality_findings", method: http.MethodGet, path: "/code-quality/findings", scope: "code_quality:read", access: mcp.AccessRead,
		desc: "List Aikido code quality findings from the latest scan of one pull request, with rule, reasoning, file and lines.",
		params: ps(
			codeRepoID().req(),
			str("pr_number", "Pull request number.").req(),
		),
	},
	{
		tool: "aikido_ignore_code_quality_finding", method: http.MethodPut, path: "/code-quality/repo-findings/{finding_id}/ignore", scope: "code_quality:write", access: mcp.AccessWrite,
		desc:   "Ignore one open Aikido code quality finding with a reason. Finding ids come from aikido_list_code_quality_findings.",
		params: ps(integer("finding_id", "Code quality finding id."), str("reason", "Why the finding is ignored.")),
	},
	{
		tool: "aikido_list_code_coverage", method: http.MethodGet, path: "/code-coverage/repositories", scope: "code_coverage:read", access: mcp.AccessRead,
		desc:   "List repositories with test code coverage results in Aikido: covered, partial and missed lines and coverage percentage per branch (paid plans).",
		params: ps(str("branch_name", "Only coverage for this branch; default is each repository's default branch.")),
		unwrap: "code_coverage_repositories",
		page:   pages(10, 100, 20),
	},
	{
		tool: "aikido_get_code_coverage", method: http.MethodGet, path: "/code-coverage/repositories/{code_repo_id}", scope: "code_coverage:read", access: mcp.AccessRead,
		desc:   "Get the latest test code coverage summary for one Aikido code repository and branch (paid plans).",
		params: ps(codeRepoID(), str("branch_name", "Branch; default is the repository's default branch.")),
	},
	{
		tool: "aikido_list_scanner_rules", method: http.MethodGet, path: "/repositories/code/{kind}/rules", scope: "repositories:read", access: mcp.AccessRead,
		desc: "List the built-in Aikido scanner rules for SAST, infrastructure as code (IaC) or mobile scanning, with description, how to fix, language and open issue count.",
		params: ps(
			str("kind", "Rule set.").req().oneOf("sast", "iac", "mobile"),
			integer("repo_id", "Count open issues for this code repository id only."),
		),
	},
	{
		tool: "aikido_list_custom_rules", method: http.MethodGet, path: "/repositories/sast/custom-rules", scope: "custom_sast_rules:read", access: mcp.AccessRead,
		desc: "List custom Semgrep SAST rules defined in Aikido, with title, language, priority and whether the rule has an error.",
	},
	{
		tool: "aikido_get_custom_rule", method: http.MethodGet, path: "/repositories/sast/custom-rules/{rule_id}", scope: "custom_sast_rules:read", access: mcp.AccessRead,
		desc:   "Get one custom Semgrep SAST rule from Aikido, including the rule YAML.",
		params: ps(integer("rule_id", "Custom rule id.")),
	},
	{
		tool: "aikido_create_custom_rule", method: http.MethodPost, path: "/repositories/sast/custom-rules", scope: "custom_sast_rules:write", access: mcp.AccessWrite,
		desc:   "Create a custom Semgrep SAST rule in Aikido so code scans raise issues for it.",
		params: customRule(),
	},
	{
		tool: "aikido_update_custom_rule", method: http.MethodPut, path: "/repositories/sast/custom-rules/{rule_id}", scope: "custom_sast_rules:write", access: mcp.AccessWrite,
		desc:   "Replace a custom Semgrep SAST rule in Aikido. Every field is required; read the current rule with aikido_get_custom_rule first.",
		params: append(ps(integer("rule_id", "Custom rule id.")), customRule()...),
	},
	{
		tool: "aikido_delete_custom_rule", method: http.MethodDelete, path: "/repositories/sast/custom-rules/{rule_id}", scope: "custom_sast_rules:write", access: mcp.AccessDestructive,
		desc:   "Delete a custom Semgrep SAST rule from Aikido.",
		params: ps(integer("rule_id", "Custom rule id.")),
	},
}
