package gcp

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func iamListServiceAccounts(ctx context.Context, g *integration, _ map[string]any) (*mcp.ToolResult, error) {
	resp, err := g.iamService.Projects.ServiceAccounts.List(g.projectName()).Context(ctx).Do()
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(resp.Accounts)
}

func iamGetServiceAccount(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	email := r.Str("email")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	name := fmt.Sprintf("projects/%s/serviceAccounts/%s", g.projectID, email)
	sa, err := g.iamService.Projects.ServiceAccounts.Get(name).Context(ctx).Do()
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(sa)
}

func iamListServiceAccountKeys(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	email := r.Str("email")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	name := fmt.Sprintf("projects/%s/serviceAccounts/%s", g.projectID, email)
	resp, err := g.iamService.Projects.ServiceAccounts.Keys.List(name).Context(ctx).Do()
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(resp.Keys)
}

func iamListRoles(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	call := g.iamService.Roles.List()
	if r.Bool("show_deleted") {
		call = call.ShowDeleted(true)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, err := call.Context(ctx).Do()
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(resp.Roles)
}

func iamGetRole(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	role, err := g.iamService.Roles.Get(name).Context(ctx).Do()
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(role)
}
