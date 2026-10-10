package imessage

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

type scriptRunner func(ctx context.Context, script string, args ...string) ([]byte, error)

// Message text and recipients are always passed as argv to the AppleScript
// run handler, never interpolated into the script source, so message content
// cannot inject AppleScript.
const sendToChatScript = `on run argv
	tell application "Messages"
		send (item 2 of argv) to chat id (item 1 of argv)
	end tell
end run`

const sendToParticipantScript = `on run argv
	set targetHandle to item 1 of argv
	set messageText to item 2 of argv
	set serviceKind to item 3 of argv
	tell application "Messages"
		if serviceKind is "SMS" then
			set targetAccount to first account whose service type = SMS
		else
			set targetAccount to first account whose service type = iMessage
		end if
		send messageText to participant targetHandle of targetAccount
	end tell
end run`

const osascriptPath = "/usr/bin/osascript"

const confirmPollInterval = 250 * time.Millisecond

func runOSAScript(ctx context.Context, script string, args ...string) ([]byte, error) {
	if runtimeGOOS != "darwin" {
		return nil, errors.New("sending messages requires macOS with the Messages app")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, osascriptPath, append([]string{"-"}, args...)...) // #nosec G204 -- fixed binary; args are passed to the script's argv, not a shell
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("osascript: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

type sendResponse struct {
	Status    string      `json:"status"`
	ChatID    int64       `json:"chat_id,omitempty"`
	To        string      `json:"to,omitempty"`
	Service   string      `json:"service,omitempty"`
	ErrorCode int         `json:"error_code,omitempty"`
	Note      string      `json:"note,omitempty"`
	Message   *messageOut `json:"message,omitempty"`
}

func (m *imessage) checkAllowed(handles ...string) error {
	if len(m.allowlist) == 0 {
		return nil
	}
	for _, h := range handles {
		if !m.allowlist[normalizeHandle(h)] {
			return fmt.Errorf("recipient %s is not in send_allowlist", h)
		}
	}
	return nil
}

func parseService(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "":
		return "", nil
	case "imessage":
		return "iMessage", nil
	case "sms":
		return "SMS", nil
	}
	return "", fmt.Errorf("service must be 'imessage' or 'sms', got %q", v)
}

func sendMessage(ctx context.Context, m *imessage, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	text := r.Str("text")
	chatID := r.Int64("chat_id")
	to := strings.TrimSpace(r.Str("to"))
	serviceArg := r.Str("service")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if !m.allowSend {
		return mcp.ErrResult(fmt.Errorf("sending is disabled; set allow_send to true in the imessage integration settings to enable imessage_send_message"))
	}
	if strings.TrimSpace(text) == "" {
		return mcp.ErrResult(fmt.Errorf("text is required"))
	}
	service, err := parseService(serviceArg)
	if err != nil {
		return mcp.ErrResult(err)
	}

	resp := sendResponse{}
	var script string
	var scriptArgs []string
	switch {
	case chatID > 0:
		guid, err := m.chatGUIDForSend(ctx, chatID)
		if err != nil {
			return mcp.ErrResult(err)
		}
		resp.ChatID = chatID
		script, scriptArgs = sendToChatScript, []string{guid, text}
	case to != "":
		if err := m.checkAllowed(to); err != nil {
			return mcp.ErrResult(err)
		}
		resp.To = to
		if service == "" {
			chat, ok, err := m.recentDirectChat(ctx, to)
			if err != nil {
				return mcp.ErrResult(err)
			}
			if ok {
				resp.ChatID = chat.id
				resp.Service = chat.service
				script, scriptArgs = sendToChatScript, []string{chat.guid, text}
				break
			}
			service = "iMessage"
		}
		resp.Service = service
		script, scriptArgs = sendToParticipantScript, []string{to, text, service}
	default:
		return mcp.ErrResult(fmt.Errorf("chat_id or to is required"))
	}

	start := time.Now().Add(-2 * time.Second)
	if _, err := m.runScript(ctx, script, scriptArgs...); err != nil {
		return mcp.ErrResult(fmt.Errorf("messages app rejected the send: %w (on first use, allow switchboard to control Messages in System Settings > Privacy & Security > Automation)", err))
	}

	msg, errCode, err := m.confirmSent(ctx, resp.ChatID, text, start)
	if err != nil {
		return mcp.ErrResult(err)
	}
	switch {
	case msg == nil:
		resp.Status = "queued"
		resp.Note = "Messages accepted the send but it is not yet visible in the message database; check the chat shortly"
	case errCode != 0:
		resp.Status = "failed"
		resp.ErrorCode = errCode
		resp.Message = msg
		resp.Note = "Messages recorded a delivery error for this message"
	default:
		resp.Status = "sent"
		resp.Message = msg
		if resp.ChatID == 0 {
			resp.ChatID = msg.ChatID
		}
	}
	return mcp.JSONResult(resp)
}

func (m *imessage) chatGUIDForSend(ctx context.Context, chatID int64) (string, error) {
	chats, err := m.loadChats(ctx, []int64{chatID})
	if err != nil {
		return "", err
	}
	if len(chats) == 0 {
		return "", fmt.Errorf("chat %d not found; use imessage_list_chats to find a chat_id", chatID)
	}
	parts, err := m.loadParticipants(ctx, []int64{chatID})
	if err != nil {
		return "", err
	}
	handles := make([]string, 0, len(parts[chatID]))
	for _, p := range parts[chatID] {
		handles = append(handles, p.Handle)
	}
	if len(handles) == 0 {
		handles = append(handles, chats[0].ident)
	}
	if err := m.checkAllowed(handles...); err != nil {
		return "", err
	}
	return chats[0].guid, nil
}

func (m *imessage) recentDirectChat(ctx context.Context, handle string) (chatRow, bool, error) {
	key := normalizeHandle(handle)
	chats, err := m.loadChats(ctx, nil)
	if err != nil {
		return chatRow{}, false, err
	}
	for _, c := range chats {
		if c.style != groupChatStyle && normalizeHandle(c.ident) == key {
			return c, true, nil
		}
	}
	return chatRow{}, false, nil
}

// confirmSent polls chat.db for the outgoing message so the caller learns
// whether Messages actually recorded (or failed) the send.
func (m *imessage) confirmSent(ctx context.Context, chatID int64, text string, since time.Time) (*messageOut, int, error) {
	q := m.messageSelectSQL() + ` WHERE m.is_from_me = 1 AND m.date >= ?`
	args := []any{toAppleTime(since)}
	if chatID > 0 {
		q += ` AND cmj.chat_id = ?`
		args = append(args, chatID)
	}
	q += ` ORDER BY m.date DESC, m.ROWID DESC LIMIT 10`
	want := strings.TrimSpace(text)
	deadline := time.Now().Add(m.confirmWait)
	for {
		msgs, err := m.queryMessages(ctx, q, args...)
		if err != nil {
			return nil, 0, err
		}
		for i := range msgs {
			if strings.TrimSpace(msgs[i].Text) == want {
				var code int
				if err := m.db.QueryRowContext(ctx, `SELECT COALESCE(error, 0) FROM message WHERE ROWID = ?`, msgs[i].ID).Scan(&code); err != nil {
					return nil, 0, err
				}
				return &msgs[i], code, nil
			}
		}
		if !time.Now().Before(deadline) {
			return nil, 0, nil
		}
		select {
		case <-ctx.Done():
			return nil, 0, nil
		case <-time.After(confirmPollInterval):
		}
	}
}
