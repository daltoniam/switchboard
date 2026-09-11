package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/daltoniam/switchboard/web/templates/layouts"
	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderIntegrationDetail(t *testing.T, data IntegrationDetailData) string {
	t.Helper()
	var buf bytes.Buffer
	page := layouts.PageData{Title: data.Name, CurrentPath: "/integrations"}
	require.NoError(t, IntegrationDetail(page, data).Render(context.Background(), &buf))
	return buf.String()
}

func TestIntegrationDetail_RendersToolDescriptions(t *testing.T) {
	data := IntegrationDetailData{
		Name:    "github",
		Enabled: true,
		Healthy: true,
		Tools: []ToolInfo{
			{Name: "github_search_repos", Description: "Search across GitHub repositories."},
			{Name: "github_create_issue", Description: ""},
		},
	}

	out := renderIntegrationDetail(t, data)

	assert.Contains(t, out, "Available Tools (2)")
	assert.Contains(t, out, "github_search_repos")
	assert.Contains(t, out, "Search across GitHub repositories.")
	assert.Contains(t, out, "github_create_issue")
	assert.Contains(t, out, "No description available")
	assert.True(t, strings.Contains(out, "tool-row"), "expected tool-row layout class")
}

func TestIntegrationDetail_OmitsToolsSectionWhenEmpty(t *testing.T) {
	data := IntegrationDetailData{Name: "github"}
	out := renderIntegrationDetail(t, data)
	assert.NotContains(t, out, "Available Tools")
}

func TestIntegrationDetail_RendersIdentityEditorWithoutSecrets(t *testing.T) {
	data := IntegrationDetailData{
		Name:               "slackmcp",
		SupportsIdentities: true,
		IdentityCredentialKeys: []string{
			"access_token",
		},
		IdentityMetadataKeys: []string{"label", "app_id"},
		Identities: []IdentityView{
			{
				ID:    "work",
				Label: "Work Slack",
				Credentials: []IdentityCredentialField{
					{Key: "access_token", Configured: true},
				},
				Metadata: []CredentialField{
					{Key: "label", Value: "Work Slack"},
					{Key: "app_id", Value: "A123"},
				},
			},
		},
	}

	out := renderIntegrationDetail(t, data)
	assert.Contains(t, out, "Named identities")
	assert.Contains(t, out, "Work Slack")
	assert.Contains(t, out, "work")
	assert.Contains(t, out, "Configured — leave blank to keep")
	assert.Contains(t, out, "identity_cred_access_token")
	assert.Contains(t, out, "identity_meta_label")
	assert.Contains(t, out, `id="identity-work-meta-label"`)
	assert.Contains(t, out, `id="identity-new-meta-label"`)
	assert.NotContains(t, out, `id="identity_meta_label"`)
	assert.Contains(t, out, "Add identity")
	assert.NotContains(t, out, "secret-token-value")
}

func TestIntegrationDetail_OAuthConnectUsesEditedCredentials(t *testing.T) {
	out := renderIntegrationDetail(t, IntegrationDetailData{
		Name:                   "primer",
		ConnectOAuth:           true,
		SupportsIdentities:     true,
		IdentityCredentialKeys: []string{"oauth_client_id"},
		Credentials: []CredentialField{
			{Key: "oauth_issuer", Value: "https://saved.example"},
			{Key: "oauth_client_id", Value: "saved-client"},
			{Key: "oauth_scopes", Value: "saved-scope"},
			{Key: "base_url", Value: "https://saved-api.example"},
		},
	})
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(out))
	require.NoError(t, err)
	doc.Find("body").PrependHtml(`<form><input name="cred_oauth_client_id" value="unrelated-client"><input name="cred_unrelated" value="unrelated-secret"></form>`)
	button := doc.Find("button[data-oauth-start]")
	require.Equal(t, 1, button.Length())
	formID, ok := button.Attr("form")
	require.True(t, ok, "Connect must be associated with its configuration form")
	form := doc.Find("form").FilterFunction(func(_ int, s *goquery.Selection) bool {
		return s.AttrOr("id", "") == formID
	})
	require.Equal(t, 1, form.Length())
	assert.Equal(t, "/integrations/primer", form.AttrOr("action", ""))
	assert.Zero(t, form.Find(`[name^="identity_"]`).Length())
	form.Find(`[name="cred_oauth_issuer"]`).SetAttr("value", "https://edited.example")
	form.Find(`[name="cred_oauth_client_id"]`).SetAttr("value", "edited-client")
	form.Find(`[name="cred_oauth_scopes"]`).SetAttr("value", "")
	form.Find(`[name="cred_base_url"]`).SetAttr("value", "https://edited-api.example")

	vm := goja.New()
	require.NoError(t, vm.Set("button", map[string]any{
		"disabled": false,
		"dataset":  map[string]string{"oauthStart": button.AttrOr("data-oauth-start", "")},
		"form": map[string]any{
			"querySelectorAll": func(selector string) []map[string]string {
				var inputs []map[string]string
				form.Find(selector).Each(func(_ int, input *goquery.Selection) {
					inputs = append(inputs, map[string]string{"name": input.AttrOr("name", ""), "value": input.AttrOr("value", "")})
				})
				return inputs
			},
		},
	}))
	_, err = vm.RunString(`
		let request, destination;
		function fetch(url, options) {
			request = {url, options};
			return Promise.resolve({ok: true, json: () => Promise.resolve({authorize_url: "https://edited.example/authorize"})});
		}
		const window = {location: {assign: url => {destination = url;}}};
		function alert(message) { throw new Error(message); }
	`)
	require.NoError(t, err)
	script := doc.Find("script").FilterFunction(func(_ int, s *goquery.Selection) bool {
		return strings.Contains(s.Text(), "async function connectPluginOAuth")
	}).Text()
	require.NotEmpty(t, script)
	_, err = vm.RunString(script)
	require.NoError(t, err)
	result, err := vm.RunString("connectPluginOAuth(button)")
	require.NoError(t, err)
	promise, ok := result.Export().(*goja.Promise)
	require.True(t, ok)
	require.Equal(t, goja.PromiseStateFulfilled, promise.State(), "%v", promise.Result())
	for expression, want := range map[string]string{
		"request.url":                             "/api/integrations/primer/oauth/start",
		"request.options.method":                  "POST",
		"request.options.credentials":             "same-origin",
		`request.options.headers["Content-Type"]`: "application/json",
		"destination":                             "https://edited.example/authorize",
	} {
		value, err := vm.RunString(expression)
		require.NoError(t, err)
		assert.Equal(t, want, value.String(), expression)
	}
	body, err := vm.RunString("request.options.body")
	require.NoError(t, err)
	assert.JSONEq(t, `{"credentials":{"oauth_issuer":"https://edited.example","oauth_client_id":"edited-client","oauth_scopes":"","base_url":"https://edited-api.example"}}`, body.String())
}

func TestIntegrationDetail_OmitsIdentityEditorWhenUnsupported(t *testing.T) {
	out := renderIntegrationDetail(t, IntegrationDetailData{Name: "github"})
	assert.NotContains(t, out, "Named identities")
	assert.NotContains(t, out, "Add identity")
}
