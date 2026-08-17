// Package gdrive's OAuth flow is a thin wrapper around the shared
// googleoauth package, mirroring the gmail and gcal wrappers. The
// Drive-specific bits are the scope set, the integration name, and the
// public function names exposed to web/web.go.
package gdrive

import (
	"context"

	"github.com/daltoniam/switchboard/googleoauth"
)

const (
	// integrationName matches the registry / config key for the gdrive
	// adapter. Each Google service keys its in-progress OAuth flow by
	// this name so concurrent flows don't collide.
	integrationName = "gdrive"
)

// OAuthStartResult is the wire shape returned by /api/gdrive/oauth/start.
type OAuthStartResult = googleoauth.StartResult

// OAuthPollResult is the wire shape returned by /api/gdrive/oauth/poll.
type OAuthPollResult = googleoauth.PollResult

// StartGdriveOAuth begins a new OAuth + PKCE flow for the gdrive integration.
func StartGdriveOAuth(clientID, clientSecret, redirectURI string) (*OAuthStartResult, error) {
	return googleoauth.Start(googleoauth.Config{
		IntegrationName: integrationName,
		ClientID:        clientID,
		ClientSecret:    clientSecret,
		RedirectURI:     redirectURI,
		Scopes:          googleoauth.ScopesFor(integrationName),
	})
}

// HandleGdriveCallback completes the token exchange for the in-progress flow.
func HandleGdriveCallback(code, state string) error {
	return googleoauth.HandleCallback(integrationName, code, state)
}

// PollGdriveOAuth reports the status of the in-progress flow.
func PollGdriveOAuth() OAuthPollResult {
	return googleoauth.Poll(integrationName)
}

// RefreshAccessToken exchanges a refresh token for a new access token. Used
// by gdrive.go's request retry path on 401.
func RefreshAccessToken(ctx context.Context, clientID, clientSecret, refreshToken string) (string, error) {
	return googleoauth.RefreshAccessToken(ctx, clientID, clientSecret, refreshToken)
}
