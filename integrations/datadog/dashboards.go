package datadog

import (
	"context"
	"encoding/json"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	mcp "github.com/daltoniam/switchboard"
)

func listDashboards(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV1.NewDashboardsApi(d.client)
	opts := datadogV1.NewListDashboardsOptionalParameters()
	if v := r.Str("filter_shared"); v != "" {
		opts = opts.WithFilterShared(r.Bool("filter_shared"))
	}
	if v := r.Int64("count"); v > 0 {
		opts = opts.WithCount(v)
	}
	if v := r.Int64("start"); v > 0 {
		opts = opts.WithStart(v)
	}

	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.ListDashboards(ctx, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getDashboard(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV1.NewDashboardsApi(d.client)
	resp, _, err := api.GetDashboard(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func createDashboard(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV1.NewDashboardsApi(d.client)

	layoutType, _ := datadogV1.NewDashboardLayoutTypeFromValue(r.Str("layout_type"))
	if layoutType == nil {
		lt := datadogV1.DASHBOARDLAYOUTTYPE_ORDERED
		layoutType = &lt
	}

	body := datadogV1.Dashboard{
		Title:      r.Str("title"),
		LayoutType: *layoutType,
		Widgets:    []datadogV1.Widget{},
	}
	if v := r.Str("description"); v != "" {
		body.Description = *datadog.NewNullableString(&v)
	}

	if wj := r.Str("widgets_json"); wj != "" {
		var widgets []datadogV1.Widget
		if err := json.Unmarshal([]byte(wj), &widgets); err == nil {
			body.Widgets = widgets
		}
	}

	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.CreateDashboard(ctx, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func deleteDashboard(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV1.NewDashboardsApi(d.client)
	resp, _, err := api.DeleteDashboard(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}
