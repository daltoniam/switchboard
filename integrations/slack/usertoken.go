package slack

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/slack-go/slack"
)

var errNotUserToken = errors.New("only Slack user tokens (xoxp-…) are accepted here")

// VerifiedUserToken is a user OAuth token that Slack's auth.test accepted,
// with the workspace it belongs to. One value carries all three fields from
// verify to save, so a caller cannot swap team ID, name and token.
type VerifiedUserToken struct {
	TeamID   string
	TeamName string
	Token    string
}

// VerifyUserToken checks a user OAuth token with auth.test and returns the
// workspace it belongs to. A token Slack rejects as dead goes on the revoked
// record, so the setup page refuses it next time without resending it.
func VerifyUserToken(ctx context.Context, token string) (VerifiedUserToken, error) {
	revoked, err := newRevokedCredentials(revokedFilePath())
	if err != nil {
		return VerifiedUserToken{}, err
	}
	return verifyUserTokenWith(ctx, token, "https://slack.com/api/", revoked)
}

func verifyUserTokenWith(ctx context.Context, token, apiURL string, revoked *revokedCredentials) (VerifiedUserToken, error) {
	if parseCredKind(token) != kindUserToken {
		return VerifiedUserToken{}, errNotUserToken
	}
	client := slack.New(token,
		slack.OptionAPIURL(apiURL),
		slack.OptionHTTPClient(&http.Client{Timeout: 10 * time.Second}))
	resp, err := client.AuthTestContext(ctx)
	if err != nil {
		var slackErr slack.SlackErrorResponse
		if errors.As(err, &slackErr) && terminalAuthErrors[slackErr.Err] {
			revoked.mark(token)
		}
		return VerifiedUserToken{}, err
	}
	return VerifiedUserToken{TeamID: resp.TeamID, TeamName: resp.Team, Token: token}, nil
}

// SaveUserTokenForWeb stores a verified user token under its team ID in the
// persistent token file, leaving other workspaces untouched.
func SaveUserTokenForWeb(v VerifiedUserToken) error {
	return saveUserTokenTo(tokenFilePath(), v)
}

func saveUserTokenTo(path string, v VerifiedUserToken) error {
	store := &tokenStore{workspaces: make(map[string]*workspace), filePath: path}
	store.loadFromFile()
	store.setWorkspace(&workspace{
		TeamID:   v.TeamID,
		TeamName: v.TeamName,
		Token:    v.Token,
		Source:   kindUserToken.String(),
	})
	return store.saveToFile()
}

// UserTokenManifest creates a Slack app with user scopes only, so its
// install yields an xoxp- token that acts as the installing user.
const UserTokenManifest = `display_information:
  name: Switchboard
oauth_config:
  scopes:
    user:
      - channels:read
      - channels:history
      - groups:read
      - groups:history
      - im:read
      - im:history
      - mpim:read
      - mpim:history
      - chat:write
      - users:read
      - reactions:read
      - reactions:write
      - search:read
      - team:read
settings:
  org_deploy_enabled: false
  socket_mode_enabled: false
`
