package posthog

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listCohorts(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	q := queryEncode(map[string]string{
		"limit":  r.Str("limit"),
		"offset": r.Str("offset"),
	})
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/cohorts/%s", projID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getCohort(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	cohortID := r.Str("cohort_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/cohorts/%s/", projID, cohortID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createCohort(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	name := r.Str("name")
	description := r.Str("description")
	isStatic := r.Bool("is_static")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"name": name}
	if description != "" {
		body["description"] = description
	}
	if filters, err := parseJSON(args, "filters"); err != nil {
		return mcp.ErrResult(err)
	} else if filters != nil {
		body["filters"] = filters
	}
	if _, ok := args["is_static"]; ok {
		body["is_static"] = isStatic
	}
	path := fmt.Sprintf("/api/projects/%s/cohorts/", projID)
	data, err := p.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateCohort(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	cohortID := r.Str("cohort_id")
	name := r.Str("name")
	description := r.Str("description")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	if description != "" {
		body["description"] = description
	}
	if filters, err := parseJSON(args, "filters"); err != nil {
		return mcp.ErrResult(err)
	} else if filters != nil {
		body["filters"] = filters
	}
	path := fmt.Sprintf("/api/projects/%s/cohorts/%s/", projID, cohortID)
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteCohort(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	cohortID, err := mcp.ArgStr(args, "cohort_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/api/projects/%s/cohorts/%s/", projID, cohortID)
	body := map[string]any{"deleted": true}
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listCohortPersons(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	cohortID := r.Str("cohort_id")
	q := queryEncode(map[string]string{
		"limit":  r.Str("limit"),
		"offset": r.Str("offset"),
	})
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/cohorts/%s/persons/%s", projID, cohortID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
