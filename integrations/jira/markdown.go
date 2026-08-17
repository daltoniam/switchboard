package jira

import (
	"encoding/json"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

// ── Semantic domain types ────────────────────────────────────────────

type renderedIssue struct {
	Key         string
	Summary     string
	Status      string
	Assignee    string
	Reporter    string
	Priority    string
	IssueType   string
	Created     string
	Updated     string
	Labels      []string
	Components  []string
	FixVersions []string
	Description string // ADF already converted to markdown
}

type renderedJiraComment struct {
	Author  string
	Created string
	Body    string // ADF already converted to markdown
}

// ── Parse boundary ──────────────────────────────────────────────────

var markdownRenderers = map[mcp.ToolName]func([]byte) (markdown.Markdown, bool){
	"jira_get_issue":     renderIssueMD,
	"jira_list_comments": renderJiraCommentsMD,
}

func (j *jira) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	if fn, ok := markdownRenderers[toolName]; ok {
		return fn(data)
	}
	return "", false
}

// ── Raw JSON parse types ────────────────────────────────────────────

type rawIssueResponse struct {
	Key    string `json:"key"`
	Fields struct {
		Summary  string                `json:"summary"`
		Status   struct{ Name string } `json:"status"`
		Assignee *struct {
			DisplayName string `json:"displayName"`
		} `json:"assignee"`
		Reporter *struct {
			DisplayName string `json:"displayName"`
		} `json:"reporter"`
		Priority    struct{ Name string }   `json:"priority"`
		IssueType   struct{ Name string }   `json:"issuetype"`
		Description json.RawMessage         `json:"description"`
		Created     string                  `json:"created"`
		Updated     string                  `json:"updated"`
		Labels      []string                `json:"labels"`
		Components  []struct{ Name string } `json:"components"`
		FixVersions []struct{ Name string } `json:"fixVersions"`
	} `json:"fields"`
}

type rawJiraCommentsResponse struct {
	Comments []struct {
		Body   map[string]any `json:"body"`
		Author struct {
			DisplayName string `json:"displayName"`
		} `json:"author"`
		Created string `json:"created"`
	} `json:"comments"`
}

func renderIssueMD(data []byte) (markdown.Markdown, bool) {
	var raw rawIssueResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}

	components := make([]string, len(raw.Fields.Components))
	for i, c := range raw.Fields.Components {
		components[i] = c.Name
	}
	fixVersions := make([]string, len(raw.Fields.FixVersions))
	for i, v := range raw.Fields.FixVersions {
		fixVersions[i] = v.Name
	}

	assignee := ""
	if raw.Fields.Assignee != nil {
		assignee = raw.Fields.Assignee.DisplayName
	}
	reporter := ""
	if raw.Fields.Reporter != nil {
		reporter = raw.Fields.Reporter.DisplayName
	}

	issue := renderedIssue{
		Key:         raw.Key,
		Summary:     raw.Fields.Summary,
		Status:      raw.Fields.Status.Name,
		Assignee:    assignee,
		Reporter:    reporter,
		Priority:    raw.Fields.Priority.Name,
		IssueType:   raw.Fields.IssueType.Name,
		Created:     raw.Fields.Created,
		Updated:     raw.Fields.Updated,
		Labels:      raw.Fields.Labels,
		Components:  components,
		FixVersions: fixVersions,
		Description: parseDescription(raw.Fields.Description),
	}
	return issueToMarkdown(issue), true
}

func renderJiraCommentsMD(data []byte) (markdown.Markdown, bool) {
	var raw rawJiraCommentsResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}

	comments := make([]renderedJiraComment, len(raw.Comments))
	for i, c := range raw.Comments {
		comments[i] = renderedJiraComment{
			Author:  c.Author.DisplayName,
			Created: c.Created,
			Body:    adfToMarkdown(c.Body),
		}
	}
	return jiraCommentsToMarkdown(comments), true
}

// ── ADF → Markdown converter ────────────────────────────────────────

// parseDescription tries ADF (map) first, falls back to plain string.
// Jira Cloud always uses ADF, but this prevents silent data loss if the
// format is ever a plain string (e.g., migrated from Server/DC).
func parseDescription(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	// Try ADF (JSON object).
	var adf map[string]any
	if err := json.Unmarshal(raw, &adf); err == nil {
		return adfToMarkdown(adf)
	}
	// Fall back to plain string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s + "\n\n"
	}
	return ""
}

