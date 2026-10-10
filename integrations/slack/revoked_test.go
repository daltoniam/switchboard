package slack

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Workspaces with stricter session security (observed: a 2FA-enforcing
// workspace) end the user's live browser and desktop sessions when a request
// arrives carrying a session cookie Slack already revoked. Verified live on
// 2026-10-01: one auth.test with a dead token+cookie logged the user out.
// So once Slack answers invalid_auth for a cookie, that cookie must never be
// sent again — not by tool calls, startup auth.test, or cookie refresh —
// until new keys (a different cookie) arrive.

func countingSlackServer(t *testing.T, body string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func mustRevoked(t *testing.T, path string) *revokedCredentials {
	t.Helper()
	r, err := newRevokedCredentials(path)
	require.NoError(t, err)
	return r
}

func guardedClient(srv *httptest.Server, cookie string, revoked *revokedCredentials) *slack.Client {
	return guardedClientWithToken(srv, "xoxc-test", cookie, revoked)
}

func guardedClientWithToken(srv *httptest.Server, token, cookie string, revoked *revokedCredentials) *slack.Client {
	transport := newCookieTransport(&workspace{Token: token, Cookie: cookie}, revoked)
	return slack.New(token, slack.OptionAPIURL(srv.URL+"/"), slack.OptionHTTPClient(&http.Client{Transport: transport}))
}

func TestCookieTransport_NeverResendsCookieAfterInvalidAuth(t *testing.T) {
	srv, hits := countingSlackServer(t, `{"ok":false,"error":"invalid_auth"}`)
	revoked := mustRevoked(t, filepath.Join(t.TempDir(), "revoked.json"))
	client := guardedClient(srv, "xoxd-dead", revoked)

	_, err := client.AuthTest()
	require.Error(t, err)
	_, err = client.AuthTest()
	require.Error(t, err)

	assert.Equal(t, int32(1), hits.Load(), "a revoked cookie must not reach Slack a second time")
}

func TestCookieTransport_NewCookieStillSent(t *testing.T) {
	srv, hits := countingSlackServer(t, `{"ok":false,"error":"invalid_auth"}`)
	revoked := mustRevoked(t, filepath.Join(t.TempDir(), "revoked.json"))
	_, _ = guardedClient(srv, "xoxd-dead", revoked).AuthTest()

	_, _ = guardedClient(srv, "xoxd-fresh", revoked).AuthTest()

	assert.Equal(t, int32(2), hits.Load(), "fresh keys must not be blocked by an older revoked cookie")
}

func TestCookieTransport_SuccessfulResponseDoesNotRevoke(t *testing.T) {
	srv, hits := countingSlackServer(t, `{"ok":true,"team_id":"T1","team":"Team"}`)
	revoked := mustRevoked(t, filepath.Join(t.TempDir(), "revoked.json"))
	client := guardedClient(srv, "xoxd-live", revoked)

	_, err := client.AuthTest()
	require.NoError(t, err)
	_, err = client.AuthTest()
	require.NoError(t, err)

	assert.Equal(t, int32(2), hits.Load())
}

// The setup page and every restart rebuild the integration, so the record
// must outlive the in-memory transport.
func TestRevokedCredentials_PersistAcrossInstances(t *testing.T) {
	srv, hits := countingSlackServer(t, `{"ok":false,"error":"invalid_auth"}`)
	path := filepath.Join(t.TempDir(), "revoked.json")
	_, _ = guardedClient(srv, "xoxd-dead", mustRevoked(t, path)).AuthTest()

	_, err := guardedClient(srv, "xoxd-dead", mustRevoked(t, path)).AuthTest()

	require.Error(t, err)
	assert.Equal(t, int32(1), hits.Load())
}

func TestTokenStatus_ScopeProbeSkipsRevokedCookie(t *testing.T) {
	srv, hits := countingSlackServer(t, `{"ok":true}`)
	revoked := mustRevoked(t, filepath.Join(t.TempDir(), "revoked.json"))
	revoked.mark(credentialKey(&workspace{Token: "xoxc-one", Cookie: "xoxd-dead"}))
	store := &tokenStore{workspaces: map[string]*workspace{
		"T1": {TeamID: "T1", Token: "xoxc-one", Cookie: "xoxd-dead"},
	}}

	_, err := tokenStatusWithEndpoint(t.Context(), &slackIntegration{store: store, revoked: revoked}, srv.URL+"/auth.test")

	require.NoError(t, err)
	assert.Equal(t, int32(0), hits.Load(), "token status must not send a revoked cookie")
}

// A corrupt record must not silently reset to empty: that would resend every
// dead cookie. The user has to see that the configuration is bad.
func TestRevokedCredentials_CorruptFileIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "revoked.json")
	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o600))

	_, err := newRevokedCredentials(path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), path)
}

func TestRevokedCredentials_MissingFileStartsEmpty(t *testing.T) {
	r, err := newRevokedCredentials(filepath.Join(t.TempDir(), "absent.json"))
	require.NoError(t, err)
	assert.False(t, r.isRevoked("xoxd-anything"))
}

