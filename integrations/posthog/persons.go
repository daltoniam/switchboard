package posthog

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// -- Persons --

func listPersons(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := queryEncode(map[string]string{
		"search":      r.Str("search"),
		"distinct_id": r.Str("distinct_id"),
		"email":       r.Str("email"),
		"limit":       r.Str("limit"),
		"offset":      r.Str("offset"),
	})
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/persons/%s", p.proj(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getPerson(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	personID, err := mcp.ArgStr(args, "person_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/persons/%s/", p.proj(args), personID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deletePerson(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	personID, err := mcp.ArgStr(args, "person_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.del(ctx, "/api/projects/%s/persons/%s/", p.proj(args), personID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updatePersonProperty(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	personID := r.Str("person_id")
	key := r.Str("key")
	value := r.Str("value")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"key": key, "value": value}
	path := fmt.Sprintf("/api/projects/%s/persons/%s/update_property/", p.proj(args), personID)
	data, err := p.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deletePersonProperty(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	personID := r.Str("person_id")
	key := r.Str("key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"$unset": key}
	path := fmt.Sprintf("/api/projects/%s/persons/%s/delete_property/", p.proj(args), personID)
	data, err := p.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// -- Groups --

func listGroups(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := queryEncode(map[string]string{
		"group_type_index": r.Str("group_type_index"),
		"search":           r.Str("search"),
		"cursor":           r.Str("cursor"),
	})
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/groups/%s", p.proj(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func findGroup(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := queryEncode(map[string]string{
		"group_type_index": r.Str("group_type_index"),
		"group_key":        r.Str("group_key"),
	})
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/groups/find/%s", p.proj(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
