package forgejo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
	"github.com/stretchr/testify/require"
)

func markdownRenderer(t *testing.T) mcp.MarkdownIntegration {
	t.Helper()
	f, ok := New().(mcp.MarkdownIntegration)
	require.True(t, ok, "Forgejo must implement markdown rendering")
	return f
}

func TestRenderMarkdown_ToolsCovered(t *testing.T) {
	f := markdownRenderer(t)
	fixtures := map[mcp.ToolName]string{
		"forgejo_get_issue":           `{"id":999,"number":12,"title":"Bug","body":"Details"}`,
		"forgejo_list_issue_comments": `[{"id":10,"body":"Hello"}]`,
	}
	covered := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		t.Run(string(tool.Name), func(t *testing.T) {
			input, supported := fixtures[tool.Name]
			if !supported {
				input = fixtures["forgejo_get_issue"]
			}
			md, ok := f.RenderMarkdown(tool.Name, []byte(input))
			require.Equal(t, supported, ok)
			if supported {
				require.Contains(t, dispatch, tool.Name)
				require.NotEmpty(t, md)
				covered[tool.Name] = true
			} else {
				require.Empty(t, md)
			}
		})
	}
	require.Len(t, covered, len(fixtures))
	md, ok := f.RenderMarkdown("forgejo_unknown", []byte(fixtures["forgejo_get_issue"]))
	require.False(t, ok)
	require.Empty(t, md)
}

func TestRenderMarkdown_IssueSDK(t *testing.T) {
	stamp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	issue := sdk.Issue{
		ID: 999, Index: 12, Title: "Fix Unicode café", Body: "## Steps\n\n- Run `go test`\n- Keep **markdown**\n\n```go\nfmt.Println(\"hello\")\n```",
		State: sdk.StateOpen, Poster: &sdk.User{ID: 7, UserName: "alice", AvatarURL: "noise"},
		Labels:    []*sdk.Label{{ID: 8, Name: "bug"}, {ID: 9, Name: "docs"}},
		Assignees: []*sdk.User{{ID: 10, UserName: "bob"}, {ID: 11, UserName: "carol"}},
		Milestone: &sdk.Milestone{ID: 4, Title: "v2"}, Ref: "feature/docs", Comments: 2,
		HTMLURL: "https://forge.test/alice/demo/issues/12", Created: stamp, Updated: stamp,
		Repository: &sdk.RepositoryMeta{FullName: "alice/demo"},
	}
	data, err := json.Marshal(issue)
	require.NoError(t, err)
	original := append([]byte(nil), data...)
	md, ok := markdownRenderer(t).RenderMarkdown("forgejo_get_issue", data)
	require.True(t, ok)
	for _, text := range []string{
		"<!-- forgejo:issue_id=999 number=12 -->", "# #12 Fix Unicode café", "Author: alice", "Status: open",
		"Labels: bug, docs", "Assignees: bob, carol", "Milestone: v2", "Repository: alice/demo", "Ref: feature/docs",
		"Comments: 2", "2026-01-02T03:04:05Z", issue.HTMLURL, issue.Body,
	} {
		require.Contains(t, string(md), text)
	}
	require.NotContains(t, string(md), "avatar_url")
	require.Equal(t, original, data, "rendering must not mutate raw JSON")
}

func TestRenderMarkdown_IssueOptionalFields(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input string
		want  string
	}{
		{"empty body", `{"id":999,"number":12,"title":"Bug","body":""}`, "# #12 Bug"},
		{"deleted user", `{"id":999,"number":12,"title":"Bug","body":"Details","user":null,"labels":null,"assignees":null,"milestone":null}`, "Author: Unknown"},
		{"imported author", `{"id":999,"number":12,"title":"Bug","body":"Details","user":null,"original_author":"imported"}`, "Author: imported"},
		{"null optional entries", `{"id":999,"number":12,"title":"Bug","body":"Details","labels":[null,{"name":"bug"}],"assignees":[null,{"login":"bob"}]}`, "Assignees: bob"},
		{"closed", `{"id":999,"number":12,"title":"Bug","body":"Details","state":"closed","closed_at":"2026-01-02T03:04:05Z"}`, "Closed: 2026-01-02T03:04:05Z"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			md, ok := markdownRenderer(t).RenderMarkdown("forgejo_get_issue", []byte(tt.input))
			require.True(t, ok)
			require.Contains(t, string(md), tt.want)
		})
	}
}

