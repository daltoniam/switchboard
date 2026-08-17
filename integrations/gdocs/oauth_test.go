package gdocs

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daltoniam/switchboard/googleoauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The OAuth flow itself is exercised in googleoauth/oauth_test.go. Tests
// here verify the gdocs-specific wiring: integration name (so flows don't
// collide with sibling Google integrations) and the documents scope.

func TestStartGdocsOAuth_Success(t *testing.T) {
	googleoauth.Reset()

	result, err := StartGdocsOAuth("client-id", "client-secret", "http://localhost:3847/callback")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.AuthorizeURL)
	assert.Contains(t, result.AuthorizeURL, "accounts.google.com")
	assert.Contains(t, result.AuthorizeURL, "client_id=client-id")
	// gdocs-specific: the documents scope is requested.
	assert.Contains(t, result.AuthorizeURL, "auth%2Fdocuments")
}

func TestStartGdocsOAuth_MissingClientID(t *testing.T) {
	googleoauth.Reset()
	_, err := StartGdocsOAuth("", "client-secret", "http://localhost:3847/callback")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client_id")
}

func TestStartGdocsOAuth_MissingClientSecret(t *testing.T) {
	googleoauth.Reset()
	_, err := StartGdocsOAuth("client-id", "", "http://localhost:3847/callback")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client_secret")
}

func TestPollGdocsOAuth_NoFlow(t *testing.T) {
	googleoauth.Reset()
	result := PollGdocsOAuth()
	assert.Equal(t, "no_flow", result.Status)
	assert.NotEmpty(t, result.Error)
}

func TestPollGdocsOAuth_Pending(t *testing.T) {
	googleoauth.Reset()
	_, err := StartGdocsOAuth("client-id", "client-secret", "http://localhost:3847/callback")
	require.NoError(t, err)
	assert.Equal(t, "pending", PollGdocsOAuth().Status)
}

func TestHandleGdocsCallback_NoFlow(t *testing.T) {
	googleoauth.Reset()
	err := HandleGdocsCallback("code", "state")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no OAuth flow in progress")
}

func TestHandleGdocsCallback_InvalidState(t *testing.T) {
	googleoauth.Reset()
	_, err := StartGdocsOAuth("client-id", "client-secret", "http://localhost:3847/callback")
	require.NoError(t, err)

	err = HandleGdocsCallback("code", "wrong-state")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid state parameter")
	assert.Equal(t, "error", PollGdocsOAuth().Status)
}

// TestTokenRefreshOnUnauthorized exercises gdocs.go's doRequestInner retry
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
		_, _ = w.Write([]byte(`{"documentId":"x"}`))
	}))
	defer ts.Close()

	g := &gdocs{
		accessToken:  "expired-token",
		refreshToken: "refresh",
		clientID:     "cid",
		clientSecret: "csec",
		client:       ts.Client(),
		baseURL:      ts.URL,
	}

	_, err := g.get(t.Context(), "/documents/x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}
