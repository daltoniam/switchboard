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
	r := mcp.NewArgs(args)
	req := &monitoringpb.ListMetricDescriptorsRequest{
		Name: g.projectName(),
	}
	if v := r.Str("filter"); v != "" {
		req.Filter = v
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
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
	return mcp.JSONResult(descriptors)
}

func monitoringListTimeSeries(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	startStr := r.Str("start_time")
	endStr := r.Str("end_time")
	filter := r.Str("filter")
	alignPeriod := r.Str("alignment_period")
	aligner := r.Str("per_series_aligner")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	startTime, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return errResult(fmt.Errorf("invalid start_time: %w", err))
	}

	endTime := time.Now().UTC()
	if endStr != "" {
		t, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			return errResult(fmt.Errorf("invalid end_time: %w", err))
		}
		endTime = t
	}

	req := &monitoringpb.ListTimeSeriesRequest{
		Name:   g.projectName(),
		Filter: filter,
		Interval: &monitoringpb.TimeInterval{
			StartTime: timestamppb.New(startTime),
			EndTime:   timestamppb.New(endTime),
		},
		View: monitoringpb.ListTimeSeriesRequest_FULL,
	}

	if alignPeriod != "" {
		d, err := time.ParseDuration(alignPeriod)
		if err != nil {
			return errResult(fmt.Errorf("invalid alignment_period: %w", err))
		}
		alignerVal := monitoringpb.Aggregation_ALIGN_NONE
		if aligner != "" {
			if val, ok := monitoringpb.Aggregation_Aligner_value[aligner]; ok {
				alignerVal = monitoringpb.Aggregation_Aligner(val)
			}
		}
		req.Aggregation = &monitoringpb.Aggregation{
			AlignmentPeriod:  durationpb.New(d),
			PerSeriesAligner: alignerVal,
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
	return mcp.JSONResult(series)
}

func monitoringListAlertPolicies(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	req := &monitoringpb.ListAlertPoliciesRequest{
		Name: g.projectName(),
	}
	if v := r.Str("filter"); v != "" {
		req.Filter = v
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
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
	return mcp.JSONResult(policies)
}

func monitoringGetAlertPolicy(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	policy, err := g.alertClient.GetAlertPolicy(ctx, &monitoringpb.GetAlertPolicyRequest{
		Name: name,
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(policy)
}

func monitoringListMonitoredResources(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	req := &monitoringpb.ListMonitoredResourceDescriptorsRequest{
		Name: g.projectName(),
	}
	if v := r.Str("filter"); v != "" {
		req.Filter = v
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
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
	return mcp.JSONResult(resources)
}
