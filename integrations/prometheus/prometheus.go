package prometheus

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var fieldCompactionSpecs = compact.MustLoadWithOverlay("prometheus", compactYAML, compact.Options{Strict: false}).Specs

const maxResponseSize = 2 * 1024 * 1024

var (
	_          mcp.Integration                = (*prometheus)(nil)
	_          mcp.FieldCompactionIntegration = (*prometheus)(nil)
	_          mcp.PlainTextCredentials       = (*prometheus)(nil)
	_          mcp.OptionalCredentials        = (*prometheus)(nil)
	_          mcp.PlaceholderHints           = (*prometheus)(nil)
	headerName                                = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")
)

type prometheus struct {
	baseURL     string
	username    string
	password    string
	bearerToken string
	headers     http.Header
	client      *http.Client
}

type envelope struct {
	Status    string   `json:"status"`
	Data      any      `json:"data"`
	ErrorType string   `json:"errorType,omitempty"`
	Error     string   `json:"error,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
	Infos     []string `json:"infos,omitempty"`
	Truncated bool     `json:"truncated,omitempty"`
}

func New() mcp.Integration                        { return &prometheus{} }
func (p *prometheus) Name() string                { return "prometheus" }
func (p *prometheus) Tools() []mcp.ToolDefinition { return tools }
func (p *prometheus) PlainTextKeys() []string     { return []string{"base_url", "username"} }
func (p *prometheus) OptionalKeys() []string {
	return []string{"username", "password", "bearer_token", "headers", "ca_cert"}
}
func (p *prometheus) Placeholders() map[string]string {
	return map[string]string{"base_url": "Trusted HTTPS Prometheus endpoint, including optional proxy path", "username": "Basic auth username or Grafana Cloud instance ID", "password": "Basic auth password or API token; do not combine with bearer_token", "bearer_token": "Optional bearer token instead of Basic auth", "headers": `Optional JSON tenant headers, e.g. {"X-Scope-OrgID":"tenant"}`, "ca_cert": "Optional PEM CA certificate for a private TLS deployment"}
}
func (p *prometheus) CompactSpec(name mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[name]
	return fields, ok
}

func (p *prometheus) Configure(_ context.Context, creds mcp.Credentials) error {
	p.client = nil
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(creds["base_url"]), "/"))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.ForceQuery || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return fmt.Errorf("prometheus: base_url must be a trusted HTTPS URL without userinfo, query or fragment")
	}
	if parsed.Scheme == "http" {
		address := net.ParseIP(parsed.Hostname())
		if address == nil || !address.IsLoopback() {
			return fmt.Errorf("prometheus: HTTP is allowed only for loopback development endpoints; use HTTPS")
		}
	}
	username, password, token := creds["username"], creds["password"], creds["bearer_token"]
	if (username == "") != (password == "") {
		return fmt.Errorf("prometheus: username and password must be provided together")
	}
	if username != "" && token != "" {
		return fmt.Errorf("prometheus: choose Basic auth or bearer_token, not both")
	}
	if strings.ContainsAny(username, ":\r\n") || strings.ContainsAny(token, "\r\n") {
		return fmt.Errorf("prometheus: invalid authentication credential")
	}
	headers := http.Header{}
	if raw := creds["headers"]; raw != "" {
		var values map[string]string
		if err := json.Unmarshal([]byte(raw), &values); err != nil || values == nil {
			return fmt.Errorf("prometheus: headers must be a JSON object of strings")
		}
		for key, value := range values {
			lower := strings.ToLower(key)
			if !headerName.MatchString(key) || strings.ContainsAny(value, "\r\n") || lower == "authorization" || lower == "cookie" || lower == "host" || lower == "proxy-authorization" || lower == "connection" || lower == "content-length" || lower == "transfer-encoding" || lower == "accept-encoding" {
				return fmt.Errorf("prometheus: invalid or reserved custom header")
			}
			headers.Set(key, value)
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if pem := creds["ca_cert"]; pem != "" {
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM([]byte(pem)) {
			return fmt.Errorf("prometheus: ca_cert must contain a valid PEM CA certificate")
		}
		transport.TLSClientConfig.RootCAs = roots
	}
	p.baseURL, p.username, p.password, p.bearerToken, p.headers = parsed.String(), username, password, token, headers
	p.client = &http.Client{Timeout: 15 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return nil
}

func (p *prometheus) Healthy(ctx context.Context) bool {
	if p.client == nil {
		return false
	}
	_, err := p.get(ctx, "/status/buildinfo", nil)
	return err == nil
}

func (p *prometheus) Execute(ctx context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	handler, ok := dispatch[name]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("unknown tool: %s", name))
	}
	if p.client == nil {
		return mcp.ErrResult(fmt.Errorf("prometheus: not configured"))
	}
	return handler(ctx, p, args)
}

func (p *prometheus) redact(text string) string {
	for _, secret := range []string{p.password, p.bearerToken, p.headers.Get("X-Scope-OrgID")} {
		if secret != "" {
			text = strings.ReplaceAll(text, secret, "[redacted]")
		}
	}
	for _, values := range p.headers {
		for _, secret := range values {
			if secret != "" {
				text = strings.ReplaceAll(text, secret, "[redacted]")
			}
		}
	}
	return text
}

func (p *prometheus) get(ctx context.Context, path string, params url.Values) (*envelope, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/v1"+path+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("prometheus: cannot construct request")
	}
	for key, values := range p.headers {
		request.Header[key] = append([]string(nil), values...)
	}
	request.Header.Set("Accept", "application/json")
	if p.bearerToken != "" {
		request.Header.Set("Authorization", "Bearer "+p.bearerToken)
	} else if p.username != "" {
		request.SetBasicAuth(p.username, p.password)
	}
	response, err := p.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("prometheus: request failed (check endpoint connectivity and TLS)")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode == 401 || response.StatusCode == 403 {
		return nil, fmt.Errorf("prometheus: authentication or permission denied (%d); check credentials and tenant configuration", response.StatusCode)
	}
	if response.StatusCode == 429 || response.StatusCode >= 500 {
		return nil, &mcp.RetryableError{StatusCode: response.StatusCode, RetryAfter: mcp.ParseRetryAfter(response.Header.Get("Retry-After")), Err: fmt.Errorf("prometheus: upstream temporarily unavailable (%d)", response.StatusCode)}
	}
	if response.StatusCode >= 300 && response.StatusCode < 400 {
		return nil, fmt.Errorf("prometheus: redirects are not supported; configure the final trusted endpoint")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("prometheus: incomplete response")
	}
	if len(body) > maxResponseSize {
		return nil, fmt.Errorf("prometheus: response exceeds 2 MiB; narrow the query or filters")
	}
	var result envelope
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("prometheus: invalid JSON API response (%d)", response.StatusCode)
	}
	if result.Status != "success" || response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("prometheus API error (%d, %s): %s", response.StatusCode, p.redact(result.ErrorType), p.redact(result.Error))
	}
	if result.Data == nil {
		return nil, fmt.Errorf("prometheus: success response is missing data")
	}
	for index, text := range result.Warnings {
		result.Warnings[index] = p.redact(text)
	}
	for index, text := range result.Infos {
		result.Infos[index] = p.redact(text)
	}
	return &result, nil
}
