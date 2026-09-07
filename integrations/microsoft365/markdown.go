package microsoft365

import (
	"encoding/json"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

type renderedMessage struct {
	ID      string
	Subject string
	From    string
	To      string
	Date    string
	Body    markdown.Markdown
}

type rawGraphMessage struct {
	ID               string         `json:"id"`
	Subject          string         `json:"subject"`
	ReceivedDateTime string         `json:"receivedDateTime"`
	From             rawRecipient   `json:"from"`
	ToRecipients     []rawRecipient `json:"toRecipients"`
	Body             rawItemBody    `json:"body"`
	BodyPreview      string         `json:"bodyPreview"`
}

type rawRecipient struct {
	EmailAddress rawEmailAddress `json:"emailAddress"`
}

type rawEmailAddress struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type rawItemBody struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

var markdownRenderers = map[mcp.ToolName]func([]byte) (markdown.Markdown, bool){
	"microsoft365_get_message": renderMessageMD,
}

func (m *m365) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	if fn, ok := markdownRenderers[toolName]; ok {
		return fn(data)
	}
	return "", false
}

func renderMessageMD(data []byte) (markdown.Markdown, bool) {
	var raw rawGraphMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}
	body := itemBodyMarkdown(raw.Body)
	if body == "" {
		body = markdown.Markdown(raw.BodyPreview)
	}
	msg := renderedMessage{
		ID:      raw.ID,
		Subject: raw.Subject,
		From:    formatRecipient(raw.From),
		To:      formatRecipients(raw.ToRecipients),
		Date:    raw.ReceivedDateTime,
		Body:    body,
	}
	return messageToMarkdown(msg), true
}

func itemBodyMarkdown(body rawItemBody) markdown.Markdown {
	if body.Content == "" {
		return ""
	}
	if strings.EqualFold(body.ContentType, "html") {
		return markdown.FromHTML(body.Content)
	}
	return markdown.Markdown(body.Content)
}

func formatRecipient(r rawRecipient) string {
	name := strings.TrimSpace(r.EmailAddress.Name)
	addr := strings.TrimSpace(r.EmailAddress.Address)
	if name != "" && addr != "" {
		return name + " <" + addr + ">"
	}
	if addr != "" {
		return addr
	}
	return name
}

func formatRecipients(rs []rawRecipient) string {
	parts := make([]string, 0, len(rs))
	for _, r := range rs {
		if s := formatRecipient(r); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}

func messageToMarkdown(msg renderedMessage) markdown.Markdown {
	b := markdown.NewBuilder()
	b.Metadata("microsoft365", "message_id", msg.ID)
	subject := msg.Subject
	if subject == "" {
		subject = "(no subject)"
	}
	b.Heading(1, subject)
	b.Attribution("From: "+msg.From, "To: "+msg.To)
	if msg.Date != "" {
		b.Attribution("Date: " + msg.Date)
	}
	if msg.Body != "" {
		b.BlankLine()
		b.WriteMarkdown(msg.Body)
		if !strings.HasSuffix(string(msg.Body), "\n") {
			b.Raw("\n")
		}
	}
	return b.Build()
}
