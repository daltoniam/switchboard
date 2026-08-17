package salesforce

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func listAPIVersions(ctx context.Context, s *salesforce, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/services/data/")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getLimits(ctx context.Context, s *salesforce, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "%s/limits", s.ver())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listRecentlyViewed(ctx context.Context, s *salesforce, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	limit := r.Str("limit")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"limit": limit})
	data, err := s.get(ctx, "%s/recent%s", s.ver(), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func compositeBatch(ctx context.Context, s *salesforce, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	requestsStr := r.Str("requests")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var batchRequests any
	if err := json.Unmarshal([]byte(requestsStr), &batchRequests); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid JSON for requests: %w", err))
	}
	body := map[string]any{"batchRequests": batchRequests}
	path := fmt.Sprintf("%s/composite/batch", s.ver())
	data, err := s.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func sObjectCollections(ctx context.Context, s *salesforce, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	method := strings.ToUpper(r.Str("method"))
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	allOrNone, _ := mcp.ArgStr(args, "all_or_none")

	path := fmt.Sprintf("%s/composite/sobjects", s.ver())

	switch method {
	case "POST", "PATCH":
		recordsStr, err := mcp.ArgStr(args, "records")
		if err != nil {
			return mcp.ErrResult(err)
		}
		if recordsStr == "" {
			return mcp.ErrResult(fmt.Errorf("records is required for %s", method))
		}
		var records any
		if err := json.Unmarshal([]byte(recordsStr), &records); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid JSON for records: %w", err))
		}
		body := map[string]any{
			"records":   records,
			"allOrNone": allOrNone == "true",
		}
		var data json.RawMessage
		if method == "POST" {
			data, err = s.post(ctx, path, body)
		} else {
			data, err = s.patch(ctx, path, body)
		}
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)

	case "DELETE":
		ids, err := mcp.ArgStr(args, "ids")
		if err != nil {
			return mcp.ErrResult(err)
		}
		if ids == "" {
			return mcp.ErrResult(fmt.Errorf("ids is required for DELETE"))
		}
		q := queryEncode(map[string]string{
			"ids":       ids,
			"allOrNone": allOrNone,
		})
		data, err := s.del(ctx, "%s%s", path, q)
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)

	default:
		return mcp.ErrResult(fmt.Errorf("unsupported method: %s (use POST, PATCH, or DELETE)", method))
	}
}
