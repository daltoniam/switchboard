package digitalocean

import (
	"context"
	"strconv"

	"github.com/digitalocean/godo"

	mcp "github.com/daltoniam/switchboard"
)

func listDroplets(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	opt := listOpts(args)
	if v := argStr(args, "tag_name"); v != "" {
		droplets, _, err := d.client.Droplets.ListByTag(ctx, v, opt)
		if err != nil {
			return errResult(err)
		}
		return mcp.JSONResult(droplets)
	}
	droplets, _, err := d.client.Droplets.List(ctx, opt)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(droplets)
}

func getDroplet(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id := argInt(args, "droplet_id")
	droplet, _, err := d.client.Droplets.Get(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(droplet)
}

func createDroplet(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := &godo.DropletCreateRequest{
		Name:   argStr(args, "name"),
		Region: argStr(args, "region"),
		Size:   argStr(args, "size"),
		Image: godo.DropletCreateImage{
			Slug: argStr(args, "image"),
		},
	}

	if keys := argStrSlice(args, "ssh_keys"); len(keys) > 0 {
		for _, k := range keys {
			if id, err := strconv.Atoi(k); err == nil {
				req.SSHKeys = append(req.SSHKeys, godo.DropletCreateSSHKey{ID: id})
			} else {
				req.SSHKeys = append(req.SSHKeys, godo.DropletCreateSSHKey{Fingerprint: k})
			}
		}
	}

	if tags := argStrSlice(args, "tags"); len(tags) > 0 {
		req.Tags = tags
	}

	if v := argStr(args, "vpc_uuid"); v != "" {
		req.VPCUUID = v
	}

	droplet, _, err := d.client.Droplets.Create(ctx, req)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(droplet)
}

func deleteDroplet(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id := argInt(args, "droplet_id")
	_, err := d.client.Droplets.Delete(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted", "droplet_id": strconv.Itoa(id)})
}

func rebootDroplet(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id := argInt(args, "droplet_id")
	action, _, err := d.client.DropletActions.Reboot(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(action)
}

func powerOffDroplet(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id := argInt(args, "droplet_id")
	action, _, err := d.client.DropletActions.PowerOff(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(action)
}

func powerOnDroplet(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id := argInt(args, "droplet_id")
	action, _, err := d.client.DropletActions.PowerOn(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(action)
}
