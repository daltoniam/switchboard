// Package gmeet's OAuth flow is a thin wrapper around the shared
// googleoauth package, mirroring the gmail, gcal, gdrive, gdocs, gsheets,
// gslides, gforms, gtasks, gchat, and gpeople wrappers. The Meet-specific
// bits are the scope set, the integration name, and the public function
// names exposed to web/web.go.
package gmeet

import (
	"context"

	"github.com/daltoniam/switchboard/googleoauth"
)

const (
	// integrationName matches the registry / config key for the gmeet
	// adapter. Each Google service keys its in-progress OAuth flow by
	// this name so concurrent flows don't collide.
	integrationName = "gmeet"

	// meetingsCreatedScope lets the app create + manage spaces it owns
	// and read conference records for those spaces. This is the default
	// scope a user-facing meet integration needs.
	meetingsCreatedScope = "https://www.googleapis.com/auth/meetings.space.created"

	// meetingsReadonlyScope grants read access to all spaces and
	// conference records the user has access to (not just ones created
	// by this app). Needed for listing past meetings the user attended.
	meetingsReadonlyScope = "https://www.googleapis.com/auth/meetings.space.readonly"

	// meetingsSettingsScope grants permission to update a space's
	// configuration (access type, moderation, etc.). Without it,
	// gmeet_update_space returns 403.
	meetingsSettingsScope = "https://www.googleapis.com/auth/meetings.space.settings"
)

// OAuthStartResult is the wire shape returned by /api/gmeet/oauth/start.
type OAuthStartResult = googleoauth.StartResult

// OAuthPollResult is the wire shape returned by /api/gmeet/oauth/poll.
type OAuthPollResult = googleoauth.PollResult

// StartGmeetOAuth begins a new OAuth + PKCE flow for the gmeet integration.
func StartGmeetOAuth(clientID, clientSecret, redirectURI string) (*OAuthStartResult, error) {
	return googleoauth.Start(googleoauth.Config{
		IntegrationName: integrationName,
		ClientID:        clientID,
		ClientSecret:    clientSecret,
		RedirectURI:     redirectURI,
		Scopes:          []string{meetingsCreatedScope, meetingsReadonlyScope, meetingsSettingsScope},
	})
}

// HandleGmeetCallback completes the token exchange for the in-progress flow.
func HandleGmeetCallback(code, state string) error {
	return googleoauth.HandleCallback(integrationName, code, state)
}

// PollGmeetOAuth reports the status of the in-progress flow.
func PollGmeetOAuth() OAuthPollResult {
	return googleoauth.Poll(integrationName)
}

// RefreshAccessToken exchanges a refresh token for a new access token. Used
// by gmeet.go's request retry path on 401.
func RefreshAccessToken(ctx context.Context, clientID, clientSecret, refreshToken string) (string, error) {
	return googleoauth.RefreshAccessToken(ctx, clientID, clientSecret, refreshToken)
}
