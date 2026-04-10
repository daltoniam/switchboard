package digitalocean

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listApps(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	page := mcp.OptInt(args, "page", 1)
	perPage := mcp.OptInt(args, "per_page", 20)
	data, err := d.doGet(ctx, "/v2/apps?page=%d&per_page=%d", page, perPage)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getApp(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "app_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := d.doGet(ctx, "/v2/apps/%s", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createApp(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	specStr := r.Str("spec")
	projectID := r.Str("project_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	var spec json.RawMessage
	if err := json.Unmarshal([]byte(specStr), &spec); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid spec JSON: %w", err))
	}

	body := map[string]any{"spec": spec}
	if projectID != "" {
		body["project_id"] = projectID
	}

	data, err := d.doPost(ctx, "/v2/apps", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateApp(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("app_id")
	specStr := r.Str("spec")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	var spec json.RawMessage
	if err := json.Unmarshal([]byte(specStr), &spec); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid spec JSON: %w", err))
	}

	data, err := d.doPut(ctx, fmt.Sprintf("/v2/apps/%s", id), map[string]any{"spec": spec})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteApp(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "app_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := d.doDel(ctx, "/v2/apps/%s", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func restartApp(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("app_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{}
	components, _ := mcp.ArgStrSlice(args, "components")
	if len(components) > 0 {
		body["components"] = components
	}

	data, err := d.doPost(ctx, fmt.Sprintf("/v2/apps/%s/restart", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listAppDeployments(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "app_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	page := mcp.OptInt(args, "page", 1)
	perPage := mcp.OptInt(args, "per_page", 20)
	data, err := d.doGet(ctx, "/v2/apps/%s/deployments?page=%d&per_page=%d", id, page, perPage)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAppDeployment(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	appID := r.Str("app_id")
	deployID := r.Str("deployment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := d.doGet(ctx, "/v2/apps/%s/deployments/%s", appID, deployID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createAppDeployment(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "app_id")
	if err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{}
	forceBuild, _ := mcp.ArgBool(args, "force_build")
	if forceBuild {
		body["force_build"] = true
	}

	data, err := d.doPost(ctx, fmt.Sprintf("/v2/apps/%s/deployments", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func cancelAppDeployment(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	appID := r.Str("app_id")
	deployID := r.Str("deployment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := d.doPost(ctx, fmt.Sprintf("/v2/apps/%s/deployments/%s/cancel", appID, deployID), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAppLogs(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	appID := r.Str("app_id")
	deployID := r.Str("deployment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	component, _ := mcp.ArgStr(args, "component")
	logType, _ := mcp.ArgStr(args, "log_type")

	path := fmt.Sprintf("/v2/apps/%s/deployments/%s/logs", appID, deployID)
	params := url.Values{}
	if component != "" {
		params.Set("component_name", component)
	}
	if logType != "" {
		params.Set("type", logType)
	}
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	data, err := d.doGet(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAppHealth(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "app_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := d.doGet(ctx, "/v2/apps/%s/health", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listAppAlerts(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "app_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := d.doGet(ctx, "/v2/apps/%s/alerts", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func rollbackApp(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	appID := r.Str("app_id")
	deployID := r.Str("deployment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{"deployment_id": deployID}

	data, err := d.doPost(ctx, fmt.Sprintf("/v2/apps/%s/rollback", appID), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
