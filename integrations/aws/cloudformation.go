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
	r := mcp.NewArgs(args)
	input := &cloudformation.ListStacksInput{}
	statusFilter := r.Str("status_filter")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if statusFilter != "" {
		parts := strings.Split(statusFilter, ",")
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
	return mcp.JSONResult(out)
}

func cfnDescribeStack(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	stackName := r.Str("stack_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func cfnListStackResources(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	stackName := r.Str("stack_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.cfnClient.ListStackResources(ctx, &cloudformation.ListStackResourcesInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func cfnGetTemplate(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	stackName := r.Str("stack_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.cfnClient.GetTemplate(ctx, &cloudformation.GetTemplateInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func cfnDescribeStackEvents(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	stackName := r.Str("stack_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.cfnClient.DescribeStackEvents(ctx, &cloudformation.DescribeStackEventsInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}
