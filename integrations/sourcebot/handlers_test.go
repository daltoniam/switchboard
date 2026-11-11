package sourcebot

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const searchFixture = `{
  "stats": {"actualMatchCount": 3, "totalMatchCount": 7, "fileCount": 2, "duration": 448511, "contentBytesLoaded": 333, "flushReason": "FLUSH_REASON_UNKNOWN_UNSPECIFIED"},
  "files": [
    {
      "fileName": {"text": "internal/billing/invoice.go", "matchRanges": []},
      "repository": "github.com/acme/api", "repositoryId": 1, "language": "Go",
      "webUrl": "https://sourcebot.example.com/browse/github.com/acme/api@refs/heads/main/-/blob/internal%2Fbilling%2Finvoice.go",
      "externalWebUrl": "https://github.com/acme/api/blob/main/internal/billing/invoice.go",
      "chunks": [{
        "content": "\n// ComputeInvoiceTotal sums line items.\nfunc ComputeInvoiceTotal(items []int) int {\n",
        "matchRanges": [
          {"start": {"byteOffset": 62, "column": 6, "lineNumber": 4}, "end": {"byteOffset": 81, "column": 25, "lineNumber": 4}},
          {"start": {"byteOffset": 20, "column": 4, "lineNumber": 3}, "end": {"byteOffset": 39, "column": 23, "lineNumber": 3}}
        ],
        "contentStart": {"byteOffset": 16, "column": 1, "lineNumber": 2},
        "symbols": []
      }],
      "branches": ["refs/heads/main"], "content": ""
    },
    {
      "fileName": {"text": "cmd/server/main.go", "matchRanges": []},
      "repository": "github.com/acme/api", "repositoryId": 1, "language": "Go",
      "webUrl": "https://sourcebot.example.com/browse/github.com/acme/api@refs/heads/main/-/blob/cmd%2Fserver%2Fmain.go",
      "chunks": [{
        "content": "\t_ = billing.ComputeInvoiceTotal([]int{1, 2, 3})\n",
        "matchRanges": [{"start": {"byteOffset": 88, "column": 14, "lineNumber": 6}, "end": {"byteOffset": 107, "column": 33, "lineNumber": 6}}],
        "contentStart": {"byteOffset": 61, "column": 1, "lineNumber": 6}
      }],
      "branches": ["refs/heads/main"]
    }
  ],
  "repositoryInfo": [{"id": 1, "codeHostType": "github", "name": "github.com/acme/api"}],
  "isSearchExhaustive": false
}`

func TestBuildSearchQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		filters searchFilters
		want    string
	}{
		{"literal", "ComputeInvoiceTotal", searchFilters{}, `"ComputeInvoiceTotal"`},
		{"literal escapes quotes and backslashes", `say "hi" \n`, searchFilters{}, `"say \"hi\" \\n"`},
		{"regex passes through", `func \w+\(`, searchFilters{regex: true}, `func \w+\(`},
		{"repos are regex escaped", "x", searchFilters{repos: []string{"github.com/acme/api", "github.com/acme/web+ui"}}, `"x" (repo:github\.com/acme/api or repo:github\.com/acme/web\+ui)`},
		{"languages", "x", searchFilters{languages: []string{"Go", "TypeScript"}}, `"x" (lang:Go or lang:TypeScript)`},
		{"file paths are regex escaped", "x", searchFilters{filePaths: []string{"internal/billing/", "main.go"}}, `"x" (file:internal/billing/ or file:main\.go)`},
		{"ref", "x", searchFilters{ref: "release-1.2"}, `"x" ( rev:release-1.2 )`},
		{"blank filter values ignored", "x", searchFilters{repos: []string{" ", ""}, languages: []string{""}, ref: " "}, `"x"`},
		{"all filters in order", "x", searchFilters{repos: []string{"a"}, languages: []string{"Go"}, filePaths: []string{"b"}, ref: "main"}, `"x" (repo:a) (lang:Go) (file:b) ( rev:main )`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, buildSearchQuery(tt.query, tt.filters))
		})
	}
}

