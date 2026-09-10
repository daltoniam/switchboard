package forgejo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func compactConfig(t *testing.T) (compact.Result, compact.SpecFile) {
	t.Helper()
	data, err := os.ReadFile("compact.yaml")
	require.NoError(t, err)
	loaded, err := compact.Load(data, compact.Options{Strict: true})
	require.NoError(t, err)
	require.Empty(t, loaded.Warnings)
	var raw compact.SpecFile
	require.NoError(t, yaml.Unmarshal(data, &raw))
	return loaded, raw
}

func compactor(t *testing.T) mcp.FieldCompactionIntegration {
	t.Helper()
	f, ok := New().(mcp.FieldCompactionIntegration)
	require.True(t, ok, "Forgejo must implement field compaction")
	return f
}

func TestFieldCompactionSpecs_StrictLoad(t *testing.T) {
	loaded, raw := compactConfig(t)
	require.Equal(t, 1, raw.Version)
	require.Len(t, loaded.Specs, 23)
	require.Empty(t, loaded.Views)
	f := compactor(t)
	for name, fields := range loaded.Specs {
		t.Run(string(name), func(t *testing.T) {
			require.NotEmpty(t, fields)
			actual, ok := f.CompactSpec(name)
			require.True(t, ok)
			require.Equal(t, fields, actual)
			seen := map[compact.RawSpec]bool{}
			for _, path := range raw.Tools[string(name)].Spec {
				require.False(t, seen[path], "duplicate path %s", path)
				seen[path] = true
				require.NotContains(t, path, "*")
				require.False(t, strings.HasPrefix(string(path), "-"))
			}
		})
	}
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	loaded, _ := compactConfig(t)
	definitions := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		definitions[tool.Name] = true
	}
	for name := range loaded.Specs {
		t.Run(string(name), func(t *testing.T) {
			require.Contains(t, dispatch, name)
			require.True(t, definitions[name])
		})
	}
}

func TestFieldCompactionSpecs_AllReadsAndNoMutations(t *testing.T) {
	f := compactor(t)
	cases := toolCases()
	require.Len(t, cases, len(dispatch))
	covered := map[mcp.ToolName]bool{}
	for _, tt := range cases {
		t.Run(string(tt.name), func(t *testing.T) {
			require.False(t, covered[tt.name])
			covered[tt.name] = true
			fields, ok := f.CompactSpec(tt.name)
			require.Equal(t, tt.method == http.MethodGet, ok)
			if ok {
				require.NotEmpty(t, fields)
			} else {
				require.Nil(t, fields)
			}
		})
	}
	for name := range dispatch {
		require.True(t, covered[name], "unclassified handler %s", name)
	}
}

func TestFieldCompactionSpecs_UnknownAndMaxBytes(t *testing.T) {
	loaded, _ := compactConfig(t)
	f := compactor(t)
	fields, ok := f.CompactSpec("forgejo_unknown")
	require.False(t, ok)
	require.Nil(t, fields)
	caps, ok := New().(mcp.ToolMaxBytesIntegration)
	require.True(t, ok)
	for _, name := range []mcp.ToolName{"forgejo_get_file_contents", "forgejo_get_pull_diff", "forgejo_get_issue", "forgejo_create_issue", "forgejo_unknown"} {
		t.Run(string(name), func(t *testing.T) {
			limit, found := caps.MaxBytes(name)
			want, exists := loaded.MaxBytes[name]
			require.Equal(t, exists, found)
			require.Equal(t, want, limit)
			if name == "forgejo_get_file_contents" || name == "forgejo_get_pull_diff" {
				require.True(t, found)
				require.Equal(t, 1024*1024, limit)
			} else {
				require.False(t, found)
			}
		})
	}
}

type compactFixture struct {
	tools []mcp.ToolName
	value any
	want  string
	drop  []string
}

