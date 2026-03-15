package github

import (
	"context"
	"time"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

// ── Issue Comments Extended ───────────────────────────────────────

func updateIssueComment(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	comment := &gh.IssueComment{Body: gh.Ptr(argStr(args, "body"))}
	c, _, err := g.client.Issues.EditComment(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "comment_id"), comment)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(c)
}

func deleteIssueComment(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Issues.DeleteComment(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "comment_id"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

// ── Milestones Extended ───────────────────────────────────────────

func updateMilestone(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	m := &gh.Milestone{}
	if v := argStr(args, "title"); v != "" {
		m.Title = gh.Ptr(v)
	}
	if v := argStr(args, "description"); v != "" {
		m.Description = gh.Ptr(v)
	}
	if v := argStr(args, "state"); v != "" {
		m.State = gh.Ptr(v)
	}
	if due := argStr(args, "due_on"); due != "" {
		if t, err := time.Parse(time.RFC3339, due); err == nil {
			ts := gh.Timestamp{Time: t}
			m.DueOn = &ts
		}
	}
	milestone, _, err := g.client.Issues.EditMilestone(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), m)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(milestone)
}

func deleteMilestone(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Issues.DeleteMilestone(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

// ── Labels (Repo-Level) ──────────────────────────────────────────

func listLabels(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	labels, _, err := g.client.Issues.ListLabels(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(labels)
}

func createLabel(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	label := &gh.Label{
		Name:        gh.Ptr(argStr(args, "name")),
		Color:       gh.Ptr(argStr(args, "color")),
		Description: gh.Ptr(argStr(args, "description")),
	}
	l, _, err := g.client.Issues.CreateLabel(ctx, argStr(args, "owner"), argStr(args, "repo"), label)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(l)
}

func editLabel(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	label := &gh.Label{}
	if v := argStr(args, "new_name"); v != "" {
		label.Name = gh.Ptr(v)
	}
	if v := argStr(args, "color"); v != "" {
		label.Color = gh.Ptr(v)
	}
	if v := argStr(args, "description"); v != "" {
		label.Description = gh.Ptr(v)
	}
	l, _, err := g.client.Issues.EditLabel(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "name"), label)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(l)
}

func deleteLabel(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Issues.DeleteLabel(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "name"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

// ── Reactions ─────────────────────────────────────────────────────

func createIssueReaction(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	reaction, _, err := g.client.Reactions.CreateIssueReaction(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), argStr(args, "content"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(reaction)
}

func createIssueCommentReaction(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	reaction, _, err := g.client.Reactions.CreateIssueCommentReaction(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "comment_id"), argStr(args, "content"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(reaction)
}

func listIssueReactions(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	reactions, _, err := g.client.Reactions.ListIssueReactions(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "number"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(reactions)
}
