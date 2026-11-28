package server

import (
	"testing"

	"github.com/daltoniam/switchboard/integrations/aikido"
	"github.com/stretchr/testify/require"
)

func TestAikidoSearchDiscoverability(t *testing.T) {
	raw := collectAllTools()
	tools := make([]toolWithIntegration, 0, len(raw)+160)
	for _, entry := range raw {
		if entry.Integration == "aikido" {
			continue
		}
		tools = append(tools, toolWithIntegration{Integration: entry.Integration, Tool: entry.Tool})
	}
	for _, tool := range aikido.New().Tools() {
		tools = append(tools, toolWithIntegration{Integration: "aikido", Tool: tool})
	}
	idf := computeIDF(tools)
	synonyms := buildSynonymMap(synonymGroups)
	for _, test := range []struct{ query, want string }{
		{"aikido security issues to fix first", "aikido_list_issue_groups"},
		{"aikido open vulnerabilities to fix first", "aikido_list_issue_groups"},
		{"security vulnerabilities findings", "aikido_list_issue_groups"},
		{"aikido CVE vulnerabilities in a repository", "aikido_list_issues"},
		{"aikido leaked secrets", "aikido_list_issue_groups"},
		{"aikido SAST code findings", "aikido_list_issue_groups"},
		{"aikido cloud posture misconfigurations", "aikido_list_clouds"},
		{"aikido container image vulnerabilities", "aikido_list_containers"},
		{"aikido ignore issue false positive", "aikido_ignore_issue"},
		{"aikido snooze issue", "aikido_snooze_issue"},
		{"aikido SBOM export", "aikido_export_code_repo_sbom"},
		{"aikido PR check failed", "aikido_list_pr_checks"},
		{"aikido SOC 2 compliance", "aikido_get_compliance_overview"},
		{"aikido zen firewall attacks", "aikido_list_zen_apps"},
		{"aikido CVE details lookup", "aikido_get_cve"},
		{"aikido malicious packages malware", "aikido_list_malware_packages"},
		{"aikido issue counts by severity", "aikido_get_issue_counts"},
		{"aikido code repositories", "aikido_list_code_repos"},
	} {
		t.Run(test.query, func(t *testing.T) {
			results := scoreTools(test.query, tools, idf, synonyms)
			names := make([]string, 0, 5)
			for _, result := range results[:min(len(results), 5)] {
				names = append(names, string(result.Tool.Name))
			}
			require.Contains(t, names, test.want)
			t.Logf("query=%q top5=%v", test.query, names)
		})
	}
}
