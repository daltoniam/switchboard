package fly

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listVolumes(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	data, err := f.get(ctx, "/apps/%s/volumes", app)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getVolume(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	volID := argStr(args, "volume_id")
	data, err := f.get(ctx, "/apps/%s/volumes/%s", app, volID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createVolume(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	body := map[string]any{
		"name":   argStr(args, "name"),
		"region": argStr(args, "region"),
	}
	if v := argInt(args, "size_gb"); v > 0 {
		body["size_gb"] = v
	}
	if v, ok := args["encrypted"]; ok {
		body["encrypted"] = argBool(map[string]any{"v": v}, "v")
	}
	if v := argInt(args, "snapshot_retention"); v > 0 {
		body["snapshot_retention"] = v
	}
	if v, ok := args["auto_backup_enabled"]; ok {
		body["auto_backup_enabled"] = argBool(map[string]any{"v": v}, "v")
	}
	data, err := f.post(ctx, fmt.Sprintf("/apps/%s/volumes", app), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateVolume(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	volID := argStr(args, "volume_id")
	body := map[string]any{}
	if v := argInt(args, "snapshot_retention"); v > 0 {
		body["snapshot_retention"] = v
	}
	if v, ok := args["auto_backup_enabled"]; ok {
		body["auto_backup_enabled"] = argBool(map[string]any{"v": v}, "v")
	}
	data, err := f.post(ctx, fmt.Sprintf("/apps/%s/volumes/%s", app, volID), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteVolume(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	volID := argStr(args, "volume_id")
	data, err := f.del(ctx, "/apps/%s/volumes/%s", app, volID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listVolumeSnapshots(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	volID := argStr(args, "volume_id")
	data, err := f.get(ctx, "/apps/%s/volumes/%s/snapshots", app, volID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
