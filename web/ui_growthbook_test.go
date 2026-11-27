package web

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

const uiGrowthBookKey = "secret_ui_growthbook"

func uiGrowthBookCall(t *testing.T, ctx context.Context, session *mcpsdk.ClientSession, args map[string]any) string {
	t.Helper()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: args})
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	require.False(t, result.IsError, text.Text)
	require.NotContains(t, text.Text, "SECRET_SENTINEL")
	require.NotContains(t, text.Text, uiGrowthBookKey)
	return text.Text
}

func uiGrowthBookMCP(t *testing.T, app *uiApp) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "growthbook-ui-acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"query": "growthbook feature flags experiments", "integration": "growthbook", "limit": 10}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	names := uiToolNames(t, text.Text)
	if len(names) == 0 {
		return nil
	}
	require.Contains(t, names, "growthbook_list_features")
	require.Contains(t, names, "growthbook_list_experiments")

	features := uiGrowthBookCall(t, ctx, session, map[string]any{"tool_name": "growthbook_list_features", "arguments": map[string]any{}})
	require.Contains(t, features, "ui-checkout-flag")
	require.NotContains(t, features, "definition")
	connections := uiGrowthBookCall(t, ctx, session, map[string]any{"tool_name": "growthbook_list_sdk_connections", "arguments": map[string]any{}})
	require.Contains(t, connections, "UI Web App")
	script := uiGrowthBookCall(t, ctx, session, map[string]any{"script": `JSON.stringify(api.call('growthbook_list_sdk_connections', {}))`})
	require.Contains(t, script, "UI Web App")
	require.NotContains(t, script, "encryptionKey")
	toggle := uiGrowthBookCall(t, ctx, session, map[string]any{"tool_name": "growthbook_toggle_feature", "arguments": map[string]any{"feature_id": "ui-checkout-flag", "environment": "production", "enabled": false}})
	require.Contains(t, toggle, "pending_confirmation")
	require.Contains(t, toggle, "https://app.growthbook.example/confirm/cnf_ui")
	return names
}

func TestUIGrowthBook(t *testing.T) {
	app := newUIApp(t)
	t.Logf("isolated development URL: %s", app.URL)
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+uiGrowthBookKey {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"message":"Invalid API key"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		page := `"limit":20,"offset":0,"count":1,"total":1,"hasMore":false,"nextOffset":null`
		switch r.Method + " " + r.URL.Path {
		case "GET /gb/api/v1/projects":
			_, _ = io.WriteString(w, `{"projects":[{"id":"prj_ui","name":"UI Project"}],`+page+`}`)
		case "GET /gb/api/v2/features":
			_, _ = io.WriteString(w, `{"features":[{"id":"ui-checkout-flag","valueType":"boolean","defaultValue":"false","environments":{"production":{"enabled":true,"definition":"{\"defaultValue\":false}"}},"rules":[]}],`+page+`}`)
		case "GET /gb/api/v1/sdk-connections":
			_, _ = io.WriteString(w, `{"connections":[{"id":"sdk_ui","name":"UI Web App","key":"sdk-ui-public","environment":"production","encryptionKey":"SECRET_SENTINEL","proxySigningKey":"SECRET_SENTINEL"}],`+page+`}`)
		case "POST /gb/api/v2/features/ui-checkout-flag/toggle":
			var body struct {
				Environments map[string]any `json:"environments"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Environments) != 1 || body.Environments["production"] != false {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"message":"expected a request that disables only production"}`)
				return
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, `{"confirmation":{"id":"cnf_ui","status":"pending","url":"https://app.growthbook.example/confirm/cnf_ui","actions":[{"action":"feature.toggle","environments":["production"]}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(provider.Close)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})

	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/growthbook"]`)
	uiText(t, card, "Disabled")
	require.Empty(t, uiGrowthBookMCP(t, app))
	require.NoError(t, card.Click())
	form := page.Locator("#integration-configuration")
	for _, field := range []struct{ key, inputType, placeholder string }{
		{"api_key", "password", "Secret Key"}, {"base_url", "text", "api.growthbook.io"}, {"ca_cert", "text", "PEM CA certificate"},
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
	uiText(t, form.Locator(`label[for="cred_base_url"]`), "optional")
	uiText(t, form.Locator(`label[for="cred_ca_cert"]`), "optional")
	uiText(t, page.Locator("body"), "Available Tools (37)")

	require.NoError(t, form.Locator("#cred_base_url").Fill(provider.URL+"/gb"))
	require.NoError(t, form.Locator("#cred_ca_cert").Fill(strings.ReplaceAll(string(cert), "\n", `\n`)))
	require.NoError(t, form.Locator("#cred_api_key").Fill("secret_wrong_key"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_api_key").Fill(uiGrowthBookKey))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.NotEmpty(t, uiGrowthBookMCP(t, app))
	uiCapture(t, page, "growthbook-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	uiCapture(t, page, "growthbook-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))

	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/growthbook")
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.NotEmpty(t, uiGrowthBookMCP(t, app))

	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiGrowthBookMCP(t, app))
	uiNavigate(t, page, app.URL+"/integrations")
	uiText(t, card, "Disabled")
}
