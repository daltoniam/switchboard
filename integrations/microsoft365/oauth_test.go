package microsoft365

import (
	"context"
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

func TestStartM365OAuth_Success(t *testing.T) {
	resetOAuthState()
	result, err := StartM365OAuth("client-id", "client-secret", "http://localhost:8080/callback", "common")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.AuthorizeURL, "client_id=client-id")
	assert.Contains(t, result.AuthorizeURL, "code_challenge_method=S256")
	assert.Contains(t, result.AuthorizeURL, "login.microsoftonline.com/common")
	assert.Contains(t, result.AuthorizeURL, "User.Read.All")
	assert.Contains(t, msDefaultScope, "User.Read.All")

	s := getActiveOAuth()
	require.NotNil(t, s)
	assert.Equal(t, "client-id", s.clientID)
	assert.Equal(t, "client-secret", s.clientSecret)
}

func TestStartM365OAuth_MissingClientID(t *testing.T) {
	resetOAuthState()
	_, err := StartM365OAuth("", "secret", "http://localhost/callback", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client_id")
}

func TestHandleM365Callback_NoActiveFlow(t *testing.T) {
	resetOAuthState()
	err := HandleM365Callback(context.Background(), "code", "state")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no OAuth flow")
}

func TestHandleM365Callback_InvalidState(t *testing.T) {
	resetOAuthState()
	_, err := StartM365OAuth("client-id", "secret", "http://localhost/callback", "common")
	require.NoError(t, err)

	err = HandleM365Callback(context.Background(), "code", "wrong-state")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CSRF")

	result := PollM365OAuth()
	assert.Equal(t, "error", result.Status)
	assert.Contains(t, result.Error, "CSRF")
}

func TestHandleM365Callback_TokenExchange(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		err := r.ParseForm()
		require.NoError(t, err)
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "test-access-token",
			"refresh_token": "test-refresh-token",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	defer ts.Close()

	resetOAuthState()
	_, err := StartM365OAuth("client-id", "secret", "http://localhost/callback", "common")
	require.NoError(t, err)
	s := getActiveOAuth()
	require.NotNil(t, s)
	s.tokenURL = ts.URL

	err = HandleM365Callback(context.Background(), "auth-code", s.state)
	require.NoError(t, err)

	result := PollM365OAuth()
	assert.Equal(t, "complete", result.Status)
	assert.Equal(t, "test-access-token", result.AccessToken)
	assert.Equal(t, "test-refresh-token", result.RefreshToken)
}
