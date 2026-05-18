// Package gchat's OAuth flow is a thin wrapper around the shared
// googleoauth package, mirroring the gmail, gcal, gdrive, gdocs, gsheets,
// gslides, gforms, and gtasks wrappers. The Chat-specific bits are the
// scope set, the integration name, and the public function names exposed
// to web/web.go.
package gchat

import "github.com/daltoniam/switchboard/googleoauth"

const (
	// integrationName matches the registry / config key for the gchat
	// adapter. Each Google service keys its in-progress OAuth flow by
	// this name so concurrent flows don't collide.
	integrationName = "gchat"

	// gchatSpacesScope grants read-only access to the Chat spaces the
	// user belongs to (rooms, group DMs, direct messages). Needed to
	// enumerate spaces and list members.
	gchatSpacesScope = "https://www.googleapis.com/auth/chat.spaces.readonly"

	// gchatMessagesScope grants read/write access to messages within
	// spaces the user belongs to. Covers list/get/create/update/delete.
	gchatMessagesScope = "https://www.googleapis.com/auth/chat.messages"
)

// OAuthStartResult is the wire shape returned by /api/gchat/oauth/start.
type OAuthStartResult = googleoauth.StartResult

// OAuthPollResult is the wire shape returned by /api/gchat/oauth/poll.
type OAuthPollResult = googleoauth.PollResult

// StartGchatOAuth begins a new OAuth + PKCE flow for the gchat integration.
func StartGchatOAuth(clientID, clientSecret, redirectURI string) (*OAuthStartResult, error) {
	return googleoauth.Start(googleoauth.Config{
		IntegrationName: integrationName,
		ClientID:        clientID,
		ClientSecret:    clientSecret,
		RedirectURI:     redirectURI,
		Scopes:          []string{gchatSpacesScope, gchatMessagesScope},
	})
}

// HandleGchatCallback completes the token exchange for the in-progress flow.
func HandleGchatCallback(code, state string) error {
	return googleoauth.HandleCallback(integrationName, code, state)
}

// PollGchatOAuth reports the status of the in-progress flow.
func PollGchatOAuth() OAuthPollResult {
	return googleoauth.Poll(integrationName)
}

// RefreshAccessToken exchanges a refresh token for a new access token. Used
// by gchat.go's request retry path on 401.
func RefreshAccessToken(clientID, clientSecret, refreshToken string) (string, error) {
	return googleoauth.RefreshAccessToken(clientID, clientSecret, refreshToken)
}
