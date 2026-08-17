package gmail

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listThreads(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{
		"q":                r.Str("q"),
		"maxResults":       r.Str("max_results"),
		"pageToken":        r.Str("page_token"),
		"includeSpamTrash": r.Str("include_spam_trash"),
	}
	var multi map[string][]string
	if ids := r.Str("label_ids"); ids != "" {
		multi = map[string][]string{"labelIds": r.StrSlice("label_ids")}
	}
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncodeMulti(params, multi)
	data, err := g.get(ctx, "/gmail/v1/users/%s/threads%s", u, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getThread(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{
		"format": r.Str("format"),
	}
	if hdrs := r.Str("metadata_headers"); hdrs != "" {
		params["metadataHeaders"] = hdrs
	}
	u := user(r)
	threadID := r.Str("thread_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(params)
	data, err := g.get(ctx, "/gmail/v1/users/%s/threads/%s%s", u, threadID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteThread(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	threadID := r.Str("thread_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.del(ctx, "/gmail/v1/users/%s/threads/%s", u, threadID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func trashThread(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	threadID := r.Str("thread_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/threads/%s/trash", u, threadID)
	data, err := g.post(ctx, path, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func untrashThread(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	threadID := r.Str("thread_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/threads/%s/untrash", u, threadID)
	data, err := g.post(ctx, path, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func modifyThread(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{}
	if ids := r.StrSlice("add_label_ids"); len(ids) > 0 {
		body["addLabelIds"] = ids
	}
	if ids := r.StrSlice("remove_label_ids"); len(ids) > 0 {
		body["removeLabelIds"] = ids
	}
	u := user(r)
	threadID := r.Str("thread_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/threads/%s/modify", u, threadID)
	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
