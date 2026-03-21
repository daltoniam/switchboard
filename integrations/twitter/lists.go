package twitter

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func getList(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "id")
	q := queryEncode(map[string]string{
		"list.fields": argStr(args, "list_fields"),
	})
	data, err := t.get(ctx, "/lists/%s%s", id, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getOwnedLists(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	userID := argStr(args, "user_id")
	q := queryEncode(map[string]string{
		"max_results":      argStr(args, "max_results"),
		"list.fields":      argStr(args, "list_fields"),
		"pagination_token": argStr(args, "pagination_token"),
	})
	data, err := t.get(ctx, "/users/%s/owned_lists%s", userID, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createList(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"name": argStr(args, "name"),
	}
	if v := argStr(args, "description"); v != "" {
		body["description"] = v
	}
	if _, ok := args["private"]; ok {
		body["private"] = argBool(args, "private")
	}
	data, err := t.post(ctx, "/lists", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateList(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "id")
	body := map[string]any{}
	if v := argStr(args, "name"); v != "" {
		body["name"] = v
	}
	if v := argStr(args, "description"); v != "" {
		body["description"] = v
	}
	if _, ok := args["private"]; ok {
		body["private"] = argBool(args, "private")
	}
	data, err := t.put(ctx, fmt.Sprintf("/lists/%s", id), body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteList(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.del(ctx, "/lists/%s", argStr(args, "id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getListTweets(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "id")
	q := queryEncode(map[string]string{
		"max_results":      argStr(args, "max_results"),
		"tweet.fields":     argStr(args, "tweet_fields"),
		"pagination_token": argStr(args, "pagination_token"),
	})
	data, err := t.get(ctx, "/lists/%s/tweets%s", id, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getListMembers(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "id")
	q := queryEncode(map[string]string{
		"max_results":      argStr(args, "max_results"),
		"user.fields":      argStr(args, "user_fields"),
		"pagination_token": argStr(args, "pagination_token"),
	})
	data, err := t.get(ctx, "/lists/%s/members%s", id, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func addListMember(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "id")
	data, err := t.post(ctx, fmt.Sprintf("/lists/%s/members", id), map[string]string{
		"user_id": argStr(args, "user_id"),
	})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func removeListMember(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.del(ctx, "/lists/%s/members/%s", argStr(args, "id"), argStr(args, "user_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getListFollowers(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "id")
	q := queryEncode(map[string]string{
		"max_results":      argStr(args, "max_results"),
		"user.fields":      argStr(args, "user_fields"),
		"pagination_token": argStr(args, "pagination_token"),
	})
	data, err := t.get(ctx, "/lists/%s/followers%s", id, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func followList(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.post(ctx, fmt.Sprintf("/users/%s/followed_lists", t.me()), map[string]string{
		"list_id": argStr(args, "list_id"),
	})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func unfollowList(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.del(ctx, "/users/%s/followed_lists/%s", t.me(), argStr(args, "list_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getPinnedLists(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"list.fields": argStr(args, "list_fields"),
	})
	data, err := t.get(ctx, "/users/%s/pinned_lists%s", t.me(), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func pinList(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.post(ctx, fmt.Sprintf("/users/%s/pinned_lists", t.me()), map[string]string{
		"list_id": argStr(args, "list_id"),
	})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func unpinList(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.del(ctx, "/users/%s/pinned_lists/%s", t.me(), argStr(args, "list_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
