package github

import (
	"context"
	"time"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

func getCommit(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	commit, _, err := g.client.Repositories.GetCommit(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "sha"), nil)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(commit)
}

func listCommits(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.CommitsListOptions{
		SHA:         argStr(args, "sha"),
		Path:        argStr(args, "path"),
		Author:      argStr(args, "author"),
		ListOptions: listOpts(args),
	}
	if since := argStr(args, "since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			opts.Since = t
		}
	}
	if until := argStr(args, "until"); until != "" {
		if t, err := time.Parse(time.RFC3339, until); err == nil {
			opts.Until = t
		}
	}
	commits, _, err := g.client.Repositories.ListCommits(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(commits)
}

func getRef(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	ref, _, err := g.client.Git.GetRef(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "ref"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(ref)
}

func createRef(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	ref := &gh.Reference{
		Ref:    gh.Ptr(argStr(args, "ref")),
		Object: &gh.GitObject{SHA: gh.Ptr(argStr(args, "sha"))},
	}
	r, _, err := g.client.Git.CreateRef(ctx, argStr(args, "owner"), argStr(args, "repo"), ref)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(r)
}

func deleteRef(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Git.DeleteRef(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "ref"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "deleted"})
}

func getTree(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	recursive := argBool(args, "recursive")
	var tree *gh.Tree
	var err error
	if recursive {
		tree, _, err = g.client.Git.GetTree(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "sha"), true)
	} else {
		tree, _, err = g.client.Git.GetTree(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "sha"), false)
	}
	if err != nil {
		return errResult(err)
	}
	return jsonResult(tree)
}

func createTag(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	objType := argStr(args, "type")
	if objType == "" {
		objType = "commit"
	}
	tag := &gh.Tag{
		Tag:     gh.Ptr(argStr(args, "tag")),
		Message: gh.Ptr(argStr(args, "message")),
		Object:  &gh.GitObject{SHA: gh.Ptr(argStr(args, "sha")), Type: gh.Ptr(objType)},
	}
	t, _, err := g.client.Git.CreateTag(ctx, argStr(args, "owner"), argStr(args, "repo"), tag)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(t)
}
