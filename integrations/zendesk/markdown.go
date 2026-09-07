package zendesk

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

type renderedTicket struct {
	ID          int64
	Subject     string
	Status      string
	Priority    string
	Type        string
	RequesterID int64
	AssigneeID  int64
	GroupID     int64
	Tags        []string
	CreatedAt   string
	UpdatedAt   string
	Description string
}

type renderedComment struct {
	AuthorID  int64
	CreatedAt string
	Public    bool
	Body      string
}

type renderedArticle struct {
	ID        int64
	Title     string
	Locale    string
	AuthorID  int64
	CreatedAt string
	UpdatedAt string
	Body      markdown.Markdown
}

var markdownRenderers = map[mcp.ToolName]func([]byte) (markdown.Markdown, bool){
	"zendesk_get_ticket":           renderTicketMD,
	"zendesk_list_ticket_comments": renderCommentsMD,
	"zendesk_get_article":          renderArticleMD,
}

func (z *zendesk) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	if fn, ok := markdownRenderers[toolName]; ok {
		return fn(data)
	}
	return "", false
}

type rawTicketResponse struct {
	Ticket struct {
		ID          int64    `json:"id"`
		Subject     string   `json:"subject"`
		Status      string   `json:"status"`
		Priority    string   `json:"priority"`
		Type        string   `json:"type"`
		RequesterID int64    `json:"requester_id"`
		AssigneeID  int64    `json:"assignee_id"`
		GroupID     int64    `json:"group_id"`
		Tags        []string `json:"tags"`
		CreatedAt   string   `json:"created_at"`
		UpdatedAt   string   `json:"updated_at"`
		Description string   `json:"description"`
	} `json:"ticket"`
}

type rawCommentsResponse struct {
	Comments []struct {
		AuthorID  int64  `json:"author_id"`
		CreatedAt string `json:"created_at"`
		Public    bool   `json:"public"`
		Body      string `json:"body"`
		HTMLBody  string `json:"html_body"`
	} `json:"comments"`
}

type rawArticleResponse struct {
	Article struct {
		ID        int64  `json:"id"`
		Title     string `json:"title"`
		Locale    string `json:"locale"`
		AuthorID  int64  `json:"author_id"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
		Body      string `json:"body"`
	} `json:"article"`
}

func renderTicketMD(data []byte) (markdown.Markdown, bool) {
	var raw rawTicketResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}
	t := renderedTicket{
		ID:          raw.Ticket.ID,
		Subject:     raw.Ticket.Subject,
		Status:      raw.Ticket.Status,
		Priority:    raw.Ticket.Priority,
		Type:        raw.Ticket.Type,
		RequesterID: raw.Ticket.RequesterID,
		AssigneeID:  raw.Ticket.AssigneeID,
		GroupID:     raw.Ticket.GroupID,
		Tags:        raw.Ticket.Tags,
		CreatedAt:   raw.Ticket.CreatedAt,
		UpdatedAt:   raw.Ticket.UpdatedAt,
		Description: raw.Ticket.Description,
	}
	return ticketToMarkdown(t), true
}

func renderCommentsMD(data []byte) (markdown.Markdown, bool) {
	var raw rawCommentsResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}
	comments := make([]renderedComment, len(raw.Comments))
	for i, c := range raw.Comments {
		body := c.Body
		if body == "" && c.HTMLBody != "" {
			body = string(markdown.FromHTML(c.HTMLBody))
		}
		comments[i] = renderedComment{
			AuthorID:  c.AuthorID,
			CreatedAt: c.CreatedAt,
			Public:    c.Public,
			Body:      body,
		}
	}
	return commentsToMarkdown(comments), true
}

func renderArticleMD(data []byte) (markdown.Markdown, bool) {
	var raw rawArticleResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}
	a := renderedArticle{
		ID:        raw.Article.ID,
		Title:     raw.Article.Title,
		Locale:    raw.Article.Locale,
		AuthorID:  raw.Article.AuthorID,
		CreatedAt: raw.Article.CreatedAt,
		UpdatedAt: raw.Article.UpdatedAt,
		Body:      markdown.FromHTML(raw.Article.Body),
	}
	return articleToMarkdown(a), true
}

func ticketToMarkdown(t renderedTicket) markdown.Markdown {
	b := markdown.NewBuilder()
	b.Metadata("zendesk", "ticket_id", strconv.FormatInt(t.ID, 10))
	title := t.Subject
	if title == "" {
		title = fmt.Sprintf("Ticket %d", t.ID)
	}
	b.Heading(1, fmt.Sprintf("#%d: %s", t.ID, title))
	b.Attribution(
		"Status: "+emptyDash(t.Status),
		"Priority: "+emptyDash(t.Priority),
		"Type: "+emptyDash(t.Type),
	)
	b.Attribution(
		"Requester: "+idOrDash(t.RequesterID),
		"Assignee: "+idOrDash(t.AssigneeID),
		"Group: "+idOrDash(t.GroupID),
	)
	b.Attribution("Created: "+emptyDash(t.CreatedAt), "Updated: "+emptyDash(t.UpdatedAt))
	if len(t.Tags) > 0 {
		b.Attribution("Tags: " + strings.Join(t.Tags, ", "))
	}
	if t.Description != "" {
		b.BlankLine()
		b.Raw(t.Description)
	}
	return b.Build()
}

func commentsToMarkdown(comments []renderedComment) markdown.Markdown {
	if len(comments) == 0 {
		return markdown.NoComments
	}
	b := markdown.NewBuilder()
	b.Heading(2, fmt.Sprintf("Comments (%d)", len(comments)))
	b.BlankLine()
	for _, c := range comments {
		kind := "public"
		if !c.Public {
			kind = "internal note"
		}
		body := strings.TrimRight(c.Body, "\n")
		b.CommentAttribution(idOrDash(c.AuthorID), kind+", "+c.CreatedAt, body)
	}
	return b.Build()
}

func articleToMarkdown(a renderedArticle) markdown.Markdown {
	b := markdown.NewBuilder()
	b.Metadata("zendesk", "article_id", strconv.FormatInt(a.ID, 10), "locale", a.Locale)
	b.Heading(1, a.Title)
	b.Attribution(
		"Author: "+idOrDash(a.AuthorID),
		"Locale: "+emptyDash(a.Locale),
		"Created: "+emptyDash(a.CreatedAt),
		"Updated: "+emptyDash(a.UpdatedAt),
	)
	if a.Body != "" {
		b.BlankLine()
		b.WriteMarkdown(a.Body)
	}
	return b.Build()
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func idOrDash(id int64) string {
	if id == 0 {
		return "-"
	}
	return strconv.FormatInt(id, 10)
}
