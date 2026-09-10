package forgejo

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

type toolCase struct {
	name     mcp.ToolName
	args     map[string]any
	method   string
	path     string
	query    string
	payload  map[string]any
	response string
}

func toolCases() []toolCase {
	repo := map[string]any{"owner": "alice", "repo": "demo"}
	with := func(extra map[string]any) map[string]any {
		result := maps.Clone(repo)
		maps.Copy(result, extra)
		return result
	}
	return []toolCase{
		{"forgejo_list_user_repos", nil, "GET", "/user/repos", "limit=30&page=1", nil, `[{"id":1,"name":"demo"}]`},
		{"forgejo_search_repos", map[string]any{"query": "demo"}, "GET", "/repos/search", "limit=30&page=1&q=demo", nil, `{"data":[{"id":1,"name":"demo"}]}`},
		{"forgejo_get_repo", repo, "GET", "/repos/alice/demo", "", nil, `{"id":1,"name":"demo"}`},
		{"forgejo_list_org_repos", map[string]any{"org": "team"}, "GET", "/orgs/team/repos", "limit=30&page=1", nil, `[{"id":1}]`},
		{"forgejo_list_user_orgs", nil, "GET", "/user/orgs", "limit=30&page=1", nil, `[{"id":1}]`},
		{"forgejo_get_current_user", nil, "GET", "/user", "", nil, `{"id":1,"login":"alice"}`},
		{"forgejo_list_branches", repo, "GET", "/repos/alice/demo/branches", "limit=30&page=1", nil, `[{"name":"main"}]`},
		{"forgejo_get_branch", with(map[string]any{"branch": "feature/docs"}), "GET", "/repos/alice/demo/branches/feature%2Fdocs", "", nil, `{"name":"feature/docs"}`},
		{"forgejo_list_commits", with(map[string]any{"sha": "feature/docs", "path": "src/main.go"}), "GET", "/repos/alice/demo/commits", "limit=0&page=1&sha=feature%2Fdocs&path=src%2Fmain.go&stat=false&verification=false&files=false", nil, `[{"sha":"abc"}]`},
		{"forgejo_get_commit", with(map[string]any{"sha": "abc"}), "GET", "/repos/alice/demo/git/commits/abc", "", nil, `{"sha":"abc"}`},
		{"forgejo_get_file_contents", with(map[string]any{"path": "src/a b.go", "ref": "feature/docs"}), "GET", "/repos/alice/demo/contents/src/a%20b.go", "ref=feature%2Fdocs", nil, `{"name":"a b.go","path":"src/a b.go","type":"file","encoding":"base64","content":"aGVsbG8=","size":5}`},
		{"forgejo_list_directory", with(map[string]any{"path": "src", "ref": "main"}), "GET", "/repos/alice/demo/contents/src", "ref=main", nil, `[{"name":"main.go","type":"file"}]`},
		{"forgejo_list_releases", repo, "GET", "/repos/alice/demo/releases", "limit=30&page=1", nil, `[{"id":3,"tag_name":"v1"}]`},
		{"forgejo_get_release", with(map[string]any{"release_id": 3}), "GET", "/repos/alice/demo/releases/3", "", nil, `{"id":3,"tag_name":"v1"}`},
		{"forgejo_list_labels", repo, "GET", "/repos/alice/demo/labels", "limit=30&page=1", nil, `[{"id":8,"name":"bug"}]`},
		{"forgejo_list_issues", repo, "GET", "/repos/alice/demo/issues", "limit=30&page=1&state=open&type=issues", nil, `[{"id":999,"number":12,"title":"bug"}]`},
		{"forgejo_get_issue", with(map[string]any{"number": 12}), "GET", "/repos/alice/demo/issues/12", "", nil, `{"id":999,"number":12,"title":"bug"}`},
		{"forgejo_create_issue", with(map[string]any{"title": "bug", "body": "details", "assignees": []any{"bob"}, "labels": []any{float64(8)}, "closed": false}), "POST", "/repos/alice/demo/issues", "", map[string]any{"title": "bug", "body": "details", "assignees": []any{"bob"}, "labels": []any{float64(8)}, "closed": false}, `{"number":12}`},
		{"forgejo_update_issue", with(map[string]any{"number": 12, "body": "", "assignees": []any{}, "state": "closed"}), "PATCH", "/repos/alice/demo/issues/12", "", map[string]any{"body": "", "assignees": []any{}, "state": "closed"}, `{"number":12}`},
		{"forgejo_list_issue_comments", with(map[string]any{"number": 12}), "GET", "/repos/alice/demo/issues/12/comments", "limit=0&page=0", nil, `[{"id":1,"body":"hello"}]`},
		{"forgejo_create_issue_comment", with(map[string]any{"number": 12, "body": "hello"}), "POST", "/repos/alice/demo/issues/12/comments", "", map[string]any{"body": "hello"}, `{"id":1,"body":"hello"}`},
		{"forgejo_list_pulls", repo, "GET", "/repos/alice/demo/pulls", "limit=30&page=1&state=open", nil, `[{"id":998,"number":12}]`},
		{"forgejo_get_pull", with(map[string]any{"number": 12}), "GET", "/repos/alice/demo/pulls/12", "", nil, `{"id":998,"number":12}`},
		{"forgejo_create_pull", with(map[string]any{"title": "fix", "head": "bob:feature/docs", "base": "main", "body": "", "labels": []any{float64(8)}, "assignees": []any{}}), "POST", "/repos/alice/demo/pulls", "", map[string]any{"title": "fix", "head": "bob:feature/docs", "base": "main", "body": "", "labels": []any{float64(8)}, "assignees": []any{}}, `{"number":12}`},
		{"forgejo_update_pull", with(map[string]any{"number": 12, "body": "", "assignees": []any{}, "labels": []any{}, "allow_maintainer_edit": false}), "PATCH", "/repos/alice/demo/pulls/12", "", map[string]any{"body": "", "assignees": []any{}, "labels": []any{}, "allow_maintainer_edit": false}, `{"number":12}`},
		{"forgejo_get_pull_diff", with(map[string]any{"number": 12}), "GET", "/repos/alice/demo/pulls/12.diff", "", nil, "diff --git a/a b/a\n+hello\n"},
		{"forgejo_list_pull_files", with(map[string]any{"number": 12}), "GET", "/repos/alice/demo/pulls/12/files", "limit=30&page=1", nil, `[{"filename":"main.go"}]`},
		{"forgejo_list_pull_reviews", with(map[string]any{"number": 12}), "GET", "/repos/alice/demo/pulls/12/reviews", "limit=30&page=1", nil, `[{"id":1,"state":"APPROVED"}]`},
		{"forgejo_create_pull_review", with(map[string]any{"number": 12, "event": "APPROVED", "body": "LGTM", "commit_id": "abc"}), "POST", "/repos/alice/demo/pulls/12/reviews", "", map[string]any{"event": "APPROVED", "body": "LGTM", "commit_id": "abc"}, `{"id":1,"state":"APPROVED"}`},
		{"forgejo_merge_pull", with(map[string]any{"number": 12, "merge_method": "squash", "sha": "abc", "delete_branch_after_merge": false}), "POST", "/repos/alice/demo/pulls/12/merge", "", map[string]any{"Do": "squash", "head_commit_id": "abc", "delete_branch_after_merge": false, "force_merge": false}, `{}`},
	}
}

