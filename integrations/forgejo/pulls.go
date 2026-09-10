package forgejo

import (
	sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	mcp "github.com/daltoniam/switchboard"
)

func listPulls(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo := r.Str("owner"), r.Str("repo")
	opts := sdk.ListPullRequestsOptions{ListOptions: pagination(r), State: sdk.StateType(r.Str("state")), Sort: r.Str("sort")}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if opts.State == "" {
		opts.State = sdk.StateOpen
	}
	return finish(c.ListRepoPullRequests(owner, repo, opts))
}

func getPull(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo, number := r.Str("owner"), r.Str("repo"), r.Int64("number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return finish(c.GetPullRequest(owner, repo, number))
}

func createPull(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo := r.Str("owner"), r.Str("repo")
	opts := sdk.CreatePullRequestOption{Title: r.Str("title"), Head: r.Str("head"), Base: r.Str("base"), Body: r.Str("body"), Assignees: r.StrSlice("assignees"), Milestone: r.Int64("milestone")}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	labels, err := labelIDs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	opts.Labels = labels
	return finish(c.CreatePullRequest(owner, repo, opts))
}

func updatePull(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo, number := r.Str("owner"), r.Str("repo"), r.Int64("number")
	body, state, allow := r.Str("body"), sdk.StateType(r.Str("state")), r.Bool("allow_maintainer_edit")
	opts := sdk.EditPullRequestOption{Title: r.Str("title"), Base: r.Str("base"), Assignees: r.StrSlice("assignees")}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	labels, err := labelIDs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	opts.Labels = labels
	if _, ok := args["body"]; ok {
		opts.Body = &body
	}
	if _, ok := args["state"]; ok {
		opts.State = &state
	}
	if _, ok := args["allow_maintainer_edit"]; ok {
		opts.AllowMaintainerEdit = &allow
	}
	return finish(c.EditPullRequest(owner, repo, number, opts))
}

func getPullDiff(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo, number := r.Str("owner"), r.Str("repo"), r.Int64("number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, resp, err := c.GetPullRequestDiff(owner, repo, number, sdk.PullRequestDiffOptions{})
	return finish(map[string]string{"diff": string(data)}, resp, err)
}

func listPullFiles(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo, number := r.Str("owner"), r.Str("repo"), r.Int64("number")
	opts := sdk.ListPullRequestFilesOptions{ListOptions: pagination(r)}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return finish(c.ListPullRequestFiles(owner, repo, number, opts))
}

func listPullReviews(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo, number := r.Str("owner"), r.Str("repo"), r.Int64("number")
	opts := sdk.ListPullReviewsOptions{ListOptions: pagination(r)}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return finish(c.ListPullReviews(owner, repo, number, opts))
}

func createPullReview(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo, number := r.Str("owner"), r.Str("repo"), r.Int64("number")
	opts := sdk.CreatePullReviewOptions{State: sdk.ReviewStateType(r.Str("event")), Body: r.Str("body"), CommitID: r.Str("commit_id")}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return finish(c.CreatePullReview(owner, repo, number, opts))
}

func mergePull(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo, number := r.Str("owner"), r.Str("repo"), r.Int64("number")
	opts := sdk.MergePullRequestOption{Style: sdk.MergeStyle(r.Str("merge_method")), HeadCommitId: r.Str("sha"), Title: r.Str("commit_title"), Message: r.Str("commit_message"), DeleteBranchAfterMerge: r.Bool("delete_branch_after_merge")}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if opts.Style == "" {
		opts.Style = sdk.MergeStyleMerge
	}
	merged, resp, err := c.MergePullRequest(owner, repo, number, opts)
	return finish(map[string]bool{"merged": merged}, resp, err)
}
