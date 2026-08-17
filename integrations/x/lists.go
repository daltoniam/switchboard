package x

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func getList(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	id := r.Str("id")
	q := queryEncode(map[string]string{
		"list.fields": r.Str("list_fields"),
	})
	data, err := t.get(ctx, "/lists/%s%s", id, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getOwnedLists(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	userID := r.Str("user_id")
	q := queryEncode(map[string]string{
		"max_results":      r.Str("max_results"),
		"list.fields":      r.Str("list_fields"),
		"pagination_token": r.Str("pagination_token"),
	})
	data, err := t.get(ctx, "/users/%s/owned_lists%s", userID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createList(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"name": r.Str("name"),
	}
	if v := r.Str("description"); v != "" {
		body["description"] = v
	}
	if _, ok := args["private"]; ok {
		body["private"] = r.Bool("private")
	}
	data, err := t.post(ctx, "/lists", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateList(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	id := r.Str("id")
	body := map[string]any{}
	if v := r.Str("name"); v != "" {
		body["name"] = v
	}
	if v := r.Str("description"); v != "" {
		body["description"] = v
	}
	if _, ok := args["private"]; ok {
		body["private"] = r.Bool("private")
	}
	data, err := t.put(ctx, fmt.Sprintf("/lists/%s", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteList(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.del(ctx, "/lists/%s", r.Str("id"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getListTweets(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	id := r.Str("id")
	q := queryEncode(map[string]string{
		"max_results":      r.Str("max_results"),
		"tweet.fields":     r.Str("tweet_fields"),
		"pagination_token": r.Str("pagination_token"),
	})
	data, err := t.get(ctx, "/lists/%s/tweets%s", id, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getListMembers(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	id := r.Str("id")
	q := queryEncode(map[string]string{
		"max_results":      r.Str("max_results"),
		"user.fields":      r.Str("user_fields"),
		"pagination_token": r.Str("pagination_token"),
	})
	data, err := t.get(ctx, "/lists/%s/members%s", id, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func addListMember(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	id := r.Str("id")
	data, err := t.post(ctx, fmt.Sprintf("/lists/%s/members", id), map[string]string{
		"user_id": r.Str("user_id"),
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func removeListMember(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.del(ctx, "/lists/%s/members/%s", r.Str("id"), r.Str("user_id"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getListFollowers(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	id := r.Str("id")
	q := queryEncode(map[string]string{
		"max_results":      r.Str("max_results"),
		"user.fields":      r.Str("user_fields"),
		"pagination_token": r.Str("pagination_token"),
	})
	data, err := t.get(ctx, "/lists/%s/followers%s", id, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func followList(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.post(ctx, fmt.Sprintf("/users/%s/followed_lists", t.me()), map[string]string{
		"list_id": r.Str("list_id"),
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func unfollowList(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.del(ctx, "/users/%s/followed_lists/%s", t.me(), r.Str("list_id"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getPinnedLists(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"list.fields": r.Str("list_fields"),
	})
	data, err := t.get(ctx, "/users/%s/pinned_lists%s", t.me(), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func pinList(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.post(ctx, fmt.Sprintf("/users/%s/pinned_lists", t.me()), map[string]string{
		"list_id": r.Str("list_id"),
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func unpinList(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.del(ctx, "/users/%s/pinned_lists/%s", t.me(), r.Str("list_id"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
