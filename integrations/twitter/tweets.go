package twitter

import (
	"context"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func getTweet(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "id")
	q := queryEncode(map[string]string{
		"tweet.fields": argStr(args, "tweet_fields"),
		"expansions":   argStr(args, "expansions"),
		"user.fields":  argStr(args, "user_fields"),
	})
	data, err := t.get(ctx, "/tweets/%s%s", id, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getTweets(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"ids":          argStr(args, "ids"),
		"tweet.fields": argStr(args, "tweet_fields"),
		"expansions":   argStr(args, "expansions"),
		"user.fields":  argStr(args, "user_fields"),
	})
	data, err := t.get(ctx, "/tweets%s", q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createTweet(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"text": argStr(args, "text"),
	}
	if v := argStr(args, "reply_to"); v != "" {
		body["reply"] = map[string]string{"in_reply_to_tweet_id": v}
	}
	if v := argStr(args, "quote_tweet_id"); v != "" {
		body["quote_tweet_id"] = v
	}
	if v := argStr(args, "poll_options"); v != "" {
		options := strings.Split(v, ",")
		for i := range options {
			options[i] = strings.TrimSpace(options[i])
		}
		duration := 1440
		if d := argInt(args, "poll_duration_minutes"); d > 0 {
			duration = d
		}
		body["poll"] = map[string]any{
			"options":          options,
			"duration_minutes": duration,
		}
	}
	data, err := t.post(ctx, "/tweets", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteTweet(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.del(ctx, "/tweets/%s", argStr(args, "id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func searchRecent(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"query":        argStr(args, "query"),
		"max_results":  argStr(args, "max_results"),
		"next_token":   argStr(args, "next_token"),
		"tweet.fields": argStr(args, "tweet_fields"),
		"start_time":   argStr(args, "start_time"),
		"end_time":     argStr(args, "end_time"),
		"sort_order":   argStr(args, "sort_order"),
	})
	data, err := t.get(ctx, "/tweets/search/recent%s", q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func searchAll(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"query":        argStr(args, "query"),
		"max_results":  argStr(args, "max_results"),
		"next_token":   argStr(args, "next_token"),
		"tweet.fields": argStr(args, "tweet_fields"),
		"start_time":   argStr(args, "start_time"),
		"end_time":     argStr(args, "end_time"),
		"sort_order":   argStr(args, "sort_order"),
	})
	data, err := t.get(ctx, "/tweets/search/all%s", q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getTweetCount(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"query":       argStr(args, "query"),
		"granularity": argStr(args, "granularity"),
		"start_time":  argStr(args, "start_time"),
		"end_time":    argStr(args, "end_time"),
	})
	data, err := t.get(ctx, "/tweets/counts/recent%s", q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getQuoteTweets(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "id")
	q := queryEncode(map[string]string{
		"max_results":      argStr(args, "max_results"),
		"tweet.fields":     argStr(args, "tweet_fields"),
		"pagination_token": argStr(args, "pagination_token"),
	})
	data, err := t.get(ctx, "/tweets/%s/quote_tweets%s", id, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func hideReply(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.put(ctx, fmt.Sprintf("/tweets/%s/hidden", argStr(args, "id")), map[string]any{"hidden": true})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func unhideReply(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.put(ctx, fmt.Sprintf("/tweets/%s/hidden", argStr(args, "id")), map[string]any{"hidden": false})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// --- Timelines ---

func getUserTweets(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	userID := argStr(args, "user_id")
	q := queryEncode(map[string]string{
		"max_results":      argStr(args, "max_results"),
		"tweet.fields":     argStr(args, "tweet_fields"),
		"pagination_token": argStr(args, "pagination_token"),
		"start_time":       argStr(args, "start_time"),
		"end_time":         argStr(args, "end_time"),
		"exclude":          argStr(args, "exclude"),
	})
	data, err := t.get(ctx, "/users/%s/tweets%s", userID, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getUserMentions(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	userID := argStr(args, "user_id")
	q := queryEncode(map[string]string{
		"max_results":      argStr(args, "max_results"),
		"tweet.fields":     argStr(args, "tweet_fields"),
		"pagination_token": argStr(args, "pagination_token"),
		"start_time":       argStr(args, "start_time"),
		"end_time":         argStr(args, "end_time"),
	})
	data, err := t.get(ctx, "/users/%s/mentions%s", userID, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getHomeTimeline(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"max_results":      argStr(args, "max_results"),
		"tweet.fields":     argStr(args, "tweet_fields"),
		"pagination_token": argStr(args, "pagination_token"),
		"start_time":       argStr(args, "start_time"),
		"end_time":         argStr(args, "end_time"),
		"exclude":          argStr(args, "exclude"),
	})
	data, err := t.get(ctx, "/users/%s/timelines/reverse_chronological%s", t.me(), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// --- Likes ---

func getLikingUsers(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	tweetID := argStr(args, "tweet_id")
	q := queryEncode(map[string]string{
		"max_results":      argStr(args, "max_results"),
		"user.fields":      argStr(args, "user_fields"),
		"pagination_token": argStr(args, "pagination_token"),
	})
	data, err := t.get(ctx, "/tweets/%s/liking_users%s", tweetID, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getLikedTweets(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	userID := argStr(args, "user_id")
	q := queryEncode(map[string]string{
		"max_results":      argStr(args, "max_results"),
		"tweet.fields":     argStr(args, "tweet_fields"),
		"pagination_token": argStr(args, "pagination_token"),
	})
	data, err := t.get(ctx, "/users/%s/liked_tweets%s", userID, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func likeTweet(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.post(ctx, fmt.Sprintf("/users/%s/likes", t.me()), map[string]string{
		"tweet_id": argStr(args, "tweet_id"),
	})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func unlikeTweet(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.del(ctx, "/users/%s/likes/%s", t.me(), argStr(args, "tweet_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// --- Retweets ---

func getRetweeters(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	tweetID := argStr(args, "tweet_id")
	q := queryEncode(map[string]string{
		"max_results":      argStr(args, "max_results"),
		"user.fields":      argStr(args, "user_fields"),
		"pagination_token": argStr(args, "pagination_token"),
	})
	data, err := t.get(ctx, "/tweets/%s/retweeted_by%s", tweetID, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func retweet(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.post(ctx, fmt.Sprintf("/users/%s/retweets", t.me()), map[string]string{
		"tweet_id": argStr(args, "tweet_id"),
	})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func unretweet(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.del(ctx, "/users/%s/retweets/%s", t.me(), argStr(args, "tweet_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// --- Bookmarks ---

func getBookmarks(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"max_results":      argStr(args, "max_results"),
		"tweet.fields":     argStr(args, "tweet_fields"),
		"pagination_token": argStr(args, "pagination_token"),
	})
	data, err := t.get(ctx, "/users/%s/bookmarks%s", t.me(), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func bookmarkTweet(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.post(ctx, fmt.Sprintf("/users/%s/bookmarks", t.me()), map[string]string{
		"tweet_id": argStr(args, "tweet_id"),
	})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func removeBookmark(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error) {
	data, err := t.del(ctx, "/users/%s/bookmarks/%s", t.me(), argStr(args, "tweet_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}


