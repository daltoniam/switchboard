# Discord setup

The `discord` integration acts as a **Discord bot** through the REST API (`https://discord.com/api/v10`). It reads servers, channels, and messages the bot can see, and sends channel messages and direct messages as the bot.

Only bot tokens are supported. Using a personal account's token (a "self-bot") breaks Discord's Terms of Service and can get the account banned, so Switchboard always sends `Authorization: Bot <token>`.

## 1. Create the bot

1. Open the [Discord Developer Portal](https://discord.com/developers/applications) and select **New Application**. Give it a name; this is the name people see on the bot's messages.
2. Open **Bot** in the sidebar.
3. Select **Reset Token**, then copy the token. Discord shows it only once. Treat it like a password: anyone with it controls the bot.
4. Under **Privileged Gateway Intents**, turn on **Message Content Intent** and save. Without it, Discord returns an empty `content` for messages that don't mention the bot, including over REST. Bots in fewer than 100 servers need no verification for this intent.
5. Optional: turn off **Public Bot** so only you can invite it.

## 2. Invite the bot to a server

Copy the **Application ID** from **General Information**, then open this URL with your ID filled in:

```
https://discord.com/oauth2/authorize?client_id=<APPLICATION_ID>&scope=bot&permissions=68608
```

`scope=bot` adds the bot user. `permissions=68608` grants exactly what the tools use:

| Permission | Value | Used by |
|---|---|---|
| View Channels | 1024 | `discord_list_channels`, reading messages |
| Send Messages | 2048 | `discord_send_channel_message` |
| Read Message History | 65536 | `discord_list_channel_messages` |

You need **Manage Server** on the server to invite a bot. Channel permission overrides still apply, so a private channel needs the bot (or one of its roles) added to it.

## 3. Configure Switchboard

Enter the token as `bot_token` on the Discord integration page and enable it, or set `DISCORD_BOT_TOKEN`. A token pasted with its `Bot ` prefix is accepted. The badge shows **Connected** once `GET /users/@me` succeeds.

## Tools

| Tool | Access | Endpoint |
|---|---|---|
| `discord_get_me` | read | `GET /users/@me` |
| `discord_list_guilds` (start here) | read | `GET /users/@me/guilds` |
| `discord_list_channels` | read | `GET /guilds/{guild_id}/channels` |
| `discord_list_channel_messages` | read | `GET /channels/{channel_id}/messages` |
| `discord_get_user` | read | `GET /users/{user_id}` |
| `discord_send_channel_message` | write | `POST /channels/{channel_id}/messages` |
| `discord_send_direct_message` | write | `POST /users/@me/channels`, then `POST /channels/{id}/messages` |

Sends never ping `@everyone`, `@here`, or roles (`allowed_mentions` parses user mentions only).

Direct messages: a bot can only DM users who share a server with it and allow DMs from server members (User Settings → Privacy). Otherwise Discord returns error `50007`. The send result includes the DM `channel_id`; pass it to `discord_list_channel_messages` to read replies.

## Rate limits

On a 429, Switchboard waits for the `retry_after` Discord returns and retries. Sends retry only on 429, never after a server error, because Discord may have delivered the message already.
