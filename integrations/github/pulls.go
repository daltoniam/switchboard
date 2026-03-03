package github

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

func listPRs(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.PullRequestListOptions{
		State:       argStr(args, "state"),
		Head:        argStr(args, "head"),
		Base:        argStr(args, "base"),
		Sort:        argStr(args, "sort"),
		Direction:   argStr(args, "direction"),
		ListOptions: listOpts(args),
	}
	prs, _, err := g.client.PullRequests.List(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(prs)
}

func getPR(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	pr, _, err := g.client.PullRequests.Get(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(pr)
}

func createPR(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	pr := &gh.NewPullRequest{
		Title: gh.Ptr(argStr(args, "title")),
		Head:  gh.Ptr(argStr(args, "head")),
		Base:  gh.Ptr(argStr(args, "base")),
		Body:  gh.Ptr(argStr(args, "body")),
		Draft: gh.Ptr(argBool(args, "draft")),
	}
	pull, _, err := g.client.PullRequests.Create(ctx, argStr(args, "owner"), argStr(args, "repo"), pr)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(pull)
}

func updatePR(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	pr := &gh.PullRequest{}
	if v := argStr(args, "title"); v != "" {
		pr.Title = gh.Ptr(v)
	}
	if v := argStr(args, "body"); v != "" {
		pr.Body = gh.Ptr(v)
	}
	if v := argStr(args, "state"); v != "" {
		pr.State = gh.Ptr(v)
	}
	if v := argStr(args, "base"); v != "" {
		pr.Base = &gh.PullRequestBranch{Ref: gh.Ptr(v)}
	}
	pull, _, err := g.client.PullRequests.Edit(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), pr)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(pull)
}

func listPRCommits(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	commits, _, err := g.client.PullRequests.ListCommits(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(commits)
}

func listPRFiles(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	files, _, err := g.client.PullRequests.ListFiles(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(files)
}

func listPRReviews(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	reviews, _, err := g.client.PullRequests.ListReviews(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(reviews)
}

func createPRReview(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	review := &gh.PullRequestReviewRequest{
		Body:  gh.Ptr(argStr(args, "body")),
		Event: gh.Ptr(argStr(args, "event")),
	}
	r, _, err := g.client.PullRequests.CreateReview(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), review)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(r)
}

func listPRComments(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.PullRequestListCommentsOptions{ListOptions: listOpts(args)}
	comments, _, err := g.client.PullRequests.ListComments(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(comments)
}

func createPRComment(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	comment := &gh.PullRequestComment{
		Body:     gh.Ptr(argStr(args, "body")),
		CommitID: gh.Ptr(argStr(args, "commit_id")),
		Path:     gh.Ptr(argStr(args, "path")),
	}
	if line := argInt(args, "line"); line > 0 {
		comment.Line = gh.Ptr(line)
	}
	c, _, err := g.client.PullRequests.CreateComment(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), comment)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(c)
}

func mergePR(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.PullRequestOptions{}
	if method := argStr(args, "merge_method"); method != "" {
		opts.MergeMethod = method
	}
	result, _, err := g.client.PullRequests.Merge(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), argStr(args, "commit_message"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(result)
}

func listRequestedReviewers(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	reviewers, _, err := g.client.PullRequests.ListReviewers(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), nil)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(reviewers)
}

func requestReviewers(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := gh.ReviewersRequest{
		Reviewers:     argStrSlice(args, "reviewers"),
		TeamReviewers: argStrSlice(args, "team_reviewers"),
	}
	pr, _, err := g.client.PullRequests.RequestReviewers(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), req)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(pr)
}
