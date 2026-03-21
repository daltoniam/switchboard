package github

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

// ── PR Extended ───────────────────────────────────────────────────

func dismissPullReview(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	pull := r.Int("pull_number")
	reviewID := r.Int64("review_id")
	message := r.Str("message")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	req := &gh.PullRequestReviewDismissalRequest{
		Message: gh.Ptr(message),
	}
	review, _, err := g.client.PullRequests.DismissReview(ctx, owner, repo, pull, reviewID, req)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(review)
}

func updatePullBranch(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	pull := r.Int("pull_number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.PullRequestBranchUpdateOptions{}
	if sha, err := mcp.ArgStr(args, "expected_head_sha"); err != nil {
		return mcp.ErrResult(err)
	} else if sha != "" {
		opts.ExpectedHeadSHA = gh.Ptr(sha)
	}
	result, _, err := g.client.PullRequests.UpdateBranch(ctx, owner, repo, pull, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(result)
}

func removeReviewers(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	pull := r.Int("pull_number")
	reviewers := r.StrSlice("reviewers")
	teamReviewers := r.StrSlice("team_reviewers")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	req := gh.ReviewersRequest{
		Reviewers:     reviewers,
		TeamReviewers: teamReviewers,
	}
	_, err := g.client.PullRequests.RemoveReviewers(ctx, owner, repo, pull, req)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "removed"})
}

func listPullsWithCommit(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	sha := r.Str("sha")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: page, PerPage: perPage}
	prs, _, err := g.client.PullRequests.ListPullRequestsWithCommit(ctx, owner, repo, sha, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(prs)
}
