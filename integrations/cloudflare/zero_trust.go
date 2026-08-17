package cloudflare

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// --- Zero Trust Access ---

func listAccessApps(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"page":     fmt.Sprintf("%d", mcp.OptInt(args, "page", 1)),
		"per_page": fmt.Sprintf("%d", mcp.OptInt(args, "per_page", 25)),
	})
	data, err := c.get(ctx, "/accounts/%s/access/apps%s", acct, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listAccessAppPolicies(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	appID := r.Str("app_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/access/apps/%s/policies", acct, appID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listAccessIdentityProviders(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/access/identity_providers", acct)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Cloudflared (Zero Trust) Tunnels ---

func listTunnels(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	name, _ := mcp.ArgStr(args, "name")
	status, _ := mcp.ArgStr(args, "status")
	includeDeleted, _ := mcp.ArgStr(args, "include_deleted")
	q := queryEncode(map[string]string{
		"name":            name,
		"status":          status,
		"include_deleted": includeDeleted,
		"page":            fmt.Sprintf("%d", mcp.OptInt(args, "page", 1)),
		"per_page":        fmt.Sprintf("%d", mcp.OptInt(args, "per_page", 20)),
	})
	data, err := c.get(ctx, "/accounts/%s/cfd_tunnel%s", acct, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTunnel(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tunnelID := r.Str("tunnel_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/cfd_tunnel/%s", acct, tunnelID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteTunnel(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tunnelID := r.Str("tunnel_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.del(ctx, "/accounts/%s/cfd_tunnel/%s", acct, tunnelID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
