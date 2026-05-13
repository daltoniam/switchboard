package github

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	gh "github.com/google/go-github/v68/github"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("github", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*integration)(nil)
	_ mcp.FieldCompactionIntegration = (*integration)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*integration)(nil)
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

func (g *integration) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (g *integration) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (g *integration) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
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

var dispatch = map[mcp.ToolName]handlerFunc{
	// Repositories
	mcp.ToolName("github_search_repos"):         searchRepos,
	mcp.ToolName("github_get_repo"):             getRepo,
	mcp.ToolName("github_list_user_repos"):      listUserRepos,
	mcp.ToolName("github_list_org_repos"):       listOrgRepos,
	mcp.ToolName("github_create_repo"):          createRepo,
	mcp.ToolName("github_delete_repo"):          deleteRepo,
	mcp.ToolName("github_list_branches"):        listBranches,
	mcp.ToolName("github_get_branch"):           getBranch,
	mcp.ToolName("github_list_tags"):            listTags,
	mcp.ToolName("github_list_contributors"):    listContributors,
	mcp.ToolName("github_list_languages"):       listLanguages,
	mcp.ToolName("github_list_topics"):          listTopics,
	mcp.ToolName("github_get_readme"):           getReadme,
	mcp.ToolName("github_get_file_contents"):    getFileContents,
	mcp.ToolName("github_create_update_file"):   createOrUpdateFile,
	mcp.ToolName("github_delete_file"):          deleteFile,
	mcp.ToolName("github_list_forks"):           listForks,
	mcp.ToolName("github_create_fork"):          createFork,
	mcp.ToolName("github_list_collaborators"):   listCollaborators,
	mcp.ToolName("github_list_commit_activity"): listCommitActivity,
	mcp.ToolName("github_list_repo_teams"):      listRepoTeams,
	mcp.ToolName("github_compare_commits"):      compareCommits,
	mcp.ToolName("github_merge_upstream"):       mergeUpstream,
	mcp.ToolName("github_list_autolinks"):       listAutolinks,

	// Repositories (extended)
	mcp.ToolName("github_edit_repo"):                editRepo,
	mcp.ToolName("github_replace_topics"):           replaceTopics,
	mcp.ToolName("github_rename_branch"):            renameBranch,
	mcp.ToolName("github_add_collaborator"):         addCollaborator,
	mcp.ToolName("github_remove_collaborator"):      removeCollaborator,
	mcp.ToolName("github_get_combined_status"):      getCombinedStatus,
	mcp.ToolName("github_list_statuses"):            listStatuses,
	mcp.ToolName("github_create_status"):            createStatus,
	mcp.ToolName("github_list_deployments"):         listDeployments,
	mcp.ToolName("github_get_deployment"):           getDeployment,
	mcp.ToolName("github_create_deployment"):        createDeployment,
	mcp.ToolName("github_list_deployment_statuses"): listDeploymentStatuses,
	mcp.ToolName("github_create_deployment_status"): createDeploymentStatus,
	mcp.ToolName("github_list_environments"):        listEnvironments,
	mcp.ToolName("github_get_environment"):          getEnvironment,
	mcp.ToolName("github_get_branch_protection"):    getBranchProtection,
	mcp.ToolName("github_remove_branch_protection"): removeBranchProtection,
	mcp.ToolName("github_list_rulesets"):            listRulesets,
	mcp.ToolName("github_get_ruleset"):              getRuleset,
	mcp.ToolName("github_get_rules_for_branch"):     getRulesForBranch,
	mcp.ToolName("github_list_traffic_views"):       listTrafficViews,
	mcp.ToolName("github_list_traffic_clones"):      listTrafficClones,
	mcp.ToolName("github_list_traffic_referrers"):   listTrafficReferrers,
	mcp.ToolName("github_list_traffic_paths"):       listTrafficPaths,
	mcp.ToolName("github_get_community_health"):     getCommunityHealth,
	mcp.ToolName("github_dispatch_event"):           dispatchEvent,
	mcp.ToolName("github_merge_branch"):             mergeBranch,
	mcp.ToolName("github_edit_release"):             editRelease,
	mcp.ToolName("github_generate_release_notes"):   generateReleaseNotes,
	mcp.ToolName("github_list_commit_comments"):     listCommitComments,
	mcp.ToolName("github_create_commit_comment"):    createCommitComment,

	// Issues
	mcp.ToolName("github_list_issues"):          listIssues,
	mcp.ToolName("github_get_issue"):            getIssue,
	mcp.ToolName("github_create_issue"):         createIssue,
	mcp.ToolName("github_update_issue"):         updateIssue,
	mcp.ToolName("github_list_issue_comments"):  listIssueComments,
	mcp.ToolName("github_create_issue_comment"): createIssueComment,
	mcp.ToolName("github_list_issue_labels"):    listIssueLabels,
	mcp.ToolName("github_add_issue_labels"):     addIssueLabels,
	mcp.ToolName("github_remove_issue_label"):   removeIssueLabel,
	mcp.ToolName("github_lock_issue"):           lockIssue,
	mcp.ToolName("github_unlock_issue"):         unlockIssue,
	mcp.ToolName("github_list_milestones"):      listMilestones,
	mcp.ToolName("github_create_milestone"):     createMilestone,
	mcp.ToolName("github_list_issue_events"):    listIssueEvents,
	mcp.ToolName("github_list_issue_timeline"):  listIssueTimeline,
	mcp.ToolName("github_list_assignees"):       listAssignees,

	// Issues (extended)
	mcp.ToolName("github_update_issue_comment"):          updateIssueComment,
	mcp.ToolName("github_delete_issue_comment"):          deleteIssueComment,
	mcp.ToolName("github_update_milestone"):              updateMilestone,
	mcp.ToolName("github_delete_milestone"):              deleteMilestone,
	mcp.ToolName("github_list_labels"):                   listLabels,
	mcp.ToolName("github_create_label"):                  createLabel,
	mcp.ToolName("github_edit_label"):                    editLabel,
	mcp.ToolName("github_delete_label"):                  deleteLabel,
	mcp.ToolName("github_create_issue_reaction"):         createIssueReaction,
	mcp.ToolName("github_create_issue_comment_reaction"): createIssueCommentReaction,
	mcp.ToolName("github_list_issue_reactions"):          listIssueReactions,

	// Pull Requests
	mcp.ToolName("github_list_pulls"):               listPRs,
	mcp.ToolName("github_get_pull"):                 getPR,
	mcp.ToolName("github_get_pull_diff"):            getPRDiff,
	mcp.ToolName("github_create_pull"):              createPR,
	mcp.ToolName("github_update_pull"):              updatePR,
	mcp.ToolName("github_list_pull_commits"):        listPRCommits,
	mcp.ToolName("github_list_pull_files"):          listPRFiles,
	mcp.ToolName("github_list_pull_reviews"):        listPRReviews,
	mcp.ToolName("github_create_pull_review"):       createPRReview,
	mcp.ToolName("github_list_pull_comments"):       listPRComments,
	mcp.ToolName("github_create_pull_comment"):      createPRComment,
	mcp.ToolName("github_get_pull_comment"):         getPRComment,
	mcp.ToolName("github_reply_to_pull_comment"):    replyToPRComment,
	mcp.ToolName("github_update_pull_comment"):      updatePRComment,
	mcp.ToolName("github_delete_pull_comment"):      deletePRComment,
	mcp.ToolName("github_merge_pull"):               mergePR,
	mcp.ToolName("github_list_requested_reviewers"): listRequestedReviewers,
	mcp.ToolName("github_request_reviewers"):        requestReviewers,

	// Pull Requests (extended)
	mcp.ToolName("github_dismiss_pull_review"):    dismissPullReview,
	mcp.ToolName("github_update_pull_branch"):     updatePullBranch,
	mcp.ToolName("github_remove_reviewers"):       removeReviewers,
	mcp.ToolName("github_list_pulls_with_commit"): listPullsWithCommit,

	// Git (low-level)
	mcp.ToolName("github_get_commit"):   getCommit,
	mcp.ToolName("github_list_commits"): listCommits,
	mcp.ToolName("github_get_ref"):      getRef,
	mcp.ToolName("github_create_ref"):   createRef,
	mcp.ToolName("github_delete_ref"):   deleteRef,
	mcp.ToolName("github_get_tree"):     getTree,
	mcp.ToolName("github_create_tag"):   createTag,

	// Users
	mcp.ToolName("github_get_authenticated_user"): getAuthenticatedUser,
	mcp.ToolName("github_get_user"):               getUser,
	mcp.ToolName("github_list_user_followers"):    listUserFollowers,
	mcp.ToolName("github_list_user_following"):    listUserFollowing,
	mcp.ToolName("github_list_user_keys"):         listUserKeys,

	// Organizations
	mcp.ToolName("github_get_org"):           getOrg,
	mcp.ToolName("github_list_user_orgs"):    listUserOrgs,
	mcp.ToolName("github_list_org_members"):  listOrgMembers,
	mcp.ToolName("github_list_org_teams"):    listOrgTeams,
	mcp.ToolName("github_get_team_by_slug"):  getTeamBySlug,
	mcp.ToolName("github_list_team_members"): listTeamMembers,
	mcp.ToolName("github_list_team_repos"):   listTeamRepos,

	// Teams/Orgs (extended)
	mcp.ToolName("github_create_team"):                  createTeam,
	mcp.ToolName("github_edit_team"):                    editTeam,
	mcp.ToolName("github_delete_team"):                  deleteTeam,
	mcp.ToolName("github_add_team_member"):              addTeamMember,
	mcp.ToolName("github_remove_team_member"):           removeTeamMember,
	mcp.ToolName("github_add_team_repo"):                addTeamRepo,
	mcp.ToolName("github_remove_team_repo"):             removeTeamRepo,
	mcp.ToolName("github_list_pending_org_invitations"): listPendingOrgInvitations,
	mcp.ToolName("github_list_outside_collaborators"):   listOutsideCollaborators,

	// Actions (CI/CD)
	mcp.ToolName("github_list_workflows"):           listWorkflows,
	mcp.ToolName("github_list_workflow_runs"):       listWorkflowRuns,
	mcp.ToolName("github_get_workflow_run"):         getWorkflowRun,
	mcp.ToolName("github_list_workflow_jobs"):       listWorkflowJobs,
	mcp.ToolName("github_download_workflow_logs"):   downloadWorkflowLogs,
	mcp.ToolName("github_rerun_workflow"):           rerunWorkflow,
	mcp.ToolName("github_cancel_workflow_run"):      cancelWorkflowRun,
	mcp.ToolName("github_list_repo_secrets"):        listRepoSecrets,
	mcp.ToolName("github_list_artifacts"):           listArtifacts,
	mcp.ToolName("github_list_environment_secrets"): listEnvironmentSecrets,
	mcp.ToolName("github_list_org_secrets"):         listOrgSecrets,

	// Actions (extended)
	mcp.ToolName("github_trigger_workflow"):      triggerWorkflow,
	mcp.ToolName("github_rerun_failed_jobs"):     rerunFailedJobs,
	mcp.ToolName("github_get_workflow_job"):      getWorkflowJob,
	mcp.ToolName("github_get_workflow_job_logs"): getWorkflowJobLogs,
	mcp.ToolName("github_delete_workflow_run"):   deleteWorkflowRun,
	mcp.ToolName("github_list_repo_variables"):   listRepoVariables,
	mcp.ToolName("github_create_repo_variable"):  createRepoVariable,
	mcp.ToolName("github_update_repo_variable"):  updateRepoVariable,
	mcp.ToolName("github_delete_repo_variable"):  deleteRepoVariable,
	mcp.ToolName("github_list_org_variables"):    listOrgVariables,
	mcp.ToolName("github_list_env_variables"):    listEnvVariables,
	mcp.ToolName("github_list_runners"):          listRunners,
	mcp.ToolName("github_list_org_runners"):      listOrgRunners,

	// Checks
	mcp.ToolName("github_list_check_runs"):   listCheckRuns,
	mcp.ToolName("github_get_check_run"):     getCheckRun,
	mcp.ToolName("github_list_check_suites"): listCheckSuites,

	// Releases
	mcp.ToolName("github_list_releases"):       listReleases,
	mcp.ToolName("github_get_release"):         getRelease,
	mcp.ToolName("github_get_latest_release"):  getLatestRelease,
	mcp.ToolName("github_create_release"):      createRelease,
	mcp.ToolName("github_delete_release"):      deleteRelease,
	mcp.ToolName("github_list_release_assets"): listReleaseAssets,

	// Gists
	mcp.ToolName("github_list_gists"):  listGists,
	mcp.ToolName("github_get_gist"):    getGist,
	mcp.ToolName("github_create_gist"): createGist,

	// Search
	mcp.ToolName("github_search_code"):    searchCode,
	mcp.ToolName("github_search_issues"):  searchIssues,
	mcp.ToolName("github_search_users"):   searchUsers,
	mcp.ToolName("github_search_commits"): searchCommits,

	// Search (extended)
	mcp.ToolName("github_search_topics"): searchTopics,
	mcp.ToolName("github_search_labels"): searchLabels,

	// Activity
	mcp.ToolName("github_list_stargazers"):    listStargazers,
	mcp.ToolName("github_list_watchers"):      listWatchers,
	mcp.ToolName("github_list_notifications"): listNotifications,
	mcp.ToolName("github_list_repo_events"):   listRepoEvents,

	// Activity (extended)
	mcp.ToolName("github_mark_notifications_read"): markNotificationsRead,
	mcp.ToolName("github_star_repo"):               starRepo,
	mcp.ToolName("github_unstar_repo"):             unstarRepo,
	mcp.ToolName("github_list_starred"):            listStarred,

	// Code Scanning
	mcp.ToolName("github_list_code_scanning_alerts"): listCodeScanningAlerts,
	mcp.ToolName("github_get_code_scanning_alert"):   getCodeScanningAlert,

	// Secret Scanning
	mcp.ToolName("github_list_secret_scanning_alerts"): listSecretScanningAlerts,

	// Secret Scanning (extended)
	mcp.ToolName("github_get_secret_scanning_alert"): getSecretScanningAlert,

	// Dependabot
	mcp.ToolName("github_list_dependabot_alerts"): listDependabotAlerts,

	// Dependabot (extended)
	mcp.ToolName("github_get_dependabot_alert"): getDependabotAlert,

	// Code Scanning (extended)
	mcp.ToolName("github_list_code_scanning_analyses"): listCodeScanningAnalyses,

	// SBOM
	mcp.ToolName("github_get_sbom"): getSBOM,

	// Copilot
	mcp.ToolName("github_get_copilot_org_usage"): getCopilotOrgUsage,

	// Webhooks
	mcp.ToolName("github_list_hooks"):  listHooks,
	mcp.ToolName("github_create_hook"): createHook,
	mcp.ToolName("github_delete_hook"): deleteHook,

	// Deploy Keys
	mcp.ToolName("github_list_deploy_keys"): listDeployKeys,

	// Rate Limit
	mcp.ToolName("github_get_rate_limit"): getRateLimit,
}
