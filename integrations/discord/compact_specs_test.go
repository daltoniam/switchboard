package discord

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
	d := &discord{}
	for _, tool := range tools {
		_, ok := d.CompactSpec(tool.Name)
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
		{"discord_get_me", `{"id":"9","username":"hermes","global_name":null,"bot":true,"avatar":"abc","flags":0,"mfa_enabled":true}`,
			[]string{`"hermes"`, `"9"`, `"bot":true`}, []string{"avatar", "mfa_enabled"}},
		{"discord_list_guilds", `[{"id":"1","name":"Home","icon":"abc","banner":null,"owner":false,"permissions":"2248473465835073","features":["COMMUNITY"],"approximate_member_count":5}]`,
			[]string{`"Home"`, `"1"`}, []string{"icon", "permissions", "features"}},
		{"discord_list_channels", `[{"id":"2","type":0,"guild_id":"1","name":"general","position":0,"parent_id":"3","topic":"chat","nsfw":false,"permission_overwrites":[{"id":"1"}],"rate_limit_per_user":0,"last_message_id":"99"}]`,
			[]string{`"general"`, `"type":0`, `"parent_id":"3"`, `"chat"`}, []string{"permission_overwrites", "rate_limit_per_user", "nsfw"}},
		{"discord_list_channel_messages", `[{"id":"98","channel_id":"2","type":0,"content":"hello","timestamp":"2026-11-11T00:00:00Z","edited_timestamp":null,"author":{"id":"7","username":"ann","global_name":"Ann","avatar":"x","discriminator":"0"},"attachments":[{"id":"a","filename":"pic.png","url":"https://cdn/pic.png","proxy_url":"https://media/pic.png","size":10}],"embeds":[],"mentions":[],"mention_roles":[],"pinned":false,"tts":false,"flags":0,"message_reference":{"message_id":"97"},"nonce":"n"}]`,
			[]string{`"hello"`, `"98"`, `"ann"`, `"Ann"`, `"pic.png"`, `"97"`}, []string{"avatar", "proxy_url", "discriminator", "nonce", "mention_roles"}},
		{"discord_get_user", `{"id":"7","username":"ann","global_name":"Ann","bot":false,"avatar":"x","banner":null,"accent_color":null,"discriminator":"0"}`,
			[]string{`"ann"`, `"Ann"`}, []string{"avatar", "discriminator"}},
	}
	d := &discord{}
	for _, tc := range tests {
		t.Run(string(tc.tool), func(t *testing.T) {
			fields, ok := d.CompactSpec(tc.tool)
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