func TestAllToolsHTTP(t *testing.T) {
	cases := toolCases()
	require.Len(t, cases, 30)
	for _, tt := range cases {
		t.Run(string(tt.name), func(t *testing.T) {
			var calls atomic.Int32
			f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				require.Equal(t, tt.method, r.Method)
				require.Equal(t, "/forge/api/v1"+tt.path, r.URL.EscapedPath())
				query, err := url.ParseQuery(tt.query)
				require.NoError(t, err)
				require.Equal(t, query, r.URL.Query())
				require.Equal(t, "token test-token", r.Header.Get("Authorization"))
				require.Empty(t, r.Header.Get("Sudo"))
				if tt.payload != nil {
					require.Equal(t, "application/json", r.Header.Get("Content-Type"))
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					for key, want := range tt.payload {
						require.Equal(t, want, payload[key], key)
					}
				}
				fmt.Fprint(w, tt.response)
			})
			result, err := f.Execute(context.Background(), tt.name, tt.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			require.True(t, json.Valid([]byte(result.Data)), result.Data)
			require.EqualValues(t, 1, calls.Load())
			if tt.name == "forgejo_get_pull_diff" {
				require.JSONEq(t, `{"diff":"diff --git a/a b/a\n+hello\n"}`, result.Data)
			}
			if tt.name == "forgejo_merge_pull" {
				require.JSONEq(t, `{"merged":true}`, result.Data)
			}
			if tt.name == "forgejo_search_repos" {
				require.True(t, strings.HasPrefix(result.Data, "["), result.Data)
			}
			if tt.name == "forgejo_get_issue" || tt.name == "forgejo_get_pull" {
				require.Contains(t, result.Data, `"number":12`)
			}
		})
	}
}

