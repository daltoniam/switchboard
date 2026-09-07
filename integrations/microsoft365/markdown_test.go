package microsoft365

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	assert.True(t, toolNames["microsoft365_get_message"])
}

func TestRenderMarkdown_GetMessage(t *testing.T) {
	payload := []byte(`{
		"id":"m1",
		"subject":"Budget review",
		"receivedDateTime":"2024-03-15T10:00:00Z",
		"from":{"emailAddress":{"name":"Ada","address":"ada@contoso.com"}},
		"toRecipients":[{"emailAddress":{"name":"Grace","address":"grace@contoso.com"}}],
		"body":{"contentType":"text","content":"Please review the numbers."}
	}`)
	m := &m365{}
	md, ok := m.RenderMarkdown("microsoft365_get_message", payload)
	require.True(t, ok)
	assert.Contains(t, string(md), "Budget review")
	assert.Contains(t, string(md), "Ada <ada@contoso.com>")
	assert.Contains(t, string(md), "Please review the numbers.")
}

func TestRenderMarkdown_UnknownTool(t *testing.T) {
	m := &m365{}
	_, ok := m.RenderMarkdown("microsoft365_list_messages", []byte(`{}`))
	assert.False(t, ok)
}
