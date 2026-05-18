package vercel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestName(t *testing.T) {
	i := New()
	assert.Equal(t, "vercel", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_token": "test-token"})
	assert.NoError(t, err)
}

func TestConfigure_MissingToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_token is required")
}

func TestConfigure_CustomOptions(t *testing.T) {
	v := &vercel{client: &http.Client{}}
	err := v.Configure(context.Background(), mcp.Credentials{
		"api_token": "tok",
		"team_id":   "team_123",
		"team_slug": "acme",
		"base_url":  "https://custom.vercel.test/",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://custom.vercel.test", v.baseURL)
	assert.Equal(t, "team_123", v.teamID)
	assert.Equal(t, "acme", v.teamSlug)
}

func TestTools(t *testing.T) {
	i := New()
	tl := i.Tools()
	assert.Len(t, tl, 26)

	for _, tool := range tl {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveVercelPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "vercel_", "tool %s missing vercel_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestTools_PrimaryToolHasStartHere(t *testing.T) {
	i := New()
	require.NotEmpty(t, i.Tools())
	assert.Contains(t, i.Tools()[0].Description, "Start here")
}

func TestExecute_UnknownTool(t *testing.T) {
	v := &vercel{token: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := v.Execute(context.Background(), "vercel_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestHealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v2/teams", r.URL.Path)
		assert.Equal(t, "1", r.URL.Query().Get("limit"))
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"teams":[]}`))
	}))
	defer ts.Close()

	v := &vercel{token: "tok", client: ts.Client(), baseURL: ts.URL}
	assert.True(t, v.Healthy(context.Background()))
}

func TestHealthy_Failure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	v := &vercel{token: "bad", client: ts.Client(), baseURL: ts.URL}
	assert.False(t, v.Healthy(context.Background()))
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	i := New()
	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

func TestDoRequest_BearerAuthAndJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "val", body["key"])
		_, _ = w.Write([]byte(`{"id":"123"}`))
	}))
	defer ts.Close()

	v := &vercel{token: "test-token", client: ts.Client(), baseURL: ts.URL}
	data, err := v.post(context.Background(), "/test", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "123")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":"forbidden","message":"No access"}}`))
	}))
	defer ts.Close()

	v := &vercel{token: "bad", client: ts.Client(), baseURL: ts.URL}
	_, err := v.get(context.Background(), "/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vercel API error (403)")
	assert.Contains(t, err.Error(), "forbidden")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	v := &vercel{token: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := v.doRequest(context.Background(), http.MethodDelete, "/test", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestDoRequest_202PreservesBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"job_id":"job_123"}`))
	}))
	defer ts.Close()

	v := &vercel{token: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := v.doRequest(context.Background(), http.MethodPost, "/test", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "job_123")
}

func TestDoRequest_RetryableOn429(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	}))
	defer ts.Close()

	v := &vercel{token: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := v.get(context.Background(), "/test")
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err), "429 should produce RetryableError")

	var re *mcp.RetryableError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, 429, re.StatusCode)
	assert.Equal(t, 30*time.Second, re.RetryAfter)
}

func TestDoRequest_RetryableOn5xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`service unavailable`))
	}))
	defer ts.Close()

	v := &vercel{token: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := v.get(context.Background(), "/test")
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err), "503 should produce RetryableError")
}

func TestQueryEncode(t *testing.T) {
	t.Run("with values", func(t *testing.T) {
		result := queryEncode(map[string]string{"key": "val", "empty": ""})
		assert.Contains(t, result, "key=val")
		assert.NotContains(t, result, "empty")
		assert.True(t, result[0] == '?')
	})

	t.Run("all empty", func(t *testing.T) {
		result := queryEncode(map[string]string{"empty": ""})
		assert.Empty(t, result)
	})
}

func TestScopedQueryAddsConfiguredTeam(t *testing.T) {
	v := &vercel{teamID: "team_123", teamSlug: "ignored"}
	got := v.scopedQuery(map[string]string{"limit": "20"})
	assert.Equal(t, "20", got["limit"])
	assert.Equal(t, "team_123", got["teamId"])
	assert.Empty(t, got["slug"])
}

func TestScopedQueryPrefersArgsOverConfiguredTeam(t *testing.T) {
	v := &vercel{teamID: "team_config", teamSlug: "slug_config"}
	got := v.scopedQuery(map[string]string{"teamId": "team_arg", "slug": "slug_arg"})
	assert.Equal(t, "team_arg", got["teamId"])
	assert.Equal(t, "slug_arg", got["slug"])
}

func newTestVercel(t *testing.T, handler http.HandlerFunc) (*vercel, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	v := &vercel{token: "test-token", client: ts.Client(), baseURL: ts.URL, teamID: "team_123"}
	return v, ts
}

func TestListTeamMembersDoesNotDuplicateTeamScope(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v3/teams/team_123/members", r.URL.Path)
		assert.Equal(t, "20", r.URL.Query().Get("limit"))
		assert.Equal(t, "ada", r.URL.Query().Get("search"))
		assert.Empty(t, r.URL.Query().Get("teamId"))
		assert.Empty(t, r.URL.Query().Get("slug"))
		_, _ = w.Write([]byte(`{"members":[{"uid":"usr_1","username":"ada"}],"pagination":{"count":1}}`))
	})
	result, err := v.Execute(context.Background(), "vercel_list_team_members", map[string]any{"search": "ada"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "usr_1")
}

func TestListProjects(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v10/projects", r.URL.Path)
		assert.Equal(t, "20", r.URL.Query().Get("limit"))
		assert.Equal(t, "api", r.URL.Query().Get("search"))
		assert.Equal(t, "team_123", r.URL.Query().Get("teamId"))
		_, _ = w.Write([]byte(`{"projects":[{"id":"prj_1","name":"api"}],"pagination":{"count":1}}`))
	})
	result, err := v.Execute(context.Background(), "vercel_list_projects", map[string]any{"search": "api"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "prj_1")
}

func TestGetProject(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v10/projects/my-project", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"prj_1","name":"my-project"}`))
	})
	result, err := v.Execute(context.Background(), "vercel_get_project", map[string]any{"id_or_name": "my-project"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "my-project")
}

