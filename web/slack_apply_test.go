package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	slackInt "github.com/daltoniam/switchboard/integrations/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each setup action that saves Slack credentials or the default workspace
// must apply them to the running integration. Otherwise the page reports
// success while the running client keeps sending the old credentials until
// the next restart.
func TestSlackSetupActions_ApplySavedStateToRunningIntegration(t *testing.T) {
	origExtract, origSave, origDefault := slackExtractAll, slackSaveTokens, slackSetDefault
	t.Cleanup(func() { slackExtractAll, slackSaveTokens, slackSetDefault = origExtract, origSave, origDefault })
	slackExtractAll = func() (int, error) { return 1, nil }
	slackSaveTokens = func(string, string, string) (*slackInt.TokenInfo, error) { return &slackInt.TokenInfo{}, nil }
	slackSetDefault = func(string) error { return nil }

	for _, tc := range []struct {
		name    string
		path    string
		form    string
		enabled bool
	}{
		{"browser extraction", "/api/slack/extract-browser", "", false},
		{"manual token save", "/api/slack/save-tokens", "token=xoxc-new&cookie=xoxd-new", false},
		{"default workspace", "/api/slack/set-default", "team_id=T2", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, reg, cfgService := setupTestWeb()
			running := &runningSlack{mockIntegration: mockIntegration{name: "slack"}}
			require.NoError(t, reg.Register(running))
			cfgService.cfg.Integrations["slack"] = &mcp.IntegrationConfig{Enabled: tc.enabled, Credentials: mcp.Credentials{"team_id": "T1"}}
			notified := 0
			ws.onConfigChange = func() { notified++ }

			req := localSlackRequest(http.MethodPost, tc.path, tc.form)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()
			ws.Handler().ServeHTTP(rr, req)

			assert.Equal(t, http.StatusSeeOther, rr.Code)
			assert.Contains(t, rr.Header().Get("Location"), "result=")
			require.Len(t, running.configured, 1, "the running client must reload the saved state")
			ic, _ := cfgService.GetIntegration("slack")
			assert.Equal(t, ic.Credentials, running.configured[0])
			assert.Equal(t, 1, notified, "search visibility must follow the enabled flag")
		})
	}
}
