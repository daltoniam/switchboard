package gchat

import (
	"encoding/json"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

// ── Parse boundary ──────────────────────────────────────────────────

var markdownRenderers = map[mcp.ToolName]func([]byte) (markdown.Markdown, bool){
	"gchat_list_spaces":   renderSpacesMD,
	"gchat_get_space":     renderSpaceMD,
	"gchat_list_messages": renderMessagesMD,
	"gchat_get_message":   renderMessageMD,
	"gchat_list_members":  renderMembersMD,
}

func (g *gchat) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	if fn, ok := markdownRenderers[toolName]; ok {
		return fn(data)
	}
	return "", false
}

// ── Raw types ───────────────────────────────────────────────────────

type rawSpacesPage struct {
	Spaces        []rawSpace `json:"spaces"`
	NextPageToken string     `json:"nextPageToken"`
}

type rawSpace struct {
	Name                string `json:"name"`
	DisplayName         string `json:"displayName"`
	Type                string `json:"type"`
	SpaceType           string `json:"spaceType"`
	SingleUserBotDm     bool   `json:"singleUserBotDm"`
	ExternalUserAllowed bool   `json:"externalUserAllowed"`
	SpaceThreadingState string `json:"spaceThreadingState"`
	SpaceHistoryState   string `json:"spaceHistoryState"`
	SpaceDetails        struct {
		Description string `json:"description"`
		Guidelines  string `json:"guidelines"`
	} `json:"spaceDetails"`
}

type rawMessagesPage struct {
	Messages      []rawMessage `json:"messages"`
	NextPageToken string       `json:"nextPageToken"`
}

type rawMessage struct {
	Name           string    `json:"name"`
	Sender         rawSender `json:"sender"`
	CreateTime     string    `json:"createTime"`
	LastUpdateTime string    `json:"lastUpdateTime"`
	DeleteTime     string    `json:"deleteTime"`
	Text           string    `json:"text"`
	Thread         struct {
		Name string `json:"name"`
	} `json:"thread"`
	Space struct {
		Name string `json:"name"`
	} `json:"space"`
	Attachment       []json.RawMessage `json:"attachment"`
	DeletionMetadata json.RawMessage   `json:"deletionMetadata"`
}

type rawSender struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Type        string `json:"type"`
	DomainID    string `json:"domainId"`
}

type rawMembersPage struct {
	Memberships   []rawMembership `json:"memberships"`
	NextPageToken string          `json:"nextPageToken"`
}

type rawMembership struct {
	Name       string `json:"name"`
	State      string `json:"state"`
	Role       string `json:"role"`
	CreateTime string `json:"createTime"`
	Member     struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		Type        string `json:"type"`
		DomainID    string `json:"domainId"`
	} `json:"member"`
}

// ── Spaces list ─────────────────────────────────────────────────────

func renderSpacesMD(data []byte) (markdown.Markdown, bool) {
	var page rawSpacesPage
	if err := json.Unmarshal(data, &page); err != nil {
		return "", false
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return "", false
	}
	if _, hasSpaces := probe["spaces"]; !hasSpaces {
		if _, hasToken := probe["nextPageToken"]; !hasToken {
			return "", false
		}
	}

	b := markdown.NewBuilder()
	b.Heading(1, "Chat Spaces")
	if len(page.Spaces) == 0 {
		b.BlankLine()
		b.Raw("_No spaces._\n")
		return b.Build(), true
	}

	var sb strings.Builder
	rows := [][]string{{"Display Name", "Type", "Resource Name"}}
	for _, s := range page.Spaces {
		name := s.DisplayName
		if name == "" {
			name = "(no name)"
		}
		typ := s.SpaceType
		if typ == "" {
			typ = s.Type
		}
		rows = append(rows, []string{
			pipeSafe(name),
			pipeSafe(typ),
			pipeSafe(s.Name),
		})
	}
	markdown.WriteTable(&sb, rows)
	b.Raw(sb.String())

	if page.NextPageToken != "" {
		b.BlankLine()
		b.Attribution("next_page_token: " + page.NextPageToken)
	}
	return b.Build(), true
}

// ── Single space ────────────────────────────────────────────────────

func renderSpaceMD(data []byte) (markdown.Markdown, bool) {
	var s rawSpace
	if err := json.Unmarshal(data, &s); err != nil {
		return "", false
	}
	if s.Name == "" {
		return "", false
	}

	b := markdown.NewBuilder()
	title := s.DisplayName
	if title == "" {
		title = "(unnamed space)"
	}
	b.Heading(1, title)

	var attrs []string
	typ := s.SpaceType
	if typ == "" {
		typ = s.Type
	}
	if typ != "" {
		attrs = append(attrs, "type: "+typ)
	}
	if s.SpaceThreadingState != "" {
		attrs = append(attrs, "threading: "+s.SpaceThreadingState)
	}
	if s.SpaceHistoryState != "" {
		attrs = append(attrs, "history: "+s.SpaceHistoryState)
	}
	attrs = append(attrs, "name="+s.Name)
	b.Attribution(attrs...)

	if s.SpaceDetails.Description != "" {
		b.BlankLine()
		b.Heading(2, "Description")
		b.Raw(s.SpaceDetails.Description)
		if !strings.HasSuffix(s.SpaceDetails.Description, "\n") {
			b.Raw("\n")
		}
	}
	if s.SpaceDetails.Guidelines != "" {
		b.BlankLine()
		b.Heading(2, "Guidelines")
		b.Raw(s.SpaceDetails.Guidelines)
		if !strings.HasSuffix(s.SpaceDetails.Guidelines, "\n") {
			b.Raw("\n")
		}
	}

	return b.Build(), true
}

