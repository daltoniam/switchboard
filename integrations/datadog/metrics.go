package datadog

import (
	"context"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	mcp "github.com/daltoniam/switchboard"
)

func queryMetrics(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewMetricsApi(d.client)

	from := parseTime(argStr(args, "from"), -3600_000_000_000) // -1h in ns
	to := parseTime(argStr(args, "to"), 0)
	query := argStr(args, "query")

	resp, _, err := api.QueryMetrics(ctx, from.Unix(), to.Unix(), query)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func listActiveMetrics(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewMetricsApi(d.client)

	from := optInt64(args, "from", parseTime("", -3600_000_000_000).Unix())
	opts := datadogV1.NewListActiveMetricsOptionalParameters()
	if v := argStr(args, "host"); v != "" {
		opts = opts.WithHost(v)
	}
	if v := argStr(args, "tag_filter"); v != "" {
		opts = opts.WithTagFilter(v)
	}

	resp, _, err := api.ListActiveMetrics(ctx, from, *opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func searchMetrics(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewMetricsApi(d.client)
	resp, _, err := api.ListMetrics(ctx, argStr(args, "query"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func getMetricMetadata(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewMetricsApi(d.client)
	resp, _, err := api.GetMetricMetadata(ctx, argStr(args, "metric"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}
