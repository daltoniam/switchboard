package github

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

// searchResult wraps search items with total_count so the LLM knows
// whether to refine the query or paginate for more results.
func searchResult(total int, items any) (*mcp.ToolResult, error) {
	return mcp.JSONResult(map[string]any{
		"total_count": total,
		"items":       items,
	})
}

func searchCode(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	query, err := mcp.ArgStr(args, "query")
	if err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.SearchOptions{ListOptions: lo}
	resp, _, err := g.client.Search.Code(ctx, query, opts)
	if err != nil {
		return errResult(err)
	}
	return searchResult(resp.GetTotal(), resp.CodeResults)
}

func searchIssues(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	query := r.Str("query")
	sort := r.Str("sort")
	order := r.Str("order")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.SearchOptions{
		Sort:        sort,
		Order:       order,
		ListOptions: lo,
	}
	resp, _, err := g.client.Search.Issues(ctx, query, opts)
	if err != nil {
		return errResult(err)
	}
	return searchResult(resp.GetTotal(), resp.Issues)
}

func searchUsers(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	query := r.Str("query")
	sort := r.Str("sort")
	order := r.Str("order")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.SearchOptions{
		Sort:        sort,
		Order:       order,
		ListOptions: lo,
	}
	resp, _, err := g.client.Search.Users(ctx, query, opts)
	if err != nil {
		return errResult(err)
	}
	return searchResult(resp.GetTotal(), resp.Users)
}

func searchCommits(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	query := r.Str("query")
	sort := r.Str("sort")
	order := r.Str("order")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.SearchOptions{
		Sort:        sort,
		Order:       order,
		ListOptions: lo,
	}
	resp, _, err := g.client.Search.Commits(ctx, query, opts)
	if err != nil {
		return errResult(err)
	}
	return searchResult(resp.GetTotal(), resp.Commits)
}
