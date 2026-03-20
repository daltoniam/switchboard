package digitalocean

import (
	"context"
	"strconv"

	"github.com/digitalocean/godo"

	mcp "github.com/daltoniam/switchboard"
)

func listDroplets(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	opt := listOpts(args)
	tag, _ := mcp.ArgStr(args, "tag_name")
	if tag != "" {
		droplets, _, err := d.client.Droplets.ListByTag(ctx, tag, opt)
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
	id, err := mcp.ArgInt(args, "droplet_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	droplet, _, err := d.client.Droplets.Get(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(droplet)
}

func createDroplet(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	region := r.Str("region")
	size := r.Str("size")
	image := r.Str("image")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	req := &godo.DropletCreateRequest{
		Name:   name,
		Region: region,
		Size:   size,
		Image:  godo.DropletCreateImage{Slug: image},
	}

	keys, _ := mcp.ArgStrSlice(args, "ssh_keys")
	for _, k := range keys {
		if id, err := strconv.Atoi(k); err == nil {
			req.SSHKeys = append(req.SSHKeys, godo.DropletCreateSSHKey{ID: id})
		} else {
			req.SSHKeys = append(req.SSHKeys, godo.DropletCreateSSHKey{Fingerprint: k})
		}
	}

	tags, _ := mcp.ArgStrSlice(args, "tags")
	if len(tags) > 0 {
		req.Tags = tags
	}

	vpcUUID, _ := mcp.ArgStr(args, "vpc_uuid")
	if vpcUUID != "" {
		req.VPCUUID = vpcUUID
	}

	droplet, _, err := d.client.Droplets.Create(ctx, req)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(droplet)
}

func deleteDroplet(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgInt(args, "droplet_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	_, err = d.client.Droplets.Delete(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted", "droplet_id": strconv.Itoa(id)})
}

func rebootDroplet(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgInt(args, "droplet_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	action, _, err := d.client.DropletActions.Reboot(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(action)
}

func powerOffDroplet(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgInt(args, "droplet_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	action, _, err := d.client.DropletActions.PowerOff(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(action)
}

func powerOnDroplet(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgInt(args, "droplet_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	action, _, err := d.client.DropletActions.PowerOn(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(action)
}
