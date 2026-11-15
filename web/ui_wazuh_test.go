package web

import (
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func uiWazuhMCP(t *testing.T, app *uiApp) []string {
	t.Helper()
	session := liveSession(t, app)
	names := liveSearch(t, session, "wazuh", "Wazuh endpoint agent connection health")
	if len(names) == 0 {
		return nil
	}
	require.ElementsMatch(t, []string{"wazuh_list_agents", "wazuh_get_agent", "wazuh_get_agent_status_summary"}, names)
	text, failed := liveCall(t, session, "execute", map[string]any{"tool_name": "wazuh_list_agents", "arguments": map[string]any{}})
	require.False(t, failed, text)
	var discovered struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"affected_items"`
	}
	require.NoError(t, json.NewDecoder(strings.NewReader(text)).Decode(&discovered))
	require.NotEmpty(t, discovered.Items)
	for _, name := range names {
		args := map[string]any{}
		if name == "wazuh_get_agent" {
			args["agent_id"] = discovered.Items[0].ID
		}
		text, failed := liveCall(t, session, "execute", map[string]any{"tool_name": name, "arguments": args})
		require.False(t, failed, text)
		if name == "wazuh_get_agent_status_summary" {
			require.Contains(t, text, "not_synced")
		} else {
			require.Contains(t, text, "endpoint-one")
		}
		require.NotContains(t, text, "SECRET_SENTINEL")
		t.Logf("MCP smoke tool=%s bytes=%d passed=true", name, len(text))
	}
	for _, script := range []string{
		`var page = api.call("wazuh_list_agents", {}); var agent = api.tryCall("wazuh_get_agent", {agent_id: page.affected_items[0].id}); JSON.stringify({agent: agent, summary: api.tryCall("wazuh_get_agent_status_summary", {})})`,
		`var page = api.call("wazuh_list_agents", {status: "disconnected"}); JSON.stringify(page.affected_items.map(function(agent) { return api.tryCall("wazuh_get_agent", {agent_id: agent.id}); }))`,
	} {
		text, failed := liveCall(t, session, "execute", map[string]any{"script": script})
		require.False(t, failed, text)
		require.Contains(t, text, "endpoint-one")
		require.NotContains(t, text, "SECRET_SENTINEL")
		t.Logf("MCP chained script bytes=%d passed=true", len(text))
	}
	return names
}

func TestUIWazuh(t *testing.T) {
	app := newUIApp(t)
	provider := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/manager/security/user/authenticate" {
			user, password, ok := request.BasicAuth()
			if !ok || user != "fleet-reader" || password != "ui-wazuh-password" {
				http.Error(writer, "unauthorized", http.StatusUnauthorized)
				return
			}
			claims := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, time.Now().Add(time.Hour).Unix())))
			_, _ = fmt.Fprintf(writer, `{"error":0,"data":{"token":"eyJhbGciOiJIUzI1NiJ9.%s.signature"}}`, claims)
			return
		}
		if request.Header.Get("Authorization") == "" {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/manager/agents":
			_, _ = io.WriteString(writer, `{"error":0,"data":{"affected_items":[{"id":"001","name":"endpoint-one","status":"disconnected","version":"Wazuh v4.14.8","lastKeepAlive":"2026-11-01T00:00:00Z","group_config_status":"not synced","os":{"name":"Ubuntu","version":"24.04","uname":"SECRET_SENTINEL"},"ip":"SECRET_SENTINEL"}],"total_affected_items":1,"total_failed_items":0,"failed_items":[]}}`)
		case "/manager/agents/summary/status":
			_, _ = io.WriteString(writer, `{"error":0,"data":{"connection":{"active":0,"disconnected":1,"never_connected":0,"pending":0,"total":1},"configuration":{"synced":0,"not_synced":1,"total":1}}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	cert := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw}))
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/wazuh"]`)
	uiText(t, card, "Disabled")
	require.Empty(t, uiWazuhMCP(t, app))
	require.NoError(t, card.Click())
	form := page.Locator("#integration-configuration")
	for _, field := range []struct{ key, inputType, placeholder string }{
		{"base_url", "text", "HTTPS Wazuh manager"}, {"username", "text", "agent:read"},
		{"password", "password", "reader password"}, {"ca_cert", "text", "PEM CA certificate"},
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
	uiText(t, page.Locator("body"), "Available Tools (3)")
	require.NoError(t, form.Locator("#cred_base_url").Fill(provider.URL+"/manager"))
	require.NoError(t, form.Locator("#cred_username").Fill("fleet-reader"))
	require.NoError(t, form.Locator("#cred_ca_cert").Fill(strings.ReplaceAll(cert, "\n", `\n`)))
	require.NoError(t, form.Locator("#cred_password").Fill("invalid-password"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_password").Fill("ui-wazuh-password"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiWazuhMCP(t, app), 3)
	uiCapture(t, page, "wazuh-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	uiCapture(t, page, "wazuh-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))
	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/wazuh")
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiWazuhMCP(t, app), 3)
	require.NoError(t, form.Locator("#cred_password").Fill(""))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_password").Fill("ui-wazuh-password"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiWazuhMCP(t, app))
	uiNavigate(t, page, app.URL+"/integrations")
	uiText(t, card, "Disabled")
}
