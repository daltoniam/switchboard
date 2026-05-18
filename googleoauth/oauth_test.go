package googleoauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withTokenURL temporarily redirects the package-level token endpoint at
// url for the duration of the test.
func withTokenURL(t *testing.T, url string) {
	t.Helper()
	prev := tokenURL
	tokenURL = url
	t.Cleanup(func() { tokenURL = prev })
}

func TestStart_Success(t *testing.T) {
	Reset()

	result, err := Start(Config{
		IntegrationName: "gmail",
		ClientID:        "client-id",
		ClientSecret:    "client-secret",
		RedirectURI:     "http://localhost:3847/callback",
		Scopes:          []string{"https://mail.google.com/"},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.AuthorizeURL)
	assert.Contains(t, result.AuthorizeURL, "accounts.google.com")
	assert.Contains(t, result.AuthorizeURL, "client_id=client-id")
	assert.Contains(t, result.AuthorizeURL, "response_type=code")
	assert.Contains(t, result.AuthorizeURL, "access_type=offline")
	assert.Contains(t, result.AuthorizeURL, "code_challenge_method=S256")
	assert.Contains(t, result.AuthorizeURL, "prompt=consent")
	assert.Contains(t, result.AuthorizeURL, "scope=")
}

func TestStart_JoinsMultipleScopesWithSpace(t *testing.T) {
	Reset()
	result, err := Start(Config{
		IntegrationName: "gcal",
		ClientID:        "cid",
		ClientSecret:    "csec",
		RedirectURI:     "http://localhost/cb",
		Scopes: []string{
			"https://www.googleapis.com/auth/calendar",
			"https://www.googleapis.com/auth/calendar.events",
		},
	})
	require.NoError(t, err)
	// URL-encoded space is '+' or %20 — url.Values uses '+'.
	assert.Contains(t, result.AuthorizeURL,
		"scope=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fcalendar+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fcalendar.events")
}

func TestStart_MissingFields(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{"missing integration name", Config{ClientID: "x", ClientSecret: "y", Scopes: []string{"s"}}, "integration name"},
		{"missing client id", Config{IntegrationName: "x", ClientSecret: "y", Scopes: []string{"s"}}, "client_id"},
		{"missing client secret", Config{IntegrationName: "x", ClientID: "y", Scopes: []string{"s"}}, "client_secret"},
		{"missing scopes", Config{IntegrationName: "x", ClientID: "y", ClientSecret: "z"}, "scope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			Reset()
			_, err := Start(tc.cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestPoll_NoFlow(t *testing.T) {
	Reset()
	r := Poll("gmail")
	assert.Equal(t, "no_flow", r.Status)
	assert.NotEmpty(t, r.Error)
}

func TestPoll_Pending(t *testing.T) {
	Reset()
	_, err := Start(Config{
		IntegrationName: "gmail",
		ClientID:        "c",
		ClientSecret:    "s",
		RedirectURI:     "http://localhost/cb",
		Scopes:          []string{"scope"},
	})
	require.NoError(t, err)
	r := Poll("gmail")
	assert.Equal(t, "pending", r.Status)
}

func TestHandleCallback_NoFlow(t *testing.T) {
	Reset()
	err := HandleCallback("gmail", "code", "state")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no OAuth flow in progress")
}

func TestHandleCallback_InvalidState(t *testing.T) {
	Reset()
	_, err := Start(Config{
		IntegrationName: "gmail",
		ClientID:        "c",
		ClientSecret:    "s",
		RedirectURI:     "http://localhost/cb",
		Scopes:          []string{"scope"},
	})
	require.NoError(t, err)

	err = HandleCallback("gmail", "code", "wrong-state")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid state parameter")

	r := Poll("gmail")
	assert.Equal(t, "error", r.Status)
}

// extractStateFromAuthorizeURL pulls the `state` query parameter back out of
// the authorize URL Start returned, so callback tests can echo it verbatim.
func extractStateFromAuthorizeURL(t *testing.T, authURL string) string {
	t.Helper()
	const key = "state="
	i := strings.Index(authURL, key)
	require.NotEqual(t, -1, i, "no state= in %s", authURL)
	rest := authURL[i+len(key):]
	if j := strings.IndexAny(rest, "&"); j != -1 {
		rest = rest[:j]
	}
	return rest
}

func TestHandleCallback_TokenExchange_Success(t *testing.T) {
	var (
		gotCode         string
		gotVerifier     string
		gotGrantType    string
		gotClientID     string
		gotClientSecret string
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		require.NoError(t, r.ParseForm())
		gotCode = r.FormValue("code")
		gotVerifier = r.FormValue("code_verifier")
		gotGrantType = r.FormValue("grant_type")
		gotClientID = r.FormValue("client_id")
		gotClientSecret = r.FormValue("client_secret")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "ya29.access",
			"refresh_token": "1//refresh",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	defer ts.Close()
	withTokenURL(t, ts.URL)

	Reset()
	start, err := Start(Config{
		IntegrationName: "gmail",
		ClientID:        "cid",
		ClientSecret:    "csec",
		RedirectURI:     "http://localhost/cb",
		Scopes:          []string{"scope"},
	})
	require.NoError(t, err)

	state := extractStateFromAuthorizeURL(t, start.AuthorizeURL)
	require.NoError(t, HandleCallback("gmail", "auth-code", state))

	assert.Equal(t, "auth-code", gotCode)
	assert.Equal(t, "authorization_code", gotGrantType)
	assert.Equal(t, "cid", gotClientID)
	assert.Equal(t, "csec", gotClientSecret)
	assert.NotEmpty(t, gotVerifier)

	r := Poll("gmail")
	assert.Equal(t, "complete", r.Status)
	assert.Equal(t, "ya29.access", r.AccessToken)
	assert.Equal(t, "1//refresh", r.RefreshToken)
}

func TestHandleCallback_TokenExchange_OAuthError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "invalid_grant",
		})
	}))
	defer ts.Close()
	withTokenURL(t, ts.URL)

	Reset()
	start, err := Start(Config{
		IntegrationName: "gmail",
		ClientID:        "cid",
		ClientSecret:    "csec",
		RedirectURI:     "http://localhost/cb",
		Scopes:          []string{"scope"},
	})
	require.NoError(t, err)
	state := extractStateFromAuthorizeURL(t, start.AuthorizeURL)

	err = HandleCallback("gmail", "auth-code", state)
	require.Error(t, err)

	r := Poll("gmail")
	assert.Equal(t, "error", r.Status)
}

