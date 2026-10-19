package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// uiDbtMCP searches for dbt tools on the running binary and, when they are
// discoverable, executes a read-only run listing against the fixture.
func uiDbtMCP(t *testing.T, app *uiApp) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "ui-acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"query": "dbt run error", "integration": "dbt", "limit": 50}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	names := uiToolNames(t, text.Text)
	if len(names) == 0 {
		return nil
	}
	require.Contains(t, names, "dbt_list_jobs_runs")
	require.Contains(t, names, "dbt_get_job_run_error")

	result, err = session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{
		"tool_name": "dbt_list_jobs_runs",
		"arguments": map[string]any{"status": "error", "limit": 1},
	}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.NotEmpty(t, result.Content)
	text, ok = result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	require.Contains(t, text.Text, "uifixturesha")
	return names
}

// uiToolNames reads tool names from a search response in either the
// row-object or the columnar (columns + rows) shape.
func uiToolNames(t *testing.T, body string) []string {
	t.Helper()
	var response struct {
		Tools json.RawMessage `json:"tools"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &response))
	if len(response.Tools) == 0 || string(response.Tools) == "null" {
		return nil
	}
	var rows []struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(response.Tools, &rows) == nil {
		var names []string
		for _, row := range rows {
			names = append(names, row.Name)
		}
		return names
	}
	var columnar struct {
		Columns []string `json:"columns"`
		Rows    [][]any  `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(response.Tools, &columnar))
	nameIndex := -1
	for i, column := range columnar.Columns {
		if column == "name" {
			nameIndex = i
		}
	}
	require.GreaterOrEqual(t, nameIndex, 0, "search response has no name column")
	var names []string
	for _, row := range columnar.Rows {
		if name, ok := row[nameIndex].(string); ok {
			names = append(names, name)
		}
	}
	return names
}

func TestUIDbt(t *testing.T) {
	app := newUIApp(t)
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Token ui-dbt-token" {
			http.Error(writer, `{"status":{"code":401,"user_message":"Invalid token"}}`, http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/v2/accounts/42/":
			_, _ = io.WriteString(writer, `{"status":{"code":200},"data":{"id":42,"name":"UI fixture account"}}`)
		case "/api/v2/accounts/42/runs/":
			if request.URL.Query().Get("status") != "20" {
				http.Error(writer, "expected status=20", http.StatusBadRequest)
				return
			}
			_, _ = io.WriteString(writer, `{"status":{"code":200},"data":[{"id":100,"job_definition_id":7,"status":20,"status_humanized":"Error","git_sha":"uifixturesha","trigger":{"cause":"Scheduled"}}],"extra":{"pagination":{"count":1,"total_count":1}}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)

	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/dbt"]`)
	uiText(t, card, "Disabled")
	require.Empty(t, uiDbtMCP(t, app))
	require.NoError(t, card.Click())

	form := page.Locator("#integration-configuration")
	fields := []struct {
		key         string
		inputType   string
		label       string
		placeholder string
	}{
		{key: "token", inputType: "password", label: "token", placeholder: "Job Viewer"},
		{key: "account_id", inputType: "text", label: "account_id", placeholder: "account ID"},
		{key: "base_url", inputType: "text", label: "base_url (optional)", placeholder: "https://cloud.getdbt.com"},
		{key: "prod_job_id", inputType: "text", label: "prod_job_id (optional)", placeholder: "manifest.json"},
	}
	for _, field := range fields {
		input := form.Locator("#cred_" + field.key)
		inputType, err := input.GetAttribute("type")
		require.NoError(t, err)
		require.Equal(t, field.inputType, inputType, field.key)
		value, err := input.InputValue()
		require.NoError(t, err)
		require.Empty(t, value, "fresh install must not pre-seed %s", field.key)
		placeholder, err := input.GetAttribute("placeholder")
		require.NoError(t, err)
		require.Contains(t, placeholder, field.placeholder, field.key)
		label, err := form.Locator(`label[for="cred_` + field.key + `"]`).TextContent()
		require.NoError(t, err)
		require.Equal(t, field.label, label)
	}
	uiText(t, page.Locator("body"), "Available Tools (6)")

	token := form.Locator("#cred_token")
	require.NoError(t, form.Locator("#cred_account_id").Fill("42"))
	require.NoError(t, form.Locator("#cred_base_url").Fill(provider.URL))
	require.NoError(t, form.Locator("#cred_prod_job_id").Fill("7"))
	require.NoError(t, token.Fill("wrong-token"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")

	require.NoError(t, token.Fill("ui-dbt-token"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	names := uiDbtMCP(t, app)
	require.Len(t, names, 6)
	uiCapture(t, page, "dbt-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	uiCapture(t, page, "dbt-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))

	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/dbt")
	uiText(t, page.Locator(".badge").First(), "Connected")
	value, err := token.InputValue()
	require.NoError(t, err)
	require.Equal(t, "ui-dbt-token", value)
	require.NotEmpty(t, uiDbtMCP(t, app))

	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiDbtMCP(t, app))
	uiNavigate(t, page, app.URL+"/integrations")
	uiText(t, card, "Disabled")
	uiCapture(t, page, "dbt-disabled")

	data, err := os.ReadFile(filepath.Join(app.home, ".config", "switchboard", "config.json"))
	require.NoError(t, err)
	var saved mcp.Config
	require.NoError(t, json.Unmarshal(data, &saved))
	require.False(t, saved.Integrations["dbt"].Enabled)
	require.Equal(t, "42", saved.Integrations["dbt"].Credentials["account_id"])
	require.Equal(t, provider.URL, saved.Integrations["dbt"].Credentials["base_url"])
	require.Equal(t, "7", saved.Integrations["dbt"].Credentials["prod_job_id"])
}
