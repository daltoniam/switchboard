package keycloak

import (
	"context"
	_ "embed"
	"encoding/json"
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

var compactResult = compact.MustLoadWithOverlay("keycloak", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs

const maxResponseBytes = 2 * 1024 * 1024
const maxOutputBytes = 64 * 1024

type keycloak struct {
	mu         sync.RWMutex
	connection *connection
}

type connection struct {
	baseURL      string
	realm        string
	clientID     string
	clientSecret string
	client       *http.Client
	mu           sync.Mutex
	token        string
	expires      time.Time
}

var (
	_ mcp.Integration                = (*keycloak)(nil)
	_ mcp.FieldCompactionIntegration = (*keycloak)(nil)
	_ mcp.PlainTextCredentials       = (*keycloak)(nil)
	_ mcp.PlaceholderHints           = (*keycloak)(nil)
)

func New() mcp.Integration                        { return &keycloak{} }
func (adapter *keycloak) Name() string            { return "keycloak" }
func (adapter *keycloak) PlainTextKeys() []string { return []string{"base_url", "realm", "client_id"} }
func (adapter *keycloak) Placeholders() map[string]string {
	return map[string]string{
		"base_url":      "https://keycloak.example.com (include /auth if configured)",
		"realm":         "Target realm name, not its ID",
		"client_id":     "Dedicated read-only service-account client",
		"client_secret": "Confidential client secret",
	}
}

func (adapter *keycloak) Configure(_ context.Context, creds mcp.Credentials) error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.connection = nil
	for _, key := range []string{"base_url", "realm", "client_id", "client_secret"} {
		if strings.TrimSpace(creds[key]) == "" {
			return fmt.Errorf("keycloak: %s is required", key)
		}
	}
	endpoint, err := url.Parse(strings.TrimRight(strings.TrimSpace(creds["base_url"]), "/"))
	if err != nil || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.ForceQuery || endpoint.Fragment != "" || endpoint.Opaque != "" {
		return fmt.Errorf("keycloak: base_url must be a trusted HTTPS installation URL without credentials, query, or fragment")
	}
	loopback := net.ParseIP(endpoint.Hostname())
	if endpoint.Scheme != "https" && (endpoint.Scheme != "http" || loopback == nil || !loopback.IsLoopback()) {
		return fmt.Errorf("keycloak: base_url requires HTTPS (HTTP allowed only for literal loopback fixtures)")
	}
	for _, segment := range strings.Split(endpoint.EscapedPath(), "/") {
		if segment != "" && !safeSegment(segment) {
			return fmt.Errorf("keycloak: base_url has an unsafe path")
		}
	}
	if !safeSegment(creds["realm"]) {
		return fmt.Errorf("keycloak: realm must be one literal path segment")
	}
	adapter.connection = &connection{
		baseURL: endpoint.String(), realm: creds["realm"], clientID: creds["client_id"], clientSecret: creds["client_secret"],
		client: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}
	return nil
}

func safeSegment(value string) bool {
	if value == "" || value == "." || value == ".." || len(value) > 256 {
		return false
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_' || char == '.' || char == '~' {
			continue
		}
		return false
	}
	return true
}

func (adapter *keycloak) snapshot() *connection {
	adapter.mu.RLock()
	defer adapter.mu.RUnlock()
	return adapter.connection
}

func (adapter *keycloak) Healthy(ctx context.Context) bool {
	result, err := adapter.Execute(ctx, "keycloak_list_users", map[string]any{"max": 1})
	return err == nil && result != nil && !result.IsError
}
func (adapter *keycloak) Tools() []mcp.ToolDefinition { return tools }
func (adapter *keycloak) CompactSpec(name mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[name]
	return fields, ok
}
func (adapter *keycloak) Execute(ctx context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	handler, ok := dispatch[name]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("keycloak: unknown tool: %s", name))
	}
	for key := range args {
		allowed := false
		for _, tool := range tools {
			if tool.Name == name {
				_, allowed = tool.Parameters[key]
				break
			}
		}
		if !allowed {
			return mcp.ErrResult(fmt.Errorf("keycloak: unknown parameter"))
		}
	}
	current := adapter.snapshot()
	if current == nil {
		return mcp.ErrResult(fmt.Errorf("keycloak: not configured"))
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	result, err := handler(ctx, current, args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if result != nil && len(result.Data) > maxOutputBytes {
		return mcp.ErrResult(fmt.Errorf("keycloak: output exceeds 64 KiB; reduce max or narrow search"))
	}
	return result, nil
}

func (current *connection) accessToken(ctx context.Context, rejected string) (string, error) {
	current.mu.Lock()
	defer current.mu.Unlock()
	if current.token != "" && time.Now().Before(current.expires) && (rejected == "" || current.token != rejected) {
		return current.token, nil
	}
	current.token = ""
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {current.clientID}, "client_secret": {current.clientSecret}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, current.baseURL+"/realms/"+current.realm+"/protocol/openid-connect/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("keycloak: could not create token request")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := current.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("keycloak: token request failed (check TLS, connectivity, and deadline)")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", responseError("token", response)
	}
	var token struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := decodeResponse(response, &token); err != nil {
		return "", err
	}
	if token.AccessToken == "" || len(token.AccessToken) > 32768 || strings.ContainsAny(token.AccessToken, "\r\n\t ") || !strings.EqualFold(token.TokenType, "Bearer") || token.ExpiresIn < 1 || token.ExpiresIn > 86400 {
		return "", fmt.Errorf("keycloak: invalid token response")
	}
	current.token = token.AccessToken
	lifetime := time.Duration(token.ExpiresIn) * time.Second
	current.expires = time.Now().Add(lifetime - min(10*time.Second, lifetime/10))
	return current.token, nil
}

func (current *connection) read(ctx context.Context, path string, query url.Values, target any) error {
	token, err := current.accessToken(ctx, "")
	if err != nil {
		return err
	}
	for attempt := range 2 {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, current.baseURL+"/admin/realms/"+current.realm+path+"?"+query.Encode(), nil)
		if err != nil {
			return fmt.Errorf("keycloak: could not create API request")
		}
		request.Header.Set("Accept", "application/json")
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := current.client.Do(request)
		if err != nil {
			return fmt.Errorf("keycloak: API request failed (check TLS, connectivity, and deadline)")
		}
		if response.StatusCode == http.StatusUnauthorized && attempt == 0 {
			_ = response.Body.Close()
			token, err = current.accessToken(ctx, token)
			if err != nil {
				return err
			}
			continue
		}
		if response.StatusCode != http.StatusOK {
			err = responseError("API", response)
		} else {
			err = decodeResponse(response, target)
		}
		_ = response.Body.Close()
		return err
	}
	return fmt.Errorf("keycloak: authentication failed")
}

func responseError(operation string, response *http.Response) error {
	detail := http.StatusText(response.StatusCode)
	if response.StatusCode == http.StatusTooManyRequests {
		delay := min(mcp.ParseRetryAfter(response.Header.Get("Retry-After")), time.Minute)
		if delay > 0 {
			return fmt.Errorf("keycloak: %s HTTP 429, rate limited; retry after %s (no automatic retry)", operation, delay)
		}
	}
	return fmt.Errorf("keycloak: %s HTTP %d %s", operation, response.StatusCode, detail)
}

func decodeResponse(response *http.Response, target any) error {
	limited := &io.LimitedReader{R: response.Body, N: maxResponseBytes + 1}
	decoder := json.NewDecoder(limited)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("keycloak: invalid or oversized JSON response")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF || limited.N <= 0 {
		return fmt.Errorf("keycloak: invalid or oversized JSON response")
	}
	return nil
}
