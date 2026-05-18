package x

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Tweets ─────────────────────────────────────────────────────
	{
		Name: "x_get_tweet", Description: "Get a single tweet by ID. Returns full tweet object with text, author, metrics, and entities",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Tweet ID", Required: true}, {Name: mcp.ParamName("tweet_fields"), Description: "Comma-separated tweet fields: attachments,author_id,created_at,entities,geo,id,lang,public_metrics,source,text,conversation_id,reply_settings"}, {Name: mcp.ParamName("expansions"), Description: "Comma-separated expansions: author_id,referenced_tweets.id,attachments.media_keys"}, {Name: mcp.ParamName("user_fields"), Description: "Comma-separated user fields: id,name,username,profile_image_url,verified,public_metrics"}},
	},
	{
		Name: "x_get_tweets", Description: "Get multiple tweets by IDs (up to 100). Start here for bulk tweet lookups",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("ids"), Description: "Comma-separated tweet IDs (max 100)", Required: true}, {Name: mcp.ParamName("tweet_fields"), Description: "Comma-separated tweet fields: attachments,author_id,created_at,entities,geo,id,lang,public_metrics,source,text"}, {Name: mcp.ParamName("expansions"), Description: "Comma-separated expansions: author_id,referenced_tweets.id,attachments.media_keys"}, {Name: mcp.ParamName("user_fields"), Description: "Comma-separated user fields: id,name,username,profile_image_url,verified,public_metrics"}},
	},
	{
		Name: "x_create_tweet", Description: "Post a new tweet. Supports text, replies, quotes, and polls",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("text"), Description: "Tweet text (up to 280 chars)", Required: true}, {Name: mcp.ParamName("reply_to"), Description: "Tweet ID to reply to"}, {Name: mcp.ParamName("quote_tweet_id"), Description: "Tweet ID to quote"}, {Name: mcp.ParamName("poll_options"), Description: "Comma-separated poll options (2-4 choices)"}, {Name: mcp.ParamName("poll_duration_minutes"), Description: "Poll duration in minutes (5-10080)"}},
	},
	{
		Name: "x_delete_tweet", Description: "Delete a tweet by ID. Must be authored by the authenticated user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Tweet ID to delete", Required: true}},
	},
	{
		Name: "x_search_recent", Description: "Search tweets from the last 7 days. Start here for most tweet discovery workflows. Supports the full X search query syntax",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query (required). Supports operators: from:, to:, is:retweet, has:media, has:links, lang:, -is:retweet, etc.", Required: true}, {Name: mcp.ParamName("max_results"), Description: "Results per page (10-100, default 10)"}, {Name: mcp.ParamName("next_token"), Description: "Pagination token from previous response"}, {Name: mcp.ParamName("tweet_fields"), Description: "Comma-separated tweet fields: author_id,created_at,public_metrics,source,text,conversation_id,entities"}, {Name: mcp.ParamName("start_time"), Description: "Oldest UTC datetime (ISO 8601: 2024-01-01T00:00:00Z)"}, {Name: mcp.ParamName("end_time"), Description: "Newest UTC datetime (ISO 8601)"}, {Name: mcp.ParamName("sort_order"), Description: "Order of results: recency or relevancy"}},
	},
	{
		Name: "x_search_all", Description: "Full-archive search across all tweets (requires Pro tier or higher). Use x_search_recent for 7-day window instead",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query (required). Same operators as x_search_recent", Required: true}, {Name: mcp.ParamName("max_results"), Description: "Results per page (10-500, default 10)"}, {Name: mcp.ParamName("next_token"), Description: "Pagination token from previous response"}, {Name: mcp.ParamName("tweet_fields"), Description: "Comma-separated tweet fields"}, {Name: mcp.ParamName("start_time"), Description: "Oldest UTC datetime (ISO 8601)"}, {Name: mcp.ParamName("end_time"), Description: "Newest UTC datetime (ISO 8601)"}, {Name: mcp.ParamName("sort_order"), Description: "Order of results: recency or relevancy"}},
	},
	{
		Name: "x_get_tweet_count", Description: "Get count of tweets matching a search query from the last 7 days. Useful for analytics without retrieving full tweets",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query", Required: true}, {Name: mcp.ParamName("granularity"), Description: "Aggregation granularity: minute, hour, or day (default day)"}, {Name: mcp.ParamName("start_time"), Description: "Oldest UTC datetime (ISO 8601)"}, {Name: mcp.ParamName("end_time"), Description: "Newest UTC datetime (ISO 8601)"}},
	},
	{
		Name: "x_get_quote_tweets", Description: "Get tweets that quote a specific tweet",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Tweet ID to get quotes for", Required: true}, {Name: mcp.ParamName("max_results"), Description: "Results per page (10-100, default 10)"}, {Name: mcp.ParamName("tweet_fields"), Description: "Comma-separated tweet fields"}, {Name: mcp.ParamName("pagination_token"), Description: "Pagination token"}},
	},
	{
		Name: "x_hide_reply", Description: "Hide a reply to one of your tweets",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Reply tweet ID to hide", Required: true}},
	},
	{
		Name: "x_unhide_reply", Description: "Unhide a previously hidden reply",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Reply tweet ID to unhide", Required: true}},
	},

	// ── Timelines ──────────────────────────────────────────────────
	{
		Name: "x_get_user_tweets", Description: "Get tweets posted by a specific user. Use user_id from x_get_user or x_get_user_by_username",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (not username — use x_get_user_by_username to resolve)", Required: true}, {Name: mcp.ParamName("max_results"), Description: "Results per page (5-100, default 10)"}, {Name: mcp.ParamName("tweet_fields"), Description: "Comma-separated tweet fields"}, {Name: mcp.ParamName("pagination_token"), Description: "Pagination token"}, {Name: mcp.ParamName("start_time"), Description: "Oldest UTC datetime (ISO 8601)"}, {Name: mcp.ParamName("end_time"), Description: "Newest UTC datetime (ISO 8601)"}, {Name: mcp.ParamName("exclude"), Description: "Comma-separated: retweets,replies"}},
	},
	{
		Name: "x_get_user_mentions", Description: "Get tweets mentioning a specific user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID", Required: true}, {Name: mcp.ParamName("max_results"), Description: "Results per page (5-100, default 10)"}, {Name: mcp.ParamName("tweet_fields"), Description: "Comma-separated tweet fields"}, {Name: mcp.ParamName("pagination_token"), Description: "Pagination token"}, {Name: mcp.ParamName("start_time"), Description: "Oldest UTC datetime (ISO 8601)"}, {Name: mcp.ParamName("end_time"), Description: "Newest UTC datetime (ISO 8601)"}},
	},
	{
		Name: "x_get_home_timeline", Description: "Get the authenticated user's home timeline (reverse chronological). Requires user context auth",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("max_results"), Description: "Results per page (1-100, default 10)"}, {Name: mcp.ParamName("tweet_fields"), Description: "Comma-separated tweet fields"}, {Name: mcp.ParamName("pagination_token"), Description: "Pagination token"}, {Name: mcp.ParamName("start_time"), Description: "Oldest UTC datetime (ISO 8601)"}, {Name: mcp.ParamName("end_time"),

		// ── Users ──────────────────────────────────────────────────────
		Description: "Newest UTC datetime (ISO 8601)"}, {Name: mcp.ParamName("exclude"), Description: "Comma-separated: retweets,replies"}},
	},

	{
		Name: "x_get_user", Description: "Get a user by their numeric ID. Use x_get_user_by_username if you have a @handle instead",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "User ID", Required: true}, {Name: mcp.ParamName("user_fields"), Description: "Comma-separated: id,name,username,created_at,description,location,pinned_tweet_id,profile_image_url,protected,public_metrics,url,verified"}},
	},
	{
		Name: "x_get_user_by_username", Description: "Get a user by @username. Preferred entry point when you know the handle. Returns user ID needed for other endpoints",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("username"), Description: "Username without @ prefix", Required: true}, {Name: mcp.ParamName("user_fields"), Description: "Comma-separated user fields"}},
	},
	{
		Name: "x_get_users", Description: "Get multiple users by IDs (up to 100)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("ids"), Description: "Comma-separated user IDs (max 100)", Required: true}, {Name: mcp.ParamName("user_fields"), Description: "Comma-separated user fields"}},
	},
	{
		Name: "x_search_users", Description: "Search for users by name or username",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query", Required: true}, {Name: mcp.ParamName("max_results"), Description: "Results per page (1-100, default 10)"}, {Name: mcp.ParamName("user_fields"), Description: "Comma-separated user fields"}},
	},
	{
		Name: "x_get_me", Description: "Get the authenticated user's profile information",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_fields"), Description: "Comma-separated user fields"}},
	},

	// ── Follows ────────────────────────────────────────────────────
	{
		Name: "x_get_following", Description: "Get users that a user follows",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID", Required: true}, {Name: mcp.ParamName("max_results"), Description: "Results per page (1-1000, default 100)"}, {Name: mcp.ParamName("user_fields"), Description: "Comma-separated user fields"}, {Name: mcp.ParamName("pagination_token"), Description: "Pagination token"}},
	},
	{
		Name: "x_get_followers", Description: "Get users who follow a user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID", Required: true}, {Name: mcp.ParamName("max_results"), Description: "Results per page (1-1000, default 100)"}, {Name: mcp.ParamName("user_fields"), Description: "Comma-separated user fields"}, {Name: mcp.ParamName("pagination_token"), Description: "Pagination token"}},
	},
	{
		Name: "x_follow_user", Description: "Follow a user. Uses the authenticated user's ID automatically",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("target_user_id"), Description: "User ID to follow", Required: true}},
	},
	{
		Name: "x_unfollow_user", Description: "Unfollow a user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("target_user_id"), Description: "User ID to unfollow", Required: true}},
	},

	// ── Blocks ─────────────────────────────────────────────────────
	{
		Name: "x_get_blocked", Description: "Get users blocked by the authenticated user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("max_results"), Description: "Results per page (1-1000, default 100)"}, {Name: mcp.ParamName("user_fields"), Description: "Comma-separated user fields"}, {Name: mcp.ParamName("pagination_token"), Description: "Pagination token"}},
	},
	{
		Name: "x_block_user", Description: "Block a user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("target_user_id"), Description: "User ID to block", Required: true}},
	},
	{
		Name: "x_unblock_user", Description: "Unblock a user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("target_user_id"), Description: "User ID to unblock", Required: true}},
	},

	// ── Mutes ──────────────────────────────────────────────────────
	{
		Name: "x_get_muted", Description: "Get users muted by the authenticated user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("max_results"), Description: "Results per page (1-1000, default 100)"}, {Name: mcp.ParamName("user_fields"), Description: "Comma-separated user fields"}, {Name: mcp.ParamName("pagination_token"), Description: "Pagination token"}},
	},
	{
		Name: "x_mute_user", Description: "Mute a user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("target_user_id"), Description: "User ID to mute", Required: true}},
	},
	{
		Name: "x_unmute_user", Description: "Unmute a user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("target_user_id"), Description: "User ID to unmute", Required: true}},
	},

	// ── Likes ──────────────────────────────────────────────────────
	{
		Name: "x_get_liking_users", Description: "Get users who liked a specific tweet",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("tweet_id"), Description: "Tweet ID", Required: true}, {Name: mcp.ParamName("max_results"), Description: "Results per page (1-100, default 100)"}, {Name: mcp.ParamName("user_fields"), Description: "Comma-separated user fields"}, {Name: mcp.ParamName("pagination_token"), Description: "Pagination token"}},
	},
	{
		Name: "x_get_liked_tweets", Description: "Get tweets liked by a specific user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID", Required: true}, {Name: mcp.ParamName("max_results"), Description: "Results per page (10-100, default 10)"}, {Name: mcp.ParamName("tweet_fields"), Description: "Comma-separated tweet fields"}, {Name: mcp.ParamName("pagination_token"), Description: "Pagination token"}},
	},
	{
		Name: "x_like_tweet", Description: "Like a tweet",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("tweet_id"), Description: "Tweet ID to like", Required: true}},
	},
	{
		Name: "x_unlike_tweet", Description: "Unlike a previously liked tweet",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("tweet_id"), Description: "Tweet ID to unlike", Required: true}},
	},

	// ── Retweets ───────────────────────────────────────────────────
	{
		Name: "x_get_retweeters", Description: "Get users who retweeted a specific tweet",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("tweet_id"), Description: "Tweet ID", Required: true}, {Name: mcp.ParamName("max_results"), Description: "Results per page (1-100, default 100)"}, {Name: mcp.ParamName("user_fields"), Description: "Comma-separated user fields"}, {Name: mcp.ParamName("pagination_token"), Description: "Pagination token"}},
	},
	{
		Name: "x_retweet", Description: "Retweet a tweet",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("tweet_id"), Description: "Tweet ID to retweet", Required: true}},
	},
	{
		Name: "x_unretweet", Description: "Remove a retweet",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("tweet_id"), Description: "Tweet ID to unretweet", Required: true}},
	},

	// ── Bookmarks ──────────────────────────────────────────────────
	{
		Name: "x_get_bookmarks", Description: "Get tweets bookmarked by the authenticated user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("max_results"), Description: "Results per page (1-100, default 10)"}, {Name: mcp.ParamName("tweet_fields"), Description: "Comma-separated tweet fields"}, {Name: mcp.ParamName("pagination_token"), Description: "Pagination token"}},
	},
	{
		Name: "x_bookmark_tweet", Description: "Bookmark a tweet",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("tweet_id"), Description: "Tweet ID to bookmark", Required: true}},
	},
	{
		Name: "x_remove_bookmark", Description: "Remove a tweet from bookmarks",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("tweet_id"), Description: "Tweet ID to remove from bookmarks", Required: true}},
	},

	// ── Lists ──────────────────────────────────────────────────────
	{
		Name: "x_get_list", Description: "Get a list by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "List ID", Required: true}, {Name: mcp.ParamName("list_fields"), Description: "Comma-separated: id,name,description,private,follower_count,member_count,owner_id,created_at"}},
	},
	{
		Name: "x_get_owned_lists", Description: "Get lists owned by a user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID", Required: true}, {Name: mcp.ParamName("max_results"), Description: "Results per page (1-100, default 100)"}, {Name: mcp.ParamName("list_fields"), Description: "Comma-separated list fields"}, {Name: mcp.ParamName("pagination_token"), Description: "Pagination token"}},
	},
	{
		Name: "x_create_list", Description: "Create a new list",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "List name (1-25 chars)", Required: true}, {Name: mcp.ParamName("description"), Description: "List description"}, {Name: mcp.ParamName("private"), Description: "Whether list is private (true/false, default false)"}},
	},
	{
		Name: "x_update_list", Description: "Update a list's name, description, or privacy setting",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "List ID", Required: true}, {Name: mcp.ParamName("name"), Description: "New list name"}, {Name: mcp.ParamName("description"), Description: "New list description"}, {Name: mcp.ParamName("private"), Description: "Whether list is private (true/false)"}},
	},
	{
		Name: "x_delete_list", Description: "Delete a list. Must be owned by the authenticated user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "List ID to delete", Required: true}},
	},
	{
		Name: "x_get_list_tweets", Description: "Get tweets from a list's timeline",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "List ID", Required: true}, {Name: mcp.ParamName("max_results"), Description: "Results per page (1-100, default 100)"}, {Name: mcp.ParamName("tweet_fields"), Description: "Comma-separated tweet fields"}, {Name: mcp.ParamName("pagination_token"), Description: "Pagination token"}},
	},
	{
		Name: "x_get_list_members", Description: "Get members of a list",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "List ID", Required: true}, {Name: mcp.ParamName("max_results"), Description: "Results per page (1-100, default 100)"}, {Name: mcp.ParamName("user_fields"), Description: "Comma-separated user fields"}, {Name: mcp.ParamName("pagination_token"), Description: "Pagination token"}},
	},
	{
		Name: "x_add_list_member", Description: "Add a user to a list. Must own the list",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "List ID", Required: true}, {Name: mcp.ParamName("user_id"), Description: "User ID to add", Required: true}},
	},
	{
		Name: "x_remove_list_member", Description: "Remove a user from a list",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "List ID", Required: true}, {Name: mcp.ParamName("user_id"), Description: "User ID to remove", Required: true}},
	},
	{
		Name: "x_get_list_followers", Description: "Get users who follow a list",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "List ID", Required: true}, {Name: mcp.ParamName("max_results"), Description: "Results per page (1-100, default 100)"}, {Name: mcp.ParamName("user_fields"), Description: "Comma-separated user fields"}, {Name: mcp.ParamName("pagination_token"), Description: "Pagination token"}},
	},
	{
		Name: "x_follow_list", Description: "Follow a list",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("list_id"), Description: "List ID to follow", Required: true}},
	},
	{
		Name: "x_unfollow_list", Description: "Unfollow a list",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("list_id"), Description: "List ID to unfollow", Required: true}},
	},
	{
		Name: "x_get_pinned_lists", Description: "Get lists pinned by the authenticated user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("list_fields"), Description: "Comma-separated list fields"}},
	},
	{
		Name: "x_pin_list", Description: "Pin a list",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("list_id"), Description: "List ID to pin", Required: true}},
	},
	{
		Name: "x_unpin_list", Description: "Unpin a list",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("list_id"), Description: "List ID to unpin", Required: true}},
	},

	// ── Direct Messages ────────────────────────────────────────────
	{
		Name: "x_list_dm_events", Description: "List recent DM events for the authenticated user. Rate limit: 15 requests per 15 minutes",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("max_results"), Description: "Results per page (1-100, default 100)"}, {Name: mcp.ParamName("dm_event_fields"), Description: "Comma-separated: id,text,event_type,dm_conversation_id,created_at,sender_id"}, {Name: mcp.ParamName("pagination_token"), Description: "Pagination token"}},
	},
	{
		Name: "x_get_dm_conversation", Description: "Get DM events with a specific user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("participant_id"), Description: "User ID of the other participant", Required: true}, {Name: mcp.ParamName("max_results"), Description: "Results per page (1-100, default 100)"}, {Name: mcp.ParamName("dm_event_fields"), Description: "Comma-separated DM event fields"}, {Name: mcp.ParamName("pagination_token"), Description: "Pagination token"}},
	},
	{
		Name: "x_send_dm", Description: "Send a direct message to a user in an existing conversation",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("participant_id"), Description: "User ID to send the DM to", Required: true}, {Name: mcp.ParamName("text"), Description: "Message text", Required: true}},
	},
	{
		Name: "x_create_dm_conversation", Description: "Create a new group DM conversation",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("participant_ids"), Description: "Comma-separated user IDs to include", Required: true}, {Name: mcp.ParamName("text"), Description: "Initial message text", Required:

		// ── Spaces ─────────────────────────────────────────────────────
		true}},
	},

	{
		Name: "x_get_space", Description: "Get details about a X Space by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Space ID", Required: true}, {Name: mcp.ParamName("space_fields"), Description: "Comma-separated: id,title,state,host_ids,speaker_ids,participant_count,scheduled_start,created_at,lang"}},
	},
	{
		Name: "x_search_spaces", Description: "Search for X Spaces by title",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query for Space titles", Required: true}, {Name: mcp.ParamName("state"), Description: "Filter by state: live, scheduled, or all (default all)"}, {Name: mcp.ParamName("max_results"), Description: "Results per page (1-100, default 10)"}, {Name: mcp.ParamName(

		// ── Usage ──────────────────────────────────────────────────────
		"space_fields"), Description: "Comma-separated space fields"}},
	},

	{
		Name: "x_get_usage", Description: "Get API usage statistics for tweet endpoints. Track your consumption",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("days"), Description: "Number of days to look back (1-90, default 7)"}},
	},
}
