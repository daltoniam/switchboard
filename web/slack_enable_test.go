package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Slack setup page has no credential form fields, so its enable toggle
// cannot post to the generic /integrations/{name} handler: that handler
// replaces stored credentials with the form's cred_* fields and would wipe
// the Slack token. Disabling Slack must leave credentials intact so it can
// be re-enabled without re-extracting tokens.
func TestSlackSetEnabled_FlipsEnabledAndKeepsCredentials(t *testing.T) {
	for _, tc := range []struct {
		name    string
		form    string
		initial bool
		want    bool
	}{
		{"unchecked toggle disables", "", true, false},
		{"checked toggle enables", "enabled=true", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, _, cfgService := setupTestWeb()
			creds := mcp.Credentials{"token": "xoxc-keep", "cookie": "xoxd-keep", "team_id": "T1"}
			cfgService.cfg.Integrations["slack"] = &mcp.IntegrationConfig{Enabled: tc.initial, Credentials: creds}
			notified := 0
			ws.onConfigChange = func() { notified++ }

			req := localSlackRequest(http.MethodPost, "/api/slack/set-enabled", tc.form)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()
			ws.Handler().ServeHTTP(rr, req)

			assert.Equal(t, http.StatusSeeOther, rr.Code)
			assert.Contains(t, rr.Header().Get("Location"), "/integrations/slack/setup")
			ic, ok := cfgService.GetIntegration("slack")
			require.True(t, ok)
			assert.Equal(t, tc.want, ic.Enabled)
			assert.Equal(t, mcp.Credentials{"token": "xoxc-keep", "cookie": "xoxd-keep", "team_id": "T1"}, ic.Credentials)
			assert.Equal(t, 1, notified, "server must reload so the change takes effect without a restart")
		})
	}
}

func TestSlackSetup_RendersEnableToggle(t *testing.T) {
	ws, _, _ := setupTestWeb()
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/integrations/slack/setup", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, `action="/api/slack/set-enabled"`)
	assert.Contains(t, body, `name="enabled"`)
}

// runningSlack stands in for the registered Slack integration so tests can
// see what the setup page applies to the running server.
type runningSlack struct {
	mockIntegration
	configured []mcp.Credentials
	stops      int
}

func (r *runningSlack) Configure(_ context.Context, creds mcp.Credentials) error {
	r.configured = append(r.configured, creds)
	return nil
}
func (r *runningSlack) Stop() { r.stops++ }

// Rebuilding the search index alone leaves the running Slack client on its
// old credentials, and leaves its 4-hour cookie refresh running after a
// disable. Both handlers must apply the saved state to the integration.
func TestSlackSetEnabled_AppliesToRunningIntegration(t *testing.T) {
	for _, tc := range []struct {
		name           string
		form           string
		initial        bool
		wantConfigures int
		wantStops      int
	}{
		{"disable stops the running integration", "", true, 0, 1},
		{"enable configures the running integration", "enabled=true", false, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, reg, cfgService := setupTestWeb()
			running := &runningSlack{mockIntegration: mockIntegration{name: "slack"}}
			require.NoError(t, reg.Register(running))
			creds := mcp.Credentials{"token": "xoxp-keep", "team_id": "T1"}
			cfgService.cfg.Integrations["slack"] = &mcp.IntegrationConfig{Enabled: tc.initial, Credentials: creds}

			req := localSlackRequest(http.MethodPost, "/api/slack/set-enabled", tc.form)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			ws.Handler().ServeHTTP(httptest.NewRecorder(), req)

			require.Len(t, running.configured, tc.wantConfigures)
			assert.Equal(t, tc.wantStops, running.stops)
			if tc.wantConfigures > 0 {
				assert.Equal(t, creds, running.configured[0])
			}
		})
	}
}
