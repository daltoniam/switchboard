package server

import (
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/aws"
	"github.com/daltoniam/switchboard/integrations/clickhouse"
	"github.com/daltoniam/switchboard/integrations/datadog"
	"github.com/daltoniam/switchboard/integrations/github"
	"github.com/daltoniam/switchboard/integrations/gmail"
	"github.com/daltoniam/switchboard/integrations/homeassistant"
	"github.com/daltoniam/switchboard/integrations/linear"
	"github.com/daltoniam/switchboard/integrations/metabase"
	"github.com/daltoniam/switchboard/integrations/notion"
	"github.com/daltoniam/switchboard/integrations/pganalyze"
	"github.com/daltoniam/switchboard/integrations/postgres"
	"github.com/daltoniam/switchboard/integrations/posthog"
	"github.com/daltoniam/switchboard/integrations/sentry"
	"github.com/daltoniam/switchboard/integrations/slack"
	"github.com/daltoniam/switchboard/integrations/ynab"
)

// searchBenchmarkCase defines a test scenario for measuring search quality.
// Single-tool cases test vocabulary matching; multi-tool cases test
// cross-integration discovery.
type searchBenchmarkCase struct {
	Name                 string
	Query                string
	ExpectedTools        []string // tool names that should appear in results
	ExpectedIntegrations []string // integrations that should be represented
	K                    int      // look within top-K results
}

// collectAllTools gathers tool definitions from all real integrations,
// paired with their integration name.
func collectAllTools() []struct {
	Integration string
	Tool        mcp.ToolDefinition
} {
	integrations := []mcp.Integration{
		github.New(),
		linear.New(),
		sentry.New(),
		slack.New(),
		notion.New(),
		datadog.New(),
		posthog.New(),
		metabase.New(),
		aws.New(),
		postgres.New(),
		clickhouse.New(),
		pganalyze.New(),
		gmail.New(),
		ynab.New(),
		homeassistant.New(),
	}

	var all []struct {
		Integration string
		Tool        mcp.ToolDefinition
	}
	for _, i := range integrations {
		for _, t := range i.Tools() {
			all = append(all, struct {
				Integration string
				Tool        mcp.ToolDefinition
			}{Integration: i.Name(), Tool: t})
		}
	}
	return all
}

// --- Single-tool benchmark cases: vocabulary mismatches ---
// These test whether synonym expansion helps find the right tool
// when the LLM uses different vocabulary than the tool name.

var singleToolCases = []searchBenchmarkCase{
	// Synonym mismatches
	{Name: "synonym/ticket", Query: "create ticket", ExpectedTools: []string{"linear_create_issue"}, K: 5},
	{Name: "synonym/send-message", Query: "send message", ExpectedTools: []string{"slack_send_message"}, K: 5},
	{Name: "synonym/deploy-status", Query: "deploy status", ExpectedTools: []string{"github_list_deployments"}, K: 5},
	{Name: "synonym/view-pr", Query: "view pr", ExpectedTools: []string{"github_get_pull"}, K: 5},
	{Name: "synonym/find-bugs", Query: "find bugs", ExpectedTools: []string{"sentry_list_issues", "sentry_list_org_issues"}, K: 5},
	{Name: "synonym/add-repo", Query: "add repo", ExpectedTools: []string{"github_create_repo"}, K: 5},
	{Name: "synonym/remove-ref", Query: "remove branch", ExpectedTools: []string{"github_delete_ref"}, K: 5},
	{Name: "synonym/edit-issue", Query: "edit issue", ExpectedTools: []string{"linear_update_issue"}, K: 5},
	{Name: "synonym/lookup-user", Query: "lookup user", ExpectedTools: []string{"github_get_user"}, K: 5},
	{Name: "synonym/post-notification", Query: "post notification", ExpectedTools: []string{"slack_send_message"}, K: 5},
	{Name: "synonym/search-logs", Query: "find logs", ExpectedTools: []string{"datadog_search_logs"}, K: 5},
	{Name: "synonym/query-database", Query: "query database", ExpectedTools: []string{"postgres_execute_query", "clickhouse_execute_query"}, K: 5},

	// Workflow: project updates — the LLM should find search_projects and get_project
	// in a single search call, avoiding the 40+ tool call discovery spiral.
	// K=20 matches the default search limit the LLM sees.
	{Name: "workflow/project-update", Query: "project update", ExpectedTools: []string{"linear_get_project", "linear_list_project_updates"}, K: 20},
	{Name: "workflow/project-status", Query: "project status", ExpectedTools: []string{"linear_list_projects", "linear_search_projects"}, K: 20},
	{Name: "workflow/find-project-by-name", Query: "find project by name", ExpectedTools: []string{"linear_search_projects"}, K: 10},

	// Regression guard: exact matches that should always work
	{Name: "exact/github-list-issues", Query: "github list issues", ExpectedTools: []string{"github_list_issues"}, K: 5},
	{Name: "exact/linear-create-issue", Query: "linear create issue", ExpectedTools: []string{"linear_create_issue"}, K: 5},
	{Name: "exact/slack-send-message", Query: "slack send message", ExpectedTools: []string{"slack_send_message"}, K: 5},
	{Name: "exact/datadog-search-logs", Query: "datadog search logs", ExpectedTools: []string{"datadog_search_logs"}, K: 5},
	{Name: "exact/sentry-get-issue", Query: "sentry get issue", ExpectedTools: []string{"sentry_get_issue"}, K: 5},
}

