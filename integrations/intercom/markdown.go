package intercom

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

type renderedConversation struct {
	ID       string
	Title    string
	State    string
	Priority string
	Created  string
	Updated  string
	Assignee string
	Contact  string
	Parts    []renderedPart
}

type renderedPart struct {
	Author    string
	CreatedAt string
	PartType  string
	Body      markdown.Markdown
}

type renderedArticle struct {
	ID     string
	Title  string
	State  string
	Author string
	URL    string
	Body   markdown.Markdown
}

var markdownRenderers = map[mcp.ToolName]func([]byte) (markdown.Markdown, bool){
	"intercom_get_conversation": renderConversationMD,
	"intercom_get_article":      renderArticleMD,
}

func (c *intercom) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	if fn, ok := markdownRenderers[toolName]; ok {
		return fn(data)
	}
	return "", false
}

type rawConversationResponse struct {
	ID       any    `json:"id"`
	Title    string `json:"title"`
	State    string `json:"state"`
	Priority string `json:"priority"`
	Created  int64  `json:"created_at"`
	Updated  int64  `json:"updated_at"`
	Source   struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
		Author  struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Type  string `json:"type"`
		} `json:"author"`
	} `json:"source"`
	AdminAssigneeID any `json:"admin_assignee_id"`
	TeamAssigneeID  any `json:"team_assignee_id"`
	Contacts        struct {
		Contacts []struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"contacts"`
	} `json:"contacts"`
	ConversationParts struct {
		ConversationParts []struct {
			PartType  string `json:"part_type"`
			Body      string `json:"body"`
			CreatedAt int64  `json:"created_at"`
			Author    struct {
				Name  string `json:"name"`
				Email string `json:"email"`
				Type  string `json:"type"`
			} `json:"author"`
		} `json:"conversation_parts"`
	} `json:"conversation_parts"`
}

type rawArticleResponse struct {
	ID     any    `json:"id"`
	Title  string `json:"title"`
	State  string `json:"state"`
	URL    string `json:"url"`
	Body   string `json:"body"`
	Author struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"author"`
}

func anyID(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case json.Number:
		return t.String()
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	}
}

func unixString(ts int64) string {
	if ts == 0 {
		return ""
	}
	return strconv.FormatInt(ts, 10)
}

func authorLabel(name, email, typ string) string {
	label := name
	if label == "" {
		label = email
	}
	if label == "" {
		label = typ
	}
	if email != "" && name != "" {
		return name + " <" + email + ">"
	}
	return label
}

func renderConversationMD(data []byte) (markdown.Markdown, bool) {
	var raw rawConversationResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}
	title := raw.Title
	if title == "" {
		title = raw.Source.Subject
	}
	if title == "" {
		title = "Conversation " + anyID(raw.ID)
	}
	contact := ""
	if len(raw.Contacts.Contacts) > 0 {
		c := raw.Contacts.Contacts[0]
		contact = authorLabel(c.Name, c.Email, "contact")
	} else if raw.Source.Author.Name != "" || raw.Source.Author.Email != "" {
		contact = authorLabel(raw.Source.Author.Name, raw.Source.Author.Email, raw.Source.Author.Type)
	}
	assignee := anyID(raw.AdminAssigneeID)
	if assignee == "" {
		assignee = anyID(raw.TeamAssigneeID)
	}
	parts := make([]renderedPart, 0, len(raw.ConversationParts.ConversationParts)+1)
	if raw.Source.Body != "" {
		parts = append(parts, renderedPart{
			Author:    authorLabel(raw.Source.Author.Name, raw.Source.Author.Email, raw.Source.Author.Type),
			CreatedAt: unixString(raw.Created),
			PartType:  "source",
			Body:      markdown.FromHTML(raw.Source.Body),
		})
	}
	for _, p := range raw.ConversationParts.ConversationParts {
		parts = append(parts, renderedPart{
			Author:    authorLabel(p.Author.Name, p.Author.Email, p.Author.Type),
			CreatedAt: unixString(p.CreatedAt),
			PartType:  p.PartType,
			Body:      markdown.FromHTML(p.Body),
		})
	}
	conv := renderedConversation{
		ID:       anyID(raw.ID),
		Title:    title,
		State:    raw.State,
		Priority: raw.Priority,
		Created:  unixString(raw.Created),
		Updated:  unixString(raw.Updated),
		Assignee: assignee,
		Contact:  contact,
		Parts:    parts,
	}
	return conversationToMarkdown(conv), true
}

func renderArticleMD(data []byte) (markdown.Markdown, bool) {
	var raw rawArticleResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}
	article := renderedArticle{
		ID:     anyID(raw.ID),
		Title:  raw.Title,
		State:  raw.State,
		Author: authorLabel(raw.Author.Name, raw.Author.Email, ""),
		URL:    raw.URL,
		Body:   markdown.FromHTML(raw.Body),
	}
	return articleToMarkdown(article), true
}

func conversationToMarkdown(conv renderedConversation) markdown.Markdown {
	b := markdown.NewBuilder()
	b.Metadata("intercom", "conversation_id", conv.ID, "state", conv.State)
	b.Heading(1, conv.Title)
	attrs := []string{}
	if conv.State != "" {
		attrs = append(attrs, "State: "+conv.State)
	}
	if conv.Priority != "" {
		attrs = append(attrs, "Priority: "+conv.Priority)
	}
	if conv.Contact != "" {
		attrs = append(attrs, "Contact: "+conv.Contact)
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
	if len(conv.Parts) == 0 {
		b.BlankLine()
		b.Raw("No messages.\n")
		return b.Build()
	}
	b.BlankLine()
	b.Heading(2, fmt.Sprintf("Messages (%d)", len(conv.Parts)))
	b.BlankLine()
	for _, p := range conv.Parts {
		author := p.Author
		if author == "" {
			author = "unknown"
		}
		context := p.PartType
		if p.CreatedAt != "" {
			if context != "" {
				context += ", " + p.CreatedAt
			} else {
				context = p.CreatedAt
			}
		}
		body := strings.TrimRight(string(p.Body), "\n")
		if body == "" {
			body = "(empty)"
		}
		b.CommentAttribution(author, context, body)
	}
	return b.Build()
}

func articleToMarkdown(article renderedArticle) markdown.Markdown {
	b := markdown.NewBuilder()
	b.Metadata("intercom", "article_id", article.ID, "state", article.State)
	title := article.Title
	if title == "" {
		title = "Article " + article.ID
	}
	b.Heading(1, title)
	attrs := []string{}
	if article.Author != "" {
		attrs = append(attrs, "Author: "+article.Author)
	}
	if article.URL != "" {
		attrs = append(attrs, "URL: "+article.URL)
	}
	if len(attrs) > 0 {
		b.Attribution(attrs...)
	}
	if article.Body != "" {
		b.BlankLine()
		b.WriteMarkdown(article.Body)
	}
	return b.Build()
}
