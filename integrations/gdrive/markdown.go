package gdrive

import (
	"encoding/json"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

// ── Semantic domain types ────────────────────────────────────────────

type renderedFile struct {
	ID           string
	Name         string
	MimeType     string
	Description  string
	Size         string
	CreatedTime  string
	ModifiedTime string
	WebViewLink  string
	Owners       []renderedUser
	Trashed      bool
	Starred      bool
	Shared       bool
}

type renderedUser struct {
	Email string
	Name  string
}

type renderedDownload struct {
	ContentType   string
	Bytes         int
	Truncated     bool
	Content       string
	ContentBase64 string
}

// ── Parse boundary ──────────────────────────────────────────────────

var markdownRenderers = map[mcp.ToolName]func([]byte) (markdown.Markdown, bool){
	"gdrive_get_file":      renderFileMD,
	"gdrive_export_file":   renderDownloadMD,
	"gdrive_download_file": renderDownloadMD,
}

func (g *gdrive) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	if fn, ok := markdownRenderers[toolName]; ok {
		return fn(data)
	}
	return "", false
}

// ── Raw JSON parse types ────────────────────────────────────────────

type rawFile struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	MimeType     string    `json:"mimeType"`
	Description  string    `json:"description"`
	Size         string    `json:"size"`
	CreatedTime  string    `json:"createdTime"`
	ModifiedTime string    `json:"modifiedTime"`
	WebViewLink  string    `json:"webViewLink"`
	Owners       []rawUser `json:"owners"`
	Trashed      bool      `json:"trashed"`
	Starred      bool      `json:"starred"`
	Shared       bool      `json:"shared"`
}

type rawUser struct {
	EmailAddress string `json:"emailAddress"`
	DisplayName  string `json:"displayName"`
}

type rawDownload struct {
	ContentType   string `json:"content_type"`
	Bytes         int    `json:"bytes"`
	Truncated     bool   `json:"truncated"`
	Content       string `json:"content"`
	ContentBase64 string `json:"content_base64"`
}

// ── Rendering ───────────────────────────────────────────────────────

func renderFileMD(data []byte) (markdown.Markdown, bool) {
	var raw rawFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}
	if raw.ID == "" {
		// Not a file resource (could be a different envelope) — skip.
		return "", false
	}

	f := renderedFile{
		ID:           raw.ID,
		Name:         raw.Name,
		MimeType:     raw.MimeType,
		Description:  raw.Description,
		Size:         raw.Size,
		CreatedTime:  raw.CreatedTime,
		ModifiedTime: raw.ModifiedTime,
		WebViewLink:  raw.WebViewLink,
		Trashed:      raw.Trashed,
		Starred:      raw.Starred,
		Shared:       raw.Shared,
	}
	for _, o := range raw.Owners {
		f.Owners = append(f.Owners, renderedUser{Email: o.EmailAddress, Name: o.DisplayName})
	}
	return fileToMarkdown(f), true
}

func fileToMarkdown(f renderedFile) markdown.Markdown {
	b := markdown.NewBuilder()
	b.Metadata("gdrive", "file_id", f.ID, "mime_type", f.MimeType)

	title := f.Name
	if title == "" {
		title = "(no name)"
	}
	b.Heading(1, title)

	if f.MimeType != "" {
		b.Attribution("Type: " + f.MimeType)
	}
	if f.Size != "" {
		b.Attribution("Size: " + f.Size + " bytes")
	}
	if f.ModifiedTime != "" {
		b.Attribution("Modified: " + f.ModifiedTime)
	}
	if f.CreatedTime != "" {
		b.Attribution("Created: " + f.CreatedTime)
	}
	if len(f.Owners) > 0 {
		b.Attribution("Owners: " + formatUsers(f.Owners))
	}
	if f.WebViewLink != "" {
		b.Attribution("Link: " + f.WebViewLink)
	}
	flags := []string{}
	if f.Trashed {
		flags = append(flags, "trashed")
	}
	if f.Starred {
		flags = append(flags, "starred")
	}
	if f.Shared {
		flags = append(flags, "shared")
	}
	if len(flags) > 0 {
		b.Attribution("Flags: " + strings.Join(flags, ", "))
	}

	if f.Description != "" {
		b.BlankLine()
		b.Heading(2, "Description")
		b.WriteMarkdown(markdown.Markdown(f.Description))
		if !strings.HasSuffix(f.Description, "\n") {
			b.Raw("\n")
		}
	}

	return b.Build()
}

func formatUsers(users []renderedUser) string {
	parts := make([]string, 0, len(users))
	for _, u := range users {
		switch {
		case u.Name != "" && u.Email != "":
			parts = append(parts, fmt.Sprintf("%s <%s>", u.Name, u.Email))
		case u.Email != "":
			parts = append(parts, u.Email)
		default:
			parts = append(parts, u.Name)
		}
	}
	return strings.Join(parts, ", ")
}

// renderDownloadMD renders the JSON envelope produced by downloadFile /
// exportFile. For text content types we surface the content directly; for
// binary we render a stub with content type and size so the LLM knows it
// got a file rather than a giant base64 blob mid-conversation.
func renderDownloadMD(data []byte) (markdown.Markdown, bool) {
	var raw rawDownload
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}
	if raw.ContentType == "" {
		return "", false
	}

	d := renderedDownload(raw)
	return downloadToMarkdown(d), true
}

func downloadToMarkdown(d renderedDownload) markdown.Markdown {
	b := markdown.NewBuilder()
	b.Metadata("gdrive", "content_type", d.ContentType, "bytes", fmt.Sprintf("%d", d.Bytes))

	if d.Truncated {
		b.Attribution("Note: content was truncated to fit max_bytes")
		b.BlankLine()
	}

	if d.Content != "" {
		// Text content — emit as a code block (or markdown body for
		// markdown content type) so it round-trips cleanly.
		if strings.Contains(strings.ToLower(d.ContentType), "markdown") {
			b.WriteMarkdown(markdown.Markdown(d.Content))
			if !strings.HasSuffix(d.Content, "\n") {
				b.Raw("\n")
			}
			return b.Build()
		}
		lang := codeLang(d.ContentType)
		b.Raw("```" + lang + "\n")
		b.Raw(d.Content)
		if !strings.HasSuffix(d.Content, "\n") {
			b.Raw("\n")
		}
		b.Raw("```\n")
		return b.Build()
	}

	// Binary — describe but don't dump base64 into the LLM context.
	b.Heading(2, "Binary content")
	b.Raw(fmt.Sprintf("This file is binary (%s, %d bytes). The base64 payload is in `content_base64` of the raw response.\n", d.ContentType, d.Bytes))
	return b.Build()
}

func codeLang(contentType string) string {
	ct := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	switch ct {
	case "text/csv":
		return "csv"
	case "text/html":
		return "html"
	case "application/json":
		return "json"
	case "application/xml", "text/xml":
		return "xml"
	case "text/x-python", "application/x-python":
		return "python"
	case "text/javascript", "application/javascript":
		return "javascript"
	case "application/x-yaml", "application/yaml", "text/yaml":
		return "yaml"
	}
	return ""
}
