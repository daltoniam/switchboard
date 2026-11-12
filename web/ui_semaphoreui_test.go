package web

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func uiSemaphoreUIMCP(t *testing.T, app *uiApp) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "semaphoreui-ui-acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"query": "Semaphore UI automation task template project status", "integration": "semaphoreui", "limit": 10}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	names := uiToolNames(t, text.Text)
	if len(names) == 0 {
		return nil
	}
	require.ElementsMatch(t, []string{"semaphoreui_get_project", "semaphoreui_list_templates", "semaphoreui_get_template", "semaphoreui_list_tasks", "semaphoreui_get_task"}, names)
	for _, name := range names {
		args := map[string]any{}
		if name == "semaphoreui_get_template" {
			args["template_id"] = 3
		}
		if name == "semaphoreui_get_task" {
			args["task_id"] = 9
		}
		result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": name, "arguments": args}})
		require.NoError(t, err)
		require.False(t, result.IsError)
		require.NotEmpty(t, result.Content)
		text, ok := result.Content[0].(*mcpsdk.TextContent)
		require.True(t, ok)
		require.NotContains(t, text.Text, "SECRET_SENTINEL")
	}
	return names
}

func TestUISemaphoreUI(t *testing.T) {
	app := newUIApp(t)
	task := `{"id":9,"project_id":7,"template_id":3,"status":"error","created":"2026-11-01T10:00:00Z","start":"2026-11-01T10:00:01Z","end":"2026-11-01T10:00:02Z","environment":"SECRET_SENTINEL"}`
	template := `{"id":3,"project_id":7,"name":"Safe Template","app":"bash","type":"build","last_task":{"id":9,"status":"error"},"playbook":"SECRET_SENTINEL"}`
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ui-semaphore-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/prefix/api/user":
			_, _ = io.WriteString(w, `{"admin":false}`)
		case "/prefix/api/info":
			_, _ = io.WriteString(w, `{"version":"v2.19.12"}`)
		case "/prefix/api/project/7/role":
			_, _ = io.WriteString(w, `{"role":"guest","permissions":0}`)
		case "/prefix/api/project/7":
			_, _ = io.WriteString(w, `{"id":7,"name":"Approved Project","secret":"SECRET_SENTINEL"}`)
		case "/prefix/api/project/7/templates":
			_, _ = io.WriteString(w, `[`+template+`]`)
		case "/prefix/api/project/7/templates/3":
			_, _ = io.WriteString(w, template)
		case "/prefix/api/project/7/tasks/last":
			w.Header().Set("X-Has-Next", "false")
			_, _ = io.WriteString(w, `[`+task+`]`)
		case "/prefix/api/project/7/tasks/9":
			_, _ = io.WriteString(w, task)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(provider.Close)
	cert := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw}))
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/semaphoreui"]`)
	uiText(t, card, "Disabled")
	require.Empty(t, uiSemaphoreUIMCP(t, app))
	require.NoError(t, card.Click())
	form := page.Locator("#integration-configuration")
	for _, field := range []struct{ key, inputType, placeholder string }{
		{"base_url", "text", "HTTPS Semaphore UI"}, {"api_token", "password", "non-admin guest"}, {"project_id", "text", "permissions=0"}, {"ca_cert", "text", "PEM CA certificate"},
	} {
		input := form.Locator("#cred_" + field.key)
		inputType, err := input.GetAttribute("type")
		require.NoError(t, err)
		require.Equal(t, field.inputType, inputType)
		value, err := input.InputValue()
		require.NoError(t, err)
		require.Empty(t, value)
		placeholder, err := input.GetAttribute("placeholder")
		require.NoError(t, err)
		require.Contains(t, placeholder, field.placeholder)
	}
	uiText(t, form.Locator(`label[for="cred_ca_cert"]`), "optional")
	uiText(t, page.Locator("body"), "Available Tools (5)")
	require.NoError(t, form.Locator("#cred_base_url").Fill(provider.URL+"/prefix"))
	require.NoError(t, form.Locator("#cred_project_id").Fill("7"))
	require.NoError(t, form.Locator("#cred_ca_cert").Fill(strings.ReplaceAll(cert, "\n", `\n`)))
	require.NoError(t, form.Locator("#cred_api_token").Fill("invalid-token"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_api_token").Fill("ui-semaphore-token"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiSemaphoreUIMCP(t, app), 5)
	uiCapture(t, page, "semaphoreui-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	uiCapture(t, page, "semaphoreui-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))
	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/semaphoreui")
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiSemaphoreUIMCP(t, app), 5)
	require.NoError(t, form.Locator("#cred_api_token").Fill(""))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_api_token").Fill("ui-semaphore-token"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiSemaphoreUIMCP(t, app))
	uiNavigate(t, page, app.URL+"/integrations")
	uiText(t, card, "Disabled")
	data, err := os.ReadFile(filepath.Join(app.home, ".config", "switchboard", "config.json"))
	require.NoError(t, err)
	var saved mcp.Config
	require.NoError(t, json.Unmarshal(data, &saved))
	require.False(t, saved.Integrations["semaphoreui"].Enabled)
	require.Equal(t, "7", saved.Integrations["semaphoreui"].Credentials["project_id"])
}