// User tokens (xoxp-) carry no cookie, so the guard keys on the token instead.
func TestCookieTransport_NeverResendsUserTokenAfterInvalidAuth(t *testing.T) {
	srv, hits := countingSlackServer(t, `{"ok":false,"error":"invalid_auth"}`)
	revoked := mustRevoked(t, filepath.Join(t.TempDir(), "revoked.json"))
	client := guardedClientWithToken(srv, "xoxp-dead", "", revoked)

	_, err := client.AuthTest()
	require.Error(t, err)
	_, err = client.AuthTest()

	require.ErrorIs(t, err, errRevokedCredential)
	assert.Equal(t, int32(1), hits.Load(), "a rejected user token must not reach Slack again")
}

// Two guard instances (server and setup page) share one file; neither may
// drop the other's record.
func TestRevokedCredentials_ConcurrentInstancesMerge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "revoked.json")
	a := mustRevoked(t, path)
	b := mustRevoked(t, path)

	a.mark("xoxd-a")
	b.mark("xoxd-b")

	fresh := mustRevoked(t, path)
	assert.True(t, fresh.isRevoked("xoxd-a"), "b's write dropped a's record")
	assert.True(t, fresh.isRevoked("xoxd-b"))
}

// Once a workspace's session is rejected, pulling a fresh cookie from the
// browser and sending it logs a strict workspace (Strella) out again. A
// rejected workspace must not self-refresh until the user replaces it.
func TestTryRefreshWorkspace_RejectedWorkspaceSendsNothing(t *testing.T) {
	srv, hits := countingSlackServer(t, `{"ok":true,"team_id":"T1","team":"Strella"}`)
	revoked := mustRevoked(t, filepath.Join(t.TempDir(), "revoked.json"))
	revoked.mark(credentialKey(&workspace{TeamID: "T1", Token: "xoxc-old", Cookie: "xoxd-dead"}))
	store := &tokenStore{workspaces: map[string]*workspace{
		"T1": {TeamID: "T1", Token: "xoxc-old", Cookie: "xoxd-dead", Source: "browser"},
	}, filePath: filepath.Join(t.TempDir(), "tokens.json")}
	s := &slackIntegration{store: store, revoked: revoked, clients: map[string]*slack.Client{}}
	extracted := 0
	orig := extractFromBrowserFn
	t.Cleanup(func() { extractFromBrowserFn = orig })
	extractFromBrowserFn = func(string) *browserTokens {
		extracted++
		return &browserTokens{token: "xoxc-fresh", cookie: "xoxd-fresh", source: "chrome"}
	}
	_ = srv

	ok := s.tryRefreshWorkspace(t.Context(), "T1")

	assert.False(t, ok)
	assert.Equal(t, 0, extracted, "must not read a fresh cookie from the browser")
	assert.Equal(t, int32(0), hits.Load())
	assert.Equal(t, "xoxd-dead", store.getWorkspace("T1").Cookie, "stored credential must be left for the user to replace")
}

// Slack reports a dead credential with several error codes, not only
// invalid_auth. Each must stop resends; ordinary errors must not.
func TestCookieTransport_TerminalAuthErrorsStopResends(t *testing.T) {
	for _, tc := range []struct {
		code     string
		terminal bool
	}{
		{"invalid_auth", true},
		{"token_revoked", true},
		{"token_expired", true},
		{"account_inactive", true},
		{"not_authed", true},
		{"channel_not_found", false},
		{"ratelimited", false},
	} {
		t.Run(tc.code, func(t *testing.T) {
			srv, hits := countingSlackServer(t, `{"ok":false,"error":"`+tc.code+`"}`)
			revoked := mustRevoked(t, filepath.Join(t.TempDir(), "revoked.json"))
			client := guardedClient(srv, "xoxd-c", revoked)

			_, _ = client.AuthTest()
			_, _ = client.AuthTest()

			want := int32(2)
			if tc.terminal {
				want = 1
			}
			assert.Equal(t, want, hits.Load())
		})
	}
}

// Startup refuses a corrupt record; a running process must not silently
// replace it with only the hashes it holds in memory.
func TestRevokedCredentials_MarkLeavesCorruptFileAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "revoked.json")
	r := mustRevoked(t, path)
	require.NoError(t, os.WriteFile(path, []byte("{corrupt"), 0o600))

	r.mark("xoxd-new")

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "{corrupt", string(data))
	assert.True(t, r.isRevoked("xoxd-new"), "still blocked for this process")
}

func TestUserTokenRevoked_ReadsTheRecord(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	r := mustRevoked(t, revokedFilePath())
	r.mark("xoxp-dead")

	dead, err := UserTokenRevoked("xoxp-dead")
	require.NoError(t, err)
	assert.True(t, dead)
	alive, err := UserTokenRevoked("xoxp-fresh")
	require.NoError(t, err)
	assert.False(t, alive)
}
