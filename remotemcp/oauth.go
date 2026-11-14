package remotemcp

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"syscall"
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
	expiry       time.Time
	issued       time.Time
	err          string
	done         bool
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

// discoverOAuth finds the authorization server for a remote MCP server. It
// follows RFC 9728 protected resource metadata first, since an MCP resource
// may delegate to an issuer other than its origin (as Mobbin's does), and
// falls back to RFC 8414 metadata at the server origin.
//
// Metadata comes from the remote server, so it may only send discovery to a
// local address when the configured server is itself local (development and
// tests); otherwise a server could aim requests at the Switchboard host.
func discoverOAuth(serverURL string) (*oauthServerMeta, error) {
	parsed, err := url.Parse(serverURL)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("invalid MCP server URL %q", serverURL)
	}
	fetch := discoveryFetcher{allowLocal: isLoopback(parsed.Hostname())}

	if resource, ok := fetch.protectedResource(serverURL); ok {
		if resource.Resource != "" && resource.Resource != serverURL+"/mcp" && resource.Resource != serverURL {
			return nil, fmt.Errorf("protected resource metadata is for %q, not %q", resource.Resource, serverURL+"/mcp")
		}
		if len(resource.AuthorizationServers) > 0 {
			return fetch.authorizationServer(resource.AuthorizationServers[0])
		}
	}

	var origin oauthServerMeta
	if err := fetch.getJSON(serverURL+"/.well-known/oauth-authorization-server", &origin); err != nil {
		return nil, err
	}
	if !origin.hasEndpoints() {
		return nil, errMissingEndpoints
	}
	return &origin, nil
}

// authorizationServer reads RFC 8414 metadata for an issuer named by
// protected resource metadata and checks the issuer matches exactly.
func (f discoveryFetcher) authorizationServer(issuer string) (*oauthServerMeta, error) {
	// The issuer is compared exactly, so a trailing slash is kept here and
	// dropped only when building the metadata URL.
	metadataURL, err := authorizationServerMetadataURL(issuer, f.allowLocal)
	if err != nil {
		return nil, err
	}
	var meta oauthServerMeta
	if err := f.getJSON(metadataURL, &meta); err != nil {
		return nil, err
	}
	if meta.Issuer != issuer {
		return nil, fmt.Errorf("authorization server metadata issuer %q does not match %q", meta.Issuer, issuer)
	}
	if !meta.hasEndpoints() {
		return nil, errMissingEndpoints
	}
	return &meta, nil
}

type protectedResourceMeta struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
}

// protectedResource reads RFC 9728 metadata for the /mcp endpoint, then at
// the origin, which MCP servers may publish instead.
func (f discoveryFetcher) protectedResource(serverURL string) (protectedResourceMeta, bool) {
	for _, path := range []string{"/.well-known/oauth-protected-resource/mcp", "/.well-known/oauth-protected-resource"} {
		var resource protectedResourceMeta
		if f.getJSON(serverURL+path, &resource) == nil {
			return resource, true
		}
	}
	return protectedResourceMeta{}, false
}

var errMissingEndpoints = fmt.Errorf("oauth metadata has no authorization or token endpoint")

func (m *oauthServerMeta) hasEndpoints() bool {
	return m.AuthorizationEndpoint != "" && m.TokenEndpoint != ""
}

// authorizationServerMetadataURL inserts the RFC 8414 well-known segment
// between the issuer's host and path, keeping the path's escaping. Issuers
// must use HTTPS and a non-local host; a local issuer (over HTTP or HTTPS) is
// accepted only when allowLocal says the configured server is local too.
func authorizationServerMetadataURL(issuer string, allowLocal bool) (string, error) {
	parsed, err := url.Parse(issuer)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("invalid authorization server issuer %q", issuer)
	}
	if err := checkDiscoveryTarget(parsed, allowLocal); err != nil {
		return "", fmt.Errorf("authorization server issuer %q: %w", issuer, err)
	}
	const wellKnown = "/.well-known/oauth-authorization-server"
	// Trim only literal slashes from the escaped path and derive the decoded
	// path from it, so an encoded %2F stays part of the issuer.
	escaped := strings.TrimRight(parsed.EscapedPath(), "/")
	decoded, err := url.PathUnescape(escaped)
	if err != nil {
		return "", fmt.Errorf("invalid authorization server issuer %q", issuer)
	}
	parsed.Path = wellKnown + decoded
	parsed.RawPath = wellKnown + escaped
	return parsed.String(), nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// checkDiscoveryTarget allows HTTPS to non-local hosts, and local hosts
