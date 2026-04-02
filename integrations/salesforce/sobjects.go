package salesforce

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func describeGlobal(ctx context.Context, s *salesforce, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "%s/sobjects/", s.ver())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func describeSObject(ctx context.Context, s *salesforce, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sobject := r.Str("sobject")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "%s/sobjects/%s/describe", s.ver(), url.PathEscape(sobject))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getRecord(ctx context.Context, s *salesforce, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sobject := r.Str("sobject")
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	fields, _ := mcp.ArgStr(args, "fields")
	q := ""
	if fields != "" {
		q = "?fields=" + url.QueryEscape(fields)
	}
	data, err := s.get(ctx, "%s/sobjects/%s/%s%s", s.ver(), url.PathEscape(sobject), url.PathEscape(id), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createRecord(ctx context.Context, s *salesforce, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sobject := r.Str("sobject")
	dataStr := r.Str("data")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var body any
	if err := json.Unmarshal([]byte(dataStr), &body); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid JSON for data: %w", err))
	}
	path := fmt.Sprintf("%s/sobjects/%s/", s.ver(), url.PathEscape(sobject))
	data, err := s.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateRecord(ctx context.Context, s *salesforce, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sobject := r.Str("sobject")
	id := r.Str("id")
	dataStr := r.Str("data")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var body any
	if err := json.Unmarshal([]byte(dataStr), &body); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid JSON for data: %w", err))
	}
	path := fmt.Sprintf("%s/sobjects/%s/%s", s.ver(), url.PathEscape(sobject), url.PathEscape(id))
	data, err := s.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteRecord(ctx context.Context, s *salesforce, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sobject := r.Str("sobject")
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.del(ctx, "%s/sobjects/%s/%s", s.ver(), url.PathEscape(sobject), url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getRecordByExternalID(ctx context.Context, s *salesforce, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sobject := r.Str("sobject")
	field := r.Str("field")
	value := r.Str("value")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "%s/sobjects/%s/%s/%s", s.ver(), url.PathEscape(sobject), url.PathEscape(field), url.PathEscape(value))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func upsertByExternalID(ctx context.Context, s *salesforce, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sobject := r.Str("sobject")
	field := r.Str("field")
	value := r.Str("value")
	dataStr := r.Str("data")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var body any
	if err := json.Unmarshal([]byte(dataStr), &body); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid JSON for data: %w", err))
	}
	path := fmt.Sprintf("%s/sobjects/%s/%s/%s", s.ver(), url.PathEscape(sobject), url.PathEscape(field), url.PathEscape(value))
	data, err := s.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
