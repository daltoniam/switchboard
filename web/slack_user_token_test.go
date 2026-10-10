package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	slackInt "github.com/daltoniam/switchboard/integrations/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The old paste route writes browser tokens into config.json and force-enables
// Slack. This route must only verify and save a user token.

type savedToken struct{ teamID, teamName, token string }

func stubSlackUserToken(t *testing.T, verifyErr error) *[]savedToken {
	t.Helper()
	var saved []savedToken
	origVerify, origSave, origRevoked := slackVerifyUserToken, slackSaveUserToken, slackUserTokenRevoked
	t.Cleanup(func() {
		slackVerifyUserToken, slackSaveUserToken, slackUserTokenRevoked = origVerify, origSave, origRevoked
	})
	slackUserTokenRevoked = func(string) (bool, error) { return false, nil }
	slackVerifyUserToken = func(_ context.Context, token string) (slackInt.VerifiedUserToken, error) {
		if verifyErr != nil {
			return slackInt.VerifiedUserToken{}, verifyErr
		}
		if !strings.HasPrefix(token, "xoxp-") {
			return slackInt.VerifiedUserToken{}, errors.New("only Slack user tokens (xoxp-…) are accepted here")
		}
		return slackInt.VerifiedUserToken{TeamID: "T1", TeamName: "Totto Labs", Token: token}, nil
	}
	slackSaveUserToken = func(v slackInt.VerifiedUserToken) error {
		saved = append(saved, savedToken{v.TeamID, v.TeamName, v.Token})
		return nil
	}
	return &saved
}

func postUserToken(t *testing.T, ws *WebServer, token string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{"user_token": {token}}.Encode()
	req := localSlackRequest(http.MethodPost, "/api/slack/add-user-token", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, req)
	return rr
}

func TestSlackAddUserToken_SavesVerifiedTokenWithoutTouchingConfig(t *testing.T) {
	ws, _, cfgService := setupTestWeb()
	cfgService.cfg.Integrations["slack"] = &mcp.IntegrationConfig{Enabled: false, Credentials: mcp.Credentials{"client_id": "c"}}
	saved := stubSlackUserToken(t, nil)
	notified := 0
	ws.onConfigChange = func() { notified++ }

	rr := postUserToken(t, ws, "  xoxp-good\n")

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "result=")
	assert.NotContains(t, rr.Header().Get("Location"), "xoxp")
	require.Equal(t, []savedToken{{"T1", "Totto Labs", "xoxp-good"}}, *saved)
	ic, _ := cfgService.GetIntegration("slack")
	assert.False(t, ic.Enabled, "adding a token must not enable Slack")
	assert.Equal(t, mcp.Credentials{"client_id": "c"}, ic.Credentials, "config credentials must be untouched")
	assert.Equal(t, 1, notified)
}

func TestSlackAddUserToken_RejectsAndSavesNothing(t *testing.T) {
	for _, tc := range []struct {
		name, token string
		verifyErr   error
	}{
		{"empty", "", nil},
		{"bot token", "xoxb-bot", nil},
		{"browser token", "xoxc-browser", nil},
		{"slack rejects", "xoxp-dead", errors.New("invalid_auth")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, _, _ := setupTestWeb()
			saved := stubSlackUserToken(t, tc.verifyErr)

			rr := postUserToken(t, ws, tc.token)

			assert.Equal(t, http.StatusSeeOther, rr.Code)
			assert.Contains(t, rr.Header().Get("Location"), "error=")
			assert.Empty(t, *saved)
		})
	}
}

