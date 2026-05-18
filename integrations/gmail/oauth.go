// Package gmail's OAuth flow is a thin wrapper around the shared googleoauth
// package. Gmail-specific bits live here: the scope set, the integration
// name, and the public function names that web/web.go has historically
// called.
package gmail

import (
	"context"

	"github.com/daltoniam/switchboard/googleoauth"
)

const (
	// integrationName matches the registry / config key for the gmail
	// adapter. It's used to key the in-progress OAuth flow so flows for
	// different Google services don't collide.
	integrationName = "gmail"

	// gmailScope grants full read/write/send access to the user's mail.
	gmailScope = "https://mail.google.com/"
)

// OAuthStartResult is the wire shape returned by /api/gmail/oauth/start.
// Kept as an alias for backward compatibility with existing callers.
type OAuthStartResult = googleoauth.StartResult

// OAuthPollResult is the wire shape returned by /api/gmail/oauth/poll.
type OAuthPollResult = googleoauth.PollResult

// StartGmailOAuth begins a new OAuth + PKCE flow for the gmail integration.
func StartGmailOAuth(clientID, clientSecret, redirectURI string) (*OAuthStartResult, error) {
	return googleoauth.Start(googleoauth.Config{
		IntegrationName: integrationName,
		ClientID:        clientID,
		ClientSecret:    clientSecret,
		RedirectURI:     redirectURI,
		Scopes:          []string{gmailScope},
	})
}

// HandleGmailCallback completes the token exchange for the in-progress flow.
func HandleGmailCallback(code, state string) error {
	return googleoauth.HandleCallback(integrationName, code, state)
}

// PollGmailOAuth reports the status of the in-progress flow.
func PollGmailOAuth() OAuthPollResult {
	return googleoauth.Poll(integrationName)
}

// RefreshAccessToken exchanges a refresh token for a new access token. Used
// by gmail.go's request retry path on 401.
func RefreshAccessToken(ctx context.Context, clientID, clientSecret, refreshToken string) (string, error) {
	return googleoauth.RefreshAccessToken(ctx, clientID, clientSecret, refreshToken)
}
