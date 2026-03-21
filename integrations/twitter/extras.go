package twitter

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
)

// --- Spaces ---

func getSpace(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "id")
	q := queryEncode(map[string]string{
		"space.fields": argStr(args, "space_fields"),
	})
	data, err := t.get(ctx, "/spaces/%s%s", id, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func searchSpaces(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"query":        argStr(args, "query"),
		"state":        argStr(args, "state"),
		"max_results":  argStr(args, "max_results"),
		"space.fields": argStr(args, "space_fields"),
	})
	data, err := t.get(ctx, "/spaces/search%s", q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// --- Usage ---

func getUsage(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"days": argStr(args, "days"),
	})
	data, err := t.get(ctx, "/usage/tweets%s", q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