func TestRenderMarkdown_CommentsSDK(t *testing.T) {
	stamp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	comments := []*sdk.Comment{
		{ID: 10, Poster: &sdk.User{ID: 7, UserName: "alice"}, Body: "Hello **world**\n\n```go\nrun()\n```", Created: stamp, Updated: stamp, HTMLURL: "https://forge.test/alice/demo/issues/12#issuecomment-10"},
		{ID: 11, OriginalAuthor: "imported", Body: "Second comment", Created: stamp, Updated: stamp},
	}
	data, err := json.Marshal(comments)
	require.NoError(t, err)
	md, ok := markdownRenderer(t).RenderMarkdown("forgejo_list_issue_comments", data)
	require.True(t, ok)
	for _, text := range []string{"## Comments (2)", "<!-- forgejo:comment_id=10 -->", "<!-- forgejo:comment_id=11 -->", "**alice**", "**imported**", "2026-01-02T03:04:05Z", comments[0].Body, comments[0].HTMLURL, comments[1].Body} {
		require.Contains(t, string(md), text)
	}
}

func TestRenderMarkdown_CommentsIndependentSources(t *testing.T) {
	comments := []map[string]any{
		{"id": 10, "user": map[string]string{"login": "alice"}, "body": "See [first][r].\n\n[r]: https://first.example.test", "html_url": "https://forge.test/issues/12#issuecomment-10"},
		{"id": 11, "user": map[string]string{"login": "bob"}, "body": "See [second][r].\n\n[r]: https://second.example.test"},
	}
	data, err := json.Marshal(comments)
	require.NoError(t, err)
	original := append([]byte(nil), data...)
	md, ok := markdownRenderer(t).RenderMarkdown("forgejo_list_issue_comments", data)
	require.True(t, ok)
	want := "## Comments (2)\n\n" +
		"<!-- forgejo:comment_id=10 -->\n**alice** (Comment 10):\n\n" +
		"```markdown\nSee [first][r].\n\n[r]: https://first.example.test\n```\n\n" +
		"https://forge.test/issues/12#issuecomment-10\n\n" +
		"<!-- forgejo:comment_id=11 -->\n**bob** (Comment 11):\n\n" +
		"```markdown\nSee [second][r].\n\n[r]: https://second.example.test\n```\n\n"
	require.Equal(t, want, string(md))
	require.Equal(t, original, data)
}

func TestRenderMarkdown_CommentsFencedSource(t *testing.T) {
	for _, tt := range []struct {
		name  string
		body  string
		fence string
	}{
		{"prose", "Hello **world**", "```"},
		{"indented code first", "    echo *literal*", "```"},
		{"tab indented code first", "\techo *literal*", "```"},
		{"empty", "", "```"},
		{"trailing newlines", "\n    echo *literal*\n\n", "```"},
		{"CRLF", "    echo *literal*\r\n\r\nDone\r\n", "```"},
		{"inline code", "A `tick` and ``two``", "```"},
		{"embedded fence", "```sh\necho *literal*\n```", "````"},
		{"longer closing fence", "```sh\necho *literal*\n``````", "```````"},
		{"backtick runs reset", "````\n` ` ` ` `\n````", "`````"},
		{"indented embedded fence", "   ````\n# not a new comment\n   ````", "`````"},
		{"tilde fence", "~~~\n[r]: https://example.test\n~~~", "```"},
		{"long adversarial run", "Before " + strings.Repeat("`", 4096) + "\n# not attribution\n[r]: https://evil.example.test", strings.Repeat("`", 4097)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal([]map[string]any{{"id": 10, "body": tt.body}, {"id": 11, "body": "Next"}})
			require.NoError(t, err)
			md, ok := markdownRenderer(t).RenderMarkdown("forgejo_list_issue_comments", data)
			require.True(t, ok)
			body := tt.body
			if body != "" && !strings.HasSuffix(body, "\n") {
				body += "\n"
			}
			want := "## Comments (2)\n\n<!-- forgejo:comment_id=10 -->\n**Unknown** (Comment 10):\n\n" +
				tt.fence + "markdown\n" + body + tt.fence + "\n\n" +
				"<!-- forgejo:comment_id=11 -->\n**Unknown** (Comment 11):\n\n```markdown\nNext\n```\n\n"
			require.Equal(t, want, string(md))
		})
	}
}