func TestConcurrentFlows_IsolatedByName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		// Echo the client_id so the test can verify each integration got its own token.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "token-for-" + r.FormValue("client_id"),
			"refresh_token": "refresh-for-" + r.FormValue("client_id"),
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	defer ts.Close()
	withTokenURL(t, ts.URL)

	Reset()
	gmailStart, err := Start(Config{
		IntegrationName: "gmail", ClientID: "gmail-cid", ClientSecret: "gmail-csec",
		RedirectURI: "http://localhost/cb", Scopes: []string{"https://mail.google.com/"},
	})
	require.NoError(t, err)
	gcalStart, err := Start(Config{
		IntegrationName: "gcal", ClientID: "gcal-cid", ClientSecret: "gcal-csec",
		RedirectURI: "http://localhost/cb", Scopes: []string{"https://www.googleapis.com/auth/calendar"},
	})
	require.NoError(t, err)

	gmailState := extractStateFromAuthorizeURL(t, gmailStart.AuthorizeURL)
	gcalState := extractStateFromAuthorizeURL(t, gcalStart.AuthorizeURL)

	require.NoError(t, HandleCallback("gmail", "gmail-code", gmailState))
	require.NoError(t, HandleCallback("gcal", "gcal-code", gcalState))

	gmailPoll := Poll("gmail")
	gcalPoll := Poll("gcal")

	assert.Equal(t, "complete", gmailPoll.Status)
	assert.Equal(t, "token-for-gmail-cid", gmailPoll.AccessToken)
	assert.Equal(t, "complete", gcalPoll.Status)
	assert.Equal(t, "token-for-gcal-cid", gcalPoll.AccessToken)
}

func TestFlowExpiry(t *testing.T) {
	Reset()
	_, err := Start(Config{
		IntegrationName: "gmail",
		ClientID:        "c",
		ClientSecret:    "s",
		RedirectURI:     "http://localhost/cb",
		Scopes:          []string{"scope"},
	})
	require.NoError(t, err)

	// Manually backdate the flow past its TTL.
	flowsMu.Lock()
	flows["gmail"].startedAt = time.Now().Add(-2 * flowTTL)
	flowsMu.Unlock()

	r := Poll("gmail")
	assert.Equal(t, "no_flow", r.Status, "expired flow should be GC'd on the next lookup")
}

func TestRefreshAccessToken_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh_token", r.FormValue("grant_type"))
		assert.Equal(t, "cid", r.FormValue("client_id"))
		assert.Equal(t, "csec", r.FormValue("client_secret"))
		assert.Equal(t, "rtok", r.FormValue("refresh_token"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "ya29.new",
			"expires_in":   3600,
		})
	}))
	defer ts.Close()
	withTokenURL(t, ts.URL)

	tok, err := RefreshAccessToken("cid", "csec", "rtok")
	require.NoError(t, err)
	assert.Equal(t, "ya29.new", tok)
}

func TestRefreshAccessToken_GoogleErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer ts.Close()
	withTokenURL(t, ts.URL)

	_, err := RefreshAccessToken("cid", "csec", "rtok")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestRefreshAccessToken_StructuredOAuthError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_grant"})
	}))
	defer ts.Close()
	withTokenURL(t, ts.URL)

	_, err := RefreshAccessToken("cid", "csec", "rtok")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_grant")
}

func TestRandomString(t *testing.T) {
	s1 := randomString(32)
	s2 := randomString(32)
	assert.Len(t, s1, 32)
	assert.Len(t, s2, 32)
	assert.NotEqual(t, s1, s2)
}

func TestPKCEChallenge_Deterministic(t *testing.T) {
	verifier := "test-verifier-12345"
	c1 := pkceChallenge(verifier)
	c2 := pkceChallenge(verifier)
	assert.NotEmpty(t, c1)
	assert.NotEqual(t, verifier, c1)
	assert.Equal(t, c1, c2)
}
