package slack

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	// ── Token Management ─────────────────────────────────────────────
	"slack_token_status": {"token_type", "source", "age", "healthy", "auto_refresh"},

	// ── Conversations ────────────────────────────────────────────────
	"slack_list_conversations":   {"id", "name", "is_channel", "is_private", "is_im", "is_mpim", "is_archived", "num_members", "topic.value", "purpose.value"},
	"slack_get_conversation_info": {"id", "name", "is_channel", "is_private", "is_im", "is_archived", "num_members", "topic.value", "purpose.value", "created"},
	"slack_conversations_history": {"messages[].ts", "messages[].user", "messages[].text", "messages[].type", "messages[].thread_ts", "messages[].reply_count", "has_more", "response_metadata.next_cursor"},
	"slack_get_thread":            {"messages[].ts", "messages[].user", "messages[].text", "messages[].type"},

	// ── Messages ─────────────────────────────────────────────────────
	"slack_search_messages": {"messages.matches[].ts", "messages.matches[].text", "messages.matches[].username", "messages.matches[].channel.id", "messages.matches[].channel.name", "messages.total"},
	"slack_get_reactions":   {"message.reactions[].name", "message.reactions[].count", "message.reactions[].users"},
	"slack_list_pins":       {"items[].message.ts", "items[].message.text", "items[].message.user", "items[].channel"},

	// ── Users ────────────────────────────────────────────────────────
	"slack_list_users":        {"members[].id", "members[].name", "members[].real_name", "members[].profile.email", "members[].is_admin", "members[].is_bot", "members[].deleted"},
	"slack_get_user_info":     {"user.id", "user.name", "user.real_name", "user.profile.email", "user.profile.status_text", "user.profile.status_emoji", "user.is_admin", "user.is_bot", "user.tz"},
	"slack_get_user_presence": {"presence", "online", "auto_away", "connection_count"},
	"slack_list_user_groups":  {"usergroups[].id", "usergroups[].name", "usergroups[].handle", "usergroups[].description", "usergroups[].user_count"},
	"slack_get_user_group":    {"users"},

	// ── Extras ───────────────────────────────────────────────────────
	"slack_auth_test":      {"ok", "url", "team", "user", "team_id", "user_id"},
	"slack_team_info":      {"team.id", "team.name", "team.domain", "team.icon.image_original"},
	"slack_list_files":     {"files[].id", "files[].name", "files[].title", "files[].mimetype", "files[].size", "files[].user", "files[].created", "files[].url_private"},
	"slack_list_emoji":     {"emoji"},
	"slack_list_bookmarks": {"bookmarks[].id", "bookmarks[].title", "bookmarks[].link", "bookmarks[].emoji"},
	"slack_list_reminders": {"reminders[].id", "reminders[].text", "reminders[].time", "reminders[].complete_ts"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("slack: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
