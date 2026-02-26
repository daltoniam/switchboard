package aws

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func ecsListClusters(ctx context.Context, a *integration, _ map[string]any) (*mcp.ToolResult, error) {
	out, err := a.ecsClient.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func ecsDescribeClusters(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	out, err := a.ecsClient.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: argStrSlice(args, "clusters"),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func ecsListServices(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &ecs.ListServicesInput{}
	if v := argStr(args, "cluster"); v != "" {
		input.Cluster = &v
	}
	out, err := a.ecsClient.ListServices(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func ecsDescribeServices(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &ecs.DescribeServicesInput{
		Services: argStrSlice(args, "services"),
	}
	if v := argStr(args, "cluster"); v != "" {
		input.Cluster = &v
	}
	out, err := a.ecsClient.DescribeServices(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func ecsListTasks(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &ecs.ListTasksInput{}
	if v := argStr(args, "cluster"); v != "" {
		input.Cluster = &v
	}
	if v := argStr(args, "service_name"); v != "" {
		input.ServiceName = &v
	}
	if v := argStr(args, "desired_status"); v != "" {
		input.DesiredStatus = ecstypes.DesiredStatus(v)
	}
	out, err := a.ecsClient.ListTasks(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func ecsDescribeTasks(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &ecs.DescribeTasksInput{
		Tasks: argStrSlice(args, "tasks"),
	}
	if v := argStr(args, "cluster"); v != "" {
		input.Cluster = &v
	}
	out, err := a.ecsClient.DescribeTasks(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func ecsListTaskDefinitions(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &ecs.ListTaskDefinitionsInput{}
	if v := argStr(args, "family_prefix"); v != "" {
		input.FamilyPrefix = &v
	}
	if v := argStr(args, "status"); v != "" {
		input.Status = ecstypes.TaskDefinitionStatus(v)
	}
	out, err := a.ecsClient.ListTaskDefinitions(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func ecsDescribeTaskDefinition(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	td := argStr(args, "task_definition")
	out, err := a.ecsClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &td,
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}