func TestSearchCode_Request(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want map[string]any
	}{
		{"defaults", map[string]any{"query": "invoice"}, map[string]any{
			"query": `"invoice"`, "matches": float64(50), "contextLines": float64(3), "isRegexEnabled": false, "isCaseSensitivityEnabled": false,
		}},
		{"options", map[string]any{"query": "Invoice.*", "regex": true, "case_sensitive": true, "max_matches": 5, "context_lines": 0, "repos": []any{"github.com/acme/api"}, "languages": "Go", "ref": "main"}, map[string]any{
			"query": `Invoice.* (repo:github\.com/acme/api) (lang:Go) ( rev:main )`, "matches": float64(5), "contextLines": float64(0), "isRegexEnabled": true, "isCaseSensitivityEnabled": true,
		}},
		{"limits clamp", map[string]any{"query": "x", "max_matches": 5000, "context_lines": 99}, map[string]any{
			"query": `"x"`, "matches": float64(1000), "contextLines": float64(20), "isRegexEnabled": false, "isCaseSensitivityEnabled": false,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got map[string]any
			s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/api/search", r.URL.Path)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				got = readBody(t, r)
				_, _ = io.WriteString(w, `{"stats":{},"files":[],"repositoryInfo":[],"isSearchExhaustive":true}`)
			})
			out := decodeResult(t, execute(t, s, "sourcebot_search_code", tt.args))
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.want["query"], out["query"])
			assert.Equal(t, []any{}, out["files"])
		})
	}
}

func TestSearchCode_Validation(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{"missing query", map[string]any{}, "query is required"},
		{"zero matches", map[string]any{"query": "x", "max_matches": 0}, "max_matches must be at least 1"},
		{"negative context", map[string]any{"query": "x", "context_lines": -1}, "context_lines must not be negative"},
		{"bad repos type", map[string]any{"query": "x", "repos": 7}, "repos"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				t.Errorf("unexpected request %s", r.URL.Path)
			})
			result := execute(t, s, "sourcebot_search_code", tt.args)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tt.want)
		})
	}
}

func TestSearchCode_Response(t *testing.T) {
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, searchFixture)
	})
	res := execute(t, s, "sourcebot_search_code", map[string]any{"query": "ComputeInvoiceTotal"})
	out := decodeResult(t, res)
	assert.NotContains(t, res.Data, "byteOffset")
	assert.NotContains(t, res.Data, "contentBytesLoaded")
	assert.Equal(t, float64(3), out["matchCount"])
	assert.Equal(t, float64(7), out["totalMatchCount"])
	assert.Equal(t, float64(2), out["fileCount"])
	assert.Equal(t, false, out["isSearchExhaustive"])
	assert.NotContains(t, out, "stats")
	files := out["files"].([]any)
	require.Len(t, files, 2)
	first := files[0].(map[string]any)
	assert.Equal(t, "internal/billing/invoice.go", first["fileName"])
	assert.Equal(t, "github.com/acme/api", first["repository"])
	assert.Equal(t, "Go", first["language"])
	assert.Equal(t, "https://github.com/acme/api/blob/main/internal/billing/invoice.go", first["externalWebUrl"])
	assert.Contains(t, first["webUrl"], "sourcebot.example.com")
	assert.Equal(t, []any{"refs/heads/main"}, first["branches"])
	assert.Equal(t, float64(2), first["matchCount"])
	chunk := first["chunks"].([]any)[0].(map[string]any)
	assert.Equal(t, float64(2), chunk["startLine"])
	assert.Equal(t, []any{float64(3), float64(4)}, chunk["matchLines"])
	assert.Contains(t, chunk["content"], "func ComputeInvoiceTotal")
	second := files[1].(map[string]any)
	assert.NotContains(t, second, "externalWebUrl")
}

func TestSearchCode_WithoutSnippets(t *testing.T) {
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, searchFixture)
	})
	out := decodeResult(t, execute(t, s, "sourcebot_search_code", map[string]any{"query": "ComputeInvoiceTotal", "include_snippets": false}))
	first := out["files"].([]any)[0].(map[string]any)
	assert.NotContains(t, first, "chunks")
	assert.Equal(t, float64(2), first["matchCount"])
}

