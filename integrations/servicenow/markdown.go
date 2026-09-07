package servicenow

import (
	"encoding/json"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

type renderedRecord struct {
	SysID       string
	Number      string
	Title       string
	State       string
	Priority    string
	AssignedTo  string
	Group       string
	Updated     string
	Description string
	Extra       [][2]string
}

type renderedComment struct {
	Author  string
	Created string
	Element string
	Body    string
}

type renderedArticle struct {
	SysID    string
	Number   string
	Title    string
	State    string
	Base     string
	Category string
	Author   string
	Updated  string
	Body     string
}

var markdownRenderers = map[mcp.ToolName]func([]byte) (markdown.Markdown, bool){
	"servicenow_get_incident":          renderIncidentMD,
	"servicenow_get_problem":           renderProblemMD,
	"servicenow_get_change_request":    renderChangeMD,
	"servicenow_get_knowledge_article": renderArticleMD,
	"servicenow_list_comments":         renderCommentsMD,
}

func (s *servicenow) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	if fn, ok := markdownRenderers[toolName]; ok {
		return fn(data)
	}
	return "", false
}

type rawResultEnvelope struct {
	Result json.RawMessage `json:"result"`
}

func unwrapResult(data []byte) (map[string]any, bool) {
	var env rawResultEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, false
	}
	raw := env.Result
	if len(raw) == 0 {
		var direct map[string]any
		if err := json.Unmarshal(data, &direct); err != nil {
			return nil, false
		}
		return direct, true
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, false
	}
	return m, true
}

func unwrapResultList(data []byte) ([]map[string]any, bool) {
	var env rawResultEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, false
	}
	var list []map[string]any
	if err := json.Unmarshal(env.Result, &list); err != nil {
		return nil, false
	}
	return list, true
}

func display(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case map[string]any:
		if dv, ok := t["display_value"].(string); ok && dv != "" {
			return dv
		}
		if val, ok := t["value"].(string); ok {
			return val
		}
	}
	return ""
}

func field(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := display(m[k]); s != "" {
			return s
		}
	}
	return ""
}

func renderIncidentMD(data []byte) (markdown.Markdown, bool) {
	m, ok := unwrapResult(data)
	if !ok {
		return "", false
	}
	rec := renderedRecord{
		SysID:       field(m, "sys_id"),
		Number:      field(m, "number"),
		Title:       field(m, "short_description"),
		State:       field(m, "state"),
		Priority:    field(m, "priority"),
		AssignedTo:  field(m, "assigned_to"),
		Group:       field(m, "assignment_group"),
		Updated:     field(m, "sys_updated_on"),
		Description: field(m, "description"),
		Extra: [][2]string{
			{"Caller", field(m, "caller_id")},
			{"Urgency", field(m, "urgency")},
			{"Impact", field(m, "impact")},
			{"Category", field(m, "category")},
			{"Close code", field(m, "close_code")},
			{"Close notes", field(m, "close_notes")},
		},
	}
	return recordToMarkdown("incident", rec), true
}

func renderProblemMD(data []byte) (markdown.Markdown, bool) {
	m, ok := unwrapResult(data)
	if !ok {
		return "", false
	}
	rec := renderedRecord{
		SysID:       field(m, "sys_id"),
		Number:      field(m, "number"),
		Title:       field(m, "short_description"),
		State:       field(m, "state"),
		Priority:    field(m, "priority"),
		AssignedTo:  field(m, "assigned_to"),
		Group:       field(m, "assignment_group"),
		Updated:     field(m, "sys_updated_on"),
		Description: field(m, "description"),
		Extra: [][2]string{
			{"Workaround", field(m, "workaround")},
			{"Cause", field(m, "cause_notes")},
			{"Fix", field(m, "fix_notes")},
		},
	}
	return recordToMarkdown("problem", rec), true
}

func renderChangeMD(data []byte) (markdown.Markdown, bool) {
	m, ok := unwrapResult(data)
	if !ok {
		return "", false
	}
	rec := renderedRecord{
		SysID:       field(m, "sys_id"),
		Number:      field(m, "number"),
		Title:       field(m, "short_description"),
		State:       field(m, "state"),
		Priority:    field(m, "priority"),
		AssignedTo:  field(m, "assigned_to"),
		Group:       field(m, "assignment_group"),
		Updated:     field(m, "sys_updated_on"),
		Description: field(m, "description"),
		Extra: [][2]string{
			{"Type", field(m, "type")},
			{"Start", field(m, "start_date")},
			{"End", field(m, "end_date")},
			{"Risk", field(m, "risk")},
			{"Justification", field(m, "justification")},
			{"Implementation", field(m, "implementation_plan")},
			{"Backout", field(m, "backout_plan")},
		},
	}
	return recordToMarkdown("change", rec), true
}

