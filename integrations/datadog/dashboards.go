package datadog

import (
	"context"
	"encoding/json"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	mcp "github.com/daltoniam/switchboard"
)

func listDashboards(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewDashboardsApi(d.client)
	opts := datadogV1.NewListDashboardsOptionalParameters()
	if v := argStr(args, "filter_shared"); v != "" {
		opts = opts.WithFilterShared(argBool(args, "filter_shared"))
	}
	if v := argInt64(args, "count"); v > 0 {
		opts = opts.WithCount(v)
	}
	if v := argInt64(args, "start"); v > 0 {
		opts = opts.WithStart(v)
	}

	resp, _, err := api.ListDashboards(ctx, *opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func getDashboard(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewDashboardsApi(d.client)
	resp, _, err := api.GetDashboard(ctx, argStr(args, "id"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func createDashboard(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewDashboardsApi(d.client)

	layoutType, _ := datadogV1.NewDashboardLayoutTypeFromValue(argStr(args, "layout_type"))
	if layoutType == nil {
		lt := datadogV1.DASHBOARDLAYOUTTYPE_ORDERED
		layoutType = &lt
	}

	body := datadogV1.Dashboard{
		Title:      argStr(args, "title"),
		LayoutType: *layoutType,
		Widgets:    []datadogV1.Widget{},
	}
	if v := argStr(args, "description"); v != "" {
		body.Description = *datadog.NewNullableString(&v)
	}

	if wj := argStr(args, "widgets_json"); wj != "" {
		var widgets []datadogV1.Widget
		if err := json.Unmarshal([]byte(wj), &widgets); err == nil {
			body.Widgets = widgets
		}
	}

	resp, _, err := api.CreateDashboard(ctx, body)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func deleteDashboard(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewDashboardsApi(d.client)
	resp, _, err := api.DeleteDashboard(ctx, argStr(args, "id"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}
