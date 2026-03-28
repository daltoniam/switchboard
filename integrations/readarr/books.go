package readarr

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func listBooks(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	authorID := a.Str("author_id")
	includeAll := a.Str("include_all_author_books")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{}
	if authorID != "" {
		params["authorId"] = authorID
	}
	if includeAll != "" {
		params["includeAllAuthorBooks"] = includeAll
	}
	data, err := r.get(ctx, "/api/v1/book%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getBook(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	id := a.Int("id")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := r.get(ctx, "/api/v1/book/%d", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchBooks(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	term := a.Str("term")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if term == "" {
		return mcp.ErrResult(fmt.Errorf("term is required"))
	}
	qs := queryEncode(map[string]string{"term": term})
	data, err := r.get(ctx, "/api/v1/search%s", qs)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func monitorBooks(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	idsStr := a.Str("book_ids")
	monitored := a.Bool("monitored")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if idsStr == "" {
		return mcp.ErrResult(fmt.Errorf("book_ids is required"))
	}

	var bookIDs []int
	for _, s := range strings.Split(idsStr, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := strconv.Atoi(s)
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid book ID %q: %w", s, err))
		}
		bookIDs = append(bookIDs, id)
	}

	body := map[string]any{
		"bookIds":   bookIDs,
		"monitored": monitored,
	}
	data, err := r.put(ctx, "/api/v1/book/monitor", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