// --- Multi-tool benchmark cases: cross-integration intent ---
// These test whether a single search call can surface tools from
// multiple integrations to fulfill a workflow-level intent.
//
// Mental model: we want x=1 calls to find N tools across integrations.
// Best case: 1 call → all tools found. Worst case: N calls.
// Target: 1 ≤ x < N.

var multiToolCases = []searchBenchmarkCase{
	// DevOps / SRE
	{
		Name:                 "devops/alerts-firing",
		Query:                "alerts firing now",
		ExpectedIntegrations: []string{"datadog", "sentry"},
		K:                    10,
	},
	{
		Name:                 "devops/deploy-broke-something",
		Query:                "deploy broke something find the commit",
		ExpectedIntegrations: []string{"github", "datadog", "sentry"},
		K:                    10,
	},
	{
		Name:                 "devops/slow-queries-database",
		Query:                "slow queries killing the database",
		ExpectedIntegrations: []string{"postgres", "pganalyze", "datadog"},
		K:                    10,
	},
	{
		Name:                 "devops/what-changed-prod",
		Query:                "what changed in prod last 2 hours",
		ExpectedIntegrations: []string{"github", "datadog", "aws"},
		K:                    10,
	},
	{
		Name:                 "devops/postmortem-timeline",
		Query:                "postmortem yesterday outage timeline",
		ExpectedIntegrations: []string{"datadog", "sentry", "github", "slack"},
		K:                    15,
	},

	// Product Manager
	{
		Name:                 "pm/what-shipped-and-broke",
		Query:                "what shipped last week and did anything break",
		ExpectedIntegrations: []string{"github", "sentry", "datadog"},
		K:                    10,
	},
	{
		Name:                 "pm/checkout-bug-complaints",
		Query:                "show me everything about the checkout bug customers are complaining about",
		ExpectedIntegrations: []string{"sentry", "linear", "slack", "github"},
		K:                    15,
	},
	{
		Name:                 "pm/feature-adoption",
		Query:                "how many users actually use the feature we launched",
		ExpectedIntegrations: []string{"posthog", "linear", "github"},
		K:                    10,
	},
	{
		Name:                 "pm/infra-spend",
		Query:                "is our infrastructure spend going up",
		ExpectedIntegrations: []string{"aws"},
		K:                    10,
	},

	// Customer Success
	{
		Name:                 "cs/churn-risk",
		Query:                "which accounts are at risk of churning",
		ExpectedIntegrations: []string{"posthog", "metabase", "postgres"},
		K:                    10,
	},
	{
		Name:                 "cs/account-renewal-prep",
		Query:                "prepare account review for renewal",
		ExpectedIntegrations: []string{"notion", "slack", "linear", "posthog"},
		K:                    15,
	},
	{
		Name:                 "cs/incident-response-time",
		Query:                "how long is our average response time to customer incidents",
		ExpectedIntegrations: []string{"sentry", "linear", "slack"},
		K:                    10,
	},
	{
		Name:                 "cs/outage-status-update",
		Query:                "send status update to affected customers for the outage",
		ExpectedIntegrations: []string{"slack", "gmail", "sentry", "datadog"},
		K:                    15,
	},

	// CEO / Exec
	{
		Name:                 "ceo/board-health-check",
		Query:                "health check on the business before board call",
		ExpectedIntegrations: []string{"linear", "github", "sentry", "posthog", "datadog"},
		K:                    20,
	},
	{
		Name:                 "ceo/enterprise-deals-on-track",
		Query:                "are we on track to close the enterprise deals",
		ExpectedIntegrations: []string{"linear", "notion", "slack"},
		K:                    10,
	},

	// Office Manager
	{
		Name:                 "office/offsite-budget",
		Query:                "track what we spent on team offsite flag over budget",
		ExpectedIntegrations: []string{"ynab", "gmail", "slack"},
		K:                    10,
	},

	// Data Analyst
	{
		Name:                 "analyst/activation-drop",
		Query:                "why did signup to activation rate drop last Tuesday",
		ExpectedIntegrations: []string{"posthog", "sentry", "datadog", "github"},
		K:                    15,
	},
	{
		Name:                 "analyst/revenue-vs-usage",
		Query:                "revenue numbers by plan tier matched against feature usage",
		ExpectedIntegrations: []string{"metabase", "postgres", "posthog"},
		K:                    10,
	},

	// Security
	{
		Name:                 "security/access-audit",
		Query:                "who got access to production last 30 days and did any trigger alerts",
		ExpectedIntegrations: []string{"aws", "datadog"},
		K:                    10,
	},
	{
		Name:                 "security/cve-impact",
		Query:                "is the CVE from this morning affecting our running services",
		ExpectedIntegrations: []string{"github", "aws", "sentry"},
		K:                    10,
	},

	// Sales
	{
		Name:                 "sales/deal-context",
		Query:                "pull up everything on the acme deal before my call",
		ExpectedIntegrations: []string{"notion", "slack", "linear"},
		K:                    10,
	},
	{
		Name:                 "sales/follow-up-email",
		Query:                "draft follow-up email summarizing what we agreed to fix for the pilot",
		ExpectedIntegrations: []string{"notion", "linear", "gmail"},
		K:                    10,
	},
	{
		Name:                 "sales/pipeline-health",
		Query:                "pipeline health snapshot which deals slipped this week",
		ExpectedIntegrations: []string{"notion", "linear", "slack"},
		K:                    10,
	},
	{
		Name:                 "sales/forecast-accuracy",
		Query:                "forecast accuracy last three quarters what we called vs what closed",
		ExpectedIntegrations: []string{"notion", "metabase", "postgres"},
		K:                    10,
	},

	// Marketing
	{
		Name:                 "marketing/launch-performance",
		Query:                "how did last week's product launch perform organic traffic and signups",
		ExpectedIntegrations: []string{"posthog", "metabase", "notion"},
		K:                    10,
	},
	{
		Name:                 "marketing/pipeline-attribution",
		Query:                "what campaigns drove the most qualified pipeline last quarter",
		ExpectedIntegrations: []string{"metabase", "postgres", "notion"},
		K:                    10,
	},

	// Growth / PLG
	{
		Name:                 "growth/trial-funnel-dropoff",
		Query:                "activation funnel for last month trial cohort where are people dropping off",
		ExpectedIntegrations: []string{"posthog", "sentry", "datadog"},
		K:                    10,
	},
	{
		Name:                 "growth/expansion-accounts",
		Query:                "which self-serve accounts hit expansion threshold but haven't been contacted",
		ExpectedIntegrations: []string{"metabase", "postgres", "notion", "slack"},
		K:                    15,
	},
	{
		Name:                 "growth/trial-error-churn",
		Query:                "flag trial accounts with sentry error spike past 48 hours churn risks",
		ExpectedIntegrations: []string{"sentry", "metabase", "postgres", "slack"},
		K:                    15,
	},
}