func TestListRepos(t *testing.T) {
	var query string
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/repos", r.URL.Path)
		query = r.URL.RawQuery
		w.Header().Set("X-Total-Count", "42")
		_, _ = io.WriteString(w, `[{"codeHostType":"github","repoId":1,"repoName":"github.com/acme/api","webUrl":"https://sourcebot.example.com/browse/github.com/acme/api","externalWebUrl":"https://github.com/acme/api","defaultBranch":"main","isFork":false,"isArchived":false}]`)
	})
	out := decodeResult(t, execute(t, s, "sourcebot_list_repos", map[string]any{"query": "acme", "page": 2, "per_page": 500, "sort": "pushed", "direction": "desc"}))
	assert.Equal(t, "direction=desc&page=2&perPage=100&query=acme&sort=pushed", query)
	assert.Equal(t, float64(42), out["totalCount"])
	assert.Equal(t, float64(2), out["page"])
	assert.Equal(t, float64(100), out["perPage"])
	assert.Equal(t, false, out["hasMore"])
	repos := out["repos"].([]any)
	require.Len(t, repos, 1)
	assert.Equal(t, "github.com/acme/api", repos[0].(map[string]any)["repoName"])
}

func TestListRepos_DefaultsAndHasMore(t *testing.T) {
	var query string
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Header().Set("X-Total-Count", "31")
		_, _ = io.WriteString(w, `[]`)
	})
	out := decodeResult(t, execute(t, s, "sourcebot_list_repos", nil))
	assert.Equal(t, "page=1&perPage=30", query)
	assert.Equal(t, true, out["hasMore"])
	assert.Equal(t, []any{}, out["repos"])
}

func TestListRepos_Validation(t *testing.T) {
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request")
	})
	for args, want := range map[string]string{"sort": "sort must be", "direction": "direction must be"} {
		result := execute(t, s, "sourcebot_list_repos", map[string]any{args: "sideways"})
		assert.True(t, result.IsError)
		assert.Contains(t, result.Data, want)
	}
}

const sourceFixture = `{"source":"package billing\n\nfunc A() {}\n\nfunc B() {}\n","language":"Go","path":"internal/billing/invoice.go","repo":"github.com/acme/api","repoCodeHostType":"github","webUrl":"https://sourcebot.example.com/browse/github.com/acme/api/-/blob/internal/billing/invoice.go","externalWebUrl":"https://github.com/acme/api/blob/main/internal/billing/invoice.go"}`

func TestReadFile(t *testing.T) {
	tests := []struct {
		name       string
		args       map[string]any
		wantSource string
		wantStart  float64
		wantEnd    float64
	}{
		{"whole file", map[string]any{}, "package billing\n\nfunc A() {}\n\nfunc B() {}\n", 1, 5},
		{"range", map[string]any{"start_line": 3, "end_line": 3}, "func A() {}\n", 3, 3},
		{"open end", map[string]any{"start_line": 5}, "func B() {}\n", 5, 5},
		{"end clamps", map[string]any{"start_line": 4, "end_line": 400}, "\nfunc B() {}\n", 4, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var query string
			s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/source", r.URL.Path)
				query = r.URL.RawQuery
				_, _ = io.WriteString(w, sourceFixture)
			})
			args := map[string]any{"repo": "github.com/acme/api", "path": "/internal/billing/invoice.go", "ref": "main"}
			for k, v := range tt.args {
				args[k] = v
			}
			out := decodeResult(t, execute(t, s, "sourcebot_read_file", args))
			assert.Equal(t, "path=internal%2Fbilling%2Finvoice.go&ref=main&repo=github.com%2Facme%2Fapi", query)
			assert.Equal(t, tt.wantSource, out["source"])
			assert.Equal(t, float64(5), out["totalLines"])
			assert.Equal(t, tt.wantStart, out["startLine"])
			assert.Equal(t, tt.wantEnd, out["endLine"])
			assert.Equal(t, "Go", out["language"])
			assert.Equal(t, "https://github.com/acme/api/blob/main/internal/billing/invoice.go", out["externalWebUrl"])
		})
	}
}

