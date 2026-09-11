package front

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderMarkdown_ConversationStaysJSON(t *testing.T) {
	f := &front{}
	_, ok := f.RenderMarkdown("front_get_conversation", []byte(`{"id":"cnv_1","subject":"Billing help"}`))
	assert.False(t, ok)
}

func TestRenderMarkdown_Messages(t *testing.T) {
	f := &front{}
	data := `{"_results":[{"id":"msg_1","type":"email","is_inbound":true,"created_at":1663597223,"subject":"Help","text":"I need help","author":{"email":"ada@example.com","username":"ada","first_name":"Ada","last_name":"Lovelace"}}]}`

	md, ok := f.RenderMarkdown("front_list_conversation_messages", []byte(data))
	require.True(t, ok)
	assert.Contains(t, string(md), "# Messages (1)")
	assert.Contains(t, string(md), "I need help")
	assert.Contains(t, string(md), "Ada Lovelace <ada@example.com>")
	assert.NotContains(t, string(md), "Next page_token:")
}

func TestRenderMarkdown_Messages_FromRecipient(t *testing.T) {
	f := &front{}
	data := `{"_results":[{"id":"msg_1","type":"email","is_inbound":true,"created_at":1663597223,"text":"I need help","author":null,"recipients":[{"name":"Ada","handle":"ada@example.com","role":"from"},{"handle":"support@acme.com","role":"to"}]}]}`

	md, ok := f.RenderMarkdown("front_list_conversation_messages", []byte(data))
	require.True(t, ok)
	assert.Contains(t, string(md), "Ada <ada@example.com>")
	assert.NotContains(t, string(md), "unknown")
}

func TestRenderMarkdown_Messages_NextPage(t *testing.T) {
	f := &front{}
	data := `{"_pagination":{"next":"https://api2.frontapp.com/conversations/cnv_1/messages?page_token=n1"},"_results":[{"id":"msg_1","type":"email","is_inbound":true,"created_at":1663597223,"text":"I need help","author":{"email":"ada@example.com"}}]}`

	md, ok := f.RenderMarkdown("front_list_conversation_messages", []byte(data))
	require.True(t, ok)
	assert.Contains(t, string(md), "Next page_token: https://api2.frontapp.com/conversations/cnv_1/messages?page_token=n1")
}

func TestRenderMarkdown_UnknownTool(t *testing.T) {
	f := &front{}
	_, ok := f.RenderMarkdown("front_search_conversations", []byte(`{}`))
	assert.False(t, ok)
}

func TestRenderMarkdown_InvalidJSON(t *testing.T) {
	f := &front{}
	_, ok := f.RenderMarkdown("front_list_conversation_messages", []byte(`not json`))
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
		"front_list_conversation_messages",
	}
	for _, name := range markdownTools {
		assert.True(t, toolNames[name], "RenderMarkdown handles %q but it's not in Tools()", name)
	}
}
