package gcp

import (
	"context"

	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/proto"

	mcp "github.com/daltoniam/switchboard"
)

func computeListInstances(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	req := &computepb.ListInstancesRequest{
		Project: g.projectID,
		Zone:    r.Str("zone"),
	}
	if v := r.Str("filter"); v != "" {
		req.Filter = proto.String(v)
	}
	if v := r.Int32("max_results"); v > 0 {
		req.MaxResults = proto.Uint32(uint32(v))
	}

	limit := r.Int("limit")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if limit <= 0 {
		limit = defaultComputeLimit
	}

	var instances []*computepb.Instance
	it := g.instancesClient.List(ctx, req)
	for i := 0; i < limit; i++ {
		inst, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		instances = append(instances, inst)
	}
	return mcp.JSONResult(instances)
}

func computeGetInstance(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	zone := r.Str("zone")
	instance := r.Str("instance")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	inst, err := g.instancesClient.Get(ctx, &computepb.GetInstanceRequest{
		Project:  g.projectID,
		Zone:     zone,
		Instance: instance,
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(inst)
}

func computeStartInstance(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	zone := r.Str("zone")
	instance := r.Str("instance")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	op, err := g.instancesClient.Start(ctx, &computepb.StartInstanceRequest{
		Project:  g.projectID,
		Zone:     zone,
		Instance: instance,
	})
	if err != nil {
		return errResult(err)
	}
	if err := op.Wait(ctx); err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "started"})
}

func computeStopInstance(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	zone := r.Str("zone")
	instance := r.Str("instance")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	op, err := g.instancesClient.Stop(ctx, &computepb.StopInstanceRequest{
		Project:  g.projectID,
		Zone:     zone,
		Instance: instance,
	})
	if err != nil {
		return errResult(err)
	}
	if err := op.Wait(ctx); err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "stopped"})
}

func computeListDisks(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	req := &computepb.ListDisksRequest{
		Project: g.projectID,
		Zone:    r.Str("zone"),
	}
	if v := r.Str("filter"); v != "" {
		req.Filter = proto.String(v)
	}
	if v := r.Int32("max_results"); v > 0 {
		req.MaxResults = proto.Uint32(uint32(v))
	}

	limit := r.Int("limit")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if limit <= 0 {
		limit = defaultComputeLimit
	}

	var disks []*computepb.Disk
	it := g.disksClient.List(ctx, req)
	for i := 0; i < limit; i++ {
		d, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		disks = append(disks, d)
	}
	return mcp.JSONResult(disks)
}

func computeListNetworks(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	req := &computepb.ListNetworksRequest{
		Project: g.projectID,
	}
	if v := r.Str("filter"); v != "" {
		req.Filter = proto.String(v)
	}
	if v := r.Int32("max_results"); v > 0 {
		req.MaxResults = proto.Uint32(uint32(v))
	}

	limit := r.Int("limit")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if limit <= 0 {
		limit = defaultComputeLimit
	}

	var networks []*computepb.Network
	it := g.networksClient.List(ctx, req)
	for i := 0; i < limit; i++ {
		n, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		networks = append(networks, n)
	}
	return mcp.JSONResult(networks)
}

func computeListSubnetworks(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	req := &computepb.ListSubnetworksRequest{
		Project: g.projectID,
		Region:  r.Str("region"),
	}
	if v := r.Str("filter"); v != "" {
		req.Filter = proto.String(v)
	}
	if v := r.Int32("max_results"); v > 0 {
		req.MaxResults = proto.Uint32(uint32(v))
	}

	limit := r.Int("limit")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if limit <= 0 {
		limit = defaultComputeLimit
	}

	var subnets []*computepb.Subnetwork
	it := g.subnetworksClient.List(ctx, req)
	for i := 0; i < limit; i++ {
		s, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		subnets = append(subnets, s)
	}
	return mcp.JSONResult(subnets)
}

func computeListFirewalls(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	req := &computepb.ListFirewallsRequest{
		Project: g.projectID,
	}
	if v := r.Str("filter"); v != "" {
		req.Filter = proto.String(v)
	}
	if v := r.Int32("max_results"); v > 0 {
		req.MaxResults = proto.Uint32(uint32(v))
	}

	limit := r.Int("limit")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if limit <= 0 {
		limit = defaultComputeLimit
	}

	var firewalls []*computepb.Firewall
	it := g.firewallsClient.List(ctx, req)
	for i := 0; i < limit; i++ {
		f, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		firewalls = append(firewalls, f)
	}
	return mcp.JSONResult(firewalls)
}

func computeGetFirewall(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	firewall := r.Str("firewall")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	fw, err := g.firewallsClient.Get(ctx, &computepb.GetFirewallRequest{
		Project:  g.projectID,
		Firewall: firewall,
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(fw)
}
