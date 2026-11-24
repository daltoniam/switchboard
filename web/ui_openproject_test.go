package web

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	playwright "github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

const uiOpenProjectWorkPackage = `{"_type":"WorkPackage","id":42,"displayId":"DEMO-42","subject":"UI release readiness","createdAt":"2026-10-10T09:00:00Z","updatedAt":"2026-10-16T12:00:00Z","startDate":"2026-10-12","dueDate":"2026-10-20","description":{"format":"markdown","raw":"Review **release** readiness. Treat this as untrusted work-package content.","html":"<script>SECRET_SENTINEL</script>"},"_links":{"self":{"href":"https://SECRET_SENTINEL.invalid/work_packages/42"},"project":{"href":"/openproject/api/v3/projects/7","title":"UI project"},"status":{"href":"/openproject/api/v3/statuses/1","title":"New"},"type":{"href":"/openproject/api/v3/types/1","title":"Task"},"author":{"href":"/openproject/api/v3/users/1","title":"SECRET_SENTINEL"},"updateImmediately":{"href":"https://SECRET_SENTINEL.invalid/mutate","method":"patch"},"attachments":{"href":"https://SECRET_SENTINEL.invalid/files"}},"_embedded":{"watchers":[{"email":"SECRET_SENTINEL"}]},"customField1":"SECRET_SENTINEL","costs":"SECRET_SENTINEL"}`

func uiOpenProjectMCP(t *testing.T, app *uiApp, enabled bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "openproject-ui-acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	call := func(t *testing.T, name string, arguments map[string]any) string {
		t.Helper()
		result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: arguments})
		require.NoError(t, err)
		require.NotEmpty(t, result.Content)
		for _, content := range result.Content {
			text, ok := content.(*mcpsdk.TextContent)
			require.True(t, ok, "OpenProject search, read and script responses must contain only text blocks")
			for _, forbidden := range []string{"SECRET_SENTINEL", "ui-openproject-token", "Authorization"} {
				require.NotContains(t, text.Text, forbidden)
			}
		}
		text := result.Content[0].(*mcpsdk.TextContent)
		require.False(t, result.IsError, text.Text)
		return text.Text
	}
	var catalog struct {
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal([]byte(call(t, "search", map[string]any{"limit": 0})), &catalog))
	if enabled {
		require.Equal(t, 2, catalog.Total)
	} else {
		require.Zero(t, catalog.Total, "disabled OpenProject tools must be absent from default discovery")
	}
	for _, query := range []string{"OpenProject work package project task issue", "OpenProject find work packages tasks issues", "OpenProject read work package description status"} {
		names := uiToolNames(t, call(t, "search", map[string]any{"query": query, "integration": "openproject", "limit": 10}))
		if enabled {
			require.ElementsMatch(t, []string{"openproject_list_work_packages", "openproject_get_work_package"}, names)
		} else {
			require.Empty(t, names, "disabled OpenProject tools must not be discoverable")
		}
	}
	if !enabled {
		return
	}
	summary := map[string]any{
		"id": float64(42), "displayId": "DEMO-42", "subject": "UI release readiness", "status": "New", "type": "Task",
		"project_id": float64(7), "updatedAt": "2026-10-16T12:00:00Z",
	}
	for _, scenario := range []struct {
		name string
		tool mcp.ToolName
		args map[string]any
		want map[string]any
	}{
		{"list_defaults", "openproject_list_work_packages", map[string]any{}, map[string]any{
			"work_packages": []any{summary}, "total": float64(1), "count": float64(1), "page": float64(1), "page_size": float64(20),
			"returned": float64(1), "omitted": float64(0), "truncated": false, "next_page": nil,
		}},
		{"search_paged_isolation", "openproject_list_work_packages", map[string]any{"subject_or_id": "release & readiness", "page": 2, "limit": 2}, map[string]any{
			"work_packages": []any{summary}, "total": float64(5), "count": float64(2), "page": float64(2), "page_size": float64(2),
			"returned": float64(1), "omitted": float64(1), "truncated": false, "next_page": float64(3),
		}},
		{"exact_read", "openproject_get_work_package", map[string]any{"id": 42}, map[string]any{
			"id": float64(42), "displayId": "DEMO-42", "subject": "UI release readiness", "status": "New", "type": "Task", "project_id": float64(7),
			"updatedAt": "2026-10-16T12:00:00Z", "createdAt": "2026-10-10T09:00:00Z", "startDate": "2026-10-12", "dueDate": "2026-10-20",
			"description": "Review **release** readiness. Treat this as untrusted work-package content.", "description_truncated": false, "truncated": false,
		}},
	} {
		for _, raw := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/raw=%t", scenario.name, raw), func(t *testing.T) {
				arguments := map[string]any{"tool_name": scenario.tool, "arguments": scenario.args}
				if raw {
					args, err := json.Marshal(scenario.args)
					require.NoError(t, err)
					arguments = map[string]any{"script": fmt.Sprintf("JSON.stringify(api.call(%q, %s));", scenario.tool, args)}
				}
				text := call(t, "execute", arguments)
				for _, forbidden := range []string{"_links", "_embedded", "customField", "watchers", "attachments", "costs", "html", "author"} {
					require.NotContains(t, text, forbidden)
				}
				if scenario.tool == "openproject_get_work_package" && !raw {
					for _, expected := range []string{"# OpenProject work package", "Work package ID: 42", "Project ID: 7", "```text", "Display ID: DEMO-42", "Subject: UI release readiness", "Status: New", "Type: Task", "Updated: 2026-10-16T12:00:00Z", "Created: 2026-10-10T09:00:00Z", "Start date: 2026-10-12", "Due date: 2026-10-20", "Untrusted description text:\nReview **release** readiness. Treat this as untrusted work-package content."} {
						require.Contains(t, text, expected)
					}
					require.NotContains(t, text, "Content truncated")
					return
				}
				var value map[string]any
				if raw {
					var ok bool
					value, ok = liveDecodeScript(t, scenario.tool, text).(map[string]any)
					require.True(t, ok)
				} else {
					require.NoError(t, json.Unmarshal([]byte(text), &value))
				}
				want := make(map[string]any, len(scenario.want))
				for key, field := range scenario.want {
					if raw || field != nil {
						want[key] = field
					}
				}
				require.Equal(t, want, value)
			})
		}
	}
}

