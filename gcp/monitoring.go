package gcp

import (
	"context"
	"fmt"
	"time"

	monitoringpb "cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	mcp "github.com/daltoniam/switchboard"
)

func monitoringListMetricDescriptors(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := &monitoringpb.ListMetricDescriptorsRequest{
		Name: g.projectName(),
	}
	if v := argStr(args, "filter"); v != "" {
		req.Filter = v
	}

	var descriptors []map[string]any
	it := g.monitoringClient.ListMetricDescriptors(ctx, req)
	for i := 0; i < 500; i++ {
		d, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		descriptors = append(descriptors, map[string]any{
			"type":         d.Type,
			"display_name": d.DisplayName,
			"description":  d.Description,
			"metric_kind":  d.MetricKind.String(),
			"value_type":   d.ValueType.String(),
			"unit":         d.Unit,
		})
	}
	return jsonResult(descriptors)
}

func monitoringListTimeSeries(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	startStr := argStr(args, "start_time")
	startTime, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return errResult(fmt.Errorf("invalid start_time: %w", err))
	}

	endTime := time.Now().UTC()
	if v := argStr(args, "end_time"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return errResult(fmt.Errorf("invalid end_time: %w", err))
		}
		endTime = t
	}

	req := &monitoringpb.ListTimeSeriesRequest{
		Name:   g.projectName(),
		Filter: argStr(args, "filter"),
		Interval: &monitoringpb.TimeInterval{
			StartTime: timestamppb.New(startTime),
			EndTime:   timestamppb.New(endTime),
		},
		View: monitoringpb.ListTimeSeriesRequest_FULL,
	}

	if v := argStr(args, "alignment_period"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return errResult(fmt.Errorf("invalid alignment_period: %w", err))
		}
		aligner := monitoringpb.Aggregation_ALIGN_NONE
		if a := argStr(args, "per_series_aligner"); a != "" {
			if val, ok := monitoringpb.Aggregation_Aligner_value[a]; ok {
				aligner = monitoringpb.Aggregation_Aligner(val)
			}
		}
		req.Aggregation = &monitoringpb.Aggregation{
			AlignmentPeriod: durationpb.New(d),
			PerSeriesAligner: aligner,
		}
	}

	var series []any
	it := g.monitoringClient.ListTimeSeries(ctx, req)
	for i := 0; i < 500; i++ {
		ts, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		series = append(series, ts)
	}
	return jsonResult(series)
}

func monitoringListAlertPolicies(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := &monitoringpb.ListAlertPoliciesRequest{
		Name: g.projectName(),
	}
	if v := argStr(args, "filter"); v != "" {
		req.Filter = v
	}

	var policies []any
	it := g.alertClient.ListAlertPolicies(ctx, req)
	for i := 0; i < 500; i++ {
		p, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		policies = append(policies, p)
	}
	return jsonResult(policies)
}

func monitoringGetAlertPolicy(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	policy, err := g.alertClient.GetAlertPolicy(ctx, &monitoringpb.GetAlertPolicyRequest{
		Name: argStr(args, "name"),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(policy)
}

func monitoringListMonitoredResources(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := &monitoringpb.ListMonitoredResourceDescriptorsRequest{
		Name: g.projectName(),
	}
	if v := argStr(args, "filter"); v != "" {
		req.Filter = v
	}

	var resources []map[string]any
	it := g.monitoringClient.ListMonitoredResourceDescriptors(ctx, req)
	for i := 0; i < 500; i++ {
		r, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		resources = append(resources, map[string]any{
			"type":         r.Type,
			"display_name": r.DisplayName,
			"description":  r.Description,
		})
	}
	return jsonResult(resources)
}
