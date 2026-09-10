package forgejo

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

type rawMarkdownUser struct {
	Login string `json:"login"`
}

type rawMarkdownLabel struct {
	Name string `json:"name"`
}

type rawMarkdownIssue struct {
	ID             int64               `json:"id"`
	Number         int64               `json:"number"`
	Title          string              `json:"title"`
	Body           *string             `json:"body"`
	State          string              `json:"state"`
	User           *rawMarkdownUser    `json:"user"`
	OriginalAuthor string              `json:"original_author"`
	Labels         []*rawMarkdownLabel `json:"labels"`
	Assignees      []*rawMarkdownUser  `json:"assignees"`
	Milestone      *struct {
		Title string `json:"title"`
	} `json:"milestone"`
	Repository *struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Ref       string `json:"ref"`
	Comments  int    `json:"comments"`
	HTMLURL   string `json:"html_url"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	ClosedAt  string `json:"closed_at"`
}

type renderedIssue struct {
	id          string
	number      string
	title       string
	body        markdown.Markdown
	attribution []string
	details     []string
	url         string
}

type rawMarkdownComment struct {
	ID             int64            `json:"id"`
	Body           *string          `json:"body"`
	User           *rawMarkdownUser `json:"user"`
	OriginalAuthor string           `json:"original_author"`
	HTMLURL        string           `json:"html_url"`
	CreatedAt      string           `json:"created_at"`
	UpdatedAt      string           `json:"updated_at"`
}

type renderedComment struct {
	id      string
	author  string
	context string
	body    string
	url     string
}

func (f *forgejo) RenderMarkdown(name mcp.ToolName, data []byte) (mcp.Markdown, bool) {
	switch name {
	case "forgejo_get_issue":
		issue, ok := parseMarkdownIssue(data)
		if !ok {
			return "", false
		}
		return renderMarkdownIssue(issue), true
	case "forgejo_list_issue_comments":
		comments, ok := parseMarkdownComments(data)
		if !ok {
			return "", false
		}
		return renderMarkdownComments(comments), true
	default:
		return "", false
	}
}

func markdownAuthor(user *rawMarkdownUser, original string) string {
	if user != nil && user.Login != "" {
		return user.Login
	}
	if original != "" {
		return original
	}
	return "Unknown"
}

func parseMarkdownIssue(data []byte) (renderedIssue, bool) {
	var raw *rawMarkdownIssue
	if err := json.Unmarshal(data, &raw); err != nil || raw == nil || raw.ID <= 0 || raw.Number <= 0 || strings.TrimSpace(raw.Title) == "" || raw.Body == nil {
		return renderedIssue{}, false
	}
	issue := renderedIssue{
		id: strconv.FormatInt(raw.ID, 10), number: strconv.FormatInt(raw.Number, 10),
		title: raw.Title, body: markdown.Markdown(*raw.Body), url: raw.HTMLURL,
		attribution: []string{"Author: " + markdownAuthor(raw.User, raw.OriginalAuthor)},
	}
	if raw.State != "" {
		issue.attribution = append(issue.attribution, "Status: "+raw.State)
	}
	var labels, assignees []string
	for _, label := range raw.Labels {
		if label != nil && label.Name != "" {
			labels = append(labels, label.Name)
		}
	}
	for _, user := range raw.Assignees {
		if user != nil && user.Login != "" {
			assignees = append(assignees, user.Login)
		}
	}
	if len(labels) > 0 {
		issue.details = append(issue.details, "Labels: "+strings.Join(labels, ", "))
	}
	if len(assignees) > 0 {
		issue.details = append(issue.details, "Assignees: "+strings.Join(assignees, ", "))
	}
	if raw.Milestone != nil && raw.Milestone.Title != "" {
		issue.details = append(issue.details, "Milestone: "+raw.Milestone.Title)
	}
	if raw.Repository != nil && raw.Repository.FullName != "" {
		issue.details = append(issue.details, "Repository: "+raw.Repository.FullName)
	}
	for _, detail := range []struct{ label, value string }{
		{"Ref", raw.Ref}, {"Created", raw.CreatedAt}, {"Updated", raw.UpdatedAt}, {"Closed", raw.ClosedAt},
	} {
		if detail.value != "" {
			issue.details = append(issue.details, detail.label+": "+detail.value)
		}
	}
	issue.details = append(issue.details, fmt.Sprintf("Comments: %d", raw.Comments))
	return issue, true
}

func renderMarkdownIssue(issue renderedIssue) mcp.Markdown {
	b := markdown.NewBuilder()
	b.Metadata("forgejo", "issue_id", issue.id, "number", issue.number)
	b.Heading(1, "#"+issue.number+" "+issue.title)
	b.Attribution(issue.attribution...)
	b.BlankLine()
	for _, detail := range issue.details {
		b.Raw(detail + "\n")
	}
	if issue.url != "" {
		b.Raw(issue.url + "\n")
	}
	b.BlankLine()
	b.WriteMarkdown(issue.body)
	b.BlankLine()
	return b.Build()
}

func parseMarkdownComments(data []byte) ([]renderedComment, bool) {
	var raw []*rawMarkdownComment
	if err := json.Unmarshal(data, &raw); err != nil || raw == nil {
		return nil, false
	}
	comments := make([]renderedComment, 0, len(raw))
	for _, item := range raw {
		if item == nil || item.ID <= 0 || item.Body == nil {
			return nil, false
		}
		id := strconv.FormatInt(item.ID, 10)
		context := "Comment " + id
		if item.CreatedAt != "" {
			context += " | " + item.CreatedAt
		}
		if item.UpdatedAt != "" && item.UpdatedAt != item.CreatedAt {
			context += " | Updated: " + item.UpdatedAt
		}
		comments = append(comments, renderedComment{
			id: id, author: markdownAuthor(item.User, item.OriginalAuthor), context: context,
			body: *item.Body, url: item.HTMLURL,
		})
	}
	return comments, true
}

func commentMarkdownFence(body string) string {
	length, run := 3, 0
	for i := 0; i < len(body); i++ {
		if body[i] == '`' {
			run++
			length = max(length, run+1)
		} else {
			run = 0
		}
	}
	return strings.Repeat("`", length)
}

func renderMarkdownComments(comments []renderedComment) mcp.Markdown {
	if len(comments) == 0 {
		return markdown.NoComments
	}
	b := markdown.NewBuilder()
	b.Heading(2, fmt.Sprintf("Comments (%d)", len(comments)))
	b.BlankLine()
	for _, comment := range comments {
		b.Metadata("forgejo", "comment_id", comment.id)
		b.Raw(fmt.Sprintf("**%s** (%s):\n\n", comment.author, comment.context))
		fence := commentMarkdownFence(comment.body)
		b.Raw(fence + "markdown\n")
		b.Raw(comment.body)
		if comment.body != "" && !strings.HasSuffix(comment.body, "\n") {
			b.BlankLine()
		}
		b.Raw(fence + "\n\n")
		if comment.url != "" {
			b.Raw(comment.url + "\n")
			b.BlankLine()
		}
	}
	return b.Build()
}
