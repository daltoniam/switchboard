package instagram

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func getProfile(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	fields := argStr(args, "fields")
	if fields == "" {
		fields = "id,username,account_type,media_count,followers_count,follows_count,biography,website,profile_picture_url,name"
	}
	data, err := ig.get(ctx, "/%s?fields=%s", ig.uid(args), fields)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func discoverUser(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	username := argStr(args, "username")
	fields := argStr(args, "fields")
	if fields == "" {
		fields = "username,biography,media_count,followers_count,follows_count,website,profile_picture_url"
	}
	path := fmt.Sprintf("/%s?fields=business_discovery.username(%s){%s}", ig.uid(args), username, fields)
	data, err := ig.get(ctx, "%s", path)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getRecentlySearched(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	data, err := ig.get(ctx, "/%s/recently_searched_hashtags", ig.uid(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
