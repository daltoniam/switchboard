package github

import (
	"context"
	"time"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

func listIssues(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	state := r.Str("state")
	sort := r.Str("sort")
	direction := r.Str("direction")
	assignee := r.Str("assignee")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	labels, err := mcp.ArgStrSlice(args, "labels")
	if err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.IssueListByRepoOptions{
		State:       state,
		Labels:      labels,
		Sort:        sort,
		Direction:   direction,
		Assignee:    assignee,
		ListOptions: lo,
	}
	if m, err := mcp.ArgInt(args, "milestone"); err != nil {
		return mcp.ErrResult(err)
	} else if m > 0 {
		if ms, err := mcp.ArgStr(args, "milestone"); err != nil {
			return mcp.ErrResult(err)
		} else {
			opts.Milestone = ms
		}
	}
	issues, _, err := g.client.Issues.ListByRepo(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(issues)
}

func getIssue(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	number := r.Int("number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	issue, _, err := g.client.Issues.Get(ctx, owner, repo, number)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(issue)
}

func createIssue(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	title := r.Str("title")
	body := r.Str("body")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	req := &gh.IssueRequest{
		Title: gh.Ptr(title),
		Body:  gh.Ptr(body),
	}
	if assignees, err := mcp.ArgStrSlice(args, "assignees"); err != nil {
		return mcp.ErrResult(err)
	} else if len(assignees) > 0 {
		req.Assignees = &assignees
	}
	if labels, err := mcp.ArgStrSlice(args, "labels"); err != nil {
		return mcp.ErrResult(err)
	} else if len(labels) > 0 {
		req.Labels = &labels
	}
	if m, err := mcp.ArgInt(args, "milestone"); err != nil {
		return mcp.ErrResult(err)
	} else if m > 0 {
		req.Milestone = gh.Ptr(m)
	}
	issue, _, err := g.client.Issues.Create(ctx, owner, repo, req)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(issue)
}

func updateIssue(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	number := r.Int("number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	req := &gh.IssueRequest{}
	if v, err := mcp.ArgStr(args, "title"); err != nil {
		return mcp.ErrResult(err)
	} else if v != "" {
		req.Title = gh.Ptr(v)
	}
	if v, err := mcp.ArgStr(args, "body"); err != nil {
		return mcp.ErrResult(err)
	} else if v != "" {
		req.Body = gh.Ptr(v)
	}
	if v, err := mcp.ArgStr(args, "state"); err != nil {
		return mcp.ErrResult(err)
	} else if v != "" {
		req.State = gh.Ptr(v)
	}
	if assignees, err := mcp.ArgStrSlice(args, "assignees"); err != nil {
		return mcp.ErrResult(err)
	} else if len(assignees) > 0 {
		req.Assignees = &assignees
	}
	if labels, err := mcp.ArgStrSlice(args, "labels"); err != nil {
		return mcp.ErrResult(err)
	} else if len(labels) > 0 {
		req.Labels = &labels
	}
	if m, err := mcp.ArgInt(args, "milestone"); err != nil {
		return mcp.ErrResult(err)
	} else if m > 0 {
		req.Milestone = gh.Ptr(m)
	}
	issue, _, err := g.client.Issues.Edit(ctx, owner, repo, number, req)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(issue)
}

func listIssueComments(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	number := r.Int("number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.IssueListCommentsOptions{ListOptions: lo}
	comments, _, err := g.client.Issues.ListComments(ctx, owner, repo, number, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(comments)
}

func createIssueComment(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	number := r.Int("number")
	body := r.Str("body")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	comment := &gh.IssueComment{Body: gh.Ptr(body)}
	c, _, err := g.client.Issues.CreateComment(ctx, owner, repo, number, comment)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(c)
}

func listIssueLabels(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	number := r.Int("number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	labels, _, err := g.client.Issues.ListLabelsByIssue(ctx, owner, repo, number, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(labels)
}

func addIssueLabels(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	number := r.Int("number")
	labels := r.StrSlice("labels")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	result, _, err := g.client.Issues.AddLabelsToIssue(ctx, owner, repo, number, labels)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(result)
}

func removeIssueLabel(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	number := r.Int("number")
	label := r.Str("label")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Issues.RemoveLabelForIssue(ctx, owner, repo, number, label)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "removed"})
}

func lockIssue(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	number := r.Int("number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.LockIssueOptions{}
	if reason, err := mcp.ArgStr(args, "lock_reason"); err != nil {
		return mcp.ErrResult(err)
	} else if reason != "" {
		opts.LockReason = reason
	}
	_, err := g.client.Issues.Lock(ctx, owner, repo, number, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "locked"})
}

func unlockIssue(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	number := r.Int("number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Issues.Unlock(ctx, owner, repo, number)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "unlocked"})
}

func listMilestones(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	state := r.Str("state")
	sort := r.Str("sort")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.MilestoneListOptions{
		State:       state,
		Sort:        sort,
		ListOptions: lo,
	}
	milestones, _, err := g.client.Issues.ListMilestones(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(milestones)
}

func createMilestone(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	title := r.Str("title")
	description := r.Str("description")
	state := r.Str("state")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	m := &gh.Milestone{
		Title:       gh.Ptr(title),
		Description: gh.Ptr(description),
		State:       gh.Ptr(state),
	}
	if due, err := mcp.ArgStr(args, "due_on"); err != nil {
		return mcp.ErrResult(err)
	} else if due != "" {
		if t, err := time.Parse(time.RFC3339, due); err == nil {
			ts := gh.Timestamp{Time: t}
			m.DueOn = &ts
		}
	}
	milestone, _, err := g.client.Issues.CreateMilestone(ctx, owner, repo, m)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(milestone)
}

func listIssueEvents(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	number := r.Int("number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	events, _, err := g.client.Issues.ListIssueEvents(ctx, owner, repo, number, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(events)
}

func listIssueTimeline(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	number := r.Int("number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	events, _, err := g.client.Issues.ListIssueTimeline(ctx, owner, repo, number, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(events)
}

func listAssignees(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	users, _, err := g.client.Issues.ListAssignees(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(users)
}
