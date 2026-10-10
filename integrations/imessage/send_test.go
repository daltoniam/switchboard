package imessage

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sendResponseT = sendResponse

func insertOutgoing(t *testing.T, path string, chatID int64, text string, errCode int) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	date := toAppleTime(time.Now())
	res, err := db.Exec(`INSERT INTO message (guid, text, handle_id, service, date, is_from_me, is_read, error)
		VALUES ('sent-1', ?, 0, 'iMessage', ?, 1, 1, ?)`, text, date, errCode)
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	if chatID > 0 {
		_, err = db.Exec(`INSERT INTO chat_message_join VALUES (?, ?, ?)`, chatID, id, date)
		require.NoError(t, err)
	}
}

func TestSendMessage_Disabled(t *testing.T) {
	m, runner := newConfigured(t, nil)
	msg := execErr(t, m, "imessage_send_message", map[string]any{"text": "hi", "to": "+15551234567"})
	assert.Contains(t, msg, "allow_send")
	assert.Empty(t, runner.calls)
}

func TestSendMessage_Validation(t *testing.T) {
	m, runner := newConfigured(t, mcp.Credentials{"allow_send": "true", "send_allowlist": "+15551234567"})
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "blank text", args: map[string]any{"text": " ", "to": "+15551234567"}, want: "text is required"},
		{name: "no recipient", args: map[string]any{"text": "hi"}, want: "chat_id or to is required"},
		{name: "bad service", args: map[string]any{"text": "hi", "to": "+15551234567", "service": "fax"}, want: "service must be"},
		{name: "unknown chat", args: map[string]any{"text": "hi", "chat_id": 42}, want: "chat 42 not found"},
		{name: "to not allowlisted", args: map[string]any{"text": "hi", "to": "+15559876543"}, want: "not in send_allowlist"},
		{name: "group with non-allowlisted member", args: map[string]any{"text": "hi", "chat_id": 2}, want: "bob@example.com is not in send_allowlist"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, execErr(t, m, "imessage_send_message", tt.args), tt.want)
		})
	}
	assert.Empty(t, runner.calls)
}

func TestSendMessage_ToChat(t *testing.T) {
	m, runner := newConfigured(t, mcp.Credentials{"allow_send": "true"})
	runner.onRun = func(args []string) { insertOutgoing(t, m.dbPath, 2, args[1], 0) }

	resp := execJSON[sendResponseT](t, m, "imessage_send_message", map[string]any{"text": "On my way \"now\"", "chat_id": 2})
	require.Len(t, runner.calls, 1)
	assert.Equal(t, []string{"any;+;chat123", "On my way \"now\""}, runner.calls[0])
	assert.Contains(t, runner.scripts[0], "on run argv")
	assert.NotContains(t, runner.scripts[0], "On my way", "message text must be passed as argv, never interpolated")
	assert.Equal(t, "sent", resp.Status)
	assert.Equal(t, int64(2), resp.ChatID)
	require.NotNil(t, resp.Message)
	assert.Equal(t, "On my way \"now\"", resp.Message.Text)
}

func TestSendMessage_ToHandleUsesExistingChat(t *testing.T) {
	m, runner := newConfigured(t, mcp.Credentials{"allow_send": "true", "send_allowlist": "5551234567"})
	resp := execJSON[sendResponseT](t, m, "imessage_send_message", map[string]any{"text": "hey", "to": "(555) 123-4567"})
	require.Len(t, runner.calls, 1)
	assert.Equal(t, []string{"any;-;+15551234567-sms", "hey"}, runner.calls[0], "most recently active 1:1 chat")
	assert.Equal(t, "queued", resp.Status)
	assert.Equal(t, int64(3), resp.ChatID)
}

func TestSendMessage_ToNewHandle(t *testing.T) {
	m, runner := newConfigured(t, mcp.Credentials{"allow_send": "true"})
	tests := []struct {
		name    string
		service string
		want    string
	}{
		{name: "default iMessage", service: "", want: "iMessage"},
		{name: "explicit sms", service: "SMS", want: "SMS"},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := execJSON[sendResponseT](t, m, "imessage_send_message", map[string]any{"text": "hello", "to": "new@example.com", "service": tt.service})
			require.Len(t, runner.calls, i+1)
			assert.Equal(t, []string{"new@example.com", "hello", tt.want}, runner.calls[i])
			assert.Contains(t, runner.scripts[i], "participant")
			assert.Equal(t, "queued", resp.Status)
			assert.Equal(t, "new@example.com", resp.To)
		})
	}

	t.Run("explicit service bypasses existing chat", func(t *testing.T) {
		before := len(runner.calls)
		execJSON[sendResponseT](t, m, "imessage_send_message", map[string]any{"text": "hello", "to": "+15551234567", "service": "imessage"})
		assert.Equal(t, []string{"+15551234567", "hello", "iMessage"}, runner.calls[before])
	})
}

func TestSendMessage_Failures(t *testing.T) {
	t.Run("script error", func(t *testing.T) {
		m, runner := newConfigured(t, mcp.Credentials{"allow_send": "true"})
		runner.err = errors.New("Not authorized to send Apple events to Messages")
		msg := execErr(t, m, "imessage_send_message", map[string]any{"text": "hi", "chat_id": 1})
		assert.Contains(t, msg, "Not authorized")
		assert.Contains(t, msg, "Automation")
	})

	t.Run("delivery error recorded", func(t *testing.T) {
		m, runner := newConfigured(t, mcp.Credentials{"allow_send": "true"})
		runner.onRun = func(args []string) { insertOutgoing(t, m.dbPath, 1, args[1], 22) }
		resp := execJSON[sendResponseT](t, m, "imessage_send_message", map[string]any{"text": "hi", "chat_id": 1})
		assert.Equal(t, "failed", resp.Status)
		assert.Equal(t, 22, resp.ErrorCode)
	})
}

func TestRunOSAScript_NonDarwin(t *testing.T) {
	orig := runtimeGOOS
	runtimeGOOS = "linux"
	t.Cleanup(func() { runtimeGOOS = orig })
	_, err := runOSAScript(t.Context(), "on run argv\nend run", "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires macOS")
}
