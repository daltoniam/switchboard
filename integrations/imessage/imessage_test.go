package imessage

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "imessage", i.Name())
}

func TestConfigure(t *testing.T) {
	t.Run("success with fixture db", func(t *testing.T) {
		m, _ := newConfigured(t, mcp.Credentials{"allow_send": "true", "send_allowlist": "+1 (555) 123-4567, Bob@Example.com"})
		assert.True(t, m.allowSend)
		assert.True(t, m.allowlist["5551234567"])
		assert.True(t, m.allowlist["bob@example.com"])
		assert.True(t, m.Healthy(context.Background()))
	})

	t.Run("requires macOS without db_path", func(t *testing.T) {
		orig := runtimeGOOS
		runtimeGOOS = "linux"
		t.Cleanup(func() { runtimeGOOS = orig })
		err := New().Configure(context.Background(), mcp.Credentials{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires macOS")
	})

	t.Run("invalid allow_send", func(t *testing.T) {
		err := New().Configure(context.Background(), mcp.Credentials{"db_path": newFixtureDB(t), "allow_send": "maybe"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "allow_send must be true or false")
	})

	t.Run("missing database", func(t *testing.T) {
		err := New().Configure(context.Background(), mcp.Credentials{"db_path": filepath.Join(t.TempDir(), "missing.db")})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot read")
	})

	t.Run("not a messages database", func(t *testing.T) {
		path := newFixtureContacts(t)
		err := New().Configure(context.Background(), mcp.Credentials{"db_path": filepath.Join(path, "Sources", "ABC", "AddressBook-v22.abcddb")})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "table message not found")
	})
}

func TestHealthy_Unconfigured(t *testing.T) {
	assert.False(t, New().Healthy(context.Background()))
}

func TestTools(t *testing.T) {
	seen := make(map[mcp.ToolName]bool)
	for _, tool := range New().Tools() {
		assert.NotEmpty(t, tool.Name)
		assert.NotEmpty(t, tool.Description)
		assert.Contains(t, string(tool.Name), "imessage_")
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
	assert.Contains(t, tools[0].Description, "Start here")
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range New().Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range New().Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	m, _ := newConfigured(t, nil)
	result, err := m.Execute(context.Background(), "imessage_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestExecute_NotConfigured(t *testing.T) {
	result, err := New().Execute(context.Background(), "imessage_list_chats", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "not configured")
}

func execJSON[T any](t *testing.T, m *imessage, tool mcp.ToolName, args map[string]any) T {
	t.Helper()
	result, err := m.Execute(context.Background(), tool, args)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	var out T
	require.NoError(t, json.Unmarshal([]byte(result.Data), &out), result.Data)
	return out
}

func execErr(t *testing.T, m *imessage, tool mcp.ToolName, args map[string]any) string {
	t.Helper()
	result, err := m.Execute(context.Background(), tool, args)
	require.NoError(t, err)
	require.True(t, result.IsError, result.Data)
	return result.Data
}

func chatIDs(chats []chatSummary) []int64 {
	ids := make([]int64, 0, len(chats))
	for _, c := range chats {
		ids = append(ids, c.ChatID)
	}
	return ids
}

func messageIDs(msgs []messageOut) []int64 {
	ids := make([]int64, 0, len(msgs))
	for _, msg := range msgs {
		ids = append(ids, msg.ID)
	}
	return ids
}

func TestListChats(t *testing.T) {
	m, _ := newConfigured(t, nil)

	t.Run("ordered by recent activity with enrichment", func(t *testing.T) {
		resp := execJSON[listChatsResponse](t, m, "imessage_list_chats", nil)
		require.Equal(t, []int64{2, 4, 3, 1}, chatIDs(resp.Chats))
		assert.Equal(t, 4, resp.Total)
		assert.False(t, resp.HasMore)

		group := resp.Chats[0]
		assert.Equal(t, "Weekend Plans", group.Name)
		assert.True(t, group.IsGroup)
		assert.Equal(t, "any;+;chat123", group.GUID)
		assert.Equal(t, []participant{{Handle: "+15551234567", Name: "Alice Smith"}, {Handle: "bob@example.com", Name: "Bob Jones"}}, group.Participants)
		assert.Equal(t, "I will", group.LastMessage)
		assert.True(t, group.LastFromMe)
		assert.Equal(t, 1, group.UnreadCount)

		direct := resp.Chats[3]
		assert.Equal(t, "Alice Smith", direct.Name)
		assert.False(t, direct.IsGroup)
		assert.Equal(t, "Lunch tomorrow?", direct.LastMessage)
		assert.Equal(t, 0, direct.UnreadCount)

		unknown := resp.Chats[1]
		assert.Equal(t, "+15559876543", unknown.Name)
		assert.Equal(t, "SMS", unknown.Service)
	})

	tests := []struct {
		name  string
		args  map[string]any
		want  []int64
		more  bool
		total int
	}{
		{name: "query by contact name", args: map[string]any{"query": "bob"}, want: []int64{2}, total: 1},
		{name: "query by chat name", args: map[string]any{"query": "weekend"}, want: []int64{2}, total: 1},
		{name: "query by phone digits ranks direct chats first", args: map[string]any{"query": "555-123-4567"}, want: []int64{3, 1, 2}, total: 3},
		{name: "query by contact name ranks direct chats first", args: map[string]any{"query": "Alice Smith"}, want: []int64{3, 1, 2}, total: 3},
		{name: "query by email ranks direct chats first", args: map[string]any{"query": "bob@example.com"}, want: []int64{2}, total: 1},
		{name: "pagination", args: map[string]any{"limit": 2, "offset": 1}, want: []int64{4, 3}, more: true, total: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := execJSON[listChatsResponse](t, m, "imessage_list_chats", tt.args)
			assert.Equal(t, tt.want, chatIDs(resp.Chats))
			assert.Equal(t, tt.more, resp.HasMore)
			assert.Equal(t, tt.total, resp.Total)
		})
	}
}

func TestGetChatMessages(t *testing.T) {
	m, _ := newConfigured(t, nil)

	t.Run("one to one with reactions and decoded body", func(t *testing.T) {
		resp := execJSON[chatMessagesResponse](t, m, "imessage_get_chat_messages", map[string]any{"chat_id": 1})
		require.Equal(t, []int64{1, 2}, messageIDs(resp.Messages))
		assert.Equal(t, "Alice Smith", resp.Name)
		first, second := resp.Messages[0], resp.Messages[1]
		assert.Equal(t, "hello there", first.Text)
		assert.Equal(t, "+15551234567", first.Sender)
		assert.Equal(t, "Alice Smith", first.SenderName)
		assert.False(t, first.FromMe)
		assert.Equal(t, "Lunch tomorrow?", second.Text)
		assert.True(t, second.FromMe)
		assert.Equal(t, "me", second.Sender)
		assert.Equal(t, []reaction{{Type: "love", By: "+15551234567", ByName: "Alice Smith"}}, second.Reactions)
		assert.False(t, resp.HasMore)
	})

	t.Run("group with attachments replies and reaction removal", func(t *testing.T) {
		resp := execJSON[chatMessagesResponse](t, m, "imessage_get_chat_messages", map[string]any{"chat_id": "2"})
		require.Equal(t, []int64{5, 6}, messageIDs(resp.Messages))
		snack := resp.Messages[0]
		assert.Equal(t, "Bob Jones", snack.SenderName)
		require.Len(t, snack.Attachments, 1)
		assert.Equal(t, "IMG_0001.HEIC", snack.Attachments[0].Name)
		assert.Equal(t, "image/heic", snack.Attachments[0].MIMEType)
		assert.Equal(t, int64(2048), snack.Attachments[0].Bytes)
		assert.Equal(t, []reaction{{Type: "emoji", Emoji: "\U0001F525", By: "+15551234567", ByName: "Alice Smith"}}, snack.Reactions)
		assert.Equal(t, "msg-5", resp.Messages[1].ReplyTo)
	})

	t.Run("handle merges iMessage and SMS chats", func(t *testing.T) {
		resp := execJSON[chatMessagesResponse](t, m, "imessage_get_chat_messages", map[string]any{"handle": "555.123.4567"})
		assert.Equal(t, []int64{1, 3}, resp.ChatIDs)
		assert.Equal(t, []int64{1, 2, 4}, messageIDs(resp.Messages))
		assert.Equal(t, "SMS", resp.Messages[2].Service)
	})

	t.Run("pagination with next_before", func(t *testing.T) {
		page1 := execJSON[chatMessagesResponse](t, m, "imessage_get_chat_messages", map[string]any{"chat_id": 1, "limit": 1})
		require.Equal(t, []int64{2}, messageIDs(page1.Messages))
		require.True(t, page1.HasMore)
		require.NotEmpty(t, page1.NextBefore)
		page2 := execJSON[chatMessagesResponse](t, m, "imessage_get_chat_messages", map[string]any{"chat_id": 1, "limit": 1, "before": page1.NextBefore})
		assert.Equal(t, []int64{1}, messageIDs(page2.Messages))
		assert.False(t, page2.HasMore)
	})

	t.Run("since filter", func(t *testing.T) {
		since := fixtureT0.Add(30 * 1e9).Format("2006-01-02T15:04:05Z07:00")
		resp := execJSON[chatMessagesResponse](t, m, "imessage_get_chat_messages", map[string]any{"chat_id": 1, "since": since})
		assert.Equal(t, []int64{2}, messageIDs(resp.Messages))
	})

	errTests := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing chat and handle", args: map[string]any{}, want: "chat_id or handle is required"},
		{name: "unknown chat", args: map[string]any{"chat_id": 99}, want: "chat 99 not found"},
		{name: "unknown handle", args: map[string]any{"handle": "+19995550000"}, want: "no conversation found"},
		{name: "bad before", args: map[string]any{"chat_id": 1, "before": "yesterday"}, want: "before must be"},
	}
	for _, tt := range errTests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, execErr(t, m, "imessage_get_chat_messages", tt.args), tt.want)
		})
	}
}

