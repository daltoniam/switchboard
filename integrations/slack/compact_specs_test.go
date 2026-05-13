package slack

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compactyaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs, "fieldCompactionSpecs should not be empty")
}

func TestFieldCompactionSpecs_NoDuplicateTools(t *testing.T) {
	var sf compactyaml.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &sf))
	assert.Equal(t, len(sf.Tools), len(fieldCompactionSpecs))
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for toolName := range fieldCompactionSpecs {
		_, ok := dispatch[toolName]
		assert.True(t, ok, "field compaction spec for %q has no dispatch handler", toolName)
	}
}

func TestFieldCompactionSpecs_OnlyReadTools(t *testing.T) {
	mutationPrefixes := []string{"create", "archive", "set", "send", "update", "delete", "add", "remove", "schedule"}
	for toolName := range fieldCompactionSpecs {
		for _, prefix := range mutationPrefixes {
			assert.NotContains(t, toolName, "_"+prefix+"_",
				"mutation tool %q should not have a field compaction spec", toolName)
		}
	}
}

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	s := &slackIntegration{}
	fields, ok := s.CompactSpec("slack_token_status")
	require.True(t, ok, "slack_token_status should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	s := &slackIntegration{}
	_, ok := s.CompactSpec("slack_send_message")
	assert.False(t, ok, "mutation tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	s := &slackIntegration{}
	_, ok := s.CompactSpec("slack_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

// TestFieldCompactionSpecs_ShapeParity verifies that compaction specs match
// the actual handler output structure. A spec that targets fields not in the
// handler output produces {} — this test catches that.
func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	// Representative handler output shapes, one per structural pattern.
	handlerOutputs := map[string]string{
		// Object-wrapped list: {count, items[], ...}
		"slack_list_conversations": `{"count":2,"conversations":[{"id":"C1","name":"general","type":"public_channel","num_members":10,"topic":"","purpose":"","is_archived":false}],"next_cursor":""}`,
		// Flat object (single-record get)
		"slack_get_conversation_info": `{"id":"C1","name":"general","type":"public_channel","num_members":10,"topic":"General","purpose":"General talk","is_archived":false,"creator":"U1","created":1234567890}`,
		// Nested messages array
		"slack_conversations_history": `{"channel":"C1","count":1,"has_more":false,"messages":[{"ts":"1234.5678","user":"U1","text":"hello","thread_ts":"","reply_count":0}],"next_cursor":""}`,
		// Thread with is_parent field
		"slack_get_thread": `{"channel":"C1","thread_ts":"1234.5678","count":2,"messages":[{"ts":"1234.5678","user":"U1","text":"hello","is_parent":true}]}`,
		// Search: flat matches array
		"slack_search_messages": `{"query":"test","total":1,"matches":[{"ts":"1234.5678","text":"found it","user":"U1","channel":"general","channel_id":"C1","permalink":"https://slack.com/archives/C1/p1234"}]}`,
		// Bare array (reactions)
		"slack_get_reactions": `[{"name":"thumbsup","count":3,"users":["U1","U2"]}]`,
		// Pins with nested message
		"slack_list_pins": `{"count":1,"pins":[{"type":"message","channel":"C1","message":{"ts":"1234.5678","text":"pinned","user":"U1"},"created":1234567890}]}`,
		// Users list
		"slack_list_users": `{"count":1,"users":[{"id":"U1","name":"alice","real_name":"Alice","display_name":"alice","email":"a@b.com","is_admin":false,"is_bot":false,"deleted":false,"timezone":"America/New_York"}]}`,
		// User info (flat)
		"slack_get_user_info": `{"id":"U1","name":"alice","real_name":"Alice","display_name":"alice","email":"a@b.com","status_text":"","status_emoji":"","timezone":"America/New_York","is_admin":false,"is_bot":false}`,
		// User presence
		"slack_get_user_presence": `{"user_id":"U1","presence":"active","online":true}`,
		// User groups
		"slack_list_user_groups": `{"count":1,"user_groups":[{"id":"G1","name":"Engineers","handle":"engineers","description":"Engineering team","user_count":5}]}`,
		// Get user group
		"slack_get_user_group": `{"usergroup_id":"G1","count":3,"members":["U1","U2","U3"]}`,
		// Auth test
		"slack_auth_test": `{"user":"alice","user_id":"U1","team":"Acme","team_id":"T1","url":"https://acme.slack.com"}`,
		// Team info (flat, no team wrapper)
		"slack_team_info": `{"id":"T1","name":"Acme","domain":"acme","icon":"https://example.com/icon.png"}`,
		// Files
		"slack_list_files": `{"count":1,"files":[{"id":"F1","name":"doc.pdf","title":"Document","filetype":"pdf","size":1024,"user":"U1"}]}`,
		// Emoji
		"slack_list_emoji": `{"count":5,"emoji":{"tada":"https://emoji.slack-edge.com/tada.png"}}`,
		// Bookmarks
		"slack_list_bookmarks": `{"count":1,"bookmarks":[{"id":"B1","title":"Wiki","link":"https://wiki.com","emoji":":book:","type":"link"}]}`,
		// Reminders
		"slack_list_reminders": `{"count":1,"reminders":[{"id":"R1","text":"standup","user":"U1","time":1234567890}]}`,
		// Token status (multi-workspace)
		"slack_token_status": `{"workspace_count":1,"default_team_id":"T1","workspaces":[{"team_id":"T1","team_name":"Acme","status":"healthy","token_type":"browser_session","age_hours":2.5,"source":"config","is_default":true}],"auto_refresh":{"enabled":true,"interval":"4 hours"}}`,
		// List workspaces
		"slack_list_workspaces": `{"count":2,"default_team_id":"T1","workspaces":[{"team_id":"T1","team_name":"Acme","is_default":true},{"team_id":"T2","team_name":"Beta","is_default":false}]}`,
	}

	for toolName, jsonPayload := range handlerOutputs {
		t.Run(toolName, func(t *testing.T) {
			fields, ok := fieldCompactionSpecs[mcp.ToolName(toolName)]
			require.True(t, ok, "spec must exist for %s", toolName)

			compacted, err := mcp.CompactJSON([]byte(jsonPayload), fields)
			require.NoError(t, err, "compaction should not error for %s", toolName)

			// Most tools return objects; slack_get_reactions returns a bare array.
			// Both must produce non-empty compacted output.
			assert.NotEqual(t, "{}", string(compacted), "compacted %s should not be empty object", toolName)
			assert.NotEqual(t, "[]", string(compacted), "compacted %s should not be empty array", toolName)
			assert.NotEqual(t, "[{}]", string(compacted), "compacted %s should not be array of empty objects", toolName)
		})
	}
}
