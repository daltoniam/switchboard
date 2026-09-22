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
	resource     string
	token        string
	refreshToken string
	err          string
	done         bool
}

// TokenSet is everything a remote integration needs to keep using and refreshing an OAuth grant.
type TokenSet struct {
	AccessToken  string
	RefreshToken string
	ClientID     string
	ClientSecret string
}

// OAuthOptions configures provider-specific OAuth requirements.
type OAuthOptions struct {
	Scope      string
	ClientName string
	Resource   string
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

func discoverOAuth(ctx context.Context, serverURL string) (*oauthServerMeta, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL+"/.well-known/oauth-authorization-server", nil)
	if err != nil {
		return nil, fmt.Errorf("discover oauth: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
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

func registerClient(registerURL, redirectURI string, clientNames ...string) (clientID, clientSecret string, err error) {
	clientName := "Switchboard"
	if len(clientNames) > 0 && strings.TrimSpace(clientNames[0]) != "" {
		clientName = strings.TrimSpace(clientNames[0])
	}
	body, _ := json.Marshal(map[string]any{
		"client_name":                clientName,
		"redirect_uris":              []string{redirectURI},
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
		"application_type":           "native",
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
func StartOAuth(name, serverURL, redirectURI string, options ...OAuthOptions) (string, error) {
	meta, err := discoverOAuth(context.Background(), serverURL)
	if err != nil {
		return "", err
	}

	if meta.RegistrationEndpoint == "" {
		return "", fmt.Errorf("remote server does not support dynamic client registration")
	}

	config := OAuthOptions{Scope: "read,write", ClientName: "Switchboard"}
	if len(options) > 0 {
		if value := strings.TrimSpace(options[0].Scope); value != "" {
			config.Scope = value
		}
		if value := strings.TrimSpace(options[0].ClientName); value != "" {
			config.ClientName = value
		}
		config.Resource = strings.TrimSpace(options[0].Resource)
	}
	clientID, clientSecret, err := registerClient(meta.RegistrationEndpoint, redirectURI, config.ClientName)
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
		"scope":                 {config.Scope},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	if config.Resource != "" {
		params.Set("resource", config.Resource)
	}

	authorizeURL := meta.AuthorizationEndpoint + "?" + params.Encode()

	os := &OAuthState{
		serverURL:    serverURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		state:        state,
		codeVerifier: verifier,
		resource:     config.Resource,
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

	meta, err := discoverOAuth(context.Background(), os.serverURL)
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
	if os.resource != "" {
		data.Set("resource", os.resource)
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

	tokenResp, err := parseTokenResponse(body)
	if err != nil {
		os.err = err.Error()
		os.done = true
		return err
	}

	os.token = tokenResp.AccessToken
	os.refreshToken = tokenResp.RefreshToken
	os.done = true
	return nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	Error        string `json:"error"`
}

func parseTokenResponse(body []byte) (tokenResponse, error) {
	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return tokenResp, fmt.Errorf("parse token response: %v", err)
	}
	if tokenResp.Error != "" {
		return tokenResp, fmt.Errorf("OAuth error: %s", tokenResp.Error)
	}
	if tokenResp.AccessToken == "" {
		return tokenResp, fmt.Errorf("no access_token in response")
	}
	return tokenResp, nil
}

// refreshAccessToken exchanges the stored refresh token for a new grant. rejected is the
// access token the server just refused; when another caller already replaced it, the
// retry can proceed without a second refresh.
func (r *remote) refreshAccessToken(ctx context.Context, rejected string) error {
	r.refreshMu.Lock()
	defer r.refreshMu.Unlock()

	r.mu.RLock()
	current := TokenSet{AccessToken: r.token, RefreshToken: r.refreshToken, ClientID: r.clientID, ClientSecret: r.clientSecret}
	rejectedRefresh, rejectedErr := r.rejectedRefreshToken, r.rejectedRefreshErr
	r.mu.RUnlock()
	if current.AccessToken != rejected && current.AccessToken != "" {
		return nil
	}
	if current.AccessToken == "" {
		return fmt.Errorf("%s: the server requires authorization but no access token is configured", r.name)
	}
	if current.RefreshToken == "" {
		return fmt.Errorf("%s: access token rejected and no refresh token is stored; reconnect via the web UI", r.name)
	}
	if rejectedErr != nil && rejectedRefresh == current.RefreshToken {
		return rejectedErr
	}

	meta, err := discoverOAuth(ctx, r.serverURL)
	if err != nil {
		return fmt.Errorf("%s: refresh: %w", r.name, err)
	}
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {current.RefreshToken},
		"client_id":     {current.ClientID},
	}
	if current.ClientSecret != "" {
		data.Set("client_secret", current.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, meta.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("%s: refresh: %w", r.name, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s: refresh: %w", r.name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("%s: refresh: read token response: %w", r.name, err)
	}
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("%s: refresh: token endpoint returned %d: %s; reconnect via the web UI", r.name, resp.StatusCode, truncate(body, maxErrorBodyBytes))
		if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized {
			r.mu.Lock()
			r.rejectedRefreshToken = current.RefreshToken
			r.rejectedRefreshErr = err
			r.mu.Unlock()
		}
		return err
	}
	tokenResp, err := parseTokenResponse(body)
	if err != nil {
		return fmt.Errorf("%s: refresh: %w", r.name, err)
	}

	next := current
	next.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		next.RefreshToken = tokenResp.RefreshToken
	}
	r.mu.Lock()
	r.token = next.AccessToken
	r.refreshToken = next.RefreshToken
	r.mu.Unlock()
	if r.onTokenRefresh != nil {
		r.onTokenRefresh(next)
	}
	return nil
}

// PollOAuth checks the status of a pending OAuth flow.
func PollOAuth(name string) (status, token, errStr string) {
	status, tokens, errStr := PollOAuthTokens(name)
	return status, tokens.AccessToken, errStr
}

// PollOAuthTokens is PollOAuth with the full token set, including the refresh token and
// registered client, so callers can persist what a later refresh needs.
func PollOAuthTokens(name string) (status string, tokens TokenSet, errStr string) {
	activeRemoteOAuth.mu.Lock()
	os := activeRemoteOAuth.states[name]
	activeRemoteOAuth.mu.Unlock()

	if os == nil {
		return "no_flow", TokenSet{}, "No OAuth flow in progress"
	}

	os.mu.Lock()
	defer os.mu.Unlock()

	if !os.done {
		return "pending", TokenSet{}, ""
	}
	if os.err != "" {
		return "error", TokenSet{}, os.err
	}
	return "complete", TokenSet{AccessToken: os.token, RefreshToken: os.refreshToken, ClientID: os.clientID, ClientSecret: os.clientSecret}, ""
}

// ServerURL returns the configured server URL for a remote MCP integration.
func ServerURL(i mcp.Integration) string {
	if r, ok := i.(*remote); ok {
		return r.serverURL
	}
	return ""
}

func truncate(body []byte, limit int) string {
	if len(body) <= limit {
		return string(body)
	}
	return string(body[:limit]) + "..."
}
