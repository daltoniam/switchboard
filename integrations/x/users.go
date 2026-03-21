package x

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func getUser(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "id")
	q := queryEncode(map[string]string{
		"user.fields": argStr(args, "user_fields"),
	})
	data, err := t.get(ctx, "/users/%s%s", id, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getUserByUsername(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	username := argStr(args, "username")
	q := queryEncode(map[string]string{
		"user.fields": argStr(args, "user_fields"),
	})
	data, err := t.get(ctx, "/users/by/username/%s%s", username, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getUsers(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"ids":         argStr(args, "ids"),
		"user.fields": argStr(args, "user_fields"),
	})
	data, err := t.get(ctx, "/users%s", q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func searchUsers(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"query":       argStr(args, "query"),
		"max_results": argStr(args, "max_results"),
		"user.fields": argStr(args, "user_fields"),
	})
	data, err := t.get(ctx, "/users/search%s", q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getMe(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"user.fields": argStr(args, "user_fields"),
	})
	data, err := t.get(ctx, "/users/me%s", q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// --- Follows ---

func getFollowing(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	userID := argStr(args, "user_id")
	q := queryEncode(map[string]string{
		"max_results":      argStr(args, "max_results"),
		"user.fields":      argStr(args, "user_fields"),
		"pagination_token": argStr(args, "pagination_token"),
	})
	data, err := t.get(ctx, "/users/%s/following%s", userID, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getFollowers(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	userID := argStr(args, "user_id")
	q := queryEncode(map[string]string{
		"max_results":      argStr(args, "max_results"),
		"user.fields":      argStr(args, "user_fields"),
		"pagination_token": argStr(args, "pagination_token"),
	})
	data, err := t.get(ctx, "/users/%s/followers%s", userID, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func followUser(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.post(ctx, fmt.Sprintf("/users/%s/following", t.me()), map[string]string{
		"target_user_id": argStr(args, "target_user_id"),
	})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func unfollowUser(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.del(ctx, "/users/%s/following/%s", t.me(), argStr(args, "target_user_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// --- Blocks ---

func getBlocked(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"max_results":      argStr(args, "max_results"),
		"user.fields":      argStr(args, "user_fields"),
		"pagination_token": argStr(args, "pagination_token"),
	})
	data, err := t.get(ctx, "/users/%s/blocking%s", t.me(), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func blockUser(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.post(ctx, fmt.Sprintf("/users/%s/blocking", t.me()), map[string]string{
		"target_user_id": argStr(args, "target_user_id"),
	})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func unblockUser(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.del(ctx, "/users/%s/blocking/%s", t.me(), argStr(args, "target_user_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// --- Mutes ---

func getMuted(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"max_results":      argStr(args, "max_results"),
		"user.fields":      argStr(args, "user_fields"),
		"pagination_token": argStr(args, "pagination_token"),
	})
	data, err := t.get(ctx, "/users/%s/muting%s", t.me(), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func muteUser(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.post(ctx, fmt.Sprintf("/users/%s/muting", t.me()), map[string]string{
		"target_user_id": argStr(args, "target_user_id"),
	})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func unmuteUser(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.del(ctx, "/users/%s/muting/%s", t.me(), argStr(args, "target_user_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
