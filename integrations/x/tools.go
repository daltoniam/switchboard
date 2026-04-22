package x

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Tweets ─────────────────────────────────────────────────────
	{
		Name: "x_get_tweet", Description: "Get a single tweet by ID. Returns full tweet object with text, author, metrics, and entities",
		Parameters: map[string]string{
			"id":           "Tweet ID",
			"tweet_fields": "Comma-separated tweet fields: attachments,author_id,created_at,entities,geo,id,lang,public_metrics,source,text,conversation_id,reply_settings",
			"expansions":   "Comma-separated expansions: author_id,referenced_tweets.id,attachments.media_keys",
			"user_fields":  "Comma-separated user fields: id,name,username,profile_image_url,verified,public_metrics",
		},
		Required: []string{"id"},
	},
	{
		Name: "x_get_tweets", Description: "Get multiple tweets by IDs (up to 100). Start here for bulk tweet lookups",
		Parameters: map[string]string{
			"ids":          "Comma-separated tweet IDs (max 100)",
			"tweet_fields": "Comma-separated tweet fields: attachments,author_id,created_at,entities,geo,id,lang,public_metrics,source,text",
			"expansions":   "Comma-separated expansions: author_id,referenced_tweets.id,attachments.media_keys",
			"user_fields":  "Comma-separated user fields: id,name,username,profile_image_url,verified,public_metrics",
		},
		Required: []string{"ids"},
	},
	{
		Name: "x_create_tweet", Description: "Post a new tweet. Supports text, replies, quotes, and polls",
		Parameters: map[string]string{
			"text":                  "Tweet text (up to 280 chars)",
			"reply_to":              "Tweet ID to reply to",
			"quote_tweet_id":        "Tweet ID to quote",
			"poll_options":          "Comma-separated poll options (2-4 choices)",
			"poll_duration_minutes": "Poll duration in minutes (5-10080)",
		},
		Required: []string{"text"},
	},
	{
		Name: "x_delete_tweet", Description: "Delete a tweet by ID. Must be authored by the authenticated user",
		Parameters: map[string]string{"id": "Tweet ID to delete"},
		Required:   []string{"id"},
	},
	{
		Name: "x_search_recent", Description: "Search tweets from the last 7 days. Start here for most tweet discovery workflows. Supports the full X search query syntax",
		Parameters: map[string]string{
			"query":        "Search query (required). Supports operators: from:, to:, is:retweet, has:media, has:links, lang:, -is:retweet, etc.",
			"max_results":  "Results per page (10-100, default 10)",
			"next_token":   "Pagination token from previous response",
			"tweet_fields": "Comma-separated tweet fields: author_id,created_at,public_metrics,source,text,conversation_id,entities",
			"start_time":   "Oldest UTC datetime (ISO 8601: 2024-01-01T00:00:00Z)",
			"end_time":     "Newest UTC datetime (ISO 8601)",
			"sort_order":   "Order of results: recency or relevancy",
		},
		Required: []string{"query"},
	},
	{
		Name: "x_search_all", Description: "Full-archive search across all tweets (requires Pro tier or higher). Use x_search_recent for 7-day window instead",
		Parameters: map[string]string{
			"query":        "Search query (required). Same operators as x_search_recent",
			"max_results":  "Results per page (10-500, default 10)",
			"next_token":   "Pagination token from previous response",
			"tweet_fields": "Comma-separated tweet fields",
			"start_time":   "Oldest UTC datetime (ISO 8601)",
			"end_time":     "Newest UTC datetime (ISO 8601)",
			"sort_order":   "Order of results: recency or relevancy",
		},
		Required: []string{"query"},
	},
	{
		Name: "x_get_tweet_count", Description: "Get count of tweets matching a search query from the last 7 days. Useful for analytics without retrieving full tweets",
		Parameters: map[string]string{
			"query":       "Search query",
			"granularity": "Aggregation granularity: minute, hour, or day (default day)",
			"start_time":  "Oldest UTC datetime (ISO 8601)",
			"end_time":    "Newest UTC datetime (ISO 8601)",
		},
		Required: []string{"query"},
	},
	{
		Name: "x_get_quote_tweets", Description: "Get tweets that quote a specific tweet",
		Parameters: map[string]string{
			"id":               "Tweet ID to get quotes for",
			"max_results":      "Results per page (10-100, default 10)",
			"tweet_fields":     "Comma-separated tweet fields",
			"pagination_token": "Pagination token",
		},
		Required: []string{"id"},
	},
	{
		Name: "x_hide_reply", Description: "Hide a reply to one of your tweets",
		Parameters: map[string]string{"id": "Reply tweet ID to hide"},
		Required:   []string{"id"},
	},
	{
		Name: "x_unhide_reply", Description: "Unhide a previously hidden reply",
		Parameters: map[string]string{"id": "Reply tweet ID to unhide"},
		Required:   []string{"id"},
	},

	// ── Timelines ──────────────────────────────────────────────────
	{
		Name: "x_get_user_tweets", Description: "Get tweets posted by a specific user. Use user_id from x_get_user or x_get_user_by_username",
		Parameters: map[string]string{
			"user_id":          "User ID (not username — use x_get_user_by_username to resolve)",
			"max_results":      "Results per page (5-100, default 10)",
			"tweet_fields":     "Comma-separated tweet fields",
			"pagination_token": "Pagination token",
			"start_time":       "Oldest UTC datetime (ISO 8601)",
			"end_time":         "Newest UTC datetime (ISO 8601)",
			"exclude":          "Comma-separated: retweets,replies",
		},
		Required: []string{"user_id"},
	},
	{
		Name: "x_get_user_mentions", Description: "Get tweets mentioning a specific user",
		Parameters: map[string]string{
			"user_id":          "User ID",
			"max_results":      "Results per page (5-100, default 10)",
			"tweet_fields":     "Comma-separated tweet fields",
			"pagination_token": "Pagination token",
			"start_time":       "Oldest UTC datetime (ISO 8601)",
			"end_time":         "Newest UTC datetime (ISO 8601)",
		},
		Required: []string{"user_id"},
	},
	{
		Name: "x_get_home_timeline", Description: "Get the authenticated user's home timeline (reverse chronological). Requires user context auth",
		Parameters: map[string]string{
			"max_results":      "Results per page (1-100, default 10)",
			"tweet_fields":     "Comma-separated tweet fields",
			"pagination_token": "Pagination token",
			"start_time":       "Oldest UTC datetime (ISO 8601)",
			"end_time":         "Newest UTC datetime (ISO 8601)",
			"exclude":          "Comma-separated: retweets,replies",
		},
	},

	// ── Users ──────────────────────────────────────────────────────
	{
		Name: "x_get_user", Description: "Get a user by their numeric ID. Use x_get_user_by_username if you have a @handle instead",
		Parameters: map[string]string{
			"id":          "User ID",
			"user_fields": "Comma-separated: id,name,username,created_at,description,location,pinned_tweet_id,profile_image_url,protected,public_metrics,url,verified",
		},
		Required: []string{"id"},
	},
	{
		Name: "x_get_user_by_username", Description: "Get a user by @username. Preferred entry point when you know the handle. Returns user ID needed for other endpoints",
		Parameters: map[string]string{
			"username":    "Username without @ prefix",
			"user_fields": "Comma-separated user fields",
		},
		Required: []string{"username"},
	},
	{
		Name: "x_get_users", Description: "Get multiple users by IDs (up to 100)",
		Parameters: map[string]string{
			"ids":         "Comma-separated user IDs (max 100)",
			"user_fields": "Comma-separated user fields",
		},
		Required: []string{"ids"},
	},
	{
		Name: "x_search_users", Description: "Search for users by name or username",
		Parameters: map[string]string{
			"query":       "Search query",
			"max_results": "Results per page (1-100, default 10)",
			"user_fields": "Comma-separated user fields",
		},
		Required: []string{"query"},
	},
	{
		Name: "x_get_me", Description: "Get the authenticated user's profile information",
		Parameters: map[string]string{
			"user_fields": "Comma-separated user fields",
		},
	},

	// ── Follows ────────────────────────────────────────────────────
	{
		Name: "x_get_following", Description: "Get users that a user follows",
		Parameters: map[string]string{
			"user_id":          "User ID",
			"max_results":      "Results per page (1-1000, default 100)",
			"user_fields":      "Comma-separated user fields",
			"pagination_token": "Pagination token",
		},
		Required: []string{"user_id"},
	},
	{
		Name: "x_get_followers", Description: "Get users who follow a user",
		Parameters: map[string]string{
			"user_id":          "User ID",
			"max_results":      "Results per page (1-1000, default 100)",
			"user_fields":      "Comma-separated user fields",
			"pagination_token": "Pagination token",
		},
		Required: []string{"user_id"},
	},
	{
		Name: "x_follow_user", Description: "Follow a user. Uses the authenticated user's ID automatically",
		Parameters: map[string]string{"target_user_id": "User ID to follow"},
		Required:   []string{"target_user_id"},
	},
	{
		Name: "x_unfollow_user", Description: "Unfollow a user",
		Parameters: map[string]string{"target_user_id": "User ID to unfollow"},
		Required:   []string{"target_user_id"},
	},

	// ── Blocks ─────────────────────────────────────────────────────
	{
		Name: "x_get_blocked", Description: "Get users blocked by the authenticated user",
		Parameters: map[string]string{
			"max_results":      "Results per page (1-1000, default 100)",
			"user_fields":      "Comma-separated user fields",
			"pagination_token": "Pagination token",
		},
	},
	{
		Name: "x_block_user", Description: "Block a user",
		Parameters: map[string]string{"target_user_id": "User ID to block"},
		Required:   []string{"target_user_id"},
	},
	{
		Name: "x_unblock_user", Description: "Unblock a user",
		Parameters: map[string]string{"target_user_id": "User ID to unblock"},
		Required:   []string{"target_user_id"},
	},

	// ── Mutes ──────────────────────────────────────────────────────
	{
		Name: "x_get_muted", Description: "Get users muted by the authenticated user",
		Parameters: map[string]string{
			"max_results":      "Results per page (1-1000, default 100)",
			"user_fields":      "Comma-separated user fields",
			"pagination_token": "Pagination token",
		},
	},
	{
		Name: "x_mute_user", Description: "Mute a user",
		Parameters: map[string]string{"target_user_id": "User ID to mute"},
		Required:   []string{"target_user_id"},
	},
	{
		Name: "x_unmute_user", Description: "Unmute a user",
		Parameters: map[string]string{"target_user_id": "User ID to unmute"},
		Required:   []string{"target_user_id"},
	},

	// ── Likes ──────────────────────────────────────────────────────
	{
		Name: "x_get_liking_users", Description: "Get users who liked a specific tweet",
		Parameters: map[string]string{
			"tweet_id":         "Tweet ID",
			"max_results":      "Results per page (1-100, default 100)",
			"user_fields":      "Comma-separated user fields",
			"pagination_token": "Pagination token",
		},
		Required: []string{"tweet_id"},
	},
	{
		Name: "x_get_liked_tweets", Description: "Get tweets liked by a specific user",
		Parameters: map[string]string{
			"user_id":          "User ID",
			"max_results":      "Results per page (10-100, default 10)",
			"tweet_fields":     "Comma-separated tweet fields",
			"pagination_token": "Pagination token",
		},
		Required: []string{"user_id"},
	},
	{
		Name: "x_like_tweet", Description: "Like a tweet",
		Parameters: map[string]string{"tweet_id": "Tweet ID to like"},
		Required:   []string{"tweet_id"},
	},
	{
		Name: "x_unlike_tweet", Description: "Unlike a previously liked tweet",
		Parameters: map[string]string{"tweet_id": "Tweet ID to unlike"},
		Required:   []string{"tweet_id"},
	},

	// ── Retweets ───────────────────────────────────────────────────
	{
		Name: "x_get_retweeters", Description: "Get users who retweeted a specific tweet",
		Parameters: map[string]string{
			"tweet_id":         "Tweet ID",
			"max_results":      "Results per page (1-100, default 100)",
			"user_fields":      "Comma-separated user fields",
			"pagination_token": "Pagination token",
		},
		Required: []string{"tweet_id"},
	},
	{
		Name: "x_retweet", Description: "Retweet a tweet",
		Parameters: map[string]string{"tweet_id": "Tweet ID to retweet"},
		Required:   []string{"tweet_id"},
	},
	{
		Name: "x_unretweet", Description: "Remove a retweet",
		Parameters: map[string]string{"tweet_id": "Tweet ID to unretweet"},
		Required:   []string{"tweet_id"},
	},

	// ── Bookmarks ──────────────────────────────────────────────────
	{
		Name: "x_get_bookmarks", Description: "Get tweets bookmarked by the authenticated user",
		Parameters: map[string]string{
			"max_results":      "Results per page (1-100, default 10)",
			"tweet_fields":     "Comma-separated tweet fields",
			"pagination_token": "Pagination token",
		},
	},
	{
		Name: "x_bookmark_tweet", Description: "Bookmark a tweet",
		Parameters: map[string]string{"tweet_id": "Tweet ID to bookmark"},
		Required:   []string{"tweet_id"},
	},
	{
		Name: "x_remove_bookmark", Description: "Remove a tweet from bookmarks",
		Parameters: map[string]string{"tweet_id": "Tweet ID to remove from bookmarks"},
		Required:   []string{"tweet_id"},
	},

	// ── Lists ──────────────────────────────────────────────────────
	{
		Name: "x_get_list", Description: "Get a list by ID",
		Parameters: map[string]string{
			"id":          "List ID",
			"list_fields": "Comma-separated: id,name,description,private,follower_count,member_count,owner_id,created_at",
		},
		Required: []string{"id"},
	},
	{
		Name: "x_get_owned_lists", Description: "Get lists owned by a user",
		Parameters: map[string]string{
			"user_id":          "User ID",
			"max_results":      "Results per page (1-100, default 100)",
			"list_fields":      "Comma-separated list fields",
			"pagination_token": "Pagination token",
		},
		Required: []string{"user_id"},
	},
	{
		Name: "x_create_list", Description: "Create a new list",
		Parameters: map[string]string{
			"name":        "List name (1-25 chars)",
			"description": "List description",
			"private":     "Whether list is private (true/false, default false)",
		},
		Required: []string{"name"},
	},
	{
		Name: "x_update_list", Description: "Update a list's name, description, or privacy setting",
		Parameters: map[string]string{
			"id":          "List ID",
			"name":        "New list name",
			"description": "New list description",
			"private":     "Whether list is private (true/false)",
		},
		Required: []string{"id"},
	},
	{
		Name: "x_delete_list", Description: "Delete a list. Must be owned by the authenticated user",
		Parameters: map[string]string{"id": "List ID to delete"},
		Required:   []string{"id"},
	},
	{
		Name: "x_get_list_tweets", Description: "Get tweets from a list's timeline",
		Parameters: map[string]string{
			"id":               "List ID",
			"max_results":      "Results per page (1-100, default 100)",
			"tweet_fields":     "Comma-separated tweet fields",
			"pagination_token": "Pagination token",
		},
		Required: []string{"id"},
	},
	{
		Name: "x_get_list_members", Description: "Get members of a list",
		Parameters: map[string]string{
			"id":               "List ID",
			"max_results":      "Results per page (1-100, default 100)",
			"user_fields":      "Comma-separated user fields",
			"pagination_token": "Pagination token",
		},
		Required: []string{"id"},
	},
	{
		Name: "x_add_list_member", Description: "Add a user to a list. Must own the list",
		Parameters: map[string]string{"id": "List ID", "user_id": "User ID to add"},
		Required:   []string{"id", "user_id"},
	},
	{
		Name: "x_remove_list_member", Description: "Remove a user from a list",
		Parameters: map[string]string{"id": "List ID", "user_id": "User ID to remove"},
		Required:   []string{"id", "user_id"},
	},
	{
		Name: "x_get_list_followers", Description: "Get users who follow a list",
		Parameters: map[string]string{
			"id":               "List ID",
			"max_results":      "Results per page (1-100, default 100)",
			"user_fields":      "Comma-separated user fields",
			"pagination_token": "Pagination token",
		},
		Required: []string{"id"},
	},
	{
		Name: "x_follow_list", Description: "Follow a list",
		Parameters: map[string]string{"list_id": "List ID to follow"},
		Required:   []string{"list_id"},
	},
	{
		Name: "x_unfollow_list", Description: "Unfollow a list",
		Parameters: map[string]string{"list_id": "List ID to unfollow"},
		Required:   []string{"list_id"},
	},
	{
		Name: "x_get_pinned_lists", Description: "Get lists pinned by the authenticated user",
		Parameters: map[string]string{
			"list_fields": "Comma-separated list fields",
		},
	},
	{
		Name: "x_pin_list", Description: "Pin a list",
		Parameters: map[string]string{"list_id": "List ID to pin"},
		Required:   []string{"list_id"},
	},
	{
		Name: "x_unpin_list", Description: "Unpin a list",
		Parameters: map[string]string{"list_id": "List ID to unpin"},
		Required:   []string{"list_id"},
	},

	// ── Direct Messages ────────────────────────────────────────────
	{
		Name: "x_list_dm_events", Description: "List recent DM events for the authenticated user. Rate limit: 15 requests per 15 minutes",
		Parameters: map[string]string{
			"max_results":      "Results per page (1-100, default 100)",
			"dm_event_fields":  "Comma-separated: id,text,event_type,dm_conversation_id,created_at,sender_id",
			"pagination_token": "Pagination token",
		},
	},
	{
		Name: "x_get_dm_conversation", Description: "Get DM events with a specific user",
		Parameters: map[string]string{
			"participant_id":   "User ID of the other participant",
			"max_results":      "Results per page (1-100, default 100)",
			"dm_event_fields":  "Comma-separated DM event fields",
			"pagination_token": "Pagination token",
		},
		Required: []string{"participant_id"},
	},
	{
		Name: "x_send_dm", Description: "Send a direct message to a user in an existing conversation",
		Parameters: map[string]string{
			"participant_id": "User ID to send the DM to",
			"text":           "Message text",
		},
		Required: []string{"participant_id", "text"},
	},
	{
		Name: "x_create_dm_conversation", Description: "Create a new group DM conversation",
		Parameters: map[string]string{
			"participant_ids": "Comma-separated user IDs to include",
			"text":            "Initial message text",
		},
		Required: []string{"participant_ids", "text"},
	},

	// ── Spaces ─────────────────────────────────────────────────────
	{
		Name: "x_get_space", Description: "Get details about a X Space by ID",
		Parameters: map[string]string{
			"id":           "Space ID",
			"space_fields": "Comma-separated: id,title,state,host_ids,speaker_ids,participant_count,scheduled_start,created_at,lang",
		},
		Required: []string{"id"},
	},
	{
		Name: "x_search_spaces", Description: "Search for X Spaces by title",
		Parameters: map[string]string{
			"query":        "Search query for Space titles",
			"state":        "Filter by state: live, scheduled, or all (default all)",
			"max_results":  "Results per page (1-100, default 10)",
			"space_fields": "Comma-separated space fields",
		},
		Required: []string{"query"},
	},

	// ── Usage ──────────────────────────────────────────────────────
	{
		Name: "x_get_usage", Description: "Get API usage statistics for tweet endpoints. Track your consumption",
		Parameters: map[string]string{
			"days": "Number of days to look back (1-90, default 7)",
		},
	},
}
