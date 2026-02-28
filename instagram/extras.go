package instagram

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// ── Comments ────────────────────────────────────────────────────────

func listComments(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	mediaID := argStr(args, "media_id")
	fields := argStr(args, "fields")
	if fields == "" {
		fields = "id,text,timestamp,username,like_count"
	}
	q := "fields=" + fields
	q += queryEncode(map[string]string{
		"limit": argStr(args, "limit"),
		"after": argStr(args, "after"),
	})
	data, err := ig.get(ctx, "/%s/comments?%s", mediaID, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getComment(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	commentID := argStr(args, "comment_id")
	fields := argStr(args, "fields")
	if fields == "" {
		fields = "id,text,timestamp,username,like_count"
	}
	data, err := ig.get(ctx, "/%s?fields=%s", commentID, fields)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func replyToComment(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	mediaID := argStr(args, "media_id")
	message := argStr(args, "message")
	path := fmt.Sprintf("/%s/comments?message=%s", mediaID, message)
	data, err := ig.post(ctx, path, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listCommentReplies(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	commentID := argStr(args, "comment_id")
	fields := argStr(args, "fields")
	if fields == "" {
		fields = "id,text,timestamp,username"
	}
	q := "fields=" + fields
	q += queryEncode(map[string]string{
		"limit": argStr(args, "limit"),
	})
	data, err := ig.get(ctx, "/%s/replies?%s", commentID, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func hideComment(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	commentID := argStr(args, "comment_id")
	hide := true
	if v, ok := args["hide"]; ok {
		hide = argBool(map[string]any{"hide": v}, "hide")
	}
	path := fmt.Sprintf("/%s?hide=%t", commentID, hide)
	data, err := ig.post(ctx, path, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteComment(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	commentID := argStr(args, "comment_id")
	data, err := ig.del(ctx, "/%s", commentID)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getMentionedComment(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	commentID := argStr(args, "comment_id")
	fields := argStr(args, "fields")
	if fields == "" {
		fields = "id,text,timestamp"
	}
	path := fmt.Sprintf("/%s/mentioned_comment?comment_id=%s&fields=%s", ig.uid(args), commentID, fields)
	data, err := ig.get(ctx, "%s", path)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getMentionedMedia(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	mediaID := argStr(args, "media_id")
	fields := argStr(args, "fields")
	if fields == "" {
		fields = "id,caption,media_type,timestamp"
	}
	path := fmt.Sprintf("/%s/mentioned_media?media_id=%s&fields=%s", ig.uid(args), mediaID, fields)
	data, err := ig.get(ctx, "%s", path)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Insights ────────────────────────────────────────────────────────

func getMediaInsights(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	mediaID := argStr(args, "media_id")
	metric := argStr(args, "metric")
	data, err := ig.get(ctx, "/%s/insights?metric=%s", mediaID, metric)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getAccountInsights(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	metric := argStr(args, "metric")
	period := argStr(args, "period")
	q := fmt.Sprintf("metric=%s&period=%s", metric, period)
	q += queryEncode(map[string]string{
		"since": argStr(args, "since"),
		"until": argStr(args, "until"),
	})
	data, err := ig.get(ctx, "/%s/insights?%s", ig.uid(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Hashtags ────────────────────────────────────────────────────────

func searchHashtag(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	q := argStr(args, "q")
	data, err := ig.get(ctx, "/ig_hashtag_search?user_id=%s&q=%s", ig.uid(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getHashtagRecent(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	hashtagID := argStr(args, "hashtag_id")
	fields := argStr(args, "fields")
	if fields == "" {
		fields = "id,caption,media_type,permalink,timestamp"
	}
	data, err := ig.get(ctx, "/%s/recent_media?user_id=%s&fields=%s", hashtagID, ig.uid(args), fields)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getHashtagTop(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	hashtagID := argStr(args, "hashtag_id")
	fields := argStr(args, "fields")
	if fields == "" {
		fields = "id,caption,media_type,permalink,timestamp"
	}
	data, err := ig.get(ctx, "/%s/top_media?user_id=%s&fields=%s", hashtagID, ig.uid(args), fields)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