// adfToMarkdown converts an Atlassian Document Format JSON tree to Markdown.
func adfToMarkdown(doc map[string]any) string {
	if doc == nil {
		return ""
	}
	content, _ := doc["content"].([]any)
	var sb strings.Builder
	renderADFNodes(&sb, content, "")
	return sb.String()
}

func renderADFNodes(sb *strings.Builder, nodes []any, listPrefix string) {
	for _, node := range nodes {
		n, ok := node.(map[string]any)
		if !ok {
			continue
		}
		nodeType, _ := n["type"].(string)
		content, _ := n["content"].([]any)

		switch nodeType {
		case "paragraph":
			renderADFInline(sb, content)
			sb.WriteString("\n\n")

		case "heading":
			level := adfAttrInt(n, "level", 1)
			sb.WriteString(strings.Repeat("#", level) + " ")
			renderADFInline(sb, content)
			sb.WriteString("\n\n")

		case "bulletList":
			for _, item := range content {
				li, _ := item.(map[string]any)
				if li == nil {
					continue
				}
				sb.WriteString(listPrefix + "- ")
				renderADFListItem(sb, li, listPrefix)
			}
			if listPrefix == "" {
				sb.WriteString("\n")
			}

		case "orderedList":
			for idx, item := range content {
				li, _ := item.(map[string]any)
				if li == nil {
					continue
				}
				fmt.Fprintf(sb, "%s%d. ", listPrefix, idx+1)
				renderADFListItem(sb, li, listPrefix)
			}
			if listPrefix == "" {
				sb.WriteString("\n")
			}

		case "codeBlock":
			lang, _ := adfAttrStr(n, "language")
			sb.WriteString("```" + strings.ToLower(lang) + "\n")
			renderADFInline(sb, content)
			sb.WriteString("\n```\n\n")

		case "blockquote":
			var inner strings.Builder
			renderADFNodes(&inner, content, "")
			for _, line := range strings.Split(strings.TrimRight(inner.String(), "\n"), "\n") {
				sb.WriteString("> " + line + "\n")
			}
			sb.WriteString("\n")

		case "rule":
			sb.WriteString("---\n\n")

		case "table":
			renderADFTable(sb, content)

		case "mediaSingle", "mediaGroup":
			// Skip media — no markdown equivalent.

		case "panel":
			panelType, _ := adfAttrStr(n, "panelType")
			if panelType == "" {
				panelType = "info"
			}
			label := strings.ToUpper(panelType[:1]) + panelType[1:]
			var inner strings.Builder
			renderADFNodes(&inner, content, "")
			body := strings.TrimRight(inner.String(), "\n")
			fmt.Fprintf(sb, "> **%s:** %s\n\n", label, body)

		default:
			// Unknown block types — render children.
			if len(content) > 0 {
				renderADFNodes(sb, content, listPrefix)
			}
		}
	}
}

func renderADFListItem(sb *strings.Builder, li map[string]any, parentPrefix string) {
	content, _ := li["content"].([]any)
	for i, child := range content {
		c, _ := child.(map[string]any)
		if c == nil {
			continue
		}
		childType, _ := c["type"].(string)
		childContent, _ := c["content"].([]any)

		switch childType {
		case "paragraph":
			renderADFInline(sb, childContent)
			sb.WriteString("\n")
		case "bulletList", "orderedList":
			// Nested list — increase indent.
			renderADFNodes(sb, []any{c}, parentPrefix+"  ")
		default:
			if i == 0 {
				renderADFInline(sb, childContent)
				sb.WriteString("\n")
			}
		}
	}
}

func renderADFInline(sb *strings.Builder, nodes []any) {
	for _, node := range nodes {
		n, ok := node.(map[string]any)
		if !ok {
			continue
		}
		nodeType, _ := n["type"].(string)

		switch nodeType {
		case "text":
			text, _ := n["text"].(string)
			marks, _ := n["marks"].([]any)
			renderADFMarkedText(sb, text, marks)

		case "mention":
			mentionText, _ := adfAttrStr(n, "text")
			fmt.Fprintf(sb, "**%s**", mentionText)

		case "inlineCard":
			url, _ := adfAttrStr(n, "url")
			if url != "" {
				sb.WriteString(url)
			}

		case "hardBreak":
			sb.WriteString("\n")

		case "emoji":
			shortName, _ := adfAttrStr(n, "shortName")
			sb.WriteString(shortName)

		default:
			// Unknown inline — try text field.
			if text, ok := n["text"].(string); ok {
				sb.WriteString(text)
			}
		}
	}
}

