package cloudflare

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// --- Workers extras ---

func listWorkerSecrets(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	scriptName := r.Str("script_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/workers/scripts/%s/secrets", acct, scriptName)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listWorkerDeployments(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	scriptName := r.Str("script_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/workers/scripts/%s/deployments", acct, scriptName)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getWorkerSubdomain(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/workers/subdomain", acct)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listWorkerTails(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	scriptName := r.Str("script_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/workers/scripts/%s/tails", acct, scriptName)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Pages: create project / deployment / domains ---

func createPagesProject(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	productionBranch := r.Str("production_branch")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"name":              name,
		"production_branch": productionBranch,
	}
	if buildConfig, _ := mcp.ArgMap(args, "build_config"); buildConfig != nil {
		body["build_config"] = buildConfig
	}
	if source, _ := mcp.ArgMap(args, "source"); source != nil {
		body["source"] = source
	}
	if deploymentConfigs, _ := mcp.ArgMap(args, "deployment_configs"); deploymentConfigs != nil {
		body["deployment_configs"] = deploymentConfigs
	}
	data, err := c.post(ctx, fmt.Sprintf("/accounts/%s/pages/projects", acct), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createPagesDeployment(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectName := r.Str("project_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	branch, _ := mcp.ArgStr(args, "branch")
	body := map[string]any{}
	if branch != "" {
		body["branch"] = branch
	}
	data, err := c.post(ctx, fmt.Sprintf("/accounts/%s/pages/projects/%s/deployments", acct, projectName), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listPagesDomains(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectName := r.Str("project_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/pages/projects/%s/domains", acct, projectName)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- KV bulk delete ---

func bulkDeleteKVValues(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	namespaceID := r.Str("namespace_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	keys, err := mcp.ArgStrSlice(args, "keys")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if len(keys) == 0 {
		return mcp.ErrResult(fmt.Errorf("keys (non-empty array of strings) is required"))
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.post(ctx, fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/bulk/delete", acct, namespaceID), keys)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
