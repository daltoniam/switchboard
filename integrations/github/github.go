package github

import (
	"context"
	"errors"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*integration)(nil)
	_ mcp.FieldCompactionIntegration = (*integration)(nil)
)

type integration struct {
	token  string
	client *gh.Client
}

func New() mcp.Integration {
	return &integration{}
}

func (g *integration) Name() string { return "github" }

func (g *integration) Configure(_ context.Context, creds mcp.Credentials) error {
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

func (g *integration) CompactSpec(toolName string) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (g *integration) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, g, args)
}

// --- helpers ---

func wrapRetryable(err error) error {
	if err == nil {
		return nil
	}
	var ghErr *gh.ErrorResponse
	if errors.As(err, &ghErr) && ghErr.Response != nil && (ghErr.Response.StatusCode == 429 || ghErr.Response.StatusCode >= 500) {
		re := &mcp.RetryableError{StatusCode: ghErr.Response.StatusCode, Err: err}
		re.RetryAfter = mcp.ParseRetryAfter(ghErr.Response.Header.Get("Retry-After"))
		return re
	}
	var abuseErr *gh.AbuseRateLimitError
	if errors.As(err, &abuseErr) {
		re := &mcp.RetryableError{StatusCode: 429, Err: err}
		if abuseErr.RetryAfter != nil {
			re.RetryAfter = *abuseErr.RetryAfter
		}
		return re
	}
	var rateErr *gh.RateLimitError
	if errors.As(err, &rateErr) {
		return &mcp.RetryableError{StatusCode: 429, Err: err}
	}
	return err
}

func errResult(err error) (*mcp.ToolResult, error) {
	return mcp.ErrResult(wrapRetryable(err))
}

type handlerFunc func(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error)

