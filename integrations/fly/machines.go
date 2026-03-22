package fly

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listMachines(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	params := map[string]string{}
	if argBool(args, "include_deleted") {
		params["include_deleted"] = "true"
	}
	if v := argStr(args, "region"); v != "" {
		params["region"] = v
	}
	if v := argStr(args, "state"); v != "" {
		params["state"] = v
	}
	if argBool(args, "summary") {
		params["summary"] = "true"
	}
	data, err := f.get(ctx, "/apps/%s/machines%s", app, queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getMachine(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	machineID := argStr(args, "machine_id")
	data, err := f.get(ctx, "/apps/%s/machines/%s", app, machineID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createMachine(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	body := map[string]any{}
	if v := argStr(args, "name"); v != "" {
		body["name"] = v
	}
	if v := argStr(args, "region"); v != "" {
		body["region"] = v
	}
	if cfg := argMap(args, "config"); cfg != nil {
		body["config"] = cfg
	}
	data, err := f.post(ctx, fmt.Sprintf("/apps/%s/machines", app), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateMachine(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	machineID := argStr(args, "machine_id")
	body := map[string]any{}
	if cfg := argMap(args, "config"); cfg != nil {
		body["config"] = cfg
	}
	data, err := f.post(ctx, fmt.Sprintf("/apps/%s/machines/%s", app, machineID), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteMachine(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	machineID := argStr(args, "machine_id")
	params := map[string]string{}
	if argBool(args, "force") {
		params["force"] = "true"
	}
	path := fmt.Sprintf("/apps/%s/machines/%s", app, machineID)
	data, err := f.delWithQuery(ctx, path, params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func startMachine(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	machineID := argStr(args, "machine_id")
	data, err := f.post(ctx, fmt.Sprintf("/apps/%s/machines/%s/start", app, machineID), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func stopMachine(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	machineID := argStr(args, "machine_id")
	data, err := f.post(ctx, fmt.Sprintf("/apps/%s/machines/%s/stop", app, machineID), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func restartMachine(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	machineID := argStr(args, "machine_id")
	data, err := f.post(ctx, fmt.Sprintf("/apps/%s/machines/%s/restart", app, machineID), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func signalMachine(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	machineID := argStr(args, "machine_id")
	body := map[string]any{
		"signal": argStr(args, "signal"),
	}
	data, err := f.post(ctx, fmt.Sprintf("/apps/%s/machines/%s/signal", app, machineID), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func waitMachine(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	machineID := argStr(args, "machine_id")
	params := map[string]string{}
	if v := argStr(args, "state"); v != "" {
		params["state"] = v
	}
	if v := argInt(args, "timeout"); v > 0 {
		params["timeout"] = fmt.Sprintf("%d", v)
	}
	data, err := f.get(ctx, "/apps/%s/machines/%s/wait%s", app, machineID, queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func execMachine(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	machineID := argStr(args, "machine_id")
	body := map[string]any{}
	if cmd := argStrSlice(args, "command"); len(cmd) > 0 {
		body["command"] = cmd
	}
	data, err := f.post(ctx, fmt.Sprintf("/apps/%s/machines/%s/exec", app, machineID), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
