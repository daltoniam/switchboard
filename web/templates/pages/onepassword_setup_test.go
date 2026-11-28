package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/daltoniam/switchboard/web/templates/layouts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderOnePasswordSetup(t *testing.T, data OnePasswordSetupData) *goquery.Document {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, OnePasswordSetup(layouts.PageData{Title: "1Password Setup", CurrentPath: "/integrations"}, data).Render(context.Background(), &buf))
	doc, err := goquery.NewDocumentFromReader(&buf)
	require.NoError(t, err)
	return doc
}

func TestOnePasswordSetup_States(t *testing.T) {
	tests := []struct {
		name        string
		data        OnePasswordSetupData
		badge       string
		method      string
		signInError string
		unavailable bool
	}{
		{"fresh install", OnePasswordSetupData{DesktopAvailable: true}, "Not Configured", "", "", false},
		{"build without desktop app support", OnePasswordSetupData{}, "Not Configured", "", "", true},
		{"desktop app connected", OnePasswordSetupData{Enabled: true, Healthy: true, AccountName: "My Team", DesktopAvailable: true}, "Connected", "desktop app as My Team", "", false},
		{"desktop app unreachable", OnePasswordSetupData{Enabled: true, AccountName: "My Team", DesktopAvailable: true, SignInError: "desktop application not found"}, "Not Verified", "desktop app as My Team", "desktop application not found", false},
		{"service account saved but disabled", OnePasswordSetupData{ServiceAccountToken: "ops_x", DesktopAvailable: true, SignInError: "stale"}, "Disabled", "", "", false},
		{"service account connected", OnePasswordSetupData{Enabled: true, Healthy: true, ServiceAccountToken: "ops_x", DesktopAvailable: true}, "Connected", "service account token", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := renderOnePasswordSetup(t, tc.data)
			assert.Equal(t, tc.badge, strings.TrimSpace(doc.Find(".badge").First().Text()))
			assert.Contains(t, doc.Find("#onepassword-method").Text(), tc.method)
			if tc.method == "" {
				assert.Zero(t, doc.Find("#onepassword-method").Length())
			}
			assert.Equal(t, tc.signInError, strings.TrimSpace(doc.Find("#onepassword-sign-in-error").Text()))
			assert.Equal(t, tc.unavailable, doc.Find("#onepassword-desktop-unavailable").Length() == 1)
		})
	}
}

// Prebuilt macOS downloads are built with cgo and support the desktop app;
// only the static Linux downloads and Docker images lack it.
func TestOnePasswordSetup_UnavailableNoteNamesLinuxBuilds(t *testing.T) {
	note := strings.Join(strings.Fields(renderOnePasswordSetup(t, OnePasswordSetupData{}).Find("#onepassword-desktop-unavailable").Text()), " ")
	assert.Contains(t, note, "prebuilt Linux downloads and Docker images")
	assert.Contains(t, note, "Prebuilt macOS and Windows downloads support the desktop app")
	assert.NotContains(t, note, "macOS and Linux downloads are")
}

func TestOnePasswordSetup_FormPostsBothCredentials(t *testing.T) {
	doc := renderOnePasswordSetup(t, OnePasswordSetupData{AccountName: "My Team", DesktopAvailable: true, Tools: []ToolInfo{{Name: "onepassword_list_vaults"}}})
	form := doc.Find("form#integration-configuration")
	assert.Equal(t, "/integrations/onepassword", form.AttrOr("action", ""))
	assert.Equal(t, "My Team", form.Find(`input[name="cred_account_name"]`).AttrOr("value", ""))
	assert.Equal(t, "text", form.Find(`input[name="cred_account_name"]`).AttrOr("type", ""))
	assert.Equal(t, "password", form.Find(`input[name="cred_service_account_token"]`).AttrOr("type", ""))
	assert.Equal(t, 1, form.Find(`input[name="enabled"]`).Length())
	assert.Contains(t, form.Text(), "Integrate with 1Password SDKs")
	for _, link := range []string{
		"https://start.1password.com/developer-tools/infrastructure-secrets/serviceaccount/",
		"https://developer.1password.com/docs/service-accounts/get-started/",
		"https://developer.1password.com/docs/sdks/concepts#1password-desktop-app",
		"https://1password.com/downloads/",
	} {
		a := form.Find(`a[href="` + link + `"]`)
		require.Equal(t, 1, a.Length(), link)
		assert.Equal(t, "noopener noreferrer", a.AttrOr("rel", ""), link)
	}
	assert.Contains(t, doc.Text(), "Available Tools (1)")
}
