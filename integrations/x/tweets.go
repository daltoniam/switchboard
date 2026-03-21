package x

import (
	"context"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func getTweet(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	id := r.Str("id")
	q := queryEncode(map[string]string{
		"tweet.fields": r.Str("tweet_fields"),
		"expansions":   r.Str("expansions"),
		"user.fields":  r.Str("user_fields"),
	})
	data, err := t.get(ctx, "/tweets/%s%s", id, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTweets(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"ids":          r.Str("ids"),
		"tweet.fields": r.Str("tweet_fields"),
		"expansions":   r.Str("expansions"),
		"user.fields":  r.Str("user_fields"),
	})
	data, err := t.get(ctx, "/tweets%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createTweet(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"text": r.Str("text"),
	}
	if v := r.Str("reply_to"); v != "" {
		body["reply"] = map[string]string{"in_reply_to_tweet_id": v}
	}
	if v := r.Str("quote_tweet_id"); v != "" {
		body["quote_tweet_id"] = v
	}
	if v := r.Str("poll_options"); v != "" {
		options := strings.Split(v, ",")
		for i := range options {
			options[i] = strings.TrimSpace(options[i])
		}
		duration := 1440
		if d := r.Int("poll_duration_minutes"); d > 0 {
			duration = d
		}
		body["poll"] = map[string]any{
			"options":          options,
			"duration_minutes": duration,
		}
	}
	data, err := t.post(ctx, "/tweets", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteTweet(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.del(ctx, "/tweets/%s", r.Str("id"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchRecent(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"query":        r.Str("query"),
		"max_results":  r.Str("max_results"),
		"next_token":   r.Str("next_token"),
		"tweet.fields": r.Str("tweet_fields"),
		"start_time":   r.Str("start_time"),
		"end_time":     r.Str("end_time"),
		"sort_order":   r.Str("sort_order"),
	})
	data, err := t.get(ctx, "/tweets/search/recent%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchAll(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"query":        r.Str("query"),
		"max_results":  r.Str("max_results"),
		"next_token":   r.Str("next_token"),
		"tweet.fields": r.Str("tweet_fields"),
		"start_time":   r.Str("start_time"),
		"end_time":     r.Str("end_time"),
		"sort_order":   r.Str("sort_order"),
	})
	data, err := t.get(ctx, "/tweets/search/all%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTweetCount(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"query":       r.Str("query"),
		"granularity": r.Str("granularity"),
		"start_time":  r.Str("start_time"),
		"end_time":    r.Str("end_time"),
	})
	data, err := t.get(ctx, "/tweets/counts/recent%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getQuoteTweets(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
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
	data, err := t.get(ctx, "/tweets/%s/quote_tweets%s", id, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func hideReply(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.put(ctx, fmt.Sprintf("/tweets/%s/hidden", r.Str("id")), map[string]any{"hidden": true})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func unhideReply(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.put(ctx, fmt.Sprintf("/tweets/%s/hidden", r.Str("id")), map[string]any{"hidden": false})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Timelines ---

func getUserTweets(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	userID := r.Str("user_id")
	q := queryEncode(map[string]string{
		"max_results":      r.Str("max_results"),
		"tweet.fields":     r.Str("tweet_fields"),
		"pagination_token": r.Str("pagination_token"),
		"start_time":       r.Str("start_time"),
		"end_time":         r.Str("end_time"),
		"exclude":          r.Str("exclude"),
	})
	data, err := t.get(ctx, "/users/%s/tweets%s", userID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getUserMentions(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	userID := r.Str("user_id")
	q := queryEncode(map[string]string{
		"max_results":      r.Str("max_results"),
		"tweet.fields":     r.Str("tweet_fields"),
		"pagination_token": r.Str("pagination_token"),
		"start_time":       r.Str("start_time"),
		"end_time":         r.Str("end_time"),
	})
	data, err := t.get(ctx, "/users/%s/mentions%s", userID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getHomeTimeline(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"max_results":      r.Str("max_results"),
		"tweet.fields":     r.Str("tweet_fields"),
		"pagination_token": r.Str("pagination_token"),
		"start_time":       r.Str("start_time"),
		"end_time":         r.Str("end_time"),
		"exclude":          r.Str("exclude"),
	})
	data, err := t.get(ctx, "/users/%s/timelines/reverse_chronological%s", t.me(), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Likes ---

func getLikingUsers(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	tweetID := r.Str("tweet_id")
	q := queryEncode(map[string]string{
		"max_results":      r.Str("max_results"),
		"user.fields":      r.Str("user_fields"),
		"pagination_token": r.Str("pagination_token"),
	})
	data, err := t.get(ctx, "/tweets/%s/liking_users%s", tweetID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getLikedTweets(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	userID := r.Str("user_id")
	q := queryEncode(map[string]string{
		"max_results":      r.Str("max_results"),
		"tweet.fields":     r.Str("tweet_fields"),
		"pagination_token": r.Str("pagination_token"),
	})
	data, err := t.get(ctx, "/users/%s/liked_tweets%s", userID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func likeTweet(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.post(ctx, fmt.Sprintf("/users/%s/likes", t.me()), map[string]string{
		"tweet_id": r.Str("tweet_id"),
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func unlikeTweet(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.del(ctx, "/users/%s/likes/%s", t.me(), r.Str("tweet_id"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Retweets ---

func getRetweeters(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	tweetID := r.Str("tweet_id")
	q := queryEncode(map[string]string{
		"max_results":      r.Str("max_results"),
		"user.fields":      r.Str("user_fields"),
		"pagination_token": r.Str("pagination_token"),
	})
	data, err := t.get(ctx, "/tweets/%s/retweeted_by%s", tweetID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func retweet(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.post(ctx, fmt.Sprintf("/users/%s/retweets", t.me()), map[string]string{
		"tweet_id": r.Str("tweet_id"),
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func unretweet(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.del(ctx, "/users/%s/retweets/%s", t.me(), r.Str("tweet_id"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Bookmarks ---

func getBookmarks(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"max_results":      r.Str("max_results"),
		"tweet.fields":     r.Str("tweet_fields"),
		"pagination_token": r.Str("pagination_token"),
	})
	data, err := t.get(ctx, "/users/%s/bookmarks%s", t.me(), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func bookmarkTweet(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.post(ctx, fmt.Sprintf("/users/%s/bookmarks", t.me()), map[string]string{
		"tweet_id": r.Str("tweet_id"),
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func removeBookmark(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.del(ctx, "/users/%s/bookmarks/%s", t.me(), r.Str("tweet_id"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