// ── Messages list ───────────────────────────────────────────────────

func renderMessagesMD(data []byte) (markdown.Markdown, bool) {
	var page rawMessagesPage
	if err := json.Unmarshal(data, &page); err != nil {
		return "", false
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return "", false
	}
	if _, hasMessages := probe["messages"]; !hasMessages {
		if _, hasToken := probe["nextPageToken"]; !hasToken {
			return "", false
		}
	}

	b := markdown.NewBuilder()
	b.Heading(1, "Chat Messages")
	if len(page.Messages) == 0 {
		b.BlankLine()
		b.Raw("_No messages._\n")
		return b.Build(), true
	}

	for _, m := range page.Messages {
		writeMessage(b, m)
	}

	if page.NextPageToken != "" {
		b.Attribution("next_page_token: " + page.NextPageToken)
	}
	return b.Build(), true
}

// ── Single message ──────────────────────────────────────────────────

func renderMessageMD(data []byte) (markdown.Markdown, bool) {
	var m rawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return "", false
	}
	if m.Name == "" {
		return "", false
	}

	b := markdown.NewBuilder()
	author := m.Sender.DisplayName
	if author == "" {
		author = m.Sender.Name
	}
	if author == "" {
		author = "(unknown sender)"
	}
	b.Heading(1, "Message from "+author)

	var attrs []string
	if m.CreateTime != "" {
		attrs = append(attrs, "created: "+m.CreateTime)
	}
	if m.LastUpdateTime != "" && m.LastUpdateTime != m.CreateTime {
		attrs = append(attrs, "updated: "+m.LastUpdateTime)
	}
	if m.DeleteTime != "" {
		attrs = append(attrs, "deleted: "+m.DeleteTime)
	}
	if m.Thread.Name != "" {
		attrs = append(attrs, "thread: "+m.Thread.Name)
	}
	attrs = append(attrs, "name="+m.Name)
	b.Attribution(attrs...)

	if m.Text != "" {
		b.BlankLine()
		b.Raw(m.Text)
		if !strings.HasSuffix(m.Text, "\n") {
			b.Raw("\n")
		}
	}
	if len(m.Attachment) > 0 {
		b.BlankLine()
		b.Attribution("attachments: " + strconv.Itoa(len(m.Attachment)))
	}

	return b.Build(), true
}

// writeMessage renders one message as a blockquoted comment-style block
// within a list. Used by the message-list renderer.
func writeMessage(b *markdown.Builder, m rawMessage) {
	author := m.Sender.DisplayName
	if author == "" {
		author = m.Sender.Name
	}
	if author == "" {
		author = "(unknown)"
	}
	ts := m.CreateTime
	if m.LastUpdateTime != "" && m.LastUpdateTime != m.CreateTime {
		ts = ts + " (edited " + m.LastUpdateTime + ")"
	}
	text := m.Text
	if text == "" && m.DeleteTime != "" {
		text = "_[deleted]_"
	} else if text == "" {
		text = "_(no text)_"
	}
	b.BlockquoteAttribution(author, ts, text)
}

// ── Members list ────────────────────────────────────────────────────

func renderMembersMD(data []byte) (markdown.Markdown, bool) {
	var page rawMembersPage
	if err := json.Unmarshal(data, &page); err != nil {
		return "", false
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return "", false
	}
	if _, hasM := probe["memberships"]; !hasM {
		if _, hasToken := probe["nextPageToken"]; !hasToken {
			return "", false
		}
	}

	b := markdown.NewBuilder()
	b.Heading(1, "Space Members")
	if len(page.Memberships) == 0 {
		b.BlankLine()
		b.Raw("_No members._\n")
		return b.Build(), true
	}

	var sb strings.Builder
	rows := [][]string{{"Display Name", "Type", "Role", "State", "Resource Name"}}
	for _, m := range page.Memberships {
		name := m.Member.DisplayName
		if name == "" {
			name = m.Member.Name
		}
		rows = append(rows, []string{
			pipeSafe(name),
			pipeSafe(m.Member.Type),
			pipeSafe(m.Role),
			pipeSafe(m.State),
			pipeSafe(m.Name),
		})
	}
	markdown.WriteTable(&sb, rows)
	b.Raw(sb.String())

	if page.NextPageToken != "" {
		b.BlankLine()
		b.Attribution("next_page_token: " + page.NextPageToken)
	}
	return b.Build(), true
}

// ── Helpers ─────────────────────────────────────────────────────────

// pipeSafe escapes newlines and pipes so a cell stays on one row.
func pipeSafe(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}
