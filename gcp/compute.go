package gcp

import (
	"context"

	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/proto"

	mcp "github.com/daltoniam/switchboard"
)

func computeListInstances(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := &computepb.ListInstancesRequest{
		Project: g.projectID,
		Zone:    argStr(args, "zone"),
	}
	if v := argStr(args, "filter"); v != "" {
		req.Filter = proto.String(v)
	}
	if v := argInt32(args, "max_results"); v > 0 {
		req.MaxResults = proto.Uint32(uint32(v))
	}

	var instances []*computepb.Instance
	it := g.instancesClient.List(ctx, req)
	for i := 0; i < 500; i++ {
		inst, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		instances = append(instances, inst)
	}
	return jsonResult(instances)
}

func computeGetInstance(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	inst, err := g.instancesClient.Get(ctx, &computepb.GetInstanceRequest{
		Project:  g.projectID,
		Zone:     argStr(args, "zone"),
		Instance: argStr(args, "instance"),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(inst)
}

func computeStartInstance(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	op, err := g.instancesClient.Start(ctx, &computepb.StartInstanceRequest{
		Project:  g.projectID,
		Zone:     argStr(args, "zone"),
		Instance: argStr(args, "instance"),
	})
	if err != nil {
		return errResult(err)
	}
	if err := op.Wait(ctx); err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "started"})
}

func computeStopInstance(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	op, err := g.instancesClient.Stop(ctx, &computepb.StopInstanceRequest{
		Project:  g.projectID,
		Zone:     argStr(args, "zone"),
		Instance: argStr(args, "instance"),
	})
	if err != nil {
		return errResult(err)
	}
	if err := op.Wait(ctx); err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "stopped"})
}

func computeListDisks(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := &computepb.ListDisksRequest{
		Project: g.projectID,
		Zone:    argStr(args, "zone"),
	}
	if v := argStr(args, "filter"); v != "" {
		req.Filter = proto.String(v)
	}
	if v := argInt32(args, "max_results"); v > 0 {
		req.MaxResults = proto.Uint32(uint32(v))
	}

	var disks []*computepb.Disk
	it := g.disksClient.List(ctx, req)
	for i := 0; i < 500; i++ {
		d, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		disks = append(disks, d)
	}
	return jsonResult(disks)
}

func computeListNetworks(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := &computepb.ListNetworksRequest{
		Project: g.projectID,
	}
	if v := argStr(args, "filter"); v != "" {
		req.Filter = proto.String(v)
	}
	if v := argInt32(args, "max_results"); v > 0 {
		req.MaxResults = proto.Uint32(uint32(v))
	}

	var networks []*computepb.Network
	it := g.networksClient.List(ctx, req)
	for i := 0; i < 500; i++ {
		n, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		networks = append(networks, n)
	}
	return jsonResult(networks)
}

func computeListSubnetworks(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := &computepb.ListSubnetworksRequest{
		Project: g.projectID,
		Region:  argStr(args, "region"),
	}
	if v := argStr(args, "filter"); v != "" {
		req.Filter = proto.String(v)
	}
	if v := argInt32(args, "max_results"); v > 0 {
		req.MaxResults = proto.Uint32(uint32(v))
	}

	var subnets []*computepb.Subnetwork
	it := g.subnetworksClient.List(ctx, req)
	for i := 0; i < 500; i++ {
		s, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		subnets = append(subnets, s)
	}
	return jsonResult(subnets)
}

func computeListFirewalls(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := &computepb.ListFirewallsRequest{
		Project: g.projectID,
	}
	if v := argStr(args, "filter"); v != "" {
		req.Filter = proto.String(v)
	}
	if v := argInt32(args, "max_results"); v > 0 {
		req.MaxResults = proto.Uint32(uint32(v))
	}

	var firewalls []*computepb.Firewall
	it := g.firewallsClient.List(ctx, req)
	for i := 0; i < 500; i++ {
		f, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		firewalls = append(firewalls, f)
	}
	return jsonResult(firewalls)
}

func computeGetFirewall(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	fw, err := g.firewallsClient.Get(ctx, &computepb.GetFirewallRequest{
		Project:  g.projectID,
		Firewall: argStr(args, "firewall"),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(fw)
}
