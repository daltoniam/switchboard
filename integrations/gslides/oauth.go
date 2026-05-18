// Package gslides's OAuth flow is a thin wrapper around the shared
// googleoauth package, mirroring the gmail, gcal, gdrive, gdocs, and
// gsheets wrappers. The Slides-specific bits are the scope set, the
// integration name, and the public function names exposed to web/web.go.
package gslides

import "github.com/daltoniam/switchboard/googleoauth"

const (
	// integrationName matches the registry / config key for the gslides
	// adapter. Each Google service keys its in-progress OAuth flow by
	// this name so concurrent flows don't collide.
	integrationName = "gslides"

	// gslidesScope grants full read/write access to the user's Google
	// Slides presentations. Drive scope is intentionally NOT requested
	// here — the gdrive integration owns that surface.
	gslidesScope = "https://www.googleapis.com/auth/presentations"
)

// OAuthStartResult is the wire shape returned by /api/gslides/oauth/start.
type OAuthStartResult = googleoauth.StartResult

// OAuthPollResult is the wire shape returned by /api/gslides/oauth/poll.
type OAuthPollResult = googleoauth.PollResult

// StartGslidesOAuth begins a new OAuth + PKCE flow for the gslides
// integration.
func StartGslidesOAuth(clientID, clientSecret, redirectURI string) (*OAuthStartResult, error) {
	return googleoauth.Start(googleoauth.Config{
		IntegrationName: integrationName,
		ClientID:        clientID,
		ClientSecret:    clientSecret,
		RedirectURI:     redirectURI,
		Scopes:          []string{gslidesScope},
	})
}

// HandleGslidesCallback completes the token exchange for the in-progress flow.
func HandleGslidesCallback(code, state string) error {
	return googleoauth.HandleCallback(integrationName, code, state)
}

// PollGslidesOAuth reports the status of the in-progress flow.
func PollGslidesOAuth() OAuthPollResult {
	return googleoauth.Poll(integrationName)
}

// RefreshAccessToken exchanges a refresh token for a new access token. Used
// by gslides.go's request retry path on 401.
func RefreshAccessToken(clientID, clientSecret, refreshToken string) (string, error) {
	return googleoauth.RefreshAccessToken(clientID, clientSecret, refreshToken)
}
