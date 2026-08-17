package vercel

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listProjectEnvVars(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectIDOrName := r.Str("project_id_or_name")
	teamID, teamSlug := optionalScopeArgs(r)
	params := paginationArgs(r, 20)
	if gitBranch := r.Str("git_branch"); gitBranch != "" {
		params["gitBranch"] = gitBranch
	}
	if target := r.Str("target"); target != "" {
		params["target"] = target
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(projectIDOrName, "project_id_or_name"); err != nil {
		return mcp.ErrResult(err)
	}
	params["teamId"] = teamID
	params["slug"] = teamSlug
	data, err := v.get(ctx, "/v10/projects/%s/env%s", url.PathEscape(projectIDOrName), queryEncode(v.scopedQuery(params)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createProjectEnvVars(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectIDOrName := r.Str("project_id_or_name")
	teamID, teamSlug := optionalScopeArgs(r)
	upsert := r.Bool("upsert")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	envs, err := envVarsArg(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(projectIDOrName, "project_id_or_name"); err != nil {
		return mcp.ErrResult(err)
	}
	if len(envs) == 0 {
		return mcp.ErrResult(required("", "envs"))
	}
	params := v.scopedQuery(map[string]string{"teamId": teamID, "slug": teamSlug})
	if upsert {
		params["upsert"] = "1"
	}
	data, err := v.post(ctx, "/v10/projects/"+url.PathEscape(projectIDOrName)+"/env"+queryEncode(params), map[string]any{"envs": envs})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateProjectEnvVar(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectIDOrName := r.Str("project_id_or_name")
	envID := r.Str("env_id")
	teamID, teamSlug := optionalScopeArgs(r)
	body := r.Map("body")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(projectIDOrName, "project_id_or_name"); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(envID, "env_id"); err != nil {
		return mcp.ErrResult(err)
	}
	if body == nil {
		return mcp.ErrResult(required("", "body"))
	}
	params := v.scopedQuery(map[string]string{"teamId": teamID, "slug": teamSlug})
	data, err := v.patch(ctx, "/v9/projects/"+url.PathEscape(projectIDOrName)+"/env/"+url.PathEscape(envID)+queryEncode(params), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteProjectEnvVar(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectIDOrName := r.Str("project_id_or_name")
	envID := r.Str("env_id")
	teamID, teamSlug := optionalScopeArgs(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(projectIDOrName, "project_id_or_name"); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(envID, "env_id"); err != nil {
		return mcp.ErrResult(err)
	}
	params := v.scopedQuery(map[string]string{"teamId": teamID, "slug": teamSlug})
	data, err := v.del(ctx, "/v9/projects/%s/env/%s%s", url.PathEscape(projectIDOrName), url.PathEscape(envID), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func envVarsArg(args map[string]any) ([]map[string]any, error) {
	value, ok := args["envs"]
	if !ok || value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case []map[string]any:
		return typed, nil
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for i, item := range typed {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("parameter %q: element %d is %T, not map", "envs", i, item)
			}
			out = append(out, m)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("parameter %q: cannot convert %T to env var list", "envs", value)
	}
}
