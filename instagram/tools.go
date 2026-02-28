package instagram

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Profiles ────────────────────────────────────────────────────
	{
		Name:        "instagram_get_profile",
		Description: "Get the authenticated Instagram user's profile info (username, account type, media count, followers, bio)",
		Parameters: map[string]string{
			"user_id": "Instagram user ID (defaults to authenticated user)",
			"fields":  "Comma-separated fields to return (e.g. id,username,account_type,media_count,followers_count,follows_count,biography,website,profile_picture_url,name)",
		},
	},
	{
		Name:        "instagram_discover_user",
		Description: "Discover a business/creator Instagram account's public info by username using business discovery",
		Parameters: map[string]string{
			"username": "Instagram username to look up (without @)",
			"fields":   "Comma-separated fields (e.g. username,biography,media_count,followers_count,follows_count,website,profile_picture_url)",
		},
		Required: []string{"username"},
	},
	{
		Name:        "instagram_get_recently_searched",
		Description: "Get hashtags the authenticated user has searched in the past 7 days",
		Parameters: map[string]string{
			"user_id": "Instagram user ID (defaults to authenticated user)",
		},
	},

	// ── Media ───────────────────────────────────────────────────────
	{
		Name:        "instagram_list_media",
		Description: "List media (photos, videos, reels, carousels) for an Instagram user",
		Parameters: map[string]string{
			"user_id": "Instagram user ID (defaults to authenticated user)",
			"fields":  "Comma-separated fields (e.g. id,caption,media_type,media_url,permalink,timestamp,thumbnail_url,like_count,comments_count)",
			"limit":   "Maximum number of results to return",
			"after":   "Cursor for pagination (next page)",
			"before":  "Cursor for pagination (previous page)",
		},
	},
	{
		Name:        "instagram_get_media",
		Description: "Get a single Instagram media object by ID with metadata",
		Parameters: map[string]string{
			"media_id": "Instagram media ID",
			"fields":   "Comma-separated fields (e.g. id,caption,media_type,media_url,permalink,timestamp,thumbnail_url,like_count,comments_count,children)",
		},
		Required: []string{"media_id"},
	},
	{
		Name:        "instagram_list_stories",
		Description: "List active stories for an Instagram business/creator account",
		Parameters: map[string]string{
			"user_id": "Instagram user ID (defaults to authenticated user)",
		},
	},
	{
		Name:        "instagram_get_story",
		Description: "Get a single Instagram story object by ID",
		Parameters: map[string]string{
			"story_id": "Instagram story ID",
			"fields":   "Comma-separated fields (e.g. id,media_type,media_url,timestamp)",
		},
		Required: []string{"story_id"},
	},
	{
		Name:        "instagram_list_media_children",
		Description: "List children of a carousel (album) media object",
		Parameters: map[string]string{
			"media_id": "Instagram carousel media ID",
			"fields":   "Comma-separated fields for each child (e.g. id,media_type,media_url,timestamp)",
		},
		Required: []string{"media_id"},
	},

	// ── Messaging ───────────────────────────────────────────────────
	{
		Name:        "instagram_list_conversations",
		Description: "List recent Instagram DM conversations for the authenticated user",
		Parameters: map[string]string{
			"user_id":  "Instagram user ID (defaults to authenticated user)",
			"platform": "Filter by platform: instagram (default)",
			"folder":   "Conversation folder to list (e.g. inbox)",
		},
	},
	{
		Name:        "instagram_get_conversation",
		Description: "Get messages in a specific Instagram DM conversation",
		Parameters: map[string]string{
			"conversation_id": "Conversation ID",
			"fields":          "Comma-separated fields for messages (e.g. id,message,from,to,created_time)",
		},
		Required: []string{"conversation_id"},
	},
	{
		Name:        "instagram_send_message",
		Description: "Send a text direct message to an Instagram user via the messaging API. Requires instagram_manage_messages permission.",
		Parameters: map[string]string{
			"recipient_id": "Instagram-scoped ID of the recipient user",
			"message":      "Text message content to send",
		},
		Required: []string{"recipient_id", "message"},
	},
	{
		Name:        "instagram_send_media_message",
		Description: "Send a media attachment (image/video) as a direct message to an Instagram user",
		Parameters: map[string]string{
			"recipient_id": "Instagram-scoped ID of the recipient user",
			"media_url":    "URL of the image or video to send",
			"media_type":   "Type of media: image or video",
		},
		Required: []string{"recipient_id", "media_url", "media_type"},
	},

	// ── Comments ────────────────────────────────────────────────────
	{
		Name:        "instagram_list_comments",
		Description: "List comments on an Instagram media object",
		Parameters: map[string]string{
			"media_id": "Instagram media ID",
			"fields":   "Comma-separated fields (e.g. id,text,timestamp,username,like_count,replies)",
			"limit":    "Maximum number of results",
			"after":    "Cursor for pagination",
		},
		Required: []string{"media_id"},
	},
	{
		Name:        "instagram_get_comment",
		Description: "Get a single Instagram comment by ID",
		Parameters: map[string]string{
			"comment_id": "Instagram comment ID",
			"fields":     "Comma-separated fields (e.g. id,text,timestamp,username,like_count)",
		},
		Required: []string{"comment_id"},
	},
	{
		Name:        "instagram_reply_to_comment",
		Description: "Reply to a comment on an Instagram media object",
		Parameters: map[string]string{
			"media_id":   "Instagram media ID the comment is on",
			"comment_id": "Comment ID to reply to",
			"message":    "Reply text",
		},
		Required: []string{"media_id", "comment_id", "message"},
	},
	{
		Name:        "instagram_list_comment_replies",
		Description: "List replies to a specific Instagram comment",
		Parameters: map[string]string{
			"comment_id": "Instagram comment ID",
			"fields":     "Comma-separated fields (e.g. id,text,timestamp,username)",
			"limit":      "Maximum number of results",
		},
		Required: []string{"comment_id"},
	},
	{
		Name:        "instagram_hide_comment",
		Description: "Hide or unhide a comment on an Instagram media object",
		Parameters: map[string]string{
			"comment_id": "Instagram comment ID",
			"hide":       "Set to true to hide, false to unhide (default: true)",
		},
		Required: []string{"comment_id"},
	},
	{
		Name:        "instagram_delete_comment",
		Description: "Delete a comment on an Instagram media object",
		Parameters: map[string]string{
			"comment_id": "Instagram comment ID to delete",
		},
		Required: []string{"comment_id"},
	},
	{
		Name:        "instagram_get_mentioned_comment",
		Description: "Get a comment where the authenticated user was @mentioned",
		Parameters: map[string]string{
			"user_id":    "Instagram user ID (defaults to authenticated user)",
			"comment_id": "Comment ID where user was mentioned",
			"fields":     "Comma-separated fields (e.g. id,text,timestamp)",
		},
		Required: []string{"comment_id"},
	},
	{
		Name:        "instagram_get_mentioned_media",
		Description: "Get a media object where the authenticated user was @mentioned in the caption",
		Parameters: map[string]string{
			"user_id":  "Instagram user ID (defaults to authenticated user)",
			"media_id": "Media ID where user was mentioned",
			"fields":   "Comma-separated fields (e.g. id,caption,media_type,timestamp)",
		},
		Required: []string{"media_id"},
	},

	// ── Insights ────────────────────────────────────────────────────
	{
		Name:        "instagram_get_media_insights",
		Description: "Get insights (impressions, reach, engagement, saves) for a specific Instagram media object",
		Parameters: map[string]string{
			"media_id": "Instagram media ID",
			"metric":   "Comma-separated metrics (e.g. impressions,reach,engagement,saved,video_views,shares)",
		},
		Required: []string{"media_id", "metric"},
	},
	{
		Name:        "instagram_get_account_insights",
		Description: "Get account-level insights for the authenticated Instagram business/creator account",
		Parameters: map[string]string{
			"user_id": "Instagram user ID (defaults to authenticated user)",
			"metric":  "Comma-separated metrics (e.g. impressions,reach,accounts_engaged,total_interactions)",
			"period":  "Aggregation period: day, week, days_28, month, lifetime",
			"since":   "Unix timestamp for start of range",
			"until":   "Unix timestamp for end of range",
		},
		Required: []string{"metric", "period"},
	},

	// ── Hashtags ────────────────────────────────────────────────────
	{
		Name:        "instagram_search_hashtag",
		Description: "Search for a hashtag and get its ID. Limited to 30 unique hashtags per 7-day rolling window.",
		Parameters: map[string]string{
			"user_id": "Instagram user ID (defaults to authenticated user)",
			"q":       "Hashtag name to search for (without #)",
		},
		Required: []string{"q"},
	},
	{
		Name:        "instagram_get_hashtag_recent",
		Description: "Get recent media for a hashtag by hashtag ID",
		Parameters: map[string]string{
			"hashtag_id": "Hashtag ID (from instagram_search_hashtag)",
			"user_id":    "Instagram user ID (defaults to authenticated user)",
			"fields":     "Comma-separated fields (e.g. id,caption,media_type,permalink,timestamp)",
		},
		Required: []string{"hashtag_id"},
	},
	{
		Name:        "instagram_get_hashtag_top",
		Description: "Get top/trending media for a hashtag by hashtag ID",
		Parameters: map[string]string{
			"hashtag_id": "Hashtag ID (from instagram_search_hashtag)",
			"user_id":    "Instagram user ID (defaults to authenticated user)",
			"fields":     "Comma-separated fields (e.g. id,caption,media_type,permalink,timestamp)",
		},
		Required: []string{"hashtag_id"},
	},

	// ── Publishing ──────────────────────────────────────────────────
	{
		Name:        "instagram_create_media_container",
		Description: "Create a media container for publishing. Step 1 of the two-step publish flow. Supports IMAGE, VIDEO, REELS, CAROUSEL_ALBUM, and STORIES.",
		Parameters: map[string]string{
			"user_id":    "Instagram user ID (defaults to authenticated user)",
			"image_url":  "Public URL of the image (for IMAGE/STORIES)",
			"video_url":  "Public URL of the video (for VIDEO/REELS/STORIES)",
			"media_type": "Type: IMAGE, VIDEO, REELS, CAROUSEL_ALBUM, STORIES",
			"caption":    "Caption text for the post",
			"children":   "Comma-separated container IDs for CAROUSEL_ALBUM",
		},
	},
	{
		Name:        "instagram_publish_media",
		Description: "Publish a media container. Step 2 of the two-step publish flow. The container must have status FINISHED.",
		Parameters: map[string]string{
			"user_id":      "Instagram user ID (defaults to authenticated user)",
			"container_id": "Container ID from instagram_create_media_container",
		},
		Required: []string{"container_id"},
	},
	{
		Name:        "instagram_get_publish_status",
		Description: "Check the publishing status of a media container (IN_PROGRESS, FINISHED, ERROR)",
		Parameters: map[string]string{
			"container_id": "Container ID to check status of",
		},
		Required: []string{"container_id"},
	},
}
