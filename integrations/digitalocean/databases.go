package digitalocean

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
)

func listDatabases(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	opt := listOpts(args)
	dbs, _, err := d.client.Databases.List(ctx, opt)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(dbs)
}

func getDatabase(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "database_id")
	db, _, err := d.client.Databases.Get(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(db)
}

func listDatabaseDBs(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "database_id")
	dbs, _, err := d.client.Databases.ListDBs(ctx, id, listOpts(args))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(dbs)
}

func listDatabaseUsers(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "database_id")
	users, _, err := d.client.Databases.ListUsers(ctx, id, listOpts(args))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(users)
}

func listDatabasePools(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "database_id")
	pools, _, err := d.client.Databases.ListPools(ctx, id, listOpts(args))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(pools)
}
