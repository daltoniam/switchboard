package openproject

import (
	"encoding/json"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

var _ mcp.MarkdownIntegration = (*openproject)(nil)

func (o *openproject) RenderMarkdown(name mcp.ToolName, data []byte) (mcp.Markdown, bool) {
	if name != "openproject_get_work_package" {
		return "", false
	}
	var detail workPackageDetail
	if json.Unmarshal(data, &detail) != nil || detail.ID < 1 || detail.ProjectID < 1 {
		return "", false
	}
	builder := markdown.NewBuilder()
	builder.Heading(1, "OpenProject work package")
	builder.Raw(fmt.Sprintf("Work package ID: %d\nProject ID: %d\n\n", detail.ID, detail.ProjectID))
	var content strings.Builder
	fmt.Fprintf(&content, "Display ID: %s\nSubject: %s\nStatus: %s\nType: %s\nUpdated: %s\nCreated: %s\n", detail.DisplayID, detail.Subject, detail.Status, detail.Type, detail.UpdatedAt, detail.CreatedAt)
	if detail.StartDate != nil {
		fmt.Fprintf(&content, "Start date: %s\n", *detail.StartDate)
	}
	if detail.DueDate != nil {
		fmt.Fprintf(&content, "Due date: %s\n", *detail.DueDate)
	}
	content.WriteString("\nUntrusted description text:\n")
	content.WriteString(detail.Description)
	text := content.String()
	longest, run := 0, 0
	for _, char := range text {
		if char != '`' {
			run = 0
			continue
		}
		run++
		longest = max(longest, run)
	}
	fence := strings.Repeat("`", max(3, longest+1))
	builder.Raw(fence + "text\n" + text + "\n" + fence + "\n")
	if detail.Truncated || detail.DescriptionTruncated {
		builder.Raw("\nContent truncated by the configured output bounds.\n")
	}
	rendered := builder.Build()
	if len(rendered) >= 40*1024 {
		return "", false
	}
	return rendered, true
}
