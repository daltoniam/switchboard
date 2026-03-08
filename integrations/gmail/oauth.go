package gmail

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
	googleAuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL     = "https://oauth2.googleapis.com/token"
	gmailScope         = "https://mail.google.com/"
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

func StartGmailOAuth(clientID, clientSecret, redirectURI string) (*OAuthStartResult, error) {
	if clientID == "" {
		return nil, fmt.Errorf("gmail OAuth client_id is not configured")
	}
	if clientSecret == "" {
		return nil, fmt.Errorf("gmail OAuth client_secret is not configured")
	}

	state := oauthRandomString(32)
	codeVerifier := oauthRandomString(64)
	codeChallenge := oauthPKCEChallenge(codeVerifier)

	params := url.Values{
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"scope":                 {gmailScope},
		"access_type":           {"offline"},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
		"prompt":                {"consent"},
	}

	authURL := googleAuthorizeURL + "?" + params.Encode()

	oauthState := &OAuthState{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		state:        state,
		codeVerifier: codeVerifier,
		startedAt:    time.Now(),
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

func HandleGmailCallback(code, state string) error {
	oauthState := getActiveOAuth()

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

	req, err := http.NewRequest("POST", googleTokenURL, strings.NewReader(data.Encode()))
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		oauthState.err = fmt.Sprintf("Failed to read token response: %v", err)
		oauthState.done = true
		return fmt.Errorf("%s", oauthState.err)
	}

	if resp.StatusCode != 200 {
		oauthState.err = fmt.Sprintf("Google returned %d: %s", resp.StatusCode, string(body))
		oauthState.done = true
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

	oauthState.accessToken = tokenResp.AccessToken
	oauthState.refreshToken = tokenResp.RefreshToken
	oauthState.done = true
	return nil
}

func PollGmailOAuth() OAuthPollResult {
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

func RefreshAccessToken(clientID, clientSecret, refreshToken string) (string, error) {
	data := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	}

	req, err := http.NewRequest("POST", googleTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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
		return "", fmt.Errorf("google returned %d: %s", resp.StatusCode, string(body))
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
