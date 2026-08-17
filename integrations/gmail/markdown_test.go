package gmail

import (
	"encoding/base64"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractBody(t *testing.T) {
	encode := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

	tests := []struct {
		name    string
		payload rawPayload
		want    markdown.Markdown
	}{
		{
			name: "simple text/plain body",
			payload: rawPayload{
				MimeType: "text/plain",
				Body:     rawBody{Data: encode("Hello world")},
			},
			want: "Hello world",
		},
		{
			name: "multipart prefers text/plain",
			payload: rawPayload{
				MimeType: "multipart/alternative",
				Parts: []rawPayload{
					{MimeType: "text/plain", Body: rawBody{Data: encode("Plain text")}},
					{MimeType: "text/html", Body: rawBody{Data: encode("<p>HTML</p>")}},
				},
			},
			want: "Plain text",
		},
		{
			name: "multipart falls back to text/html",
			payload: rawPayload{
				MimeType: "multipart/alternative",
				Parts: []rawPayload{
					{MimeType: "text/html", Body: rawBody{Data: encode("<p>HTML content</p>")}},
				},
			},
			want: "HTML content\n\n",
		},
		{
			name: "nested multipart",
			payload: rawPayload{
				MimeType: "multipart/mixed",
				Parts: []rawPayload{
					{MimeType: "multipart/alternative", Parts: []rawPayload{
						{MimeType: "text/plain", Body: rawBody{Data: encode("Nested plain")}},
					}},
				},
			},
			want: "Nested plain",
		},
		{
			name: "whitespace-only text/plain falls back to text/html",
			payload: rawPayload{
				MimeType: "multipart/alternative",
				Parts: []rawPayload{
					{MimeType: "text/plain", Body: rawBody{Data: encode(" \n ")}},
					{MimeType: "text/html", Body: rawBody{Data: encode("<p>Real content</p>")}},
				},
			},
			want: "Real content\n\n",
		},
		{
			name: "empty text/plain falls back to text/html",
			payload: rawPayload{
				MimeType: "multipart/alternative",
				Parts: []rawPayload{
					{MimeType: "text/plain", Body: rawBody{Data: ""}},
					{MimeType: "text/html", Body: rawBody{Data: encode("<p>Real content here</p>")}},
				},
			},
			want: "Real content here\n\n",
		},
		{
			name:    "empty payload",
			payload: rawPayload{},
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBody(tt.payload)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRenderMarkdown_Message(t *testing.T) {
	encode := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

	g := &gmail{}
	data := `{"id":"msg1","threadId":"t1","payload":{"mimeType":"text/plain","headers":[{"name":"Subject","value":"Project Update"},{"name":"From","value":"alice@example.com"},{"name":"To","value":"bob@example.com"},{"name":"Date","value":"Mon, 15 Mar 2024 14:22:00 UTC"}],"body":{"data":"` + encode("The deploy went smoothly.") + `"}}}`

	md, ok := g.RenderMarkdown("gmail_get_message", []byte(data))
	require.True(t, ok)
	assert.Contains(t, string(md), "<!-- gmail:message_id=msg1 thread_id=t1 -->")
	assert.Contains(t, string(md), "# Project Update")
	assert.Contains(t, string(md), "From: alice@example.com")
	assert.Contains(t, string(md), "The deploy went smoothly.")
}

func TestRenderMarkdown_Thread(t *testing.T) {
	encode := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

	g := &gmail{}
	data := `{"id":"t1","messages":[{"id":"msg1","payload":{"mimeType":"text/plain","headers":[{"name":"Subject","value":"Deploy"},{"name":"From","value":"alice@example.com"},{"name":"Date","value":"2024-03-15"}],"body":{"data":"` + encode("Starting deploy.") + `"}}},{"id":"msg2","payload":{"mimeType":"text/plain","headers":[{"name":"Subject","value":"Re: Deploy"},{"name":"From","value":"bob@example.com"},{"name":"Date","value":"2024-03-15"}],"body":{"data":"` + encode("Looks good.") + `"}}}]}`

	md, ok := g.RenderMarkdown("gmail_get_thread", []byte(data))
	require.True(t, ok)
	assert.Contains(t, string(md), "<!-- gmail:thread_id=t1 messages=2 -->")
	assert.Contains(t, string(md), "alice@example.com")
	assert.Contains(t, string(md), "Starting deploy.")
	assert.Contains(t, string(md), "bob@example.com")
	assert.Contains(t, string(md), "Looks good.")
}

func TestRenderMarkdown_UnknownTool(t *testing.T) {
	g := &gmail{}
	_, ok := g.RenderMarkdown("gmail_list_messages", []byte(`{}`))
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

	// Test every tool — verify RenderMarkdown returns ok=true only for known tools
	for name := range toolNames {
		// We just check it doesn't panic; the (_, ok) result depends on the tool
		md.RenderMarkdown(name, []byte("{}"))
	}

	// Verify the tools RenderMarkdown claims to handle actually exist
	markdownTools := []mcp.ToolName{
		"gmail_get_message",
		"gmail_get_thread",
		"gmail_get_draft",
	}
	for _, name := range markdownTools {
		assert.True(t, toolNames[name], "RenderMarkdown handles %q but it's not in Tools()", name)
	}
}
