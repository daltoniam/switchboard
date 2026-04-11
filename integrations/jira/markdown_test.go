package jira

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestADFToMarkdown(t *testing.T) {
	tests := []struct {
		name string
		adf  map[string]any
		want string
	}{
		{name: "nil", adf: nil, want: ""},
		{
			name: "simple paragraph",
			adf: map[string]any{"type": "doc", "version": float64(1), "content": []any{
				map[string]any{"type": "paragraph", "content": []any{
					map[string]any{"type": "text", "text": "Hello world"},
				}},
			}},
			want: "Hello world\n\n",
		},
		{
			name: "heading level 2",
			adf: map[string]any{"type": "doc", "content": []any{
				map[string]any{"type": "heading", "attrs": map[string]any{"level": float64(2)}, "content": []any{
					map[string]any{"type": "text", "text": "Section"},
				}},
			}},
			want: "## Section\n\n",
		},
		{
			name: "bold text",
			adf: map[string]any{"type": "doc", "content": []any{
				map[string]any{"type": "paragraph", "content": []any{
					map[string]any{"type": "text", "text": "bold", "marks": []any{
						map[string]any{"type": "strong"},
					}},
				}},
			}},
			want: "**bold**\n\n",
		},
		{
			name: "italic text",
			adf: map[string]any{"type": "doc", "content": []any{
				map[string]any{"type": "paragraph", "content": []any{
					map[string]any{"type": "text", "text": "italic", "marks": []any{
						map[string]any{"type": "em"},
					}},
				}},
			}},
			want: "*italic*\n\n",
		},
		{
			name: "inline code",
			adf: map[string]any{"type": "doc", "content": []any{
				map[string]any{"type": "paragraph", "content": []any{
					map[string]any{"type": "text", "text": "code", "marks": []any{
						map[string]any{"type": "code"},
					}},
				}},
			}},
			want: "`code`\n\n",
		},
		{
			name: "link",
			adf: map[string]any{"type": "doc", "content": []any{
				map[string]any{"type": "paragraph", "content": []any{
					map[string]any{"type": "text", "text": "click", "marks": []any{
						map[string]any{"type": "link", "attrs": map[string]any{"href": "https://example.com"}},
					}},
				}},
			}},
			want: "[click](https://example.com)\n\n",
		},
		{
			name: "strikethrough",
			adf: map[string]any{"type": "doc", "content": []any{
				map[string]any{"type": "paragraph", "content": []any{
					map[string]any{"type": "text", "text": "removed", "marks": []any{
						map[string]any{"type": "strike"},
					}},
				}},
			}},
			want: "~~removed~~\n\n",
		},
		{
			name: "bullet list",
			adf: map[string]any{"type": "doc", "content": []any{
				map[string]any{"type": "bulletList", "content": []any{
					map[string]any{"type": "listItem", "content": []any{
						map[string]any{"type": "paragraph", "content": []any{
							map[string]any{"type": "text", "text": "One"},
						}},
					}},
					map[string]any{"type": "listItem", "content": []any{
						map[string]any{"type": "paragraph", "content": []any{
							map[string]any{"type": "text", "text": "Two"},
						}},
					}},
				}},
			}},
			want: "- One\n- Two\n\n",
		},
		{
			name: "ordered list",
			adf: map[string]any{"type": "doc", "content": []any{
				map[string]any{"type": "orderedList", "content": []any{
					map[string]any{"type": "listItem", "content": []any{
						map[string]any{"type": "paragraph", "content": []any{
							map[string]any{"type": "text", "text": "First"},
						}},
					}},
					map[string]any{"type": "listItem", "content": []any{
						map[string]any{"type": "paragraph", "content": []any{
							map[string]any{"type": "text", "text": "Second"},
						}},
					}},
				}},
			}},
			want: "1. First\n2. Second\n\n",
		},
		{
			name: "code block with language",
			adf: map[string]any{"type": "doc", "content": []any{
				map[string]any{"type": "codeBlock", "attrs": map[string]any{"language": "go"}, "content": []any{
					map[string]any{"type": "text", "text": "func main() {}"},
				}},
			}},
			want: "```go\nfunc main() {}\n```\n\n",
		},
		{
			name: "blockquote",
			adf: map[string]any{"type": "doc", "content": []any{
				map[string]any{"type": "blockquote", "content": []any{
					map[string]any{"type": "paragraph", "content": []any{
						map[string]any{"type": "text", "text": "A quote"},
					}},
				}},
			}},
			want: "> A quote\n\n",
		},
		{
			name: "rule (divider)",
			adf: map[string]any{"type": "doc", "content": []any{
				map[string]any{"type": "rule"},
			}},
			want: "---\n\n",
		},
		{
			name: "mention",
			adf: map[string]any{"type": "doc", "content": []any{
				map[string]any{"type": "paragraph", "content": []any{
					map[string]any{"type": "mention", "attrs": map[string]any{"text": "@alice"}},
				}},
			}},
			want: "**@alice**\n\n",
		},
		{
			name: "mixed paragraph",
			adf: map[string]any{"type": "doc", "content": []any{
				map[string]any{"type": "paragraph", "content": []any{
					map[string]any{"type": "text", "text": "Hello "},
					map[string]any{"type": "text", "text": "bold", "marks": []any{
						map[string]any{"type": "strong"},
					}},
					map[string]any{"type": "text", "text": " world"},
				}},
			}},
			want: "Hello **bold** world\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adfToMarkdown(tt.adf)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRenderMarkdown_Issue(t *testing.T) {
	j := &jira{}
	data := `{"key":"PROJ-123","fields":{"summary":"Fix auth timeout","status":{"name":"In Progress"},"assignee":{"displayName":"Alice","accountId":"u1"},"reporter":{"displayName":"Bob"},"priority":{"name":"High"},"issuetype":{"name":"Bug"},"description":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"The auth service is timing out."}]}]},"created":"2024-01-15T10:00:00Z","updated":"2024-03-10T14:00:00Z","labels":["backend"],"components":[{"name":"auth"}],"fixVersions":[{"name":"v2.1"}]}}`

	md, ok := j.RenderMarkdown("jira_get_issue", []byte(data))
	require.True(t, ok)
	assert.Contains(t, string(md), "<!-- jira:key=PROJ-123 -->")
	assert.Contains(t, string(md), "# PROJ-123: Fix auth timeout")
	assert.Contains(t, string(md), "Status: In Progress")
	assert.Contains(t, string(md), "Assignee: Alice")
	assert.Contains(t, string(md), "Priority: High")
	assert.Contains(t, string(md), "The auth service is timing out.")
}

func TestRenderMarkdown_Comments(t *testing.T) {
	j := &jira{}
	data := `{"comments":[{"id":"1","body":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Reproduced in staging."}]}]},"author":{"displayName":"Alice"},"created":"2024-01-16T09:00:00Z","updated":"2024-01-16T09:00:00Z"},{"id":"2","body":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Working on a fix."}]}]},"author":{"displayName":"Bob"},"created":"2024-01-16T10:30:00Z","updated":"2024-01-16T10:30:00Z"}]}`

	md, ok := j.RenderMarkdown("jira_list_comments", []byte(data))
	require.True(t, ok)
	assert.Contains(t, string(md), "## Comments (2)")
	assert.Contains(t, string(md), "**Alice**")
	assert.Contains(t, string(md), "Reproduced in staging.")
	assert.Contains(t, string(md), "**Bob**")
	assert.Contains(t, string(md), "Working on a fix.")
}

func TestRenderMarkdown_IssueWithStringDescription(t *testing.T) {
	j := &jira{}
	// Jira Server/DC may return description as a plain string, not ADF.
	data := `{"key":"PROJ-99","fields":{"summary":"Legacy issue","status":{"name":"Open"},"priority":{"name":"Medium"},"issuetype":{"name":"Task"},"description":"This is a plain text description.","created":"2024-01-01","updated":"2024-01-01"}}`

	md, ok := j.RenderMarkdown("jira_get_issue", []byte(data))
	require.True(t, ok)
	assert.Contains(t, string(md), "This is a plain text description.")
}

func TestRenderMarkdown_IssueWithNullDescription(t *testing.T) {
	j := &jira{}
	data := `{"key":"PROJ-100","fields":{"summary":"No desc","status":{"name":"Open"},"priority":{"name":"Low"},"issuetype":{"name":"Bug"},"description":null,"created":"2024-01-01","updated":"2024-01-01"}}`

	md, ok := j.RenderMarkdown("jira_get_issue", []byte(data))
	require.True(t, ok)
	assert.Contains(t, string(md), "PROJ-100")
	assert.NotContains(t, string(md), "null")
}

func TestRenderMarkdown_UnknownTool(t *testing.T) {
	j := &jira{}
	_, ok := j.RenderMarkdown("jira_search_issues", []byte(`{}`))
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
		"jira_get_issue",
		"jira_list_comments",
	}
	for _, name := range markdownTools {
		assert.True(t, toolNames[name], "RenderMarkdown handles %q but it's not in Tools()", name)
	}
}