func compactFixtures() []compactFixture {
	stamp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	user := &sdk.User{ID: 7, UserName: "alice", FullName: "Alice", Email: "alice@example.test", AvatarURL: "noise"}
	label := &sdk.Label{ID: 8, Name: "bug", Color: "ff0000", Description: "Defect"}
	repo := &sdk.Repository{ID: 1, Name: "demo", FullName: "alice/demo", Owner: user, Description: "A project", DefaultBranch: "main", OpenIssues: 3, OpenPulls: 2, HTMLURL: "https://forge.test/alice/demo", Permissions: &sdk.Permission{Admin: false, Push: true, Pull: true}, AvatarURL: "noise", Updated: stamp}
	branch := &sdk.Branch{Name: "feature/docs", Protected: true, RequiredApprovals: 2, Commit: &sdk.PayloadCommit{ID: "abc", Message: "Fix docs", Timestamp: stamp, Author: &sdk.PayloadUser{Name: "Alice", UserName: "alice"}}}
	commit := &sdk.Commit{CommitMeta: &sdk.CommitMeta{SHA: "abc", Created: stamp}, HTMLURL: "https://forge.test/alice/demo/commit/abc", RepoCommit: &sdk.RepoCommit{Message: "Fix docs", Author: &sdk.CommitUser{Identity: sdk.Identity{Name: "Alice"}, Date: "2026-01-02T03:04:05Z"}, Committer: &sdk.CommitUser{Identity: sdk.Identity{Name: "Alice"}, Date: "2026-01-02T03:04:05Z"}, Tree: &sdk.CommitMeta{SHA: "tree"}}, Author: user, Parents: []*sdk.CommitMeta{{SHA: "parent"}}, Stats: &sdk.CommitStats{Total: 3, Additions: 2, Deletions: 1}, Files: []*sdk.CommitAffectedFiles{{Filename: "README.md"}}}
	encoding, content, target, submodule := "base64", "aGVsbG8=", "README.md", "https://forge.test/alice/lib.git"
	file := &sdk.ContentsResponse{Name: "README.md", Path: "docs/README.md", SHA: "blob", Type: "file", Size: 5, Encoding: &encoding, Content: &content, Target: &target, SubmoduleGitURL: &submodule}
	release := &sdk.Release{ID: 3, TagName: "v1", Target: "main", Title: "Version one", Note: "Release **notes**", Publisher: user, PublishedAt: stamp, Attachments: []*sdk.Attachment{{ID: 4, Name: "app.zip", Size: 123, DownloadURL: "https://forge.test/download/4"}}}
	milestone := &sdk.Milestone{ID: 9, Title: "v2"}
	issue := &sdk.Issue{ID: 999, Index: 12, Title: "Fix docs", Body: "Issue **body**", Ref: "main", Poster: user, State: sdk.StateOpen, Labels: []*sdk.Label{label}, Assignees: []*sdk.User{user}, Milestone: milestone, Comments: 2, Created: stamp, Updated: stamp, Repository: &sdk.RepositoryMeta{ID: 1, Name: "demo", Owner: "alice", FullName: "alice/demo"}}
	pull := &sdk.PullRequest{ID: 998, Index: 13, Title: "Fix docs", Body: "Pull **body**", Poster: user, State: sdk.StateOpen, Labels: []*sdk.Label{label}, Assignees: []*sdk.User{user}, Assignee: user, Milestone: milestone, Mergeable: true, Head: &sdk.PRBranchInfo{Name: "alice:feature/docs", Ref: "feature/docs", Sha: "abc", RepoID: 1, Repository: repo}, Base: &sdk.PRBranchInfo{Name: "alice:main", Ref: "main", Sha: "def", RepoID: 1, Repository: repo}, Created: &stamp, Updated: &stamp}
	return []compactFixture{
		{[]mcp.ToolName{"forgejo_list_user_repos", "forgejo_search_repos", "forgejo_list_org_repos"}, repo, `{"id":1,"name":"demo","full_name":"alice/demo","owner":{"id":7,"login":"alice"},"description":"A project","default_branch":"main","open_issues_count":3,"open_pr_counter":2,"private":false,"archived":false}`, []string{"permissions", "parent", "avatar_url"}},
		{[]mcp.ToolName{"forgejo_get_repo"}, repo, `{"id":1,"name":"demo","full_name":"alice/demo","owner":{"id":7,"login":"alice"},"description":"A project","default_branch":"main","open_issues_count":3,"open_pr_counter":2,"private":false,"archived":false,"permissions":{"admin":false,"push":true,"pull":true}}`, []string{"parent", "avatar_url"}},
		{[]mcp.ToolName{"forgejo_list_user_orgs"}, &sdk.Organization{ID: 2, UserName: "team", FullName: "Team", Description: "Our team", AvatarURL: "noise"}, `{"id":2,"username":"team","full_name":"Team","description":"Our team"}`, []string{"avatar_url"}},
		{[]mcp.ToolName{"forgejo_get_current_user"}, user, `{"id":7,"login":"alice","full_name":"Alice","email":"alice@example.test"}`, []string{"avatar_url", "source_id", "login_name"}},
		{[]mcp.ToolName{"forgejo_list_branches", "forgejo_get_branch"}, branch, `{"name":"feature/docs","protected":true,"required_approvals":2,"commit":{"id":"abc","message":"Fix docs","timestamp":"2026-01-02T03:04:05Z"}}`, nil},
		{[]mcp.ToolName{"forgejo_list_commits", "forgejo_get_commit"}, commit, `{"sha":"abc","author":{"id":7,"login":"alice"},"commit":{"message":"Fix docs","author":{"name":"Alice","date":"2026-01-02T03:04:05Z"},"committer":{"name":"Alice","date":"2026-01-02T03:04:05Z"}},"parents":["parent"]}`, nil},
		{[]mcp.ToolName{"forgejo_get_file_contents"}, file, `{"name":"README.md","path":"docs/README.md","sha":"blob","type":"file","size":5,"encoding":"base64","content":"aGVsbG8=","target":"README.md","submodule_git_url":"https://forge.test/alice/lib.git"}`, []string{"_links", "git_url", "url"}},
		{[]mcp.ToolName{"forgejo_list_directory"}, file, `{"name":"README.md","path":"docs/README.md","sha":"blob","type":"file","size":5}`, []string{"content", "encoding", "_links"}},
		{[]mcp.ToolName{"forgejo_list_releases", "forgejo_get_release"}, release, `{"id":3,"tag_name":"v1","target_commitish":"main","name":"Version one","draft":false,"prerelease":false,"author":{"id":7,"login":"alice"},"assets":[{"id":4,"name":"app.zip","size":123,"browser_download_url":"https://forge.test/download/4"}]}`, []string{"url"}},
		{[]mcp.ToolName{"forgejo_list_labels"}, label, `{"id":8,"name":"bug","color":"ff0000","description":"Defect"}`, []string{"url"}},
		{[]mcp.ToolName{"forgejo_list_issues", "forgejo_get_issue"}, issue, `{"id":999,"number":12,"title":"Fix docs","state":"open","ref":"main","user":{"id":7,"login":"alice"},"labels":[{"id":8,"name":"bug"}],"assignees":[{"id":7,"login":"alice"}],"milestone":{"id":9,"title":"v2"},"repository":{"id":1,"name":"demo","owner":"alice","full_name":"alice/demo"}}`, []string{"url", "original_author_id"}},
		{[]mcp.ToolName{"forgejo_list_issue_comments"}, &sdk.Comment{ID: 10, Poster: user, Body: "Comment **body**", Created: stamp, Updated: stamp, OriginalAuthor: "imported"}, `{"id":10,"body":"Comment **body**","user":{"id":7,"login":"alice"},"original_author":"imported","created_at":"2026-01-02T03:04:05Z"}`, []string{"original_author_id"}},
		{[]mcp.ToolName{"forgejo_list_pulls", "forgejo_get_pull"}, pull, `{"id":998,"number":13,"state":"open","merged":false,"mergeable":true,"labels":[{"id":8,"name":"bug"}],"assignees":[{"id":7,"login":"alice"}],"head":{"label":"alice:feature/docs","ref":"feature/docs","sha":"abc","repo":{"id":1,"full_name":"alice/demo","owner":{"id":7,"login":"alice"}}},"base":{"label":"alice:main","ref":"main","sha":"def","repo":{"id":1,"full_name":"alice/demo","owner":{"id":7,"login":"alice"}}}}`, []string{"diff_url", "patch_url", "url"}},
		{[]mcp.ToolName{"forgejo_get_pull_diff"}, map[string]string{"diff": "diff --git a/a b/a\n+hello\n"}, `{"diff":"diff --git a/a b/a\n+hello\n"}`, nil},
		{[]mcp.ToolName{"forgejo_list_pull_files"}, &sdk.ChangedFile{Filename: "new.go", PreviousFilename: "old.go", Status: "renamed", Additions: 2, Deletions: 1, Changes: 3}, `{"filename":"new.go","previous_filename":"old.go","status":"renamed","additions":2,"deletions":1,"changes":3}`, []string{"contents_url", "raw_url"}},
		{[]mcp.ToolName{"forgejo_list_pull_reviews"}, &sdk.PullReview{ID: 14, Reviewer: user, ReviewerTeam: &sdk.Team{ID: 15, Name: "maintainers"}, State: sdk.ReviewStateType("REQUEST_CHANGES"), Body: "Needs tests", CommitID: "abc", Stale: true, Official: true, CodeCommentsCount: 2, Submitted: stamp}, `{"id":14,"user":{"id":7,"login":"alice"},"team":{"id":15,"name":"maintainers"},"state":"REQUEST_CHANGES","body":"Needs tests","commit_id":"abc","stale":true,"official":true,"dismissed":false,"comments_count":2,"submitted_at":"2026-01-02T03:04:05Z"}`, nil},
	}
}

