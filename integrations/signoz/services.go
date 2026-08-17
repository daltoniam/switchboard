package signoz

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func listServices(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	start := r.Str("start")
	end := r.Str("end")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if _, _, err := parseTimeRange(start, end); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.post(ctx, "/api/v2/services", map[string]any{
		"start": start,
		"end":   end,
		"tags":  []any{},
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(unwrapData(data))
}

func getServiceOverview(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	service := r.Str("service")
	start := r.Str("start")
	end := r.Str("end")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	step := r.OptInt("step", 60)

	startMs, endMs, err := parseTimeRange(start, end)
	if err != nil {
		return mcp.ErrResult(err)
	}

	data, err := s.post(ctx, "/api/v4/query_range", buildBuilderQuery(
		"graph",
		map[string]any{
			"dataSource":         "metrics",
			"queryName":          "A",
			"aggregateOperator":  "rate",
			"aggregateAttribute": map[string]any{"key": "signoz_calls_total", "dataType": "float64", "type": "Sum", "isColumn": true},
			"filters": map[string]any{
				"op": "AND",
				"items": []any{
					map[string]any{"key": map[string]any{"key": "service_name", "dataType": "string", "type": "tag"}, "op": "=", "value": service},
				},
			},
			"expression":   "A",
			"disabled":     false,
			"stepInterval": step,
			"having":       []any{},
			"groupBy":      []any{},
		},
		startMs, endMs, step,
	))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(unwrapData(data))
}

func topOperations(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	service := r.Str("service")
	start := r.Str("start")
	end := r.Str("end")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if _, _, err := parseTimeRange(start, end); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.post(ctx, "/api/v2/service/top_operations", map[string]any{
		"start":   start,
		"end":     end,
		"service": service,
		"tags":    []any{},
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(unwrapData(data))
}

func topLevelOperations(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	start := r.Str("start")
	end := r.Str("end")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if _, _, err := parseTimeRange(start, end); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.post(ctx, "/api/v1/service/top_level_operations", map[string]any{
		"start": start,
		"end":   end,
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(unwrapData(data))
}

func entryPointOperations(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	service := r.Str("service")
	start := r.Str("start")
	end := r.Str("end")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if _, _, err := parseTimeRange(start, end); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.post(ctx, "/api/v2/service/entry_point_operations", map[string]any{
		"start":   start,
		"end":     end,
		"service": service,
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(unwrapData(data))
}

// buildBuilderQuery constructs the v4 query_range composite query payload.
func buildBuilderQuery(panelType string, query map[string]any, start, end int64, step int) map[string]any {
	return map[string]any{
		"start": start,
		"end":   end,
		"step":  step,
		"compositeQuery": map[string]any{
			"queryType": "builder",
			"panelType": panelType,
			"builderQueries": map[string]any{
				"A": query,
			},
		},
	}
}

// parseFilterItems builds a SigNoz filter object from optional filter string and service name.
// Filter format: "key op value", e.g. "severity_text = 'ERROR'".
func parseFilterItems(filter, service string) (map[string]any, error) {
	items := []any{}

	if service != "" {
		items = append(items, map[string]any{
			"key":   map[string]any{"key": "serviceName", "dataType": "string", "type": "tag", "isColumn": true},
			"op":    "=",
			"value": service,
		})
	}

	if filter != "" {
		parts := strings.SplitN(filter, " ", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid filter format %q: expected \"key op value\" (e.g. \"severity_text = 'ERROR'\")", filter)
		}
		key := parts[0]
		op := parts[1]
		value := strings.Trim(parts[2], "'\"")
		items = append(items, map[string]any{
			"key":   map[string]any{"key": key, "dataType": "string", "type": "tag", "isColumn": false},
			"op":    op,
			"value": value,
		})
	}

	return map[string]any{"op": "AND", "items": items}, nil
}

func strMs(s string) (int64, error) {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid epoch milliseconds %q: %w", s, err)
	}
	return v, nil
}

// parseTimeRange validates and parses start/end epoch millisecond strings.
func parseTimeRange(start, end string) (int64, int64, error) {
	s, err := strMs(start)
	if err != nil {
		return 0, 0, err
	}
	e, err := strMs(end)
	if err != nil {
		return 0, 0, err
	}
	return s, e, nil
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
