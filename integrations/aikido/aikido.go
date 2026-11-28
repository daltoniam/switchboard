package aikido

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("aikido", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

const (
	maxResponseSize = 10 * 1024 * 1024
	maxErrorSize    = 1000
	requestTimeout  = 60 * time.Second
	// tokenRefreshMargin renews the access token before it expires so a call
	// never starts with a token that lapses mid-request.
	tokenRefreshMargin = 60 * time.Second
	apiPath            = "/api/public/v1"
	tokenPath          = "/api/oauth/token"
	credentialsPage    = "Aikido Settings > Integrations > REST API (https://app.aikido.dev/settings/integrations/api/aikido/rest)"
)

// regionOrigins are the Aikido app hosts per data region, from the server
// list in Aikido's public API reference.
var regionOrigins = map[string]string{
	"eu": "https://app.aikido.dev",
	"us": "https://app.us.aikido.dev",
	"au": "https://app.au.aikido.dev",
	"me": "https://app.me.aikido.dev",
}

var (
	_ mcp.Integration                = (*aikido)(nil)
	_ mcp.FieldCompactionIntegration = (*aikido)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*aikido)(nil)
	_ mcp.PlainTextCredentials       = (*aikido)(nil)
	_ mcp.OptionalCredentials        = (*aikido)(nil)
	_ mcp.PlaceholderHints           = (*aikido)(nil)
	_ mcp.SetupGuide                 = (*aikido)(nil)
	_ io.Closer                      = (*aikido)(nil)
)

type aikido struct {
	mu           sync.RWMutex
	client       *http.Client
	origin       string
	clientID     string
	clientSecret string

	tokenMu     sync.Mutex
	accessToken string
	tokenExpiry time.Time
	now         func() time.Time
}

// New returns an unconfigured Aikido Security integration.
func New() mcp.Integration { return &aikido{now: time.Now} }

func (a *aikido) Name() string { return "aikido" }

func (a *aikido) PlainTextKeys() []string { return []string{"client_id", "region", "base_url"} }

func (a *aikido) OptionalKeys() []string { return []string{"region", "base_url"} }

func (a *aikido) Placeholders() map[string]string {
	return map[string]string{
		"client_id":     "REST API client ID (AIK_CLIENT_...) from Aikido Settings > Integrations > REST API",
		"client_secret": "Client secret shown once when the REST API client is created",
		"region":        "eu (default, app.aikido.dev), us, au or me: the region your workspace lives in",
		"base_url":      "Optional Aikido app URL override, e.g. https://app.us.aikido.dev",
	}
}

func (a *aikido) SetupSteps() []string {
	return []string{
		"In Aikido, a workspace admin opens " + credentialsPage + " and creates a client with App Type Private.",
		"Grant read scopes for what agents should see: basics:read (required, used for the health check), issues:read, repositories:read, containers:read, clouds:read, domains:read, virtual_machines:read, teams:read, users:read, reports:read, licenses:read, research:read, firewall:read, pentests:read, autofix:read, code_quality:read, code_coverage:read, custom_sast_rules:read, task_tracking:read, webhooks:read, request_inbox:read and endpoint_protection:read.",
		"Add a write scope only for actions agents may take, such as issues:write (ignore, snooze, solve, severity, notes), repositories:write and containers:write (scans, activation), domains:write, teams:write, users:write, firewall:write (Zen), autofix:write (fix pull requests) and task_tracking:write. A tool whose scope is missing fails with a 403 that names the scope.",
		"Copy the client ID (AIK_CLIENT_...) and the client secret, which is shown only once. Set region to us, au or me if your workspace is not on app.aikido.dev (EU).",
		"Enable the integration and save. Aikido allows 20 API calls per minute per workspace, so prefer filtered lists over many single lookups.",
	}
}

func (a *aikido) Tools() []mcp.ToolDefinition { return tools }

func (a *aikido) CompactSpec(name mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[name]
	return fields, ok
}

func (a *aikido) MaxBytes(name mcp.ToolName) (int, bool) {
	limit, ok := maxBytesByTool[name]
	return limit, ok
}

func (a *aikido) clear() {
	if a.client != nil {
		a.client.CloseIdleConnections()
	}
	a.client, a.origin, a.clientID, a.clientSecret = nil, "", "", ""
	a.tokenMu.Lock()
	a.accessToken, a.tokenExpiry = "", time.Time{}
	a.tokenMu.Unlock()
}

