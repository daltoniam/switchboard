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
	"front_list_conversation_messages": renderMessagesMD,
	"front_list_conversation_comments": renderCommentsMD,
}

func (f *front) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	if fn, ok := markdownRenderers[toolName]; ok {
		return fn(data)
	}
	return "", false
}

type rawMessageListResponse struct {
	Results    []rawMessage `json:"_results"`
	Pagination struct {
		Next string `json:"next"`
	} `json:"_pagination"`
}

type rawRecipient struct {
	Name   string `json:"name"`
	Handle string `json:"handle"`
	Role   string `json:"role"`
}

type rawMessage struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	IsInbound  bool           `json:"is_inbound"`
	CreatedAt  any            `json:"created_at"`
	Subject    string         `json:"subject"`
	Text       string         `json:"text"`
	Body       string         `json:"body"`
	Recipients []rawRecipient `json:"recipients"`
	Author     struct {
		Email     string `json:"email"`
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	} `json:"author"`
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
			Author:    messageAuthor(m),
			Subject:   m.Subject,
			Body:      markdown.Markdown(body),
		})
	}
	return messagesToMarkdown(msgs, raw.Pagination.Next), true
}

type renderedComment struct {
	Author string
	Posted string
	Pinned bool
	Body   markdown.Markdown
}

type rawCommentListResponse struct {
	Results    []rawComment `json:"_results"`
	Pagination struct {
		Next string `json:"next"`
	} `json:"_pagination"`
}

type rawComment struct {
	ID       string `json:"id"`
	Body     string `json:"body"`
	PostedAt any    `json:"posted_at"`
	IsPinned bool   `json:"is_pinned"`
	Author   struct {
		Email     string `json:"email"`
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	} `json:"author"`
}

func renderCommentsMD(data []byte) (markdown.Markdown, bool) {
	var raw rawCommentListResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}
	comments := make([]renderedComment, 0, len(raw.Results))
	for _, c := range raw.Results {
		comments = append(comments, renderedComment{
			Author: personLabel(c.Author.FirstName, c.Author.LastName, c.Author.Username, c.Author.Email),
			Posted: unixString(c.PostedAt),
			Pinned: c.IsPinned,
			Body:   markdown.Markdown(c.Body),
		})
	}
	return commentsToMarkdown(comments, raw.Pagination.Next), true
}

func commentsToMarkdown(comments []renderedComment, next string) markdown.Markdown {
	b := markdown.NewBuilder()
	b.Heading(1, fmt.Sprintf("Comments (%d)", len(comments)))
	if len(comments) == 0 {
		b.BlankLine()
		b.Raw("No comments.\n")
		appendNextPage(b, next)
		return b.Build()
	}
	b.BlankLine()
	for _, c := range comments {
		author := c.Author
		if author == "" {
			author = "unknown"
		}
		context := "internal note"
		if c.Pinned {
			context = "pinned, " + context
		}
		if c.Posted != "" {
			context += ", " + c.Posted
		}
		body := strings.TrimRight(string(c.Body), "\n")
		if body == "" {
			body = "(empty)"
		}
		b.CommentAttribution(author, context, body)
	}
	appendNextPage(b, next)
	return b.Build()
}

func messagesToMarkdown(msgs []renderedMessage, next string) markdown.Markdown {
	b := markdown.NewBuilder()
	b.Heading(1, fmt.Sprintf("Messages (%d)", len(msgs)))
	if len(msgs) == 0 {
		b.BlankLine()
		b.Raw("No messages.\n")
		appendNextPage(b, next)
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
		if m.Subject != "" {
			context += ", " + m.Subject
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
	appendNextPage(b, next)
	return b.Build()
}

func appendNextPage(b *markdown.Builder, next string) {
	if next == "" {
		return
	}
	b.BlankLine()
	b.Raw("Next page_token: " + next + "\n")
}

func messageAuthor(m rawMessage) string {
	author := personLabel(m.Author.FirstName, m.Author.LastName, m.Author.Username, m.Author.Email)
	if author != "" {
		return author
	}
	for _, r := range m.Recipients {
		if r.Role == "from" {
			if label := personLabel(r.Name, "", "", r.Handle); label != "" {
				return label
			}
		}
	}
	return ""
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
