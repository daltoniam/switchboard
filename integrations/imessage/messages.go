package imessage

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

// visibleFilter excludes tapback/reaction rows and group system events
// (renames, joins) so transcripts contain only real messages.
const visibleFilter = `COALESCE(m.associated_message_type, 0) NOT BETWEEN 2000 AND 3999 AND COALESCE(m.item_type, 0) = 0`

const groupChatStyle = 43

var reactionNames = map[int]string{
	0: "love", 1: "like", 2: "dislike", 3: "laugh",
	4: "emphasize", 5: "question", 6: "emoji", 7: "sticker",
}

type participant struct {
	Handle string `json:"handle"`
	Name   string `json:"name,omitempty"`
}

type chatSummary struct {
	ChatID        int64         `json:"chat_id"`
	GUID          string        `json:"guid"`
	Name          string        `json:"name"`
	Identifier    string        `json:"identifier"`
	Service       string        `json:"service"`
	IsGroup       bool          `json:"is_group"`
	Participants  []participant `json:"participants"`
	LastMessageAt string        `json:"last_message_at"`
	LastMessage   string        `json:"last_message,omitempty"`
	LastFromMe    bool          `json:"last_message_from_me"`
	UnreadCount   int           `json:"unread_count"`
}

type listChatsResponse struct {
	Chats   []chatSummary `json:"chats"`
	Total   int           `json:"total"`
	HasMore bool          `json:"has_more"`
}

type attachmentOut struct {
	Name     string `json:"name,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`
	Bytes    int64  `json:"bytes,omitempty"`
	Path     string `json:"path,omitempty"`
}

type reaction struct {
	Type   string `json:"type"`
	Emoji  string `json:"emoji,omitempty"`
	By     string `json:"by"`
	ByName string `json:"by_name,omitempty"`
}

type messageOut struct {
	ID          int64           `json:"id"`
	GUID        string          `json:"guid"`
	ChatID      int64           `json:"chat_id,omitempty"`
	ChatName    string          `json:"chat_name,omitempty"`
	Date        string          `json:"date"`
	FromMe      bool            `json:"from_me"`
	Sender      string          `json:"sender"`
	SenderName  string          `json:"sender_name,omitempty"`
	Text        string          `json:"text"`
	Service     string          `json:"service,omitempty"`
	ReplyTo     string          `json:"reply_to,omitempty"`
	Edited      bool            `json:"edited,omitempty"`
	Unsent      bool            `json:"unsent,omitempty"`
	Attachments []attachmentOut `json:"attachments,omitempty"`
	Reactions   []reaction      `json:"reactions,omitempty"`

	rawDate        int64
	hasAttachments bool
}

type chatMessagesResponse struct {
	ChatIDs      []int64       `json:"chat_ids"`
	Name         string        `json:"name"`
	Participants []participant `json:"participants"`
	Messages     []messageOut  `json:"messages"`
	HasMore      bool          `json:"has_more"`
	NextBefore   string        `json:"next_before,omitempty"`
}

type messagesResponse struct {
	Count    int          `json:"count"`
	Messages []messageOut `json:"messages"`
}

type contactsResponse struct {
	Count    int       `json:"count"`
	Contacts []contact `json:"contacts"`
}

type chatRow struct {
	id       int64
	guid     string
	ident    string
	display  string
	service  string
	style    int64
	lastDate int64
}

func clamp(v, lo, hi int) int {
	return max(lo, min(v, hi))
}

func inClause(ids []int64) (string, []any) {
	ph := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		ph[i] = "?"
		args[i] = id
	}
	return "(" + strings.Join(ph, ",") + ")", args
}

