package growthbook

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("growthbook", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

const (
	defaultBaseURL  = "https://api.growthbook.io/api"
	maxResponseSize = 10 * 1024 * 1024
	maxErrorSize    = 4096
	requestTimeout  = 30 * time.Second
)

var (
	_ mcp.Integration                = (*growthbook)(nil)
	_ mcp.FieldCompactionIntegration = (*growthbook)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*growthbook)(nil)
	_ mcp.PlainTextCredentials       = (*growthbook)(nil)
	_ mcp.OptionalCredentials        = (*growthbook)(nil)
	_ mcp.PlaceholderHints           = (*growthbook)(nil)
	_ io.Closer                      = (*growthbook)(nil)
)

type growthbook struct {
	mu      sync.RWMutex
	client  *http.Client
	baseURL string
	apiKey  string
}

// New returns an unconfigured GrowthBook integration.
func New() mcp.Integration { return &growthbook{} }

func (g *growthbook) Name() string { return "growthbook" }

func (g *growthbook) PlainTextKeys() []string { return []string{"base_url", "ca_cert"} }

func (g *growthbook) OptionalKeys() []string { return []string{"base_url", "ca_cert"} }

func (g *growthbook) Placeholders() map[string]string {
	return map[string]string{
		"api_key":  "GrowthBook Secret Key (secret_...) or Personal Access Token, not an SDK client key",
		"base_url": "https://api.growthbook.io/api (Cloud default); self-hosted: your HTTPS API_HOST",
		"ca_cert":  `Optional PEM CA certificate for a self-hosted API, literal \n accepted`,
	}
}

func (g *growthbook) Tools() []mcp.ToolDefinition { return tools }

func (g *growthbook) CompactSpec(name mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[name]
	return fields, ok
}

func (g *growthbook) MaxBytes(name mcp.ToolName) (int, bool) {
	limit, ok := maxBytesByTool[name]
	return limit, ok
}

func (g *growthbook) clear() {
	if g.client != nil {
		g.client.CloseIdleConnections()
	}
	g.client, g.baseURL, g.apiKey = nil, "", ""
}

// Close releases idle connections. It is safe to call more than once.
func (g *growthbook) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.clear()
	return nil
}

func (g *growthbook) Configure(_ context.Context, creds mcp.Credentials) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.clear()

	apiKey := strings.TrimSpace(creds["api_key"])
	if apiKey == "" || len(apiKey) > 4096 || strings.ContainsFunc(apiKey, func(r rune) bool { return r < 33 || r > 126 }) {
		return fmt.Errorf("growthbook: api_key is required, using printable ASCII without whitespace")
	}
	if strings.HasPrefix(apiKey, "sdk-") {
		return fmt.Errorf("growthbook: api_key must be a Secret Key or Personal Access Token; SDK client keys cannot call the REST API")
	}

	baseURL, err := normalizeBaseURL(creds["base_url"])
	if err != nil {
		return err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if cert := strings.TrimSpace(creds["ca_cert"]); cert != "" {
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM([]byte(strings.ReplaceAll(cert, `\n`, "\n"))) {
			return fmt.Errorf("growthbook: ca_cert must contain a PEM CA certificate")
		}
		transport.TLSClientConfig.RootCAs = pool
	}

	g.client = &http.Client{
		Transport: transport,
		Timeout:   requestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	g.baseURL, g.apiKey = baseURL, apiKey
	return nil
}

// normalizeBaseURL accepts the Cloud default, a self-hosted API_HOST, or an
// API_HOST that already ends in /api, and returns the URL ending in /api.
func normalizeBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultBaseURL, nil
	}
	invalid := fmt.Errorf("growthbook: base_url must be an HTTPS URL without credentials, query, fragment or encoded path")
	endpoint, err := url.Parse(raw)
	if err != nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.Opaque != "" ||
		strings.ContainsAny(raw, "?#%\\") || strings.ContainsFunc(raw, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return "", invalid
	}
	if port := endpoint.Port(); port != "" || strings.HasSuffix(endpoint.Host, ":") {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return "", invalid
		}
	}
	for _, segment := range strings.Split(endpoint.Path, "/") {
		if segment == "." || segment == ".." {
			return "", invalid
		}
	}
	base := strings.TrimRight(endpoint.String(), "/")
	if !strings.HasSuffix(base, "/api") {
		base += "/api"
	}
	return base, nil
}

func (g *growthbook) snapshot() (*http.Client, string, string) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.client, g.baseURL, g.apiKey
}

func (g *growthbook) Healthy(ctx context.Context) bool {
	client, _, _ := g.snapshot()
	if client == nil {
		return false
	}
	_, err := g.get(ctx, "/v1/projects", url.Values{"limit": {"1"}})
	return err == nil
}

func (g *growthbook) Execute(ctx context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[name]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("growthbook: unknown tool: %s", name))
	}
	if client, _, _ := g.snapshot(); client == nil {
		return mcp.ErrResult(fmt.Errorf("growthbook: integration is not configured"))
	}
	return fn(ctx, g, args)
}

// apiResponse is a successful GrowthBook API response. Status 202 means the
// API key requires a person to confirm the write before it runs.
type apiResponse struct {
	status int
	body   []byte
}

func (g *growthbook) get(ctx context.Context, path string, query url.Values) (*apiResponse, error) {
	return g.do(ctx, http.MethodGet, path, query, nil)
}

func (g *growthbook) do(ctx context.Context, method, path string, query url.Values, body any) (*apiResponse, error) {
	client, baseURL, apiKey := g.snapshot()
	if client == nil {
		return nil, fmt.Errorf("growthbook: integration is not configured")
	}
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("growthbook: encode request: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	target := baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, fmt.Errorf("growthbook: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("growthbook: %s %s: %w", method, path, redactURLError(err))
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("growthbook: read response: %w", err)
	}
	if len(data) > maxResponseSize {
		return nil, fmt.Errorf("growthbook: response exceeds %d bytes", maxResponseSize)
	}

	switch {
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return nil, &mcp.RetryableError{
			StatusCode: resp.StatusCode,
			Err:        fmt.Errorf("growthbook API error (%d): %s", resp.StatusCode, errorText(data)),
			RetryAfter: mcp.ParseRetryAfter(resp.Header.Get("Retry-After")),
		}
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		return nil, fmt.Errorf("growthbook API returned redirect (%d); check base_url points at the API host", resp.StatusCode)
	case resp.StatusCode >= 400:
		return nil, fmt.Errorf("growthbook API error (%d): %s", resp.StatusCode, errorText(data))
	}
	if len(bytes.TrimSpace(data)) == 0 {
		data = []byte(`{}`)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("growthbook: response from %s is not JSON; check base_url points at the API host", path)
	}
	return &apiResponse{status: resp.StatusCode, body: data}, nil
}

// errorText keeps GrowthBook's structured error (message, code, gates) so
// callers can explain a blocked publish to a person, and bounds its size.
func errorText(data []byte) string {
	text := strings.TrimSpace(string(data))
	if len(text) > maxErrorSize {
		text = text[:maxErrorSize] + "...(truncated)"
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