// (over HTTP or HTTPS) only when allowLocal is set.
func checkDiscoveryTarget(target *url.URL, allowLocal bool) error {
	local := isLoopback(target.Hostname())
	if local && !allowLocal {
		return fmt.Errorf("refusing local address %s for a remote MCP server", target.Host)
	}
	if target.Scheme != "https" && (target.Scheme != "http" || !local) {
		return fmt.Errorf("must use HTTPS")
	}
	return nil
}

// discoveryTransport replaces the checked transport in tests that serve
// discovery from handlers; nil uses the real network with address checks.
var discoveryTransport http.RoundTripper

// discoveryTLSConfig lets tests trust a local TLS server; nil uses defaults.
var discoveryTLSConfig *tls.Config

// discoveryFetcher reads OAuth discovery documents. Every request and every
// redirect is checked with checkDiscoveryTarget.
type discoveryFetcher struct {
	allowLocal bool
}

// transport dials only addresses checkDialAddress allows. The check runs on
// the resolved IP at connect time, so a hostname that resolves (or is
// rebound) to a local address cannot get past the hostname check.
func (f discoveryFetcher) transport() http.RoundTripper {
	if discoveryTransport != nil {
		return discoveryTransport
	}
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			return f.checkDialAddress(address)
		},
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = dialer.DialContext
	// Connect directly: through a proxy the check would only see the
	// proxy's address, never the destination it connects to.
	transport.Proxy = nil
	if discoveryTLSConfig != nil {
		transport.TLSClientConfig = discoveryTLSConfig
	}
	return transport
}

// checkDialAddress refuses loopback, link-local (including cloud metadata
// at 169.254.169.254) and unspecified addresses unless allowLocal is set.
// Private ranges stay reachable for self-hosted MCP servers.
func (f discoveryFetcher) checkDialAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("discover oauth: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("discover oauth: unresolved address %q", address)
	}
	if f.allowLocal {
		return nil
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return fmt.Errorf("refusing local address %s for a remote MCP server", address)
	}
	return nil
}

func (f discoveryFetcher) getJSON(target string, v any) error {
	parsed, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("discover oauth: %w", err)
	}
	if err := checkDiscoveryTarget(parsed, f.allowLocal); err != nil {
		return fmt.Errorf("discover oauth: %w", err)
	}
	client := &http.Client{
		Transport: f.transport(),
		Timeout:   15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return checkDiscoveryTarget(req.URL, f.allowLocal)
		},
	}
	resp, err := client.Get(target) // #nosec G107 -- URL from the configured server or its OAuth metadata, checked above
	if err != nil {
		return fmt.Errorf("discover oauth: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return fmt.Errorf("oauth discovery returned %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("parse oauth metadata: %w", err)
	}
	return nil
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
	meta, err := discoverOAuth(serverURL)
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

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
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
	os.issued = time.Now()
	os.expiry = expiryAfter(os.issued, tokenResp.ExpiresIn)
	os.done = true
	return nil
}

// LoginCredentials returns the credentials of a completed OAuth flow under
// the CredKey* names, including what TokenRenewal needs to renew it: the
// refresh token, the expiry, and the dynamically registered client. Values
// the server did not send are empty, so saving them clears any old login.
func LoginCredentials(name string) (mcp.Credentials, bool) {
	activeRemoteOAuth.mu.Lock()
	os := activeRemoteOAuth.states[name]
	activeRemoteOAuth.mu.Unlock()
	if os == nil {
		return nil, false
	}
	os.mu.Lock()
	defer os.mu.Unlock()
	if !os.done || os.err != "" || os.token == "" {
		return nil, false
	}
	return mcp.Credentials{
		CredKeyAccessToken:  os.token,
		CredKeyRefreshToken: os.refreshToken,
		CredKeyTokenExpiry:  formatExpiry(os.expiry),
		CredKeyTokenIssued:  formatExpiry(os.issued),
		CredKeyClientID:     os.clientID,
		CredKeyClientSecret: os.clientSecret,
	}, true
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

// ServerURL returns the configured server URL for a remote MCP integration.
func ServerURL(i mcp.Integration) string {
	if r, ok := i.(*remote); ok {
		return r.serverURL
	}
	return ""
}