// TestSearchBenchmark runs the full benchmark corpus against both the
// current matches() function and the new scoreTools() function,
// reporting hit rate metrics side-by-side.
//
// This test reports only — it does not assert pass/fail.
func TestSearchBenchmark(t *testing.T) {
	raw := collectAllTools()
	t.Logf("Total tools in corpus: %d", len(raw))

	// Build scoring index from real tools.
	tools := make([]toolWithIntegration, len(raw))
	for i, r := range raw {
		tools[i] = toolWithIntegration{Integration: r.Integration, Tool: r.Tool}
	}
	synMap := buildSynonymMap(synonymGroups)
	idf := computeIDF(tools)

	// matchesSearch wraps the old matches() approach, returning tool names.
	matchesSearch := func(query string) []string {
		q := strings.ToLower(query)
		var out []string
		for _, ti := range raw {
			if matches(ti.Tool, ti.Integration, q) {
				out = append(out, ti.Tool.Name)
			}
		}
		return out
	}

	// scoredSearch wraps the new scoring approach, returning tool names.
	scoredSearch := func(query string) []string {
		results := scoreTools(query, tools, idf, synMap)
		out := make([]string, len(results))
		for i, r := range results {
			out[i] = r.Tool.Name
		}
		return out
	}

	t.Run("single-tool", func(t *testing.T) {
		oldHits, newHits, total := benchmarkSingleTool(t, singleToolCases, matchesSearch, scoredSearch)
		t.Logf("\n--- Single-Tool Recall@K ---")
		t.Logf("  Old (substring AND): %d/%d (%.0f%%)", oldHits, total, pct(oldHits, total))
		t.Logf("  New (synonym+TF-IDF): %d/%d (%.0f%%)", newHits, total, pct(newHits, total))
		t.Logf("  Delta: %+d", newHits-oldHits)
	})

	t.Run("multi-tool", func(t *testing.T) {
		oldHits, newHits, total := benchmarkMultiTool(t, multiToolCases, matchesSearch, scoredSearch)
		t.Logf("\n--- Multi-Tool Integration Recall ---")
		t.Logf("  Old (substring AND): %d/%d (%.0f%%)", oldHits, total, pct(oldHits, total))
		t.Logf("  New (synonym+TF-IDF): %d/%d (%.0f%%)", newHits, total, pct(newHits, total))
		t.Logf("  Delta: %+d", newHits-oldHits)
	})
}

