package gtasks

import (
	"encoding/json"
	"sort"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

// ── Parse boundary ──────────────────────────────────────────────────

var markdownRenderers = map[mcp.ToolName]func([]byte) (markdown.Markdown, bool){
	"gtasks_list_tasklists": renderTasklistsMD,
	"gtasks_list_tasks":     renderTasksMD,
	"gtasks_get_task":       renderTaskMD,
}

func (g *gtasks) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	if fn, ok := markdownRenderers[toolName]; ok {
		return fn(data)
	}
	return "", false
}

// ── Raw types ───────────────────────────────────────────────────────

type rawTasklistsPage struct {
	Items         []rawTasklist `json:"items"`
	NextPageToken string        `json:"nextPageToken"`
}

type rawTasklist struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Updated string `json:"updated"`
}

type rawTasksPage struct {
	Items         []rawTask `json:"items"`
	NextPageToken string    `json:"nextPageToken"`
}

type rawTask struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Notes       string    `json:"notes"`
	Status      string    `json:"status"`
	Due         string    `json:"due"`
	Completed   string    `json:"completed"`
	Deleted     bool      `json:"deleted"`
	Hidden      bool      `json:"hidden"`
	Parent      string    `json:"parent"`
	Position    string    `json:"position"`
	Updated     string    `json:"updated"`
	WebViewLink string    `json:"webViewLink"`
	Links       []rawLink `json:"links"`
}

type rawLink struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Link        string `json:"link"`
}

// ── Tasklists ───────────────────────────────────────────────────────

func renderTasklistsMD(data []byte) (markdown.Markdown, bool) {
	var page rawTasklistsPage
	if err := json.Unmarshal(data, &page); err != nil {
		return "", false
	}
	// Refuse if the JSON has neither the items key nor a nextPageToken —
	// signals this isn't actually a tasklists response (the script path
	// may have routed something else here).
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return "", false
	}
	if _, hasItems := probe["items"]; !hasItems {
		if _, hasToken := probe["nextPageToken"]; !hasToken {
			return "", false
		}
	}

	b := markdown.NewBuilder()
	b.Heading(1, "Tasklists")
	if len(page.Items) == 0 {
		b.BlankLine()
		b.Raw("_No tasklists._\n")
		return b.Build(), true
	}

	var sb strings.Builder
	rows := [][]string{{"Title", "ID", "Updated"}}
	for _, tl := range page.Items {
		rows = append(rows, []string{
			pipeSafe(tl.Title),
			pipeSafe(tl.ID),
			pipeSafe(tl.Updated),
		})
	}
	markdown.WriteTable(&sb, rows)
	b.Raw(sb.String())

	if page.NextPageToken != "" {
		b.BlankLine()
		b.Attribution("next_page_token: " + page.NextPageToken)
	}
	return b.Build(), true
}

// ── Tasks ───────────────────────────────────────────────────────────

func renderTasksMD(data []byte) (markdown.Markdown, bool) {
	var page rawTasksPage
	if err := json.Unmarshal(data, &page); err != nil {
		return "", false
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return "", false
	}
	if _, hasItems := probe["items"]; !hasItems {
		if _, hasToken := probe["nextPageToken"]; !hasToken {
			return "", false
		}
	}

	b := markdown.NewBuilder()
	b.Heading(1, "Tasks")
	if len(page.Items) == 0 {
		b.BlankLine()
		b.Raw("_No tasks._\n")
		return b.Build(), true
	}

	// Render top-level tasks first, with subtasks nested as indented bullets
	// underneath their parent. Sort by position (lexicographic, as the API
	// returns) to preserve user-defined ordering.
	byParent := map[string][]rawTask{}
	for _, t := range page.Items {
		byParent[t.Parent] = append(byParent[t.Parent], t)
	}
	for parent := range byParent {
		sort.SliceStable(byParent[parent], func(i, j int) bool {
			return byParent[parent][i].Position < byParent[parent][j].Position
		})
	}

	var sb strings.Builder
	for _, t := range byParent[""] {
		writeTask(&sb, t, 0)
		writeChildren(&sb, byParent, t.ID, 1)
	}
	// Surface any orphan tasks (parent referenced but parent task not in page).
	for parent, children := range byParent {
		if parent == "" {
			continue
		}
		if _, parentInPage := findTaskByID(page.Items, parent); !parentInPage {
			for _, t := range children {
				writeTask(&sb, t, 0)
				writeChildren(&sb, byParent, t.ID, 1)
			}
		}
	}
	b.Raw(sb.String())

	if page.NextPageToken != "" {
		b.BlankLine()
		b.Attribution("next_page_token: " + page.NextPageToken)
	}
	return b.Build(), true
}

