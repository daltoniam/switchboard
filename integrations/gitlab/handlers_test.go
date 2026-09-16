package gitlab

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

func TestHandlers_HTTPRouting(t *testing.T) {
	cases := []struct {
		tool     mcp.ToolName
		args     map[string]any
		method   string
		path     string
		body     string
		response string
	}{
		{"gitlab_list_projects", nil, "GET", "/api/v4/projects", "", `[{"id":1,"name":"demo"}]`},
		{"gitlab_get_project", map[string]any{"project_id": "42"}, "GET", "/api/v4/projects/42", "", `{"id":1}`},
		{"gitlab_list_merge_requests", map[string]any{"project_id": "1"}, "GET", "/api/v4/projects/1/merge_requests", "", `[{"iid":2}]`},
		{"gitlab_get_merge_request", map[string]any{"project_id": "1", "merge_request_iid": 2}, "GET", "/api/v4/projects/1/merge_requests/2", "", `{"iid":2}`},
		{"gitlab_create_merge_request_note", map[string]any{"project_id": "1", "merge_request_iid": 2, "body": "hi"}, "POST", "/api/v4/projects/1/merge_requests/2/notes", `{"body":"hi"}`, `{"id":9}`},
		{"gitlab_list_issues", map[string]any{"project_id": "1"}, "GET", "/api/v4/projects/1/issues", "", `[{"iid":3}]`},
		{"gitlab_list_pipelines", map[string]any{"project_id": "1", "status": "failed"}, "GET", "/api/v4/projects/1/pipelines", "", `[{"id":10}]`},
		{"gitlab_list_pipeline_jobs", map[string]any{"project_id": "1", "pipeline_id": 10}, "GET", "/api/v4/projects/1/pipelines/10/jobs", "", `[{"id":88,"status":"failed"}]`},
		{"gitlab_get_job_trace", map[string]any{"project_id": "1", "job_id": 88}, "GET", "/api/v4/projects/1/jobs/88/trace", "", "ERROR: job failed"},
	}
	for _, tc := range cases {
		t.Run(string(tc.tool), func(t *testing.T) {
			g, s := configured(t, func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, tc.method, r.Method)
				require.Equal(t, tc.path, r.URL.EscapedPath())
				if tc.body != "" {
					data, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					require.JSONEq(t, tc.body, string(data))
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tc.response))
			})
			_ = s
			res, err := g.Execute(context.Background(), tc.tool, tc.args)
			require.NoError(t, err)
			require.False(t, res.IsError, res.Data)
			if tc.tool == "gitlab_get_job_trace" {
				require.Contains(t, res.Data, "ERROR")
			}
		})
	}
}

func TestCreateNote_MutationErrorSurface(t *testing.T) {
	g, _ := configured(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"403 Forbidden"}`))
	})
	res, err := g.Execute(context.Background(), "gitlab_create_merge_request_note", map[string]any{
		"project_id": "1", "merge_request_iid": 1, "body": "test",
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, strings.ToLower(res.Data), "403")
}

func TestHandlers_ProjectPathEncoding(t *testing.T) {
	g, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v4/projects/group%2Frepo", r.URL.EscapedPath())
		fmt.Fprint(w, `{"id":1}`)
	})
	res, err := g.Execute(context.Background(), "gitlab_get_project", map[string]any{"project_id": "group/repo"})
	require.NoError(t, err)
	require.False(t, res.IsError, res.Data)
}

func TestListProjects_QueryMembership(t *testing.T) {
	var q string
	g, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.RawQuery
		fmt.Fprint(w, `[]`)
	})
	_, err := g.Execute(context.Background(), "gitlab_list_projects", nil)
	require.NoError(t, err)
	require.Contains(t, q, "membership=true")
}

func TestListProjects_MembershipStringFalse(t *testing.T) {
	var q string
	g, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.RawQuery
		fmt.Fprint(w, `[]`)
	})
	_, err := g.Execute(context.Background(), "gitlab_list_projects", map[string]any{"membership": "false"})
	require.NoError(t, err)
	require.NotContains(t, q, "membership=")
}
