package gslides

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daltoniam/switchboard/googleoauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The OAuth flow itself is exercised in googleoauth/oauth_test.go. Tests
// here verify the gslides-specific wiring: integration name (so flows don't
// collide with sibling Google integrations) and the presentations scope.

func TestStartGslidesOAuth_Success(t *testing.T) {
	googleoauth.Reset()

	result, err := StartGslidesOAuth("client-id", "client-secret", "http://localhost:3847/callback")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.AuthorizeURL)
	assert.Contains(t, result.AuthorizeURL, "accounts.google.com")
	assert.Contains(t, result.AuthorizeURL, "client_id=client-id")
	// gslides-specific: the presentations scope is requested.
	assert.Contains(t, result.AuthorizeURL, "auth%2Fpresentations")
}

func TestStartGslidesOAuth_MissingClientID(t *testing.T) {
	googleoauth.Reset()
	_, err := StartGslidesOAuth("", "client-secret", "http://localhost:3847/callback")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client_id")
}

func TestStartGslidesOAuth_MissingClientSecret(t *testing.T) {
	googleoauth.Reset()
	_, err := StartGslidesOAuth("client-id", "", "http://localhost:3847/callback")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client_secret")
}

func TestPollGslidesOAuth_NoFlow(t *testing.T) {
	googleoauth.Reset()
	result := PollGslidesOAuth()
	assert.Equal(t, "no_flow", result.Status)
	assert.NotEmpty(t, result.Error)
}

func TestPollGslidesOAuth_Pending(t *testing.T) {
	googleoauth.Reset()
	_, err := StartGslidesOAuth("client-id", "client-secret", "http://localhost:3847/callback")
	require.NoError(t, err)
	assert.Equal(t, "pending", PollGslidesOAuth().Status)
}

func TestHandleGslidesCallback_NoFlow(t *testing.T) {
	googleoauth.Reset()
	err := HandleGslidesCallback("code", "state")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no OAuth flow in progress")
}

func TestHandleGslidesCallback_InvalidState(t *testing.T) {
	googleoauth.Reset()
	_, err := StartGslidesOAuth("client-id", "client-secret", "http://localhost:3847/callback")
	require.NoError(t, err)

	err = HandleGslidesCallback("code", "wrong-state")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid state parameter")
	assert.Equal(t, "error", PollGslidesOAuth().Status)
}

// TestTokenRefreshOnUnauthorized exercises gslides.go's doRequestInner retry
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
		_, _ = w.Write([]byte(`{"presentationId":"x"}`))
	}))
	defer ts.Close()

	g := &gslides{
		accessToken:  "expired-token",
		refreshToken: "refresh",
		clientID:     "cid",
		clientSecret: "csec",
		client:       ts.Client(),
		baseURL:      ts.URL,
	}

	_, err := g.get(t.Context(), "/presentations/x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}
