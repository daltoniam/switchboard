package aws

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

func iamListUsers(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &iam.ListUsersInput{}
	if v := argStr(args, "path_prefix"); v != "" {
		input.PathPrefix = aws.String(v)
	}
	if v := argInt32(args, "max_items"); v > 0 {
		input.MaxItems = aws.Int32(v)
	}
	out, err := a.iamClient.ListUsers(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func iamGetUser(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &iam.GetUserInput{}
	if v := argStr(args, "username"); v != "" {
		input.UserName = aws.String(v)
	}
	out, err := a.iamClient.GetUser(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func iamListRoles(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &iam.ListRolesInput{}
	if v := argStr(args, "path_prefix"); v != "" {
		input.PathPrefix = aws.String(v)
	}
	if v := argInt32(args, "max_items"); v > 0 {
		input.MaxItems = aws.Int32(v)
	}
	out, err := a.iamClient.ListRoles(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func iamGetRole(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	out, err := a.iamClient.GetRole(ctx, &iam.GetRoleInput{
		RoleName: aws.String(argStr(args, "role_name")),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func iamListPolicies(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &iam.ListPoliciesInput{}
	if v := argStr(args, "scope"); v != "" {
		input.Scope = iamtypes.PolicyScopeType(v)
	}
	if argBool(args, "only_attached") {
		input.OnlyAttached = true
	}
	if v := argStr(args, "path_prefix"); v != "" {
		input.PathPrefix = aws.String(v)
	}
	if v := argInt32(args, "max_items"); v > 0 {
		input.MaxItems = aws.Int32(v)
	}
	out, err := a.iamClient.ListPolicies(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func iamGetPolicy(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	out, err := a.iamClient.GetPolicy(ctx, &iam.GetPolicyInput{
		PolicyArn: aws.String(argStr(args, "policy_arn")),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func iamListGroups(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &iam.ListGroupsInput{}
	if v := argStr(args, "path_prefix"); v != "" {
		input.PathPrefix = aws.String(v)
	}
	if v := argInt32(args, "max_items"); v > 0 {
		input.MaxItems = aws.Int32(v)
	}
	out, err := a.iamClient.ListGroups(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func iamListAttachedRolePolicies(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &iam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(argStr(args, "role_name")),
	}
	if v := argStr(args, "path_prefix"); v != "" {
		input.PathPrefix = aws.String(v)
	}
	out, err := a.iamClient.ListAttachedRolePolicies(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func iamListAttachedUserPolicies(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &iam.ListAttachedUserPoliciesInput{
		UserName: aws.String(argStr(args, "username")),
	}
	if v := argStr(args, "path_prefix"); v != "" {
		input.PathPrefix = aws.String(v)
	}
	out, err := a.iamClient.ListAttachedUserPolicies(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func iamListAttachedGroupPolicies(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &iam.ListAttachedGroupPoliciesInput{
		GroupName: aws.String(argStr(args, "group_name")),
	}
	if v := argStr(args, "path_prefix"); v != "" {
		input.PathPrefix = aws.String(v)
	}
	out, err := a.iamClient.ListAttachedGroupPolicies(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}
