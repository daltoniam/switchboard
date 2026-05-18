// Package gdocs's OAuth flow is a thin wrapper around the shared
// googleoauth package, mirroring the gmail, gcal, and gdrive wrappers. The
// Docs-specific bits are the scope set, the integration name, and the
// public function names exposed to web/web.go.
package gdocs

import "github.com/daltoniam/switchboard/googleoauth"

const (
	// integrationName matches the registry / config key for the gdocs
	// adapter. Each Google service keys its in-progress OAuth flow by
	// this name so concurrent flows don't collide.
	integrationName = "gdocs"

	// gdocsScope grants full read/write access to the user's Google Docs
	// documents. Drive scope is intentionally NOT requested here — the
	// gdrive integration owns that surface.
	gdocsScope = "https://www.googleapis.com/auth/documents"
)

// OAuthStartResult is the wire shape returned by /api/gdocs/oauth/start.
type OAuthStartResult = googleoauth.StartResult

// OAuthPollResult is the wire shape returned by /api/gdocs/oauth/poll.
type OAuthPollResult = googleoauth.PollResult

// StartGdocsOAuth begins a new OAuth + PKCE flow for the gdocs integration.
func StartGdocsOAuth(clientID, clientSecret, redirectURI string) (*OAuthStartResult, error) {
	return googleoauth.Start(googleoauth.Config{
		IntegrationName: integrationName,
		ClientID:        clientID,
		ClientSecret:    clientSecret,
		RedirectURI:     redirectURI,
		Scopes:          []string{gdocsScope},
	})
}

// HandleGdocsCallback completes the token exchange for the in-progress flow.
func HandleGdocsCallback(code, state string) error {
	return googleoauth.HandleCallback(integrationName, code, state)
}

// PollGdocsOAuth reports the status of the in-progress flow.
func PollGdocsOAuth() OAuthPollResult {
	return googleoauth.Poll(integrationName)
}

// RefreshAccessToken exchanges a refresh token for a new access token. Used
// by gdocs.go's request retry path on 401.
func RefreshAccessToken(clientID, clientSecret, refreshToken string) (string, error) {
	return googleoauth.RefreshAccessToken(clientID, clientSecret, refreshToken)
}
