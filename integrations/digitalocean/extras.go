package digitalocean

import (
	"context"

	"github.com/digitalocean/godo"

	mcp "github.com/daltoniam/switchboard"
)

func getAccount(ctx context.Context, d *integration, _ map[string]any) (*mcp.ToolResult, error) {
	acct, _, err := d.client.Account.Get(ctx)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(acct)
}

func listRegions(ctx context.Context, d *integration, _ map[string]any) (*mcp.ToolResult, error) {
	regions, _, err := d.client.Regions.List(ctx, &godo.ListOptions{PerPage: 200})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(regions)
}

func listSizes(ctx context.Context, d *integration, _ map[string]any) (*mcp.ToolResult, error) {
	sizes, _, err := d.client.Sizes.List(ctx, &godo.ListOptions{PerPage: 200})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(sizes)
}

func listImages(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	opt := listOpts(args)
	typ, _ := mcp.ArgStr(args, "type")
	var images []godo.Image
	var err error
	switch typ {
	case "distribution":
		images, _, err = d.client.Images.ListDistribution(ctx, opt)
	case "application":
		images, _, err = d.client.Images.ListApplication(ctx, opt)
	case "user":
		images, _, err = d.client.Images.ListUser(ctx, opt)
	default:
		images, _, err = d.client.Images.List(ctx, opt)
	}
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(images)
}

func listSSHKeys(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	keys, _, err := d.client.Keys.List(ctx, listOpts(args))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(keys)
}

func listSnapshots(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	opt := listOpts(args)
	resType, _ := mcp.ArgStr(args, "resource_type")
	var snapshots []godo.Snapshot
	var err error
	switch resType {
	case "droplet":
		snapshots, _, err = d.client.Snapshots.ListDroplet(ctx, opt)
	case "volume":
		snapshots, _, err = d.client.Snapshots.ListVolume(ctx, opt)
	default:
		snapshots, _, err = d.client.Snapshots.List(ctx, opt)
	}
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(snapshots)
}

func listProjects(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	projects, _, err := d.client.Projects.List(ctx, listOpts(args))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(projects)
}

func getProject(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "project_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	proj, _, err := d.client.Projects.Get(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(proj)
}

func getBalance(ctx context.Context, d *integration, _ map[string]any) (*mcp.ToolResult, error) {
	bal, _, err := d.client.Balance.Get(ctx)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(bal)
}

func listInvoices(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	invoices, _, err := d.client.Invoices.List(ctx, listOpts(args))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(invoices)
}

func listCDNEndpoints(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	endpoints, _, err := d.client.CDNs.List(ctx, listOpts(args))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(endpoints)
}

func listCertificates(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	certs, _, err := d.client.Certificates.List(ctx, listOpts(args))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(certs)
}

func listRegistryRepositories(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	name, err := mcp.ArgStr(args, "registry_name")
	if err != nil {
		return mcp.ErrResult(err)
	}
	opt := &godo.TokenListOptions{PerPage: mcp.OptInt(args, "per_page", 200)}
	repos, _, err := d.client.Registry.ListRepositoriesV2(ctx, name, opt)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(repos)
}

func listTags(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	tags, _, err := d.client.Tags.List(ctx, listOpts(args))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(tags)
}