func renderADFMarkedText(sb *strings.Builder, text string, marks []any) {
	var hasBold, hasItalic, hasCode, hasStrike bool
	var linkHref string

	for _, mark := range marks {
		m, ok := mark.(map[string]any)
		if !ok {
			continue
		}
		markType, _ := m["type"].(string)
		switch markType {
		case "strong":
			hasBold = true
		case "em":
			hasItalic = true
		case "code":
			hasCode = true
		case "strike":
			hasStrike = true
		case "link":
			attrs, _ := m["attrs"].(map[string]any)
			if attrs != nil {
				linkHref, _ = attrs["href"].(string)
			}
		}
	}

	markdown.ApplyMarks(sb, text, hasBold, hasItalic, hasCode, hasStrike, linkHref)
}

func renderADFTable(sb *strings.Builder, rows []any) {
	var table [][]string
	for _, row := range rows {
		r, _ := row.(map[string]any)
		if r == nil {
			continue
		}
		cells, _ := r["content"].([]any)
		var rowCells []string
		for _, cell := range cells {
			c, _ := cell.(map[string]any)
			if c == nil {
				continue
			}
			cellContent, _ := c["content"].([]any)
			var cellBuf strings.Builder
			renderADFNodes(&cellBuf, cellContent, "")
			rowCells = append(rowCells, strings.TrimSpace(cellBuf.String()))
		}
		if len(rowCells) > 0 {
			table = append(table, rowCells)
		}
	}

	markdown.WriteTable(sb, table)
	sb.WriteString("\n")
}

// ── Attribute helpers ───────────────────────────────────────────────

func adfAttrStr(n map[string]any, key string) (string, bool) {
	attrs, _ := n["attrs"].(map[string]any)
	if attrs == nil {
		return "", false
	}
	v, ok := attrs[key].(string)
	return v, ok
}

func adfAttrInt(n map[string]any, key string, fallback int) int {
	attrs, _ := n["attrs"].(map[string]any)
	if attrs == nil {
		return fallback
	}
	if v, ok := attrs[key].(float64); ok {
		return int(v)
	}
	return fallback
}

// ── Typed rendering functions ───────────────────────────────────────

func issueToMarkdown(issue renderedIssue) markdown.Markdown {
	b := markdown.NewBuilder()
	b.Metadata("jira", "key", issue.Key)
	b.Heading(1, issue.Key+": "+issue.Summary)
	b.Attribution(
		"Status: "+issue.Status,
		"Assignee: "+issue.Assignee,
		"Priority: "+issue.Priority,
		"Type: "+issue.IssueType,
	)
	b.Attribution("Created: "+issue.Created, "Updated: "+issue.Updated)

	if len(issue.Labels) > 0 || len(issue.Components) > 0 {
		var parts []string
		if len(issue.Labels) > 0 {
			parts = append(parts, "Labels: "+strings.Join(issue.Labels, ", "))
		}
		if len(issue.Components) > 0 {
			parts = append(parts, "Components: "+strings.Join(issue.Components, ", "))
		}
		if len(issue.FixVersions) > 0 {
			parts = append(parts, "Fix: "+strings.Join(issue.FixVersions, ", "))
		}
		b.Attribution(parts...)
	}

	if issue.Description != "" {
		b.BlankLine()
		b.Raw(issue.Description)
	}

	return b.Build()
}

func jiraCommentsToMarkdown(comments []renderedJiraComment) markdown.Markdown {
	if len(comments) == 0 {
		return markdown.NoComments
	}

	b := markdown.NewBuilder()
	b.Heading(2, fmt.Sprintf("Comments (%d)", len(comments)))
	b.BlankLine()

	for _, c := range comments {
		body := strings.TrimRight(c.Body, "\n")
		b.CommentAttribution(c.Author, c.Created, body)
	}

	return b.Build()
}
