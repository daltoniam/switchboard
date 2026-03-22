package github

import (
	"context"
	"time"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

// ── Issue Comments Extended ───────────────────────────────────────

func updateIssueComment(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	commentID := r.Int64("comment_id")
	body := r.Str("body")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	comment := &gh.IssueComment{Body: gh.Ptr(body)}
	c, _, err := g.client.Issues.EditComment(ctx, owner, repo, commentID, comment)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(c)
}

func deleteIssueComment(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	commentID := r.Int64("comment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Issues.DeleteComment(ctx, owner, repo, commentID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

// ── Milestones Extended ───────────────────────────────────────────

func updateMilestone(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	number := r.Int("number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	m := &gh.Milestone{}
	if v, err := mcp.ArgStr(args, "title"); err != nil {
		return mcp.ErrResult(err)
	} else if v != "" {
		m.Title = gh.Ptr(v)
	}
	if v, err := mcp.ArgStr(args, "description"); err != nil {
		return mcp.ErrResult(err)
	} else if v != "" {
		m.Description = gh.Ptr(v)
	}
	if v, err := mcp.ArgStr(args, "state"); err != nil {
		return mcp.ErrResult(err)
	} else if v != "" {
		m.State = gh.Ptr(v)
	}
	if due, err := mcp.ArgStr(args, "due_on"); err != nil {
		return mcp.ErrResult(err)
	} else if due != "" {
		if t, err := time.Parse(time.RFC3339, due); err == nil {
			ts := gh.Timestamp{Time: t}
			m.DueOn = &ts
		}
	}
	milestone, _, err := g.client.Issues.EditMilestone(ctx, owner, repo, number, m)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(milestone)
}

func deleteMilestone(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	number := r.Int("number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Issues.DeleteMilestone(ctx, owner, repo, number)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

// ── Labels (Repo-Level) ──────────────────────────────────────────

func listLabels(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: page, PerPage: perPage}
	labels, _, err := g.client.Issues.ListLabels(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(labels)
}

func createLabel(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	name := r.Str("name")
	color := r.Str("color")
	description := r.Str("description")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	label := &gh.Label{
		Name:        gh.Ptr(name),
		Color:       gh.Ptr(color),
		Description: gh.Ptr(description),
	}
	l, _, err := g.client.Issues.CreateLabel(ctx, owner, repo, label)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(l)
}

func editLabel(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	label := &gh.Label{}
	if v, err := mcp.ArgStr(args, "new_name"); err != nil {
		return mcp.ErrResult(err)
	} else if v != "" {
		label.Name = gh.Ptr(v)
	}
	if v, err := mcp.ArgStr(args, "color"); err != nil {
		return mcp.ErrResult(err)
	} else if v != "" {
		label.Color = gh.Ptr(v)
	}
	if v, err := mcp.ArgStr(args, "description"); err != nil {
		return mcp.ErrResult(err)
	} else if v != "" {
		label.Description = gh.Ptr(v)
	}
	l, _, err := g.client.Issues.EditLabel(ctx, owner, repo, name, label)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(l)
}

func deleteLabel(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Issues.DeleteLabel(ctx, owner, repo, name)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

// ── Reactions ─────────────────────────────────────────────────────

func createIssueReaction(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	number := r.Int("number")
	content := r.Str("content")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	reaction, _, err := g.client.Reactions.CreateIssueReaction(ctx, owner, repo, number, content)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(reaction)
}

func createIssueCommentReaction(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	commentID := r.Int64("comment_id")
	content := r.Str("content")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	reaction, _, err := g.client.Reactions.CreateIssueCommentReaction(ctx, owner, repo, commentID, content)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(reaction)
}

func listIssueReactions(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	number := r.Int("number")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: page, PerPage: perPage}
	reactions, _, err := g.client.Reactions.ListIssueReactions(ctx, owner, repo, number, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(reactions)
}
