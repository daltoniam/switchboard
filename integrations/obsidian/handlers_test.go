package obsidian

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capturedRequest struct {
	Method      string
	Path        string
	Query       string
	Accept      string
	ContentType string
	Headers     http.Header
	Body        string
}

func captureServer(t *testing.T, status int, contentType, body string) (*obsidian, *capturedRequest) {
	t.Helper()
	got := &capturedRequest{}
	o := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		*got = capturedRequest{
			Method:      r.Method,
			Path:        r.URL.EscapedPath(),
			Query:       r.URL.RawQuery,
			Accept:      r.Header.Get("Accept"),
			ContentType: r.Header.Get("Content-Type"),
			Headers:     r.Header.Clone(),
			Body:        string(data),
		}
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	})
	return o, got
}

const noteFixture = `{"path":"Projects/Plan A.md","content":"---\nstatus: draft\n---\n# Plan\n\nShip it.\n","frontmatter":{"status":"draft"},"tags":["work","work/plans"],"stat":{"ctime":1700000000000,"mtime":1700000500000,"size":42},"links":["Ideas.md"],"backlinks":[],"unresolvedLinks":[]}`

func TestRequests(t *testing.T) {
	tests := []struct {
		name        string
		tool        string
		args        map[string]any
		status      int
		respType    string
		resp        string
		method      string
		path        string
		query       string
		accept      string
		contentType string
		headers     map[string]string
		body        string
		bodyJSON    map[string]any
		want        map[string]any
	}{
		{
			name: "list root", tool: "obsidian_list_files", status: 200, resp: `{"files":["Daily/","Inbox.md"]}`,
			method: "GET", path: "/vault/", accept: "application/json",
			want: map[string]any{"path": "", "files": []any{"Daily/", "Inbox.md"}, "count": float64(2)},
		},
		{
			name: "list folder prefixes paths", tool: "obsidian_list_files", args: map[string]any{"path": "/Daily Notes/2026"}, status: 200, resp: `{"files":["01.md","Archive/"]}`,
			method: "GET", path: "/vault/Daily%20Notes/2026/",
			want: map[string]any{"path": "Daily Notes/2026/", "files": []any{"Daily Notes/2026/01.md", "Daily Notes/2026/Archive/"}, "count": float64(2)},
		},
		{
			name: "read full note", tool: "obsidian_read_note", args: map[string]any{"path": "Projects/Plan A.md"}, status: 200, respType: "application/vnd.olrapi.note+json", resp: noteFixture,
			method: "GET", path: "/vault/Projects/Plan%20A.md", accept: "application/vnd.olrapi.note+json",
			want: map[string]any{"path": "Projects/Plan A.md", "tags": []any{"work", "work/plans"}},
		},
		{
			name: "read heading section", tool: "obsidian_read_note", args: map[string]any{"path": "Projects/Plan A.md", "heading": []any{"Plan", "TODO/DONE"}}, status: 200, respType: "text/markdown", resp: "Ship it.\n",
			method: "GET", path: "/vault/Projects/Plan%20A.md/heading/Plan/TODO%2FDONE", accept: "text/markdown",
			want: map[string]any{"path": "Projects/Plan A.md", "target": "heading: Plan > TODO/DONE", "content": "Ship it.\n"},
		},
		{
			name: "read block", tool: "obsidian_read_note", args: map[string]any{"path": "a.md", "block": "^abc123"}, status: 200, resp: "A block.",
			method: "GET", path: "/vault/a.md/block/abc123", accept: "text/markdown",
			want: map[string]any{"path": "a.md", "target": "block: abc123", "content": "A block."},
		},
		{
			name: "read frontmatter field", tool: "obsidian_read_note", args: map[string]any{"path": "a.md", "frontmatter_field": "status"}, status: 200, respType: "application/json", resp: `"draft"`,
			method: "GET", path: "/vault/a.md/frontmatter/status", accept: "text/markdown",
			want: map[string]any{"path": "a.md", "target": "frontmatter: status", "content": `"draft"`},
		},
		{
			name: "document map", tool: "obsidian_get_document_map", args: map[string]any{"path": "a.md"}, status: 200, resp: `{"version":"v1","headings":{"Plan":{"TODO":{}}},"blocks":["abc"],"frontmatterFields":["status"]}`,
			method: "GET", path: "/vault/a.md", accept: "application/vnd.olrapi.document-map+json",
			want: map[string]any{"path": "a.md", "version": "v1", "headings": map[string]any{"Plan": map[string]any{"TODO": map[string]any{}}}, "blocks": []any{"abc"}, "frontmatterFields": []any{"status"}},
		},
		{
			name: "simple search limits results", tool: "obsidian_search", args: map[string]any{"query": "budget plan", "limit": 1, "context_length": 30}, status: 200,
			resp:   `[{"filename":"a.md","score":-0.1,"matches":[{"match":{"start":1,"end":5,"source":"content"},"context":"the budget"}]},{"filename":"b.md","score":-0.2,"matches":[]}]`,
			method: "POST", path: "/search/simple/", query: "contextLength=30&query=budget+plan",
			want: map[string]any{"query": "budget plan", "total": float64(2), "returned": float64(1), "results": []any{map[string]any{"filename": "a.md", "score": -0.1, "matches": []any{map[string]any{"match": map[string]any{"start": float64(1), "end": float64(5), "source": "content"}, "context": "the budget"}}}}},
		},
		{
			name: "jsonlogic query object", tool: "obsidian_query", args: map[string]any{"jsonlogic": map[string]any{"in": []any{"work", map[string]any{"var": "tags"}}}}, status: 200, resp: `[{"filename":"a.md","result":true}]`,
			method: "POST", path: "/search/", contentType: "application/vnd.olrapi.jsonlogic+json", bodyJSON: map[string]any{"in": []any{"work", map[string]any{"var": "tags"}}},
			want: map[string]any{"total": float64(1), "returned": float64(1)},
		},
		{
			name: "jsonlogic query string", tool: "obsidian_query", args: map[string]any{"jsonlogic": `{"glob":["Daily/*.md",{"var":"path"}]}`}, status: 200, resp: `[]`,
			method: "POST", path: "/search/", contentType: "application/vnd.olrapi.jsonlogic+json", bodyJSON: map[string]any{"glob": []any{"Daily/*.md", map[string]any{"var": "path"}}},
			want: map[string]any{"total": float64(0), "returned": float64(0), "results": []any{}},
		},
		{
			name: "list tags", tool: "obsidian_list_tags", status: 200, resp: `{"tags":[{"name":"work","count":3}]}`,
			method: "GET", path: "/tags/",
			want: map[string]any{"tags": []any{map[string]any{"name": "work", "count": float64(3)}}},
		},
		{
			name: "list commands filtered", tool: "obsidian_list_commands", args: map[string]any{"query": "GRAPH"}, status: 200, resp: `{"commands":[{"id":"graph:open","name":"Graph view: Open graph view"},{"id":"editor:save-file","name":"Save current file"}]}`,
			method: "GET", path: "/commands/",
			want: map[string]any{"total": float64(1), "commands": []any{map[string]any{"id": "graph:open", "name": "Graph view: Open graph view"}}},
		},
		{
			name: "active note", tool: "obsidian_get_active_note", status: 200, resp: noteFixture,
			method: "GET", path: "/active/", accept: "application/vnd.olrapi.note+json",
			want: map[string]any{"path": "Projects/Plan A.md"},
		},
		{
			name: "write note", tool: "obsidian_write_note", args: map[string]any{"path": "Inbox/New.md", "content": "# New\n"}, status: 204,
			method: "PUT", path: "/vault/Inbox/New.md", contentType: "text/markdown", body: "# New\n",
			want: map[string]any{"path": "Inbox/New.md", "status": "written"},
		},
		{
			name: "append note", tool: "obsidian_append_note", args: map[string]any{"path": "Inbox/New.md", "content": "- item\n"}, status: 204,
			method: "POST", path: "/vault/Inbox/New.md", contentType: "text/markdown", body: "- item\n",
			want: map[string]any{"path": "Inbox/New.md", "status": "appended"},
		},
		{
			name: "patch heading", tool: "obsidian_patch_note", args: map[string]any{"path": "a.md", "operation": "append", "target_type": "heading", "target": []any{"Plan", "TODO"}, "content": "- next", "create_target_if_missing": true}, status: 200,
			method: "PATCH", path: "/vault/a.md", contentType: "application/json", headers: map[string]string{"Markdown-Patch-Version": "2"},
			bodyJSON: map[string]any{"operation": "append", "targetType": "heading", "target": []any{"Plan", "TODO"}, "content": "- next", "createTargetIfMissing": true},
			want:     map[string]any{"path": "a.md", "status": "patched"},
		},
		{
			name: "patch frontmatter value", tool: "obsidian_patch_note", args: map[string]any{"path": "a.md", "operation": "replace", "target_type": "frontmatter", "target": "status", "value": "done", "if_match": "v9", "scope": "content"}, status: 200,
			method: "PATCH", path: "/vault/a.md",
			bodyJSON: map[string]any{"operation": "replace", "targetType": "frontmatter", "target": "status", "value": "done", "ifMatch": "v9", "scope": "content"},
			want:     map[string]any{"path": "a.md", "status": "patched"},
		},
		{
			name: "patch delete block", tool: "obsidian_patch_note", args: map[string]any{"path": "a.md", "operation": "delete", "target_type": "block", "target": "^abc"}, status: 200,
			method: "PATCH", path: "/vault/a.md",
			bodyJSON: map[string]any{"operation": "delete", "targetType": "block", "target": "abc"},
			want:     map[string]any{"path": "a.md", "status": "patched"},
		},
		{
			name: "delete note", tool: "obsidian_delete_note", args: map[string]any{"path": "Inbox/Old.md"}, status: 204,
			method: "DELETE", path: "/vault/Inbox/Old.md",
			want: map[string]any{"path": "Inbox/Old.md", "status": "deleted"},
		},
		{
			name: "execute command", tool: "obsidian_execute_command", args: map[string]any{"command_id": "graph:open"}, status: 204,
			method: "POST", path: "/commands/graph:open/",
			want: map[string]any{"command_id": "graph:open", "status": "executed"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, got := captureServer(t, tt.status, tt.respType, tt.resp)
			out := decodeResult(t, execute(t, o, tt.tool, tt.args))
			assert.Equal(t, tt.method, got.Method)
			assert.Equal(t, tt.path, got.Path)
			assert.Equal(t, "Bearer "+testKey, got.Headers.Get("Authorization"))
			if tt.query != "" {
				assert.Equal(t, tt.query, got.Query)
			}
			if tt.accept != "" {
				assert.Equal(t, tt.accept, got.Accept)
			}
			if tt.contentType != "" {
				assert.Equal(t, tt.contentType, got.ContentType)
			}
			for k, v := range tt.headers {
				assert.Equal(t, v, got.Headers.Get(k), k)
			}
			if tt.body != "" {
				assert.Equal(t, tt.body, got.Body)
			}
			if tt.bodyJSON != nil {
				var body map[string]any
				require.NoError(t, json.Unmarshal([]byte(got.Body), &body), got.Body)
				assert.Equal(t, tt.bodyJSON, body)
			}
			for k, v := range tt.want {
				assert.Equal(t, v, out[k], k)
			}
		})
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name string
		tool string
		args map[string]any
		want string
	}{
		{"read needs path", "obsidian_read_note", nil, "path is required"},
		{"read rejects traversal", "obsidian_read_note", map[string]any{"path": "../secrets.md"}, "'..'"},
		{"read rejects two targets", "obsidian_read_note", map[string]any{"path": "a.md", "block": "x", "heading": []any{"A"}}, "only one of"},
		{"read rejects folder", "obsidian_read_note", map[string]any{"path": "Daily/"}, "folder"},
		{"map needs path", "obsidian_get_document_map", nil, "path is required"},
		{"search needs query", "obsidian_search", nil, "query is required"},
		{"query needs jsonlogic", "obsidian_query", nil, "jsonlogic is required"},
		{"query rejects non-object", "obsidian_query", map[string]any{"jsonlogic": "[1]"}, "jsonlogic"},
		{"query rejects bad json", "obsidian_query", map[string]any{"jsonlogic": "{nope"}, "jsonlogic"},
		{"write needs content key", "obsidian_write_note", map[string]any{"path": "a.md"}, "content is required"},
		{"append needs content", "obsidian_append_note", map[string]any{"path": "a.md", "content": ""}, "content is required"},
		{"patch bad operation", "obsidian_patch_note", map[string]any{"path": "a.md", "operation": "upsert", "target_type": "heading", "target": "A", "content": "x"}, "operation"},
		{"patch bad target type", "obsidian_patch_note", map[string]any{"path": "a.md", "operation": "append", "target_type": "line", "target": "A", "content": "x"}, "target_type"},
		{"patch needs payload", "obsidian_patch_note", map[string]any{"path": "a.md", "operation": "append", "target_type": "heading", "target": "A"}, "content or value"},
		{"patch rejects both payloads", "obsidian_patch_note", map[string]any{"path": "a.md", "operation": "append", "target_type": "frontmatter", "target": "k", "content": "x", "value": 1}, "content or value"},
		{"patch block needs string target", "obsidian_patch_note", map[string]any{"path": "a.md", "operation": "delete", "target_type": "block", "target": []any{"a"}}, "target"},
		{"delete needs path", "obsidian_delete_note", nil, "path is required"},
		{"command needs id", "obsidian_execute_command", nil, "command_id is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			o := newTestServer(t, func(w http.ResponseWriter, r *http.Request) { called = true })
			result := execute(t, o, tt.tool, tt.args)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tt.want)
			assert.False(t, called, "invalid input must not reach the vault")
		})
	}
}

func TestPatch_HeadingStringTarget(t *testing.T) {
	o, got := captureServer(t, 200, "", "")
	decodeResult(t, execute(t, o, "obsidian_patch_note", map[string]any{"path": "a.md", "operation": "prepend", "target_type": "heading", "target": "Plan", "content": "x"}))
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.Body), &body))
	assert.Equal(t, []any{"Plan"}, body["target"])
}

func TestActiveNote_NoneOpen(t *testing.T) {
	o, _ := captureServer(t, 404, "application/json", `{"message":"Not Found","errorCode":40400}`)
	result := execute(t, o, "obsidian_get_active_note", nil)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "no note is open")
}

func TestReadLimited(t *testing.T) {
	big := make([]byte, maxResponseSize+1)
	o, _ := captureServer(t, 200, "text/markdown", string(big))
	result := execute(t, o, "obsidian_read_note", map[string]any{"path": "a.md", "block": "x"})
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "exceeds")
}