func writeChildren(sb *strings.Builder, byParent map[string][]rawTask, parentID string, depth int) {
	for _, c := range byParent[parentID] {
		writeTask(sb, c, depth)
		writeChildren(sb, byParent, c.ID, depth+1)
	}
}

func findTaskByID(items []rawTask, id string) (rawTask, bool) {
	for _, t := range items {
		if t.ID == id {
			return t, true
		}
	}
	return rawTask{}, false
}

func writeTask(sb *strings.Builder, t rawTask, depth int) {
	indent := strings.Repeat("  ", depth)
	checkbox := "[ ]"
	if t.Status == "completed" {
		checkbox = "[x]"
	}
	sb.WriteString(indent)
	sb.WriteString("- ")
	sb.WriteString(checkbox)
	sb.WriteString(" ")
	title := t.Title
	if title == "" {
		title = "(untitled)"
	}
	sb.WriteString(title)

	// Inline metadata: due date and id, for quick LLM reference.
	var meta []string
	if t.Due != "" {
		meta = append(meta, "due "+t.Due)
	}
	if t.ID != "" {
		meta = append(meta, "id="+t.ID)
	}
	if len(meta) > 0 {
		sb.WriteString("  _(" + strings.Join(meta, " · ") + ")_")
	}
	sb.WriteString("\n")

	if t.Notes != "" {
		for _, line := range strings.Split(strings.TrimRight(t.Notes, "\n"), "\n") {
			sb.WriteString(indent)
			sb.WriteString("  > ")
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}
	for _, l := range t.Links {
		desc := l.Description
		if desc == "" {
			desc = l.Link
		}
		sb.WriteString(indent)
		sb.WriteString("  - 🔗 [")
		sb.WriteString(desc)
		sb.WriteString("](")
		sb.WriteString(l.Link)
		sb.WriteString(")\n")
	}
}

// ── Get task ────────────────────────────────────────────────────────

func renderTaskMD(data []byte) (markdown.Markdown, bool) {
	var t rawTask
	if err := json.Unmarshal(data, &t); err != nil {
		return "", false
	}
	if t.ID == "" {
		return "", false
	}

	b := markdown.NewBuilder()
	title := t.Title
	if title == "" {
		title = "(untitled)"
	}
	b.Heading(1, title)

	var attrs []string
	if t.Status != "" {
		attrs = append(attrs, "status: "+t.Status)
	}
	if t.Due != "" {
		attrs = append(attrs, "due: "+t.Due)
	}
	if t.Completed != "" {
		attrs = append(attrs, "completed: "+t.Completed)
	}
	if t.Parent != "" {
		attrs = append(attrs, "parent: "+t.Parent)
	}
	attrs = append(attrs, "id="+t.ID)
	b.Attribution(attrs...)

	if t.Notes != "" {
		b.BlankLine()
		b.Raw(t.Notes)
		if !strings.HasSuffix(t.Notes, "\n") {
			b.Raw("\n")
		}
	}

	if len(t.Links) > 0 {
		b.BlankLine()
		b.Heading(2, "Links")
		var sb strings.Builder
		for _, l := range t.Links {
			desc := l.Description
			if desc == "" {
				desc = l.Link
			}
			sb.WriteString("- [")
			sb.WriteString(desc)
			sb.WriteString("](")
			sb.WriteString(l.Link)
			sb.WriteString(")")
			if l.Type != "" {
				sb.WriteString(" _(" + l.Type + ")_")
			}
			sb.WriteString("\n")
		}
		b.Raw(sb.String())
	}

	return b.Build(), true
}

// ── Helpers ─────────────────────────────────────────────────────────

// pipeSafe escapes newlines and pipes so a cell stays on one row.
func pipeSafe(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}
