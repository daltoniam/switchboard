package fly

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listVolumes(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := f.get(ctx, "/apps/%s/volumes", url.PathEscape(app))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getVolume(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	volID := r.Str("volume_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := f.get(ctx, "/apps/%s/volumes/%s", url.PathEscape(app), url.PathEscape(volID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createVolume(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	name := r.Str("name")
	region := r.Str("region")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"name":   name,
		"region": region,
	}
	sizeGB, err := mcp.ArgInt(args, "size_gb")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if sizeGB > 0 {
		body["size_gb"] = sizeGB
	}
	if _, ok := args["encrypted"]; ok {
		encrypted, err := mcp.ArgBool(args, "encrypted")
		if err != nil {
			return mcp.ErrResult(err)
		}
		body["encrypted"] = encrypted
	}
	snapshotRetention, err := mcp.ArgInt(args, "snapshot_retention")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if snapshotRetention > 0 {
		body["snapshot_retention"] = snapshotRetention
	}
	if _, ok := args["auto_backup_enabled"]; ok {
		autoBackup, err := mcp.ArgBool(args, "auto_backup_enabled")
		if err != nil {
			return mcp.ErrResult(err)
		}
		body["auto_backup_enabled"] = autoBackup
	}
	data, err := f.post(ctx, fmt.Sprintf("/apps/%s/volumes", url.PathEscape(app)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateVolume(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	volID := r.Str("volume_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	snapshotRetention, err := mcp.ArgInt(args, "snapshot_retention")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if snapshotRetention > 0 {
		body["snapshot_retention"] = snapshotRetention
	}
	if _, ok := args["auto_backup_enabled"]; ok {
		autoBackup, err := mcp.ArgBool(args, "auto_backup_enabled")
		if err != nil {
			return mcp.ErrResult(err)
		}
		body["auto_backup_enabled"] = autoBackup
	}
	data, err := f.put(ctx, fmt.Sprintf("/apps/%s/volumes/%s", url.PathEscape(app), url.PathEscape(volID)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteVolume(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	volID := r.Str("volume_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := f.del(ctx, "/apps/%s/volumes/%s", url.PathEscape(app), url.PathEscape(volID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listVolumeSnapshots(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	volID := r.Str("volume_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := f.get(ctx, "/apps/%s/volumes/%s/snapshots", url.PathEscape(app), url.PathEscape(volID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