var dispatch = map[string]handlerFunc{
	// Repositories
	"github_search_repos":         searchRepos,
	"github_get_repo":             getRepo,
	"github_list_user_repos":      listUserRepos,
	"github_list_org_repos":       listOrgRepos,
	"github_create_repo":          createRepo,
	"github_delete_repo":          deleteRepo,
	"github_list_branches":        listBranches,
	"github_get_branch":           getBranch,
	"github_list_tags":            listTags,
	"github_list_contributors":    listContributors,
	"github_list_languages":       listLanguages,
	"github_list_topics":          listTopics,
	"github_get_readme":           getReadme,
	"github_get_file_contents":    getFileContents,
	"github_create_update_file":   createOrUpdateFile,
	"github_delete_file":          deleteFile,
	"github_list_forks":           listForks,
	"github_create_fork":          createFork,
	"github_list_collaborators":   listCollaborators,
	"github_list_commit_activity": listCommitActivity,
	"github_list_repo_teams":      listRepoTeams,
	"github_compare_commits":      compareCommits,
	"github_merge_upstream":       mergeUpstream,
	"github_list_autolinks":       listAutolinks,

	// Repositories (extended)
	"github_edit_repo":                editRepo,
	"github_replace_topics":           replaceTopics,
	"github_rename_branch":            renameBranch,
	"github_add_collaborator":         addCollaborator,
	"github_remove_collaborator":      removeCollaborator,
	"github_get_combined_status":      getCombinedStatus,
	"github_list_statuses":            listStatuses,
	"github_create_status":            createStatus,
	"github_list_deployments":         listDeployments,
	"github_get_deployment":           getDeployment,
	"github_create_deployment":        createDeployment,
	"github_list_deployment_statuses": listDeploymentStatuses,
	"github_create_deployment_status": createDeploymentStatus,
	"github_list_environments":        listEnvironments,
	"github_get_environment":          getEnvironment,
	"github_get_branch_protection":    getBranchProtection,
	"github_remove_branch_protection": removeBranchProtection,
	"github_list_rulesets":            listRulesets,
	"github_get_ruleset":              getRuleset,
	"github_get_rules_for_branch":     getRulesForBranch,
	"github_list_traffic_views":       listTrafficViews,
	"github_list_traffic_clones":      listTrafficClones,
	"github_list_traffic_referrers":   listTrafficReferrers,
	"github_list_traffic_paths":       listTrafficPaths,
	"github_get_community_health":     getCommunityHealth,
	"github_dispatch_event":           dispatchEvent,
	"github_merge_branch":             mergeBranch,
	"github_edit_release":             editRelease,
	"github_generate_release_notes":   generateReleaseNotes,
	"github_list_commit_comments":     listCommitComments,
	"github_create_commit_comment":    createCommitComment,

	// Issues
	"github_list_issues":          listIssues,
	"github_get_issue":            getIssue,
	"github_create_issue":         createIssue,
	"github_update_issue":         updateIssue,
	"github_list_issue_comments":  listIssueComments,
	"github_create_issue_comment": createIssueComment,
	"github_list_issue_labels":    listIssueLabels,
	"github_add_issue_labels":     addIssueLabels,
	"github_remove_issue_label":   removeIssueLabel,
	"github_lock_issue":           lockIssue,
	"github_unlock_issue":         unlockIssue,
	"github_list_milestones":      listMilestones,
	"github_create_milestone":     createMilestone,
	"github_list_issue_events":    listIssueEvents,
	"github_list_issue_timeline":  listIssueTimeline,
	"github_list_assignees":       listAssignees,

	// Issues (extended)
	"github_update_issue_comment":          updateIssueComment,
	"github_delete_issue_comment":          deleteIssueComment,
	"github_update_milestone":              updateMilestone,
	"github_delete_milestone":              deleteMilestone,
	"github_list_labels":                   listLabels,
	"github_create_label":                  createLabel,
	"github_edit_label":                    editLabel,
	"github_delete_label":                  deleteLabel,
	"github_create_issue_reaction":         createIssueReaction,
	"github_create_issue_comment_reaction": createIssueCommentReaction,
	"github_list_issue_reactions":          listIssueReactions,

	// Pull Requests
	"github_list_pulls":               listPRs,
	"github_get_pull":                 getPR,
	"github_get_pull_diff":            getPRDiff,
	"github_create_pull":              createPR,
	"github_update_pull":              updatePR,
	"github_list_pull_commits":        listPRCommits,
	"github_list_pull_files":          listPRFiles,
	"github_list_pull_reviews":        listPRReviews,
	"github_create_pull_review":       createPRReview,
	"github_list_pull_comments":       listPRComments,
	"github_create_pull_comment":      createPRComment,
	"github_merge_pull":               mergePR,
	"github_list_requested_reviewers": listRequestedReviewers,
	"github_request_reviewers":        requestReviewers,

	// Pull Requests (extended)
	"github_dismiss_pull_review":    dismissPullReview,
	"github_update_pull_branch":     updatePullBranch,
	"github_remove_reviewers":       removeReviewers,
	"github_list_pulls_with_commit": listPullsWithCommit,

	// Git (low-level)
	"github_get_commit":   getCommit,
	"github_list_commits": listCommits,
	"github_get_ref":      getRef,
	"github_create_ref":   createRef,
	"github_delete_ref":   deleteRef,
	"github_get_tree":     getTree,
	"github_create_tag":   createTag,

	// Users
	"github_get_authenticated_user": getAuthenticatedUser,
	"github_get_user":               getUser,
	"github_list_user_followers":    listUserFollowers,
	"github_list_user_following":    listUserFollowing,
	"github_list_user_keys":         listUserKeys,

	// Organizations
	"github_get_org":           getOrg,
	"github_list_user_orgs":    listUserOrgs,
	"github_list_org_members":  listOrgMembers,
	"github_list_org_teams":    listOrgTeams,
	"github_get_team_by_slug":  getTeamBySlug,
	"github_list_team_members": listTeamMembers,
	"github_list_team_repos":   listTeamRepos,

	// Teams/Orgs (extended)
	"github_create_team":                  createTeam,
	"github_edit_team":                    editTeam,
	"github_delete_team":                  deleteTeam,
	"github_add_team_member":              addTeamMember,
	"github_remove_team_member":           removeTeamMember,
	"github_add_team_repo":                addTeamRepo,
	"github_remove_team_repo":             removeTeamRepo,
	"github_list_pending_org_invitations": listPendingOrgInvitations,
	"github_list_outside_collaborators":   listOutsideCollaborators,

	// Actions (CI/CD)
	"github_list_workflows":           listWorkflows,
	"github_list_workflow_runs":       listWorkflowRuns,
	"github_get_workflow_run":         getWorkflowRun,
	"github_list_workflow_jobs":       listWorkflowJobs,
	"github_download_workflow_logs":   downloadWorkflowLogs,
	"github_rerun_workflow":           rerunWorkflow,
	"github_cancel_workflow_run":      cancelWorkflowRun,
	"github_list_repo_secrets":        listRepoSecrets,
	"github_list_artifacts":           listArtifacts,
	"github_list_environment_secrets": listEnvironmentSecrets,
	"github_list_org_secrets":         listOrgSecrets,

	// Actions (extended)
	"github_trigger_workflow":      triggerWorkflow,
	"github_rerun_failed_jobs":     rerunFailedJobs,
	"github_get_workflow_job":      getWorkflowJob,
	"github_get_workflow_job_logs": getWorkflowJobLogs,
	"github_delete_workflow_run":   deleteWorkflowRun,
	"github_list_repo_variables":   listRepoVariables,
	"github_create_repo_variable":  createRepoVariable,
	"github_update_repo_variable":  updateRepoVariable,
	"github_delete_repo_variable":  deleteRepoVariable,
	"github_list_org_variables":    listOrgVariables,
	"github_list_env_variables":    listEnvVariables,
	"github_list_runners":          listRunners,
	"github_list_org_runners":      listOrgRunners,

	// Checks
	"github_list_check_runs":   listCheckRuns,
	"github_get_check_run":     getCheckRun,
	"github_list_check_suites": listCheckSuites,

	// Releases
	"github_list_releases":       listReleases,
	"github_get_release":         getRelease,
	"github_get_latest_release":  getLatestRelease,
	"github_create_release":      createRelease,
	"github_delete_release":      deleteRelease,
	"github_list_release_assets": listReleaseAssets,

	// Gists
	"github_list_gists":  listGists,
	"github_get_gist":    getGist,
	"github_create_gist": createGist,

	// Search
	"github_search_code":    searchCode,
	"github_search_issues":  searchIssues,
	"github_search_users":   searchUsers,
	"github_search_commits": searchCommits,

	// Search (extended)
	"github_search_topics": searchTopics,
	"github_search_labels": searchLabels,

	// Activity
	"github_list_stargazers":    listStargazers,
	"github_list_watchers":      listWatchers,
	"github_list_notifications": listNotifications,
	"github_list_repo_events":   listRepoEvents,

	// Activity (extended)
	"github_mark_notifications_read": markNotificationsRead,
	"github_star_repo":               starRepo,
	"github_unstar_repo":             unstarRepo,
	"github_list_starred":            listStarred,

	// Code Scanning
	"github_list_code_scanning_alerts": listCodeScanningAlerts,
	"github_get_code_scanning_alert":   getCodeScanningAlert,

	// Secret Scanning
	"github_list_secret_scanning_alerts": listSecretScanningAlerts,

	// Secret Scanning (extended)
	"github_get_secret_scanning_alert": getSecretScanningAlert,

	// Dependabot
	"github_list_dependabot_alerts": listDependabotAlerts,

	// Dependabot (extended)
	"github_get_dependabot_alert": getDependabotAlert,

	// Code Scanning (extended)
	"github_list_code_scanning_analyses": listCodeScanningAnalyses,

	// SBOM
	"github_get_sbom": getSBOM,

	// Copilot
	"github_get_copilot_org_usage": getCopilotOrgUsage,

	// Webhooks
	"github_list_hooks":  listHooks,
	"github_create_hook": createHook,
	"github_delete_hook": deleteHook,

	// Deploy Keys
	"github_list_deploy_keys": listDeployKeys,

	// Rate Limit
	"github_get_rate_limit": getRateLimit,
}
