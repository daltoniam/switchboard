package openproject

import (
	"encoding/json"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
)

func TestRenderMarkdown_ExpandedFenceFallsBackToBoundedJSON(t *testing.T) {
	metadata := strings.Repeat("😀", 512)
	detail := workPackageDetail{
		workPackageSummary: workPackageSummary{
			ID: 42, ProjectID: 7, DisplayID: metadata, Subject: metadata, Status: metadata, Type: metadata, UpdatedAt: metadata,
		},
		CreatedAt: metadata, StartDate: &metadata, DueDate: &metadata,
		Description: strings.Repeat("`", 4000) + strings.Repeat("😀", 4000),
	}
	data, err := json.Marshal(detail)
	if err != nil || len(data) >= 40*1024 {
		t.Fatalf("regression fixture must fit the handler JSON bound: bytes=%d err=%v", len(data), err)
	}
	renderer := New().(mcp.MarkdownIntegration)
	rendered, supported := renderer.RenderMarkdown("openproject_get_work_package", data)
	if supported || rendered != "" {
		t.Fatalf("oversized fenced Markdown must fall back to bounded JSON: bytes=%d supported=%v", len(rendered), supported)
	}
	fields, ok := New().(mcp.FieldCompactionIntegration).CompactSpec("openproject_get_work_package")
	if !ok {
		t.Fatal("safe JSON fallback requires compaction")
	}
	projected, err := mcp.CompactJSON(data, fields)
	if err != nil || len(projected) >= 40*1024 {
		t.Fatalf("JSON fallback failed or exceeded cap: bytes=%d err=%v", len(projected), err)
	}
	var fallback workPackageDetail
	if err := json.Unmarshal(projected, &fallback); err != nil {
		t.Fatalf("JSON fallback is invalid: %v", err)
	}
	if fallback.Description != detail.Description {
		t.Fatalf("JSON fallback changed description: got %d bytes, want %d bytes", len(fallback.Description), len(detail.Description))
	}
}
