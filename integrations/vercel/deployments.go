package vercel

import (
	"context"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listDeployments(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID, teamSlug := optionalScopeArgs(r)
	params := paginationArgs(r, 20)
	for _, key := range []string{"project_id", "app", "state", "target"} {
		if val := r.Str(key); val != "" {
			params[camelParam(key)] = val
		}
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params["teamId"] = teamID
	params["slug"] = teamSlug
	data, err := v.get(ctx, "/v6/deployments%s", queryEncode(v.scopedQuery(params)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getDeployment(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	deploymentID := r.Str("deployment_id")
	teamID, teamSlug := optionalScopeArgs(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(deploymentID, "deployment_id"); err != nil {
		return mcp.ErrResult(err)
	}
	params := v.scopedQuery(map[string]string{"teamId": teamID, "slug": teamSlug})
	data, err := v.get(ctx, "/v13/deployments/%s%s", url.PathEscape(deploymentID), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createDeployment(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID, teamSlug := optionalScopeArgs(r)
	body := r.Map("body")
	forceNew := r.Bool("force_new")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if body == nil {
		return mcp.ErrResult(required("", "body"))
	}
	params := v.scopedQuery(map[string]string{"teamId": teamID, "slug": teamSlug})
	if forceNew {
		params["forceNew"] = "1"
	}
	data, err := v.post(ctx, "/v13/deployments"+queryEncode(params), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func cancelDeployment(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	deploymentID := r.Str("deployment_id")
	teamID, teamSlug := optionalScopeArgs(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(deploymentID, "deployment_id"); err != nil {
		return mcp.ErrResult(err)
	}
	params := v.scopedQuery(map[string]string{"teamId": teamID, "slug": teamSlug})
	data, err := v.patch(ctx, "/v12/deployments/"+url.PathEscape(deploymentID)+"/cancel"+queryEncode(params), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteDeployment(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	deploymentID := r.Str("deployment_id")
	deploymentURL := r.Str("url")
	teamID, teamSlug := optionalScopeArgs(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(deploymentID, "deployment_id"); err != nil {
		return mcp.ErrResult(err)
	}
	params := v.scopedQuery(map[string]string{"teamId": teamID, "slug": teamSlug, "url": deploymentURL})
	data, err := v.del(ctx, "/v13/deployments/%s%s", url.PathEscape(deploymentID), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listDeploymentEvents(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	deploymentIDOrURL := r.Str("deployment_id_or_url")
	teamID, teamSlug := optionalScopeArgs(r)
	params := paginationArgs(r, 100)
	for _, key := range []string{"direction", "name", "status_code"} {
		if val := r.Str(key); val != "" {
			params[camelParam(key)] = val
		}
	}
	if r.Bool("builds") {
		params["builds"] = "1"
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(deploymentIDOrURL, "deployment_id_or_url"); err != nil {
		return mcp.ErrResult(err)
	}
	params["teamId"] = teamID
	params["slug"] = teamSlug
	data, err := v.get(ctx, "/v3/deployments/%s/events%s", url.PathEscape(deploymentIDOrURL), queryEncode(v.scopedQuery(params)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listRuntimeLogs(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectID := r.Str("project_id")
	deploymentID := r.Str("deployment_id")
	teamID, teamSlug := optionalScopeArgs(r)
	params := paginationArgs(r, 100)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(projectID, "project_id"); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(deploymentID, "deployment_id"); err != nil {
		return mcp.ErrResult(err)
	}
	params["teamId"] = teamID
	params["slug"] = teamSlug
	data, err := v.get(ctx, "/v1/projects/%s/deployments/%s/runtime-logs%s", url.PathEscape(projectID), url.PathEscape(deploymentID), queryEncode(v.scopedQuery(params)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func camelParam(key string) string {
	switch key {
	case "project_id":
		return "projectId"
	case "status_code":
		return "statusCode"
	default:
		return key
	}
}
