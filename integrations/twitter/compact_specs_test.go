package twitter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs, "fieldCompactionSpecs should not be empty")
}

func TestFieldCompactionSpecs_NoDuplicateTools(t *testing.T) {
	assert.Equal(t, len(rawFieldCompactionSpecs), len(fieldCompactionSpecs))
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for toolName := range fieldCompactionSpecs {
		_, ok := dispatch[toolName]
		assert.True(t, ok, "field compaction spec for %q has no dispatch handler", toolName)
	}
}

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	tw := &twitter{}
	fields, ok := tw.CompactSpec("twitter_search_recent")
	require.True(t, ok, "twitter_search_recent should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	tw := &twitter{}
	_, ok := tw.CompactSpec("twitter_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	tw := &twitter{}
	_, ok := tw.CompactSpec("twitter_create_tweet")
	assert.False(t, ok, "mutation tools should not have compaction specs")
}

func TestFieldCompactionSpec_NoMissingReadTools(t *testing.T) {
	readTools := map[string]bool{
		"twitter_get_tweet":            true,
		"twitter_get_tweets":           true,
		"twitter_search_recent":        true,
		"twitter_search_all":           true,
		"twitter_get_tweet_count":      true,
		"twitter_get_quote_tweets":     true,
		"twitter_get_user_tweets":      true,
		"twitter_get_user_mentions":    true,
		"twitter_get_home_timeline":    true,
		"twitter_get_user":             true,
		"twitter_get_user_by_username": true,
		"twitter_get_users":            true,
		"twitter_search_users":         true,
		"twitter_get_me":               true,
		"twitter_get_following":        true,
		"twitter_get_followers":        true,
		"twitter_get_blocked":          true,
		"twitter_get_muted":            true,
		"twitter_get_liking_users":     true,
		"twitter_get_liked_tweets":     true,
		"twitter_get_retweeters":       true,
		"twitter_get_bookmarks":        true,
		"twitter_get_list":             true,
		"twitter_get_owned_lists":      true,
		"twitter_get_list_tweets":      true,
		"twitter_get_list_members":     true,
		"twitter_get_list_followers":   true,
		"twitter_get_pinned_lists":     true,
		"twitter_list_dm_events":       true,
		"twitter_get_dm_conversation":  true,
		"twitter_get_space":            true,
		"twitter_search_spaces":        true,
		"twitter_get_usage":            true,
	}

	for toolName := range readTools {
		_, ok := fieldCompactionSpecs[toolName]
		assert.True(t, ok, "read tool %q should have a compaction spec", toolName)
	}
}