// Close drops the cached access token and idle connections. It is safe to
// call more than once.
func (a *aikido) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.clear()
	return nil
}

func (a *aikido) Configure(_ context.Context, creds mcp.Credentials) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.clear()

	clientID := strings.TrimSpace(creds["client_id"])
	clientSecret := strings.TrimSpace(creds["client_secret"])
	if !printableToken(clientID) {
		return fmt.Errorf("aikido: client_id is required; create a REST API client in %s", credentialsPage)
	}
	if !printableToken(clientSecret) {
		return fmt.Errorf("aikido: client_secret is required; it is shown once when the REST API client is created")
	}
	origin, err := resolveOrigin(creds["region"], creds["base_url"])
	if err != nil {
		return err
	}
	a.client = &http.Client{
		Timeout: requestTimeout,
		// The bearer token and client secret must never follow a redirect.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	a.origin, a.clientID, a.clientSecret = origin, clientID, clientSecret
	return nil
}

func printableToken(s string) bool {
	return s != "" && len(s) <= 4096 && !strings.ContainsFunc(s, func(r rune) bool { return r < 33 || r > 126 })
}

// resolveOrigin picks the Aikido app origin from an explicit base_url, else
// the region, else the EU default.
func resolveOrigin(region, baseURL string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL != "" {
		return normalizeBaseURL(baseURL)
	}
	region = strings.ToLower(strings.TrimSpace(region))
	if region == "" {
		region = "eu"
	}
	origin, ok := regionOrigins[region]
	if !ok {
		return "", fmt.Errorf("aikido: region must be eu, us, au or me (got %q)", region)
	}
	return origin, nil
}

// normalizeBaseURL accepts an HTTPS Aikido app origin, or plain HTTP on a
// loopback host for local proxies and fixtures, and strips any API path the
// user pasted with it.
func normalizeBaseURL(raw string) (string, error) {
	invalid := fmt.Errorf("aikido: base_url must be an HTTPS URL such as https://app.us.aikido.dev, without credentials, query or fragment")
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", invalid
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !isLoopback(u.Hostname()) {
			return "", invalid
		}
	default:
		return "", invalid
	}
	path := strings.TrimRight(u.Path, "/")
	path = strings.TrimSuffix(path, apiPath)
	path = strings.TrimSuffix(path, "/api")
	return u.Scheme + "://" + u.Host + path, nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type connection struct {
	client       *http.Client
	origin       string
	clientID     string
	clientSecret string
}

func (a *aikido) snapshot() (connection, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return connection{a.client, a.origin, a.clientID, a.clientSecret}, a.client != nil
}

func (a *aikido) Healthy(ctx context.Context) bool {
	if _, ok := a.snapshot(); !ok {
		return false
	}
	_, err := a.do(ctx, http.MethodGet, "/workspace", nil, nil, "basics:read")
	return err == nil
}

func (a *aikido) Execute(ctx context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	ep, ok := dispatch[name]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("aikido: unknown tool: %s", name))
	}
	if _, ok := a.snapshot(); !ok {
		return mcp.ErrResult(fmt.Errorf("aikido: integration is not configured"))
	}
	return ep.run(ctx, a, args)
}

// token returns a cached access token, fetching a new one with the client
// credentials grant when none is cached or it is about to expire. Aikido's
// rate limit is per workspace and small, so tokens are reused until expiry.
func (a *aikido) token(ctx context.Context, conn connection) (string, error) {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	if a.accessToken != "" && a.now().Add(tokenRefreshMargin).Before(a.tokenExpiry) {
		return a.accessToken, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, conn.origin+tokenPath, strings.NewReader(`{"grant_type":"client_credentials"}`))
	if err != nil {
		return "", fmt.Errorf("aikido: build token request: %w", err)
	}
	req.SetBasicAuth(conn.clientID, conn.clientSecret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := conn.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("aikido: token request: %w", redactURLError(err))
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("aikido: read token response: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return "", &mcp.RetryableError{
			StatusCode: resp.StatusCode,
			Err:        fmt.Errorf("aikido token endpoint error (%d): %s", resp.StatusCode, errorText(data)),
			RetryAfter: mcp.ParseRetryAfter(resp.Header.Get("Retry-After")),
		}
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("aikido: could not get an access token (%d): %s; check the client ID, client secret and region of the REST API client in %s", resp.StatusCode, errorText(data), credentialsPage)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(data, &tok); err != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("aikido: token response has no access_token; check base_url points at the Aikido app")
	}
	lifetime := time.Duration(tok.ExpiresIn) * time.Second
	if lifetime <= 0 {
		lifetime = time.Hour
	}
	a.accessToken, a.tokenExpiry = tok.AccessToken, a.now().Add(lifetime)
	return a.accessToken, nil
}

