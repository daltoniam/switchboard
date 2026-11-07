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

func uiArgoWorkflowsMCP(t *testing.T, app *uiApp) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "argo-ui-acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"query": "Argo workflow failed status", "integration": "argoworkflows", "limit": 10}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	names := uiToolNames(t, text.Text)
	if len(names) == 0 {
		return nil
	}
	require.ElementsMatch(t, []string{"argoworkflows_list_workflows", "argoworkflows_get_workflow"}, names)
	for _, name := range names {
		args := map[string]any{}
		if name == "argoworkflows_get_workflow" {
			args["name"] = "failed-run"
		}
		result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": name, "arguments": args}})
		require.NoError(t, err)
		require.False(t, result.IsError)
		require.NotEmpty(t, result.Content)
		text, ok := result.Content[0].(*mcpsdk.TextContent)
		require.True(t, ok)
		require.Contains(t, text.Text, "failed-run")
		require.NotContains(t, text.Text, "secret-sentinel")
	}
	return names
}

func TestUIArgoWorkflows(t *testing.T) {
	app := newUIApp(t)
	workflow := `{"metadata":{"name":"failed-run","namespace":"approved"},"spec":{"secret":"secret-sentinel"},"status":{"phase":"Failed","startedAt":"2026-11-01T10:00:00Z","finishedAt":"2026-11-01T10:01:00Z","progress":"0/1","message":"secret-sentinel","nodes":{"failed-run":{"phase":"Failed","message":"secret-sentinel"}}}}`
	provider := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer ui-argo-token" {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/argo/api/v1/workflows/approved":
			_, _ = io.WriteString(writer, `{"metadata":{},"items":[`+workflow+`]}`)
		case "/argo/api/v1/workflows/approved/failed-run":
			_, _ = io.WriteString(writer, workflow)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	cert := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw}))
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/argoworkflows"]`)
	uiText(t, card, "Disabled")
	require.Empty(t, uiArgoWorkflowsMCP(t, app))
	require.NoError(t, card.Click())
	form := page.Locator("#integration-configuration")
	for _, field := range []struct{ key, inputType, placeholder string }{
		{"base_url", "text", "HTTPS Argo Server"},
		{"namespace", "text", "get/list workflows only"},
		{"token", "password", "client auth mode"},
		{"ca_cert", "text", "PEM CA certificate"},
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
	uiText(t, page.Locator("body"), "Available Tools (2)")
	require.NoError(t, form.Locator("#cred_base_url").Fill(provider.URL+"/argo"))
	require.NoError(t, form.Locator("#cred_namespace").Fill("approved"))
	require.NoError(t, form.Locator("#cred_ca_cert").Fill(strings.ReplaceAll(cert, "\n", `\n`)))
	require.NoError(t, form.Locator("#cred_token").Fill("wrong-token"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_token").Fill("ui-argo-token"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiArgoWorkflowsMCP(t, app), 2)
	uiCapture(t, page, "argoworkflows-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	uiCapture(t, page, "argoworkflows-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))
	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/argoworkflows")
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiArgoWorkflowsMCP(t, app), 2)
	require.NoError(t, form.Locator("#cred_token").Fill(""))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_token").Fill("ui-argo-token"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiArgoWorkflowsMCP(t, app))
	uiNavigate(t, page, app.URL+"/integrations")
	uiText(t, card, "Disabled")
	data, err := os.ReadFile(filepath.Join(app.home, ".config", "switchboard", "config.json"))
	require.NoError(t, err)
	var saved mcp.Config
	require.NoError(t, json.Unmarshal(data, &saved))
	require.False(t, saved.Integrations["argoworkflows"].Enabled)
	require.Equal(t, "approved", saved.Integrations["argoworkflows"].Credentials["namespace"])
}
