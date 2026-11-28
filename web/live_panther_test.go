package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/panther"
	"github.com/stretchr/testify/require"
)

// Panther is hosted SaaS with no container image. The live spec drives API
// token mode, because the harness configures integrations by filling
// credential fields and Panther Remote MCP accepts only per-user browser
// OAuth. Run it with PANTHER_LIVE_INSTANCE_URL and PANTHER_LIVE_API_TOKEN set
// to a dedicated token with only the read permissions the setup page
// documents. Provision reads (never writes) the instance to learn record ids,
// so the cases assert real data.
func init() {
	adapter := panther.New()
	since := time.Now().UTC().AddDate(0, 0, -90).Format(time.RFC3339)
	cases := []liveCase{
		{Tool: "panther_get_permissions", Args: map[string]any{}, Expect: []string{"${token_name}", "AlertRead"}},
		{Tool: "panther_list_alerts", Args: map[string]any{"created_after": since}, Expect: []string{"${alert_id}"}},
		{Tool: "panther_get_alert", Args: map[string]any{"alert_id": "${alert_id}"}, Expect: []string{"${alert_id}"}},
		{Tool: "panther_list_alert_events", Args: map[string]any{"alert_id": "${alert_id}", "limit": 5}, Expect: []string{"p_event_time"}},
		{Tool: "panther_list_rules", Args: map[string]any{}, Expect: []string{"${rule_id}"}},
		{Tool: "panther_get_rule", Args: map[string]any{"rule_id": "${rule_id}"}, Expect: []string{"${rule_id}"}},
		{Tool: "panther_list_databases", Args: map[string]any{}, Expect: []string{"panther_logs"}},
		{Tool: "panther_list_database_tables", Args: map[string]any{"database_name": "panther_logs"}, Expect: []string{"${table_name}"}},
		{Tool: "panther_get_table_schema", Args: map[string]any{"database_name": "panther_logs", "table_name": "${table_name}"}, Expect: []string{"p_event_time"}},
		{Tool: "panther_query_data_lake", Args: map[string]any{"sql": "SELECT 1 AS switchboard_live_check", "timeout_seconds": 120}, Expect: []string{"switchboard_live_check"}},
		{Tool: "panther_list_data_lake_queries", Args: map[string]any{"contains": "switchboard_live_check"}, Expect: []string{"switchboard_live_check"}},
		{Tool: "panther_list_log_sources", Args: map[string]any{}, Expect: []string{"${source_id}"}},
		{Tool: "panther_get_log_source", Args: map[string]any{"source_id": "${source_id}"}, Expect: []string{"${source_id}"}},
		{Tool: "panther_list_schemas", Args: map[string]any{"contains": "${schema_name}"}, Expect: []string{"${schema_name}"}},
		{Tool: "panther_get_schema", Args: map[string]any{"name": "${schema_name}"}, Expect: []string{"${schema_name}"}},
		{Tool: "panther_list_users", Args: map[string]any{}, Expect: []string{"${user_id}"}},
		{Tool: "panther_get_user", Args: map[string]any{"user_id": "${user_id}"}, Expect: []string{"${user_id}"}},
		{Tool: "panther_list_roles", Args: map[string]any{}, Expect: []string{"${role_id}"}},
		{Tool: "panther_get_role", Args: map[string]any{"role_id": "${role_id}"}, Expect: []string{"${role_id}"}},
		{Tool: "panther_get_alert_metrics", Args: map[string]any{"from": since}, Expect: []string{"totalAlerts"}},
		{Tool: "panther_get_data_metrics", Args: map[string]any{"from": since}, Expect: []string{"totalEventsProcessed"}},
	}
	covered := map[mcp.ToolName]bool{}
	for _, c := range cases {
		covered[c.Tool] = true
	}
	const (
		writeReason = "write tool; sandbox specs run read tools only"
		dataReason  = "depends on records (comments, scheduled, simple, and correlation rules, policies, data models, global helpers, saved queries) that a given Panther instance may not have, so the live compaction check would flag missing data rather than code; unit fixtures cover the request and response shape"
		queryReason = "needs the id of a query started earlier in the run; panther_query_data_lake already polls the same dataLakeQuery operation to completion"
	)
	skips := map[mcp.ToolName]string{}
	for _, tool := range adapter.Tools() {
		switch {
		case covered[tool.Name]:
		case tool.Access != mcp.AccessRead:
			skips[tool.Name] = writeReason
		case tool.Name == "panther_get_data_lake_query":
			skips[tool.Name] = queryReason
		default:
			skips[tool.Name] = dataReason
		}
	}
	registerLiveSpec(liveSpec{
		Integration: "panther",
		Tier:        liveSandbox,
		Adapter:     adapter,
		Provision:   provisionPantherLive,
		Cases:       cases,
		Skips:       skips,
	})
}