func TestFieldCompactionSpecs_HandlerShapes(t *testing.T) {
	_, raw := compactConfig(t)
	f := compactor(t)
	covered := map[mcp.ToolName]bool{}
	cases := map[mcp.ToolName]toolCase{}
	for _, tt := range toolCases() {
		cases[tt.name] = tt
	}
	for _, fixture := range compactFixtures() {
		for _, name := range fixture.tools {
			t.Run(string(name), func(t *testing.T) {
				require.False(t, covered[name])
				covered[name] = true
				value := fixture.value
				list := strings.HasPrefix(string(name), "forgejo_list_") || name == "forgejo_search_repos"
				if list {
					value = []any{value}
				}
				data, err := json.Marshal(value)
				require.NoError(t, err)
				upstream := string(data)
				if name == "forgejo_search_repos" {
					upstream = `{"data":` + upstream + `}`
				}
				if name == "forgejo_get_pull_diff" {
					upstream = fixture.value.(map[string]string)["diff"]
				}
				adapter, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
					require.Equal(t, http.MethodGet, r.Method)
					fmt.Fprint(w, upstream)
				})
				result, err := adapter.Execute(context.Background(), name, cases[name].args)
				require.NoError(t, err)
				require.False(t, result.IsError, result.Data)
				require.JSONEq(t, string(data), result.Data, "handler must preserve raw SDK JSON")
				fields, ok := f.CompactSpec(name)
				require.True(t, ok)
				output, err := mcp.CompactJSON([]byte(result.Data), fields)
				require.NoError(t, err)
				var got any
				require.NoError(t, json.Unmarshal(output, &got))
				if list {
					items, ok := got.([]any)
					require.True(t, ok, "list must stay unwrapped: %s", output)
					require.Len(t, items, 1)
					got = items[0]
				}
				object, ok := got.(map[string]any)
				require.True(t, ok)
				var want map[string]any
				require.NoError(t, json.Unmarshal([]byte(fixture.want), &want))
				for key, value := range want {
					require.Equal(t, value, object[key], "field %s in %s", key, output)
				}
				for _, key := range fixture.drop {
					require.NotContains(t, object, key)
				}
				require.NotContains(t, string(output), `"avatar_url"`)
				if name != "forgejo_get_repo" {
					require.NotContains(t, string(output), `"permissions"`)
				}
				if name == "forgejo_get_issue" || name == "forgejo_get_pull" || name == "forgejo_get_release" {
					require.NotEmpty(t, object["body"])
				}
				if name == "forgejo_list_issues" || name == "forgejo_list_pulls" || name == "forgejo_list_releases" {
					require.NotContains(t, object, "body")
				}
				if name == "forgejo_get_commit" {
					require.Equal(t, map[string]any{"total": float64(3), "additions": float64(2), "deletions": float64(1)}, object["stats"])
					require.Equal(t, []any{"README.md"}, object["files"])
				}
				var source any
				record, err := json.Marshal(fixture.value)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(record, &source))
				for _, path := range raw.Tools[string(name)].Spec {
					assertCompactPath(t, source, strings.Split(string(path), "."))
				}
			})
		}
	}
	for name := range raw.Tools {
		require.True(t, covered[mcp.ToolName(name)], "spec has no handler shape test: %s", name)
	}
}

