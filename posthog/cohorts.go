package posthog

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listCohorts(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"limit":  argStr(args, "limit"),
		"offset": argStr(args, "offset"),
	})
	data, err := p.get(ctx, "/api/projects/%s/cohorts/%s", p.proj(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getCohort(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	data, err := p.get(ctx, "/api/projects/%s/cohorts/%s/", p.proj(args), argStr(args, "cohort_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createCohort(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{"name": argStr(args, "name")}
	if v := argStr(args, "description"); v != "" {
		body["description"] = v
	}
	if filters, err := parseJSON(args, "filters"); err != nil {
		return errResult(err)
	} else if filters != nil {
		body["filters"] = filters
	}
	if _, ok := args["is_static"]; ok {
		body["is_static"] = argBool(args, "is_static")
	}
	path := fmt.Sprintf("/api/projects/%s/cohorts/", p.proj(args))
	data, err := p.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateCohort(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if v := argStr(args, "name"); v != "" {
		body["name"] = v
	}
	if v := argStr(args, "description"); v != "" {
		body["description"] = v
	}
	if filters, err := parseJSON(args, "filters"); err != nil {
		return errResult(err)
	} else if filters != nil {
		body["filters"] = filters
	}
	path := fmt.Sprintf("/api/projects/%s/cohorts/%s/", p.proj(args), argStr(args, "cohort_id"))
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteCohort(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	path := fmt.Sprintf("/api/projects/%s/cohorts/%s/", p.proj(args), argStr(args, "cohort_id"))
	body := map[string]any{"deleted": true}
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listCohortPersons(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"limit":  argStr(args, "limit"),
		"offset": argStr(args, "offset"),
	})
	data, err := p.get(ctx, "/api/projects/%s/cohorts/%s/persons/%s", p.proj(args), argStr(args, "cohort_id"), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
