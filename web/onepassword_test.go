package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/onepassword"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func postOnePasswordIdentity(t *testing.T, ws *WebServer, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/integrations/onepassword/identities", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, req)
	require.Equal(t, http.StatusSeeOther, rr.Code)
	return rr
}

func TestIdentityResult_RedirectsToSetupPage(t *testing.T) {
	tests := []struct {
		name    string
		form    url.Values
		wantKey string
	}{
		{"saved", url.Values{"identity_id": {"work"}, "identity_cred_service_account_token": {"ops_work"}}, "success"},
		{"rejected", url.Values{"identity_id": {"not/safe"}}, "error"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ws, reg, cfgService := setupTestWeb()
			reg.integrations["onepassword"] = &multiIdentityWebMock{name: "onepassword", healthy: true}
			cfgService.cfg.Integrations["onepassword"] = &mcp.IntegrationConfig{Credentials: mcp.Credentials{"service_account_token": "ops_default"}}
			location := postOnePasswordIdentity(t, ws, tc.form).Header().Get("Location")
			assert.True(t, strings.HasPrefix(location, "/integrations/onepassword/setup?"+tc.wantKey+"="), location)
		})
	}
}

func TestOnePasswordIdentity_RejectsSecondDesktopIdentity(t *testing.T) {
	if !onepassword.DesktopAppAvailable() {
		t.Skip("this test binary was built without CGO, so desktop app identities are rejected for that reason instead")
	}
	ws, reg, cfgService := setupTestWeb()
	reg.integrations["onepassword"] = onepassword.New()
	cfgService.cfg.Integrations["onepassword"] = &mcp.IntegrationConfig{
		Credentials: mcp.Credentials{"account_name": "My Team"},
		Identities: map[string]mcp.IntegrationIdentity{
			"work": {Credentials: mcp.Credentials{"service_account_token": "ops_work"}},
		},
	}
	rr := postOnePasswordIdentity(t, ws, url.Values{"identity_id": {"family"}, "identity_cred_account_name": {"Family"}, "identity_meta_label": {"Family"}})
	location, err := url.Parse(rr.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "/integrations/onepassword/setup", location.Path)
	msg := location.Query().Get("error")
	assert.Contains(t, msg, "only one identity can sign in through the 1Password desktop app")
	assert.Contains(t, msg, `"default" and "family"`)
	ic, ok := cfgService.GetIntegration("onepassword")
	require.True(t, ok)
	assert.NotContains(t, ic.Identities, "family", "a rejected identity is not saved")
	assert.Contains(t, ic.Identities, "work")

	rr = postOnePasswordIdentity(t, ws, url.Values{"identity_id": {"ci"}, "identity_cred_service_account_token": {"ops_ci"}, "identity_meta_label": {"CI"}})
	assert.Contains(t, rr.Header().Get("Location"), "success=")
	ic, _ = cfgService.GetIntegration("onepassword")
	assert.Equal(t, "ops_ci", ic.Identities["ci"].Credentials["service_account_token"])
	assert.Equal(t, "CI", ic.Identities["ci"].Metadata["label"])
}

func TestOnePasswordSetup_ListsIdentitiesWithoutSecrets(t *testing.T) {
	ws, reg, cfgService := setupTestWeb()
	reg.integrations["onepassword"] = onepassword.New()
	cfgService.cfg.Integrations["onepassword"] = &mcp.IntegrationConfig{
		Credentials: mcp.Credentials{"service_account_token": "ops_default-never-render"},
		Identities: map[string]mcp.IntegrationIdentity{
			"work": {Credentials: mcp.Credentials{"service_account_token": "ops_work-never-render"}, Metadata: map[string]string{"label": "Work vaults"}},
			"me":   {Credentials: mcp.Credentials{"account_name": "My Team"}},
		},
	}
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/integrations/onepassword/setup", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.False(t, strings.Contains(body, "ops_work-never-render"), "identity tokens are never rendered")
	for _, want := range []string{`data-onepassword-identity="work"`, "Work vaults", `data-onepassword-identity="me"`, "My Team"} {
		assert.True(t, strings.Contains(body, want), want)
	}
}
