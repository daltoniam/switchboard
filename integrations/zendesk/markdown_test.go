package zendesk

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderMarkdown_Ticket(t *testing.T) {
	z := &zendesk{}
	md, ok := z.RenderMarkdown("zendesk_get_ticket", []byte(`{"ticket":{"id":42,"subject":"Cannot login","status":"open","priority":"high","type":"incident","requester_id":10,"assignee_id":20,"group_id":30,"tags":["auth"],"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z","description":"Password reset failed"}}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "#42: Cannot login")
	assert.Contains(t, string(md), "Status: open")
	assert.Contains(t, string(md), "Password reset failed")
}

func TestRenderMarkdown_Comments(t *testing.T) {
	z := &zendesk{}
	md, ok := z.RenderMarkdown("zendesk_list_ticket_comments", []byte(`{"comments":[{"author_id":10,"created_at":"2024-01-01T00:00:00Z","public":true,"body":"hello"},{"author_id":20,"created_at":"2024-01-02T00:00:00Z","public":false,"body":"internal note"}]}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "Comments (2)")
	assert.Contains(t, string(md), "hello")
	assert.Contains(t, string(md), "internal note")
}

func TestRenderMarkdown_CommentsEmpty(t *testing.T) {
	z := &zendesk{}
	md, ok := z.RenderMarkdown("zendesk_list_ticket_comments", []byte(`{"comments":[]}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "No comments")
}

func TestRenderMarkdown_Article(t *testing.T) {
	z := &zendesk{}
	md, ok := z.RenderMarkdown("zendesk_get_article", []byte(`{"article":{"id":70,"title":"Reset password","locale":"en-us","author_id":20,"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z","body":"<p>Click <strong>reset</strong></p>"}}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "Reset password")
	assert.Contains(t, string(md), "reset")
}

func TestRenderMarkdown_UnknownTool(t *testing.T) {
	z := &zendesk{}
	_, ok := z.RenderMarkdown("zendesk_list_tickets", []byte(`{}`))
	assert.False(t, ok)
}

func TestRenderMarkdown_InvalidJSON(t *testing.T) {
	z := &zendesk{}
	_, ok := z.RenderMarkdown("zendesk_get_ticket", []byte(`not json`))
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
		"zendesk_get_ticket",
		"zendesk_list_ticket_comments",
		"zendesk_get_article",
	}
	for _, name := range markdownTools {
		assert.True(t, toolNames[name], "RenderMarkdown handles %q but it's not in Tools()", name)
	}
}