func provisionPantherLive(t *testing.T) liveProvision {
	instance := strings.TrimSpace(os.Getenv("PANTHER_LIVE_INSTANCE_URL"))
	token := os.Getenv("PANTHER_LIVE_API_TOKEN")
	if instance == "" || token == "" {
		t.Skip("PANTHER_LIVE_INSTANCE_URL and PANTHER_LIVE_API_TOKEN are not set: create a Panther API token under Settings > Developer Tools > API Tokens with Read Alerts, View Rules, View Policies, Query Data Lake, View Log Sources, Read User Info, and View Overview, and export it with the instance URL to run the Panther live read cases")
	}
	apiBase, err := panther.APIBase(instance)
	require.NoError(t, err)
	live := pantherLiveClient{t: t, token: token, apiBase: apiBase}

	var self struct {
		Name string `json:"name"`
	}
	live.rest("/api-tokens/self", nil, &self)
	require.NotEmpty(t, self.Name, "the token lookup returned no name")
	since := time.Now().UTC().AddDate(0, 0, -90).Format(time.RFC3339)
	first := func(path string, query url.Values, field string) string {
		var page struct {
			Results []map[string]any `json:"results"`
		}
		live.rest(path, query, &page)
		require.NotEmpty(t, page.Results, "the Panther instance needs at least one record from %s for the live cases", path)
		value, _ := page.Results[0][field].(string)
		require.NotEmpty(t, value, "%s: first record has no %s", path, field)
		return value
	}
	var tables struct {
		Tables struct {
			Edges []struct {
				Node struct {
					Name string `json:"name"`
				} `json:"node"`
			} `json:"edges"`
		} `json:"dataLakeDatabaseTables"`
	}
	live.graphql(`query { dataLakeDatabaseTables(input: { databaseName: "panther_logs", pageSize: 1 }) { edges { node { name } } } }`, &tables)
	require.NotEmpty(t, tables.Tables.Edges, "panther_logs has no tables")
	var sources struct {
		Sources struct {
			Edges []struct {
				Node struct {
					ID string `json:"integrationId"`
				} `json:"node"`
			} `json:"edges"`
		} `json:"sources"`
	}
	live.graphql(`query { sources { edges { node { integrationId } } } }`, &sources)
	require.NotEmpty(t, sources.Sources.Edges, "the instance has no log sources")
	var schemas struct {
		Schemas struct {
			Edges []struct {
				Node struct {
					Name string `json:"name"`
				} `json:"node"`
			} `json:"edges"`
		} `json:"schemas"`
	}
	live.graphql(`query { schemas(input: { isInUse: true }) { edges { node { name } } } }`, &schemas)
	require.NotEmpty(t, schemas.Schemas.Edges, "the instance has no schema in use")
	return liveProvision{
		Credentials: map[string]string{"instance_url": instance, "api_token": token},
		Values: map[string]string{
			"token_name":  self.Name,
			"alert_id":    first("/alerts", url.Values{"type": {"ALERT"}, "created-after": {since}, "limit": {"1"}}, "id"),
			"rule_id":     first("/rules", url.Values{"limit": {"1"}}, "id"),
			"user_id":     first("/users", url.Values{"limit": {"1"}}, "id"),
			"role_id":     first("/roles", nil, "id"),
			"table_name":  tables.Tables.Edges[0].Node.Name,
			"source_id":   sources.Sources.Edges[0].Node.ID,
			"schema_name": schemas.Schemas.Edges[0].Node.Name,
		},
	}
}

type pantherLiveClient struct {
	t       *testing.T
	token   string
	apiBase string
}

// do reads one endpoint with the live token. It never prints the response,
// which holds security data.
func (c pantherLiveClient) do(method, target string, body []byte, out any) {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(c.t.Context(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	require.NoError(c.t, err)
	req.Header.Set("X-API-Key", c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	require.NoError(c.t, err)
	require.Equal(c.t, http.StatusOK, resp.StatusCode, "%s returned HTTP %d; check the instance URL and the token's read permissions", target, resp.StatusCode)
	require.NoError(c.t, json.Unmarshal(data, out), "%s did not return JSON", target)
}

func (c pantherLiveClient) rest(path string, query url.Values, out any) {
	c.t.Helper()
	target := c.apiBase + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	c.do(http.MethodGet, target, nil, out)
}

func (c pantherLiveClient) graphql(query string, out any) {
	c.t.Helper()
	body, err := json.Marshal(map[string]any{"query": query})
	require.NoError(c.t, err)
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	c.do(http.MethodPost, c.apiBase+"/public/graphql", body, &envelope)
	require.Empty(c.t, envelope.Errors, "GraphQL errors")
	require.NoError(c.t, json.Unmarshal(envelope.Data, out))
}
