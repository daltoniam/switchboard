package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/render"
	"github.com/stretchr/testify/require"
)

// Render is hosted SaaS with no container image, so this spec reaches the
// hosted Render MCP server (https://mcp.render.com/mcp) with stored sandbox
// credentials and runs read tools only. Render API keys cannot be scoped to
// read-only, so use a dedicated sandbox account or workspace:
//
//	RENDER_LIVE_API_KEY       API key of the sandbox account
//	RENDER_LIVE_WORKSPACE_ID  sandbox workspace (tea-... or usr-...)
//	RENDER_LIVE_SERVICE_ID    Git-backed web service that deployed and logged in the last 30 days
//	RENDER_LIVE_POSTGRES_ID   Postgres instance in that workspace
//	RENDER_LIVE_KEY_VALUE_ID  Key Value instance in that workspace
func init() {
	writes := "changes the shared Render sandbox; sandbox specs run read tools only and request shapes are covered by fixture tests in integrations/render"
	creates := "creates billable Render resources; sandbox specs run read tools only"
	workflows := "Render Workflows are a separate feature with no workflow deployed in the sandbox workspace"
	registerLiveSpec(liveSpec{
		Integration: "render",
		Tier:        liveSandbox,
		Adapter:     render.New(),
		Provision:   liveRenderProvision,
		Cases: []liveCase{
			{Tool: "render_list_workspaces", Expect: []string{"${workspace_id}"}},
			{Tool: "render_list_services", Expect: []string{"${service_id}", "serviceDetails"}},
			{Tool: "render_get_service", Args: map[string]any{"serviceId": "${service_id}"}, Expect: []string{"${service_id}", "dashboardUrl"}},
			{Tool: "render_list_deploys", Args: map[string]any{"serviceId": "${service_id}", "limit": 2}, Expect: []string{"${deploy_id}", "next_cursor"}},
			{Tool: "render_get_deploy", Args: map[string]any{"serviceId": "${service_id}", "deployId": "${deploy_id}"}, Expect: []string{"${deploy_id}", "status"}},
			{Tool: "render_list_events", Args: map[string]any{"serviceId": "${service_id}", "startTime": "${since}", "limit": 5}, Expect: []string{"items", "timestamp"}},
			{Tool: "render_list_logs", Args: map[string]any{"resource": []any{"${service_id}"}, "startTime": "${since}", "limit": 5}, Expect: []string{"hasMore", "message"}},
			{Tool: "render_list_log_label_values", Args: map[string]any{"label": "type", "resource": []any{"${service_id}"}, "startTime": "${since}"}, Expect: []string{"build"}},
			{Tool: "render_get_metrics", Args: map[string]any{"resourceId": "${service_id}", "metricTypes": []any{"cpu_usage"}}, Expect: []string{"cpu_usage", "${service_id}"}},
			{Tool: "render_list_postgres_instances", Expect: []string{"${postgres_id}"}},
			{Tool: "render_get_postgres", Args: map[string]any{"postgresId": "${postgres_id}"}, Expect: []string{"${postgres_id}", "databaseName"}},
			{Tool: "render_query_postgres", Args: map[string]any{"postgresId": "${postgres_id}", "sql": "select 1 as switchboard_live"}, Expect: []string{"switchboard_live"}},
			{Tool: "render_list_key_value", Expect: []string{"${key_value_id}"}},
			{Tool: "render_get_key_value", Args: map[string]any{"keyValueId": "${key_value_id}"}, Expect: []string{"${key_value_id}", "options"}},
		},
		Skips: map[mcp.ToolName]string{
			"render_get_blueprint":                "Blueprints are early access upstream and the sandbox workspace has none",
			"render_create_web_service":           creates,
			"render_create_static_site":           creates,
			"render_create_cron_job":              creates,
			"render_create_postgres":              creates,
			"render_create_key_value":             creates,
			"render_create_blueprint":             creates,
			"render_create_workflow":              creates,
			"render_update_environment_variables": writes,
			"render_trigger_deploy":               writes,
			"render_run_cron_job":                 writes,
			"render_cancel_cron_job_run":          writes,
			"render_plan_blueprint":               writes,
			"render_approve_blueprint":            writes,
			"render_update_blueprint":             writes,
			"render_create_workflow_version":      writes,
			"render_start_flow_run":               writes,
			"render_cancel_flow_run":              writes,
			"render_list_workflows":               workflows,
			"render_get_workflow":                 workflows,
			"render_list_workflow_versions":       workflows,
			"render_get_workflow_version":         workflows,
			"render_list_workflow_tasks":          workflows,
			"render_get_workflow_task":            workflows,
			"render_list_workflow_runs":           workflows,
			"render_get_workflow_run":             workflows,
			"render_get_workflow_run_results":     workflows,
			"render_list_flows":                   workflows,
			"render_get_flow":                     workflows,
			"render_list_flow_runs":               workflows,
			"render_get_flow_run":                 workflows,
			"render_get_flow_run_results":         workflows,
			"render_list_flow_run_steps":          workflows,
			"render_get_flow_run_step":            workflows,
		},
	})
}

func liveRenderProvision(t *testing.T) liveProvision {
	key := os.Getenv("RENDER_LIVE_API_KEY")
	workspace := os.Getenv("RENDER_LIVE_WORKSPACE_ID")
	service := os.Getenv("RENDER_LIVE_SERVICE_ID")
	postgres := os.Getenv("RENDER_LIVE_POSTGRES_ID")
	keyValue := os.Getenv("RENDER_LIVE_KEY_VALUE_ID")
	if key == "" || workspace == "" || service == "" || postgres == "" || keyValue == "" {
		t.Skip("set RENDER_LIVE_API_KEY, RENDER_LIVE_WORKSPACE_ID, RENDER_LIVE_SERVICE_ID, RENDER_LIVE_POSTGRES_ID, and RENDER_LIVE_KEY_VALUE_ID to reach a real Render sandbox; Render has no container image")
	}
	return liveProvision{
		Credentials: map[string]string{"api_key": key, "workspace_id": workspace},
		Values: map[string]string{
			"workspace_id": workspace,
			"service_id":   service,
			"postgres_id":  postgres,
			"key_value_id": keyValue,
			"deploy_id":    liveRenderLatestDeploy(t, key, service),
			"since":        time.Now().UTC().Add(-29 * 24 * time.Hour).Format(time.RFC3339),
		},
	}
}

// liveRenderLatestDeploy reads the newest deploy ID from the Render REST API,
// so get_deploy can assert a real ID without another stored variable.
func liveRenderLatestDeploy(t *testing.T, key, service string) string {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.render.com/v1/services/"+url.PathEscape(service)+"/deploys?limit=1", nil)
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set("Accept", "application/json")
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode, "list deploys for the sandbox service")
	var deploys []struct {
		Deploy struct {
			ID string `json:"id"`
		} `json:"deploy"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&deploys))
	require.NotEmpty(t, deploys, "the sandbox service needs at least one deploy")
	return deploys[0].Deploy.ID
}