func (m *imessage) messageSelectSQL() string {
	return `SELECT m.ROWID, COALESCE(m.guid, ''), COALESCE(m.text, ''), m.attributedBody, COALESCE(m.is_from_me, 0),
  COALESCE(h.id, ''), COALESCE(m.service, ''), COALESCE(m.date, 0), COALESCE(cmj.chat_id, 0),
  COALESCE(` + m.msgCol("thread_originator_guid", "''") + `, ''),
  COALESCE(` + m.msgCol("date_edited", "0") + `, 0),
  COALESCE(` + m.msgCol("date_retracted", "0") + `, 0),
  COALESCE(` + m.msgCol("cache_has_attachments", "0") + `, 0)
FROM message m
LEFT JOIN handle h ON h.ROWID = m.handle_id
LEFT JOIN chat_message_join cmj ON cmj.message_id = m.ROWID`
}

func (m *imessage) queryMessages(ctx context.Context, query string, args ...any) ([]messageOut, error) {
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	msgs := []messageOut{}
	for rows.Next() {
		var (
			msg                       messageOut
			text, handle              string
			body                      []byte
			fromMe, hasAtt            int64
			dateEdited, dateRetracted int64
		)
		if err := rows.Scan(&msg.ID, &msg.GUID, &text, &body, &fromMe, &handle, &msg.Service, &msg.rawDate,
			&msg.ChatID, &msg.ReplyTo, &dateEdited, &dateRetracted, &hasAtt); err != nil {
			return nil, err
		}
		msg.Text = messageBody(text, body)
		msg.FromMe = fromMe != 0
		msg.Date = formatTime(appleTime(msg.rawDate))
		msg.Edited = dateEdited != 0
		msg.Unsent = dateRetracted != 0
		msg.hasAttachments = hasAtt != 0
		if msg.FromMe {
			msg.Sender = "me"
		} else {
			msg.Sender = handle
			msg.SenderName = m.contacts.name(ctx, handle)
		}
		msgs = append(msgs, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := m.attachAttachments(ctx, msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

func (m *imessage) attachAttachments(ctx context.Context, msgs []messageOut) error {
	idx := map[int64]int{}
	var ids []int64
	for i, msg := range msgs {
		if msg.hasAttachments {
			idx[msg.ID] = i
			ids = append(ids, msg.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	in, args := inClause(ids)
	query := `
SELECT maj.message_id, COALESCE(a.transfer_name, ''), COALESCE(a.mime_type, ''), COALESCE(a.total_bytes, 0), COALESCE(a.filename, '')
FROM message_attachment_join maj JOIN attachment a ON a.ROWID = maj.attachment_id
WHERE maj.message_id IN ` + in + ` ORDER BY maj.message_id, a.ROWID` // #nosec G202 -- in is only "?" placeholders
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var a attachmentOut
		var filename string
		if err := rows.Scan(&id, &a.Name, &a.MIMEType, &a.Bytes, &filename); err != nil {
			return err
		}
		if filename != "" {
			a.Path = expandHome(filename)
		}
		if a.Name == "" && filename != "" {
			a.Name = filepath.Base(filename)
		}
		msgs[idx[id]].Attachments = append(msgs[idx[id]].Attachments, a)
	}
	return rows.Err()
}

func stripAssocPrefix(g string) string {
	if i := strings.Index(g, "/"); i >= 0 {
		return g[i+1:]
	}
	return strings.TrimPrefix(g, "bp:")
}

// attachReactions folds tapback rows into the messages they target. Each
// person keeps at most one reaction per message; a 3xxx row removes the
// matching 2xxx reaction.
func (m *imessage) attachReactions(ctx context.Context, msgs []messageOut, chatIDs []int64) error {
	if len(msgs) == 0 {
		return nil
	}
	byGUID := map[string]int{}
	minDate := msgs[0].rawDate
	for i, msg := range msgs {
		byGUID[msg.GUID] = i
		minDate = min(minDate, msg.rawDate)
	}
	in, args := inClause(chatIDs)
	args = append(args, minDate)
	query := `
SELECT COALESCE(m.associated_message_guid, ''), m.associated_message_type, COALESCE(` + m.msgCol("associated_message_emoji", "''") + `, ''),
  COALESCE(m.is_from_me, 0), COALESCE(h.id, '')
FROM message m
JOIN chat_message_join cmj ON cmj.message_id = m.ROWID
LEFT JOIN handle h ON h.ROWID = m.handle_id
WHERE cmj.chat_id IN ` + in + ` AND m.associated_message_type BETWEEN 2000 AND 3999 AND m.date >= ?
ORDER BY m.date, m.ROWID` // #nosec G202 -- in is only "?" placeholders; column names are fixed literals
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var guid, emoji, handle string
		var typ, fromMe int64
		if err := rows.Scan(&guid, &typ, &emoji, &fromMe, &handle); err != nil {
			return err
		}
		i, ok := byGUID[stripAssocPrefix(guid)]
		if !ok {
			continue
		}
		name, known := reactionNames[int(typ%1000)]
		if !known {
			continue
		}
		r := reaction{Type: name, By: handle}
		if fromMe != 0 {
			r.By = "me"
		} else {
			r.ByName = m.contacts.name(ctx, handle)
		}
		if name == "emoji" {
			r.Emoji = emoji
		}
		existing := msgs[i].Reactions
		kept := existing[:0]
		for _, e := range existing {
			if e.By != r.By {
				kept = append(kept, e)
			} else if typ >= 3000 && e.Type != r.Type {
				kept = append(kept, e)
			}
		}
		if typ < 3000 {
			kept = append(kept, r)
		}
		msgs[i].Reactions = kept
	}
	return rows.Err()
}

func (m *imessage) loadChats(ctx context.Context, ids []int64) ([]chatRow, error) {
	where := ""
	var args []any
	if ids != nil {
		in, a := inClause(ids)
		where = "WHERE c.ROWID IN " + in
		args = a
	}
	rows, err := m.db.QueryContext(ctx, `
SELECT c.ROWID, COALESCE(c.guid, ''), COALESCE(c.chat_identifier, ''), COALESCE(c.display_name, ''),
  COALESCE(c.service_name, ''), COALESCE(c.style, 0), COALESCE(MAX(cmj.message_date), 0)
FROM chat c LEFT JOIN chat_message_join cmj ON cmj.chat_id = c.ROWID
`+where+`
GROUP BY c.ROWID ORDER BY 7 DESC, c.ROWID DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []chatRow
	for rows.Next() {
		var c chatRow
		if err := rows.Scan(&c.id, &c.guid, &c.ident, &c.display, &c.service, &c.style, &c.lastDate); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (m *imessage) loadParticipants(ctx context.Context, ids []int64) (map[int64][]participant, error) {
	where := ""
	var args []any
	if ids != nil {
		in, a := inClause(ids)
		where = "WHERE chj.chat_id IN " + in
		args = a
	}
	rows, err := m.db.QueryContext(ctx, `
SELECT chj.chat_id, COALESCE(h.id, '')
FROM chat_handle_join chj JOIN handle h ON h.ROWID = chj.handle_id
`+where+`
ORDER BY chj.chat_id, h.ROWID`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[int64][]participant{}
	for rows.Next() {
		var id int64
		var handle string
		if err := rows.Scan(&id, &handle); err != nil {
			return nil, err
		}
		out[id] = append(out[id], participant{Handle: handle, Name: m.contacts.name(ctx, handle)})
	}
	return out, rows.Err()
}

func (m *imessage) unreadCounts(ctx context.Context) (map[int64]int, error) {
	rows, err := m.db.QueryContext(ctx, `
SELECT cmj.chat_id, COUNT(*)
FROM message m JOIN chat_message_join cmj ON cmj.message_id = m.ROWID
WHERE m.is_read = 0 AND m.is_from_me = 0 AND `+visibleFilter+`
GROUP BY cmj.chat_id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[int64]int{}
	for rows.Next() {
		var id int64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

func chatName(c chatRow, parts []participant) string {
	if strings.TrimSpace(c.display) != "" {
		return c.display
	}
	names := make([]string, 0, len(parts))
	for _, p := range parts {
		if p.Name != "" {
			names = append(names, p.Name)
		} else {
			names = append(names, p.Handle)
		}
	}
	if len(names) > 0 {
		return strings.Join(names, ", ")
	}
	return c.ident
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func chatMatches(query string, c chatRow, name string, parts []participant) bool {
	if strings.TrimSpace(query) == "" {
		return true
	}
	fields := []string{name, c.ident}
	handles := []string{c.ident}
	for _, p := range parts {
		fields = append(fields, p.Handle, p.Name)
		handles = append(handles, p.Handle)
	}
	return textMatches(query, fields, handles)
}

// textMatches reports whether query is a case-insensitive substring of any
// field, or (for phone-like queries) a digit substring of any handle.
func textMatches(query string, fields, handles []string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	qNorm := normalizeHandle(query)
	if isDigits(qNorm) && len(qNorm) >= 4 {
		for _, h := range handles {
			if strings.Contains(normalizeHandle(h), qNorm) {
				return true
			}
		}
	}
	return false
}

// isDirectMatch reports whether c is a 1:1 conversation with the person the
// query names, so searching for someone surfaces their direct thread ahead of
// the (often far more numerous and more recent) group chats they belong to.
func isDirectMatch(query string, c chatRow, parts []participant) bool {
	if strings.TrimSpace(query) == "" || c.style == groupChatStyle {
		return false
	}
	fields := []string{c.ident}
	handles := []string{c.ident}
	for _, p := range parts {
		fields = append(fields, p.Handle, p.Name)
		handles = append(handles, p.Handle)
	}
	return textMatches(query, fields, handles)
}

func listChats(ctx context.Context, m *imessage, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	limit := clamp(r.OptInt("limit", 20), 1, 100)
	offset := max(r.OptInt("offset", 0), 0)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	chats, err := m.loadChats(ctx, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	parts, err := m.loadParticipants(ctx, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	unread, err := m.unreadCounts(ctx)
	if err != nil {
		return mcp.ErrResult(err)
	}

	var direct, others []chatSummary
	for _, c := range chats {
		if c.lastDate == 0 {
			continue
		}
		name := chatName(c, parts[c.id])
		if !chatMatches(query, c, name, parts[c.id]) {
			continue
		}
		p := parts[c.id]
		if p == nil {
			p = []participant{}
		}
		summary := chatSummary{
			ChatID:        c.id,
			GUID:          c.guid,
			Name:          name,
			Identifier:    c.ident,
			Service:       c.service,
			IsGroup:       c.style == groupChatStyle,
			Participants:  p,
			LastMessageAt: formatTime(appleTime(c.lastDate)),
			UnreadCount:   unread[c.id],
		}
		if isDirectMatch(query, c, parts[c.id]) {
			direct = append(direct, summary)
		} else {
			others = append(others, summary)
		}
	}
	matched := append(direct, others...)

	resp := listChatsResponse{Chats: []chatSummary{}, Total: len(matched)}
	if offset < len(matched) {
		end := min(offset+limit, len(matched))
		resp.Chats = matched[offset:end]
		resp.HasMore = end < len(matched)
	}
	lastQuery := m.messageSelectSQL() + ` WHERE cmj.chat_id = ? AND ` + visibleFilter + ` ORDER BY cmj.message_date DESC, m.ROWID DESC LIMIT 1`
	for i := range resp.Chats {
		msgs, err := m.queryMessages(ctx, lastQuery, resp.Chats[i].ChatID)
		if err != nil {
			return mcp.ErrResult(err)
		}
		if len(msgs) > 0 {
			resp.Chats[i].LastMessage = truncate(msgs[0].Text, 200)
			resp.Chats[i].LastFromMe = msgs[0].FromMe
		}
	}
	return mcp.JSONResult(resp)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func (m *imessage) findDirectChats(ctx context.Context, handle string) ([]int64, error) {
	if strings.TrimSpace(handle) == "" {
		return nil, nil
	}
	chats, err := m.loadChats(ctx, nil)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for _, c := range chats {
		if c.style != groupChatStyle && sameHandle(c.ident, handle) {
			ids = append(ids, c.id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func getChatMessages(ctx context.Context, m *imessage, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	chatID := r.Int64("chat_id")
	handle := r.Str("handle")
	limit := clamp(r.OptInt("limit", 30), 1, 200)
	beforeStr := r.Str("before")
	sinceStr := r.Str("since")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	before, err := parseTimeArg("before", beforeStr)
	if err != nil {
		return mcp.ErrResult(err)
	}
	since, err := parseTimeArg("since", sinceStr)
	if err != nil {
		return mcp.ErrResult(err)
	}

	var ids []int64
	switch {
	case chatID > 0:
		ids = []int64{chatID}
	case strings.TrimSpace(handle) != "":
		ids, err = m.findDirectChats(ctx, handle)
		if err != nil {
			return mcp.ErrResult(err)
		}
		if len(ids) == 0 {
			return mcp.ErrResult(fmt.Errorf("no conversation found with %s; use imessage_list_chats to find a chat_id", handle))
		}
	default:
		return mcp.ErrResult(fmt.Errorf("chat_id or handle is required"))
	}

	chats, err := m.loadChats(ctx, ids)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if len(chats) == 0 {
		return mcp.ErrResult(fmt.Errorf("chat %d not found; use imessage_list_chats to find a chat_id", chatID))
	}
	parts, err := m.loadParticipants(ctx, ids)
	if err != nil {
		return mcp.ErrResult(err)
	}

	in, qargs := inClause(ids)
	q := m.messageSelectSQL() + ` WHERE cmj.chat_id IN ` + in + ` AND ` + visibleFilter
	if !before.IsZero() {
		q += ` AND m.date < ?`
		qargs = append(qargs, toAppleTime(before))
	}
	if !since.IsZero() {
		q += ` AND m.date >= ?`
		qargs = append(qargs, toAppleTime(since))
	}
	q += ` ORDER BY m.date DESC, m.ROWID DESC LIMIT ?`
	qargs = append(qargs, limit+1)
	msgs, err := m.queryMessages(ctx, q, qargs...)
	if err != nil {
		return mcp.ErrResult(err)
	}

	resp := chatMessagesResponse{ChatIDs: ids, Participants: []participant{}}
	if len(msgs) > limit {
		msgs = msgs[:limit]
		resp.HasMore = true
	}
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	if err := m.attachReactions(ctx, msgs, ids); err != nil {
		return mcp.ErrResult(err)
	}
	if resp.HasMore && len(msgs) > 0 {
		resp.NextBefore = appleTime(msgs[0].rawDate).Local().Format(time.RFC3339Nano)
	}
	resp.Messages = msgs

	seen := map[string]bool{}
	for _, c := range chats {
		for _, p := range parts[c.id] {
			if !seen[p.Handle] {
				seen[p.Handle] = true
				resp.Participants = append(resp.Participants, p)
			}
		}
	}
	resp.Name = chatName(chats[0], parts[chats[0].id])
	return mcp.JSONResult(resp)
}

func (m *imessage) attachChatNames(ctx context.Context, msgs []messageOut) error {
	seen := map[int64]bool{}
	var ids []int64
	for _, msg := range msgs {
		if msg.ChatID != 0 && !seen[msg.ChatID] {
			seen[msg.ChatID] = true
			ids = append(ids, msg.ChatID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	chats, err := m.loadChats(ctx, ids)
	if err != nil {
		return err
	}
	parts, err := m.loadParticipants(ctx, ids)
	if err != nil {
		return err
	}
	names := map[int64]string{}
	for _, c := range chats {
		names[c.id] = chatName(c, parts[c.id])
	}
	for i := range msgs {
		msgs[i].ChatName = names[msgs[i].ChatID]
	}
	return nil
}

// chatsWithHandle returns every chat (1:1 or group) the person is part of, so
// a handle-scoped search covers the whole conversation, including messages
// sent by me (which carry no sender handle).
func (m *imessage) chatsWithHandle(ctx context.Context, handle string) ([]int64, error) {
	rows, err := m.db.QueryContext(ctx, `
SELECT chj.chat_id, COALESCE(h.id, '') FROM chat_handle_join chj JOIN handle h ON h.ROWID = chj.handle_id
UNION
SELECT ROWID, COALESCE(chat_identifier, '') FROM chat`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	seen := map[int64]bool{}
	var ids []int64
	for rows.Next() {
		var id int64
		var h string
		if err := rows.Scan(&id, &h); err != nil {
			return nil, err
		}
		if !seen[id] && sameHandle(h, handle) {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

// liveFilter hides messages that are not in any conversation: ones in the
// Recently Deleted folder and orphans left behind by deleted chats.
func (m *imessage) liveFilter() string {
	f := ` AND cmj.chat_id IS NOT NULL`
	if m.hasRecoverable {
		f += ` AND NOT EXISTS (SELECT 1 FROM chat_recoverable_message_join r WHERE r.message_id = m.ROWID)`
	}
	return f
}

func messagesResult(ctx context.Context, m *imessage, msgs []messageOut) (*mcp.ToolResult, error) {
	if err := m.attachChatNames(ctx, msgs); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(messagesResponse{Count: len(msgs), Messages: msgs})
}

func searchMessages(ctx context.Context, m *imessage, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	chatID := r.Int64("chat_id")
	handle := r.Str("handle")
	sinceStr := r.Str("since")
	beforeStr := r.Str("before")
	limit := clamp(r.OptInt("limit", 20), 1, 100)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return mcp.ErrResult(fmt.Errorf("query is required"))
	}
	since, err := parseTimeArg("since", sinceStr)
	if err != nil {
		return mcp.ErrResult(err)
	}
	before, err := parseTimeArg("before", beforeStr)
	if err != nil {
		return mcp.ErrResult(err)
	}

	q := m.messageSelectSQL() + ` WHERE ` + visibleFilter + m.liveFilter()
	var qargs []any
	if chatID > 0 {
		q += ` AND cmj.chat_id = ?`
		qargs = append(qargs, chatID)
	}
	if strings.TrimSpace(handle) != "" {
		ids, err := m.chatsWithHandle(ctx, handle)
		if err != nil {
			return mcp.ErrResult(err)
		}
		if len(ids) == 0 {
			return messagesResult(ctx, m, []messageOut{})
		}
		in, a := inClause(ids)
		q += ` AND cmj.chat_id IN ` + in
		qargs = append(qargs, a...)
	}
	if !since.IsZero() {
		q += ` AND m.date >= ?`
		qargs = append(qargs, toAppleTime(since))
	}
	if !before.IsZero() {
		q += ` AND m.date < ?`
		qargs = append(qargs, toAppleTime(before))
	}
	q += ` AND ` + matchFuncName + `(m.text, m.attributedBody, ?) = 1 ORDER BY m.date DESC, m.ROWID DESC LIMIT ?`
	qargs = append(qargs, needle, limit)

	msgs, err := m.queryMessages(ctx, q, qargs...)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return messagesResult(ctx, m, msgs)
}

func listUnread(ctx context.Context, m *imessage, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	limit := clamp(r.OptInt("limit", 50), 1, 200)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := m.messageSelectSQL() + ` WHERE m.is_read = 0 AND m.is_from_me = 0 AND ` + visibleFilter + m.liveFilter() +
		` ORDER BY m.date DESC, m.ROWID DESC LIMIT ?`
	msgs, err := m.queryMessages(ctx, q, limit)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return messagesResult(ctx, m, msgs)
}

func lookupContact(ctx context.Context, m *imessage, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	limit := clamp(r.OptInt("limit", 10), 1, 50)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if strings.TrimSpace(query) == "" {
		return mcp.ErrResult(fmt.Errorf("query is required"))
	}
	found, err := m.contacts.search(ctx, query, limit)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if found == nil {
		found = []contact{}
	}
	return mcp.JSONResult(contactsResponse{Count: len(found), Contacts: found})
}
