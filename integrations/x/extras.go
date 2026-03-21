package x

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
)

// --- Spaces ---

func getSpace(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	id := r.Str("id")
	q := queryEncode(map[string]string{
		"space.fields": r.Str("space_fields"),
	})
	data, err := t.get(ctx, "/spaces/%s%s", id, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchSpaces(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"query":        r.Str("query"),
		"state":        r.Str("state"),
		"max_results":  r.Str("max_results"),
		"space.fields": r.Str("space_fields"),
	})
	data, err := t.get(ctx, "/spaces/search%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Usage ---

func getUsage(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"days": r.Str("days"),
	})
	data, err := t.get(ctx, "/usage/tweets%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
