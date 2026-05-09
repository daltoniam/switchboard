package remotemcp

import (
	"bytes"
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

	mcp "github.com/daltoniam/switchboard"
)

// OAuthState tracks an in-progress OAuth flow for a remote MCP server.
type OAuthState struct {
	mu           sync.Mutex
	serverURL    string
	clientID     string
	clientSecret string
	redirectURI  string
	state        string
	codeVerifier string
	token        string
	refreshToken string
	expiresAt    time.Time
	err          string
	done         bool
}

// TokenResult holds the tokens returned from an OAuth exchange or refresh.
type TokenResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

type oauthServerMeta struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	RegistrationEndpoint  string   `json:"registration_endpoint"`
	CodeChallengeMethods  []string `json:"code_challenge_methods_supported"`
}

var activeRemoteOAuth struct {
	mu     sync.Mutex
	states map[string]*OAuthState
}

func init() {
	activeRemoteOAuth.states = make(map[string]*OAuthState)
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

func discoverOAuth(serverURL string) (*oauthServerMeta, error) {
	resp, err := http.Get(serverURL + "/.well-known/oauth-authorization-server")
	if err != nil {
		return nil, fmt.Errorf("discover oauth: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("oauth discovery returned %d", resp.StatusCode)
	}

	var meta oauthServerMeta
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("parse oauth metadata: %w", err)
	}
	return &meta, nil
}

func registerClient(registerURL, redirectURI string) (clientID, clientSecret string, err error) {
	body, _ := json.Marshal(map[string]any{
		"client_name":                "Switchboard",
		"redirect_uris":              []string{redirectURI},
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
	})

	resp, err := http.Post(registerURL, "application/json", bytes.NewReader(body)) // #nosec G107 -- URL from OAuth metadata discovery
	if err != nil {
		return "", "", fmt.Errorf("register client: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read registration response: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return "", "", fmt.Errorf("registration failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var reg struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.Unmarshal(respBody, &reg); err != nil {
		return "", "", fmt.Errorf("parse registration: %w", err)
	}
	if reg.ClientID == "" {
		return "", "", fmt.Errorf("no client_id in registration response")
	}
	return reg.ClientID, reg.ClientSecret, nil
}

// StartOAuth begins the MCP OAuth flow for a remote server.
// It discovers the OAuth endpoints, registers a dynamic client, and returns the authorize URL.
func StartOAuth(name, serverURL, redirectURI string) (string, error) {
	meta, err := discoverOAuth(serverURL)
	if err != nil {
		return "", err
	}

	if meta.RegistrationEndpoint == "" {
		return "", fmt.Errorf("remote server does not support dynamic client registration")
	}

	clientID, clientSecret, err := registerClient(meta.RegistrationEndpoint, redirectURI)
	if err != nil {
		return "", err
	}

	state := randomString(32)
	verifier := randomString(64)
	challenge := pkceChallenge(verifier)

	params := url.Values{
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"scope":                 {"read,write"},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}

	authorizeURL := meta.AuthorizationEndpoint + "?" + params.Encode()

	os := &OAuthState{
		serverURL:    serverURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		state:        state,
		codeVerifier: verifier,
	}

	activeRemoteOAuth.mu.Lock()
	activeRemoteOAuth.states[name] = os
	activeRemoteOAuth.mu.Unlock()

	return authorizeURL, nil
}

// HandleOAuthCallback exchanges the authorization code for an access token.
func HandleOAuthCallback(name, code, stateParam string) error {
	activeRemoteOAuth.mu.Lock()
	os := activeRemoteOAuth.states[name]
	activeRemoteOAuth.mu.Unlock()

	if os == nil {
		return fmt.Errorf("no OAuth flow in progress for %s", name)
	}

	os.mu.Lock()
	defer os.mu.Unlock()

	if os.state != stateParam {
		os.err = "invalid state parameter"
		os.done = true
		return fmt.Errorf("%s", os.err)
	}

	meta, err := discoverOAuth(os.serverURL)
	if err != nil {
		os.err = err.Error()
		os.done = true
		return err
	}

	data := url.Values{
		"code":          {code},
		"redirect_uri":  {os.redirectURI},
		"client_id":     {os.clientID},
		"code_verifier": {os.codeVerifier},
		"grant_type":    {"authorization_code"},
	}
	if os.clientSecret != "" {
		data.Set("client_secret", os.clientSecret)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", meta.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		os.err = err.Error()
		os.done = true
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		os.err = fmt.Sprintf("token exchange failed: %v", err)
		os.done = true
		return fmt.Errorf("%s", os.err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		os.err = fmt.Sprintf("read token response: %v", err)
		os.done = true
		return fmt.Errorf("%s", os.err)
	}

	if resp.StatusCode != 200 {
		os.err = fmt.Sprintf("token endpoint returned %d: %s", resp.StatusCode, string(body))
		os.done = true
		return fmt.Errorf("%s", os.err)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
		Error        string `json:"error"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		os.err = fmt.Sprintf("parse token response: %v", err)
		os.done = true
		return fmt.Errorf("%s", os.err)
	}

	if tokenResp.Error != "" {
		os.err = fmt.Sprintf("OAuth error: %s", tokenResp.Error)
		os.done = true
		return fmt.Errorf("%s", os.err)
	}

	if tokenResp.AccessToken == "" {
		os.err = "no access_token in response"
		os.done = true
		return fmt.Errorf("%s", os.err)
	}

	os.token = tokenResp.AccessToken
	os.refreshToken = tokenResp.RefreshToken
	if tokenResp.ExpiresIn > 0 {
		os.expiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}
	os.done = true
	return nil
}

// PollOAuth checks the status of a pending OAuth flow.
func PollOAuth(name string) (status, token, errStr string) {
	activeRemoteOAuth.mu.Lock()
	os := activeRemoteOAuth.states[name]
	activeRemoteOAuth.mu.Unlock()

	if os == nil {
		return "no_flow", "", "No OAuth flow in progress"
	}

	os.mu.Lock()
	defer os.mu.Unlock()

	if !os.done {
		return "pending", "", ""
	}
	if os.err != "" {
		return "error", "", os.err
	}
	return "complete", os.token, ""
}

// PollOAuthFull checks the status and returns full token details including refresh token.
func PollOAuthFull(name string) (status string, result *TokenResult, errStr string) {
	activeRemoteOAuth.mu.Lock()
	os := activeRemoteOAuth.states[name]
	activeRemoteOAuth.mu.Unlock()

	if os == nil {
		return "no_flow", nil, "No OAuth flow in progress"
	}

	os.mu.Lock()
	defer os.mu.Unlock()

	if !os.done {
		return "pending", nil, ""
	}
	if os.err != "" {
		return "error", nil, os.err
	}
	return "complete", &TokenResult{
		AccessToken:  os.token,
		RefreshToken: os.refreshToken,
		ExpiresIn:    int(time.Until(os.expiresAt).Seconds()),
	}, ""
}

// RefreshToken uses a refresh token to obtain a new access token.
func RefreshToken(serverURL, clientID, clientSecret, refreshToken string) (*TokenResult, error) {
	meta, err := discoverOAuth(serverURL)
	if err != nil {
		return nil, err
	}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {clientID},
	}
	if clientSecret != "" {
		data.Set("client_secret", clientSecret)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", meta.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh token request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, fmt.Errorf("read refresh response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("refresh token failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse refresh response: %w", err)
	}
	if tokenResp.Error != "" {
		return nil, fmt.Errorf("refresh error: %s", tokenResp.Error)
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("no access_token in refresh response")
	}

	result := &TokenResult{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    tokenResp.ExpiresIn,
	}
	if result.RefreshToken == "" {
		result.RefreshToken = refreshToken
	}
	return result, nil
}

// ServerURL returns the configured server URL for a remote MCP integration.
func ServerURL(i mcp.Integration) string {
	if r, ok := i.(*remote); ok {
		return r.serverURL
	}
	return ""
}
