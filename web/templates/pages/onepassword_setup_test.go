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

func TestOnePasswordSetup_Identities(t *testing.T) {
	tests := []struct {
		name         string
		data         OnePasswordSetupData
		badge        string
		identities   []string
		methods      []string
		desktopInUse string
	}{
		{"none yet", OnePasswordSetupData{DesktopAvailable: true}, "Not Configured", nil, nil, ""},
		{"named identities only",
			OnePasswordSetupData{Enabled: true, Healthy: true, DesktopAvailable: true, Identities: []OnePasswordIdentity{
				{ID: "me", Account: "My Team"},
				{ID: "work", Label: "Work vaults"},
			}, DesktopIdentity: "me"},
			"Connected", []string{"me", "work"}, []string{"Desktop app as My Team", "Service account token"}, "me"},
		{"default desktop app and a service account",
			OnePasswordSetupData{Enabled: true, AccountName: "My Team", DesktopAvailable: true, Identities: []OnePasswordIdentity{{ID: "work"}}, DesktopIdentity: "default"},
			"Not Verified", []string{"work"}, []string{"Service account token"}, "default"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := renderOnePasswordSetup(t, tc.data)
			assert.Equal(t, tc.badge, strings.TrimSpace(doc.Find(".badge").First().Text()))
			card := doc.Find("#onepassword-identities")
			require.Equal(t, 1, card.Length())
			var ids, methods []string
			card.Find("[data-onepassword-identity]").Each(func(_ int, s *goquery.Selection) {
				ids = append(ids, s.AttrOr("data-onepassword-identity", ""))
				methods = append(methods, strings.TrimSpace(s.Find(".onepassword-identity-method").Text()))
				form := s.Find("form")
				assert.Equal(t, "/integrations/onepassword/identities/"+s.AttrOr("data-onepassword-identity", "")+"/delete", form.AttrOr("action", ""))
			})
			assert.Equal(t, tc.identities, ids)
			assert.Equal(t, tc.methods, methods)
			limit := card.Find("#onepassword-desktop-limit").Text()
			assert.Contains(t, limit, "Only one identity can sign in through the 1Password desktop app")
			if tc.desktopInUse != "" {
				assert.Contains(t, limit, tc.desktopInUse+" identity uses the desktop app")
			}

			add := card.Find("form#onepassword-identity-add")
			assert.Equal(t, "/integrations/onepassword/identities", add.AttrOr("action", ""))
			assert.Equal(t, "POST", add.AttrOr("method", ""))
			for name, typ := range map[string]string{
				"identity_id":                         "text",
				"identity_meta_label":                 "text",
				"identity_cred_account_name":          "text",
				"identity_cred_service_account_token": "password",
			} {
				assert.Equal(t, typ, add.Find(`input[name="`+name+`"]`).AttrOr("type", ""), name)
			}
		})
	}
}