func TestRenderMarkdown_IssueBodyRemainsMarkdown(t *testing.T) {
	for _, body := range []string{"See [first][r].\n\n[r]: https://first.example.test", "    echo *literal*", "```sh\necho *literal*\n```"} {
		t.Run(body, func(t *testing.T) {
			data, err := json.Marshal(sdk.Issue{ID: 999, Index: 12, Title: "Bug", Body: body})
			require.NoError(t, err)
			md, ok := markdownRenderer(t).RenderMarkdown("forgejo_get_issue", data)
			require.True(t, ok)
			require.True(t, strings.HasSuffix(string(md), "\n\n"+body+"\n"), "%s", md)
			require.NotContains(t, string(md), "```markdown")
		})
	}
}

func TestRenderMarkdown_CommentsOptionalFields(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input string
		want  string
	}{
		{"empty", `[]`, string(markdown.NoComments)},
		{"empty body", `[{"id":1,"body":"","user":null}]`, "**Unknown**"},
		{"no timestamp", `[{"id":1,"body":"Text","user":{"login":"bob"}}]`, "**bob**"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			md, ok := markdownRenderer(t).RenderMarkdown("forgejo_list_issue_comments", []byte(tt.input))
			require.True(t, ok)
			require.Contains(t, string(md), tt.want)
			if tt.name == "empty" {
				require.Equal(t, markdown.NoComments, md)
			}
		})
	}
}

func TestRenderMarkdown_InvalidShapesFallBack(t *testing.T) {
	f := markdownRenderer(t)
	for _, name := range []mcp.ToolName{"forgejo_get_issue", "forgejo_list_issue_comments"} {
		t.Run(string(name)+"/nil", func(t *testing.T) {
			md, ok := f.RenderMarkdown(name, nil)
			require.False(t, ok)
			require.Empty(t, md)
		})
		for _, input := range []string{"", "{", "null", "{}", `"text"`, "42", "true", `{"message":"Not found"}`, `{"data":[]}`} {
			t.Run(string(name)+"/"+input, func(t *testing.T) {
				md, ok := f.RenderMarkdown(name, []byte(input))
				require.False(t, ok)
				require.Empty(t, md)
			})
		}
	}
	for _, tt := range []struct {
		name  mcp.ToolName
		input string
	}{
		{"forgejo_get_issue", `[]`},
		{"forgejo_get_issue", `{"id":999,"title":"Bug","body":"Text"}`},
		{"forgejo_get_issue", `{"id":999,"number":0,"title":"Bug","body":"Text"}`},
		{"forgejo_get_issue", `{"id":999,"number":12,"title":"","body":"Text"}`},
		{"forgejo_get_issue", `{"id":999,"number":12,"title":"Bug","body":null}`},
		{"forgejo_get_issue", `{"id":999,"number":12,"title":"Bug"}`},
		{"forgejo_get_issue", `{"id":999,"number":12,"title":"Bug","body":{}}`},
		{"forgejo_get_issue", `{"id":999,"number":12,"title":"Bug","body":"Text","labels":"bug"}`},
		{"forgejo_get_issue", `{"id":999,"number":12,"title":"Bug","body":"Text","user":"alice"}`},
		{"forgejo_get_issue", `{"id":999,"number":"12","title":"Bug","body":"Text"}`},
		{"forgejo_list_issue_comments", `[null]`},
		{"forgejo_list_issue_comments", `[{}]`},
		{"forgejo_list_issue_comments", `[{"id":1,"body":"First"},null]`},
		{"forgejo_list_issue_comments", `[{"id":1,"body":"First"},{"message":"Not found"}]`},
		{"forgejo_list_issue_comments", `[{"id":1,"body":null}]`},
		{"forgejo_list_issue_comments", `[{"id":1}]`},
		{"forgejo_list_issue_comments", `[{"id":0,"body":"Text"}]`},
		{"forgejo_list_issue_comments", `[{"id":1,"body":{}}]`},
		{"forgejo_list_issue_comments", `[{"id":1,"body":"Text","user":[]}]`},
		{"forgejo_list_issue_comments", `[{"id":1,"body":"Text"}] {}`},
	} {
		t.Run(string(tt.name)+"/"+tt.input, func(t *testing.T) {
			md, ok := f.RenderMarkdown(tt.name, []byte(tt.input))
			require.False(t, ok)
			require.Empty(t, md)
		})
	}
}

