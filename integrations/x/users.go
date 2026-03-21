package x

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func getUser(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	id := r.Str("id")
	q := queryEncode(map[string]string{
		"user.fields": r.Str("user_fields"),
	})
	data, err := t.get(ctx, "/users/%s%s", id, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getUserByUsername(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	username := r.Str("username")
	q := queryEncode(map[string]string{
		"user.fields": r.Str("user_fields"),
	})
	data, err := t.get(ctx, "/users/by/username/%s%s", username, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getUsers(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"ids":         r.Str("ids"),
		"user.fields": r.Str("user_fields"),
	})
	data, err := t.get(ctx, "/users%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchUsers(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"query":       r.Str("query"),
		"max_results": r.Str("max_results"),
		"user.fields": r.Str("user_fields"),
	})
	data, err := t.get(ctx, "/users/search%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getMe(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"user.fields": r.Str("user_fields"),
	})
	data, err := t.get(ctx, "/users/me%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Follows ---

func getFollowing(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	userID := r.Str("user_id")
	q := queryEncode(map[string]string{
		"max_results":      r.Str("max_results"),
		"user.fields":      r.Str("user_fields"),
		"pagination_token": r.Str("pagination_token"),
	})
	data, err := t.get(ctx, "/users/%s/following%s", userID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getFollowers(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	userID := r.Str("user_id")
	q := queryEncode(map[string]string{
		"max_results":      r.Str("max_results"),
		"user.fields":      r.Str("user_fields"),
		"pagination_token": r.Str("pagination_token"),
	})
	data, err := t.get(ctx, "/users/%s/followers%s", userID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func followUser(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.post(ctx, fmt.Sprintf("/users/%s/following", t.me()), map[string]string{
		"target_user_id": r.Str("target_user_id"),
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func unfollowUser(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.del(ctx, "/users/%s/following/%s", t.me(), r.Str("target_user_id"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Blocks ---

func getBlocked(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"max_results":      r.Str("max_results"),
		"user.fields":      r.Str("user_fields"),
		"pagination_token": r.Str("pagination_token"),
	})
	data, err := t.get(ctx, "/users/%s/blocking%s", t.me(), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func blockUser(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.post(ctx, fmt.Sprintf("/users/%s/blocking", t.me()), map[string]string{
		"target_user_id": r.Str("target_user_id"),
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func unblockUser(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.del(ctx, "/users/%s/blocking/%s", t.me(), r.Str("target_user_id"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Mutes ---

func getMuted(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"max_results":      r.Str("max_results"),
		"user.fields":      r.Str("user_fields"),
		"pagination_token": r.Str("pagination_token"),
	})
	data, err := t.get(ctx, "/users/%s/muting%s", t.me(), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func muteUser(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.post(ctx, fmt.Sprintf("/users/%s/muting", t.me()), map[string]string{
		"target_user_id": r.Str("target_user_id"),
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func unmuteUser(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.del(ctx, "/users/%s/muting/%s", t.me(), r.Str("target_user_id"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
