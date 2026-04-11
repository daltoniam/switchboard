package digitalocean

import (
	"context"

	"github.com/digitalocean/godo"

	mcp "github.com/daltoniam/switchboard"
)

func listApps(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	apps, _, err := d.client.Apps.List(ctx, listOpts(args))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(apps)
}

func getApp(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "app_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	app, _, err := d.client.Apps.Get(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(app)
}

func deleteApp(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "app_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	_, err = d.client.Apps.Delete(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted", "app_id": id})
}

func restartApp(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "app_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	components, _ := mcp.ArgStrSlice(args, "components")
	deployment, _, err := d.client.Apps.Restart(ctx, id, &godo.AppRestartRequest{
		Components: components,
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(deployment)
}

func listAppDeployments(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "app_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	deployments, _, err := d.client.Apps.ListDeployments(ctx, id, listOpts(args))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(deployments)
}

func getAppDeployment(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	appID := r.Str("app_id")
	deploymentID := r.Str("deployment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	deployment, _, err := d.client.Apps.GetDeployment(ctx, appID, deploymentID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(deployment)
}

func createAppDeployment(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "app_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	forceBuild, _ := mcp.ArgBool(args, "force_build")
	var deployment *godo.Deployment
	if forceBuild {
		deployment, _, err = d.client.Apps.CreateDeployment(ctx, id, &godo.DeploymentCreateRequest{ForceBuild: true})
	} else {
		deployment, _, err = d.client.Apps.CreateDeployment(ctx, id)
	}
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(deployment)
}

func getAppLogs(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	appID := r.Str("app_id")
	logType := r.Str("log_type")
	deploymentID := r.Str("deployment_id")
	component := r.Str("component")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	tailLines := mcp.OptInt(args, "tail_lines", 100)
	logs, _, err := d.client.Apps.GetLogs(ctx, appID, deploymentID, component, godo.AppLogType(logType), false, tailLines)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(logs)
}

func getAppHealth(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "app_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	health, _, err := d.client.Apps.GetAppHealth(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(health)
}

func listAppAlerts(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "app_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	alerts, _, err := d.client.Apps.ListAlerts(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(alerts)
}
