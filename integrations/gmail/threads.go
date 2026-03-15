package gmail

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listThreads(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{
		"q":                argStr(args, "q"),
		"maxResults":       argStr(args, "max_results"),
		"pageToken":        argStr(args, "page_token"),
		"includeSpamTrash": argStr(args, "include_spam_trash"),
	}
	var multi map[string][]string
	if ids := argStr(args, "label_ids"); ids != "" {
		multi = map[string][]string{"labelIds": argStrSlice(args, "label_ids")}
	}
	q := queryEncodeMulti(params, multi)
	data, err := g.get(ctx, "/gmail/v1/users/%s/threads%s", user(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getThread(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{
		"format": argStr(args, "format"),
	}
	if hdrs := argStr(args, "metadata_headers"); hdrs != "" {
		params["metadataHeaders"] = hdrs
	}
	q := queryEncode(params)
	data, err := g.get(ctx, "/gmail/v1/users/%s/threads/%s%s", user(args), argStr(args, "thread_id"), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteThread(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.del(ctx, "/gmail/v1/users/%s/threads/%s", user(args), argStr(args, "thread_id"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func trashThread(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	path := fmt.Sprintf("/gmail/v1/users/%s/threads/%s/trash", user(args), argStr(args, "thread_id"))
	data, err := g.post(ctx, path, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func untrashThread(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	path := fmt.Sprintf("/gmail/v1/users/%s/threads/%s/untrash", user(args), argStr(args, "thread_id"))
	data, err := g.post(ctx, path, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func modifyThread(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if ids := argStrSlice(args, "add_label_ids"); len(ids) > 0 {
		body["addLabelIds"] = ids
	}
	if ids := argStrSlice(args, "remove_label_ids"); len(ids) > 0 {
		body["removeLabelIds"] = ids
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/threads/%s/modify", user(args), argStr(args, "thread_id"))
	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
