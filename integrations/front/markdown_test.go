package front

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderMarkdown_Conversation(t *testing.T) {
	f := &front{}
	data := `{"id":"cnv_1","subject":"Billing help","status":"unassigned","created_at":1663597223,"assignee":{"email":"sam@example.com","username":"sam","first_name":"Sam","last_name":"Support"},"recipient":{"name":"Ada","handle":"ada@example.com"}}`

	md, ok := f.RenderMarkdown("front_get_conversation", []byte(data))
	require.True(t, ok)
	assert.Contains(t, string(md), "<!-- front:conversation_id=cnv_1 status=unassigned -->")
	assert.Contains(t, string(md), "# Billing help")
	assert.Contains(t, string(md), "Ada <ada@example.com>")
}

func TestRenderMarkdown_Messages(t *testing.T) {
	f := &front{}
	data := `{"_results":[{"id":"msg_1","type":"email","is_inbound":true,"created_at":1663597223,"subject":"Help","text":"I need help","author":{"email":"ada@example.com","username":"ada","first_name":"Ada","last_name":"Lovelace"}}]}`

	md, ok := f.RenderMarkdown("front_list_conversation_messages", []byte(data))
	require.True(t, ok)
	assert.Contains(t, string(md), "# Messages (1)")
	assert.Contains(t, string(md), "I need help")
	assert.Contains(t, string(md), "Ada Lovelace <ada@example.com>")
}

func TestRenderMarkdown_UnknownTool(t *testing.T) {
	f := &front{}
	_, ok := f.RenderMarkdown("front_search_conversations", []byte(`{}`))
	assert.False(t, ok)
}

func TestRenderMarkdown_InvalidJSON(t *testing.T) {
	f := &front{}
	_, ok := f.RenderMarkdown("front_get_conversation", []byte(`not json`))
	assert.False(t, ok)
}

func TestRenderMarkdown_ToolsCovered(t *testing.T) {
	adapter := New()
	md, ok := adapter.(mcp.MarkdownIntegration)
	require.True(t, ok, "adapter should implement MarkdownIntegration")

	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range adapter.Tools() {
		toolNames[tool.Name] = true
	}

	for name := range toolNames {
		md.RenderMarkdown(name, []byte("{}"))
	}

	markdownTools := []mcp.ToolName{
		"front_get_conversation",
		"front_list_conversation_messages",
	}
	for _, name := range markdownTools {
		assert.True(t, toolNames[name], "RenderMarkdown handles %q but it's not in Tools()", name)
	}
}