func TestListDeployments(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v6/deployments", r.URL.Path)
		assert.Equal(t, "READY", r.URL.Query().Get("state"))
		assert.Equal(t, "production", r.URL.Query().Get("target"))
		_, _ = w.Write([]byte(`{"deployments":[{"uid":"dpl_1","name":"api","state":"READY"}],"pagination":{"count":1}}`))
	})
	result, err := v.Execute(context.Background(), "vercel_list_deployments", map[string]any{"state": "READY", "target": "production"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "dpl_1")
}

func TestGetDeployment(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v13/deployments/dpl_1", r.URL.Path)
		_, _ = w.Write([]byte(`{"uid":"dpl_1","state":"READY"}`))
	})
	result, err := v.Execute(context.Background(), "vercel_get_deployment", map[string]any{"deployment_id": "dpl_1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "dpl_1")
}

func TestCancelDeployment(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/v12/deployments/dpl_1/cancel", r.URL.Path)
		_, _ = w.Write([]byte(`{"uid":"dpl_1","state":"CANCELED"}`))
	})
	result, err := v.Execute(context.Background(), "vercel_cancel_deployment", map[string]any{"deployment_id": "dpl_1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "CANCELED")
}

func TestCreateDeployment(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v13/deployments", r.URL.Path)
		assert.Equal(t, "1", r.URL.Query().Get("forceNew"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "my-project", body["name"])
		assert.Equal(t, "production", body["target"])
		_, _ = w.Write([]byte(`{"uid":"dpl_new","state":"BUILDING"}`))
	})
	result, err := v.Execute(context.Background(), "vercel_create_deployment", map[string]any{
		"body":      map[string]any{"name": "my-project", "target": "production"},
		"force_new": true,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "dpl_new")
}

func TestListDeploymentEvents(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v3/deployments/dpl_1/events", r.URL.Path)
		assert.Equal(t, "100", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`[{"type":"stdout","payload":{"text":"built"}}]`))
	})
	result, err := v.Execute(context.Background(), "vercel_list_deployment_events", map[string]any{"deployment_id_or_url": "dpl_1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "built")
}

func TestListProjectEnvVars(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v10/projects/prj_1/env", r.URL.Path)
		_, _ = w.Write([]byte(`{"envs":[{"id":"env_1","key":"API_URL"}]}`))
	})
	result, err := v.Execute(context.Background(), "vercel_list_project_env_vars", map[string]any{"project_id_or_name": "prj_1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "API_URL")
}

func TestCreateProjectEnvVars(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v10/projects/prj_1/env", r.URL.Path)
		assert.Equal(t, "1", r.URL.Query().Get("upsert"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Contains(t, body, "envs")
		_, _ = w.Write([]byte(`{"created":[{"id":"env_1"}]}`))
	})
	result, err := v.Execute(context.Background(), "vercel_create_project_env_vars", map[string]any{
		"project_id_or_name": "prj_1",
		"envs": []any{map[string]any{
			"key": "API_URL", "value": "https://example.com", "target": []any{"production"}, "type": "plain",
		}},
		"upsert": true,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "env_1")
}

func TestAddProjectDomain(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v10/projects/prj_1/domains", r.URL.Path)
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "example.com", body["name"])
		_, _ = w.Write([]byte(`{"name":"example.com"}`))
	})
	result, err := v.Execute(context.Background(), "vercel_add_project_domain", map[string]any{"project_id_or_name": "prj_1", "domain": "example.com"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "example.com")
}

