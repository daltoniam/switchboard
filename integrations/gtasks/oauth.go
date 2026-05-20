// Package gtasks's OAuth flow is a thin wrapper around the shared
// googleoauth package, mirroring the gmail, gcal, gdrive, gdocs, gsheets,
// gslides, and gforms wrappers. The Tasks-specific bits are the scope set,
// the integration name, and the public function names exposed to web/web.go.
package gtasks

import (
	"context"

	"github.com/daltoniam/switchboard/googleoauth"
)

const (
	// integrationName matches the registry / config key for the gtasks
	// adapter. Each Google service keys its in-progress OAuth flow by
	// this name so concurrent flows don't collide.
	integrationName = "gtasks"

	// gtasksScope grants full read/write access to the user's Google
	// Tasks resources (tasklists and tasks). The Tasks API does not
	// expose a separate read-only scope at the per-service level —
	// /auth/tasks.readonly exists but is broader than needed; we use
	// the read/write scope so the same token can power both list and
	// mutation tools.
	gtasksScope = "https://www.googleapis.com/auth/tasks"
)

// OAuthStartResult is the wire shape returned by /api/gtasks/oauth/start.
type OAuthStartResult = googleoauth.StartResult

// OAuthPollResult is the wire shape returned by /api/gtasks/oauth/poll.
type OAuthPollResult = googleoauth.PollResult

// StartGtasksOAuth begins a new OAuth + PKCE flow for the gtasks
// integration.
func StartGtasksOAuth(clientID, clientSecret, redirectURI string) (*OAuthStartResult, error) {
	return googleoauth.Start(googleoauth.Config{
		IntegrationName: integrationName,
		ClientID:        clientID,
		ClientSecret:    clientSecret,
		RedirectURI:     redirectURI,
		Scopes:          []string{gtasksScope},
	})
}

// HandleGtasksCallback completes the token exchange for the in-progress flow.
func HandleGtasksCallback(code, state string) error {
	return googleoauth.HandleCallback(integrationName, code, state)
}

// PollGtasksOAuth reports the status of the in-progress flow.
func PollGtasksOAuth() OAuthPollResult {
	return googleoauth.Poll(integrationName)
}

// RefreshAccessToken exchanges a refresh token for a new access token. Used
// by gtasks.go's request retry path on 401.
func RefreshAccessToken(ctx context.Context, clientID, clientSecret, refreshToken string) (string, error) {
	return googleoauth.RefreshAccessToken(ctx, clientID, clientSecret, refreshToken)
}
