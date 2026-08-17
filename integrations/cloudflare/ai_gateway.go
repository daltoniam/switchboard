package cloudflare

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// --- AI Gateway: gateways ---

func listAIGateways(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"page":     fmt.Sprintf("%d", mcp.OptInt(args, "page", 1)),
		"per_page": fmt.Sprintf("%d", mcp.OptInt(args, "per_page", 20)),
	})
	data, err := c.get(ctx, "/accounts/%s/ai-gateway/gateways%s", acct, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAIGateway(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	gatewayID := r.Str("gateway_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/ai-gateway/gateways/%s", acct, gatewayID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- AI Gateway: logs (token usage + cost) ---

func listAIGatewayLogs(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	gatewayID := r.Str("gateway_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	// All filters are optional; ArgStr returns "" for missing keys, and
	// queryEncode drops empty values so upstream defaults are preserved.
	startDate, _ := mcp.ArgStr(args, "start_date")
	endDate, _ := mcp.ArgStr(args, "end_date")
	provider, _ := mcp.ArgStr(args, "provider")
	model, _ := mcp.ArgStr(args, "model")
	success, _ := mcp.ArgStr(args, "success")
	cached, _ := mcp.ArgStr(args, "cached")
	feedback, _ := mcp.ArgStr(args, "feedback")
	search, _ := mcp.ArgStr(args, "search")
	orderBy, _ := mcp.ArgStr(args, "order_by")
	orderByDir, _ := mcp.ArgStr(args, "order_by_direction")
	q := queryEncode(map[string]string{
		"start_date":         startDate,
		"end_date":           endDate,
		"provider":           provider,
		"model":              model,
		"success":            success,
		"cached":             cached,
		"feedback":           feedback,
		"search":             search,
		"order_by":           orderBy,
		"order_by_direction": orderByDir,
		"page":               fmt.Sprintf("%d", mcp.OptInt(args, "page", 1)),
		"per_page":           fmt.Sprintf("%d", mcp.OptInt(args, "per_page", 20)),
	})
	data, err := c.get(ctx, "/accounts/%s/ai-gateway/gateways/%s/logs%s", acct, gatewayID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAIGatewayLog(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	gatewayID := r.Str("gateway_id")
	logID := r.Str("log_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/ai-gateway/gateways/%s/logs/%s", acct, gatewayID, logID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAIGatewayLogRequest(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	gatewayID := r.Str("gateway_id")
	logID := r.Str("log_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/ai-gateway/gateways/%s/logs/%s/request", acct, gatewayID, logID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAIGatewayLogResponse(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	gatewayID := r.Str("gateway_id")
	logID := r.Str("log_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/ai-gateway/gateways/%s/logs/%s/response", acct, gatewayID, logID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