func TestPaginationEveryList(t *testing.T) {
	for _, tt := range toolCases() {
		if !strings.Contains(tt.query, "page=") || tt.name == "forgejo_list_issue_comments" {
			continue
		}
		t.Run(string(tt.name), func(t *testing.T) {
			f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "2", r.URL.Query().Get("page"))
				require.Equal(t, "50", r.URL.Query().Get("limit"))
				fmt.Fprint(w, tt.response)
			})
			args := maps.Clone(tt.args)
			if args == nil {
				args = map[string]any{}
			}
			delete(args, "path")
			args["page"], args["per_page"] = 2, 50
			result, err := f.Execute(context.Background(), tt.name, args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
		})
	}
}

func TestInvalidArgumentsNeverRequest(t *testing.T) {
	var calls atomic.Int32
	f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); fmt.Fprint(w, `{}`) })
	for _, tt := range toolCases() {
		t.Run(string(tt.name), func(t *testing.T) {
			var definition mcp.ToolDefinition
			for _, tool := range f.Tools() {
				if tool.Name == tt.name {
					definition = tool
				}
			}
			for _, key := range definition.Required {
				args := maps.Clone(tt.args)
				delete(args, key)
				result, err := f.Execute(context.Background(), tt.name, args)
				require.NoError(t, err)
				require.True(t, result.IsError, "missing %s", key)
			}
			for key := range definition.Parameters {
				for _, value := range []any{nil, map[string]any{"bad": true}} {
					args := maps.Clone(tt.args)
					if args == nil {
						args = map[string]any{}
					}
					args[key] = value
					result, err := f.Execute(context.Background(), tt.name, args)
					require.NoError(t, err)
					require.True(t, result.IsError, "invalid %s=%v", key, value)
				}
			}
			if strings.Contains(tt.query, "page=") {
				for _, key := range []string{"page", "per_page"} {
					for _, value := range []any{0, -1, 1.5, "bad", true, math.Inf(1), math.NaN(), float64(1 << 63)} {
						args := maps.Clone(tt.args)
						if args == nil {
							args = map[string]any{}
						}
						args[key] = value
						result, err := f.Execute(context.Background(), tt.name, args)
						require.NoError(t, err)
						require.True(t, result.IsError, "invalid %s=%v", key, value)
					}
				}
				args := maps.Clone(tt.args)
				if args == nil {
					args = map[string]any{}
				}
				args["per_page"] = 51
				result, err := f.Execute(context.Background(), tt.name, args)
				require.NoError(t, err)
				require.True(t, result.IsError)
			}
		})
	}
	require.Zero(t, calls.Load())
}

