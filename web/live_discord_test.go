package web

import (
	"os"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/discord"
)

// Discord is a hosted service with no container image, so this spec reaches
// a real bot through stored sandbox credentials and runs read tools only:
//
//	DISCORD_LIVE_BOT_TOKEN   bot token with the Message Content intent
//	DISCORD_LIVE_GUILD_ID    a server the bot was invited to
//	DISCORD_LIVE_CHANNEL_ID  a text channel in that server the bot can read
//	DISCORD_LIVE_USER_ID     any user in that server
func init() {
	sends := "sends a real message other people see; request shapes are covered by fixture tests in integrations/discord"
	registerLiveSpec(liveSpec{
		Integration: "discord",
		Tier:        liveSandbox,
		Adapter:     discord.New(),
		Provision:   liveDiscordProvision,
		Cases: []liveCase{
			{Tool: "discord_get_me", Expect: []string{`"bot":true`, `"username"`}},
			{Tool: "discord_list_guilds", Expect: []string{"${guild_id}"}},
			{Tool: "discord_list_channels", Args: map[string]any{"guild_id": "${guild_id}"}, Expect: []string{"${channel_id}"}},
			{Tool: "discord_list_channel_messages", Args: map[string]any{"channel_id": "${channel_id}", "limit": 3}, Expect: []string{`"author"`}},
			{Tool: "discord_get_user", Args: map[string]any{"user_id": "${user_id}"}, Expect: []string{"${user_id}", `"username"`}},
		},
		Skips: map[mcp.ToolName]string{
			"discord_send_channel_message": sends,
			"discord_send_direct_message":  sends,
		},
	})
}

func liveDiscordProvision(t *testing.T) liveProvision {
	token := os.Getenv("DISCORD_LIVE_BOT_TOKEN")
	guildID := os.Getenv("DISCORD_LIVE_GUILD_ID")
	channelID := os.Getenv("DISCORD_LIVE_CHANNEL_ID")
	userID := os.Getenv("DISCORD_LIVE_USER_ID")
	if token == "" || guildID == "" || channelID == "" || userID == "" {
		t.Skip("set DISCORD_LIVE_BOT_TOKEN, DISCORD_LIVE_GUILD_ID, DISCORD_LIVE_CHANNEL_ID, and DISCORD_LIVE_USER_ID to reach a real Discord bot; it has no container image")
	}
	return liveProvision{
		Credentials: map[string]string{"bot_token": token},
		Values:      map[string]string{"guild_id": guildID, "channel_id": channelID, "user_id": userID},
	}
}
