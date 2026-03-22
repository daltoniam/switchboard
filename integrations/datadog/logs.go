package datadog

import (
	"context"
	"math"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	mcp "github.com/daltoniam/switchboard"
)

func searchLogs(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV2.NewLogsApi(d.client)

	from := parseTime(r.Str("from"), -time.Hour)
	to := parseTime(r.Str("to"), 0)
	limit := int32(min(mcp.OptInt(args, "limit", 50), math.MaxInt32))
	query := r.Str("query")
	sortStr := r.Str("sort")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	sort := datadogV2.LOGSSORT_TIMESTAMP_DESCENDING
	if sortStr == "timestamp" {
		sort = datadogV2.LOGSSORT_TIMESTAMP_ASCENDING
	}

	body := datadogV2.LogsListRequest{
		Filter: &datadogV2.LogsQueryFilter{
			Query: &query,
			From:  datadog.PtrString(from.Format(time.RFC3339)),
			To:    datadog.PtrString(to.Format(time.RFC3339)),
		},
		Page: &datadogV2.LogsListRequestPage{
			Limit: &limit,
		},
		Sort: &sort,
	}

	resp, _, err := api.ListLogs(ctx, *datadogV2.NewListLogsOptionalParameters().WithBody(body))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func aggregateLogs(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV2.NewLogsApi(d.client)

	from := parseTime(r.Str("from"), -time.Hour)
	to := parseTime(r.Str("to"), 0)
	query := r.Str("query")

	computeType, _ := datadogV2.NewLogsAggregationFunctionFromValue(r.Str("compute_type"))
	if computeType == nil {
		ct := datadogV2.LOGSAGGREGATIONFUNCTION_COUNT
		computeType = &ct
	}

	compute := datadogV2.LogsCompute{
		Aggregation: *computeType,
	}
	if field := r.Str("compute_field"); field != "" {
		compute.Metric = &field
	}

	body := datadogV2.LogsAggregateRequest{
		Filter: &datadogV2.LogsQueryFilter{
			Query: &query,
			From:  datadog.PtrString(from.Format(time.RFC3339)),
			To:    datadog.PtrString(to.Format(time.RFC3339)),
		},
		Compute: []datadogV2.LogsCompute{compute},
	}

	groupBy := r.Str("group_by")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if groupBy != "" {
		body.GroupBy = []datadogV2.LogsGroupBy{
			{Facet: groupBy},
		}
	}

	resp, _, err := api.AggregateLogs(ctx, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}
