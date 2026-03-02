package notion

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	mcp "github.com/daltoniam/switchboard"
)

func createDatabase(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	parent := argMap(args, "parent")
	if parent == nil {
		return errResult(fmt.Errorf("parent is required"))
	}

	body := map[string]any{
		"parent": parent,
	}
	if v := args["title"]; v != nil {
		body["title"] = v
	}
	if v := argMap(args, "properties"); v != nil {
		body["properties"] = v
	}
	if v := argBool(args, "is_inline"); v {
		body["is_inline"] = true
	}

	data, err := n.post(ctx, "/v1/databases", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func retrieveDataSource(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	dataSourceID := argStr(args, "data_source_id")
	if dataSourceID == "" {
		return errResult(fmt.Errorf("data_source_id is required"))
	}

	data, err := n.get(ctx, "/v1/data_sources/%s", dataSourceID)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateDataSource(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	dataSourceID := argStr(args, "data_source_id")
	if dataSourceID == "" {
		return errResult(fmt.Errorf("data_source_id is required"))
	}

	body := map[string]any{}
	if v := args["title"]; v != nil {
		body["title"] = v
	}
	if v := argMap(args, "properties"); v != nil {
		body["properties"] = v
	}

	data, err := n.patch(ctx, fmt.Sprintf("/v1/data_sources/%s", dataSourceID), body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func queryDataSource(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	dataSourceID := argStr(args, "data_source_id")
	if dataSourceID == "" {
		return errResult(fmt.Errorf("data_source_id is required"))
	}

	body := map[string]any{}
	if v := argMap(args, "filter"); v != nil {
		body["filter"] = v
	}
	if v := args["sorts"]; v != nil {
		body["sorts"] = v
	}
	if v := argStr(args, "start_cursor"); v != "" {
		body["start_cursor"] = v
	}
	if v := argInt(args, "page_size"); v > 0 {
		body["page_size"] = v
	}

	data, err := n.post(ctx, fmt.Sprintf("/v1/data_sources/%s/query", dataSourceID), body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func retrieveDatabase(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	databaseID := argStr(args, "database_id")
	if databaseID == "" {
		return errResult(fmt.Errorf("database_id is required"))
	}
	data, err := n.get(ctx, "/v1/databases/%s", databaseID)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listDataSourceTemplates(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	dataSourceID := argStr(args, "data_source_id")
	if dataSourceID == "" {
		return errResult(fmt.Errorf("data_source_id is required"))
	}

	vals := url.Values{}
	if v := argStr(args, "start_cursor"); v != "" {
		vals.Set("start_cursor", v)
	}
	if v := argInt(args, "page_size"); v > 0 {
		vals.Set("page_size", strconv.Itoa(v))
	}

	path := fmt.Sprintf("/v1/data_sources/%s/templates", dataSourceID)
	if len(vals) > 0 {
		path += "?" + vals.Encode()
	}

	data, err := n.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
