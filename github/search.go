package github

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

func searchCode(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.SearchOptions{ListOptions: listOpts(args)}
	result, _, err := g.client.Search.Code(ctx, argStr(args, "query"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(result)
}

func searchIssues(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.SearchOptions{
		Sort:        argStr(args, "sort"),
		Order:       argStr(args, "order"),
		ListOptions: listOpts(args),
	}
	result, _, err := g.client.Search.Issues(ctx, argStr(args, "query"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(result)
}

func searchUsers(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.SearchOptions{
		Sort:        argStr(args, "sort"),
		Order:       argStr(args, "order"),
		ListOptions: listOpts(args),
	}
	result, _, err := g.client.Search.Users(ctx, argStr(args, "query"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(result)
}

func searchCommits(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.SearchOptions{
		Sort:        argStr(args, "sort"),
		Order:       argStr(args, "order"),
		ListOptions: listOpts(args),
	}
	result, _, err := g.client.Search.Commits(ctx, argStr(args, "query"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(result)
}
