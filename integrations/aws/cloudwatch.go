package aws

import (
	"context"
	"encoding/json"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	mcp "github.com/daltoniam/switchboard"
)

func cwListMetrics(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &cloudwatch.ListMetricsInput{}
	if v := r.Str("namespace"); v != "" {
		input.Namespace = aws.String(v)
	}
	if v := r.Str("metric_name"); v != "" {
		input.MetricName = aws.String(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.cwClient.ListMetrics(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func cwGetMetricData(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	now := time.Now().UTC()
	startTime := now.Add(-1 * time.Hour)
	endTime := now

	startTimeStr := r.Str("start_time")
	endTimeStr := r.Str("end_time")

	period := int32(300)
	if v := r.Int32("period"); v > 0 {
		period = v
	}

	stat := r.Str("stat")
	namespace := r.Str("namespace")
	metricName := r.Str("metric_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	if startTimeStr != "" {
		t, err := parseTime(startTimeStr)
		if err != nil {
			return errResult(err)
		}
		startTime = t
	}
	if endTimeStr != "" {
		t, err := parseTime(endTimeStr)
		if err != nil {
			return errResult(err)
		}
		endTime = t
	}

	dimensions := parseDimensions(args)

	input := &cloudwatch.GetMetricDataInput{
		StartTime: &startTime,
		EndTime:   &endTime,
		MetricDataQueries: []cwtypes.MetricDataQuery{
			{
				Id: aws.String("m1"),
				MetricStat: &cwtypes.MetricStat{
					Metric: &cwtypes.Metric{
						Namespace:  aws.String(namespace),
						MetricName: aws.String(metricName),
						Dimensions: dimensions,
					},
					Period: aws.Int32(period),
					Stat:   aws.String(stat),
				},
			},
		},
	}
	out, err := a.cwClient.GetMetricData(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func cwDescribeAlarms(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &cloudwatch.DescribeAlarmsInput{}
	if names := r.StrSlice("alarm_names"); len(names) > 0 {
		input.AlarmNames = names
	}
	if v := r.Str("state_value"); v != "" {
		input.StateValue = cwtypes.StateValue(v)
	}
	if v := r.Int32("max_records"); v > 0 {
		input.MaxRecords = aws.Int32(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.cwClient.DescribeAlarms(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func cwGetMetricStatistics(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	startTimeStr := r.Str("start_time")
	endTimeStr := r.Str("end_time")
	period := r.Int32("period")
	if period <= 0 {
		period = 300
	}
	stats := r.StrSlice("statistics")
	namespace := r.Str("namespace")
	metricName := r.Str("metric_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	startTime, err := parseTime(startTimeStr)
	if err != nil {
		return errResult(err)
	}
	endTime := time.Now().UTC()
	if endTimeStr != "" {
		if t, err := parseTime(endTimeStr); err == nil {
			endTime = t
		}
	}

	var statistics []cwtypes.Statistic
	for _, s := range stats {
		statistics = append(statistics, cwtypes.Statistic(s))
	}

	dimensions := parseDimensions(args)

	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String(namespace),
		MetricName: aws.String(metricName),
		StartTime:  &startTime,
		EndTime:    &endTime,
		Period:     aws.Int32(period),
		Statistics: statistics,
		Dimensions: dimensions,
	}
	out, err := a.cwClient.GetMetricStatistics(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func parseTime(s string) (time.Time, error) {
	if len(s) > 0 && s[0] == '-' {
		d, err := time.ParseDuration(s[1:])
		if err != nil {
			return time.Time{}, err
		}
		return time.Now().UTC().Add(-d), nil
	}
	return time.Parse(time.RFC3339, s)
}

func parseDimensions(args map[string]any) []cwtypes.Dimension {
	dimStr, _ := mcp.ArgStr(args, "dimensions")
	if dimStr == "" {
		return nil
	}
	var dimMap map[string]string
	if err := json.Unmarshal([]byte(dimStr), &dimMap); err != nil {
		return nil
	}
	var dims []cwtypes.Dimension
	for k, v := range dimMap {
		dims = append(dims, cwtypes.Dimension{
			Name:  aws.String(k),
			Value: aws.String(v),
		})
	}
	return dims
}
