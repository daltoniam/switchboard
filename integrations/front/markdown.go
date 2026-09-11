package front

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

type renderedConversation struct {
	ID        string
	Subject   string
	Status    string
	Assignee  string
	Recipient string
	Created   string
}

type renderedMessage struct {
	ID        string
	Type      string
	Inbound   bool
	CreatedAt string
	Author    string
	Subject   string
	Body      markdown.Markdown
}

var markdownRenderers = map[mcp.ToolName]func([]byte) (markdown.Markdown, bool){
	"front_get_conversation":           renderConversationMD,
	"front_list_conversation_messages": renderMessagesMD,
}

func (f *front) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	if fn, ok := markdownRenderers[toolName]; ok {
		return fn(data)
	}
	return "", false
}

type rawConversationResponse struct {
	ID       string `json:"id"`
	Subject  string `json:"subject"`
	Status   string `json:"status"`
	Created  any    `json:"created_at"`
	Assignee struct {
		Email     string `json:"email"`
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	} `json:"assignee"`
	Recipient struct {
		Name   string `json:"name"`
		Handle string `json:"handle"`
	} `json:"recipient"`
}

type rawMessageListResponse struct {
	Results []rawMessage `json:"_results"`
}

type rawMessage struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	IsInbound bool   `json:"is_inbound"`
	CreatedAt any    `json:"created_at"`
	Subject   string `json:"subject"`
	Text      string `json:"text"`
	Body      string `json:"body"`
	Author    struct {
		Email     string `json:"email"`
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	} `json:"author"`
}

func renderConversationMD(data []byte) (markdown.Markdown, bool) {
	var raw rawConversationResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}
	conv := renderedConversation{
		ID:        raw.ID,
		Subject:   raw.Subject,
		Status:    raw.Status,
		Assignee:  personLabel(raw.Assignee.FirstName, raw.Assignee.LastName, raw.Assignee.Username, raw.Assignee.Email),
		Recipient: personLabel("", "", raw.Recipient.Name, raw.Recipient.Handle),
		Created:   unixString(raw.Created),
	}
	return conversationToMarkdown(conv), true
}

func renderMessagesMD(data []byte) (markdown.Markdown, bool) {
	var raw rawMessageListResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}
	msgs := make([]renderedMessage, 0, len(raw.Results))
	for _, m := range raw.Results {
		body := m.Text
		if body == "" {
			body = string(markdown.FromHTML(m.Body))
		}
		msgs = append(msgs, renderedMessage{
			ID:        m.ID,
			Type:      m.Type,
			Inbound:   m.IsInbound,
			CreatedAt: unixString(m.CreatedAt),
			Author:    personLabel(m.Author.FirstName, m.Author.LastName, m.Author.Username, m.Author.Email),
			Subject:   m.Subject,
			Body:      markdown.Markdown(body),
		})
	}
	return messagesToMarkdown(msgs), true
}

func conversationToMarkdown(conv renderedConversation) markdown.Markdown {
	b := markdown.NewBuilder()
	b.Metadata("front", "conversation_id", conv.ID, "status", conv.Status)
	title := conv.Subject
	if title == "" {
		title = "Conversation " + conv.ID
	}
	b.Heading(1, title)
	attrs := []string{}
	if conv.Status != "" {
		attrs = append(attrs, "Status: "+conv.Status)
	}
	if conv.Recipient != "" {
		attrs = append(attrs, "Recipient: "+conv.Recipient)
	}
	if conv.Assignee != "" {
		attrs = append(attrs, "Assignee: "+conv.Assignee)
	}
	if conv.Created != "" {
		attrs = append(attrs, "Created: "+conv.Created)
	}
	if len(attrs) > 0 {
		b.Attribution(attrs...)
	}
	return b.Build()
}

func messagesToMarkdown(msgs []renderedMessage) markdown.Markdown {
	b := markdown.NewBuilder()
	b.Heading(1, fmt.Sprintf("Messages (%d)", len(msgs)))
	if len(msgs) == 0 {
		b.BlankLine()
		b.Raw("No messages.\n")
		return b.Build()
	}
	b.BlankLine()
	for _, m := range msgs {
		author := m.Author
		if author == "" {
			author = "unknown"
		}
		dir := "outbound"
		if m.Inbound {
			dir = "inbound"
		}
		context := dir
		if m.Type != "" {
			context = m.Type + ", " + dir
		}
		if m.CreatedAt != "" {
			context += ", " + m.CreatedAt
		}
		body := strings.TrimRight(string(m.Body), "\n")
		if body == "" {
			body = "(empty)"
		}
		b.CommentAttribution(author, context, body)
	}
	return b.Build()
}

func personLabel(first, last, username, email string) string {
	name := strings.TrimSpace(first + " " + last)
	if name == "" {
		name = username
	}
	switch {
	case name != "" && email != "":
		return name + " <" + email + ">"
	case name != "":
		return name
	default:
		return email
	}
}

func unixString(v any) string {
	switch n := v.(type) {
	case float64:
		if n == 0 {
			return ""
		}
		return time.Unix(int64(n), 0).UTC().Format(time.RFC3339)
	case json.Number:
		f, err := n.Float64()
		if err != nil || f == 0 {
			return ""
		}
		return time.Unix(int64(f), 0).UTC().Format(time.RFC3339)
	case int64:
		if n == 0 {
			return ""
		}
		return time.Unix(n, 0).UTC().Format(time.RFC3339)
	case string:
		if n == "" || n == "0" {
			return ""
		}
		if f, err := strconv.ParseFloat(n, 64); err == nil {
			return time.Unix(int64(f), 0).UTC().Format(time.RFC3339)
		}
		return n
	default:
		return ""
	}
}
