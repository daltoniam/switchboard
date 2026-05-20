// Package gpeople's OAuth flow is a thin wrapper around the shared
// googleoauth package, mirroring the gmail, gcal, gdrive, gdocs, gsheets,
// gslides, gforms, gtasks, and gchat wrappers. The People-specific bits
// are the scope set, the integration name, and the public function names
// exposed to web/web.go.
package gpeople

import (
	"context"

	"github.com/daltoniam/switchboard/googleoauth"
)

const (
	// integrationName matches the registry / config key for the gpeople
	// adapter. Each Google service keys its in-progress OAuth flow by
	// this name so concurrent flows don't collide.
	integrationName = "gpeople"

	// contactsScope grants full read/write access to the authenticated
	// user's Google Contacts (the /people/me/connections and the create/
	// update/delete contact endpoints).
	contactsScope = "https://www.googleapis.com/auth/contacts"

	// otherContactsScope grants read access to "Other contacts" — auto-
	// collected contacts the user has emailed but never explicitly saved.
	otherContactsScope = "https://www.googleapis.com/auth/contacts.other.readonly"

	// directoryScope grants read access to the user's Google Workspace
	// directory (coworkers in the same organization). Without it,
	// listDirectoryPeople / searchDirectoryPeople return 403.
	directoryScope = "https://www.googleapis.com/auth/directory.readonly"
)

// OAuthStartResult is the wire shape returned by /api/gpeople/oauth/start.
type OAuthStartResult = googleoauth.StartResult

// OAuthPollResult is the wire shape returned by /api/gpeople/oauth/poll.
type OAuthPollResult = googleoauth.PollResult

// StartGpeopleOAuth begins a new OAuth + PKCE flow for the gpeople
// integration.
func StartGpeopleOAuth(clientID, clientSecret, redirectURI string) (*OAuthStartResult, error) {
	return googleoauth.Start(googleoauth.Config{
		IntegrationName: integrationName,
		ClientID:        clientID,
		ClientSecret:    clientSecret,
		RedirectURI:     redirectURI,
		Scopes:          []string{contactsScope, otherContactsScope, directoryScope},
	})
}

// HandleGpeopleCallback completes the token exchange for the in-progress flow.
func HandleGpeopleCallback(code, state string) error {
	return googleoauth.HandleCallback(integrationName, code, state)
}

// PollGpeopleOAuth reports the status of the in-progress flow.
func PollGpeopleOAuth() OAuthPollResult {
	return googleoauth.Poll(integrationName)
}

// RefreshAccessToken exchanges a refresh token for a new access token. Used
// by gpeople.go's request retry path on 401.
func RefreshAccessToken(ctx context.Context, clientID, clientSecret, refreshToken string) (string, error) {
	return googleoauth.RefreshAccessToken(ctx, clientID, clientSecret, refreshToken)
}
