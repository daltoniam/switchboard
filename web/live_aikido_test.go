package web

import (
	"os"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/aikido"
)

// Aikido Security is hosted SaaS with no container image, so this spec
// reaches a real workspace through stored sandbox credentials and runs read
// tools only. Create a REST API client in Aikido Settings > Integrations >
// REST API with only :read scopes:
//
//	AIKIDO_LIVE_CLIENT_ID       REST API client ID (AIK_CLIENT_...)
//	AIKIDO_LIVE_CLIENT_SECRET   its client secret
//	AIKIDO_LIVE_REGION          optional: us, au or me for non-EU workspaces
//	AIKIDO_LIVE_ISSUE_GROUP_ID  an open issue group in that workspace
//	AIKIDO_LIVE_CODE_REPO_ID    an active, scanned code repository
func init() {
	adapter := aikido.New()
	writes := "changes the shared Aikido workspace; sandbox specs run read tools only and request shapes are covered by fixture tests in integrations/aikido"
	skips := map[mcp.ToolName]string{}
	for _, tool := range adapter.Tools() {
		if tool.Access != mcp.AccessRead {
			skips[tool.Name] = writes
		}
	}
	paid := "needs a paid Aikido plan or add-on the sandbox workspace does not have"
	for name, reason := range map[mcp.ToolName]string{
		"aikido_get_issue":                        "needs an issue id, which changes with every scan; covered by aikido_list_issues and fixture tests",
		"aikido_get_issue_reachability":           "reachability needs a reachable open source issue, which the sandbox repository may not have after a rescan",
		"aikido_get_issue_cve_exploitability":     "AI CVE exploitability results exist only after Aikido has analyzed a CVE issue",
		"aikido_list_issue_group_tasks":           "the sandbox workspace has no task tracker connected",
		"aikido_list_task_tracking_integrations":  "the sandbox workspace has no task tracker connected",
		"aikido_list_task_tracking_projects":      "the sandbox workspace has no task tracker connected",
		"aikido_list_task_tracking_tasks":         "the sandbox workspace has no task tracker connected",
		"aikido_get_pr_check":                     "needs a PR check scan id from a pull request the sandbox repository may not have",
		"aikido_list_pr_code_quality_findings":    "needs a pull request scanned by code quality",
		"aikido_list_code_quality_findings":       paid,
		"aikido_list_code_coverage":               paid,
		"aikido_get_code_coverage":                paid,
		"aikido_get_compliance_overview":          paid,
		"aikido_get_custom_rule":                  "the sandbox workspace has no custom SAST rule",
		"aikido_get_container":                    "the sandbox workspace has no container registry connected",
		"aikido_list_container_runners":           "the sandbox workspace has no container registry connected",
		"aikido_get_container_registry":           "the sandbox workspace has no container registry connected",
		"aikido_export_container_sbom":            "the sandbox workspace has no container registry connected",
		"aikido_list_cloud_assets":                "the sandbox workspace has no cloud connected",
		"aikido_list_virtual_machines":            "the sandbox workspace has no cloud with virtual machine scanning",
		"aikido_export_virtual_machine_sbom":      "the sandbox workspace has no cloud with virtual machine scanning",
		"aikido_list_subdomains":                  "the sandbox workspace monitors no attack surface domain",
		"aikido_list_domain_data_leaks":           "the sandbox workspace monitors no attack surface domain",
		"aikido_get_user":                         "needs a user id; aikido_list_users covers user reads",
		"aikido_list_issue_group_notes":           "the sandbox issue group has no notes",
		"aikido_get_package_health":               "package health scores cover only packages Aikido has scored, which changes over time",
		"aikido_get_changelog_summary":            "changelog summaries are generated asynchronously on first request",
		"aikido_list_zen_apps":                    "the sandbox workspace has no Zen firewall app",
		"aikido_get_zen_app":                      "the sandbox workspace has no Zen firewall app",
		"aikido_get_zen_event":                    "the sandbox workspace has no Zen firewall app",
		"aikido_list_zen_users":                   "the sandbox workspace has no Zen firewall app",
		"aikido_get_zen_countries":                "the sandbox workspace has no Zen firewall app",
		"aikido_get_zen_threat_lists":             "the sandbox workspace has no Zen firewall app",
		"aikido_get_zen_bot_lists":                "the sandbox workspace has no Zen firewall app",
		"aikido_list_endpoint_devices":            paid,
		"aikido_list_endpoint_installed_packages": paid,
		"aikido_list_endpoint_activity":           paid,
		"aikido_list_endpoint_exceptions":         paid,
		"aikido_list_endpoint_permission_groups":  paid,
		"aikido_get_autofix_task":                 "needs an AutoFix task the sandbox workspace has not run",
		"aikido_list_autofix_task_issues":         "needs an AutoFix task the sandbox workspace has not run",
		"aikido_list_pentest_assessments":         paid,
		"aikido_list_code_audits":                 paid,
		"aikido_get_pentest_assessment":           paid,
		"aikido_get_pentest_attack_analysis":      paid,
	} {
		skips[name] = reason
	}
	registerLiveSpec(liveSpec{
		Integration: "aikido",
		Tier:        liveSandbox,
		Adapter:     adapter,
		Provision:   liveAikidoProvision,
		Cases: []liveCase{
			{Tool: "aikido_get_workspace", Expect: []string{`"name"`, `"id"`}},
			{Tool: "aikido_get_workspace_config_errors", Expect: []string{"["}},
			{Tool: "aikido_get_sla_settings", Expect: []string{"critical", "duration_in_seconds"}},
			{Tool: "aikido_list_issue_groups", Expect: []string{"${issue_group_id}", "has_next_page"}},
			{Tool: "aikido_get_issue_group", Args: map[string]any{"issue_group_id": "${issue_group_id}"}, Expect: []string{"${issue_group_id}", "severity"}},
			{Tool: "aikido_list_issues", Args: map[string]any{"filter_issue_group_id": "${issue_group_id}", "per_page": 10}, Expect: []string{"${issue_group_id}", "severity"}},
			{Tool: "aikido_get_issue_counts", Expect: []string{"issue_groups", "critical"}},
			{Tool: "aikido_list_code_repos", Args: map[string]any{"include_labels": true, "include_config_issues_info": true}, Expect: []string{"${code_repo_id}", "has_next_page"}},
			{Tool: "aikido_get_code_repo", Args: map[string]any{"code_repo_id": "${code_repo_id}"}, Expect: []string{"${code_repo_id}", "branch"}},
			{Tool: "aikido_export_code_repo_sbom", Args: map[string]any{"code_repo_id": "${code_repo_id}"}, Expect: []string{"CycloneDX"}},
			{Tool: "aikido_list_pr_checks", Expect: []string{"has_next_page"}},
			{Tool: "aikido_list_pr_check_actions", Expect: []string{"has_next_page"}},
			{Tool: "aikido_list_pr_check_configs", Expect: []string{"has_next_page"}},
			{Tool: "aikido_get_default_pr_check_config", Expect: []string{"minimum_severity"}},
			{Tool: "aikido_list_scanner_rules", Args: map[string]any{"kind": "sast"}, Expect: []string{"title", "language"}},
			{Tool: "aikido_list_custom_rules", Expect: []string{"custom_rules"}},
			{Tool: "aikido_list_containers", Expect: []string{"has_next_page"}},
			{Tool: "aikido_list_clouds", Expect: []string{"has_next_page"}},
			{Tool: "aikido_list_cloud_rules", Expect: []string{"title"}},
			{Tool: "aikido_list_domains", Expect: []string{"has_next_page"}},
			{Tool: "aikido_list_teams", Expect: []string{"has_next_page"}},
			{Tool: "aikido_list_users", Expect: []string{"email", "role"}},
			{Tool: "aikido_list_user_login_history", Expect: []string{"login_timestamp"}},
			{Tool: "aikido_list_pending_invites", Expect: []string{"["}},
			{Tool: "aikido_list_ide_adoption", Expect: []string{"["}},
			{Tool: "aikido_list_activity_log", Expect: []string{"has_next_page"}},
			{Tool: "aikido_list_webhooks", Expect: []string{"["}},
			{Tool: "aikido_list_user_requests", Expect: []string{"["}},
			{Tool: "aikido_list_licenses", Expect: []string{"license_type", "package_name"}},
			{Tool: "aikido_get_cve", Args: map[string]any{"cve_id": "CVE-2021-44228"}, Expect: []string{"CVE-2021-44228", "severity"}},
			{Tool: "aikido_list_cwe", Expect: []string{"CWE-79"}},
			{Tool: "aikido_list_malware_packages", Args: map[string]any{"filter_ecosystem": "npm"}, Expect: []string{"npm", "package_name"}},
			{Tool: "aikido_list_eol_runtimes", Args: map[string]any{"package_name": "nodejs"}, Expect: []string{"eol_date"}},
			{Tool: "aikido_list_autofix_history", Expect: []string{"has_next_page"}},
			{Tool: "aikido_get_autofix_settings", Args: map[string]any{"kind": "dependency"}, Expect: []string{"settings"}},
		},
		Skips: skips,
	})
}

func liveAikidoProvision(t *testing.T) liveProvision {
	clientID := os.Getenv("AIKIDO_LIVE_CLIENT_ID")
	secret := os.Getenv("AIKIDO_LIVE_CLIENT_SECRET")
	groupID := os.Getenv("AIKIDO_LIVE_ISSUE_GROUP_ID")
	repoID := os.Getenv("AIKIDO_LIVE_CODE_REPO_ID")
	if clientID == "" || secret == "" || groupID == "" || repoID == "" {
		t.Skip("set AIKIDO_LIVE_CLIENT_ID, AIKIDO_LIVE_CLIENT_SECRET, AIKIDO_LIVE_ISSUE_GROUP_ID and AIKIDO_LIVE_CODE_REPO_ID to reach a real Aikido workspace; Aikido is SaaS with no container image")
	}
	creds := map[string]string{"client_id": clientID, "client_secret": secret}
	if region := os.Getenv("AIKIDO_LIVE_REGION"); region != "" {
		creds["region"] = region
	}
	return liveProvision{
		Credentials: creds,
		Values:      map[string]string{"issue_group_id": groupID, "code_repo_id": repoID},
	}
}
