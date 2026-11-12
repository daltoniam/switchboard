package discord

import mcp "github.com/daltoniam/switchboard"

const sendWarning = "WRITE: sends a real Discord message, as the bot, that people see immediately. Confirm the exact text and destination with the user before calling."

var tools = []mcp.ToolDefinition{
	{
		Name:        "discord_get_me",
		Description: "Get the Discord bot user this integration acts as (bot ID, username). Use to tell the bot's own messages apart from others.",
		Access:      mcp.AccessRead,
	},
	{
		Name:        "discord_list_guilds",
		Description: "List Discord servers (guilds) the bot has been invited to. Start here for Discord: find a guild_id, then list its channels to read or send messages.",
		Parameters: map[string]string{
			"limit":  "Servers to return, 1-200 (default 200)",
			"before": "Return servers before this guild ID",
			"after":  "Return servers after this guild ID",
		},
		Access: mcp.AccessRead,
	},
	{
		Name:        "discord_list_channels",
		Description: "List channels in a Discord server (guild): text, voice, category, announcement and forum channels with name, topic and parent category. type 0 is a text channel, 2 voice, 4 category, 5 announcement, 15 forum. Use after discord_list_guilds.",
		Parameters: map[string]string{
			"guild_id": "Server (guild) ID from discord_list_guilds",
		},
		Required: []string{"guild_id"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "discord_list_channel_messages",
		Description: "Read recent messages in a Discord channel or DM channel, newest first. Page backwards with before (oldest message ID from the previous page), forwards with after, or around one message. Message text is empty unless the bot has the Message Content intent. Use after discord_list_channels.",
		Parameters: map[string]string{
			"channel_id": "Channel ID from discord_list_channels, or a DM channel_id from discord_send_direct_message",
			"limit":      "Messages to return, 1-100 (default 50)",
			"before":     "Return messages before this message ID",
			"after":      "Return messages after this message ID",
			"around":     "Return messages around this message ID",
		},
		Required: []string{"channel_id"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "discord_get_user",
		Description: "Get a Discord user's public profile (username, display name, bot flag) by user ID, for example a message author.",
		Parameters: map[string]string{
			"user_id": "User ID, such as author.id from discord_list_channel_messages",
		},
		Required: []string{"user_id"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "discord_send_channel_message",
		Description: "Send (post) a message to a Discord channel as the bot. " + sendWarning + " @everyone, @here and role pings are suppressed; user mentions still notify. Use after discord_list_channels.",
		Parameters: map[string]string{
			"channel_id":          "Channel ID from discord_list_channels",
			"content":             "Message text, up to 2000 characters (Discord markdown)",
			"reply_to_message_id": "Optional message ID in the same channel to reply to",
		},
		Required: []string{"channel_id", "content"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        "discord_send_direct_message",
		Description: "Send a Discord direct message (DM) from the bot to one user: opens the DM channel and posts the message. " + sendWarning + " The user must share a server with the bot and allow DMs from server members. Returns the message with its channel_id for reading replies via discord_list_channel_messages.",
		Parameters: map[string]string{
			"recipient_id": "User ID to message",
			"content":      "Message text, up to 2000 characters (Discord markdown)",
		},
		Required: []string{"recipient_id", "content"},
		Access:   mcp.AccessWrite,
	},
}

var dispatch = map[mcp.ToolName]handlerFunc{
	"discord_get_me":                getMe,
	"discord_list_guilds":           listGuilds,
	"discord_list_channels":         listChannels,
	"discord_list_channel_messages": listChannelMessages,
	"discord_get_user":              getUser,
	"discord_send_channel_message":  sendChannelMessage,
	"discord_send_direct_message":   sendDirectMessage,
}
