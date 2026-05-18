// Package gsheets's OAuth flow is a thin wrapper around the shared
// googleoauth package, mirroring the gmail, gcal, gdrive, and gdocs
// wrappers. The Sheets-specific bits are the scope set, the integration
// name, and the public function names exposed to web/web.go.
package gsheets

import (
	"context"

	"github.com/daltoniam/switchboard/googleoauth"
)

const (
	// integrationName matches the registry / config key for the gsheets
	// adapter. Each Google service keys its in-progress OAuth flow by
	// this name so concurrent flows don't collide.
	integrationName = "gsheets"

	// gsheetsScope grants full read/write access to the user's Google
	// Sheets spreadsheets. Drive scope is intentionally NOT requested
	// here — the gdrive integration owns that surface.
	gsheetsScope = "https://www.googleapis.com/auth/spreadsheets"
)

// OAuthStartResult is the wire shape returned by /api/gsheets/oauth/start.
type OAuthStartResult = googleoauth.StartResult

// OAuthPollResult is the wire shape returned by /api/gsheets/oauth/poll.
type OAuthPollResult = googleoauth.PollResult

// StartGsheetsOAuth begins a new OAuth + PKCE flow for the gsheets
// integration.
func StartGsheetsOAuth(clientID, clientSecret, redirectURI string) (*OAuthStartResult, error) {
	return googleoauth.Start(googleoauth.Config{
		IntegrationName: integrationName,
		ClientID:        clientID,
		ClientSecret:    clientSecret,
		RedirectURI:     redirectURI,
		Scopes:          []string{gsheetsScope},
	})
}

// HandleGsheetsCallback completes the token exchange for the in-progress flow.
func HandleGsheetsCallback(code, state string) error {
	return googleoauth.HandleCallback(integrationName, code, state)
}

// PollGsheetsOAuth reports the status of the in-progress flow.
func PollGsheetsOAuth() OAuthPollResult {
	return googleoauth.Poll(integrationName)
}

// RefreshAccessToken exchanges a refresh token for a new access token. Used
// by gsheets.go's request retry path on 401.
func RefreshAccessToken(ctx context.Context, clientID, clientSecret, refreshToken string) (string, error) {
	return googleoauth.RefreshAccessToken(ctx, clientID, clientSecret, refreshToken)
}
