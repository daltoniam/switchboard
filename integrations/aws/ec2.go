package aws

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	mcp "github.com/daltoniam/switchboard"
)

func ec2DescribeInstances(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &ec2.DescribeInstancesInput{}
	if ids := r.StrSlice("instance_ids"); len(ids) > 0 {
		input.InstanceIds = ids
	}
	if filtersRaw := r.Str("filters"); filtersRaw != "" {
		var filters []ec2types.Filter
		if err := json.Unmarshal([]byte(filtersRaw), &filters); err != nil {
			return errResult(err)
		}
		input.Filters = filters
	}
	if v := r.Int32("max_results"); v > 0 {
		input.MaxResults = &v
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.ec2Client.DescribeInstances(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func ec2DescribeInstance(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	instanceID := r.Str("instance_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func ec2StartInstances(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	ids := r.StrSlice("instance_ids")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.ec2Client.StartInstances(ctx, &ec2.StartInstancesInput{
		InstanceIds: ids,
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func ec2StopInstances(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	ids := r.StrSlice("instance_ids")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.ec2Client.StopInstances(ctx, &ec2.StopInstancesInput{
		InstanceIds: ids,
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func ec2DescribeSecurityGroups(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &ec2.DescribeSecurityGroupsInput{}
	if ids := r.StrSlice("group_ids"); len(ids) > 0 {
		input.GroupIds = ids
	}
	filtersRaw := r.Str("filters")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filtersRaw != "" {
		var filters []ec2types.Filter
		if err := json.Unmarshal([]byte(filtersRaw), &filters); err != nil {
			return errResult(err)
		}
		input.Filters = filters
	}
	out, err := a.ec2Client.DescribeSecurityGroups(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func ec2DescribeVPCs(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &ec2.DescribeVpcsInput{}
	if ids := r.StrSlice("vpc_ids"); len(ids) > 0 {
		input.VpcIds = ids
	}
	filtersRaw := r.Str("filters")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filtersRaw != "" {
		var filters []ec2types.Filter
		if err := json.Unmarshal([]byte(filtersRaw), &filters); err != nil {
			return errResult(err)
		}
		input.Filters = filters
	}
	out, err := a.ec2Client.DescribeVpcs(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func ec2DescribeSubnets(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &ec2.DescribeSubnetsInput{}
	if ids := r.StrSlice("subnet_ids"); len(ids) > 0 {
		input.SubnetIds = ids
	}
	filtersRaw := r.Str("filters")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filtersRaw != "" {
		var filters []ec2types.Filter
		if err := json.Unmarshal([]byte(filtersRaw), &filters); err != nil {
			return errResult(err)
		}
		input.Filters = filters
	}
	out, err := a.ec2Client.DescribeSubnets(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func ec2DescribeImages(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &ec2.DescribeImagesInput{}
	if ids := r.StrSlice("image_ids"); len(ids) > 0 {
		input.ImageIds = ids
	}
	if owners := r.StrSlice("owners"); len(owners) > 0 {
		input.Owners = owners
	}
	filtersRaw := r.Str("filters")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filtersRaw != "" {
		var filters []ec2types.Filter
		if err := json.Unmarshal([]byte(filtersRaw), &filters); err != nil {
			return errResult(err)
		}
		input.Filters = filters
	}
	out, err := a.ec2Client.DescribeImages(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func ec2DescribeVolumes(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &ec2.DescribeVolumesInput{}
	if ids := r.StrSlice("volume_ids"); len(ids) > 0 {
		input.VolumeIds = ids
	}
	filtersRaw := r.Str("filters")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filtersRaw != "" {
		var filters []ec2types.Filter
		if err := json.Unmarshal([]byte(filtersRaw), &filters); err != nil {
			return errResult(err)
		}
		input.Filters = filters
	}
	out, err := a.ec2Client.DescribeVolumes(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func ec2DescribeAddresses(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &ec2.DescribeAddressesInput{}
	if ids := r.StrSlice("allocation_ids"); len(ids) > 0 {
		input.AllocationIds = ids
	}
	filtersRaw := r.Str("filters")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filtersRaw != "" {
		var filters []ec2types.Filter
		if err := json.Unmarshal([]byte(filtersRaw), &filters); err != nil {
			return errResult(err)
		}
		input.Filters = filters
	}
	out, err := a.ec2Client.DescribeAddresses(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func ec2DescribeKeyPairs(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &ec2.DescribeKeyPairsInput{}
	if names := r.StrSlice("key_names"); len(names) > 0 {
		input.KeyNames = names
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.ec2Client.DescribeKeyPairs(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}
