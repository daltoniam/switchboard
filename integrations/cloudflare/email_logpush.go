package cloudflare

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// --- Email Routing ---

func listEmailRoutingRules(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	zoneID := r.Str("zone_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/zones/%s/email/routing/rules", zoneID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listEmailRoutingAddresses(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	verified, _ := mcp.ArgStr(args, "verified")
	q := queryEncode(map[string]string{
		"verified": verified,
		"page":     fmt.Sprintf("%d", mcp.OptInt(args, "page", 1)),
		"per_page": fmt.Sprintf("%d", mcp.OptInt(args, "per_page", 20)),
	})
	data, err := c.get(ctx, "/accounts/%s/email/routing/addresses%s", acct, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getEmailRoutingSettings(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	zoneID := r.Str("zone_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/zones/%s/email/routing", zoneID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Logpush (account-scoped) ---

func listLogpushJobs(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/logpush/jobs", acct)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getLogpushJob(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	jobID := mcp.OptInt(args, "job_id", 0)
	if jobID == 0 {
		return mcp.ErrResult(fmt.Errorf("job_id (integer) is required"))
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/logpush/jobs/%d", acct, jobID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createLogpushJob(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	dataset := r.Str("dataset")
	destinationConf := r.Str("destination_conf")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"dataset":          dataset,
		"destination_conf": destinationConf,
	}
	if name, _ := mcp.ArgStr(args, "name"); name != "" {
		body["name"] = name
	}
	if frequency, _ := mcp.ArgStr(args, "frequency"); frequency != "" {
		body["frequency"] = frequency
	}
	if logpullOptions, _ := mcp.ArgStr(args, "logpull_options"); logpullOptions != "" {
		body["logpull_options"] = logpullOptions
	}
	if outputOptions, _ := mcp.ArgMap(args, "output_options"); outputOptions != nil {
		body["output_options"] = outputOptions
	}
	if filter, _ := mcp.ArgStr(args, "filter"); filter != "" {
		body["filter"] = filter
	}
	if enabled, err := mcp.ArgBool(args, "enabled"); err == nil && hasKey(args, "enabled") {
		body["enabled"] = enabled
	}
	data, err := c.post(ctx, fmt.Sprintf("/accounts/%s/logpush/jobs", acct), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