func renderArticleMD(data []byte) (markdown.Markdown, bool) {
	m, ok := unwrapResult(data)
	if !ok {
		return "", false
	}
	body := field(m, "text")
	if body == "" {
		body = field(m, "wiki")
	}
	art := renderedArticle{
		SysID:    field(m, "sys_id"),
		Number:   field(m, "number"),
		Title:    field(m, "short_description"),
		State:    field(m, "workflow_state"),
		Base:     field(m, "kb_knowledge_base"),
		Category: field(m, "category"),
		Author:   field(m, "author"),
		Updated:  field(m, "sys_updated_on"),
		Body:     string(markdown.FromHTML(body)),
	}
	return articleToMarkdown(art), true
}

func renderCommentsMD(data []byte) (markdown.Markdown, bool) {
	list, ok := unwrapResultList(data)
	if !ok {
		return "", false
	}
	comments := make([]renderedComment, 0, len(list))
	for _, m := range list {
		comments = append(comments, renderedComment{
			Author:  field(m, "sys_created_by"),
			Created: field(m, "sys_created_on"),
			Element: field(m, "element"),
			Body:    string(markdown.FromHTML(field(m, "value"))),
		})
	}
	return commentsToMarkdown(comments), true
}

func recordToMarkdown(kind string, rec renderedRecord) markdown.Markdown {
	b := markdown.NewBuilder()
	b.Metadata("servicenow", "table", kind, "sys_id", rec.SysID, "number", rec.Number)
	title := rec.Title
	if rec.Number != "" {
		title = rec.Number + ": " + rec.Title
	}
	b.Heading(1, title)
	parts := []string{}
	if rec.State != "" {
		parts = append(parts, "State: "+rec.State)
	}
	if rec.Priority != "" {
		parts = append(parts, "Priority: "+rec.Priority)
	}
	if rec.AssignedTo != "" {
		parts = append(parts, "Assigned: "+rec.AssignedTo)
	}
	if rec.Group != "" {
		parts = append(parts, "Group: "+rec.Group)
	}
	if rec.Updated != "" {
		parts = append(parts, "Updated: "+rec.Updated)
	}
	if len(parts) > 0 {
		b.Attribution(parts...)
	}
	for _, kv := range rec.Extra {
		if kv[1] != "" {
			b.Raw("**" + kv[0] + ":** ")
			b.WriteMarkdown(markdown.FromHTML(kv[1]))
			b.Raw("\n")
		}
	}
	if rec.Description != "" {
		b.BlankLine()
		b.WriteMarkdown(markdown.FromHTML(rec.Description))
	}
	return b.Build()
}

func articleToMarkdown(art renderedArticle) markdown.Markdown {
	b := markdown.NewBuilder()
	b.Metadata("servicenow", "table", "kb_knowledge", "sys_id", art.SysID, "number", art.Number)
	title := art.Title
	if art.Number != "" {
		title = art.Number + ": " + art.Title
	}
	b.Heading(1, title)
	parts := []string{}
	if art.State != "" {
		parts = append(parts, "State: "+art.State)
	}
	if art.Base != "" {
		parts = append(parts, "KB: "+art.Base)
	}
	if art.Category != "" {
		parts = append(parts, "Category: "+art.Category)
	}
	if art.Author != "" {
		parts = append(parts, "Author: "+art.Author)
	}
	if art.Updated != "" {
		parts = append(parts, "Updated: "+art.Updated)
	}
	if len(parts) > 0 {
		b.Attribution(parts...)
	}
	if art.Body != "" {
		b.BlankLine()
		b.WriteMarkdown(markdown.Markdown(art.Body))
	}
	return b.Build()
}

func commentsToMarkdown(comments []renderedComment) markdown.Markdown {
	if len(comments) == 0 {
		return markdown.NoComments
	}
	b := markdown.NewBuilder()
	b.Heading(2, "Comments")
	b.BlankLine()
	for _, c := range comments {
		label := c.Author
		if c.Element != "" {
			label += " · " + c.Element
		}
		b.CommentAttribution(label, c.Created, c.Body)
	}
	return b.Build()
}
