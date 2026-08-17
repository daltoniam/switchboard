package x

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetOAuthState() {
	activeOAuth.mu.Lock()
	activeOAuth.state = nil
	activeOAuth.mu.Unlock()
}

func TestStartXOAuth_Success(t *testing.T) {
	resetOAuthState()
	result, err := StartXOAuth("client-id", "client-secret", "http://localhost:8080/callback")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.AuthorizeURL, "client_id=client-id")
	assert.Contains(t, result.AuthorizeURL, "code_challenge_method=S256")
	assert.Contains(t, result.AuthorizeURL, "response_type=code")

	s := getActiveOAuth()
	require.NotNil(t, s)
	assert.Equal(t, "client-id", s.clientID)
	assert.Equal(t, "client-secret", s.clientSecret)
	assert.Equal(t, xTokenURL, s.tokenURL)
}

func TestStartXOAuth_MissingClientID(t *testing.T) {
	resetOAuthState()
	_, err := StartXOAuth("", "secret", "http://localhost/callback")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client_id")
}

func TestHandleXCallback_NoActiveFlow(t *testing.T) {
	resetOAuthState()
	err := HandleXCallback("code", "state")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no OAuth flow")
}

func TestHandleXCallback_InvalidState(t *testing.T) {
	resetOAuthState()
	_, err := StartXOAuth("client-id", "secret", "http://localhost/callback")
	require.NoError(t, err)

	err = HandleXCallback("code", "wrong-state")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CSRF")

	result := PollXOAuth()
	assert.Equal(t, "error", result.Status)
	assert.Contains(t, result.Error, "CSRF")
}

func TestHandleXCallback_TokenExchange(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		err := r.ParseForm()
		require.NoError(t, err)
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.NotEmpty(t, r.Form.Get("code"))
		assert.NotEmpty(t, r.Form.Get("code_verifier"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "test-access-token",
			"refresh_token": "test-refresh-token",
			"token_type":    "bearer",
			"expires_in":    7200,
			"scope":         "tweet.read",
		})
	}))
	defer ts.Close()

	resetOAuthState()
	_, err := StartXOAuth("client-id", "client-secret", "http://localhost/callback")
	require.NoError(t, err)

	s := getActiveOAuth()
	require.NotNil(t, s)
	s.mu.Lock()
	s.tokenURL = ts.URL
	state := s.state
	s.mu.Unlock()

	err = HandleXCallback("auth-code", state)
	require.NoError(t, err)

	result := PollXOAuth()
	assert.Equal(t, "complete", result.Status)
	assert.Equal(t, "test-access-token", result.AccessToken)
	assert.Equal(t, "test-refresh-token", result.RefreshToken)
}

func TestHandleXCallback_TokenExchangeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer ts.Close()

	resetOAuthState()
	_, err := StartXOAuth("client-id", "secret", "http://localhost/callback")
	require.NoError(t, err)

	s := getActiveOAuth()
	require.NotNil(t, s)
	s.mu.Lock()
	s.tokenURL = ts.URL
	state := s.state
	s.mu.Unlock()

	err = HandleXCallback("bad-code", state)
	assert.Error(t, err)

	result := PollXOAuth()
	assert.Equal(t, "error", result.Status)
	assert.Contains(t, result.Error, "400")
}

func TestHandleXCallback_OAuthError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"error": "access_denied",
		})
	}))
	defer ts.Close()

	resetOAuthState()
	_, err := StartXOAuth("client-id", "secret", "http://localhost/callback")
	require.NoError(t, err)

	s := getActiveOAuth()
	s.mu.Lock()
	s.tokenURL = ts.URL
	state := s.state
	s.mu.Unlock()

	err = HandleXCallback("code", state)
	assert.Error(t, err)

	result := PollXOAuth()
	assert.Equal(t, "error", result.Status)
	assert.Contains(t, result.Error, "access_denied")
}

func TestPollXOAuth_NoFlow(t *testing.T) {
	resetOAuthState()
	result := PollXOAuth()
	assert.Equal(t, "no_flow", result.Status)
}

func TestPollXOAuth_Pending(t *testing.T) {
	resetOAuthState()
	_, err := StartXOAuth("client-id", "secret", "http://localhost/callback")
	require.NoError(t, err)

	result := PollXOAuth()
	assert.Equal(t, "pending", result.Status)
}

func TestRefreshAccessToken_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		err := r.ParseForm()
		require.NoError(t, err)
		assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		assert.Equal(t, "old-refresh", r.Form.Get("refresh_token"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "new-access-token",
			"expires_in":   7200,
		})
	}))
	defer ts.Close()

	token, err := RefreshAccessToken("client-id", "secret", "old-refresh", ts.URL)
	require.NoError(t, err)
	assert.Equal(t, "new-access-token", token)
}

func TestRefreshAccessToken_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer ts.Close()

	_, err := RefreshAccessToken("client-id", "secret", "expired-token", ts.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestOAuthPKCEChallenge(t *testing.T) {
	verifier := "test-verifier-string"
	challenge := oauthPKCEChallenge(verifier)
	assert.NotEmpty(t, challenge)
	assert.NotEqual(t, verifier, challenge)

	challenge2 := oauthPKCEChallenge(verifier)
	assert.Equal(t, challenge, challenge2)
}

func TestOAuthRandomString(t *testing.T) {
	s1 := oauthRandomString(32)
	s2 := oauthRandomString(32)
	assert.Len(t, s1, 32)
	assert.Len(t, s2, 32)
	assert.NotEqual(t, s1, s2)
}

func TestHandleXCallback_BasicAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "client-id", user)
		assert.Equal(t, "client-secret", pass)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "tok",
			"token_type":   "bearer",
		})
	}))
	defer ts.Close()

	resetOAuthState()
	_, err := StartXOAuth("client-id", "client-secret", "http://localhost/callback")
	require.NoError(t, err)

	s := getActiveOAuth()
	s.mu.Lock()
	s.tokenURL = ts.URL
	state := s.state
	s.mu.Unlock()

	err = HandleXCallback("code", state)
	require.NoError(t, err)
}