func (a *aikido) dropToken(stale string) {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	if a.accessToken == stale {
		a.accessToken, a.tokenExpiry = "", time.Time{}
	}
}

// apiResponse is a successful Aikido API response.
type apiResponse struct {
	status  int
	body    []byte
	hasNext string
}

// do calls the public API. A 401 drops the cached token and retries once
// with a fresh one, since tokens can be revoked before they expire.
func (a *aikido) do(ctx context.Context, method, path string, query url.Values, body any, scope string) (*apiResponse, error) {
	conn, ok := a.snapshot()
	if !ok {
		return nil, fmt.Errorf("aikido: integration is not configured")
	}
	var payload []byte
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("aikido: encode request: %w", err)
		}
		payload = data
	}
	target := conn.origin + apiPath + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	for attempt := 0; ; attempt++ {
		tok, err := a.token(ctx, conn)
		if err != nil {
			return nil, err
		}
		resp, err := a.send(ctx, conn.client, method, target, payload, tok)
		if err != nil {
			return nil, fmt.Errorf("aikido: %s %s: %w", method, path, err)
		}
		if resp.status == http.StatusUnauthorized && attempt == 0 {
			a.dropToken(tok)
			continue
		}
		return checkStatus(method, path, scope, resp)
	}
}

type rawResponse struct {
	status     int
	body       []byte
	retryAfter string
	hasNext    string
}

func (a *aikido) send(ctx context.Context, client *http.Client, method, target string, payload []byte, tok string) (*rawResponse, error) {
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, redactURLError(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if len(data) > maxResponseSize {
		return nil, fmt.Errorf("response exceeds %d bytes; narrow it with filters or a smaller per_page", maxResponseSize)
	}
	return &rawResponse{status: resp.StatusCode, body: data, retryAfter: resp.Header.Get("Retry-After"), hasNext: resp.Header.Get("X-Has-Next-Page")}, nil
}

func checkStatus(method, path, scope string, resp *rawResponse) (*apiResponse, error) {
	switch {
	case resp.status == http.StatusTooManyRequests || resp.status >= 500:
		return nil, &mcp.RetryableError{
			StatusCode: resp.status,
			Err:        fmt.Errorf("aikido API error (%d) on %s %s: %s", resp.status, method, path, errorText(resp.body)),
			RetryAfter: mcp.ParseRetryAfter(resp.retryAfter),
		}
	case resp.status >= 300 && resp.status < 400:
		return nil, fmt.Errorf("aikido API returned a redirect (%d) on %s %s; check region and base_url", resp.status, method, path)
	case resp.status == http.StatusUnauthorized:
		return nil, fmt.Errorf("aikido API error (401) on %s %s: %s; the REST API client was rejected, check it in %s", method, path, errorText(resp.body), credentialsPage)
	case resp.status == http.StatusForbidden:
		return nil, fmt.Errorf("aikido API error (403) on %s %s: %s; the REST API client needs the %s scope (edit the client in %s), or the feature is not on your Aikido plan", method, path, errorText(resp.body), scope, credentialsPage)
	case resp.status >= 400:
		return nil, fmt.Errorf("aikido API error (%d) on %s %s: %s", resp.status, method, path, errorText(resp.body))
	}
	return &apiResponse{status: resp.status, body: resp.body, hasNext: resp.hasNext}, nil
}

// errorText prefers Aikido's {"error": ...} or OAuth error_description and
// bounds the size of anything else.
func errorText(data []byte) string {
	var parsed struct {
		Error            any    `json:"error"`
		ErrorDescription string `json:"error_description"`
		Message          string `json:"message"`
	}
	if json.Unmarshal(data, &parsed) == nil {
		var parts []string
		if s, ok := parsed.Error.(string); ok && s != "" {
			parts = append(parts, s)
		}
		for _, s := range []string{parsed.ErrorDescription, parsed.Message} {
			if s != "" {
				parts = append(parts, s)
			}
		}
		if len(parts) > 0 {
			return truncate(strings.Join(parts, ": "))
		}
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return "empty response body"
	}
	return truncate(text)
}

func truncate(text string) string {
	if len(text) > maxErrorSize {
		return text[:maxErrorSize] + "...(truncated)"
	}
	return text
}

func redactURLError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}
