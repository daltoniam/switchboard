package twitter

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	"twitter_get_tweet": {
		"data.id", "data.text", "data.author_id", "data.created_at",
		"data.public_metrics", "data.conversation_id", "data.lang",
		"data.entities.urls[].expanded_url", "data.entities.mentions[].username",
		"data.referenced_tweets[].type", "data.referenced_tweets[].id",
	},
	"twitter_get_tweets": {
		"data[].id", "data[].text", "data[].author_id", "data[].created_at",
		"data[].public_metrics", "data[].conversation_id", "data[].lang",
	},
	"twitter_search_recent": {
		"data[].id", "data[].text", "data[].author_id", "data[].created_at",
		"data[].public_metrics", "data[].conversation_id", "data[].source",
		"meta.next_token", "meta.result_count",
	},
	"twitter_search_all": {
		"data[].id", "data[].text", "data[].author_id", "data[].created_at",
		"data[].public_metrics", "data[].conversation_id", "data[].source",
		"meta.next_token", "meta.result_count",
	},
	"twitter_get_tweet_count": {
		"data[].start", "data[].end", "data[].tweet_count",
		"meta.total_tweet_count",
	},
	"twitter_get_quote_tweets": {
		"data[].id", "data[].text", "data[].author_id", "data[].created_at",
		"data[].public_metrics",
		"meta.next_token", "meta.result_count",
	},
	"twitter_get_user_tweets": {
		"data[].id", "data[].text", "data[].author_id", "data[].created_at",
		"data[].public_metrics", "data[].conversation_id",
		"meta.next_token", "meta.result_count",
	},
	"twitter_get_user_mentions": {
		"data[].id", "data[].text", "data[].author_id", "data[].created_at",
		"data[].public_metrics", "data[].conversation_id",
		"meta.next_token", "meta.result_count",
	},
	"twitter_get_home_timeline": {
		"data[].id", "data[].text", "data[].author_id", "data[].created_at",
		"data[].public_metrics", "data[].conversation_id",
		"meta.next_token", "meta.result_count",
	},
	"twitter_get_user": {
		"data.id", "data.name", "data.username", "data.description",
		"data.created_at", "data.location", "data.public_metrics",
		"data.verified", "data.protected", "data.url",
		"data.pinned_tweet_id", "data.profile_image_url",
	},
	"twitter_get_user_by_username": {
		"data.id", "data.name", "data.username", "data.description",
		"data.created_at", "data.location", "data.public_metrics",
		"data.verified", "data.protected", "data.url",
		"data.pinned_tweet_id", "data.profile_image_url",
	},
	"twitter_get_users": {
		"data[].id", "data[].name", "data[].username", "data[].description",
		"data[].public_metrics", "data[].verified", "data[].protected",
	},
	"twitter_search_users": {
		"data[].id", "data[].name", "data[].username", "data[].description",
		"data[].public_metrics", "data[].verified",
	},
	"twitter_get_me": {
		"data.id", "data.name", "data.username", "data.description",
		"data.created_at", "data.location", "data.public_metrics",
		"data.verified", "data.protected", "data.url",
		"data.pinned_tweet_id", "data.profile_image_url",
	},
	"twitter_get_following": {
		"data[].id", "data[].name", "data[].username", "data[].description",
		"data[].public_metrics", "data[].verified",
		"meta.next_token", "meta.result_count",
	},
	"twitter_get_followers": {
		"data[].id", "data[].name", "data[].username", "data[].description",
		"data[].public_metrics", "data[].verified",
		"meta.next_token", "meta.result_count",
	},
	"twitter_get_blocked": {
		"data[].id", "data[].name", "data[].username",
		"meta.next_token", "meta.result_count",
	},
	"twitter_get_muted": {
		"data[].id", "data[].name", "data[].username",
		"meta.next_token", "meta.result_count",
	},
	"twitter_get_liking_users": {
		"data[].id", "data[].name", "data[].username",
		"meta.next_token", "meta.result_count",
	},
	"twitter_get_liked_tweets": {
		"data[].id", "data[].text", "data[].author_id", "data[].created_at",
		"data[].public_metrics",
		"meta.next_token", "meta.result_count",
	},
	"twitter_get_retweeters": {
		"data[].id", "data[].name", "data[].username",
		"meta.next_token", "meta.result_count",
	},
	"twitter_get_bookmarks": {
		"data[].id", "data[].text", "data[].author_id", "data[].created_at",
		"data[].public_metrics",
		"meta.next_token", "meta.result_count",
	},
	"twitter_get_list": {
		"data.id", "data.name", "data.description", "data.private",
		"data.follower_count", "data.member_count", "data.owner_id",
		"data.created_at",
	},
	"twitter_get_owned_lists": {
		"data[].id", "data[].name", "data[].description", "data[].private",
		"data[].follower_count", "data[].member_count", "data[].owner_id",
		"meta.next_token", "meta.result_count",
	},
	"twitter_get_list_tweets": {
		"data[].id", "data[].text", "data[].author_id", "data[].created_at",
		"data[].public_metrics",
		"meta.next_token", "meta.result_count",
	},
	"twitter_get_list_members": {
		"data[].id", "data[].name", "data[].username",
		"data[].public_metrics", "data[].verified",
		"meta.next_token", "meta.result_count",
	},
	"twitter_get_list_followers": {
		"data[].id", "data[].name", "data[].username",
		"meta.next_token", "meta.result_count",
	},
	"twitter_get_pinned_lists": {
		"data[].id", "data[].name", "data[].description", "data[].private",
	},
	"twitter_list_dm_events": {
		"data[].id", "data[].text", "data[].event_type",
		"data[].dm_conversation_id", "data[].created_at", "data[].sender_id",
		"meta.next_token", "meta.result_count",
	},
	"twitter_get_dm_conversation": {
		"data[].id", "data[].text", "data[].event_type",
		"data[].dm_conversation_id", "data[].created_at", "data[].sender_id",
		"meta.next_token", "meta.result_count",
	},
	"twitter_get_space": {
		"data.id", "data.title", "data.state", "data.host_ids",
		"data.speaker_ids", "data.participant_count", "data.scheduled_start",
		"data.created_at", "data.lang",
	},
	"twitter_search_spaces": {
		"data[].id", "data[].title", "data[].state",
		"data[].participant_count", "data[].scheduled_start",
		"meta.result_count",
	},
	"twitter_get_usage": {
		"data[].date", "data[].usage[].tweets",
	},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("twitter: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
