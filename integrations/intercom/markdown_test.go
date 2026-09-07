package intercom

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderMarkdown_Conversation(t *testing.T) {
	c := &intercom{}
	data := `{"id":"c1","title":"Billing help","state":"open","priority":"not_priority","created_at":1663597223,"updated_at":1663597260,"admin_assignee_id":"991","source":{"subject":"Billing help","body":"<p>I need help</p>","author":{"name":"Ada","email":"ada@example.com","type":"user"}},"contacts":{"contacts":[{"name":"Ada","email":"ada@example.com"}]},"conversation_parts":{"conversation_parts":[{"part_type":"comment","body":"<p>We can help</p>","created_at":1663597260,"author":{"name":"Sam","email":"sam@example.com","type":"admin"}}]}}`

	md, ok := c.RenderMarkdown("intercom_get_conversation", []byte(data))
	require.True(t, ok)
	assert.Contains(t, string(md), "<!-- intercom:conversation_id=c1 state=open -->")
	assert.Contains(t, string(md), "# Billing help")
	assert.Contains(t, string(md), "I need help")
	assert.Contains(t, string(md), "We can help")
	assert.Contains(t, string(md), "**Ada <ada@example.com>**")
}

func TestRenderMarkdown_Article(t *testing.T) {
	c := &intercom{}
	data := `{"id":"a1","title":"Refunds","state":"published","url":"https://help.example.com/a1","body":"<p>How to get a refund</p>","author":{"name":"Docs","email":"docs@example.com"}}`

	md, ok := c.RenderMarkdown("intercom_get_article", []byte(data))
	require.True(t, ok)
	assert.Contains(t, string(md), "<!-- intercom:article_id=a1 state=published -->")
	assert.Contains(t, string(md), "# Refunds")
	assert.Contains(t, string(md), "How to get a refund")
}

func TestRenderMarkdown_UnknownTool(t *testing.T) {
	c := &intercom{}
	_, ok := c.RenderMarkdown("intercom_search_conversations", []byte(`{}`))
	assert.False(t, ok)
}

func TestRenderMarkdown_InvalidJSON(t *testing.T) {
	c := &intercom{}
	_, ok := c.RenderMarkdown("intercom_get_conversation", []byte(`not json`))
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
		"intercom_get_conversation",
		"intercom_get_article",
	}
	for _, name := range markdownTools {
		assert.True(t, toolNames[name], "RenderMarkdown handles %q but it's not in Tools()", name)
	}
}
