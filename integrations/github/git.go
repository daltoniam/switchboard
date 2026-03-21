package github

import (
	"context"
	"time"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

func getCommit(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	sha := r.Str("sha")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	commit, _, err := g.client.Repositories.GetCommit(ctx, owner, repo, sha, nil)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(commit)
}

func listCommits(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sha := r.Str("sha")
	path := r.Str("path")
	author := r.Str("author")
	since := r.Str("since")
	until := r.Str("until")
	owner := r.Str("owner")
	repo := r.Str("repo")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.CommitsListOptions{
		SHA:         sha,
		Path:        path,
		Author:      author,
		ListOptions: gh.ListOptions{Page: page, PerPage: perPage},
	}
	if since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			opts.Since = t
		}
	}
	if until != "" {
		if t, err := time.Parse(time.RFC3339, until); err == nil {
			opts.Until = t
		}
	}
	commits, _, err := g.client.Repositories.ListCommits(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(commits)
}

func getRef(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	owner := reader.Str("owner")
	repo := reader.Str("repo")
	ref := reader.Str("ref")
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	r, _, err := g.client.Git.GetRef(ctx, owner, repo, ref)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(r)
}

func createRef(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	refStr := r.Str("ref")
	sha := r.Str("sha")
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ref := &gh.Reference{
		Ref:    gh.Ptr(refStr),
		Object: &gh.GitObject{SHA: gh.Ptr(sha)},
	}
	result, _, err := g.client.Git.CreateRef(ctx, owner, repo, ref)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(result)
}

func deleteRef(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	ref := r.Str("ref")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Git.DeleteRef(ctx, owner, repo, ref)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

func getTree(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	recursive := r.Bool("recursive")
	owner := r.Str("owner")
	repo := r.Str("repo")
	sha := r.Str("sha")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	tree, _, err := g.client.Git.GetTree(ctx, owner, repo, sha, recursive)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(tree)
}

func createTag(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	objType := r.Str("type")
	tagName := r.Str("tag")
	message := r.Str("message")
	sha := r.Str("sha")
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if objType == "" {
		objType = "commit"
	}
	tag := &gh.Tag{
		Tag:     gh.Ptr(tagName),
		Message: gh.Ptr(message),
		Object:  &gh.GitObject{SHA: gh.Ptr(sha), Type: gh.Ptr(objType)},
	}
	t, _, err := g.client.Git.CreateTag(ctx, owner, repo, tag)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(t)
}
