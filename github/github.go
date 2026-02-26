package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

type integration struct {
	token  string
	client *gh.Client
}

func New() mcp.Integration {
	return &integration{}
}

func (g *integration) Name() string { return "github" }

func (g *integration) Configure(creds mcp.Credentials) error {
	g.token = creds["token"]
	if g.token == "" {
		return fmt.Errorf("github: token is required")
	}
	g.client = gh.NewClient(nil).WithAuthToken(g.token)
	return nil
}

func (g *integration) Healthy(ctx context.Context) bool {
	_, _, err := g.client.Users.Get(ctx, "")
	return err == nil
}

func (g *integration) Tools() []mcp.ToolDefinition {
	return tools
}

func (g *integration) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, g, args)
}

// --- helpers ---

func argStr(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func argInt(args map[string]any, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func argInt64(args map[string]any, key string) int64 {
	switch v := args[key].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	}
	return 0
}

func argBool(args map[string]any, key string) bool {
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}

func argStrSlice(args map[string]any, key string) []string {
	switch v := args[key].(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	case string:
		if v == "" {
			return nil
		}
		return strings.Split(v, ",")
	}
	return nil
}

func jsonResult(v any) (*mcp.ToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
	}
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
}

func listOpts(args map[string]any) gh.ListOptions {
	page := argInt(args, "page")
	perPage := argInt(args, "per_page")
	if page == 0 {
		page = 1
	}
	if perPage == 0 {
		perPage = 30
	}
	return gh.ListOptions{Page: page, PerPage: perPage}
}

type handlerFunc func(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error)

var dispatch = map[string]handlerFunc{
	// Repositories
	"github_search_repos":          searchRepos,
	"github_get_repo":              getRepo,
	"github_list_user_repos":       listUserRepos,
	"github_list_org_repos":        listOrgRepos,
	"github_create_repo":           createRepo,
	"github_delete_repo":           deleteRepo,
	"github_list_branches":         listBranches,
	"github_get_branch":            getBranch,
	"github_list_tags":             listTags,
	"github_list_contributors":     listContributors,
	"github_list_languages":        listLanguages,
	"github_list_topics":           listTopics,
	"github_get_readme":            getReadme,
	"github_get_file_contents":     getFileContents,
	"github_create_update_file":    createOrUpdateFile,
	"github_delete_file":           deleteFile,
	"github_list_forks":            listForks,
	"github_create_fork":           createFork,
	"github_list_collaborators":    listCollaborators,
	"github_list_commit_activity":  listCommitActivity,
	"github_list_repo_teams":       listRepoTeams,
	"github_compare_commits":       compareCommits,
	"github_merge_upstream":        mergeUpstream,
	"github_list_autolinks":        listAutolinks,

	// Issues
	"github_list_issues":           listIssues,
	"github_get_issue":             getIssue,
	"github_create_issue":          createIssue,
	"github_update_issue":          updateIssue,
	"github_list_issue_comments":   listIssueComments,
	"github_create_issue_comment":  createIssueComment,
	"github_list_issue_labels":     listIssueLabels,
	"github_add_issue_labels":      addIssueLabels,
	"github_remove_issue_label":    removeIssueLabel,
	"github_lock_issue":            lockIssue,
	"github_unlock_issue":          unlockIssue,
	"github_list_milestones":       listMilestones,
	"github_create_milestone":      createMilestone,
	"github_list_issue_events":     listIssueEvents,
	"github_list_issue_timeline":   listIssueTimeline,
	"github_list_assignees":        listAssignees,

	// Pull Requests
	"github_list_prs":              listPRs,
	"github_get_pr":                getPR,
	"github_create_pr":             createPR,
	"github_update_pr":             updatePR,
	"github_list_pr_commits":       listPRCommits,
	"github_list_pr_files":         listPRFiles,
	"github_list_pr_reviews":       listPRReviews,
	"github_create_pr_review":      createPRReview,
	"github_list_pr_comments":      listPRComments,
	"github_create_pr_comment":     createPRComment,
	"github_merge_pr":              mergePR,
	"github_list_requested_reviewers": listRequestedReviewers,
	"github_request_reviewers":     requestReviewers,

	// Git (low-level)
	"github_get_commit":            getCommit,
	"github_list_commits":          listCommits,
	"github_get_ref":               getRef,
	"github_create_ref":            createRef,
	"github_delete_ref":            deleteRef,
	"github_get_tree":              getTree,
	"github_create_tag":            createTag,

	// Users
	"github_get_authenticated_user": getAuthenticatedUser,
	"github_get_user":              getUser,
	"github_list_user_followers":   listUserFollowers,
	"github_list_user_following":   listUserFollowing,
	"github_list_user_keys":        listUserKeys,

	// Organizations
	"github_get_org":               getOrg,
	"github_list_user_orgs":        listUserOrgs,
	"github_list_org_members":      listOrgMembers,
	"github_list_org_teams":        listOrgTeams,
	"github_get_team_by_slug":      getTeamBySlug,
	"github_list_team_members":     listTeamMembers,
	"github_list_team_repos":       listTeamRepos,

	// Actions (CI/CD)
	"github_list_workflows":        listWorkflows,
	"github_list_workflow_runs":    listWorkflowRuns,
	"github_get_workflow_run":      getWorkflowRun,
	"github_list_workflow_jobs":    listWorkflowJobs,
	"github_download_workflow_logs": downloadWorkflowLogs,
	"github_rerun_workflow":        rerunWorkflow,
	"github_cancel_workflow_run":   cancelWorkflowRun,
	"github_list_repo_secrets":     listRepoSecrets,
	"github_list_artifacts":        listArtifacts,
	"github_list_environment_secrets": listEnvironmentSecrets,
	"github_list_org_secrets":      listOrgSecrets,

	// Checks
	"github_list_check_runs":       listCheckRuns,
	"github_get_check_run":         getCheckRun,
	"github_list_check_suites":     listCheckSuites,

	// Releases
	"github_list_releases":         listReleases,
	"github_get_release":           getRelease,
	"github_get_latest_release":    getLatestRelease,
	"github_create_release":        createRelease,
	"github_delete_release":        deleteRelease,
	"github_list_release_assets":   listReleaseAssets,

	// Gists
	"github_list_gists":            listGists,
	"github_get_gist":              getGist,
	"github_create_gist":           createGist,

	// Search
	"github_search_code":           searchCode,
	"github_search_issues":         searchIssues,
	"github_search_users":          searchUsers,
	"github_search_commits":        searchCommits,

	// Activity
	"github_list_stargazers":       listStargazers,
	"github_list_watchers":         listWatchers,
	"github_list_notifications":    listNotifications,
	"github_list_repo_events":      listRepoEvents,

	// Code Scanning
	"github_list_code_scanning_alerts":  listCodeScanningAlerts,
	"github_get_code_scanning_alert":    getCodeScanningAlert,

	// Secret Scanning
	"github_list_secret_scanning_alerts": listSecretScanningAlerts,

	// Dependabot
	"github_list_dependabot_alerts": listDependabotAlerts,

	// Copilot
	"github_get_copilot_org_usage": getCopilotOrgUsage,

	// Webhooks
	"github_list_hooks":            listHooks,
	"github_create_hook":           createHook,
	"github_delete_hook":           deleteHook,

	// Deploy Keys
	"github_list_deploy_keys":      listDeployKeys,

	// Rate Limit
	"github_get_rate_limit":        getRateLimit,
}
