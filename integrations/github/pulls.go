package github

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

func listPRs(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	state := r.Str("state")
	head := r.Str("head")
	base := r.Str("base")
	sort := r.Str("sort")
	direction := r.Str("direction")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.PullRequestListOptions{
		State:       state,
		Head:        head,
		Base:        base,
		Sort:        sort,
		Direction:   direction,
		ListOptions: gh.ListOptions{Page: page, PerPage: perPage},
	}
	prs, _, err := g.client.PullRequests.List(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(prs)
}

func getPR(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	pull := r.Int("pull_number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	pr, _, err := g.client.PullRequests.Get(ctx, owner, repo, pull)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(pr)
}

func getPRDiff(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	pull := r.Int("pull_number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	diff, _, err := g.client.PullRequests.GetRaw(ctx, owner, repo, pull, gh.RawOptions{Type: gh.Diff})
	if err != nil {
		return errResult(err)
	}
	return &mcp.ToolResult{Data: diff}, nil
}

func createPR(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	title := r.Str("title")
	head := r.Str("head")
	base := r.Str("base")
	body := r.Str("body")
	draft := r.Bool("draft")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	pr := &gh.NewPullRequest{
		Title: gh.Ptr(title),
		Head:  gh.Ptr(head),
		Base:  gh.Ptr(base),
		Body:  gh.Ptr(body),
		Draft: gh.Ptr(draft),
	}
	pull, _, err := g.client.PullRequests.Create(ctx, owner, repo, pr)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(pull)
}

func updatePR(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	pullNumber := r.Int("pull_number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	pr := &gh.PullRequest{}
	title, err := mcp.ArgStr(args, "title")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if title != "" {
		pr.Title = gh.Ptr(title)
	}
	body, err := mcp.ArgStr(args, "body")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if body != "" {
		pr.Body = gh.Ptr(body)
	}
	state, err := mcp.ArgStr(args, "state")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if state != "" {
		pr.State = gh.Ptr(state)
	}
	base, err := mcp.ArgStr(args, "base")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if base != "" {
		pr.Base = &gh.PullRequestBranch{Ref: gh.Ptr(base)}
	}
	pull, _, err := g.client.PullRequests.Edit(ctx, owner, repo, pullNumber, pr)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(pull)
}

func listPRCommits(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	pull := r.Int("pull_number")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: page, PerPage: perPage}
	commits, _, err := g.client.PullRequests.ListCommits(ctx, owner, repo, pull, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(commits)
}

func listPRFiles(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	pull := r.Int("pull_number")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: page, PerPage: perPage}
	files, _, err := g.client.PullRequests.ListFiles(ctx, owner, repo, pull, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(files)
}

func listPRReviews(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	pull := r.Int("pull_number")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: page, PerPage: perPage}
	reviews, _, err := g.client.PullRequests.ListReviews(ctx, owner, repo, pull, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(reviews)
}

func createPRReview(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	pull := r.Int("pull_number")
	body := r.Str("body")
	event := r.Str("event")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	review := &gh.PullRequestReviewRequest{
		Body:  gh.Ptr(body),
		Event: gh.Ptr(event),
	}
	result, _, err := g.client.PullRequests.CreateReview(ctx, owner, repo, pull, review)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(result)
}

func listPRComments(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	pull := r.Int("pull_number")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.PullRequestListCommentsOptions{ListOptions: gh.ListOptions{Page: page, PerPage: perPage}}
	comments, _, err := g.client.PullRequests.ListComments(ctx, owner, repo, pull, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(comments)
}

func createPRComment(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	pull := r.Int("pull_number")
	body := r.Str("body")
	commitID := r.Str("commit_id")
	path := r.Str("path")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	comment := &gh.PullRequestComment{
		Body:     gh.Ptr(body),
		CommitID: gh.Ptr(commitID),
		Path:     gh.Ptr(path),
	}
	if line, err := mcp.ArgInt(args, "line"); err != nil {
		return mcp.ErrResult(err)
	} else if line > 0 {
		comment.Line = gh.Ptr(line)
	}
	c, _, err := g.client.PullRequests.CreateComment(ctx, owner, repo, pull, comment)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(c)
}

func getPRComment(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	commentID := r.Int64("comment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	c, _, err := g.client.PullRequests.GetComment(ctx, owner, repo, commentID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(c)
}

func replyToPRComment(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	pull := r.Int("pull_number")
	body := r.Str("body")
	commentID := r.Int64("comment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	c, _, err := g.client.PullRequests.CreateCommentInReplyTo(ctx, owner, repo, pull, body, commentID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(c)
}

func updatePRComment(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	commentID := r.Int64("comment_id")
	body := r.Str("body")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	comment := &gh.PullRequestComment{Body: gh.Ptr(body)}
	c, _, err := g.client.PullRequests.EditComment(ctx, owner, repo, commentID, comment)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(c)
}

func deletePRComment(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	commentID := r.Int64("comment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.PullRequests.DeleteComment(ctx, owner, repo, commentID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

func mergePR(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	pull := r.Int("pull_number")
	commitMessage := r.Str("commit_message")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.PullRequestOptions{}
	if method, err := mcp.ArgStr(args, "merge_method"); err != nil {
		return mcp.ErrResult(err)
	} else if method != "" {
		opts.MergeMethod = method
	}
	result, _, err := g.client.PullRequests.Merge(ctx, owner, repo, pull, commitMessage, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(result)
}

func listRequestedReviewers(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	pull := r.Int("pull_number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	reviewers, _, err := g.client.PullRequests.ListReviewers(ctx, owner, repo, pull, nil)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(reviewers)
}

func requestReviewers(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
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
	pr, _, err := g.client.PullRequests.RequestReviewers(ctx, owner, repo, pull, req)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(pr)
}
