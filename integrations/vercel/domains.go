package vercel

import (
	"context"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func addProjectDomain(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectIDOrName := r.Str("project_id_or_name")
	domain := r.Str("domain")
	teamID, teamSlug := optionalScopeArgs(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(projectIDOrName, "project_id_or_name"); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(domain, "domain"); err != nil {
		return mcp.ErrResult(err)
	}
	params := v.scopedQuery(map[string]string{"teamId": teamID, "slug": teamSlug})
	data, err := v.post(ctx, "/v10/projects/"+url.PathEscape(projectIDOrName)+"/domains"+queryEncode(params), map[string]string{"name": domain})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func removeProjectDomain(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectIDOrName := r.Str("project_id_or_name")
	domain := r.Str("domain")
	teamID, teamSlug := optionalScopeArgs(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(projectIDOrName, "project_id_or_name"); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(domain, "domain"); err != nil {
		return mcp.ErrResult(err)
	}
	params := v.scopedQuery(map[string]string{"teamId": teamID, "slug": teamSlug})
	data, err := v.del(ctx, "/v9/projects/%s/domains/%s%s", url.PathEscape(projectIDOrName), url.PathEscape(domain), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getDomainConfig(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	domain := r.Str("domain")
	projectIDOrName := r.Str("project_id_or_name")
	strict := r.Bool("strict")
	teamID, teamSlug := optionalScopeArgs(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(domain, "domain"); err != nil {
		return mcp.ErrResult(err)
	}
	params := v.scopedQuery(map[string]string{"teamId": teamID, "slug": teamSlug})
	if projectIDOrName != "" {
		params["projectIdOrName"] = projectIDOrName
	}
	if strict {
		params["strict"] = "true"
	}
	data, err := v.get(ctx, "/v6/domains/%s/config%s", url.PathEscape(domain), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listDeploymentAliases(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
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
	data, err := v.get(ctx, "/v2/deployments/%s/aliases%s", url.PathEscape(deploymentID), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func assignDeploymentAlias(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	deploymentID := r.Str("deployment_id")
	alias := r.Str("alias")
	teamID, teamSlug := optionalScopeArgs(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(deploymentID, "deployment_id"); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(alias, "alias"); err != nil {
		return mcp.ErrResult(err)
	}
	params := v.scopedQuery(map[string]string{"teamId": teamID, "slug": teamSlug})
	data, err := v.post(ctx, "/v2/deployments/"+url.PathEscape(deploymentID)+"/aliases"+queryEncode(params), map[string]string{"alias": alias})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteDeploymentAlias(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	aliasID := r.Str("alias_id")
	teamID, teamSlug := optionalScopeArgs(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(aliasID, "alias_id"); err != nil {
		return mcp.ErrResult(err)
	}
	params := v.scopedQuery(map[string]string{"teamId": teamID, "slug": teamSlug})
	data, err := v.del(ctx, "/v2/aliases/%s%s", url.PathEscape(aliasID), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