func TestFieldCompactionSpecs_RepoPermissions(t *testing.T) {
	f := compactor(t)
	for _, tool := range New().Tools() {
		fields, ok := f.CompactSpec(tool.Name)
		if !ok {
			continue
		}
		t.Run(string(tool.Name), func(t *testing.T) {
			input := `{"id":1,"permissions":{"admin":false,"push":true,"pull":true,"extra":"noise"}}`
			output, err := mcp.CompactJSON([]byte(input), fields)
			require.NoError(t, err)
			var got map[string]any
			require.NoError(t, json.Unmarshal(output, &got))
			if tool.Name == "forgejo_get_repo" {
				require.Equal(t, map[string]any{"admin": false, "push": true, "pull": true}, got["permissions"])
			} else {
				require.NotContains(t, got, "permissions")
			}
		})
	}
}

func assertCompactPath(t *testing.T, value any, path []string) {
	t.Helper()
	if len(path) == 0 {
		return
	}
	object, ok := value.(map[string]any)
	require.True(t, ok, "expected object at %v, got %T", path, value)
	key := strings.TrimSuffix(path[0], "[]")
	child, ok := object[key]
	require.True(t, ok, "phantom spec path %v", path)
	if strings.HasSuffix(path[0], "[]") {
		items, ok := child.([]any)
		require.True(t, ok, "expected array at %v", path)
		require.NotEmpty(t, items, "fixture must exercise %v", path)
		for _, item := range items {
			assertCompactPath(t, item, path[1:])
		}
		return
	}
	assertCompactPath(t, child, path[1:])
}

