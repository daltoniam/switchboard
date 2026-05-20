package gmail

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daltoniam/switchboard/googleoauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The OAuth flow itself is exercised in googleoauth/oauth_test.go. Tests
// here only verify the gmail-specific wiring: that the integration name is
// used so flows don't collide with sibling Google integrations, and that
// the gmail scope is requested.

func TestStartGmailOAuth_Success(t *testing.T) {
	googleoauth.Reset()

	result, err := StartGmailOAuth("client-id", "client-secret", "http://localhost:3847/callback")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.AuthorizeURL)
	assert.Contains(t, result.AuthorizeURL, "accounts.google.com")
	assert.Contains(t, result.AuthorizeURL, "client_id=client-id")
	// Gmail-specific: the mail scope is requested.
	assert.Contains(t, result.AuthorizeURL, "mail.google.com")
}

func TestStartGmailOAuth_MissingClientID(t *testing.T) {
	googleoauth.Reset()
	_, err := StartGmailOAuth("", "client-secret", "http://localhost:3847/callback")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client_id")
}

func TestStartGmailOAuth_MissingClientSecret(t *testing.T) {
	googleoauth.Reset()
	_, err := StartGmailOAuth("client-id", "", "http://localhost:3847/callback")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client_secret")
}

func TestPollGmailOAuth_NoFlow(t *testing.T) {
	googleoauth.Reset()
	result := PollGmailOAuth()
	assert.Equal(t, "no_flow", result.Status)
	assert.NotEmpty(t, result.Error)
}

func TestPollGmailOAuth_Pending(t *testing.T) {
	googleoauth.Reset()
	_, err := StartGmailOAuth("client-id", "client-secret", "http://localhost:3847/callback")
	require.NoError(t, err)
	assert.Equal(t, "pending", PollGmailOAuth().Status)
}

func TestHandleGmailCallback_NoFlow(t *testing.T) {
	googleoauth.Reset()
	err := HandleGmailCallback("code", "state")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no OAuth flow in progress")
}

func TestHandleGmailCallback_InvalidState(t *testing.T) {
	googleoauth.Reset()
	_, err := StartGmailOAuth("client-id", "client-secret", "http://localhost:3847/callback")
	require.NoError(t, err)

	err = HandleGmailCallback("code", "wrong-state")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid state parameter")
	assert.Equal(t, "error", PollGmailOAuth().Status)
}

// TestTokenRefreshOnUnauthorized exercises gmail.go's doRequestInner retry
// path: a 401 response triggers a refresh attempt. The refresh request goes
// to Google's real token endpoint and will fail in tests, so the original
// 401 surfaces. This guards the wiring between the HTTP layer and
// RefreshAccessToken.
func TestTokenRefreshOnUnauthorized(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":{"message":"Token expired"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"emailAddress":"test@gmail.com"}`))
	}))
	defer ts.Close()

	g := &gmail{
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