func TestSearchMessages(t *testing.T) {
	m, _ := newConfigured(t, nil)
	tests := []struct {
		name string
		args map[string]any
		want []int64
	}{
		{name: "case insensitive attributedBody", args: map[string]any{"query": "LUNCH"}, want: []int64{2}},
		{name: "text column", args: map[string]any{"query": "code is"}, want: []int64{7}},
		{name: "reactions excluded", args: map[string]any{"query": "loved"}, want: []int64{}},
		{name: "scoped to chat", args: map[string]any{"query": "lunch", "chat_id": 2}, want: []int64{}},
		{name: "scoped to handle", args: map[string]any{"query": "s", "handle": "bob@example.com"}, want: []int64{5}},
		{name: "date range", args: map[string]any{"query": "n", "since": fixtureT0.Add(150 * 1e9).Format("2006-01-02T15:04:05Z07:00"), "before": fixtureT0.Add(270 * 1e9).Format("2006-01-02T15:04:05Z07:00")}, want: []int64{5, 4}},
		{name: "limit", args: map[string]any{"query": "i", "limit": 2}, want: []int64{7, 6}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := execJSON[messagesResponse](t, m, "imessage_search_messages", tt.args)
			assert.Equal(t, tt.want, messageIDs(resp.Messages))
			assert.Equal(t, len(tt.want), resp.Count)
		})
	}

	t.Run("includes chat name", func(t *testing.T) {
		resp := execJSON[messagesResponse](t, m, "imessage_search_messages", map[string]any{"query": "snacks"})
		require.Len(t, resp.Messages, 1)
		assert.Equal(t, int64(2), resp.Messages[0].ChatID)
		assert.Equal(t, "Weekend Plans", resp.Messages[0].ChatName)
	})

	t.Run("blank query", func(t *testing.T) {
		assert.Contains(t, execErr(t, m, "imessage_search_messages", map[string]any{"query": "  "}), "query is required")
	})
}

