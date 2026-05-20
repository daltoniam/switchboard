package gdrive

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daltoniam/switchboard/googleoauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The OAuth flow itself is exercised in googleoauth/oauth_test.go. Tests
// here verify the gdrive-specific wiring: integration name (so flows don't
// collide with sibling Google integrations) and the drive scope.

func TestStartGdriveOAuth_Success(t *testing.T) {
	googleoauth.Reset()

	result, err := StartGdriveOAuth("client-id", "client-secret", "http://localhost:3847/callback")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.AuthorizeURL)
	assert.Contains(t, result.AuthorizeURL, "accounts.google.com")
	assert.Contains(t, result.AuthorizeURL, "client_id=client-id")
	// gdrive-specific: the drive scope is requested.
	assert.Contains(t, result.AuthorizeURL, "auth%2Fdrive")
}

func TestStartGdriveOAuth_MissingClientID(t *testing.T) {
	googleoauth.Reset()
	_, err := StartGdriveOAuth("", "client-secret", "http://localhost:3847/callback")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client_id")
}

func TestStartGdriveOAuth_MissingClientSecret(t *testing.T) {
	googleoauth.Reset()
	_, err := StartGdriveOAuth("client-id", "", "http://localhost:3847/callback")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client_secret")
}

func TestPollGdriveOAuth_NoFlow(t *testing.T) {
	googleoauth.Reset()
	result := PollGdriveOAuth()
	assert.Equal(t, "no_flow", result.Status)
	assert.NotEmpty(t, result.Error)
}

func TestPollGdriveOAuth_Pending(t *testing.T) {
	googleoauth.Reset()
	_, err := StartGdriveOAuth("client-id", "client-secret", "http://localhost:3847/callback")
	require.NoError(t, err)
	assert.Equal(t, "pending", PollGdriveOAuth().Status)
}

func TestHandleGdriveCallback_NoFlow(t *testing.T) {
	googleoauth.Reset()
	err := HandleGdriveCallback("code", "state")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no OAuth flow in progress")
}

func TestHandleGdriveCallback_InvalidState(t *testing.T) {
	googleoauth.Reset()
	_, err := StartGdriveOAuth("client-id", "client-secret", "http://localhost:3847/callback")
	require.NoError(t, err)

	err = HandleGdriveCallback("code", "wrong-state")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid state parameter")
	assert.Equal(t, "error", PollGdriveOAuth().Status)
}

// TestTokenRefreshOnUnauthorized exercises gdrive.go's doRequestInner retry
// path: a 401 triggers a refresh attempt. The refresh request goes to
// Google's real token endpoint and fails in tests, so the original 401
// surfaces. Guards the wiring between the HTTP layer and RefreshAccessToken.
func TestTokenRefreshOnUnauthorized(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":{"message":"Token expired"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"files":[]}`))
	}))
	defer ts.Close()

	g := &gdrive{
		accessToken:  "expired-token",
		refreshToken: "refresh",
		clientID:     "cid",
		clientSecret: "csec",
		client:       ts.Client(),
		baseURL:      ts.URL,
	}

	_, err := g.get(t.Context(), "/test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}