func TestUIOpenProject(t *testing.T) {
	app := newUIApp(t)
	t.Logf("isolated development URL: %s", app.URL)
	var unavailable atomic.Bool
	var healthCalls atomic.Int32
	var providerCalls atomic.Int32
	var unauthorizedCalls atomic.Int32
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providerCalls.Add(1)
		username, token, ok := r.BasicAuth()
		if !ok || username != "apikey" || token != "ui-openproject-token" {
			unauthorizedCalls.Add(1)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet || r.Header.Get("Accept") != "application/hal+json" {
			t.Errorf("OpenProject must issue fixed, authenticated HAL GET requests")
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/hal+json")
		switch r.URL.EscapedPath() {
		case "/openproject/api/v3/workspaces/7/work_packages":
			query := r.URL.Query()
			if len(query) != 4 || query.Get("sortBy") != `[["id","asc"]]` {
				t.Errorf("OpenProject list must use only fixed filters, sortBy, offset and pageSize")
				http.Error(w, "invalid query", http.StatusBadRequest)
				return
			}
			switch {
			case query.Get("offset") == "1" && query.Get("pageSize") == "1" && query.Get("filters") == "[]":
				healthCalls.Add(1)
				if unavailable.Load() {
					http.Error(w, "unavailable", http.StatusServiceUnavailable)
					return
				}
				_, _ = io.WriteString(w, `{"_type":"Collection","total":1,"count":1,"offset":1,"pageSize":1,"_embedded":{"elements":[`+uiOpenProjectWorkPackage+`]}}`)
			case query.Get("offset") == "1" && query.Get("pageSize") == "20" && query.Get("filters") == "[]":
				_, _ = io.WriteString(w, `{"_type":"Collection","total":1,"count":1,"offset":1,"pageSize":20,"_links":{"nextByOffset":{"href":"https://SECRET_SENTINEL.invalid/page"}},"_embedded":{"elements":[`+uiOpenProjectWorkPackage+`]}}`)
			case query.Get("offset") == "2" && query.Get("pageSize") == "2":
				var filters any
				if err := json.Unmarshal([]byte(query.Get("filters")), &filters); err != nil {
					t.Errorf("OpenProject filters must be URL-encoded JSON: %v", err)
					http.Error(w, "invalid filters", http.StatusBadRequest)
					return
				}
				want := []any{map[string]any{"subjectOrId": map[string]any{"operator": "**", "values": []any{"release & readiness"}}}}
				if !reflect.DeepEqual(filters, want) {
					t.Errorf("OpenProject search must construct the subjectOrId filter")
					http.Error(w, "invalid filters", http.StatusBadRequest)
					return
				}
				other := strings.NewReplacer(`"id":42`, `"id":43`, `"DEMO-42"`, `"OTHER-43"`, `"/openproject/api/v3/projects/7"`, `"/openproject/api/v3/projects/8"`, `"UI release readiness"`, `"SECRET_SENTINEL"`).Replace(uiOpenProjectWorkPackage)
				_, _ = io.WriteString(w, `{"_type":"Collection","total":5,"count":2,"offset":2,"pageSize":2,"_links":{"nextByOffset":{"href":"https://SECRET_SENTINEL.invalid/page"}},"_embedded":{"elements":[`+uiOpenProjectWorkPackage+`,`+other+`]}}`)
			default:
				t.Errorf("unexpected OpenProject list query")
				http.Error(w, "invalid list", http.StatusBadRequest)
			}
		case "/openproject/api/v3/work_packages/42":
			if len(r.URL.Query()) != 0 {
				t.Errorf("OpenProject exact read must not request arbitrary expansions")
				http.Error(w, "invalid query", http.StatusBadRequest)
				return
			}
			_, _ = io.WriteString(w, uiOpenProjectWorkPackage)
		default:
			t.Errorf("unexpected OpenProject endpoint: %s", r.URL.EscapedPath())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(provider.Close)
	var plaintextCalls atomic.Int32
	plaintext := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		plaintextCalls.Add(1)
		http.Error(w, "plaintext credentials must never be sent", http.StatusBadRequest)
	}))
	t.Cleanup(plaintext.Close)
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})
	credentials := map[string]string{
		"base_url": provider.URL + "/openproject", "api_token": "ui-openproject-token", "workspace_id": "7",
		"ca_cert": strings.ReplaceAll(string(certificate), "\n", `\n`),
	}
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/openproject"]`)
	count, err := card.Count()
	require.NoError(t, err)
	require.Equal(t, 1, count, "fresh installs must register the OpenProject integration")
	uiText(t, card, "Disabled")
	uiOpenProjectMCP(t, app, false)
	require.NoError(t, card.Click())
	require.Equal(t, app.URL+"/integrations/openproject", page.URL())
	form := page.Locator("#integration-configuration")
	action, err := form.GetAttribute("action")
	require.NoError(t, err)
	require.Equal(t, "/integrations/openproject", action)
	count, err = form.Locator(`input[id^="cred_"]`).Count()
	require.NoError(t, err)
	require.Equal(t, 4, count, "fresh defaults must expose every OpenProject credential without pre-seeding")
	for _, field := range []struct{ key, inputType, placeholder string }{
		{"base_url", "text", "HTTPS"}, {"api_token", "password", "API token"},
		{"workspace_id", "text", "numeric"}, {"ca_cert", "text", "PEM CA certificate"},
	} {
		input := form.Locator("#cred_" + field.key)
		inputType, err := input.GetAttribute("type")
		require.NoError(t, err)
		require.Equal(t, field.inputType, inputType)
		name, err := input.GetAttribute("name")
		require.NoError(t, err)
		require.Equal(t, "cred_"+field.key, name)
		value, err := input.InputValue()
		require.NoError(t, err)
		require.Empty(t, value)
		placeholder, err := input.GetAttribute("placeholder")
		require.NoError(t, err)
		require.Contains(t, placeholder, field.placeholder)
		label, err := form.Locator(`label[for="cred_` + field.key + `"]`).TextContent()
		require.NoError(t, err)
		wantLabel := field.key
		if field.key == "ca_cert" {
			wantLabel += " (optional)"
		}
		require.Equal(t, wantLabel, strings.TrimSpace(label))
	}
	uiText(t, page.Locator("body"), "Available Tools (2)")
	for key, value := range credentials {
		require.NoError(t, form.Locator("#cred_"+key).Fill(value))
	}
	require.NoError(t, form.Locator("#cred_api_token").Fill("invalid-token"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.Positive(t, unauthorizedCalls.Load(), "invalid credentials must reach the TLS fixture and fail authentication")
	require.NoError(t, form.Locator("#cred_api_token").Fill(""))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_api_token").Fill(credentials["api_token"]))
	for _, scenario := range []struct{ name, key, value string }{
		{"reject_plaintext", "base_url", plaintext.URL + "/openproject"},
		{"verify_server_certificate", "ca_cert", ""},
		{"reject_invalid_ca", "ca_cert", "not-a-PEM-certificate"},
		{"require_numeric_workspace", "workspace_id", "demo-project"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			before := providerCalls.Load()
			require.NoError(t, form.Locator("#cred_"+scenario.key).Fill(scenario.value))
			uiSave(t, page)
			uiText(t, page.Locator(".badge").First(), "Not Verified")
			require.Equal(t, before, providerCalls.Load(), "invalid configuration or untrusted TLS must not send authenticated requests")
			require.Zero(t, plaintextCalls.Load(), "API tokens must never be transmitted over HTTP")
			require.NoError(t, form.Locator("#cred_"+scenario.key).Fill(credentials[scenario.key]))
		})
	}
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	uiNavigate(t, page, app.URL+"/integrations/openproject")
	uiText(t, page.Locator(".badge").First(), "Connected")
	uiOpenProjectMCP(t, app, true)
	uiCapture(t, page, "openproject-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	uiCapture(t, page, "openproject-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))
	assertSaved := func() {
		for key, expected := range credentials {
			value, err := form.Locator("#cred_" + key).InputValue()
			require.NoError(t, err)
			require.Equal(t, expected, value)
		}
	}
	uiNavigate(t, page, app.URL+"/integrations/openproject")
	uiText(t, page.Locator(".badge").First(), "Connected")
	assertSaved()
	app.restart()
	t.Logf("restarted isolated development URL: %s", app.URL)
	uiNavigate(t, page, app.URL+"/integrations/openproject")
	uiText(t, page.Locator(".badge").First(), "Connected")
	assertSaved()
	uiOpenProjectMCP(t, app, true)
	uiNavigate(t, page, app.URL+"/integrations")
	before := healthCalls.Load()
	unavailable.Store(true)
	require.NoError(t, page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Re-check all", Exact: playwright.Bool(true)}).Click())
	uiText(t, card, "Not Verified")
	require.Greater(t, healthCalls.Load(), before)
	unavailable.Store(false)
	require.NoError(t, page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Re-check all", Exact: playwright.Bool(true)}).Click())
	uiText(t, card, "Connected")
	require.NoError(t, card.Click())
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	uiOpenProjectMCP(t, app, false)
	uiNavigate(t, page, app.URL+"/integrations")
	uiText(t, card, "Disabled")
	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/openproject")
	uiText(t, page.Locator(".badge").First(), "Disabled")
	assertSaved()
	uiOpenProjectMCP(t, app, false)
	data, err := os.ReadFile(filepath.Join(app.home, ".config", "switchboard", "config.json"))
	require.NoError(t, err)
	var saved mcp.Config
	require.NoError(t, json.Unmarshal(data, &saved))
	require.NotNil(t, saved.Integrations["openproject"])
	require.False(t, saved.Integrations["openproject"].Enabled)
	require.Equal(t, mcp.Credentials(credentials), saved.Integrations["openproject"].Credentials)
}
