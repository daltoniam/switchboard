package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	mcp "github.com/daltoniam/switchboard"
)

func iamListUsers(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &iam.ListUsersInput{}
	if v := r.Str("path_prefix"); v != "" {
		input.PathPrefix = aws.String(v)
	}
	if v := r.Int32("max_items"); v > 0 {
		input.MaxItems = aws.Int32(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.iamClient.ListUsers(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func iamGetUser(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &iam.GetUserInput{}
	if v := r.Str("username"); v != "" {
		input.UserName = aws.String(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.iamClient.GetUser(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func iamListRoles(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &iam.ListRolesInput{}
	if v := r.Str("path_prefix"); v != "" {
		input.PathPrefix = aws.String(v)
	}
	if v := r.Int32("max_items"); v > 0 {
		input.MaxItems = aws.Int32(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.iamClient.ListRoles(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func iamGetRole(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	roleName := r.Str("role_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.iamClient.GetRole(ctx, &iam.GetRoleInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func iamListPolicies(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &iam.ListPoliciesInput{}
	if v := r.Str("scope"); v != "" {
		input.Scope = iamtypes.PolicyScopeType(v)
	}
	if r.Bool("only_attached") {
		input.OnlyAttached = true
	}
	if v := r.Str("path_prefix"); v != "" {
		input.PathPrefix = aws.String(v)
	}
	if v := r.Int32("max_items"); v > 0 {
		input.MaxItems = aws.Int32(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.iamClient.ListPolicies(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func iamGetPolicy(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	policyArn := r.Str("policy_arn")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.iamClient.GetPolicy(ctx, &iam.GetPolicyInput{
		PolicyArn: aws.String(policyArn),
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func iamListGroups(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &iam.ListGroupsInput{}
	if v := r.Str("path_prefix"); v != "" {
		input.PathPrefix = aws.String(v)
	}
	if v := r.Int32("max_items"); v > 0 {
		input.MaxItems = aws.Int32(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.iamClient.ListGroups(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func iamListAttachedRolePolicies(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &iam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(r.Str("role_name")),
	}
	if v := r.Str("path_prefix"); v != "" {
		input.PathPrefix = aws.String(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.iamClient.ListAttachedRolePolicies(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func iamListAttachedUserPolicies(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &iam.ListAttachedUserPoliciesInput{
		UserName: aws.String(r.Str("username")),
	}
	if v := r.Str("path_prefix"); v != "" {
		input.PathPrefix = aws.String(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.iamClient.ListAttachedUserPolicies(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func iamListAttachedGroupPolicies(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &iam.ListAttachedGroupPoliciesInput{
		GroupName: aws.String(r.Str("group_name")),
	}
	if v := r.Str("path_prefix"); v != "" {
		input.PathPrefix = aws.String(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.iamClient.ListAttachedGroupPolicies(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}