func TestMalformedValues(t *testing.T) {
	tests := []struct {
		tool   mcp.ToolName
		key    string
		values []any
	}{
		{"forgejo_get_repo", "owner", []any{"", " ", ".", "..", "a/b", "a\\b", "a?b", "a#b", "%2e%2e", "%252e%252e", 123}},
		{"forgejo_get_repo", "repo", []any{"", ".", "..", "a/b", "a\\b", "a\x00b"}},
		{"forgejo_get_file_contents", "path", []any{"", ".", "..", "../a", "a/../b", "a/./b", "/a", "a//b", "a/", "a\x00b", "a\\b"}},
		{"forgejo_get_branch", "branch", []any{"", ".", "..", "a/../b", "a/./b", "/a", "a/", "a//b", "a\\b", "a\x00b"}},
		{"forgejo_get_issue", "number", []any{0, -1, 1.5, "bad", true, float64(1 << 63)}},
		{"forgejo_get_release", "release_id", []any{0, -1, 1.5}},
		{"forgejo_create_issue", "title", []any{"", " ", false}},
		{"forgejo_create_issue", "labels", []any{[]any{1.5}, []any{0}, []any{-1}, "bug", []any{"bug"}}},
		{"forgejo_create_issue", "assignees", []any{[]any{true}, "bob", []any{""}}},
		{"forgejo_create_issue", "closed", []any{"false", 1}},
		{"forgejo_update_issue", "state", []any{"all", "merged", ""}},
		{"forgejo_create_pull", "head", []any{"", " "}},
		{"forgejo_create_pull", "base", []any{"", " "}},
		{"forgejo_create_issue_comment", "body", []any{"", " "}},
		{"forgejo_create_pull_review", "event", []any{"PENDING", "APPROVE", "REQUEST_REVIEW", ""}},
		{"forgejo_merge_pull", "merge_method", []any{"manually-merged", "force", ""}},
		{"forgejo_merge_pull", "force_merge", []any{true}},
	}
	var calls atomic.Int32
	f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); fmt.Fprint(w, `{}`) })
	for _, tt := range tests {
		t.Run(string(tt.tool)+"/"+tt.key, func(t *testing.T) {
			var base map[string]any
			for _, tc := range toolCases() {
				if tc.name == tt.tool {
					base = tc.args
				}
			}
			for _, value := range tt.values {
				args := maps.Clone(base)
				args[tt.key] = value
				result, err := f.Execute(context.Background(), tt.tool, args)
				require.NoError(t, err)
				require.True(t, result.IsError, "%s=%v", tt.key, value)
			}
		})
	}
	require.Zero(t, calls.Load())
}

