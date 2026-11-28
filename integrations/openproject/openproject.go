package openproject

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("openproject", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs

const (
	maxResponseBytes = 2 * 1024 * 1024
	requestTimeout   = 10 * time.Second
	maxRetryDelay    = 2 * time.Second
)

var (
	_ mcp.Integration                = (*openproject)(nil)
	_ mcp.FieldCompactionIntegration = (*openproject)(nil)
	_ mcp.PlainTextCredentials       = (*openproject)(nil)
	_ mcp.OptionalCredentials        = (*openproject)(nil)
	_ mcp.PlaceholderHints           = (*openproject)(nil)
	_ io.Closer                      = (*openproject)(nil)
)

type openproject struct {
	mu          sync.RWMutex
	baseURL     string
	workspaceID int64
	apiToken    string
	client      *http.Client
}

func New() mcp.Integration { return &openproject{} }

func (o *openproject) Name() string { return "openproject" }

func (o *openproject) PlainTextKeys() []string {
	return []string{"base_url", "workspace_id", "ca_cert"}
}

func (o *openproject) OptionalKeys() []string { return []string{"ca_cert"} }

func (o *openproject) Placeholders() map[string]string {
	return map[string]string{
		"base_url":     "HTTPS OpenProject installation URL, e.g. https://projects.example.com/openproject",
		"workspace_id": "Positive numeric project workspace ID; dedicated non-admin account needs View work packages only in this project",
		"api_token":    "API token for that read-only account; Basic username apikey, not a login password; audit all additive role grants",
		"ca_cert":      `Optional PEM CA certificate; literal \n separators accepted, TLS verification stays enabled`,
	}
}

func (o *openproject) Configure(_ context.Context, creds mcp.Credentials) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.client != nil {
		o.client.CloseIdleConnections()
	}
	o.client, o.baseURL, o.workspaceID, o.apiToken = nil, "", 0, ""
	raw := creds["base_url"]
	if len(raw) > 2048 || !utf8.ValidString(raw) {
		return fmt.Errorf("openproject: base_url must be a valid HTTPS installation URL of at most 2048 bytes")
	}
	endpoint, err := url.Parse(raw)
	if err != nil || endpoint == nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.Opaque != "" || strings.ContainsAny(raw, "?#%\\") || strings.ContainsFunc(raw, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) || !validHost(endpoint.Hostname()) {
		return fmt.Errorf("openproject: base_url must be an HTTPS installation URL without credentials, query, fragment, whitespace or encoded paths")
	}
	if strings.Contains(endpoint.Hostname(), ":") && !strings.HasPrefix(endpoint.Host, "[") {
		return fmt.Errorf("openproject: IPv6 base_url hostname must be bracketed")
	}
	if strings.Contains(endpoint.Host, "[") && (net.ParseIP(endpoint.Hostname()) == nil || !strings.Contains(endpoint.Hostname(), ":")) {
		return fmt.Errorf("openproject: invalid base_url hostname")
	}
	if strings.HasSuffix(endpoint.Host, ":") {
		return fmt.Errorf("openproject: invalid base_url port")
	}
	if port := endpoint.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("openproject: invalid base_url port")
		}
	}
	prefix := strings.TrimSuffix(endpoint.Path, "/")
	if prefix != "" {
		for _, part := range strings.Split(strings.TrimPrefix(prefix, "/"), "/") {
			if part == "" || part == "." || part == ".." {
				return fmt.Errorf("openproject: invalid base_url installation subpath")
			}
		}
	}
	token := creds["api_token"]
	if token == "" || len(token) > 4096 || !utf8.ValidString(token) || strings.ContainsFunc(token, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) {
		return fmt.Errorf("openproject: api_token is required, at most 4096 bytes, without whitespace or control characters")
	}
	workspace := creds["workspace_id"]
	if workspace == "" || len(workspace) > 19 || workspace[0] < '1' || workspace[0] > '9' || strings.ContainsFunc(workspace, func(r rune) bool { return r < '0' || r > '9' }) {
		return fmt.Errorf("openproject: workspace_id must be a canonical positive decimal integer")
	}
	workspaceID, err := strconv.ParseInt(workspace, 10, 64)
	if err != nil || workspaceID < 1 {
		return fmt.Errorf("openproject: workspace_id must be a canonical positive decimal integer within int64 range")
	}
	cert := creds["ca_cert"]
	if len(cert) > 128*1024 {
		return fmt.Errorf("openproject: ca_cert exceeds 128 KiB limit")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if cert != "" {
		pool, err := certificatePool(cert)
		if err != nil {
			return err
		}
		transport.TLSClientConfig.RootCAs = pool
	}
	client := &http.Client{Transport: transport, Timeout: requestTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	endpoint.Path, endpoint.RawPath = prefix, ""
	o.baseURL, o.workspaceID, o.apiToken, o.client = endpoint.String(), workspaceID, token, client
	return nil
}

func certificatePool(cert string) (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	rest := []byte(strings.ReplaceAll(cert, `\n`, "\n"))
	count := 0
	for len(strings.TrimSpace(string(rest))) > 0 {
		rest = []byte(strings.TrimSpace(string(rest)))
		if !strings.HasPrefix(string(rest), "-----BEGIN CERTIFICATE-----") {
			return nil, fmt.Errorf("openproject: ca_cert must contain only PEM CA certificates")
		}
		end := strings.Index(string(rest), "-----END CERTIFICATE-----")
		if end < 0 {
			return nil, fmt.Errorf("openproject: ca_cert must contain only PEM CA certificates")
		}
		end += len("-----END CERTIFICATE-----")
		encoded := rest[:end]
		if strings.Count(string(encoded), "-----BEGIN ") != 1 {
			return nil, fmt.Errorf("openproject: ca_cert must contain only PEM CA certificates")
		}
		block, remainder := pem.Decode(encoded)
		if block == nil || len(remainder) != 0 || block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			return nil, fmt.Errorf("openproject: ca_cert must contain only PEM CA certificates")
		}
		parsed, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("openproject: ca_cert contains an invalid PEM CA certificate")
		}
		pool.AddCert(parsed)
		rest = rest[end:]
		count++
	}
	if count == 0 {
		return nil, fmt.Errorf("openproject: ca_cert must contain a PEM CA certificate")
	}
	return pool, nil
}

