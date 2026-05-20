// Package googleoauth implements the shared OAuth 2.0 + PKCE flow used by
// Switchboard's Google Workspace integrations (Gmail, Calendar, Drive, Docs,
// Sheets, ...).
//
// Concurrent flows are keyed by integration name so that, e.g., a user can be
// connecting Gmail in one browser tab while Calendar is mid-setup in another
// without trampling each other's state.
//
// All Google APIs share the same authorization and token endpoints; only the
// requested scope set differs per service. Callers therefore configure a flow
// with their integration name plus the scopes they need.
package googleoauth

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

// AuthorizeURL is Google's OAuth 2.0 authorization endpoint. Exported so
// tests can assert authorization URLs are well-formed.
const AuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"

// tokenURL is the OAuth 2.0 token endpoint. It is a var (not a const) so
// tests can redirect it at an httptest server.
var tokenURL = "https://oauth2.googleapis.com/token"

// flowTTL bounds how long an in-progress flow can sit without being completed
// before it is garbage-collected on the next lookup. Matches the original
// gmail behavior (10 min).
const flowTTL = 10 * time.Minute

// Config configures a single integration's OAuth flow.
type Config struct {
	// IntegrationName uniquely identifies the integration (e.g., "gmail",
	// "gcal"). Concurrent flows for different integrations are kept isolated.
	IntegrationName string
	ClientID        string
	ClientSecret    string
	RedirectURI     string
	// Scopes are space-joined into the OAuth scope parameter. At least one
	// is required.
	Scopes []string
}

// StartResult is returned by Start.
type StartResult struct {
	AuthorizeURL string `json:"authorize_url"`
	Error        string `json:"error,omitempty"`
}

// PollResult is returned by Poll.
type PollResult struct {
	Status       string `json:"status"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Error        string `json:"error,omitempty"`
}

// flow holds the state for a single in-progress OAuth flow.
type flow struct {
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

var (
	flowsMu sync.Mutex
	flows   = make(map[string]*flow)
)

func randomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}

func pkceChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// Start initiates a new OAuth flow for cfg.IntegrationName. Any previous
// in-progress flow for the same integration is replaced.
func Start(cfg Config) (*StartResult, error) {
	if cfg.IntegrationName == "" {
		return nil, fmt.Errorf("googleoauth: integration name is required")
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("googleoauth: client_id is not configured")
	}
	if cfg.ClientSecret == "" {
		return nil, fmt.Errorf("googleoauth: client_secret is not configured")
	}
	if len(cfg.Scopes) == 0 {
		return nil, fmt.Errorf("googleoauth: at least one scope is required")
	}

	state := randomString(32)
	codeVerifier := randomString(64)
	codeChallenge := pkceChallenge(codeVerifier)

	params := url.Values{
		"client_id":             {cfg.ClientID},
		"redirect_uri":          {cfg.RedirectURI},
		"response_type":         {"code"},
		"scope":                 {strings.Join(cfg.Scopes, " ")},
		"access_type":           {"offline"},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
		"prompt":                {"consent"},
	}

	f := &flow{
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		redirectURI:  cfg.RedirectURI,
		state:        state,
		codeVerifier: codeVerifier,
		startedAt:    time.Now(),
	}

	flowsMu.Lock()
	flows[cfg.IntegrationName] = f
	flowsMu.Unlock()

	return &StartResult{AuthorizeURL: AuthorizeURL + "?" + params.Encode()}, nil
}

// getFlow returns the active flow for name, or nil if no flow is in progress
// or the flow has expired.
func getFlow(name string) *flow {
	flowsMu.Lock()
	defer flowsMu.Unlock()
	f, ok := flows[name]
	if !ok {
		return nil
	}
	if time.Since(f.startedAt) > flowTTL {
		delete(flows, name)
		return nil
	}
	return f
}

// HandleCallback completes the token exchange for the named integration's
// in-progress flow.
func HandleCallback(integrationName, code, state string) error {
	f := getFlow(integrationName)
	if f == nil {
		return fmt.Errorf("no OAuth flow in progress")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.state != state {
		f.err = "Invalid state parameter — possible CSRF attack"
		f.done = true
		return fmt.Errorf("%s", f.err)
	}

	data := url.Values{
		"code":          {code},
		"redirect_uri":  {f.redirectURI},
		"client_id":     {f.clientID},
		"client_secret": {f.clientSecret},
		"code_verifier": {f.codeVerifier},
		"grant_type":    {"authorization_code"},
	}

	tctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(tctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		f.err = fmt.Sprintf("Failed to create token request: %v", err)
		f.done = true
		return fmt.Errorf("%s", f.err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		f.err = fmt.Sprintf("Token exchange failed: %v", err)
		f.done = true
		return fmt.Errorf("%s", f.err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		f.err = fmt.Sprintf("Failed to read token response: %v", err)
		f.done = true
		return fmt.Errorf("%s", f.err)
	}

	if resp.StatusCode != 200 {
		f.err = fmt.Sprintf("Google returned %d: %s", resp.StatusCode, string(body))
		f.done = true
		return fmt.Errorf("%s", f.err)
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
		f.err = fmt.Sprintf("Failed to parse token response: %v", err)
		f.done = true
		return fmt.Errorf("%s", f.err)
	}

	if tokenResp.Error != "" {
		f.err = fmt.Sprintf("OAuth error: %s", tokenResp.Error)
		f.done = true
		return fmt.Errorf("%s", f.err)
	}

	if tokenResp.AccessToken == "" {
		f.err = "No access token in response"
		f.done = true
		return fmt.Errorf("%s", f.err)
	}

	f.accessToken = tokenResp.AccessToken
	f.refreshToken = tokenResp.RefreshToken
	f.done = true
	return nil
}

// Poll returns the current status of the named integration's flow.
func Poll(integrationName string) PollResult {
	f := getFlow(integrationName)
	if f == nil {
		return PollResult{Status: "no_flow", Error: "No OAuth flow in progress"}
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.done {
		return PollResult{Status: "pending"}
	}
	if f.err != "" {
		return PollResult{Status: "error", Error: f.err}
	}
	return PollResult{
		Status:       "complete",
		AccessToken:  f.accessToken,
		RefreshToken: f.refreshToken,
	}
}

// RefreshAccessToken exchanges a long-lived refresh token for a new access
// token via Google's token endpoint. ctx must be non-nil; callers on a hot
// path should provide a bounded context so a hung token endpoint cannot
// stall the request indefinitely.
func RefreshAccessToken(ctx context.Context, clientID, clientSecret, refreshToken string) (string, error) {
	data := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
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

// Reset clears all in-progress flows. Intended for tests.
func Reset() {
	flowsMu.Lock()
	defer flowsMu.Unlock()
	flows = make(map[string]*flow)
}

// SetTokenURLForTest redirects the package-level token endpoint at url for
// the duration of the test, restoring the previous value on cleanup. Lets
// adapter tests stand up a fake token endpoint when exercising the refresh
// path through googleoauth.RefreshAccessToken.
func SetTokenURLForTest(t interface {
	Helper()
	Cleanup(func())
}, url string) {
	t.Helper()
	prev := tokenURL
	tokenURL = url
	t.Cleanup(func() { tokenURL = prev })
}
