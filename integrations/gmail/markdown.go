package gmail

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

// ── Semantic domain types ────────────────────────────────────────────

type renderedMessage struct {
	ID       string
	ThreadID string
	Subject  string
	From     string
	To       string
	Date     string
	Body     markdown.Markdown
}

// ── Parse boundary ──────────────────────────────────────────────────

func (g *gmail) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	switch toolName {
	case "gmail_get_message", "gmail_get_draft":
		return renderMessageMD(data)
	case "gmail_get_thread":
		return renderThreadMD(data)
	default:
		return "", false
	}
}

// ── Raw JSON parse types ────────────────────────────────────────────

type rawMessageResponse struct {
	ID       string     `json:"id"`
	ThreadID string     `json:"threadId"`
	Payload  rawPayload `json:"payload"`
}

type rawPayload struct {
	MimeType string       `json:"mimeType"`
	Headers  []rawHeader  `json:"headers"`
	Body     rawBody      `json:"body"`
	Parts    []rawPayload `json:"parts"`
}

type rawHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type rawBody struct {
	Data string `json:"data"`
}

type rawThreadResponse struct {
	ID       string               `json:"id"`
	Messages []rawMessageResponse `json:"messages"`
}

// ── MIME extraction ─────────────────────────────────────────────────

// extractBody extracts the best text representation from a Gmail MIME payload.
// Prefers text/plain (already LLM-readable). Falls back to text/html → markdown.FromHTML.
func extractBody(payload rawPayload) markdown.Markdown {
	// Simple non-multipart body.
	if !strings.HasPrefix(payload.MimeType, "multipart/") {
		decoded := decodeBase64URL(payload.Body.Data)
		if payload.MimeType == "text/html" {
			return markdown.FromHTML(decoded)
		}
		return markdown.Markdown(decoded)
	}

	// Multipart: search parts recursively. Prefer text/plain if non-empty.
	// Some email clients send whitespace-only text/plain with real content in text/html.
	if plain := findPart(payload.Parts, "text/plain"); strings.TrimSpace(plain) != "" {
		return markdown.Markdown(plain)
	}
	if html := findPart(payload.Parts, "text/html"); html != "" {
		return markdown.FromHTML(html)
	}
	return ""
}

func findPart(parts []rawPayload, mimeType string) string {
	for _, p := range parts {
		if p.MimeType == mimeType {
			return decodeBase64URL(p.Body.Data)
		}
		if strings.HasPrefix(p.MimeType, "multipart/") {
			if found := findPart(p.Parts, mimeType); found != "" {
				return found
			}
		}
	}
	return ""
}

func decodeBase64URL(s string) string {
	if s == "" {
		return ""
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return s
	}
	return string(b)
}

// ── Header extraction ───────────────────────────────────────────────

func extractHeaders(payload rawPayload) (subject, from, to, date string) {
	for _, h := range payload.Headers {
		switch h.Name {
		case "Subject":
			subject = h.Value
		case "From":
			from = h.Value
		case "To":
			to = h.Value
		case "Date":
			date = h.Value
		}
	}
	return
}

// ── Rendering ───────────────────────────────────────────────────────

func renderMessageMD(data []byte) (markdown.Markdown, bool) {
	var raw rawMessageResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}

	subject, from, to, date := extractHeaders(raw.Payload)
	msg := renderedMessage{
		ID:       raw.ID,
		ThreadID: raw.ThreadID,
		Subject:  subject,
		From:     from,
		To:       to,
		Date:     date,
		Body:     extractBody(raw.Payload),
	}
	return messageToMarkdown(msg), true
}

func renderThreadMD(data []byte) (markdown.Markdown, bool) {
	var raw rawThreadResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}

	messages := make([]renderedMessage, len(raw.Messages))
	for i, m := range raw.Messages {
		subject, from, to, date := extractHeaders(m.Payload)
		messages[i] = renderedMessage{
			ID:       m.ID,
			ThreadID: m.ThreadID,
			Subject:  subject,
			From:     from,
			To:       to,
			Date:     date,
			Body:     extractBody(m.Payload),
		}
	}
	return threadToMarkdown(raw.ID, messages), true
}

func messageToMarkdown(msg renderedMessage) markdown.Markdown {
	b := markdown.NewBuilder()
	b.Metadata("gmail", "message_id", msg.ID, "thread_id", msg.ThreadID)
	b.Heading(1, msg.Subject)
	b.Attribution("From: "+msg.From, "To: "+msg.To)
	b.Attribution("Date: " + msg.Date)

	if msg.Body != "" {
		b.BlankLine()
		b.WriteMarkdown(msg.Body)
		if !strings.HasSuffix(string(msg.Body), "\n") {
			b.Raw("\n")
		}
	}

	return b.Build()
}

func threadToMarkdown(threadID string, messages []renderedMessage) markdown.Markdown {
	if len(messages) == 0 {
		return ""
	}

	b := markdown.NewBuilder()
	b.Metadata("gmail", "thread_id", threadID, "messages", fmt.Sprintf("%d", len(messages)))
	b.Heading(1, messages[0].Subject)

	for i, msg := range messages {
		b.BlankLine()
		b.Heading(3, fmt.Sprintf("Message %d — %s (%s)", i+1, msg.From, msg.Date))
		if msg.Body != "" {
			b.WriteMarkdown(msg.Body)
			if !strings.HasSuffix(string(msg.Body), "\n") {
				b.Raw("\n")
			}
		}
		if i < len(messages)-1 {
			b.BlankLine()
			b.Divider()
		}
	}

	return b.Build()
}
