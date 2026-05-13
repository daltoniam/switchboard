package x

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs, "fieldCompactionSpecs should not be empty")
}

func TestFieldCompactionSpecs_NoDuplicateTools(t *testing.T) {
	var sf compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &sf))
	assert.Equal(t, len(sf.Tools), len(fieldCompactionSpecs))
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for toolName := range fieldCompactionSpecs {
		_, ok := dispatch[toolName]
		assert.True(t, ok, "field compaction spec for %q has no dispatch handler", toolName)
	}
}

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	tw := &xClient{}
	fields, ok := tw.CompactSpec("x_search_recent")
	require.True(t, ok, "x_search_recent should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	tw := &xClient{}
	_, ok := tw.CompactSpec("x_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	tw := &xClient{}
	_, ok := tw.CompactSpec("x_create_tweet")
	assert.False(t, ok, "mutation tools should not have compaction specs")
}

func TestFieldCompactionSpec_NoMissingReadTools(t *testing.T) {
	readTools := map[mcp.ToolName]bool{
		"x_get_tweet":            true,
		"x_get_tweets":           true,
		"x_search_recent":        true,
		"x_search_all":           true,
		"x_get_tweet_count":      true,
		"x_get_quote_tweets":     true,
		"x_get_user_tweets":      true,
		"x_get_user_mentions":    true,
		"x_get_home_timeline":    true,
		"x_get_user":             true,
		"x_get_user_by_username": true,
		"x_get_users":            true,
		"x_search_users":         true,
		"x_get_me":               true,
		"x_get_following":        true,
		"x_get_followers":        true,
		"x_get_blocked":          true,
		"x_get_muted":            true,
		"x_get_liking_users":     true,
		"x_get_liked_tweets":     true,
		"x_get_retweeters":       true,
		"x_get_bookmarks":        true,
		"x_get_list":             true,
		"x_get_owned_lists":      true,
		"x_get_list_tweets":      true,
		"x_get_list_members":     true,
		"x_get_list_followers":   true,
		"x_get_pinned_lists":     true,
		"x_list_dm_events":       true,
		"x_get_dm_conversation":  true,
		"x_get_space":            true,
		"x_search_spaces":        true,
		"x_get_usage":            true,
	}

	for toolName := range readTools {
		_, ok := fieldCompactionSpecs[toolName]
		assert.True(t, ok, "read tool %q should have a compaction spec", toolName)
	}
}
