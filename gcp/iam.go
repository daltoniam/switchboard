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
	return jsonResult(resp.Accounts)
}

func iamGetServiceAccount(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	email := argStr(args, "email")
	name := fmt.Sprintf("projects/%s/serviceAccounts/%s", g.projectID, email)
	sa, err := g.iamService.Projects.ServiceAccounts.Get(name).Context(ctx).Do()
	if err != nil {
		return errResult(err)
	}
	return jsonResult(sa)
}

func iamListServiceAccountKeys(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	email := argStr(args, "email")
	name := fmt.Sprintf("projects/%s/serviceAccounts/%s", g.projectID, email)
	resp, err := g.iamService.Projects.ServiceAccounts.Keys.List(name).Context(ctx).Do()
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp.Keys)
}

func iamListRoles(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	call := g.iamService.Roles.List()
	if argBool(args, "show_deleted") {
		call = call.ShowDeleted(true)
	}
	resp, err := call.Context(ctx).Do()
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp.Roles)
}

func iamGetRole(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	role, err := g.iamService.Roles.Get(argStr(args, "name")).Context(ctx).Do()
	if err != nil {
		return errResult(err)
	}
	return jsonResult(role)
}