func TestReadFile_Errors(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{"missing repo", map[string]any{"path": "a"}, "repo is required"},
		{"missing path", map[string]any{"repo": "a"}, "path is required"},
		{"negative line", map[string]any{"repo": "a", "path": "b", "start_line": -1}, "must be positive"},
		{"start past end", map[string]any{"repo": "a", "path": "b", "start_line": 9}, "past the end of the file (5 lines)"},
		{"end before start", map[string]any{"repo": "a", "path": "b", "start_line": 4, "end_line": 2}, "before start_line"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, sourceFixture)
			})
			result := execute(t, s, "sourcebot_read_file", tt.args)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tt.want)
		})
	}
}

func TestSliceLines(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		start     int
		end       int
		want      string
		wantTotal int
	}{
		{"empty file", "", 0, 0, "", 0},
		{"no trailing newline", "a\nb", 0, 0, "a\nb", 2},
		{"single line", "a\n", 1, 1, "a\n", 1},
		{"middle", "a\nb\nc\n", 2, 2, "b\n", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, total, _, _, err := sliceLines(tt.src, tt.start, tt.end)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantTotal, total)
		})
	}
}

func treeFixture(paths ...string) treeNode {
	root := treeNode{Type: "tree", Name: "root"}
	byPath := map[string]*treeNode{"": &root}
	for _, p := range paths {
		kind := "blob"
		if strings.HasSuffix(p, "/") {
			kind = "tree"
			p = strings.TrimSuffix(p, "/")
		}
		parent := ""
		name := p
		if i := strings.LastIndex(p, "/"); i >= 0 {
			parent, name = p[:i], p[i+1:]
		}
		node := byPath[parent]
		node.Children = append(node.Children, treeNode{Type: kind, Path: p, Name: name})
		if kind == "tree" {
			byPath[p] = &node.Children[len(node.Children)-1]
		}
	}
	return root
}

// fakeTree answers like /api/tree: it returns every requested directory with
// its direct children plus the chain of ancestors leading to it.
func fakeTree(full treeNode, calls *[][]string) treeFetcher {
	return func(_ context.Context, paths []string) (treeNode, error) {
		*calls = append(*calls, paths)
		want := map[string]bool{"": true}
		for _, p := range paths {
			want[p] = true
			for i := range p {
				if p[i] == '/' {
					want[p[:i]] = true
				}
			}
		}
		var prune func(node treeNode, p string, expand bool) treeNode
		prune = func(node treeNode, p string, expand bool) treeNode {
			out := treeNode{Type: node.Type, Path: node.Path, Name: node.Name}
			if !expand {
				return out
			}
			for _, child := range node.Children {
				cp := joinTreePath(p, child.Name)
				out.Children = append(out.Children, prune(child, cp, want[cp] && child.Type == "tree"))
			}
			return out
		}
		return prune(full, "", true), nil
	}
}

func TestWalkTree(t *testing.T) {
	full := treeFixture("cmd/", "cmd/server/", "cmd/server/main.go", "internal/", "internal/billing/", "internal/billing/invoice.go", "README.md", "go.mod")
	all := treeOptions{depth: 1, maxEntries: 1000, includeFiles: true, includeDirectories: true}
	tests := []struct {
		name          string
		root          string
		opts          func(treeOptions) treeOptions
		wantPaths     []string
		wantCalls     int
		wantTruncated bool
	}{
		{"root depth 1", "", func(o treeOptions) treeOptions { return o }, []string{"cmd", "internal", "go.mod", "README.md"}, 1, false},
		{"root depth 3", "", func(o treeOptions) treeOptions { o.depth = 3; return o }, []string{"cmd", "internal", "go.mod", "README.md", "cmd/server", "cmd/server/main.go", "internal/billing", "internal/billing/invoice.go"}, 3, false},
		{"subdirectory", "internal", func(o treeOptions) treeOptions { o.depth = 2; return o }, []string{"internal/billing", "internal/billing/invoice.go"}, 2, false},
		{"files only", "", func(o treeOptions) treeOptions { o.depth = 3; o.includeDirectories = false; return o }, []string{"go.mod", "README.md", "cmd/server/main.go", "internal/billing/invoice.go"}, 3, false},
		{"directories only", "", func(o treeOptions) treeOptions { o.depth = 2; o.includeFiles = false; return o }, []string{"cmd", "internal", "cmd/server", "internal/billing"}, 2, false},
		{"neither", "", func(o treeOptions) treeOptions { o.includeFiles = false; o.includeDirectories = false; return o }, []string{}, 0, false},
		{"truncated", "", func(o treeOptions) treeOptions { o.depth = 3; o.maxEntries = 3; return o }, []string{"cmd", "internal", "README.md"}, 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls [][]string
			entries, truncated, err := walkTree(context.Background(), fakeTree(full, &calls), tt.root, tt.opts(all))
			require.NoError(t, err)
			paths := make([]string, len(entries))
			for i, e := range entries {
				paths[i] = e.Path
			}
			assert.Equal(t, tt.wantPaths, paths)
			assert.Len(t, calls, tt.wantCalls)
			assert.Equal(t, tt.wantTruncated, truncated)
		})
	}
}

