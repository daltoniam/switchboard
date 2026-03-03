package linear

import (
	"crypto/rand"
	"crypto/sha256"
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
	authorizeURL  = "https://linear.app/oauth/authorize"
	tokenURL      = "https://api.linear.app/oauth/token"
	defaultScopes = "read,write"
)

type OAuthState struct {
	mu           sync.Mutex
	clientID     string
	clientSecret string
	redirectURI  string
	state        string
	codeVerifier string
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

var activeOAuth struct {
	mu    sync.Mutex
	state *OAuthState
}

func randomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}

func pkceChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func StartLinearOAuth(clientID, clientSecret, redirectURI string) (*OAuthStartResult, error) {
	if clientID == "" {
		return nil, fmt.Errorf("linear OAuth client_id is not configured")
	}
	if clientSecret == "" {
		return nil, fmt.Errorf("linear OAuth client_secret is not configured")
	}

	state := randomString(32)
	codeVerifier := randomString(64)
	codeChallenge := pkceChallenge(codeVerifier)

	params := url.Values{
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"scope":                 {defaultScopes},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
		"prompt":                {"consent"},
	}

	authURL := authorizeURL + "?" + params.Encode()

	oauthState := &OAuthState{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		state:        state,
		codeVerifier: codeVerifier,
	}

	activeOAuth.mu.Lock()
	activeOAuth.state = oauthState
	activeOAuth.mu.Unlock()

	return &OAuthStartResult{AuthorizeURL: authURL}, nil
}

func HandleLinearCallback(code, state string) error {
	activeOAuth.mu.Lock()
	oauthState := activeOAuth.state
	activeOAuth.mu.Unlock()

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
		"code_verifier": {oauthState.codeVerifier},
		"grant_type":    {"authorization_code"},
	}

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
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
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		oauthState.err = fmt.Sprintf("Failed to read token response: %v", err)
		oauthState.done = true
		return fmt.Errorf("%s", oauthState.err)
	}

	if resp.StatusCode != 200 {
		oauthState.err = fmt.Sprintf("Linear returned %d: %s", resp.StatusCode, string(body))
		oauthState.done = true
		return fmt.Errorf("%s", oauthState.err)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		oauthState.err = fmt.Sprintf("Failed to parse token response: %v", err)
		oauthState.done = true
		return fmt.Errorf("%s", oauthState.err)
	}

	if tokenResp.Error != "" {
		oauthState.err = fmt.Sprintf("OAuth error: %s", tokenResp.Error)
		oauthState.done = true
		return fmt.Errorf("%s", oauthState.err)
	}

	if tokenResp.AccessToken == "" {
		oauthState.err = "No access token in response"
		oauthState.done = true
		return fmt.Errorf("%s", oauthState.err)
	}

	oauthState.token = tokenResp.AccessToken
	oauthState.done = true
	return nil
}

func PollLinearOAuth() OAuthPollResult {
	activeOAuth.mu.Lock()
	oauthState := activeOAuth.state
	activeOAuth.mu.Unlock()

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
