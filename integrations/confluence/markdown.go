package confluence

import (
	"encoding/json"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

// ── Semantic domain types ────────────────────────────────────────────

type renderedPage struct {
	ID        string
	Title     string
	SpaceID   string
	AuthorID  string
	Version   int
	CreatedAt string
	Body      markdown.Markdown
}

type renderedComment struct {
	AuthorID  string
	CreatedAt string
	Version   int
	Body      markdown.Markdown
}

// ── Parse boundary ──────────────────────────────────────────────────

var markdownRenderers = map[mcp.ToolName]func([]byte) (markdown.Markdown, bool){
	"confluence_get_page":      renderPageMD,
	"confluence_get_blog_post": renderPageMD,
	"confluence_list_comments": renderCommentsMD,
}

func (c *confluence) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	if fn, ok := markdownRenderers[toolName]; ok {
		return fn(data)
	}
	return "", false
}

// ── Raw JSON parse types ────────────────────────────────────────────

type rawPageResponse struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	SpaceID  string `json:"spaceId"`
	AuthorID string `json:"authorId"`
	Version  struct {
		Number int `json:"number"`
	} `json:"version"`
	CreatedAt string `json:"createdAt"`
	Body      struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body"`
}

type rawCommentsResponse struct {
	Results []struct {
		AuthorID string `json:"authorId"`
		Version  struct {
			Number int `json:"number"`
		} `json:"version"`
		CreatedAt string `json:"createdAt"`
		Body      struct {
			Storage struct {
				Value string `json:"value"`
			} `json:"storage"`
		} `json:"body"`
	} `json:"results"`
}

func renderPageMD(data []byte) (markdown.Markdown, bool) {
	var raw rawPageResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}

	page := renderedPage{
		ID:        raw.ID,
		Title:     raw.Title,
		SpaceID:   raw.SpaceID,
		AuthorID:  raw.AuthorID,
		Version:   raw.Version.Number,
		CreatedAt: raw.CreatedAt,
		Body:      markdown.FromHTML(raw.Body.Storage.Value),
	}
	return pageToMarkdown(page), true
}

func renderCommentsMD(data []byte) (markdown.Markdown, bool) {
	var raw rawCommentsResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}

	comments := make([]renderedComment, len(raw.Results))
	for i, r := range raw.Results {
		comments[i] = renderedComment{
			AuthorID:  r.AuthorID,
			CreatedAt: r.CreatedAt,
			Version:   r.Version.Number,
			Body:      markdown.FromHTML(r.Body.Storage.Value),
		}
	}
	return commentsToMarkdown(comments), true
}

// ── Typed rendering (uses MarkdownBuilder) ──────────────────────────

func pageToMarkdown(page renderedPage) markdown.Markdown {
	b := markdown.NewBuilder()
	b.Metadata("confluence", "page_id", page.ID, "space", page.SpaceID, "version", fmt.Sprintf("%d", page.Version))
	b.Heading(1, page.Title)
	b.Attribution("Author: "+page.AuthorID, "Created: "+page.CreatedAt)

	if page.Body != "" {
		b.BlankLine()
		b.WriteMarkdown(page.Body)
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
		body := strings.TrimRight(string(c.Body), "\n")
		b.CommentAttribution(c.AuthorID, fmt.Sprintf("v%d, %s", c.Version, c.CreatedAt), body)
	}

	return b.Build()
}
