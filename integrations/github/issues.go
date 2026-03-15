package github

import (
	"context"
	"time"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

func listIssues(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.IssueListByRepoOptions{
		State:       argStr(args, "state"),
		Labels:      argStrSlice(args, "labels"),
		Sort:        argStr(args, "sort"),
		Direction:   argStr(args, "direction"),
		Assignee:    argStr(args, "assignee"),
		ListOptions: listOpts(args),
	}
	if m := argInt(args, "milestone"); m > 0 {
		opts.Milestone = argStr(args, "milestone")
	}
	issues, _, err := g.client.Issues.ListByRepo(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(issues)
}

func getIssue(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	issue, _, err := g.client.Issues.Get(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(issue)
}

func createIssue(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := &gh.IssueRequest{
		Title: gh.Ptr(argStr(args, "title")),
		Body:  gh.Ptr(argStr(args, "body")),
	}
	if assignees := argStrSlice(args, "assignees"); len(assignees) > 0 {
		req.Assignees = &assignees
	}
	if labels := argStrSlice(args, "labels"); len(labels) > 0 {
		req.Labels = &labels
	}
	if m := argInt(args, "milestone"); m > 0 {
		req.Milestone = gh.Ptr(m)
	}
	issue, _, err := g.client.Issues.Create(ctx, argStr(args, "owner"), argStr(args, "repo"), req)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(issue)
}

func updateIssue(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := &gh.IssueRequest{}
	if v := argStr(args, "title"); v != "" {
		req.Title = gh.Ptr(v)
	}
	if v := argStr(args, "body"); v != "" {
		req.Body = gh.Ptr(v)
	}
	if v := argStr(args, "state"); v != "" {
		req.State = gh.Ptr(v)
	}
	if assignees := argStrSlice(args, "assignees"); len(assignees) > 0 {
		req.Assignees = &assignees
	}
	if labels := argStrSlice(args, "labels"); len(labels) > 0 {
		req.Labels = &labels
	}
	if m := argInt(args, "milestone"); m > 0 {
		req.Milestone = gh.Ptr(m)
	}
	issue, _, err := g.client.Issues.Edit(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), req)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(issue)
}

func listIssueComments(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.IssueListCommentsOptions{ListOptions: listOpts(args)}
	comments, _, err := g.client.Issues.ListComments(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(comments)
}

func createIssueComment(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	comment := &gh.IssueComment{Body: gh.Ptr(argStr(args, "body"))}
	c, _, err := g.client.Issues.CreateComment(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), comment)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(c)
}

func listIssueLabels(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	labels, _, err := g.client.Issues.ListLabelsByIssue(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(labels)
}

func addIssueLabels(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	labels := argStrSlice(args, "labels")
	result, _, err := g.client.Issues.AddLabelsToIssue(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), labels)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(result)
}

func removeIssueLabel(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Issues.RemoveLabelForIssue(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), argStr(args, "label"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "removed"})
}

func lockIssue(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.LockIssueOptions{}
	if reason := argStr(args, "lock_reason"); reason != "" {
		opts.LockReason = reason
	}
	_, err := g.client.Issues.Lock(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "locked"})
}

func unlockIssue(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Issues.Unlock(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "unlocked"})
}

func listMilestones(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.MilestoneListOptions{
		State:       argStr(args, "state"),
		Sort:        argStr(args, "sort"),
		ListOptions: listOpts(args),
	}
	milestones, _, err := g.client.Issues.ListMilestones(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(milestones)
}

func createMilestone(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	m := &gh.Milestone{
		Title:       gh.Ptr(argStr(args, "title")),
		Description: gh.Ptr(argStr(args, "description")),
		State:       gh.Ptr(argStr(args, "state")),
	}
	if due := argStr(args, "due_on"); due != "" {
		if t, err := time.Parse(time.RFC3339, due); err == nil {
			ts := gh.Timestamp{Time: t}
			m.DueOn = &ts
		}
	}
	milestone, _, err := g.client.Issues.CreateMilestone(ctx, argStr(args, "owner"), argStr(args, "repo"), m)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(milestone)
}

func listIssueEvents(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	events, _, err := g.client.Issues.ListIssueEvents(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(events)
}

func listIssueTimeline(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	events, _, err := g.client.Issues.ListIssueTimeline(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(events)
}

func listAssignees(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	users, _, err := g.client.Issues.ListAssignees(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(users)
}
