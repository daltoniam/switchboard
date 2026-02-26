package aws

import (
	"context"
	"encoding/json"

	mcp "github.com/daltoniam/switchboard"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func ec2DescribeInstances(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &ec2.DescribeInstancesInput{}
	if ids := argStrSlice(args, "instance_ids"); len(ids) > 0 {
		input.InstanceIds = ids
	}
	if filtersRaw := argStr(args, "filters"); filtersRaw != "" {
		var filters []ec2types.Filter
		if err := json.Unmarshal([]byte(filtersRaw), &filters); err != nil {
			return errResult(err)
		}
		input.Filters = filters
	}
	if v := argInt32(args, "max_results"); v > 0 {
		input.MaxResults = &v
	}
	out, err := a.ec2Client.DescribeInstances(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func ec2DescribeInstance(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	out, err := a.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{argStr(args, "instance_id")},
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func ec2StartInstances(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	ids := argStrSlice(args, "instance_ids")
	out, err := a.ec2Client.StartInstances(ctx, &ec2.StartInstancesInput{
		InstanceIds: ids,
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func ec2StopInstances(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	ids := argStrSlice(args, "instance_ids")
	out, err := a.ec2Client.StopInstances(ctx, &ec2.StopInstancesInput{
		InstanceIds: ids,
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func ec2DescribeSecurityGroups(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &ec2.DescribeSecurityGroupsInput{}
	if ids := argStrSlice(args, "group_ids"); len(ids) > 0 {
		input.GroupIds = ids
	}
	if filtersRaw := argStr(args, "filters"); filtersRaw != "" {
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
	return jsonResult(out)
}

func ec2DescribeVPCs(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &ec2.DescribeVpcsInput{}
	if ids := argStrSlice(args, "vpc_ids"); len(ids) > 0 {
		input.VpcIds = ids
	}
	if filtersRaw := argStr(args, "filters"); filtersRaw != "" {
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
	return jsonResult(out)
}

func ec2DescribeSubnets(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &ec2.DescribeSubnetsInput{}
	if ids := argStrSlice(args, "subnet_ids"); len(ids) > 0 {
		input.SubnetIds = ids
	}
	if filtersRaw := argStr(args, "filters"); filtersRaw != "" {
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
	return jsonResult(out)
}

func ec2DescribeImages(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &ec2.DescribeImagesInput{}
	if ids := argStrSlice(args, "image_ids"); len(ids) > 0 {
		input.ImageIds = ids
	}
	if owners := argStrSlice(args, "owners"); len(owners) > 0 {
		input.Owners = owners
	}
	if filtersRaw := argStr(args, "filters"); filtersRaw != "" {
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
	return jsonResult(out)
}

func ec2DescribeVolumes(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &ec2.DescribeVolumesInput{}
	if ids := argStrSlice(args, "volume_ids"); len(ids) > 0 {
		input.VolumeIds = ids
	}
	if filtersRaw := argStr(args, "filters"); filtersRaw != "" {
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
	return jsonResult(out)
}

func ec2DescribeAddresses(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &ec2.DescribeAddressesInput{}
	if ids := argStrSlice(args, "allocation_ids"); len(ids) > 0 {
		input.AllocationIds = ids
	}
	if filtersRaw := argStr(args, "filters"); filtersRaw != "" {
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
	return jsonResult(out)
}

func ec2DescribeKeyPairs(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &ec2.DescribeKeyPairsInput{}
	if names := argStrSlice(args, "key_names"); len(names) > 0 {
		input.KeyNames = names
	}
	out, err := a.ec2Client.DescribeKeyPairs(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}
