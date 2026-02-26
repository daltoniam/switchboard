package aws

import (
	"context"
	"encoding/json"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

func cwListMetrics(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &cloudwatch.ListMetricsInput{}
	if v := argStr(args, "namespace"); v != "" {
		input.Namespace = aws.String(v)
	}
	if v := argStr(args, "metric_name"); v != "" {
		input.MetricName = aws.String(v)
	}
	out, err := a.cwClient.ListMetrics(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func cwGetMetricData(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	now := time.Now().UTC()
	startTime := now.Add(-1 * time.Hour)
	endTime := now

	if v := argStr(args, "start_time"); v != "" {
		if t, err := parseTime(v); err == nil {
			startTime = t
		}
	}
	if v := argStr(args, "end_time"); v != "" {
		if t, err := parseTime(v); err == nil {
			endTime = t
		}
	}

	period := int32(300)
	if v := argInt32(args, "period"); v > 0 {
		period = v
	}

	stat := argStr(args, "stat")
	namespace := argStr(args, "namespace")
	metricName := argStr(args, "metric_name")

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
	return jsonResult(out)
}

func cwDescribeAlarms(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &cloudwatch.DescribeAlarmsInput{}
	if names := argStrSlice(args, "alarm_names"); len(names) > 0 {
		input.AlarmNames = names
	}
	if v := argStr(args, "state_value"); v != "" {
		input.StateValue = cwtypes.StateValue(v)
	}
	if v := argInt32(args, "max_records"); v > 0 {
		input.MaxRecords = aws.Int32(v)
	}
	out, err := a.cwClient.DescribeAlarms(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func cwGetMetricStatistics(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	startTime, err := parseTime(argStr(args, "start_time"))
	if err != nil {
		return errResult(err)
	}
	endTime := time.Now().UTC()
	if v := argStr(args, "end_time"); v != "" {
		if t, err := parseTime(v); err == nil {
			endTime = t
		}
	}

	period := argInt32(args, "period")
	stats := argStrSlice(args, "statistics")
	var statistics []cwtypes.Statistic
	for _, s := range stats {
		statistics = append(statistics, cwtypes.Statistic(s))
	}

	dimensions := parseDimensions(args)

	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String(argStr(args, "namespace")),
		MetricName: aws.String(argStr(args, "metric_name")),
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
	return jsonResult(out)
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
	dimStr := argStr(args, "dimensions")
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
