package cloudflare

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// --- Queues ---

func listQueues(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"page":     fmt.Sprintf("%d", mcp.OptInt(args, "page", 1)),
		"per_page": fmt.Sprintf("%d", mcp.OptInt(args, "per_page", 20)),
	})
	data, err := c.get(ctx, "/accounts/%s/queues%s", acct, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getQueue(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	queueID := r.Str("queue_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/queues/%s", acct, queueID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createQueue(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	queueName := r.Str("queue_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"queue_name": queueName}
	if settings, _ := mcp.ArgMap(args, "settings"); settings != nil {
		body["settings"] = settings
	}
	data, err := c.post(ctx, fmt.Sprintf("/accounts/%s/queues", acct), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteQueue(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	queueID := r.Str("queue_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.del(ctx, "/accounts/%s/queues/%s", acct, queueID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func sendQueueMessages(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	queueID := r.Str("queue_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	msgs, ok := args["messages"]
	if !ok || msgs == nil {
		return mcp.ErrResult(fmt.Errorf("messages (array of {body: ...} objects) is required"))
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"messages": msgs}
	data, err := c.post(ctx, fmt.Sprintf("/accounts/%s/queues/%s/messages", acct, queueID), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Hyperdrive ---

func listHyperdriveConfigs(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/hyperdrive/configs", acct)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getHyperdriveConfig(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	hyperdriveID := r.Str("hyperdrive_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/hyperdrive/configs/%s", acct, hyperdriveID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createHyperdriveConfig(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	origin, _ := mcp.ArgMap(args, "origin")
	if origin == nil {
		return mcp.ErrResult(fmt.Errorf("origin (connection details map) is required"))
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"name": name, "origin": origin}
	if caching, _ := mcp.ArgMap(args, "caching"); caching != nil {
		body["caching"] = caching
	}
	if mtls, _ := mcp.ArgMap(args, "mtls"); mtls != nil {
		body["mtls"] = mtls
	}
	data, err := c.post(ctx, fmt.Sprintf("/accounts/%s/hyperdrive/configs", acct), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteHyperdriveConfig(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	hyperdriveID := r.Str("hyperdrive_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.del(ctx, "/accounts/%s/hyperdrive/configs/%s", acct, hyperdriveID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