func TestSlackSetup_RendersUserTokenCard(t *testing.T) {
	ws, _, _ := setupTestWeb()
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/integrations/slack/setup", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, `action="/api/slack/add-user-token"`)
	assert.Contains(t, body, `id="user_token"`)
	assert.Contains(t, body, `type="password"`)
	assert.Contains(t, body, ": User Token<")
	assert.Greater(t, strings.Index(body, ": User Token<"), strings.Index(body, "Extract from Browser"),
		"the user token option follows the browser options")
	assert.Contains(t, body, "chat:write", "the manifest must be shown on the page")
	// docs/web-ui.md "Setup Page Design Rules": steps use the shared step list
	// and copyable text uses the shared code block, never a one-off <ol>/<pre>.
	assert.NotContains(t, body, "<ol")
	assert.Contains(t, body, `<code id="user-token-manifest">`)
	assert.Contains(t, body, `href="https://api.slack.com/apps?new_app=1" target="_blank" class="slack-link"`)
	assert.Contains(t, body, "https://api.slack.com/apps?new_app=1")
}

// Loading the setup page used to re-run Slack's Configure on the live
// integration: it raced in-flight tool calls and sent auth.test for every
// workspace, which resent dead credentials and logged a strict workspace out.
func TestSlackSetup_DoesNotReconfigureOnLoad(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	require.NoError(t, os.WriteFile(filepath.Join(home, ".slack-mcp-tokens.json"),
		[]byte(`{"version":2,"default_team_id":"T1","workspaces":[{"team_id":"T1","team_name":"Totto Labs","token":"xoxp-1","source":"oauth_user","updated_at":"2026-10-02T00:00:00Z"}]}`), 0o600))
	ws, reg, cfgService := setupTestWeb()
	slack := &mockIntegration{name: "slack", healthy: true}
	reg.Register(slack)
	cfgService.cfg.Integrations["slack"] = &mcp.IntegrationConfig{Enabled: true, Credentials: mcp.Credentials{"client_id": "c"}}

	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/integrations/slack/setup", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Nil(t, slack.lastCreds, "page load must not call Configure")
	assert.Contains(t, rr.Body.String(), "Connected", "health still comes from the running integration")
}

func TestSlackAddUserToken_RefusesRevokedTokenBeforeSlack(t *testing.T) {
	ws, _, _ := setupTestWeb()
	saved := stubSlackUserToken(t, nil)
	verified := 0
	origVerify, origRevoked := slackVerifyUserToken, slackUserTokenRevoked
	t.Cleanup(func() { slackVerifyUserToken, slackUserTokenRevoked = origVerify, origRevoked })
	slackVerifyUserToken = func(context.Context, string) (slackInt.VerifiedUserToken, error) {
		verified++
		return slackInt.VerifiedUserToken{TeamID: "T1", TeamName: "X", Token: "xoxp-dead"}, nil
	}
	slackUserTokenRevoked = func(string) (bool, error) { return true, nil }

	rr := postUserToken(t, ws, "xoxp-dead")

	assert.Contains(t, rr.Header().Get("Location"), "error=")
	assert.Equal(t, 0, verified, "a known-dead token must not be sent to Slack")
	assert.Empty(t, *saved)
}

// Every credential field on the Slack page is masked and has the show/hide
// toggle, so a token is never shown on screen by default.
func TestSlackSetup_MasksEveryCredentialField(t *testing.T) {
	ws, _, _ := setupTestWeb()
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/integrations/slack/setup", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	for _, name := range []string{"user_token", "token", "cookie"} {
		assert.Contains(t, body, `type="password" name="`+name+`"`, name)
	}
	assert.Equal(t, 3, strings.Count(body, `class="secret-toggle"`))
}

func TestSlackAddUserToken_ReconfiguresRunningIntegrationWhenEnabled(t *testing.T) {
	ws, reg, cfgService := setupTestWeb()
	running := &runningSlack{mockIntegration: mockIntegration{name: "slack"}}
	require.NoError(t, reg.Register(running))
	cfgService.cfg.Integrations["slack"] = &mcp.IntegrationConfig{Enabled: true, Credentials: mcp.Credentials{}}
	stubSlackUserToken(t, nil)

	postUserToken(t, ws, "xoxp-good")

	assert.Len(t, running.configured, 1, "the running client must pick up the new token without a restart")
}
