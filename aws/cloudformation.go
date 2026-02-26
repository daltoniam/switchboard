package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	mcp "github.com/daltoniam/switchboard"
)

func cfnListStacks(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &cloudformation.ListStacksInput{}
	if v := argStr(args, "status_filter"); v != "" {
		parts := strings.Split(v, ",")
		var statuses []cfntypes.StackStatus
		for _, p := range parts {
			statuses = append(statuses, cfntypes.StackStatus(strings.TrimSpace(p)))
		}
		input.StackStatusFilter = statuses
	}
	out, err := a.cfnClient.ListStacks(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func cfnDescribeStack(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	out, err := a.cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(argStr(args, "stack_name")),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func cfnListStackResources(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	out, err := a.cfnClient.ListStackResources(ctx, &cloudformation.ListStackResourcesInput{
		StackName: aws.String(argStr(args, "stack_name")),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func cfnGetTemplate(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	out, err := a.cfnClient.GetTemplate(ctx, &cloudformation.GetTemplateInput{
		StackName: aws.String(argStr(args, "stack_name")),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func cfnDescribeStackEvents(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	out, err := a.cfnClient.DescribeStackEvents(ctx, &cloudformation.DescribeStackEventsInput{
		StackName: aws.String(argStr(args, "stack_name")),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}
