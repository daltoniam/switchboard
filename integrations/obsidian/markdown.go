package obsidian

import (
	"encoding/json"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

var markdownRenderers = map[mcp.ToolName]func([]byte) (markdown.Markdown, bool){
	"obsidian_read_note":       renderNoteMD,
	"obsidian_get_active_note": renderNoteMD,
}

func (o *obsidian) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	if fn, ok := markdownRenderers[toolName]; ok {
		return fn(data)
	}
	return "", false
}

// renderedNote covers both a full note (note+json) and a targeted section
// ({path, target, content}) from obsidian_read_note.
type renderedNote struct {
	Path    *string  `json:"path"`
	Content *string  `json:"content"`
	Target  string   `json:"target"`
	Tags    []string `json:"tags"`
	Links   []string `json:"links"`
	Stat    struct {
		Mtime int64 `json:"mtime"`
	} `json:"stat"`
}

// renderNoteMD returns the note's own markdown (frontmatter included) under a
// one-line metadata comment, so the agent reads the note as written rather
// than as a JSON-escaped string.
func renderNoteMD(data []byte) (markdown.Markdown, bool) {
	var n renderedNote
	if err := json.Unmarshal(data, &n); err != nil || n.Path == nil || n.Content == nil {
		return "", false
	}
	var b strings.Builder
	b.WriteString("<!-- obsidian:path=" + *n.Path)
	if n.Stat.Mtime > 0 {
		b.WriteString(" modified=" + time.UnixMilli(n.Stat.Mtime).UTC().Format(time.RFC3339))
	}
	b.WriteString(" -->\n")
	if len(n.Tags) > 0 {
		b.WriteString("*Tags: #" + strings.Join(n.Tags, ", #") + "*\n")
	}
	if len(n.Links) > 0 {
		b.WriteString("*Links: " + strings.Join(n.Links, ", ") + "*\n")
	}
	if n.Target != "" {
		b.WriteString("*Section: " + n.Target + "*\n")
	}
	b.WriteString("\n")
	b.WriteString(*n.Content)
	return markdown.Markdown(b.String()), true
}
