package imessage

import (
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
	var specFile compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &specFile))
	assert.Equal(t, len(specFile.Tools), len(fieldCompactionSpecs))
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for toolName := range fieldCompactionSpecs {
		_, ok := dispatch[toolName]
		assert.True(t, ok, "compaction spec %s has no dispatch handler", toolName)
	}
}

func TestFieldCompactionSpecs_ReadToolsCovered(t *testing.T) {
	m := New().(*imessage)
	for _, name := range []mcp.ToolName{"imessage_list_chats", "imessage_get_chat_messages", "imessage_search_messages", "imessage_list_unread"} {
		fields, ok := m.CompactSpec(name)
		assert.True(t, ok, name)
		assert.NotEmpty(t, fields, name)
	}
	_, ok := m.CompactSpec("imessage_send_message")
	assert.False(t, ok, "mutation tools must not be compacted")
}

func TestFieldCompactionSpecs_Shape(t *testing.T) {
	m, _ := newConfigured(t, nil)
	tests := []struct {
		tool mcp.ToolName
		args map[string]any
		keep []string
		drop []string
	}{
		{tool: "imessage_list_chats", keep: []string{"chat_id", "unread_count", "last_message"}, drop: []string{"guid", "identifier"}},
		{tool: "imessage_get_chat_messages", args: map[string]any{"chat_id": 2}, keep: []string{"reactions", "attachments", "reply_to"}, drop: []string{"bytes"}},
		{tool: "imessage_search_messages", args: map[string]any{"query": "snacks"}, keep: []string{"chat_name", "text"}, drop: []string{"guid", "bytes"}},
		{tool: "imessage_list_unread", keep: []string{"chat_name", "text"}, drop: []string{"guid", "from_me"}},
	}
	for _, tt := range tests {
		t.Run(string(tt.tool), func(t *testing.T) {
			result, err := m.Execute(t.Context(), tt.tool, tt.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			fields, ok := m.CompactSpec(tt.tool)
			require.True(t, ok)
			out, err := mcp.CompactJSON([]byte(result.Data), fields)
			require.NoError(t, err)
			for _, k := range tt.keep {
				assert.Contains(t, string(out), `"`+k+`"`)
			}
			for _, k := range tt.drop {
				assert.NotContains(t, string(out), `"`+k+`"`)
			}
		})
	}
}
