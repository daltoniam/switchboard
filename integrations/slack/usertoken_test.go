package slack

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Browser session tokens copied into Switchboard log users out of strict
// workspaces (verified 2026-10-01). The user-token path accepts only xoxp-.

func TestVerifyUserToken_ReturnsTeamFromAuthTest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		sent := r.FormValue("token")
		if sent == "" {
			sent = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		}
		assert.Equal(t, "xoxp-good", sent)
		assert.Empty(t, r.Header.Get("Cookie"), "user tokens must not carry a session cookie")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"team_id":"T1","team":"Totto Labs","user_id":"U1"}`))
	}))
	defer srv.Close()

	got, err := verifyUserTokenWith(t.Context(), "xoxp-good", srv.URL+"/", nil)

	require.NoError(t, err)
	assert.Equal(t, VerifiedUserToken{TeamID: "T1", TeamName: "Totto Labs", Token: "xoxp-good"}, got)
}

func TestVerifyUserToken_RejectsNonUserTokensWithoutNetwork(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer srv.Close()

	for _, tok := range []string{"xoxb-bot", "xoxc-browser", "", "garbage"} {
		_, err := verifyUserTokenWith(t.Context(), tok, srv.URL+"/", nil)
		assert.ErrorIs(t, err, errNotUserToken, tok)
	}
	assert.Equal(t, int32(0), hits.Load())
}

func TestVerifyUserToken_SurfacesSlackRejection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"error":"invalid_auth"}`))
	}))
	defer srv.Close()

	_, err := verifyUserTokenWith(t.Context(), "xoxp-dead", srv.URL+"/", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_auth")
}

func TestSaveUserToken_AddsEntryAndKeepsOthers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	existing := &tokenStore{workspaces: map[string]*workspace{}, filePath: path}
	existing.setWorkspace(&workspace{TeamID: "T0", TeamName: "Other", Token: "xoxp-other", Source: "oauth_user"})
	require.NoError(t, existing.saveToFile())

	require.NoError(t, saveUserTokenTo(path, VerifiedUserToken{TeamID: "T1", TeamName: "Totto Labs", Token: "xoxp-new"}))

	loaded := &tokenStore{workspaces: map[string]*workspace{}, filePath: path}
	loaded.loadFromFile()
	ws := loaded.getWorkspace("T1")
	require.NotNil(t, ws)
	assert.Equal(t, "xoxp-new", ws.Token)
	assert.Empty(t, ws.Cookie)
	assert.Equal(t, "oauth_user", ws.Source)
	assert.Equal(t, "Totto Labs", ws.TeamName)
	assert.NotNil(t, loaded.getWorkspace("T0"), "existing workspaces must survive")
}

func TestSaveUserToken_SameTeamReplacesToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	require.NoError(t, saveUserTokenTo(path, VerifiedUserToken{TeamID: "T1", TeamName: "Totto Labs", Token: "xoxp-first"}))
	require.NoError(t, saveUserTokenTo(path, VerifiedUserToken{TeamID: "T1", TeamName: "Totto Labs", Token: "xoxp-second"}))

	loaded := &tokenStore{workspaces: map[string]*workspace{}, filePath: path}
	loaded.loadFromFile()
	assert.Len(t, loaded.allWorkspaces(), 1)
	assert.Equal(t, "xoxp-second", loaded.getWorkspace("T1").Token)
}

// A manifest with bot scopes yields a bot that posts as the app, not the user.
func TestUserTokenManifest_HasUserScopesOnly(t *testing.T) {
	assert.Contains(t, UserTokenManifest, "    user:\n")
	assert.Contains(t, UserTokenManifest, "      - chat:write\n")
	assert.NotContains(t, UserTokenManifest, "bot:")
	assert.NotContains(t, UserTokenManifest, "bot_user")
}

// The web save loads, edits and saves the token file. Concurrent saves must
// not lose each other's workspaces.
func TestSaveUserToken_ConcurrentSavesKeepEveryWorkspace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	const n = 20
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, saveUserTokenTo(path, VerifiedUserToken{TeamID: fmt.Sprintf("T%02d", i), TeamName: "Team", Token: fmt.Sprintf("xoxp-%02d", i)}))
		}()
	}
	wg.Wait()

	loaded := &tokenStore{workspaces: map[string]*workspace{}, filePath: path}
	loaded.loadFromFile()
	assert.Len(t, loaded.allWorkspaces(), n)
}

// The running server saves its in-memory store on refresh and Configure. That
// save used to write its whole stale view and erase a token the web page had
// just added. Saves now merge with the file: newest entry per team wins, and
// only workspaces this store removed are deleted.
func TestSaveToFile_DoesNotEraseNewerEntriesOnDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	server := &tokenStore{workspaces: map[string]*workspace{}, filePath: path}
	server.setWorkspace(&workspace{TeamID: "T1", TeamName: "Stonks", Token: "xoxc-1", Cookie: "xoxd-1", Source: "slack"})
	require.NoError(t, server.saveToFile())
	server.loadFromFile()
	time.Sleep(1100 * time.Millisecond) // updated_at has one-second resolution

	require.NoError(t, saveUserTokenTo(path, VerifiedUserToken{TeamID: "T2", TeamName: "Totto Labs", Token: "xoxp-new"}))
	require.NoError(t, saveUserTokenTo(path, VerifiedUserToken{TeamID: "T1", TeamName: "Stonks", Token: "xoxp-replaces-session"}))
	require.NoError(t, server.saveToFile())

	loaded := &tokenStore{workspaces: map[string]*workspace{}, filePath: path}
	loaded.loadFromFile()
	require.NotNil(t, loaded.getWorkspace("T2"), "pasted workspace was erased")
	assert.Equal(t, "xoxp-replaces-session", loaded.getWorkspace("T1").Token, "older in-memory entry overwrote a newer one")
}

