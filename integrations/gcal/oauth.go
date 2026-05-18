// Package gcal's OAuth flow is a thin wrapper around the shared googleoauth
// package, mirroring the gmail wrapper. The Calendar-specific bits are the
// scope set, the integration name, and the public function names exposed to
// web/web.go.
package gcal

import (
	"context"

	"github.com/daltoniam/switchboard/googleoauth"
)

const (
	// integrationName matches the registry / config key for the gcal
	// adapter. Each Google service keys its in-progress OAuth flow by
	// this name so concurrent flows don't collide.
	integrationName = "gcal"

	// gcalScope grants full read/write access to the user's calendars
	// and events.
	gcalScope = "https://www.googleapis.com/auth/calendar"
)

// OAuthStartResult is the wire shape returned by /api/gcal/oauth/start.
type OAuthStartResult = googleoauth.StartResult

// OAuthPollResult is the wire shape returned by /api/gcal/oauth/poll.
type OAuthPollResult = googleoauth.PollResult

// StartGcalOAuth begins a new OAuth + PKCE flow for the gcal integration.
func StartGcalOAuth(clientID, clientSecret, redirectURI string) (*OAuthStartResult, error) {
	return googleoauth.Start(googleoauth.Config{
		IntegrationName: integrationName,
		ClientID:        clientID,
		ClientSecret:    clientSecret,
		RedirectURI:     redirectURI,
		Scopes:          []string{gcalScope},
	})
}

// HandleGcalCallback completes the token exchange for the in-progress flow.
func HandleGcalCallback(code, state string) error {
	return googleoauth.HandleCallback(integrationName, code, state)
}

// PollGcalOAuth reports the status of the in-progress flow.
func PollGcalOAuth() OAuthPollResult {
	return googleoauth.Poll(integrationName)
}

// RefreshAccessToken exchanges a refresh token for a new access token. Used
// by gcal.go's request retry path on 401.
func RefreshAccessToken(ctx context.Context, clientID, clientSecret, refreshToken string) (string, error) {
	return googleoauth.RefreshAccessToken(ctx, clientID, clientSecret, refreshToken)
}