func TestOptionalFiltersAndAlternateRoutes(t *testing.T) {
	tests := []struct {
		name                  mcp.ToolName
		args                  map[string]any
		path, query, response string
	}{
		{"forgejo_list_user_repos", map[string]any{"username": "bob"}, "/users/bob/repos", "limit=30&page=1", "[]"},
		{"forgejo_list_user_orgs", map[string]any{"username": "bob"}, "/users/bob/orgs", "limit=30&page=1", "[]"},
		{"forgejo_search_repos", map[string]any{"query": "code & docs", "sort": "updated", "order": "desc"}, "/repos/search", "limit=30&page=1&q=code+%26+docs&sort=updated&order=desc", `{"data":[]}`},
		{"forgejo_list_directory", map[string]any{"owner": "alice", "repo": "demo"}, "/repos/alice/demo/contents/", "ref=", "[]"},
		{"forgejo_list_directory", map[string]any{"owner": "alice", "repo": "demo", "path": "", "ref": ""}, "/repos/alice/demo/contents/", "ref=", "[]"},
		{"forgejo_list_releases", map[string]any{"owner": "alice", "repo": "demo", "draft": false, "prerelease": true}, "/repos/alice/demo/releases", "limit=30&page=1&draft=false&pre-release=true", "[]"},
		{"forgejo_list_issues", map[string]any{"owner": "alice", "repo": "demo", "state": "all", "labels": []string{"bug", "help wanted"}, "query": "fix & test"}, "/repos/alice/demo/issues", "limit=30&page=1&type=issues&state=all&labels=bug%2Chelp+wanted&q=fix+%26+test", "[]"},
		{"forgejo_list_pulls", map[string]any{"owner": "alice", "repo": "demo", "state": "closed", "sort": "recentupdate"}, "/repos/alice/demo/pulls", "limit=30&page=1&state=closed&sort=recentupdate", "[]"},
		{"forgejo_list_commits", map[string]any{"owner": "alice", "repo": "demo"}, "/repos/alice/demo/commits", "limit=30&page=1&files=false&stat=false&verification=false", "[]"},
	}
	for _, tt := range tests {
		t.Run(string(tt.name)+tt.query, func(t *testing.T) {
			f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "GET", r.Method)
				require.Equal(t, "/forge/api/v1"+tt.path, r.URL.Path)
				query, err := url.ParseQuery(tt.query)
				require.NoError(t, err)
				require.Equal(t, query, r.URL.Query())
				fmt.Fprint(w, tt.response)
			})
			result, err := f.Execute(context.Background(), tt.name, tt.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
		})
	}
}

func TestReviewEvents(t *testing.T) {
	for _, event := range []string{"APPROVED", "COMMENT", "REQUEST_CHANGES"} {
		for _, body := range []string{"", "review text"} {
			t.Run(event+"/"+body, func(t *testing.T) {
				var calls atomic.Int32
				f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					require.Equal(t, event, payload["event"])
					require.Equal(t, body, payload["body"])
					fmt.Fprint(w, `{"id":1}`)
				})
				result, err := f.Execute(context.Background(), "forgejo_create_pull_review", map[string]any{"owner": "alice", "repo": "demo", "number": 12, "event": event, "body": body})
				require.NoError(t, err)
				invalid := event != "APPROVED" && body == ""
				require.Equal(t, invalid, result.IsError, result.Data)
				if invalid {
					require.Zero(t, calls.Load())
				} else {
					require.EqualValues(t, 1, calls.Load())
				}
			})
		}
	}
}

func TestMergeStyles(t *testing.T) {
	for _, style := range []string{"", "merge", "rebase", "rebase-merge", "squash", "fast-forward-only"} {
		t.Run(style, func(t *testing.T) {
			f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
				var payload map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
				want := style
				if want == "" {
					want = "merge"
				}
				require.Equal(t, want, payload["Do"])
				require.Equal(t, "expected-sha", payload["head_commit_id"])
				require.Equal(t, "merge title", payload["MergeTitleField"])
				require.Equal(t, "merge body", payload["MergeMessageField"])
				require.Equal(t, true, payload["delete_branch_after_merge"])
				require.Equal(t, false, payload["force_merge"])
				require.Equal(t, false, payload["merge_when_checks_succeed"])
				w.WriteHeader(http.StatusOK)
			})
			args := map[string]any{"owner": "alice", "repo": "demo", "number": 12, "sha": "expected-sha", "commit_title": "merge title", "commit_message": "merge body", "delete_branch_after_merge": true}
			if style != "" {
				args["merge_method"] = style
			}
			result, err := f.Execute(context.Background(), "forgejo_merge_pull", args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			require.JSONEq(t, `{"merged":true}`, result.Data)
		})
	}
}