func TestWalkTree_BatchesEachLevel(t *testing.T) {
	full := treeFixture("a/", "a/x/", "a/x/f", "b/", "b/y/", "b/y/g")
	var calls [][]string
	entries, _, err := walkTree(context.Background(), fakeTree(full, &calls), "", treeOptions{depth: 3, maxEntries: 100, includeFiles: true, includeDirectories: true})
	require.NoError(t, err)
	assert.Equal(t, [][]string{{}, {"a", "b"}, {"a/x", "b/y"}}, calls)
	last := entries[len(entries)-1]
	assert.Equal(t, treeEntry{Type: "blob", Path: "b/y/g", Name: "g", ParentPath: "b/y", Depth: 3}, last)
}

func TestWalkTree_BadRoot(t *testing.T) {
	full := treeFixture("cmd/", "README.md")
	for root, want := range map[string]string{"nope": `directory "nope" not found`, "README.md": "is a file"} {
		var calls [][]string
		_, _, err := walkTree(context.Background(), fakeTree(full, &calls), root, treeOptions{depth: 1, maxEntries: 10, includeFiles: true, includeDirectories: true})
		require.Error(t, err)
		assert.Contains(t, err.Error(), want)
	}
}

func TestListTree(t *testing.T) {
	var bodies []map[string]any
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/tree", r.URL.Path)
		bodies = append(bodies, readBody(t, r))
		if len(bodies) == 1 {
			_, _ = io.WriteString(w, `{"tree":{"name":"root","path":"","type":"tree","children":[{"name":"internal","path":"internal","type":"tree","children":[{"name":"billing","path":"internal/billing","type":"tree","children":[]},{"name":"doc.go","path":"internal/doc.go","type":"blob","children":[]}]}]}}`)
			return
		}
		_, _ = io.WriteString(w, `{"tree":{"name":"root","path":"","type":"tree","children":[{"name":"internal","path":"internal","type":"tree","children":[{"name":"billing","path":"internal/billing","type":"tree","children":[{"name":"invoice.go","path":"internal/billing/invoice.go","type":"blob","children":[]}]}]}]}}`)
	})
	out := decodeResult(t, execute(t, s, "sourcebot_list_tree", map[string]any{"repo": "github.com/acme/api", "path": "/internal/", "depth": 2}))
	require.Len(t, bodies, 2)
	assert.Equal(t, map[string]any{"repoName": "github.com/acme/api", "revisionName": "HEAD", "paths": []any{"internal"}}, bodies[0])
	assert.Equal(t, []any{"internal/billing"}, bodies[1]["paths"])
	assert.Equal(t, "internal", out["path"])
	assert.Equal(t, "HEAD", out["ref"])
	assert.Equal(t, float64(3), out["totalReturned"])
	assert.Equal(t, false, out["truncated"])
	entries := out["entries"].([]any)
	assert.Equal(t, "internal/billing", entries[0].(map[string]any)["path"])
	assert.Equal(t, "tree", entries[0].(map[string]any)["type"])
	assert.Equal(t, "internal/billing/invoice.go", entries[2].(map[string]any)["path"])
}

