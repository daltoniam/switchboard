package cloudflare

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listWorkers(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/workers/scripts", acct)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getWorker(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	scriptName := r.Str("script_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/workers/scripts/%s", acct, scriptName)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteWorker(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	scriptName := r.Str("script_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.del(ctx, "/accounts/%s/workers/scripts/%s", acct, scriptName)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listWorkerRoutes(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	zoneID := r.Str("zone_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/zones/%s/workers/routes", zoneID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Pages ---

func listPagesProjects(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/pages/projects", acct)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getPagesProject(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectName := r.Str("project_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/pages/projects/%s", acct, projectName)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listPagesDeployments(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectName := r.Str("project_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/pages/projects/%s/deployments", acct, projectName)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getPagesDeployment(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectName := r.Str("project_name")
	deploymentID := r.Str("deployment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/pages/projects/%s/deployments/%s",
		acct, projectName, deploymentID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deletePagesDeployment(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectName := r.Str("project_name")
	deploymentID := r.Str("deployment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.del(ctx, "/accounts/%s/pages/projects/%s/deployments/%s",
		acct, projectName, deploymentID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func rollbackPagesDeployment(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectName := r.Str("project_name")
	deploymentID := r.Str("deployment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/accounts/%s/pages/projects/%s/deployments/%s/rollback",
		acct, projectName, deploymentID)
	data, err := c.post(ctx, path, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