func TestAllToolsHTTPFailures(t *testing.T) {
	for _, tc := range toolCases() {
		for _, status := range []int{401, 403, 404, 429, 500} {
			t.Run(fmt.Sprintf("%s/%d", tc.name, status), func(t *testing.T) {
				var calls atomic.Int32
				f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					w.Header().Set("Retry-After", "3")
					w.WriteHeader(status)
					fmt.Fprint(w, `{"message":"not available"}`)
				})
				result, err := f.Execute(context.Background(), tc.name, tc.args)
				require.EqualValues(t, 1, calls.Load())
				if tc.method == http.MethodGet && (status == 429 || status >= 500) {
					var retry *mcp.RetryableError
					require.ErrorAs(t, err, &retry)
					require.Equal(t, status, retry.StatusCode)
					require.Nil(t, result)
				} else {
					require.NoError(t, err)
					require.True(t, result.IsError)
					if tc.name != "forgejo_merge_pull" {
						require.Contains(t, result.Data, "not available")
					}
					if status == 429 || status >= 500 {
						require.Contains(t, result.Data, fmt.Sprint(status))
						require.Contains(t, result.Data, "uncertain")
						require.Contains(t, result.Data, "verify")
						require.Contains(t, result.Data, "before retry")
					}
				}
			})
		}
	}
}

func TestAllToolsCanceledBeforeRequest(t *testing.T) {
	var calls atomic.Int32
	f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); fmt.Fprint(w, `{}`) })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range toolCases() {
		t.Run(string(tc.name), func(t *testing.T) {
			result, err := f.Execute(ctx, tc.name, tc.args)
			require.NoError(t, err)
			require.True(t, result.IsError)
			require.Contains(t, result.Data, context.Canceled.Error())
		})
	}
	require.Zero(t, calls.Load())
}

func TestFileDirectoryMismatch(t *testing.T) {
	for _, tt := range []struct {
		name mcp.ToolName
		body string
	}{{"forgejo_get_file_contents", "[]"}, {"forgejo_list_directory", `{"type":"file"}`}} {
		t.Run(string(tt.name), func(t *testing.T) {
			f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tt.body) })
			result, err := f.Execute(context.Background(), tt.name, map[string]any{"owner": "alice", "repo": "demo", "path": "src"})
			require.NoError(t, err)
			require.True(t, result.IsError)
		})
	}
}

func TestIssueCommentsFilters(t *testing.T) {
	for _, filters := range []map[string]any{
		{},
		{"since": "2025-01-02T10:00:00Z"},
		{"before": "2025-01-02T11:00:00Z"},
		{"since": "2025-01-02T12:00:00+02:00", "before": "2025-01-02T11:00:00Z"},
	} {
		t.Run(fmt.Sprint(filters), func(t *testing.T) {
			var calls atomic.Int32
			f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "/forge/api/v1/repos/alice/demo/issues/12/comments", r.URL.Path)
				query := url.Values{"page": {"0"}, "limit": {"0"}}
				for key, value := range filters {
					query.Set(key, value.(string))
				}
				require.Equal(t, query, r.URL.Query())
				comments := make([]map[string]any, 65)
				for i := range comments {
					comments[i] = map[string]any{"id": i + 1, "body": "hello"}
				}
				require.NoError(t, json.NewEncoder(w).Encode(comments))
			})
			args := map[string]any{"owner": "alice", "repo": "demo", "number": 12}
			maps.Copy(args, filters)
			result, err := f.Execute(t.Context(), "forgejo_list_issue_comments", args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			var comments []map[string]any
			require.NoError(t, json.Unmarshal([]byte(result.Data), &comments))
			require.Len(t, comments, 65)
			require.EqualValues(t, 1, calls.Load())
		})
	}
}