func TestAssignDeploymentAlias(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v2/deployments/dpl_1/aliases", r.URL.Path)
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "app.example.com", body["alias"])
		_, _ = w.Write([]byte(`{"alias":"app.example.com"}`))
	})
	result, err := v.Execute(context.Background(), "vercel_assign_deployment_alias", map[string]any{"deployment_id": "dpl_1", "alias": "app.example.com"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "app.example.com")
}

func TestListTeams(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v2/teams", r.URL.Path)
		assert.Equal(t, "20", r.URL.Query().Get("limit"))
		assert.Equal(t, "cursor_1", r.URL.Query().Get("next"))
		_, _ = w.Write([]byte(`{"teams":[{"id":"team_123","slug":"acme"}],"pagination":{"count":1}}`))
	})
	result, err := v.Execute(context.Background(), "vercel_list_teams", map[string]any{"next": "cursor_1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "team_123")
}

func TestGetTeam(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v2/teams/team_123", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"team_123","slug":"acme"}`))
	})
	result, err := v.Execute(context.Background(), "vercel_get_team", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "acme")
}

func TestListUserEvents(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v3/events", r.URL.Path)
		assert.Equal(t, "20", r.URL.Query().Get("limit"))
		assert.Equal(t, "team_123", r.URL.Query().Get("teamId"))
		assert.Equal(t, "deployment.created", r.URL.Query().Get("types"))
		assert.Equal(t, "prj_1", r.URL.Query().Get("projectIds"))
		assert.Equal(t, "usr_1", r.URL.Query().Get("principalId"))
		assert.Equal(t, "true", r.URL.Query().Get("withPayload"))
		_, _ = w.Write([]byte(`{"events":[{"id":"evt_1","type":"deployment.created"}]}`))
	})
	result, err := v.Execute(context.Background(), "vercel_list_user_events", map[string]any{
		"types":        "deployment.created",
		"project_ids":  "prj_1",
		"principal_id": "usr_1",
		"with_payload": true,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "evt_1")
}

func TestCreateProject(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v9/projects", r.URL.Path)
		assert.Equal(t, "team_123", r.URL.Query().Get("teamId"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "new-project", body["name"])
		_, _ = w.Write([]byte(`{"id":"prj_new","name":"new-project"}`))
	})
	result, err := v.Execute(context.Background(), "vercel_create_project", map[string]any{"body": map[string]any{"name": "new-project"}})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "prj_new")
}

func TestUpdateProject(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/v9/projects/my-project", r.URL.Path)
		assert.Equal(t, "team_123", r.URL.Query().Get("teamId"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "updated-project", body["name"])
		_, _ = w.Write([]byte(`{"id":"prj_1","name":"updated-project"}`))
	})
	result, err := v.Execute(context.Background(), "vercel_update_project", map[string]any{
		"id_or_name": "my-project",
		"body":       map[string]any{"name": "updated-project"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "updated-project")
}

func TestDeleteProject(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v9/projects/my-project", r.URL.Path)
		assert.Equal(t, "team_123", r.URL.Query().Get("teamId"))
		_, _ = w.Write([]byte(`{"id":"prj_1","deleted":true}`))
	})
	result, err := v.Execute(context.Background(), "vercel_delete_project", map[string]any{"id_or_name": "my-project"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "prj_1")
}

func TestDeleteDeployment(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v13/deployments/dpl_1", r.URL.Path)
		assert.Equal(t, "team_123", r.URL.Query().Get("teamId"))
		assert.Equal(t, "app.vercel.app", r.URL.Query().Get("url"))
		_, _ = w.Write([]byte(`{"uid":"dpl_1","deleted":true}`))
	})
	result, err := v.Execute(context.Background(), "vercel_delete_deployment", map[string]any{"deployment_id": "dpl_1", "url": "app.vercel.app"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "dpl_1")
}