func TestRenderMarkdown_ExactOutput(t *testing.T) {
	for _, tt := range []struct {
		name  mcp.ToolName
		input string
		want  mcp.Markdown
	}{
		{
			"forgejo_get_issue",
			`{"id":999,"number":12,"title":"Bug","body":"Read **this**","state":"open","user":{"login":"alice"}}`,
			"<!-- forgejo:issue_id=999 number=12 -->\n# #12 Bug\n*Author: alice | Status: open*\n\nComments: 0\n\nRead **this**\n",
		},
		{
			"forgejo_list_issue_comments",
			`[{"id":10,"body":"Read **this**","user":{"login":"alice"},"created_at":"2026-01-02T03:04:05Z","updated_at":"2026-01-03T03:04:05Z"}]`,
			"## Comments (1)\n\n<!-- forgejo:comment_id=10 -->\n**alice** (Comment 10 | 2026-01-02T03:04:05Z | Updated: 2026-01-03T03:04:05Z):\n\n```markdown\nRead **this**\n```\n\n",
		},
		{"forgejo_list_issue_comments", `[]`, markdown.NoComments},
	} {
		t.Run(string(tt.name)+"/"+tt.input, func(t *testing.T) {
			md, ok := markdownRenderer(t).RenderMarkdown(tt.name, []byte(tt.input))
			require.True(t, ok)
			require.Equal(t, tt.want, md)
		})
	}
}

func TestRenderMarkdown_HandlerJSONPreserved(t *testing.T) {
	for _, tt := range []struct {
		name     mcp.ToolName
		response string
	}{
		{"forgejo_get_issue", `{"id":999,"number":12,"title":"Bug","body":"Body","user":{"id":7,"login":"alice","avatar_url":"noise"}}`},
		{"forgejo_list_issue_comments", `[{"id":10,"body":"Hello","user":{"id":7,"login":"alice","avatar_url":"noise"}}]`},
	} {
		t.Run(string(tt.name), func(t *testing.T) {
			f, _ := configured(t, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, tt.response) })
			result, err := f.Execute(context.Background(), tt.name, map[string]any{"owner": "alice", "repo": "demo", "number": 12})
			require.NoError(t, err)
			require.False(t, result.IsError)
			require.True(t, json.Valid([]byte(result.Data)))
			require.Contains(t, result.Data, `"avatar_url":"noise"`)
			original := result.Data
			renderer, ok := f.(mcp.MarkdownIntegration)
			require.True(t, ok)
			md, rendered := renderer.RenderMarkdown(tt.name, []byte(result.Data))
			require.True(t, rendered)
			require.NotEmpty(t, md)
			require.Equal(t, original, result.Data)
		})
	}
}
