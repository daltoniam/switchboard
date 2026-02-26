package slack

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

const (
	slackAuthorizeURL = "https://slack.com/oauth/v2/authorize"
	slackTokenURL     = "https://slack.com/api/oauth.v2.access"
	slackUserScopes   = "channels:history,channels:read,channels:write,chat:write,emoji:read,files:read,files:write,groups:history,groups:read,groups:write,im:history,im:read,im:write,mpim:history,mpim:read,mpim:write,pins:read,pins:write,reactions:read,reactions:write,reminders:read,reminders:write,search:read,stars:read,team:read,usergroups:read,users:read,users:read.email,users.profile:write,bookmarks:read,bookmarks:write"
)

type OAuthState struct {
	mu           sync.Mutex
	clientID     string
	clientSecret string
	redirectURI  string
	state        string
	token        string
	err          string
	done         bool
}

type OAuthStartResult struct {
	AuthorizeURL string `json:"authorize_url"`
	Error        string `json:"error,omitempty"`
}

type OAuthPollResult struct {
	Status string `json:"status"`
	Token  string `json:"token,omitempty"`
	Error  string `json:"error,omitempty"`
}

var activeSlackOAuth struct {
	mu    sync.Mutex
	state *OAuthState
}

func oauthRandomString(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}

func StartSlackOAuth(clientID, clientSecret, redirectURI string) (*OAuthStartResult, error) {
	if clientID == "" {
		return nil, fmt.Errorf("slack OAuth client_id is not configured")
	}
	if clientSecret == "" {
		return nil, fmt.Errorf("slack OAuth client_secret is not configured")
	}

	state := oauthRandomString(32)

	params := url.Values{
		"client_id":    {clientID},
		"redirect_uri": {redirectURI},
		"user_scope":   {slackUserScopes},
		"state":        {state},
	}

	authURL := slackAuthorizeURL + "?" + params.Encode()

	oauthState := &OAuthState{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		state:        state,
	}

	activeSlackOAuth.mu.Lock()
	activeSlackOAuth.state = oauthState
	activeSlackOAuth.mu.Unlock()

	return &OAuthStartResult{AuthorizeURL: authURL}, nil
}

func HandleSlackCallback(code, state string) error {
	activeSlackOAuth.mu.Lock()
	oauthState := activeSlackOAuth.state
	activeSlackOAuth.mu.Unlock()

	if oauthState == nil {
		return fmt.Errorf("no OAuth flow in progress")
	}

	oauthState.mu.Lock()
	defer oauthState.mu.Unlock()

	if oauthState.state != state {
		oauthState.err = "Invalid state parameter — possible CSRF attack"
		oauthState.done = true
		return fmt.Errorf("%s", oauthState.err)
	}

	data := url.Values{
		"code":          {code},
		"redirect_uri":  {oauthState.redirectURI},
		"client_id":     {oauthState.clientID},
		"client_secret": {oauthState.clientSecret},
	}

	req, err := http.NewRequest("POST", slackTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		oauthState.err = fmt.Sprintf("Failed to create token request: %v", err)
		oauthState.done = true
		return fmt.Errorf("%s", oauthState.err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		oauthState.err = fmt.Sprintf("Token exchange failed: %v", err)
		oauthState.done = true
		return fmt.Errorf("%s", oauthState.err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		oauthState.err = fmt.Sprintf("Failed to read token response: %v", err)
		oauthState.done = true
		return fmt.Errorf("%s", oauthState.err)
	}

	if resp.StatusCode != 200 {
		oauthState.err = fmt.Sprintf("Slack returned %d: %s", resp.StatusCode, string(body))
		oauthState.done = true
		return fmt.Errorf("%s", oauthState.err)
	}

	var tokenResp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		Authed struct {
			AccessToken string `json:"access_token"`
		} `json:"authed_user"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		oauthState.err = fmt.Sprintf("Failed to parse token response: %v", err)
		oauthState.done = true
		return fmt.Errorf("%s", oauthState.err)
	}

	if !tokenResp.OK || tokenResp.Error != "" {
		errMsg := tokenResp.Error
		if errMsg == "" {
			errMsg = "unknown error"
		}
		oauthState.err = fmt.Sprintf("Slack OAuth error: %s", errMsg)
		oauthState.done = true
		return fmt.Errorf("%s", oauthState.err)
	}

	if tokenResp.Authed.AccessToken == "" {
		oauthState.err = "No user access token in response"
		oauthState.done = true
		return fmt.Errorf("%s", oauthState.err)
	}

	oauthState.token = tokenResp.Authed.AccessToken
	oauthState.done = true
	return nil
}

func PollSlackOAuth() OAuthPollResult {
	activeSlackOAuth.mu.Lock()
	oauthState := activeSlackOAuth.state
	activeSlackOAuth.mu.Unlock()

	if oauthState == nil {
		return OAuthPollResult{Status: "no_flow", Error: "No OAuth flow in progress"}
	}

	oauthState.mu.Lock()
	defer oauthState.mu.Unlock()

	if !oauthState.done {
		return OAuthPollResult{Status: "pending"}
	}

	if oauthState.err != "" {
		return OAuthPollResult{Status: "error", Error: oauthState.err}
	}

	return OAuthPollResult{Status: "complete", Token: oauthState.token}
}
