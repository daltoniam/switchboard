package x

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
	"time"
)

const (
	xAuthorizeURL = "https://x.com/i/oauth2/authorize"
	xTokenURL     = "https://api.x.com/2/oauth2/token"
	xDefaultScope = "tweet.read tweet.write tweet.moderate.write users.read follows.read follows.write offline.access space.read mute.read mute.write like.read like.write list.read list.write block.read block.write bookmark.read bookmark.write dm.read dm.write"
)

type OAuthState struct {
	mu           sync.Mutex
	clientID     string
	clientSecret string
	redirectURI  string
	state        string
	codeVerifier string
	accessToken  string
	refreshToken string
	err          string
	done         bool
	startedAt    time.Time
	tokenURL     string
}

type OAuthStartResult struct {
	AuthorizeURL string `json:"authorize_url"`
	Error        string `json:"error,omitempty"`
}

type OAuthPollResult struct {
	Status       string `json:"status"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Error        string `json:"error,omitempty"`
}

const oauthTTL = 10 * time.Minute

var activeOAuth struct {
	mu    sync.Mutex
	state *OAuthState
}

func oauthRandomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}

func oauthPKCEChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func StartXOAuth(clientID, clientSecret, redirectURI string) (*OAuthStartResult, error) {
	if clientID == "" {
		return nil, fmt.Errorf("twitter OAuth client_id is not configured")
	}

	state := oauthRandomString(32)
	codeVerifier := oauthRandomString(64)
	codeChallenge := oauthPKCEChallenge(codeVerifier)

	params := url.Values{
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"scope":                 {xDefaultScope},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}

	authURL := xAuthorizeURL + "?" + params.Encode()

	oauthState := &OAuthState{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		state:        state,
		codeVerifier: codeVerifier,
		startedAt:    time.Now(),
		tokenURL:     xTokenURL,
	}

	activeOAuth.mu.Lock()
	activeOAuth.state = oauthState
	activeOAuth.mu.Unlock()

	return &OAuthStartResult{AuthorizeURL: authURL}, nil
}

func getActiveOAuth() *OAuthState {
	activeOAuth.mu.Lock()
	defer activeOAuth.mu.Unlock()
	s := activeOAuth.state
	if s != nil && time.Since(s.startedAt) > oauthTTL {
		activeOAuth.state = nil
		return nil
	}
	return s
}

func HandleXCallback(code, state string) error {
	oauthState := getActiveOAuth()

	if oauthState == nil {
		return fmt.Errorf("no OAuth flow in progress")
	}

	oauthState.mu.Lock()
	if oauthState.state != state {
		oauthState.err = "Invalid state parameter — possible CSRF attack"
		oauthState.done = true
		oauthState.mu.Unlock()
		return fmt.Errorf("%s", oauthState.err)
	}

	data := url.Values{
		"code":          {code},
		"redirect_uri":  {oauthState.redirectURI},
		"client_id":     {oauthState.clientID},
		"code_verifier": {oauthState.codeVerifier},
		"grant_type":    {"authorization_code"},
	}
	clientID := oauthState.clientID
	clientSecret := oauthState.clientSecret
	tokenURL := oauthState.tokenURL
	oauthState.mu.Unlock()

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		oauthState.mu.Lock()
		oauthState.err = fmt.Sprintf("Failed to create token request: %v", err)
		oauthState.done = true
		oauthState.mu.Unlock()
		return fmt.Errorf("%s", oauthState.err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if clientSecret != "" {
		req.SetBasicAuth(clientID, clientSecret)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		oauthState.mu.Lock()
		oauthState.err = fmt.Sprintf("Token exchange failed: %v", err)
		oauthState.done = true
		oauthState.mu.Unlock()
		return fmt.Errorf("%s", oauthState.err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		oauthState.mu.Lock()
		oauthState.err = fmt.Sprintf("Failed to read token response: %v", err)
		oauthState.done = true
		oauthState.mu.Unlock()
		return fmt.Errorf("%s", oauthState.err)
	}

	if resp.StatusCode != 200 {
		oauthState.mu.Lock()
		oauthState.err = fmt.Sprintf("X returned %d: %s", resp.StatusCode, string(body))
		oauthState.done = true
		oauthState.mu.Unlock()
		return fmt.Errorf("%s", oauthState.err)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
		Error        string `json:"error"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		oauthState.mu.Lock()
		oauthState.err = fmt.Sprintf("Failed to parse token response: %v", err)
		oauthState.done = true
		oauthState.mu.Unlock()
		return fmt.Errorf("%s", oauthState.err)
	}

	if tokenResp.Error != "" {
		oauthState.mu.Lock()
		oauthState.err = fmt.Sprintf("OAuth error: %s", tokenResp.Error)
		oauthState.done = true
		oauthState.mu.Unlock()
		return fmt.Errorf("%s", oauthState.err)
	}

	if tokenResp.AccessToken == "" {
		oauthState.mu.Lock()
		oauthState.err = "No access token in response"
		oauthState.done = true
		oauthState.mu.Unlock()
		return fmt.Errorf("%s", oauthState.err)
	}

	oauthState.mu.Lock()
	oauthState.accessToken = tokenResp.AccessToken
	oauthState.refreshToken = tokenResp.RefreshToken
	oauthState.done = true
	oauthState.mu.Unlock()
	return nil
}

func PollXOAuth() OAuthPollResult {
	oauthState := getActiveOAuth()

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

	return OAuthPollResult{
		Status:       "complete",
		AccessToken:  oauthState.accessToken,
		RefreshToken: oauthState.refreshToken,
	}
}

func RefreshAccessToken(clientID, clientSecret, refreshToken, tokenEndpoint string) (string, error) {
	if tokenEndpoint == "" {
		tokenEndpoint = xTokenURL
	}
	data := url.Values{
		"client_id":     {clientID},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	}

	req, err := http.NewRequest("POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if clientSecret != "" {
		req.SetBasicAuth(clientID, clientSecret)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("refresh request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return "", fmt.Errorf("read refresh response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("x returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("parse refresh response: %w", err)
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("refresh error: %s", tokenResp.Error)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("no access token in refresh response")
	}

	return tokenResp.AccessToken, nil
}
