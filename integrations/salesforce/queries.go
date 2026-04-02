package salesforce

import (
	"context"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func query(ctx context.Context, s *salesforce, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := r.Str("q")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "%s/query?q=%s", s.ver(), url.QueryEscape(q))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func queryMore(ctx context.Context, s *salesforce, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	nextURL := r.Str("next_url")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "%s", nextURL)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func search(ctx context.Context, s *salesforce, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := r.Str("q")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "%s/search?q=%s", s.ver(), url.QueryEscape(q))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
