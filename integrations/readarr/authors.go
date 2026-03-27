package readarr

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listAuthors(ctx context.Context, r *readarr, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := r.get(ctx, "/api/v1/author")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAuthor(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	id := a.Int("id")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := r.get(ctx, "/api/v1/author/%d", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
