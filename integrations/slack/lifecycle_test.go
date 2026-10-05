package slack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The setup page changes Slack while the server runs: it disables Slack,
// swaps a browser session for a user token, and records rejected
// credentials. Each change must reach the running integration at once,
// because any path still holding a revoked browser cookie can send it and
// sign the user out of a two-factor workspace.

func newTestStore(t *testing.T) *tokenStore {
	t.Helper()
	ts := newTokenStore()
	ts.filePath = filepath.Join(t.TempDir(), "tokens.json")
	return ts
}

func TestStop_BackgroundRefreshSendsNothingAfterStop(t *testing.T) {
	orig := backgroundRefreshInterval
	backgroundRefreshInterval = time.Millisecond
	t.Cleanup(func() { backgroundRefreshInterval = orig })

	var calls atomic.Int32
	s := &slackIntegration{store: newTestStore(t), refreshWorkspace: func(context.Context, string) bool {
		calls.Add(1)
		return true
	}}
	s.store.setWorkspace(&workspace{TeamID: "T1", Token: "xoxc-1", Cookie: "xoxd-1", Source: "browser"})
	s.startBackgroundRefresh()
	require.Eventually(t, func() bool { return calls.Load() > 0 }, time.Second, time.Millisecond)

	s.Stop()
	after := calls.Load()
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, after, calls.Load(), "a refresh tick after Stop would resend the browser cookie")
}

func TestTryRefreshWorkspace_RejectionDuringCookieRefreshSkipsBrowserExtraction(t *testing.T) {
	revoked := mustRevoked(t, filepath.Join(t.TempDir(), "revoked.json"))
	s := &slackIntegration{store: newTestStore(t), revoked: revoked, clients: map[string]*slack.Client{}}
	s.store.setWorkspace(&workspace{TeamID: "T1", Token: "xoxc-old", Cookie: "xoxd-old", Source: "browser"})
	// Simulates tryRefreshViaCookieForTeam: it stores the refreshed pair,
	// Slack rejects it on auth.test, and the transport records it.
	s.cookieRefresh = func(_ context.Context, teamID string) bool {
		s.store.updateTokens(teamID, "xoxc-new", "xoxd-new")
		revoked.mark(credentialKey(s.store.getWorkspace(teamID)))
		return false
	}
	extracted := 0
	orig := extractFromBrowserFn
	t.Cleanup(func() { extractFromBrowserFn = orig })
	extractFromBrowserFn = func(string) *browserTokens {
		extracted++
		return &browserTokens{token: "xoxc-other", cookie: "xoxd-other", source: "chrome"}
	}

	assert.False(t, s.tryRefreshWorkspace(t.Context(), "T1"))
	assert.Equal(t, 0, extracted, "a fresh browser pair would bypass the guard on the rejected one")
	assert.Equal(t, "xoxc-new", s.store.getWorkspace("T1").Token)
}

func TestRevokedCredentials_InstancesOnOnePathShareState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "revoked.json")
	running := mustRevoked(t, path)
	setupPage := mustRevoked(t, path)
	srv, hits := countingSlackServer(t, `{"ok":true,"team_id":"T1"}`)
	client := guardedClient(srv, "xoxd-shared", running)

	setupPage.mark("xoxc-test\x00xoxd-shared")

	_, err := client.AuthTest()
	require.ErrorIs(t, err, errRevokedCredential)
	assert.Equal(t, int32(0), hits.Load())
}

func TestConfigure_ReloadIsSafeDuringToolCalls(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	creds := mcp.Credentials{"token": "xoxp-test", "team_id": "T1"}
	s := &slackIntegration{}
	require.NoError(t, s.Configure(t.Context(), creds))

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = s.getClient()
				_ = s.store.allWorkspaces()
				_ = s.revoked.isRevoked("xoxp-test")
			}
		}
	}()
	for range 3 {
		require.NoError(t, s.Configure(t.Context(), creds))
	}
	close(stop)
	wg.Wait()
}

// A cookie refresh that started before the user saved new credentials must
// not write its result over them: that would restore the browser session
// the user just replaced and send its cookie again.
func TestTryRefreshViaCookie_StaleResultDoesNotOverwriteReplacedCredentials(t *testing.T) {
	s := &slackIntegration{store: newTestStore(t), clients: map[string]*slack.Client{}}
	s.store.setWorkspace(&workspace{TeamID: "T1", Token: "xoxc-old", Cookie: "xoxd-old", Source: "browser"})
	entered, release := make(chan struct{}), make(chan struct{})
	orig := cookieFetch
	t.Cleanup(func() { cookieFetch = orig })
	cookieFetch = func(context.Context, string) (*refreshResult, error) {
		close(entered)
		<-release
		return &refreshResult{token: "xoxc-stale", cookie: "xoxd-stale"}, nil
	}

	done := make(chan bool)
	go func() { done <- s.tryRefreshViaCookieForTeam(context.Background(), "T1") }()
	<-entered
	replacement := newTestStore(t)
	replacement.setWorkspace(&workspace{TeamID: "T1", Token: "xoxp-new", Source: "user_token"})
	s.store.replaceWith(replacement)
	close(release)

	assert.False(t, <-done)
	ws := s.store.getWorkspace("T1")
	assert.Equal(t, "xoxp-new", ws.Token)
	assert.Empty(t, ws.Cookie)
}

// Stop holds the lifecycle lock while it waits for the refresh worker, so a
// cookie fetch that never gets a response would hang the setup page and
// every later reload. Stopping must cancel the fetch itself.
func TestStop_CancelsStalledCookieFetch(t *testing.T) {
	origInterval, origFetch, origExtract := backgroundRefreshInterval, cookieFetch, extractFromBrowserFn
	t.Cleanup(func() {
		backgroundRefreshInterval, cookieFetch, extractFromBrowserFn = origInterval, origFetch, origExtract
	})
	backgroundRefreshInterval = time.Millisecond
	extractFromBrowserFn = func(string) *browserTokens { return nil }

	requested := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case requested <- struct{}{}:
		default:
		}
		<-r.Context().Done()
	}))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	cookieFetch = func(ctx context.Context, cookie string) (*refreshResult, error) {
		return refreshViaCookieWithClient(ctx, nil, srv.URL, cookie)
	}

	s := &slackIntegration{store: newTestStore(t), clients: map[string]*slack.Client{}, revoked: mustRevoked(t, filepath.Join(t.TempDir(), "revoked.json"))}
	s.store.setWorkspace(&workspace{TeamID: "T1", Token: "xoxc-1", Cookie: "xoxd-1", Source: "browser"})
	s.cookieRefresh = s.tryRefreshViaCookieForTeam
	s.startBackgroundRefresh()
	<-requested

	stopped := make(chan struct{})
	go func() { s.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop is still waiting on a stalled cookie fetch")
	}
}