func TestListUnread(t *testing.T) {
	m, _ := newConfigured(t, nil)
	resp := execJSON[messagesResponse](t, m, "imessage_list_unread", nil)
	assert.Equal(t, []int64{7, 5, 4}, messageIDs(resp.Messages))
	assert.Equal(t, "+15559876543", resp.Messages[0].ChatName)

	limited := execJSON[messagesResponse](t, m, "imessage_list_unread", map[string]any{"limit": 1})
	assert.Equal(t, []int64{7}, messageIDs(limited.Messages))
}

func TestLookupContact(t *testing.T) {
	m, _ := newConfigured(t, nil)
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{name: "by first name", query: "alice", want: []string{"Alice Smith"}},
		{name: "by phone fragment", query: "555-123-4567", want: []string{"Alice Smith"}},
		{name: "by email", query: "bob@example", want: []string{"Bob Jones"}},
		{name: "organization fallback", query: "acme", want: []string{"Acme Corp"}},
		{name: "no match", query: "zed", want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := execJSON[contactsResponse](t, m, "imessage_lookup_contact", map[string]any{"query": tt.query})
			names := []string{}
			for _, c := range resp.Contacts {
				names = append(names, c.Name)
			}
			assert.Equal(t, tt.want, names)
		})
	}

	t.Run("missing contacts", func(t *testing.T) {
		m2, _ := newConfigured(t, mcp.Credentials{"contacts_dir": filepath.Join(t.TempDir(), "none")})
		assert.Contains(t, execErr(t, m2, "imessage_lookup_contact", map[string]any{"query": "alice"}), "cannot read")
	})
}

func TestAppleTime(t *testing.T) {
	assert.True(t, appleTime(0).IsZero())
	assert.Equal(t, fixtureT0.Unix(), appleTime(toAppleTime(fixtureT0)).Unix())
	assert.Equal(t, fixtureT0.Unix(), appleTime(fixtureT0.Unix()-appleEpochOffset).Unix(), "legacy seconds")
}

func TestNormalizeHandle(t *testing.T) {
	tests := map[string]string{
		"+1 (555) 123-4567": "5551234567",
		"5551234567":        "5551234567",
		" Bob@Example.com ": "bob@example.com",
		"":                  "",
		"chat123":           "123",
		"shortcode":         "shortcode",
	}
	for in, want := range tests {
		assert.Equal(t, want, normalizeHandle(in), in)
	}
}
