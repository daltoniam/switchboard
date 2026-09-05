package paperless

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs(t *testing.T) {
	var specFile compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &specFile))
	require.NotEmpty(t, fieldCompactionSpecs)
	assert.Equal(t, len(specFile.Tools), len(fieldCompactionSpecs))

	for toolName := range fieldCompactionSpecs {
		_, ok := dispatch[toolName]
		assert.True(t, ok, "field compaction spec for %q has no dispatch handler", toolName)
	}
}

func TestFieldCompactionSpecExcludesMutations(t *testing.T) {
	p := &paperless{}
	fields, ok := p.CompactSpec("paperless_list_documents")
	assert.True(t, ok)
	assert.NotEmpty(t, fields)
	_, ok = p.CompactSpec("paperless_update_document")
	assert.False(t, ok)
}

func TestFieldCompactionShapes(t *testing.T) {
	tests := []struct {
		name  string
		tool  mcp.ToolName
		input string
		want  string
	}{
		{
			name:  "document list excludes OCR content",
			tool:  "paperless_list_documents",
			input: `{"count":1,"results":[{"id":7,"title":"Invoice","content":"sensitive OCR text"}]}`,
			want:  `{"count":1,"results":[{"id":7,"title":"Invoice"}]}`,
		},
		{
			name:  "document search excludes OCR content",
			tool:  "paperless_search_documents",
			input: `{"count":1,"results":[{"id":7,"title":"Invoice","content":"sensitive OCR text","__search_hit__":{"content":"also sensitive"}}]}`,
			want:  `{"count":1,"results":[{"id":7,"title":"Invoice"}]}`,
		},
		{
			name:  "document get excludes OCR content",
			tool:  "paperless_get_document",
			input: `{"id":7,"title":"Invoice","content":"sensitive OCR text"}`,
			want:  `{"id":7,"title":"Invoice"}`,
		},
		{
			name:  "OCR text page preserves pagination envelope",
			tool:  "paperless_get_document_ocr_text",
			input: `{"document_id":7,"offset":2,"limit":3,"total_chars":6,"content":"cde","has_more":true,"next_offset":5,"ignored":"value"}`,
			want:  `{"document_id":7,"offset":2,"limit":3,"total_chars":6,"content":"cde","has_more":true,"next_offset":5}`,
		},
		{
			name:  "task status counts use upstream fields",
			tool:  "paperless_get_task_status_counts",
			input: `{"all":8,"needs_attention":2,"in_progress":3,"completed":3,"pending":99}`,
			want:  `{"all":8,"completed":3,"in_progress":3,"needs_attention":2}`,
		},
		{
			name:  "task list uses version 10 task fields",
			tool:  "paperless_list_tasks",
			input: `{"count":1,"results":[{"id":7,"task_id":"celery-id","task_type":"consume_file","status":"SUCCESS","date_created":"2025-01-01T00:00:00Z","date_started":"2025-01-01T00:01:00Z","date_done":"2025-01-01T00:02:00Z","result_data":{"document_id":1}}]}`,
			want:  `{"count":1,"results":[{"date_created":"2025-01-01T00:00:00Z","date_done":"2025-01-01T00:02:00Z","date_started":"2025-01-01T00:01:00Z","id":7,"status":"SUCCESS","task_id":"celery-id","task_type":"consume_file"}]}`,
		},
		{
			name:  "workflow preserves nested triggers and actions",
			tool:  "paperless_get_workflow",
			input: `{"id":4,"name":"Route invoices","order":1,"enabled":true,"triggers":[{"id":5,"type":1,"filter_filename":"*.pdf"}],"actions":[{"id":6,"type":1,"assign_tags":[2]}],"ignored":true}`,
			want:  `{"actions":[{"assign_tags":[2],"id":6,"type":1}],"enabled":true,"id":4,"name":"Route invoices","order":1,"triggers":[{"filter_filename":"*.pdf","id":5,"type":1}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields, ok := New().(*paperless).CompactSpec(tt.tool)
			require.True(t, ok)
			got, err := mcp.CompactJSON([]byte(tt.input), fields)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(got))
		})
	}
}
