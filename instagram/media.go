package instagram

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listMedia(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	fields := argStr(args, "fields")
	if fields == "" {
		fields = "id,caption,media_type,media_url,permalink,timestamp,thumbnail_url,like_count,comments_count"
	}
	q := "fields=" + fields
	q += queryEncode(map[string]string{
		"limit":  argStr(args, "limit"),
		"after":  argStr(args, "after"),
		"before": argStr(args, "before"),
	})
	data, err := ig.get(ctx, "/%s/media?%s", ig.uid(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getMedia(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	mediaID := argStr(args, "media_id")
	fields := argStr(args, "fields")
	if fields == "" {
		fields = "id,caption,media_type,media_url,permalink,timestamp,thumbnail_url,like_count,comments_count"
	}
	data, err := ig.get(ctx, "/%s?fields=%s", mediaID, fields)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listStories(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	data, err := ig.get(ctx, "/%s/stories", ig.uid(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getStory(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	storyID := argStr(args, "story_id")
	fields := argStr(args, "fields")
	if fields == "" {
		fields = "id,media_type,media_url,timestamp"
	}
	data, err := ig.get(ctx, "/%s?fields=%s", storyID, fields)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listMediaChildren(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	mediaID := argStr(args, "media_id")
	fields := argStr(args, "fields")
	if fields == "" {
		fields = "id,media_type,media_url,timestamp"
	}
	data, err := ig.get(ctx, "/%s/children?fields=%s", mediaID, fields)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createMediaContainer(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{}
	if v := argStr(args, "image_url"); v != "" {
		params["image_url"] = v
	}
	if v := argStr(args, "video_url"); v != "" {
		params["video_url"] = v
	}
	if v := argStr(args, "media_type"); v != "" {
		params["media_type"] = v
	}
	if v := argStr(args, "caption"); v != "" {
		params["caption"] = v
	}
	if v := argStr(args, "children"); v != "" {
		params["children"] = v
	}
	q := queryEncode(params)
	path := fmt.Sprintf("/%s/media?%s", ig.uid(args), q)
	data, err := ig.post(ctx, path, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func publishMedia(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	containerID := argStr(args, "container_id")
	path := fmt.Sprintf("/%s/media_publish?creation_id=%s", ig.uid(args), containerID)
	data, err := ig.post(ctx, path, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getPublishStatus(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	containerID := argStr(args, "container_id")
	data, err := ig.get(ctx, "/%s?fields=status_code,status", containerID)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
