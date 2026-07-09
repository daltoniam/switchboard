// Package gforms's OAuth flow is a thin wrapper around the shared
// googleoauth package, mirroring the gmail, gcal, gdrive, gdocs, gsheets,
// and gslides wrappers. The Forms-specific bits are the scope set, the
// integration name, and the public function names exposed to web/web.go.
package gforms

import (
	"context"

	"github.com/daltoniam/switchboard/googleoauth"
)

const (
	// integrationName matches the registry / config key for the gforms
	// adapter. Each Google service keys its in-progress OAuth flow by
	// this name so concurrent flows don't collide.
	integrationName = "gforms"
)

// OAuthStartResult is the wire shape returned by /api/gforms/oauth/start.
type OAuthStartResult = googleoauth.StartResult

// OAuthPollResult is the wire shape returned by /api/gforms/oauth/poll.
type OAuthPollResult = googleoauth.PollResult

// StartGformsOAuth begins a new OAuth + PKCE flow for the gforms
// integration.
func StartGformsOAuth(clientID, clientSecret, redirectURI string) (*OAuthStartResult, error) {
	return googleoauth.Start(googleoauth.Config{
		IntegrationName: integrationName,
		ClientID:        clientID,
		ClientSecret:    clientSecret,
		RedirectURI:     redirectURI,
		Scopes:          googleoauth.ScopesFor(integrationName),
	})
}

// HandleGformsCallback completes the token exchange for the in-progress flow.
func HandleGformsCallback(code, state string) error {
	return googleoauth.HandleCallback(integrationName, code, state)
}

// PollGformsOAuth reports the status of the in-progress flow.
func PollGformsOAuth() OAuthPollResult {
	return googleoauth.Poll(integrationName)
}

// RefreshAccessToken exchanges a refresh token for a new access token. Used
// by gforms.go's request retry path on 401.
func RefreshAccessToken(ctx context.Context, clientID, clientSecret, refreshToken string) (string, error) {
	return googleoauth.RefreshAccessToken(ctx, clientID, clientSecret, refreshToken)
}