func validHost(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	if len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
		if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			allowed := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-'
			if !allowed {
				return false
			}
		}
	}
	return true
}

func (o *openproject) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.client != nil {
		o.client.CloseIdleConnections()
	}
	o.client, o.baseURL, o.workspaceID, o.apiToken = nil, "", 0, ""
	return nil
}

func (o *openproject) Healthy(ctx context.Context) bool {
	result, err := o.Execute(ctx, "openproject_list_work_packages", map[string]any{"limit": 1})
	return err == nil && result != nil && !result.IsError
}

func (o *openproject) Tools() []mcp.ToolDefinition { return tools }

func (o *openproject) CompactSpec(name mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[name]
	return fields, ok
}

type handlerFunc func(context.Context, *openproject, map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	"openproject_list_work_packages": listWorkPackages,
	"openproject_get_work_package":   getWorkPackage,
}

func (o *openproject) Execute(ctx context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	handler, ok := dispatch[name]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("openproject: unknown tool"))
	}
	var parameters map[string]string
	for _, tool := range tools {
		if tool.Name == name {
			parameters = tool.Parameters
			break
		}
	}
	for key := range args {
		if _, allowed := parameters[key]; !allowed {
			return mcp.ErrResult(fmt.Errorf("openproject: unsupported argument"))
		}
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	o.mu.RLock()
	defer o.mu.RUnlock()
	if o.client == nil {
		return mcp.ErrResult(fmt.Errorf("openproject: %w", mcp.ErrNotConfigured))
	}
	return handler(ctx, o, args)
}

func (o *openproject) get(ctx context.Context, path string, query url.Values, target any) error {
	if o.client == nil {
		return fmt.Errorf("openproject: %w", mcp.ErrNotConfigured)
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	endpoint := o.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return fmt.Errorf("openproject: unable to create fixed API request")
		}
		req.SetBasicAuth("apikey", o.apiToken)
		req.Header.Set("Accept", "application/hal+json")
		resp, err := o.client.Do(req)
		if err != nil {
			return fmt.Errorf("openproject: request failed; check connectivity, verified TLS and context deadline")
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			delay, valid := retryDelay(resp.Header.Get("Retry-After"), time.Now())
			_ = resp.Body.Close()
			if attempt == 2 {
				return fmt.Errorf("openproject: HTTP 429 rate-limit retry exhausted")
			}
			if !valid {
				return fmt.Errorf("openproject: HTTP 429 rate limited; no bounded valid Retry-After")
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return fmt.Errorf("openproject: request canceled during rate-limit wait")
			case <-timer.C:
				continue
			}
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			switch {
			case resp.StatusCode >= 300 && resp.StatusCode < 400:
				return fmt.Errorf("openproject: redirect denied; configure the final HTTPS installation URL")
			case resp.StatusCode == http.StatusUnauthorized:
				return fmt.Errorf("openproject: HTTP 401 authentication denied; check api_token and administrator-enabled API token access for the dedicated account")
			case resp.StatusCode == http.StatusForbidden:
				return fmt.Errorf("openproject: HTTP 403 permission denied; dedicated account needs View work packages in the configured project")
			case resp.StatusCode == http.StatusNotFound:
				return fmt.Errorf("openproject: HTTP 404; work package may be absent or hidden by permission/visibility restrictions")
			default:
				return fmt.Errorf("openproject: HTTP %d API request failed", resp.StatusCode)
			}
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
		_ = resp.Body.Close()
		if len(data) > maxResponseBytes {
			return fmt.Errorf("openproject: response exceeds 2 MiB limit")
		}
		if readErr != nil {
			return fmt.Errorf("openproject: could not read bounded response")
		}
		mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		if err != nil || (mediaType != "application/hal+json" && mediaType != "application/json") {
			return fmt.Errorf("openproject: expected HAL JSON or JSON response; possible HTML login interception")
		}
		if len(data) == 0 || !utf8.Valid(data) || strings.TrimSpace(string(data)) == "null" || json.Unmarshal(data, target) != nil {
			return fmt.Errorf("openproject: invalid JSON API response")
		}
		return nil
	}
	return fmt.Errorf("openproject: HTTP 429 rate-limit retry exhausted")
}

func retryDelay(header string, now time.Time) (time.Duration, bool) {
	if header == "" {
		return 0, false
	}
	if strings.IndexFunc(header, func(r rune) bool { return r < '0' || r > '9' }) == -1 {
		seconds, err := strconv.ParseUint(header, 10, 64)
		if err != nil || seconds > uint64(maxRetryDelay/time.Second) {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}
	deadline, err := http.ParseTime(header)
	if err != nil {
		return 0, false
	}
	delay := deadline.Sub(now)
	if delay < 0 {
		delay = 0
	}
	return delay, delay <= maxRetryDelay
}
