package slack

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	// ── Token Management ─────────────────────────────────────────────
	// Handler: mcp.JSONResult(map{status, token_type, age_hours, source, updated_at, auto_refresh})
	"slack_token_status": {"status", "token_type", "age_hours", "source", "updated_at", "auto_refresh"},

	// ── Conversations ────────────────────────────────────────────────
	// Handler: mcp.JSONResult(map{count, conversations: []ch, next_cursor})
	"slack_list_conversations": {"conversations[].id", "conversations[].name", "conversations[].type", "conversations[].num_members", "conversations[].topic", "conversations[].purpose", "conversations[].is_archived", "count", "next_cursor"},
	// Handler: mcp.JSONResult(map{id, name, type, num_members, topic, purpose, is_archived, creator, created, ...})
	"slack_get_conversation_info": {"id", "name", "type", "num_members", "topic", "purpose", "is_archived", "creator", "created"},
	// Handler: mcp.JSONResult(map{channel, count, has_more, messages: [{ts, user, text, thread_ts, reply_count}], next_cursor})
	"slack_conversations_history": {"messages[].ts", "messages[].user", "messages[].text", "messages[].thread_ts", "messages[].reply_count", "count", "has_more", "next_cursor"},
	// Handler: mcp.JSONResult(map{channel, thread_ts, count, messages: [{ts, user, text, is_parent}]})
	"slack_get_thread": {"messages[].ts", "messages[].user", "messages[].text", "messages[].is_parent", "count", "thread_ts"},

	// ── Messages ─────────────────────────────────────────────────────
	// Handler: mcp.JSONResult(map{query, total, matches: [{ts, text, user, channel, channel_id, permalink}]})
	"slack_search_messages": {"matches[].ts", "matches[].text", "matches[].user", "matches[].channel", "matches[].channel_id", "matches[].permalink", "total", "query"},
	// Handler: mcp.JSONResult(bare array [{name, count, users}])
	"slack_get_reactions": {"name", "count", "users"},
	// Handler: mcp.JSONResult(map{count, pins: [{type, channel, message: {ts, text, user}, created}]})
	"slack_list_pins": {"pins[].type", "pins[].channel", "pins[].message.ts", "pins[].message.text", "pins[].message.user", "pins[].created", "count"},

	// ── Users ────────────────────────────────────────────────────────
	// Handler: mcp.JSONResult(map{count, users: [{id, name, real_name, display_name, email, is_admin, is_bot, deleted, timezone}]})
	"slack_list_users": {"users[].id", "users[].name", "users[].real_name", "users[].display_name", "users[].email", "users[].is_admin", "users[].is_bot", "users[].deleted", "count"},
	// Handler: mcp.JSONResult(map{id, name, real_name, display_name, email, title, status_text, status_emoji, timezone, is_admin, is_owner, is_bot, deleted})
	"slack_get_user_info": {"id", "name", "real_name", "display_name", "email", "status_text", "status_emoji", "timezone", "is_admin", "is_bot"},
	// Handler: mcp.JSONResult(map{user_id, presence, online})
	"slack_get_user_presence": {"user_id", "presence", "online"},
	// Handler: mcp.JSONResult(map{count, user_groups: [{id, name, handle, description, user_count, users}]})
	"slack_list_user_groups": {"user_groups[].id", "user_groups[].name", "user_groups[].handle", "user_groups[].description", "user_groups[].user_count", "count"},
	// Handler: mcp.JSONResult(map{usergroup_id, count, members})
	"slack_get_user_group": {"usergroup_id", "count", "members"},

	// ── Extras ───────────────────────────────────────────────────────
	// Handler: mcp.JSONResult(map{user, user_id, team, team_id, url})
	"slack_auth_test": {"user", "user_id", "team", "team_id", "url"},
	// Handler: mcp.JSONResult(map{id, name, domain, icon})
	"slack_team_info": {"id", "name", "domain", "icon"},
	// Handler: mcp.JSONResult(map{count, files: [{id, name, title, filetype, size, user}]})
	"slack_list_files": {"files[].id", "files[].name", "files[].title", "files[].filetype", "files[].size", "files[].user", "count"},
	// Handler: mcp.JSONResult(map{count, emoji: map})
	"slack_list_emoji": {"count", "emoji"},
	// Handler: mcp.JSONResult(map{count, bookmarks: [{id, title, link, emoji, type}]})
	"slack_list_bookmarks": {"bookmarks[].id", "bookmarks[].title", "bookmarks[].link", "bookmarks[].emoji", "bookmarks[].type", "count"},
	// Handler: mcp.JSONResult(map{count, reminders: [{id, text, user, time}]})
	"slack_list_reminders": {"reminders[].id", "reminders[].text", "reminders[].user", "reminders[].time", "count"},
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