func TestListTree_RequiresRepo(t *testing.T) {
	result := execute(t, newTestServer(t, nil), "sourcebot_list_tree", map[string]any{})
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "repo is required")
}

func TestListCommits(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"current field names", `[{"hash":"abc123","date":"2026-01-02T03:04:05Z","message":"Add invoice totals","refs":"HEAD -> main","body":"Totals now include tax.\n","authorName":"Ada Lovelace","authorEmail":"ada@example.com"}]`},
		{"legacy field names", `[{"hash":"abc123","date":"2026-01-02T03:04:05Z","message":"Add invoice totals","refs":"HEAD -> main","body":"Totals now include tax.\n","author_name":"Ada Lovelace","author_email":"ada@example.com"}]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var query string
			s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/commits", r.URL.Path)
				query = r.URL.RawQuery
				w.Header().Set("X-Total-Count", "1")
				_, _ = io.WriteString(w, tt.body)
			})
			out := decodeResult(t, execute(t, s, "sourcebot_list_commits", map[string]any{"repo": "github.com/acme/api", "author": "ada", "since": "30 days ago", "path": "internal/billing/invoice.go"}))
			assert.Equal(t, "author=ada&page=1&path=internal%2Fbilling%2Finvoice.go&perPage=30&repo=github.com%2Facme%2Fapi&since=30+days+ago", query)
			commits := out["commits"].([]any)
			require.Len(t, commits, 1)
			assert.Equal(t, map[string]any{
				"hash": "abc123", "date": "2026-01-02T03:04:05Z", "message": "Add invoice totals", "refs": "HEAD -> main",
				"body": "Totals now include tax.", "authorName": "Ada Lovelace", "authorEmail": "ada@example.com",
			}, commits[0])
			assert.Equal(t, float64(1), out["totalCount"])
			assert.Equal(t, false, out["hasMore"])
		})
	}
}

func TestListCommits_RequiresRepo(t *testing.T) {
	result := execute(t, newTestServer(t, nil), "sourcebot_list_commits", map[string]any{})
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "repo is required")
}

const symbolFixture = `{"stats":{"matchCount":3},"files":[{"fileName":"internal/billing/invoice.go","repository":"github.com/acme/api","repositoryId":1,"webUrl":"https://sourcebot.example.com/browse/github.com/acme/api/-/blob/internal/billing/invoice.go","language":"Go","matches":[
  {"lineContent":"func ComputeInvoiceTotal(items []int) int {\n","range":{"start":{"byteOffset":62,"column":6,"lineNumber":4},"end":{"byteOffset":81,"column":25,"lineNumber":4}}},
  {"lineContent":"// ComputeInvoiceTotal sums line items.\n","range":{"start":{"byteOffset":20,"column":4,"lineNumber":3},"end":{"byteOffset":39,"column":23,"lineNumber":3}}},
  {"lineContent":"// ComputeInvoiceTotal sums line items.\n","range":{"start":{"byteOffset":25,"column":9,"lineNumber":3},"end":{"byteOffset":30,"column":14,"lineNumber":3}}}
]}],"repositoryInfo":[{"id":1,"codeHostType":"github","name":"github.com/acme/api"}]}`

func TestFindSymbols(t *testing.T) {
	for tool, path := range map[string]string{"sourcebot_find_references": "/api/find_references", "sourcebot_find_definitions": "/api/find_definitions"} {
		t.Run(tool, func(t *testing.T) {
			var body map[string]any
			s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, path, r.URL.Path)
				body = readBody(t, r)
				_, _ = io.WriteString(w, symbolFixture)
			})
			out := decodeResult(t, execute(t, s, tool, map[string]any{"symbol": "ComputeInvoiceTotal", "language": "Go", "repo": "github.com/acme/api", "ref": "main"}))
			assert.Equal(t, map[string]any{"symbolName": "ComputeInvoiceTotal", "language": "Go", "repoName": "github.com/acme/api", "revisionName": "main"}, body)
			assert.Equal(t, float64(3), out["matchCount"])
			file := out["files"].([]any)[0].(map[string]any)
			assert.Equal(t, "internal/billing/invoice.go", file["fileName"])
			assert.Equal(t, []any{
				map[string]any{"line": float64(3), "content": "// ComputeInvoiceTotal sums line items."},
				map[string]any{"line": float64(4), "content": "func ComputeInvoiceTotal(items []int) int {"},
			}, file["matches"])
		})
	}
}

func TestFindSymbols_OptionalScope(t *testing.T) {
	var body map[string]any
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body = readBody(t, r)
		_, _ = io.WriteString(w, `{"stats":{"matchCount":0},"files":[],"repositoryInfo":[]}`)
	})
	out := decodeResult(t, execute(t, s, "sourcebot_find_references", map[string]any{"symbol": "x", "language": "Go"}))
	assert.Equal(t, map[string]any{"symbolName": "x", "language": "Go"}, body)
	assert.Equal(t, []any{}, out["files"])
}

func TestFindSymbols_Validation(t *testing.T) {
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) { t.Errorf("unexpected request") })
	for _, tt := range []struct {
		args map[string]any
		want string
	}{
		{map[string]any{"language": "Go"}, "symbol is required"},
		{map[string]any{"symbol": "x"}, "language is required"},
	} {
		result := execute(t, s, "sourcebot_find_definitions", tt.args)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Data, tt.want)
	}
}

func TestListLanguageModels(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []any
	}{
		{"configured", `[{"provider":"anthropic","model":"claude-sonnet","displayName":"Claude"}]`, []any{map[string]any{"provider": "anthropic", "model": "claude-sonnet", "displayName": "Claude"}}},
		{"none", `[]`, []any{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/models", r.URL.Path)
				_, _ = io.WriteString(w, tt.body)
			})
			out := decodeResult(t, execute(t, s, "sourcebot_list_language_models", nil))
			assert.Equal(t, tt.want, out["models"])
		})
	}
}

func TestAskCodebase(t *testing.T) {
	var body map[string]any
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat/blocking", r.URL.Path)
		body = readBody(t, r)
		_, _ = io.WriteString(w, `{"answer":"Totals are summed in ComputeInvoiceTotal.","chatId":"c1","chatUrl":"https://sourcebot.example.com/chat/c1","languageModel":{"provider":"anthropic","model":"claude-sonnet"}}`)
	})
	out := decodeResult(t, execute(t, s, "sourcebot_ask_codebase", map[string]any{
		"query": "How are invoice totals computed?", "repos": []any{"github.com/acme/api"},
		"model_provider": "anthropic", "model": "claude-sonnet", "visibility": "public",
	}))
	assert.Equal(t, map[string]any{
		"query": "How are invoice totals computed?", "repos": []any{"github.com/acme/api"},
		"languageModel": map[string]any{"provider": "anthropic", "model": "claude-sonnet"}, "visibility": "PUBLIC",
	}, body)
	assert.Equal(t, "Totals are summed in ComputeInvoiceTotal.", out["answer"])
	assert.Equal(t, "https://sourcebot.example.com/chat/c1", out["chatUrl"])
}

func TestAskCodebase_UsesLongTimeoutClient(t *testing.T) {
	s := New().(*sourcebot)
	assert.Equal(t, requestTimeout, s.client.Timeout)
	assert.Equal(t, askTimeout, s.askClient.Timeout)
	assert.Greater(t, askTimeout, requestTimeout)
}

func TestAskCodebase_NoModelConfigured(t *testing.T) {
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"statusCode":400,"errorCode":"INVALID_REQUEST_BODY","message":"No language models are configured."}`)
	})
	result := execute(t, s, "sourcebot_ask_codebase", map[string]any{"query": "why?"})
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "No language models are configured.")
}

func TestAskCodebase_Validation(t *testing.T) {
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) { t.Errorf("unexpected request") })
	for _, tt := range []struct {
		args map[string]any
		want string
	}{
		{map[string]any{}, "query is required"},
		{map[string]any{"query": "q", "model": "m"}, "must be set together"},
		{map[string]any{"query": "q", "model_provider": "p"}, "must be set together"},
		{map[string]any{"query": "q", "visibility": "secret"}, "PRIVATE or PUBLIC"},
	} {
		result := execute(t, s, "sourcebot_ask_codebase", tt.args)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Data, tt.want)
	}
}