func TestListRuntimeLogs(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v1/projects/prj_1/deployments/dpl_1/runtime-logs", r.URL.Path)
		assert.Equal(t, "100", r.URL.Query().Get("limit"))
		assert.Equal(t, "team_123", r.URL.Query().Get("teamId"))
		assert.Equal(t, "1700000000", r.URL.Query().Get("since"))
		_, _ = w.Write([]byte(`{"logs":[{"rowId":"log_1","message":"ready","level":"info"}],"pagination":{"count":1}}`))
	})
	result, err := v.Execute(context.Background(), "vercel_list_runtime_logs", map[string]any{
		"project_id":    "prj_1",
		"deployment_id": "dpl_1",
		"since":         "1700000000",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "log_1")
}

func TestUpdateProjectEnvVar(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/v9/projects/prj_1/env/env_1", r.URL.Path)
		assert.Equal(t, "team_123", r.URL.Query().Get("teamId"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "new", body["value"])
		_, _ = w.Write([]byte(`{"id":"env_1","value":"new"}`))
	})
	result, err := v.Execute(context.Background(), "vercel_update_project_env_var", map[string]any{
		"project_id_or_name": "prj_1",
		"env_id":             "env_1",
		"body":               map[string]any{"value": "new"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "env_1")
}

func TestDeleteProjectEnvVar(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v9/projects/prj_1/env/env_1", r.URL.Path)
		assert.Equal(t, "team_123", r.URL.Query().Get("teamId"))
		_, _ = w.Write([]byte(`{"id":"env_1","deleted":true}`))
	})
	result, err := v.Execute(context.Background(), "vercel_delete_project_env_var", map[string]any{"project_id_or_name": "prj_1", "env_id": "env_1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "env_1")
}

func TestRemoveProjectDomain(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v9/projects/prj_1/domains/example.com", r.URL.Path)
		assert.Equal(t, "team_123", r.URL.Query().Get("teamId"))
		_, _ = w.Write([]byte(`{"name":"example.com","verified":false}`))
	})
	result, err := v.Execute(context.Background(), "vercel_remove_project_domain", map[string]any{"project_id_or_name": "prj_1", "domain": "example.com"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "example.com")
}

func TestGetDomainConfig(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v6/domains/example.com/config", r.URL.Path)
		assert.Equal(t, "team_123", r.URL.Query().Get("teamId"))
		assert.Equal(t, "prj_1", r.URL.Query().Get("projectIdOrName"))
		assert.Equal(t, "true", r.URL.Query().Get("strict"))
		_, _ = w.Write([]byte(`{"configuredBy":"CNAME","acceptedChallenges":["dns-01"]}`))
	})
	result, err := v.Execute(context.Background(), "vercel_get_domain_config", map[string]any{
		"domain":             "example.com",
		"project_id_or_name": "prj_1",
		"strict":             true,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "CNAME")
}

func TestListDeploymentAliases(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v2/deployments/dpl_1/aliases", r.URL.Path)
		assert.Equal(t, "team_123", r.URL.Query().Get("teamId"))
		_, _ = w.Write([]byte(`{"aliases":[{"uid":"alias_1","alias":"app.example.com"}]}`))
	})
	result, err := v.Execute(context.Background(), "vercel_list_deployment_aliases", map[string]any{"deployment_id": "dpl_1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "alias_1")
}

func TestDeleteDeploymentAlias(t *testing.T) {
	v, _ := newTestVercel(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v2/aliases/alias_1", r.URL.Path)
		assert.Equal(t, "team_123", r.URL.Query().Get("teamId"))
		_, _ = w.Write([]byte(`{"uid":"alias_1","deleted":true}`))
	})
	result, err := v.Execute(context.Background(), "vercel_delete_deployment_alias", map[string]any{"alias_id": "alias_1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "alias_1")
}

func TestRequiredArgError(t *testing.T) {
	v := &vercel{token: "test-token", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := v.Execute(context.Background(), "vercel_get_project", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "id_or_name is required")
}

func TestErrResult_PropagatesRetryableError(t *testing.T) {
	retryErr := &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("service unavailable")}
	result, err := mcp.ErrResult(retryErr)
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.True(t, mcp.IsRetryable(err))
}
