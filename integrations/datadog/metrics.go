package datadog

import (
	"context"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	mcp "github.com/daltoniam/switchboard"
)

func queryMetrics(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV1.NewMetricsApi(d.client)

	from := parseTime(r.Str("from"), -3600_000_000_000) // -1h in ns
	to := parseTime(r.Str("to"), 0)
	query := r.Str("query")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	resp, _, err := api.QueryMetrics(ctx, from.Unix(), to.Unix(), query)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func listActiveMetrics(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV1.NewMetricsApi(d.client)

	from := mcp.OptInt64(args, "from", parseTime("", -3600_000_000_000).Unix())
	opts := datadogV1.NewListActiveMetricsOptionalParameters()
	if v := r.Str("host"); v != "" {
		opts = opts.WithHost(v)
	}
	if v := r.Str("tag_filter"); v != "" {
		opts = opts.WithTagFilter(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	resp, _, err := api.ListActiveMetrics(ctx, from, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func searchMetrics(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV1.NewMetricsApi(d.client)
	resp, _, err := api.ListMetrics(ctx, query)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getMetricMetadata(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	metric := r.Str("metric")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV1.NewMetricsApi(d.client)
	resp, _, err := api.GetMetricMetadata(ctx, metric)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}
