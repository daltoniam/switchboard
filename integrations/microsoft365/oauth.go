package microsoft365

import (
	"context"
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
	msAuthorizeURL = "https://login.microsoftonline.com/%s/oauth2/v2.0/authorize"
	msTokenURL     = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"
	msDefaultScope = "openid profile offline_access User.Read User.ReadBasic.All User.Read.All Mail.ReadWrite Mail.Send Calendars.ReadWrite Files.ReadWrite Files.ReadWrite.All Team.ReadBasic.All Channel.ReadBasic.All ChannelMessage.Read.All ChannelMessage.Send Chat.ReadWrite Tasks.ReadWrite People.Read"
)

type OAuthState struct {
	mu           sync.Mutex
	clientID     string
	clientSecret string
	redirectURI  string
	tenantID     string
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

func tenantOrDefault(tenantID string) string {
	if tenantID == "" {
		return defaultTenant
	}
	return tenantID
}

func StartM365OAuth(clientID, clientSecret, redirectURI, tenantID string) (*OAuthStartResult, error) {
	if clientID == "" {
		return nil, fmt.Errorf("microsoft365 OAuth client_id is not configured")
	}
	tenantID = tenantOrDefault(tenantID)
	state := oauthRandomString(32)
	codeVerifier := oauthRandomString(64)
	codeChallenge := oauthPKCEChallenge(codeVerifier)

	params := url.Values{
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"scope":                 {msDefaultScope},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
		"response_mode":         {"query"},
	}

	authURL := fmt.Sprintf(msAuthorizeURL, url.PathEscape(tenantID)) + "?" + params.Encode()

	oauthState := &OAuthState{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		tenantID:     tenantID,
		state:        state,
		codeVerifier: codeVerifier,
		startedAt:    time.Now(),
		tokenURL:     fmt.Sprintf(msTokenURL, url.PathEscape(tenantID)),
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

func HandleM365Callback(ctx context.Context, code, state string) error {
	if ctx == nil {
		ctx = context.Background()
	}
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
		"scope":         {msDefaultScope},
	}
	clientID := oauthState.clientID
	clientSecret := oauthState.clientSecret
	tokenURL := oauthState.tokenURL
	oauthState.mu.Unlock()

	return exchangeToken(ctx, oauthState, tokenURL, data, clientID, clientSecret)
}

func exchangeToken(ctx context.Context, oauthState *OAuthState, tokenURL string, data url.Values, clientID, clientSecret string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
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
		oauthState.err = fmt.Sprintf("Microsoft identity returned %d: %s", resp.StatusCode, string(body))
		oauthState.done = true
		oauthState.mu.Unlock()
		return fmt.Errorf("%s", oauthState.err)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
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
		oauthState.err = fmt.Sprintf("OAuth error: %s", tokenResp.ErrorDesc)
		if tokenResp.ErrorDesc == "" {
			oauthState.err = fmt.Sprintf("OAuth error: %s", tokenResp.Error)
		}
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

func PollM365OAuth() OAuthPollResult {
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

func RefreshAccessToken(ctx context.Context, clientID, clientSecret, refreshToken, tenantID string) (string, string, error) {
	tenantID = tenantOrDefault(tenantID)
	tokenURL := fmt.Sprintf(msTokenURL, url.PathEscape(tenantID))
	data := url.Values{
		"client_id":     {clientID},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
		"scope":         {msDefaultScope},
	}
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if clientSecret != "" {
		req.SetBasicAuth(clientID, clientSecret)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("microsoft365 token refresh failed (%d): %s", resp.StatusCode, string(body))
	}
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", "", err
	}
	if tokenResp.AccessToken == "" {
		return "", "", fmt.Errorf("microsoft365 token refresh: no access token")
	}
	return tokenResp.AccessToken, tokenResp.RefreshToken, nil
}
