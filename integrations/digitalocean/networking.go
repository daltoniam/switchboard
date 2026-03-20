package digitalocean

import (
	"context"

	"github.com/digitalocean/godo"

	mcp "github.com/daltoniam/switchboard"
)

func listDomains(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	domains, _, err := d.client.Domains.List(ctx, listOpts(args))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(domains)
}

func getDomain(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	name := argStr(args, "domain_name")
	domain, _, err := d.client.Domains.Get(ctx, name)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(domain)
}

func listDomainRecords(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	name := argStr(args, "domain_name")
	records, _, err := d.client.Domains.Records(ctx, name, listOpts(args))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(records)
}

func listLoadBalancers(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	lbs, _, err := d.client.LoadBalancers.List(ctx, listOpts(args))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(lbs)
}

func getLoadBalancer(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "lb_id")
	lb, _, err := d.client.LoadBalancers.Get(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(lb)
}

func listFirewalls(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	fws, _, err := d.client.Firewalls.List(ctx, listOpts(args))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(fws)
}

func getFirewall(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "firewall_id")
	fw, _, err := d.client.Firewalls.Get(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(fw)
}

func listVPCs(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	vpcs, _, err := d.client.VPCs.List(ctx, listOpts(args))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(vpcs)
}

func getVPC(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "vpc_id")
	vpc, _, err := d.client.VPCs.Get(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(vpc)
}

func listVolumes(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	params := &godo.ListVolumeParams{
		ListOptions: listOpts(args),
	}
	if v := argStr(args, "region"); v != "" {
		params.Region = v
	}
	volumes, _, err := d.client.Storage.ListVolumes(ctx, params)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(volumes)
}

func getVolume(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "volume_id")
	vol, _, err := d.client.Storage.GetVolume(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(vol)
}
