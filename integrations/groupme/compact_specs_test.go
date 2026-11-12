package groupme

import (
	"encoding/json"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs)
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

func TestFieldCompactionSpecs_EveryReadToolHasSpec(t *testing.T) {
	g := &groupme{}
	for _, tool := range tools {
		_, ok := g.CompactSpec(tool.Name)
		if tool.Access == mcp.AccessRead {
			assert.True(t, ok, "read tool %s needs a compaction spec", tool.Name)
		} else {
			assert.False(t, ok, "write tool %s must return its full confirmation", tool.Name)
		}
	}
}

// Shape parity: each spec, applied to a payload shaped like the handler's
// output, keeps the routing fields and drops the noise.
func TestFieldCompactionSpecs_Shapes(t *testing.T) {
	tests := []struct {
		tool    mcp.ToolName
		payload string
		keep    []string
		drop    []string
	}{
		{"groupme_get_me", `{"id":"9","user_id":"9","name":"Bryson","phone_number":"+1 555","email":"x@y.z","image_url":"https://i"}`,
			[]string{`"Bryson"`, `"9"`}, []string{"phone_number", "email", "image_url"}},
		{"groupme_list_groups", `[{"id":"1","name":"Family","type":"private","description":"d","image_url":"https://i","share_url":"https://s","updated_at":5,"members":null,"messages":{"count":3,"last_message_id":"77","last_message_created_at":4,"preview":{"nickname":"Ann","text":"hey","image_url":"https://p","attachments":[]}}}]`,
			[]string{`"Family"`, `"77"`, `"Ann"`, `"hey"`}, []string{"image_url", "share_url"}},
		{"groupme_get_group", `{"id":"1","name":"Family","members":[{"user_id":"7","nickname":"Ann","image_url":"https://i","muted":false,"roles":["admin"]}],"messages":{"count":3,"last_message_id":"77"}}`,
			[]string{`"Family"`, `"user_id":"7"`, `"Ann"`, `"admin"`}, []string{"image_url", "muted"}},
		{"groupme_list_group_messages", `{"count":1,"messages":[{"id":"54","created_at":1,"user_id":"7","sender_id":"7","name":"Ann","text":"hi","avatar_url":"https://a","source_guid":"g","favorited_by":["9"],"attachments":[],"system":false}]}`,
			[]string{`"count":1`, `"54"`, `"hi"`, `"Ann"`}, []string{"avatar_url", "source_guid"}},
		{"groupme_list_chats", `[{"created_at":1,"updated_at":2,"messages_count":4,"other_user":{"id":"7","name":"Ann","avatar_url":"https://a"},"last_message":{"id":"8","text":"yo","created_at":2,"sender_id":"7","avatar_url":"https://a"}}]`,
			[]string{`"7"`, `"Ann"`, `"yo"`}, []string{"avatar_url"}},
		{"groupme_list_direct_messages", `{"count":1,"direct_messages":[{"id":"87","created_at":1,"sender_id":"7","recipient_id":"9","name":"Ann","text":"yo","avatar_url":"https://a","source_guid":"g"}]}`,
			[]string{`"87"`, `"yo"`, `"recipient_id":"9"`}, []string{"avatar_url", "source_guid"}},
		{"groupme_list_bots", `[{"bot_id":"b1","group_id":"1","name":"Bot","avatar_url":"https://a","callback_url":"https://c","dm_notification":false,"active":true}]`,
			[]string{`"b1"`, `"Bot"`}, []string{"avatar_url", "dm_notification"}},
	}
	g := &groupme{}
	for _, tc := range tests {
		t.Run(string(tc.tool), func(t *testing.T) {
			fields, ok := g.CompactSpec(tc.tool)
			require.True(t, ok)
			var payload any
			require.NoError(t, json.Unmarshal([]byte(tc.payload), &payload))
			out, err := json.Marshal(mcp.CompactAny(payload, fields))
			require.NoError(t, err)
			for _, want := range tc.keep {
				assert.Contains(t, string(out), want)
			}
			for _, unwanted := range tc.drop {
				assert.NotContains(t, string(out), unwanted)
			}
		})
	}
}
