package posthog

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// ── Persons ─────────────────────────────────────────────────────────

func listPersons(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"search":      argStr(args, "search"),
		"distinct_id": argStr(args, "distinct_id"),
		"email":       argStr(args, "email"),
		"limit":       argStr(args, "limit"),
		"offset":      argStr(args, "offset"),
	})
	data, err := p.get(ctx, "/api/projects/%s/persons/%s", p.proj(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getPerson(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	data, err := p.get(ctx, "/api/projects/%s/persons/%s/", p.proj(args), argStr(args, "person_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deletePerson(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	data, err := p.del(ctx, "/api/projects/%s/persons/%s/", p.proj(args), argStr(args, "person_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updatePersonProperty(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{"key": argStr(args, "key"), "value": argStr(args, "value")}
	path := fmt.Sprintf("/api/projects/%s/persons/%s/update_property/", p.proj(args), argStr(args, "person_id"))
	data, err := p.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deletePersonProperty(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{"$unset": argStr(args, "key")}
	path := fmt.Sprintf("/api/projects/%s/persons/%s/delete_property/", p.proj(args), argStr(args, "person_id"))
	data, err := p.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Groups ──────────────────────────────────────────────────────────

func listGroups(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"group_type_index": argStr(args, "group_type_index"),
		"search":           argStr(args, "search"),
		"cursor":           argStr(args, "cursor"),
	})
	data, err := p.get(ctx, "/api/projects/%s/groups/%s", p.proj(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func findGroup(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"group_type_index": argStr(args, "group_type_index"),
		"group_key":        argStr(args, "group_key"),
	})
	data, err := p.get(ctx, "/api/projects/%s/groups/find/%s", p.proj(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
