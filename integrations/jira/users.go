package jira

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
)

func getMyself(ctx context.Context, j *jira, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := j.get(ctx, "/myself")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchUsers(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := queryEncode(map[string]string{"query": r.Str("query")})
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := j.get(ctx, "/user/search%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getUser(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := queryEncode(map[string]string{"accountId": r.Str("account_id")})
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := j.get(ctx, "/user%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