func TestFieldCompactionSpecs_EmptyLists(t *testing.T) {
	f := compactor(t)
	for _, tool := range New().Tools() {
		if !strings.HasPrefix(string(tool.Name), "forgejo_list_") && tool.Name != "forgejo_search_repos" {
			continue
		}
		t.Run(string(tool.Name), func(t *testing.T) {
			fields, ok := f.CompactSpec(tool.Name)
			require.True(t, ok)
			data, err := mcp.CompactJSON([]byte(`[]`), fields)
			require.NoError(t, err)
			require.JSONEq(t, `[]`, string(data))
		})
	}
}

func TestFieldCompactionSpecs_EmptyAssignments(t *testing.T) {
	f := compactor(t)
	for _, name := range []mcp.ToolName{"forgejo_list_issues", "forgejo_get_issue", "forgejo_list_pulls", "forgejo_get_pull"} {
		t.Run(string(name), func(t *testing.T) {
			fields, ok := f.CompactSpec(name)
			require.True(t, ok)
			data, err := mcp.CompactJSON([]byte(`{"id":999,"number":12,"labels":[],"assignees":[],"milestone":null,"user":null}`), fields)
			require.NoError(t, err)
			require.JSONEq(t, `{"id":999,"number":12,"labels":[],"assignees":[]}`, string(data))
		})
	}
}