func TestIssueCommentsRejectUnsupportedFilters(t *testing.T) {
	for _, tt := range []struct {
		filters map[string]any
		message string
	}{
		{map[string]any{"page": 2}, "unknown parameter"},
		{map[string]any{"per_page": 10}, "unknown parameter"},
		{map[string]any{"since": ""}, "RFC3339"},
		{map[string]any{"since": "2025-01-02"}, "RFC3339"},
		{map[string]any{"since": "2025-01-02T10:00:00"}, "RFC3339"},
		{map[string]any{"before": "2025-02-30T00:00:00Z"}, "RFC3339"},
		{map[string]any{"before": "not-a-time"}, "RFC3339"},
		{map[string]any{"since": "2025-01-02T10:00:00+24:00"}, "RFC3339"},
		{map[string]any{"since": "2025-01-02T10:00:00+02:60"}, "RFC3339"},
		{map[string]any{"since": "2025-01-02T1:00:00Z"}, "RFC3339"},
		{map[string]any{"since": "2025-01-02T10:00:00,100Z"}, "RFC3339"},
		{map[string]any{"since": "2025-01-02T10:00:00.100Z", "before": "2025-01-02T10:00:00.900Z"}, "whole-second"},
		{map[string]any{"since": "0001-01-01T00:00:00Z"}, "nonzero"},
		{map[string]any{"since": "2025-01-02T10:00:00Z", "before": "2025-01-02T10:00:00Z"}, "since must be before before"},
		{map[string]any{"since": "2025-01-02T10:00:00Z", "before": "2025-01-02T11:00:00+02:00"}, "since must be before before"},
	} {
		t.Run(fmt.Sprint(tt.filters), func(t *testing.T) {
			var calls atomic.Int32
			f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				fmt.Fprint(w, "[]")
			})
			args := map[string]any{"owner": "alice", "repo": "demo", "number": 12}
			maps.Copy(args, tt.filters)
			result, err := f.Execute(t.Context(), "forgejo_list_issue_comments", args)
			require.NoError(t, err)
			require.True(t, result.IsError, result.Data)
			require.Contains(t, result.Data, tt.message)
			require.Zero(t, calls.Load())
		})
	}
}

func TestPaginationExceptionDescriptions(t *testing.T) {
	for _, tool := range New().Tools() {
		switch tool.Name {
		case "forgejo_list_issue_comments":
			require.NotContains(t, tool.Parameters, "page")
			require.NotContains(t, tool.Parameters, "per_page")
			require.Contains(t, tool.Parameters["since"], "RFC3339")
			require.Contains(t, tool.Parameters["before"], "RFC3339")
			require.Contains(t, tool.Description, "unpaginated")
			require.Contains(t, tool.Description, "since")
			require.Contains(t, tool.Description, "before")
		case "forgejo_list_commits":
			require.Contains(t, tool.Description, "server-controlled")
			require.Contains(t, tool.Parameters["per_page"], "path")
		}
	}
}

func TestListCommitsPathPagination(t *testing.T) {
	for _, tt := range []struct {
		name        string
		extra       map[string]any
		page, limit string
		invalid     bool
	}{
		{"path default page", map[string]any{"path": "src/main.go"}, "1", "0", false},
		{"path next page", map[string]any{"path": "src/main.go", "page": 2}, "2", "0", false},
		{"path explicit size", map[string]any{"path": "src/main.go", "per_page": 30}, "", "", true},
		{"path explicit small size", map[string]any{"path": "src/main.go", "per_page": 1}, "", "", true},
		{"unfiltered explicit size", map[string]any{"per_page": 1, "page": 2}, "2", "1", false},
		{"empty path explicit size", map[string]any{"path": "", "per_page": 1}, "1", "1", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if !tt.invalid {
					require.Equal(t, tt.page, r.URL.Query().Get("page"))
					require.Equal(t, tt.limit, r.URL.Query().Get("limit"))
				}
				commits := make([]map[string]string, 65)
				for i := range commits {
					commits[i] = map[string]string{"sha": fmt.Sprint(i)}
				}
				require.NoError(t, json.NewEncoder(w).Encode(commits))
			})
			args := map[string]any{"owner": "alice", "repo": "demo"}
			maps.Copy(args, tt.extra)
			result, err := f.Execute(t.Context(), "forgejo_list_commits", args)
			require.NoError(t, err)
			require.Equal(t, tt.invalid, result.IsError, result.Data)
			if tt.invalid {
				require.Contains(t, result.Data, "per_page")
				require.Contains(t, result.Data, "path")
				require.Contains(t, result.Data, "server-controlled")
				require.Zero(t, calls.Load())
				return
			}
			var commits []map[string]any
			require.NoError(t, json.Unmarshal([]byte(result.Data), &commits))
			require.Len(t, commits, 65)
			require.EqualValues(t, 1, calls.Load())
		})
	}
}