func TestSaveToFile_RemovedWorkspaceStaysRemoved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	s := &tokenStore{workspaces: map[string]*workspace{}, filePath: path}
	s.setWorkspace(&workspace{TeamID: "_web", Token: "xoxc-1", Cookie: "xoxd-1"})
	require.NoError(t, s.saveToFile())

	s.removeWorkspace("_web")
	require.NoError(t, s.saveToFile())

	loaded := &tokenStore{workspaces: map[string]*workspace{}, filePath: path}
	loaded.loadFromFile()
	assert.Nil(t, loaded.getWorkspace("_web"))
}

// A store that never touched the default must not write its stale default
// over a choice the user just made on the setup page.
func TestSaveToFile_StaleDefaultDoesNotOverwriteNewChoice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	seed := &tokenStore{workspaces: map[string]*workspace{}, filePath: path}
	seed.setWorkspace(&workspace{TeamID: "TA", Token: "xoxp-a"})
	seed.setWorkspace(&workspace{TeamID: "TB", Token: "xoxp-b"})
	seed.setDefault("TA")
	require.NoError(t, seed.saveToFile())
	server := &tokenStore{workspaces: map[string]*workspace{}, filePath: path}
	server.loadFromFile()

	web := &tokenStore{workspaces: map[string]*workspace{}, filePath: path}
	web.loadFromFile()
	web.setDefault("TB")
	require.NoError(t, web.saveToFile())
	require.NoError(t, server.saveToFile())

	loaded := &tokenStore{workspaces: map[string]*workspace{}, filePath: path}
	loaded.loadFromFile()
	assert.Equal(t, "TB", loaded.defaultID())
}

// A removal must not delete a workspace someone re-added after it.
func TestSaveToFile_RemovalDoesNotOutliveReAdd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	a := &tokenStore{workspaces: map[string]*workspace{}, filePath: path}
	a.setWorkspace(&workspace{TeamID: "_web", Token: "xoxc-1", Cookie: "xoxd-1"})
	require.NoError(t, a.saveToFile())
	a.removeWorkspace("_web")
	require.NoError(t, a.saveToFile())

	b := &tokenStore{workspaces: map[string]*workspace{}, filePath: path}
	b.loadFromFile()
	b.setWorkspace(&workspace{TeamID: "_web", Token: "xoxc-2", Cookie: "xoxd-2"})
	require.NoError(t, b.saveToFile())
	require.NoError(t, a.saveToFile())

	loaded := &tokenStore{workspaces: map[string]*workspace{}, filePath: path}
	loaded.loadFromFile()
	require.NotNil(t, loaded.getWorkspace("_web"))
	assert.Equal(t, "xoxc-2", loaded.getWorkspace("_web").Token)
}

// Within one second, a newer on-disk entry must still beat an older copy.
func TestSaveToFile_SubSecondNewerDiskEntryWins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	server := &tokenStore{workspaces: map[string]*workspace{}, filePath: path}
	server.setWorkspace(&workspace{TeamID: "T1", Token: "xoxc-old", Cookie: "xoxd-1"})
	require.NoError(t, server.saveToFile())
	time.Sleep(20 * time.Millisecond)

	require.NoError(t, saveUserTokenTo(path, VerifiedUserToken{TeamID: "T1", TeamName: "Stonks", Token: "xoxp-new"}))
	require.NoError(t, server.saveToFile())

	loaded := &tokenStore{workspaces: map[string]*workspace{}, filePath: path}
	loaded.loadFromFile()
	assert.Equal(t, "xoxp-new", loaded.getWorkspace("T1").Token)
}

// A pasted token that Slack rejects must reach the revoked list, so the
// setup page refuses it next time without sending it to Slack again.
func TestVerifyUserToken_RejectedTokenIsRecordedRevoked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"error":"token_revoked"}`))
	}))
	defer srv.Close()
	revoked, err := newRevokedCredentials(filepath.Join(t.TempDir(), "revoked.json"))
	require.NoError(t, err)

	_, err = verifyUserTokenWith(t.Context(), "xoxp-dead", srv.URL+"/", revoked)

	require.Error(t, err)
	assert.True(t, revoked.isRevoked("xoxp-dead"))
}

// An entry whose timestamp cannot be read must not beat a newer copy. It
// used to load as "now", so the file's unreadable entry erased a fresh save.
func TestSaveToFile_UnreadableTimestampDoesNotWinMerge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version":2,"workspaces":[{"team_id":"T1","token":"xoxp-old","updated_at":"garbage"}]}`), 0600))
	s := &tokenStore{workspaces: map[string]*workspace{}, filePath: path}
	s.loadFromFile()

	s.setWorkspace(&workspace{TeamID: "T1", Token: "xoxp-new"})
	require.NoError(t, s.saveToFile())

	loaded := &tokenStore{workspaces: map[string]*workspace{}, filePath: path}
	loaded.loadFromFile()
	assert.Equal(t, "xoxp-new", loaded.getWorkspace("T1").Token)
}
