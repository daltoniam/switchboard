package github

import (
	"context"
	"time"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

func getCommit(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	owner, err := mcp.ArgStr(args, "owner")
	if err != nil {
		return mcp.ErrResult(err)
	}
	repo, err := mcp.ArgStr(args, "repo")
	if err != nil {
		return mcp.ErrResult(err)
	}
	sha, err := mcp.ArgStr(args, "sha")
	if err != nil {
		return mcp.ErrResult(err)
	}
	commit, _, err := g.client.Repositories.GetCommit(ctx, owner, repo, sha, nil)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(commit)
}

func listCommits(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	sha, err := mcp.ArgStr(args, "sha")
	if err != nil {
		return mcp.ErrResult(err)
	}
	path, err := mcp.ArgStr(args, "path")
	if err != nil {
		return mcp.ErrResult(err)
	}
	author, err := mcp.ArgStr(args, "author")
	if err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.CommitsListOptions{
		SHA:         sha,
		Path:        path,
		Author:      author,
		ListOptions: lo,
	}
	since, err := mcp.ArgStr(args, "since")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			opts.Since = t
		}
	}
	until, err := mcp.ArgStr(args, "until")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if until != "" {
		if t, err := time.Parse(time.RFC3339, until); err == nil {
			opts.Until = t
		}
	}
	owner, err := mcp.ArgStr(args, "owner")
	if err != nil {
		return mcp.ErrResult(err)
	}
	repo, err := mcp.ArgStr(args, "repo")
	if err != nil {
		return mcp.ErrResult(err)
	}
	commits, _, err := g.client.Repositories.ListCommits(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(commits)
}

func getRef(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	owner, err := mcp.ArgStr(args, "owner")
	if err != nil {
		return mcp.ErrResult(err)
	}
	repo, err := mcp.ArgStr(args, "repo")
	if err != nil {
		return mcp.ErrResult(err)
	}
	ref, err := mcp.ArgStr(args, "ref")
	if err != nil {
		return mcp.ErrResult(err)
	}
	r, _, err := g.client.Git.GetRef(ctx, owner, repo, ref)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(r)
}

func createRef(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	refStr, err := mcp.ArgStr(args, "ref")
	if err != nil {
		return mcp.ErrResult(err)
	}
	sha, err := mcp.ArgStr(args, "sha")
	if err != nil {
		return mcp.ErrResult(err)
	}
	ref := &gh.Reference{
		Ref:    gh.Ptr(refStr),
		Object: &gh.GitObject{SHA: gh.Ptr(sha)},
	}
	owner, err := mcp.ArgStr(args, "owner")
	if err != nil {
		return mcp.ErrResult(err)
	}
	repo, err := mcp.ArgStr(args, "repo")
	if err != nil {
		return mcp.ErrResult(err)
	}
	r, _, err := g.client.Git.CreateRef(ctx, owner, repo, ref)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(r)
}

func deleteRef(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	owner, err := mcp.ArgStr(args, "owner")
	if err != nil {
		return mcp.ErrResult(err)
	}
	repo, err := mcp.ArgStr(args, "repo")
	if err != nil {
		return mcp.ErrResult(err)
	}
	ref, err := mcp.ArgStr(args, "ref")
	if err != nil {
		return mcp.ErrResult(err)
	}
	_, err = g.client.Git.DeleteRef(ctx, owner, repo, ref)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

func getTree(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	recursive, err := mcp.ArgBool(args, "recursive")
	if err != nil {
		return mcp.ErrResult(err)
	}
	owner, err := mcp.ArgStr(args, "owner")
	if err != nil {
		return mcp.ErrResult(err)
	}
	repo, err := mcp.ArgStr(args, "repo")
	if err != nil {
		return mcp.ErrResult(err)
	}
	sha, err := mcp.ArgStr(args, "sha")
	if err != nil {
		return mcp.ErrResult(err)
	}
	tree, _, err := g.client.Git.GetTree(ctx, owner, repo, sha, recursive)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(tree)
}

func createTag(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	objType, err := mcp.ArgStr(args, "type")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if objType == "" {
		objType = "commit"
	}
	tagName, err := mcp.ArgStr(args, "tag")
	if err != nil {
		return mcp.ErrResult(err)
	}
	message, err := mcp.ArgStr(args, "message")
	if err != nil {
		return mcp.ErrResult(err)
	}
	sha, err := mcp.ArgStr(args, "sha")
	if err != nil {
		return mcp.ErrResult(err)
	}
	tag := &gh.Tag{
		Tag:     gh.Ptr(tagName),
		Message: gh.Ptr(message),
		Object:  &gh.GitObject{SHA: gh.Ptr(sha), Type: gh.Ptr(objType)},
	}
	owner, err := mcp.ArgStr(args, "owner")
	if err != nil {
		return mcp.ErrResult(err)
	}
	repo, err := mcp.ArgStr(args, "repo")
	if err != nil {
		return mcp.ErrResult(err)
	}
	t, _, err := g.client.Git.CreateTag(ctx, owner, repo, tag)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(t)
}