func TestDiscoveredPathsAndBranchesRoundTrip(t *testing.T) {
	for _, name := range []string{"C#.md", "100%", "why?.md", "%2e", "%2e%2e", "%252e%252e"} {
		t.Run(name, func(t *testing.T) {
			branch := "feature/#100%"
			path := "docs/" + name
			var calls atomic.Int32
			f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				require.Equal(t, http.MethodGet, r.Method)
				prefix := "/forge/api/v1/repos/alice/demo"
				switch r.URL.EscapedPath() {
				case prefix + "/branches":
					require.NoError(t, json.NewEncoder(w).Encode([]map[string]string{{"name": branch}}))
				case prefix + "/branches/" + url.PathEscape(branch):
					require.Empty(t, r.URL.RawQuery)
					require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"name": branch}))
				case prefix + "/contents/docs":
					require.Equal(t, url.Values{"ref": {branch}}, r.URL.Query())
					require.NoError(t, json.NewEncoder(w).Encode([]map[string]string{{"path": path, "name": name, "type": "file"}}))
				case prefix + "/contents/docs/" + url.PathEscape(name):
					require.Equal(t, url.Values{"ref": {branch}}, r.URL.Query())
					require.Equal(t, prefix+"/contents/"+path, r.URL.Path)
					require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"path": path, "type": "file", "content": "aGVsbG8=", "encoding": "base64"}))
				default:
					t.Errorf("unexpected request: %s", r.URL)
					http.NotFound(w, r)
				}
			})
			args := map[string]any{"owner": "alice", "repo": "demo"}
			result, err := f.Execute(t.Context(), "forgejo_list_branches", args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			var branches []map[string]any
			require.NoError(t, json.Unmarshal([]byte(result.Data), &branches))
			args["branch"] = branches[0]["name"]
			result, err = f.Execute(t.Context(), "forgejo_get_branch", args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			delete(args, "branch")
			args["ref"], args["path"] = branches[0]["name"], "docs"
			result, err = f.Execute(t.Context(), "forgejo_list_directory", args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			var entries []map[string]any
			require.NoError(t, json.Unmarshal([]byte(result.Data), &entries))
			args["path"] = entries[0]["path"]
			result, err = f.Execute(t.Context(), "forgejo_get_file_contents", args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			var file map[string]any
			require.NoError(t, json.Unmarshal([]byte(result.Data), &file))
			require.Equal(t, path, file["path"])
			require.Equal(t, "aGVsbG8=", file["content"])
			require.EqualValues(t, 4, calls.Load())
		})
	}
}

func TestSearchNullResponseDoesNotPanic(t *testing.T) {
	f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "null") })
	require.NotPanics(t, func() {
		result, err := f.Execute(context.Background(), "forgejo_search_repos", map[string]any{"query": "demo"})
		require.NoError(t, err)
		require.True(t, result.IsError)
	})
}

func TestOmittedUpdateFieldsUnchanged(t *testing.T) {
	for _, tool := range []mcp.ToolName{"forgejo_update_issue", "forgejo_update_pull"} {
		t.Run(string(tool), func(t *testing.T) {
			f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.Equal(t, "new", body["title"])
				for _, key := range []string{"body", "state", "assignees", "labels", "allow_maintainer_edit"} {
					require.Nil(t, body[key], key)
				}
				fmt.Fprint(w, `{"number":12}`)
			})
			result, err := f.Execute(context.Background(), tool, map[string]any{"owner": "alice", "repo": "demo", "number": 12, "title": "new"})
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
		})
	}
}
