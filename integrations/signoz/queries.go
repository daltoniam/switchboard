package signoz

import (
	"context"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func searchLogs(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	start := r.Str("start")
	end := r.Str("end")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	filter, _ := mcp.ArgStr(args, "filter")
	limit := clamp(r.OptInt("limit", 20), 1, 100)
	offset := r.OptInt("offset", 0)

	filters, err := parseFilterItems(filter, "")
	if err != nil {
		return mcp.ErrResult(err)
	}

	data, err := s.post(ctx, "/api/v4/query_range", buildBuilderQuery(
		"list",
		map[string]any{
			"dataSource":         "logs",
			"queryName":          "A",
			"aggregateOperator":  "noop",
			"aggregateAttribute": map[string]any{"key": "", "dataType": "", "type": "", "isColumn": false},
			"filters":            filters,
			"expression":         "A",
			"disabled":           false,
			"limit":              limit,
			"offset":             offset,
			"pageSize":           limit,
			"orderBy":            []any{map[string]any{"columnName": "timestamp", "order": "desc"}},
			"stepInterval":       60,
			"having":             []any{},
		},
		strMs(start), strMs(end), 60,
	))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchTraces(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	start := r.Str("start")
	end := r.Str("end")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	service, _ := mcp.ArgStr(args, "service")
	filter, _ := mcp.ArgStr(args, "filter")
	limit := clamp(r.OptInt("limit", 20), 1, 100)
	offset := r.OptInt("offset", 0)

	filters, err := parseFilterItems(filter, service)
	if err != nil {
		return mcp.ErrResult(err)
	}

	data, err := s.post(ctx, "/api/v4/query_range", buildBuilderQuery(
		"list",
		map[string]any{
			"dataSource":         "traces",
			"queryName":          "A",
			"aggregateOperator":  "noop",
			"aggregateAttribute": map[string]any{"key": "", "dataType": "", "type": "", "isColumn": false},
			"filters":            filters,
			"expression":         "A",
			"disabled":           false,
			"limit":              limit,
			"offset":             offset,
			"pageSize":           limit,
			"selectColumns": []any{
				map[string]any{"key": "serviceName", "dataType": "string", "type": "tag", "isColumn": true},
				map[string]any{"key": "name", "dataType": "string", "type": "tag", "isColumn": true},
				map[string]any{"key": "durationNano", "dataType": "float64", "type": "tag", "isColumn": true},
				map[string]any{"key": "httpMethod", "dataType": "string", "type": "tag", "isColumn": true},
				map[string]any{"key": "responseStatusCode", "dataType": "string", "type": "tag", "isColumn": true},
				map[string]any{"key": "traceID", "dataType": "string", "type": "tag", "isColumn": true},
				map[string]any{"key": "spanID", "dataType": "string", "type": "tag", "isColumn": true},
				map[string]any{"key": "hasError", "dataType": "bool", "type": "tag", "isColumn": true},
			},
			"orderBy":      []any{map[string]any{"columnName": "timestamp", "order": "desc"}},
			"stepInterval": 60,
			"having":       []any{},
		},
		strMs(start), strMs(end), 60,
	))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTrace(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	traceID := r.Str("trace_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/api/v1/traces/%s", traceID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func queryMetrics(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	start := r.Str("start")
	end := r.Str("end")
	metricName := r.Str("metric_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	aggregateOp, _ := mcp.ArgStr(args, "aggregate_op")
	if aggregateOp == "" {
		aggregateOp = "avg"
	}
	filter, _ := mcp.ArgStr(args, "filter")
	groupByStr, _ := mcp.ArgStr(args, "group_by")
	step := r.OptInt("step", 60)

	groupBy := []any{}
	if groupByStr != "" {
		for _, g := range strings.Split(groupByStr, ",") {
			g = strings.TrimSpace(g)
			if g != "" {
				groupBy = append(groupBy, map[string]any{
					"key": g, "dataType": "string", "type": "tag", "isColumn": false,
				})
			}
		}
	}

	filters, err := parseFilterItems(filter, "")
	if err != nil {
		return mcp.ErrResult(err)
	}

	data, err := s.post(ctx, "/api/v4/query_range", buildBuilderQuery(
		"graph",
		map[string]any{
			"dataSource":         "metrics",
			"queryName":          "A",
			"aggregateOperator":  aggregateOp,
			"aggregateAttribute": map[string]any{"key": metricName, "dataType": "float64", "type": "Sum", "isColumn": true},
			"filters":            filters,
			"expression":         "A",
			"disabled":           false,
			"stepInterval":       step,
			"having":             []any{},
			"groupBy":            groupBy,
		},
		strMs(start), strMs(end), step,
	))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
