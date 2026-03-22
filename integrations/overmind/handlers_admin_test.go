package overmind

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Agents ───────────────────────────────────────────────────────────────────

func TestListAgents(t *testing.T) {
	var gotPath, gotMethod string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		fmt.Fprintf(w, `[{"id":"a1","name":"writer"}]`)
	})

	result, err := o.Execute(context.Background(), "overmind_list_agents", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "writer")
	assert.Equal(t, "/api/agents", gotPath)
	assert.Equal(t, "GET", gotMethod)
}

func TestGetAgent(t *testing.T) {
	var gotPath string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprintf(w, `{"id":"a1","name":"writer"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_get_agent", map[string]any{"id": "a1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/agents/a1", gotPath)
}

func TestCreateAgent(t *testing.T) {
	var gotBody map[string]any
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(201)
		fmt.Fprintf(w, `{"id":"a1","name":"writer"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_create_agent", map[string]any{
		"name":  "writer",
		"model": "claude-sonnet-4-20250514",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "writer", gotBody["name"])
	assert.Equal(t, "claude-sonnet-4-20250514", gotBody["model"])
}

func TestUpdateAgent(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		fmt.Fprintf(w, `{"id":"a1","name":"editor"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_update_agent", map[string]any{
		"id":   "a1",
		"name": "editor",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/agents/a1", gotPath)
	assert.Equal(t, "PUT", gotMethod)
	assert.Equal(t, "editor", gotBody["name"])
}

func TestDeleteAgent(t *testing.T) {
	var gotPath, gotMethod string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		fmt.Fprintf(w, `{"status":"deleted"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_delete_agent", map[string]any{"id": "a1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/agents/a1", gotPath)
	assert.Equal(t, "DELETE", gotMethod)
}

// ── Flows ────────────────────────────────────────────────────────────────────

func TestListFlows(t *testing.T) {
	var gotPath string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprintf(w, `[{"id":"f1","name":"deploy"}]`)
	})

	result, err := o.Execute(context.Background(), "overmind_list_flows", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/flows", gotPath)
}

func TestGetFlow(t *testing.T) {
	var gotURI string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.RequestURI
		fmt.Fprintf(w, `{"id":"f1","name":"deploy"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_get_flow", map[string]any{
		"id":     "f1",
		"expand": "agents",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, gotURI, "/api/flows/f1")
	assert.Contains(t, gotURI, "expand=agents")
}

func TestCreateFlow(t *testing.T) {
	var gotBody map[string]any
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(201)
		fmt.Fprintf(w, `{"id":"f1","name":"deploy"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_create_flow", map[string]any{
		"name":            "deploy",
		"timeout_minutes": 30,
		"enabled":         true,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "deploy", gotBody["name"])
	assert.Equal(t, float64(30), gotBody["timeout_minutes"])
	assert.Equal(t, true, gotBody["enabled"])
}

func TestDeleteFlow(t *testing.T) {
	var gotPath, gotMethod string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		fmt.Fprintf(w, `{"status":"deleted"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_delete_flow", map[string]any{"id": "f1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/flows/f1", gotPath)
	assert.Equal(t, "DELETE", gotMethod)
}

func TestCloneFlow(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(201)
		fmt.Fprintf(w, `{"id":"f2","name":"deploy-copy"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_clone_flow", map[string]any{
		"id":   "f1",
		"name": "deploy-copy",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/flows/f1/clone", gotPath)
	assert.Equal(t, "deploy-copy", gotBody["name"])
}

func TestRunFlow(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(202)
		fmt.Fprintf(w, `{"flow_run_id":"fr1","initial_agent_run_id":"ar1"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_run_flow", map[string]any{
		"id":     "f1",
		"prompt": "Deploy staging",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/flows/f1/run", gotPath)
	assert.Equal(t, "Deploy staging", gotBody["prompt"])
}

func TestValidateFlow(t *testing.T) {
	var gotPath string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprintf(w, `{"valid":true,"errors":[]}`)
	})

	result, err := o.Execute(context.Background(), "overmind_validate_flow", map[string]any{
		"name": "test-flow",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/flows/validate", gotPath)
}

// ── Flow runs ────────────────────────────────────────────────────────────────

func TestListFlowRuns(t *testing.T) {
	var gotURI string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.RequestURI
		fmt.Fprintf(w, `[{"id":"fr1","state":"completed"}]`)
	})

	result, err := o.Execute(context.Background(), "overmind_list_flow_runs", map[string]any{
		"flow_id": "f1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, gotURI, "flow_id=f1")
}

func TestGetFlowRun(t *testing.T) {
	var gotPath string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprintf(w, `{"id":"fr1","state":"running","agent_runs":[]}`)
	})

	result, err := o.Execute(context.Background(), "overmind_get_flow_run", map[string]any{"id": "fr1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/flow_runs/fr1", gotPath)
}

func TestCancelFlowRun(t *testing.T) {
	var gotPath, gotMethod string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		fmt.Fprintf(w, `{"id":"fr1","state":"cancelled"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_cancel_flow_run", map[string]any{"id": "fr1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/flow_runs/fr1/cancel", gotPath)
	assert.Equal(t, "POST", gotMethod)
}

// ── Agent runs ───────────────────────────────────────────────────────────────

func TestListAgentRuns(t *testing.T) {
	var gotURI string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.RequestURI
		fmt.Fprintf(w, `[{"id":"ar1","state":"running"}]`)
	})

	result, err := o.Execute(context.Background(), "overmind_list_agent_runs", map[string]any{
		"flow_run_id": "fr1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, gotURI, "flow_run_id=fr1")
}

func TestGetAgentRun(t *testing.T) {
	var gotPath string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprintf(w, `{"id":"ar1","state":"completed","session_messages":[]}`)
	})

	result, err := o.Execute(context.Background(), "overmind_get_agent_run", map[string]any{"id": "ar1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/agent_runs/ar1", gotPath)
}

// ── MCP identities ──────────────────────────────────────────────────────────

func TestListMCPIdentities(t *testing.T) {
	var gotPath string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprintf(w, `[{"id":"id1","name":"github-prod"}]`)
	})

	result, err := o.Execute(context.Background(), "overmind_list_mcp_identities", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/mcp_identities", gotPath)
}

func TestGetMCPIdentity(t *testing.T) {
	var gotPath string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprintf(w, `{"id":"id1","name":"github-prod"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_get_mcp_identity", map[string]any{"id": "id1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/mcp_identities/id1", gotPath)
}

func TestCreateMCPIdentity(t *testing.T) {
	var gotBody map[string]any
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(201)
		fmt.Fprintf(w, `{"id":"id1","name":"github-prod"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_create_mcp_identity", map[string]any{
		"name":             "github-prod",
		"integration_name": "github",
		"credentials":      `{"token":"ghp_xxx"}`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "github-prod", gotBody["name"])
	assert.Equal(t, "github", gotBody["integration_name"])
}

func TestCreateMCPIdentity_InvalidJSON(t *testing.T) {
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})

	result, err := o.Execute(context.Background(), "overmind_create_mcp_identity", map[string]any{
		"name":             "test",
		"integration_name": "github",
		"credentials":      "not-json",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "credentials must be valid JSON")
}

func TestDeleteMCPIdentity(t *testing.T) {
	var gotPath, gotMethod string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		fmt.Fprintf(w, `{"status":"deleted"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_delete_mcp_identity", map[string]any{"id": "id1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/mcp_identities/id1", gotPath)
	assert.Equal(t, "DELETE", gotMethod)
}

// ── MCP roles ────────────────────────────────────────────────────────────────

func TestListMCPRoles(t *testing.T) {
	var gotPath string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprintf(w, `[{"id":"r1","name":"developer"}]`)
	})

	result, err := o.Execute(context.Background(), "overmind_list_mcp_roles", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/mcp_roles", gotPath)
}

func TestGetMCPRole(t *testing.T) {
	var gotPath string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprintf(w, `{"id":"r1","name":"developer","entries":[]}`)
	})

	result, err := o.Execute(context.Background(), "overmind_get_mcp_role", map[string]any{"id": "r1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/mcp_roles/r1", gotPath)
}

func TestCreateMCPRole(t *testing.T) {
	var gotBody map[string]any
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(201)
		fmt.Fprintf(w, `{"id":"r1","name":"developer"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_create_mcp_role", map[string]any{
		"name":        "developer",
		"description": "Full access to GitHub and Slack",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "developer", gotBody["name"])
	assert.Equal(t, "Full access to GitHub and Slack", gotBody["description"])
}

func TestDeleteMCPRole(t *testing.T) {
	var gotPath, gotMethod string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		fmt.Fprintf(w, `{"status":"deleted"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_delete_mcp_role", map[string]any{"id": "r1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/mcp_roles/r1", gotPath)
	assert.Equal(t, "DELETE", gotMethod)
}

func TestCreateMCPRoleEntry(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(201)
		fmt.Fprintf(w, `{"id":"e1","mcp_role_id":"r1","mcp_identity_id":"id1"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_create_mcp_role_entry", map[string]any{
		"role_id":         "r1",
		"mcp_identity_id": "id1",
		"tool_globs":      []any{"github_*"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/mcp_roles/r1/entries", gotPath)
	assert.Equal(t, "id1", gotBody["mcp_identity_id"])
}

func TestUpdateMCPRoleEntry(t *testing.T) {
	var gotPath, gotMethod string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		fmt.Fprintf(w, `{"id":"e1"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_update_mcp_role_entry", map[string]any{
		"role_id":  "r1",
		"entry_id": "e1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/mcp_roles/r1/entries/e1", gotPath)
	assert.Equal(t, "PUT", gotMethod)
}

func TestDeleteMCPRoleEntry(t *testing.T) {
	var gotPath, gotMethod string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		fmt.Fprintf(w, `{"status":"deleted"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_delete_mcp_role_entry", map[string]any{
		"role_id":  "r1",
		"entry_id": "e1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/mcp_roles/r1/entries/e1", gotPath)
	assert.Equal(t, "DELETE", gotMethod)
}

// ── Pipelines ────────────────────────────────────────────────────────────────

func TestListPipelines(t *testing.T) {
	var gotURI string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.RequestURI
		fmt.Fprintf(w, `[{"id":"p1","name":"build"}]`)
	})

	result, err := o.Execute(context.Background(), "overmind_list_pipelines", map[string]any{
		"global_context_id": "gc1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, gotURI, "global_context_id=gc1")
}

func TestGetPipeline(t *testing.T) {
	var gotPath string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprintf(w, `{"id":"p1","name":"build"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_get_pipeline", map[string]any{"id": "p1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/pipelines/p1", gotPath)
}

func TestCreatePipeline(t *testing.T) {
	var gotBody map[string]any
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(201)
		fmt.Fprintf(w, `{"id":"p1","name":"build"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_create_pipeline", map[string]any{
		"name":              "build",
		"global_context_id": "gc1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "build", gotBody["name"])
	assert.Equal(t, "gc1", gotBody["global_context_id"])
}

func TestDeletePipeline(t *testing.T) {
	var gotPath, gotMethod string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		fmt.Fprintf(w, `{"status":"deleted"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_delete_pipeline", map[string]any{"id": "p1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/pipelines/p1", gotPath)
	assert.Equal(t, "DELETE", gotMethod)
}

// ── Tasks ────────────────────────────────────────────────────────────────────

func TestListTasks(t *testing.T) {
	var gotURI string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.RequestURI
		fmt.Fprintf(w, `[{"id":"t1","name":"compile"}]`)
	})

	result, err := o.Execute(context.Background(), "overmind_list_tasks", map[string]any{
		"pipeline_id": "p1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, gotURI, "pipeline_id=p1")
}

func TestGetTask(t *testing.T) {
	var gotPath string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprintf(w, `{"id":"t1","name":"compile"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_get_task", map[string]any{"id": "t1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/tasks/t1", gotPath)
}

func TestCreateTask(t *testing.T) {
	var gotBody map[string]any
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(201)
		fmt.Fprintf(w, `{"id":"t1","name":"compile"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_create_task", map[string]any{
		"name":        "compile",
		"pipeline_id": "p1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "compile", gotBody["name"])
	assert.Equal(t, "p1", gotBody["pipeline_id"])
}

func TestDeleteTask(t *testing.T) {
	var gotPath, gotMethod string
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		fmt.Fprintf(w, `{"status":"deleted"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_delete_task", map[string]any{"id": "t1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "/api/tasks/t1", gotPath)
	assert.Equal(t, "DELETE", gotMethod)
}

// ── Empty required args for admin tools ──────────────────────────────────────

func TestAdminEmptyRequiredArgs(t *testing.T) {
	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	tests := []struct {
		name    string
		tool    string
		args    map[string]any
		wantErr string
	}{
		{"get_agent empty id", "overmind_get_agent", map[string]any{"id": ""}, "id is required"},
		{"delete_agent empty id", "overmind_delete_agent", map[string]any{"id": ""}, "id is required"},
		{"create_agent empty name", "overmind_create_agent", map[string]any{"name": ""}, "name is required"},
		{"get_flow empty id", "overmind_get_flow", map[string]any{"id": ""}, "id is required"},
		{"delete_flow empty id", "overmind_delete_flow", map[string]any{"id": ""}, "id is required"},
		{"create_flow empty name", "overmind_create_flow", map[string]any{"name": ""}, "name is required"},
		{"clone_flow empty id", "overmind_clone_flow", map[string]any{"id": ""}, "id is required"},
		{"run_flow empty id", "overmind_run_flow", map[string]any{"id": ""}, "id is required"},
		{"list_flow_runs empty flow_id", "overmind_list_flow_runs", map[string]any{"flow_id": ""}, "flow_id is required"},
		{"get_flow_run empty id", "overmind_get_flow_run", map[string]any{"id": ""}, "id is required"},
		{"cancel_flow_run empty id", "overmind_cancel_flow_run", map[string]any{"id": ""}, "id is required"},
		{"list_agent_runs empty flow_run_id", "overmind_list_agent_runs", map[string]any{"flow_run_id": ""}, "flow_run_id is required"},
		{"get_agent_run empty id", "overmind_get_agent_run", map[string]any{"id": ""}, "id is required"},
		{"get_mcp_identity empty id", "overmind_get_mcp_identity", map[string]any{"id": ""}, "id is required"},
		{"delete_mcp_identity empty id", "overmind_delete_mcp_identity", map[string]any{"id": ""}, "id is required"},
		{"create_mcp_identity empty name", "overmind_create_mcp_identity", map[string]any{"name": "", "integration_name": "x", "credentials": "{}"}, "name is required"},
		{"get_mcp_role empty id", "overmind_get_mcp_role", map[string]any{"id": ""}, "id is required"},
		{"delete_mcp_role empty id", "overmind_delete_mcp_role", map[string]any{"id": ""}, "id is required"},
		{"create_mcp_role empty name", "overmind_create_mcp_role", map[string]any{"name": ""}, "name is required"},
		{"create_role_entry empty role_id", "overmind_create_mcp_role_entry", map[string]any{"role_id": "", "mcp_identity_id": "x"}, "role_id is required"},
		{"create_role_entry empty identity", "overmind_create_mcp_role_entry", map[string]any{"role_id": "r1", "mcp_identity_id": ""}, "mcp_identity_id is required"},
		{"delete_role_entry empty role_id", "overmind_delete_mcp_role_entry", map[string]any{"role_id": "", "entry_id": "e1"}, "role_id is required"},
		{"delete_role_entry empty entry_id", "overmind_delete_mcp_role_entry", map[string]any{"role_id": "r1", "entry_id": ""}, "entry_id is required"},
		{"get_pipeline empty id", "overmind_get_pipeline", map[string]any{"id": ""}, "id is required"},
		{"delete_pipeline empty id", "overmind_delete_pipeline", map[string]any{"id": ""}, "id is required"},
		{"create_pipeline empty name", "overmind_create_pipeline", map[string]any{"name": "", "global_context_id": "gc1"}, "name is required"},
		{"create_pipeline empty gc_id", "overmind_create_pipeline", map[string]any{"name": "x", "global_context_id": ""}, "global_context_id is required"},
		{"list_tasks empty pipeline_id", "overmind_list_tasks", map[string]any{"pipeline_id": ""}, "pipeline_id is required"},
		{"get_task empty id", "overmind_get_task", map[string]any{"id": ""}, "id is required"},
		{"delete_task empty id", "overmind_delete_task", map[string]any{"id": ""}, "id is required"},
		{"create_task empty name", "overmind_create_task", map[string]any{"name": "", "pipeline_id": "p1"}, "name is required"},
		{"create_task empty pipeline_id", "overmind_create_task", map[string]any{"name": "x", "pipeline_id": ""}, "pipeline_id is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := o.Execute(context.Background(), tt.tool, tt.args)
			require.NoError(t, err)
			assert.True(t, result.IsError, "expected error for %s", tt.name)
			assert.Contains(t, result.Data, tt.wantErr)
		})
	}
}
