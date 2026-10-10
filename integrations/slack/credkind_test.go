package slack

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The credential kind used to be re-derived from token prefixes in about ten
// places, which could disagree with the stored source field. It is now
// decided once, and refresh, cookie sending and the revoked key follow it.

func TestParseCredKind(t *testing.T) {
	for _, tc := range []struct {
		token string
		want  credKind
	}{
		{"xoxp-1", kindUserToken},
		{"xoxc-1", kindBrowserSession},
		{"xoxb-1", kindBot},
		{"", kindUnknown},
		{"garbage", kindUnknown},
	} {
		assert.Equal(t, tc.want, parseCredKind(tc.token), tc.token)
	}
}

func TestCanSelfRefresh_OnlyLocalBrowserSessions(t *testing.T) {
	for _, tc := range []struct {
		name string
		ws   workspace
		want bool
	}{
		{"browser session", workspace{Token: "xoxc-1", Cookie: "xoxd-1", Source: "chrome"}, true},
		{"config browser session", workspace{Token: "xoxc-1", Cookie: "xoxd-1", Source: "config"}, false},
		{"user token", workspace{Token: "xoxp-1", Source: "oauth_user"}, false},
		{"user token mislabeled browser", workspace{Token: "xoxp-1", Source: "browser"}, false},
		{"bot", workspace{Token: "xoxb-1", Source: "slack"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, (&slackIntegration{}).canSelfRefresh(&tc.ws))
		})
	}
}

// One workspace's stale token must not block the desktop cookie shared by
// every other workspace, and a refreshed token on the same cookie must work.
func TestRevokedKey_BrowserSessionIsTokenAndCookiePair(t *testing.T) {
	srv, hits := countingSlackServer(t, `{"ok":false,"error":"invalid_auth"}`)
	revoked := mustRevoked(t, filepath.Join(t.TempDir(), "revoked.json"))
	_, _ = guardedClientWithToken(srv, "xoxc-stale", "xoxd-shared", revoked).AuthTest()

	_, _ = guardedClientWithToken(srv, "xoxc-other-workspace", "xoxd-shared", revoked).AuthTest()

	assert.Equal(t, int32(2), hits.Load(), "a different token on the shared cookie must still be sent")
	assert.True(t, revoked.isRevoked(credentialKey(&workspace{Token: "xoxc-stale", Cookie: "xoxd-shared"})))
}

func TestCookieTransport_UserTokenNeverSendsCookie(t *testing.T) {
	var sawCookie atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawCookie.Store(r.Header.Get("Cookie") != "")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"team_id":"T1"}`))
	}))
	defer srv.Close()
	revoked := mustRevoked(t, filepath.Join(t.TempDir(), "revoked.json"))
	transport := newCookieTransport(&workspace{Token: "xoxp-1", Cookie: "xoxd-leftover"}, revoked)
	client := slack.New("xoxp-1", slack.OptionAPIURL(srv.URL+"/"), slack.OptionHTTPClient(&http.Client{Transport: transport}))

	_, err := client.AuthTest()

	require.NoError(t, err)
	assert.False(t, sawCookie.Load(), "a user token must not carry a browser cookie")
}
