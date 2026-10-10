package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The config UI has no login. Without a same-origin check, any website the
// user visits can post an attacker's Slack token and make it the default, so
// the agent posts into the attacker's workspace.

func localSlackRequest(method, path, form string) *http.Request {
	r := httptest.NewRequest(method, "http://127.0.0.1:3847"+path, strings.NewReader(form))
	r.RemoteAddr = "127.0.0.1:54321"
	if form != "" {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return r
}

func TestSlackRoutes_RejectCrossSiteRequests(t *testing.T) {
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/slack/save-tokens"},
		{http.MethodPost, "/api/slack/set-default"},
		{http.MethodPost, "/api/slack/extract-browser"},
		{http.MethodPost, "/api/slack/set-enabled"},
		{http.MethodPost, "/api/slack/add-user-token"},
		{http.MethodGet, "/api/slack/list-workspaces"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			ws, _, _ := setupTestWeb()
			stubSlackUserToken(t, nil)

			// A cross-site GET cannot read the response (same-origin policy), so
			// only state-changing routes need the Origin check.
			if tc.method == http.MethodPost {
				crossSite := localSlackRequest(tc.method, tc.path, "token=xoxp-attacker&team_id=T9")
				crossSite.Header.Set("Origin", "https://evil.example")
				crossSite.Header.Set("Sec-Fetch-Site", "cross-site")
				rr := httptest.NewRecorder()
				ws.Handler().ServeHTTP(rr, crossSite)
				assert.Equal(t, http.StatusForbidden, rr.Code, "cross-site request must be refused")
			}

			rebound := httptest.NewRequest(tc.method, "http://attacker.example"+tc.path, strings.NewReader(""))
			rebound.RemoteAddr = "127.0.0.1:54321"
			rr := httptest.NewRecorder()
			ws.Handler().ServeHTTP(rr, rebound)
			assert.Equal(t, http.StatusForbidden, rr.Code, "DNS-rebound Host must be refused")
		})
	}
}