type searchFunc func(query string) []string

func benchmarkSingleTool(t *testing.T, cases []searchBenchmarkCase, oldSearch, newSearch searchFunc) (oldHits, newHits, total int) {
	t.Helper()

	for _, tc := range cases {
		oldResults := oldSearch(tc.Query)
		newResults := newSearch(tc.Query)

		oldTopK := truncateSlice(oldResults, tc.K)
		newTopK := truncateSlice(newResults, tc.K)

		for _, expected := range tc.ExpectedTools {
			total++
			if sliceContains(oldTopK, expected) {
				oldHits++
			}
			if sliceContains(newTopK, expected) {
				newHits++
			} else {
				t.Logf("  NEW MISS: %s — %q not in top-%d (got %d results)",
					tc.Name, expected, tc.K, len(newResults))
			}
		}
	}
	return
}

func benchmarkMultiTool(t *testing.T, cases []searchBenchmarkCase, oldSearch, newSearch searchFunc) (oldHits, newHits, total int) {
	t.Helper()

	for _, tc := range cases {
		oldResults := oldSearch(tc.Query)
		newResults := newSearch(tc.Query)

		oldIntegrations := toolIntegrations(oldResults)
		newIntegrations := toolIntegrations(newResults)

		for _, expected := range tc.ExpectedIntegrations {
			total++
			if oldIntegrations[expected] {
				oldHits++
			}
			if newIntegrations[expected] {
				newHits++
			} else {
				t.Logf("  NEW MISS: %s — integration %q not in results (got %v)",
					tc.Name, expected, mapKeys(newIntegrations))
			}
		}
	}
	return
}

// toolIntegrations extracts integration names from tool names
// (integration is the prefix before the first underscore).
func toolIntegrations(toolNames []string) map[string]bool {
	m := make(map[string]bool)
	for _, name := range toolNames {
		if idx := strings.Index(name, "_"); idx > 0 {
			m[name[:idx]] = true
		}
	}
	return m
}

func sliceContains(s []string, target string) bool {
	for _, v := range s {
		if v == target {
			return true
		}
	}
	return false
}

func truncateSlice(s []string, n int) []string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func pct(hits, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total) * 100
}
