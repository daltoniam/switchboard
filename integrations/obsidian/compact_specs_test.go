package obsidian

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs)
}

func TestFieldCompactionSpecs_NoDuplicateTools(t *testing.T) {
	var sf compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &sf))
	assert.Equal(t, len(sf.Tools), len(fieldCompactionSpecs))
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for toolName := range fieldCompactionSpecs {
		_, ok := dispatch[toolName]
		assert.True(t, ok, "field compaction spec for %q has no dispatch handler", toolName)
	}
}

func TestFieldCompactionSpecs_EveryReadToolHasSpec(t *testing.T) {
	for _, tool := range tools {
		_, ok := fieldCompactionSpecs[tool.Name]
		if tool.Access == mcp.AccessRead {
			assert.True(t, ok, "read tool %s has no compaction spec", tool.Name)
		} else {
			assert.False(t, ok, "mutation tool %s must return its full confirmation", tool.Name)
		}
	}
}

func TestCompactSpec_Lookup(t *testing.T) {
	o := &obsidian{}
	fields, ok := o.CompactSpec("obsidian_search")
	require.True(t, ok)
	assert.NotEmpty(t, fields)
	_, ok = o.CompactSpec("obsidian_nonexistent")
	assert.False(t, ok)
	_, ok = o.MaxBytes("obsidian_nonexistent")
	assert.False(t, ok)
}

// TestFieldCompactionSpecs_ShapeParity runs each read tool against a fixture
// shaped like the Local REST API's response and checks the compacted output
// keeps what an agent needs for follow-up calls and drops the noise.
func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	tests := []struct {
		tool     string
		args     map[string]any
		fixture  string
		keep     []string
		drop     []string
		wantKeys []string
	}{
		{
			tool: "obsidian_list_files", fixture: `{"files":["Daily/","Inbox.md"]}`,
			keep:     []string{"Daily/", "Inbox.md"},
			wantKeys: []string{"path", "files", "count"},
		},
		{
			tool: "obsidian_read_note", args: map[string]any{"path": "Projects/Plan A.md"}, fixture: noteFixture,
			keep:     []string{"Projects/Plan A.md", "Ship it.", `"status":"draft"`, "work/plans", "Ideas.md", "1700000500000"},
			drop:     []string{"ctime", "unresolvedLinks"},
			wantKeys: []string{"path", "content", "frontmatter", "tags", "stat", "links"},
		},
		{
			tool: "obsidian_get_document_map", args: map[string]any{"path": "a.md"}, fixture: `{"version":"v1","headings":{"Plan":{"TODO":{}}},"blocks":["abc"],"frontmatterFields":["status"]}`,
			keep:     []string{`"version":"v1"`, "TODO", "abc", "status"},
			wantKeys: []string{"path", "version", "headings", "blocks", "frontmatterFields"},
		},
		{
			tool: "obsidian_search", args: map[string]any{"query": "budget"}, fixture: `[{"filename":"a.md","score":-0.1,"matches":[{"match":{"start":1,"end":5,"source":"content"},"context":"the budget"}]}]`,
			keep:     []string{"a.md", "the budget", "-0.1"},
			drop:     []string{"start", "source"},
			wantKeys: []string{"query", "total", "returned", "results"},
		},
		{
			tool: "obsidian_query", args: map[string]any{"jsonlogic": `{"glob":["*.md",{"var":"path"}]}`}, fixture: `[{"filename":"a.md","result":{"file.mtime":1}}]`,
			keep:     []string{"a.md", "file.mtime"},
			wantKeys: []string{"total", "returned", "results"},
		},
		{
			tool: "obsidian_list_tags", fixture: `{"tags":[{"name":"work","count":3}]}`,
			keep:     []string{"work", "3"},
			wantKeys: []string{"tags"},
		},
		{
			tool: "obsidian_list_commands", fixture: `{"commands":[{"id":"graph:open","name":"Graph view: Open graph view"}]}`,
			keep:     []string{"graph:open", "Graph view"},
			wantKeys: []string{"total", "commands"},
		},
		{
			tool: "obsidian_get_active_note", fixture: noteFixture,
			keep:     []string{"Projects/Plan A.md", "Ship it."},
			drop:     []string{"ctime", "unresolvedLinks"},
			wantKeys: []string{"path", "content"},
		},
	}
	covered := map[string]bool{}
	for _, tt := range tests {
		covered[tt.tool] = true
		t.Run(tt.tool, func(t *testing.T) {
			o := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, tt.fixture)
			})
			result := execute(t, o, tt.tool, tt.args)
			require.False(t, result.IsError, result.Data)
			fields, ok := fieldCompactionSpecs[mcp.ToolName(tt.tool)]
			require.True(t, ok)
			compacted, err := mcp.CompactJSON([]byte(result.Data), fields)
			require.NoError(t, err)
			text := string(compacted)
			for _, want := range tt.keep {
				assert.Contains(t, text, want)
			}
			for _, unwanted := range tt.drop {
				assert.NotContains(t, text, unwanted)
			}
			var top map[string]any
			require.NoError(t, json.Unmarshal(compacted, &top))
			for _, key := range tt.wantKeys {
				assert.Contains(t, top, key)
			}
		})
	}
	for name := range fieldCompactionSpecs {
		assert.True(t, covered[string(name)], "compaction spec for %s has no shape test", name)
	}
}
