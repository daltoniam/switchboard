package fly

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listMachines(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	includeDeleted := r.Bool("include_deleted")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{}
	if includeDeleted {
		params["include_deleted"] = "true"
	}
	region, err := mcp.ArgStr(args, "region")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if region != "" {
		params["region"] = region
	}
	state, err := mcp.ArgStr(args, "state")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if state != "" {
		params["state"] = state
	}
	summary, err := mcp.ArgBool(args, "summary")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if summary {
		params["summary"] = "true"
	}
	data, err := f.get(ctx, "/apps/%s/machines%s", url.PathEscape(app), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getMachine(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	machineID := r.Str("machine_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := f.get(ctx, "/apps/%s/machines/%s", url.PathEscape(app), url.PathEscape(machineID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createMachine(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	name, err := mcp.ArgStr(args, "name")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if name != "" {
		body["name"] = name
	}
	region, err := mcp.ArgStr(args, "region")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if region != "" {
		body["region"] = region
	}
	cfg, err := mcp.ArgMap(args, "config")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if cfg != nil {
		body["config"] = cfg
	}
	data, err := f.post(ctx, fmt.Sprintf("/apps/%s/machines", url.PathEscape(app)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateMachine(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	machineID := r.Str("machine_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	cfg, err := mcp.ArgMap(args, "config")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if cfg != nil {
		body["config"] = cfg
	}
	data, err := f.patch(ctx, fmt.Sprintf("/apps/%s/machines/%s", url.PathEscape(app), url.PathEscape(machineID)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteMachine(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	machineID := r.Str("machine_id")
	force := r.Bool("force")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{}
	if force {
		params["force"] = "true"
	}
	path := fmt.Sprintf("/apps/%s/machines/%s", url.PathEscape(app), url.PathEscape(machineID))
	data, err := f.delWithQuery(ctx, path, params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func startMachine(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	machineID := r.Str("machine_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := f.post(ctx, fmt.Sprintf("/apps/%s/machines/%s/start", url.PathEscape(app), url.PathEscape(machineID)), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func stopMachine(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	machineID := r.Str("machine_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := f.post(ctx, fmt.Sprintf("/apps/%s/machines/%s/stop", url.PathEscape(app), url.PathEscape(machineID)), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func restartMachine(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	machineID := r.Str("machine_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := f.post(ctx, fmt.Sprintf("/apps/%s/machines/%s/restart", url.PathEscape(app), url.PathEscape(machineID)), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func signalMachine(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	machineID := r.Str("machine_id")
	signal := r.Str("signal")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"signal": signal,
	}
	data, err := f.post(ctx, fmt.Sprintf("/apps/%s/machines/%s/signal", url.PathEscape(app), url.PathEscape(machineID)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func waitMachine(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	machineID := r.Str("machine_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{}
	state, err := mcp.ArgStr(args, "state")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if state != "" {
		params["state"] = state
	}
	timeout, err := mcp.ArgInt(args, "timeout")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if timeout > 0 {
		params["timeout"] = fmt.Sprintf("%d", timeout)
	}
	data, err := f.get(ctx, "/apps/%s/machines/%s/wait%s", url.PathEscape(app), url.PathEscape(machineID), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func execMachine(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	machineID := r.Str("machine_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	cmd, err := mcp.ArgStrSlice(args, "command")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if len(cmd) > 0 {
		body["command"] = cmd
	}
	data, err := f.post(ctx, fmt.Sprintf("/apps/%s/machines/%s/exec", url.PathEscape(app), url.PathEscape(machineID)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
