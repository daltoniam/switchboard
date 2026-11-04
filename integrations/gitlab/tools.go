package gitlab

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name:        "gitlab_list_projects",
		Description: "List GitLab projects and repositories accessible to the authenticated user. Start here to discover project_id values (numeric ID or namespace/project path) for merge requests, issues, and CI/CD tools.",
		Parameters: map[string]string{
			"membership": "When true, only projects the user is a member of (default true)",
			"search":     "Optional search query matched against project name",
			"page":       "Page number (default 1)",
			"per_page":   "Results per page, 1-100 (default 20)",
		},
	},
	{
		Name:        "gitlab_get_project",
		Description: "Get GitLab project metadata, default branch, and web URL. Use after list_projects with the same project_id.",
		Parameters: map[string]string{
			"project_id": "Project ID or URL-encoded path (e.g. gitlab-org/gitlab or 12345)",
		},
		Required: []string{"project_id"},
	},
	{
		Name:        "gitlab_get_current_user",
		Description: "Get the authenticated GitLab user account. Use to verify personal access token identity before repository actions.",
		Parameters:  map[string]string{},
	},
	{
		Name:        "gitlab_list_merge_requests",
		Description: "List GitLab merge requests for code review and CI triage. Start here for MR workflows; use project_id from list_projects and merge_request_iid from results for get_merge_request and comments.",
		Parameters: map[string]string{
			"project_id": "Project ID or namespace/project path",
			"state":      "Filter: opened, closed, locked, merged, or all (default opened)",
			"scope":      "Optional scope: created_by_me, assigned_to_me, all",
			"search":     "Search MR title and description",
			"page":       "Page number (default 1)",
			"per_page":   "Results per page, 1-100 (default 20)",
		},
		Required: []string{"project_id"},
	},
	{
		Name:        "gitlab_get_merge_request",
		Description: "Get merge request details, branches, merge status, and pipeline summary. Use after list_merge_requests with merge_request_iid.",
		Parameters: map[string]string{
			"project_id":        "Project ID or namespace/project path",
			"merge_request_iid": "Project-local merge request IID (not global id)",
		},
		Required: []string{"project_id", "merge_request_iid"},
	},
	{
		Name:        "gitlab_create_merge_request_note",
		Description: "Post a comment on a GitLab merge request. Use after get_merge_request when leaving review feedback.",
		Parameters: map[string]string{
			"project_id":        "Project ID or namespace/project path",
			"merge_request_iid": "Project-local merge request IID",
			"body":              "Comment body (Markdown)",
		},
		Required: []string{"project_id", "merge_request_iid", "body"},
	},
	{
		Name:        "gitlab_list_issues",
		Description: "List GitLab issues, bugs, and tasks in a project (excludes merge requests). Use project_id from list_projects; follow up with get_issue using issue_iid.",
		Parameters: map[string]string{
			"project_id": "Project ID or namespace/project path",
			"state":      "opened, closed, or all (default opened)",
			"labels":     "Comma-separated label names",
			"search":     "Search issue title and description",
			"page":       "Page number (default 1)",
			"per_page":   "Results per page, 1-100 (default 20)",
		},
		Required: []string{"project_id"},
	},
	{
		Name:        "gitlab_get_issue",
		Description: "Get a GitLab issue description, state, and assignees. Use after list_issues with issue_iid.",
		Parameters: map[string]string{
			"project_id": "Project ID or namespace/project path",
			"issue_iid":  "Project-local issue IID",
		},
		Required: []string{"project_id", "issue_iid"},
	},
	{
		Name:        "gitlab_list_pipelines",
		Description: "List GitLab CI/CD pipelines for a project or branch ref. Use to find pipeline id before listing jobs or reading failed job logs.",
		Parameters: map[string]string{
			"project_id": "Project ID or namespace/project path",
			"ref":        "Optional branch or tag name",
			"status":     "Optional status filter (failed, success, running, etc.)",
			"page":       "Page number (default 1)",
			"per_page":   "Results per page, 1-100 (default 20)",
		},
		Required: []string{"project_id"},
	},
	{
		Name:        "gitlab_list_pipeline_jobs",
		Description: "List CI/CD jobs in a GitLab pipeline. Use job id with get_job_trace to fetch failed job logs.",
		Parameters: map[string]string{
			"project_id":  "Project ID or namespace/project path",
			"pipeline_id": "Pipeline ID from list_pipelines",
			"page":        "Page number (default 1)",
			"per_page":    "Results per page, 1-100 (default 20)",
		},
		Required: []string{"project_id", "pipeline_id"},
	},
	{
		Name:        "gitlab_get_job_trace",
		Description: "Get the CI/CD job log trace for debugging failed pipelines. Use after list_pipeline_jobs with job_id from a failed job.",
		Parameters: map[string]string{
			"project_id": "Project ID or namespace/project path",
			"job_id":     "CI job ID",
		},
		Required: []string{"project_id", "job_id"},
	},
}
