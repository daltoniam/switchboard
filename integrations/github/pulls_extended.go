package github

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

// ── PR Extended ───────────────────────────────────────────────────

func dismissPullReview(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := &gh.PullRequestReviewDismissalRequest{
		Message: gh.Ptr(argStr(args, "message")),
	}
	review, _, err := g.client.PullRequests.DismissReview(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "pull_number"), argInt64(args, "review_id"), req)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(review)
}

func updatePullBranch(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.PullRequestBranchUpdateOptions{}
	if sha := argStr(args, "expected_head_sha"); sha != "" {
		opts.ExpectedHeadSHA = gh.Ptr(sha)
	}
	result, _, err := g.client.PullRequests.UpdateBranch(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "pull_number"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(result)
}

func removeReviewers(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := gh.ReviewersRequest{
		Reviewers:     argStrSlice(args, "reviewers"),
		TeamReviewers: argStrSlice(args, "team_reviewers"),
	}
	_, err := g.client.PullRequests.RemoveReviewers(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "pull_number"), req)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "removed"})
}

func listPullsWithCommit(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	prs, _, err := g.client.PullRequests.ListPullRequestsWithCommit(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "sha"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(prs)
}
