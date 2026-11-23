package jenkins

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

var compactResult = compact.MustLoadWithOverlay("jenkins", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs

const (
	maxResponseBytes = 2 * 1024 * 1024
	requestTimeout   = 15 * time.Second
	maxRetryDelay    = 2 * time.Second
)

var (
	_ mcp.Integration                = (*jenkins)(nil)
	_ mcp.FieldCompactionIntegration = (*jenkins)(nil)
	_ mcp.PlainTextCredentials       = (*jenkins)(nil)
	_ mcp.OptionalCredentials        = (*jenkins)(nil)
	_ mcp.PlaceholderHints           = (*jenkins)(nil)
	_ io.Closer                      = (*jenkins)(nil)
)

type jenkins struct {
	mu       sync.RWMutex
	baseURL  string
	username string
	apiToken string
	client   *http.Client
}

func New() mcp.Integration { return &jenkins{} }

func (j *jenkins) Name() string { return "jenkins" }

func (j *jenkins) PlainTextKeys() []string { return []string{"base_url", "username", "ca_cert"} }

func (j *jenkins) OptionalKeys() []string { return []string{"ca_cert"} }

func (j *jenkins) Placeholders() map[string]string {
	return map[string]string{
		"base_url":  "HTTPS Jenkins installation URL, e.g. https://jenkins.example.com/jenkins",
		"username":  "Dedicated read-only Jenkins username with Overall/Read and intended Job/Read only",
		"api_token": "API token for that read-only account, not a password",
		"ca_cert":   `Optional PEM CA certificate; literal \n separators accepted, TLS verification stays enabled`,
	}
}

func (j *jenkins) Configure(_ context.Context, creds mcp.Credentials) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.client != nil {
		j.client.CloseIdleConnections()
	}
	j.client, j.baseURL, j.username, j.apiToken = nil, "", "", ""
	raw := creds["base_url"]
	endpoint, err := url.Parse(raw)
	if err != nil || endpoint == nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.Opaque != "" || strings.ContainsAny(raw, "?#%\\") || strings.ContainsFunc(raw, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) || !validHost(endpoint.Hostname()) {
		return fmt.Errorf("jenkins: base_url must be an HTTPS installation URL without credentials, query, fragment, whitespace or encoded paths")
	}
	if strings.HasSuffix(endpoint.Host, ":") {
		return fmt.Errorf("jenkins: invalid base_url port")
	}
	if port := endpoint.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("jenkins: invalid base_url port")
		}
	}
	prefix := strings.TrimSuffix(endpoint.Path, "/")
	if prefix != "" {
		for _, part := range strings.Split(strings.TrimPrefix(prefix, "/"), "/") {
			if part == "" || part == "." || part == ".." {
				return fmt.Errorf("jenkins: invalid base_url installation subpath")
			}
		}
	}
	username, token := creds["username"], creds["api_token"]
	if username == "" || utf8.RuneCountInString(username) > 512 || !utf8.ValidString(username) || strings.Contains(username, ":") || strings.ContainsFunc(username, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) {
		return fmt.Errorf("jenkins: username is required without colon, whitespace or control characters")
	}
	if token == "" || len(token) > 4096 || !utf8.ValidString(token) || strings.ContainsFunc(token, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) {
		return fmt.Errorf("jenkins: api_token is required without whitespace or control characters")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if cert := creds["ca_cert"]; cert != "" {
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		rest := []byte(strings.ReplaceAll(cert, `\n`, "\n"))
		count := 0
		for len(strings.TrimSpace(string(rest))) > 0 {
			rest = []byte(strings.TrimSpace(string(rest)))
			if !strings.HasPrefix(string(rest), "-----BEGIN CERTIFICATE-----") {
				return fmt.Errorf("jenkins: ca_cert must contain only PEM CA certificates")
			}
			var block *pem.Block
			block, rest = pem.Decode(rest)
			if block == nil || block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
				return fmt.Errorf("jenkins: ca_cert must contain only PEM CA certificates")
			}
			parsed, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return fmt.Errorf("jenkins: ca_cert contains an invalid PEM CA certificate")
			}
			pool.AddCert(parsed)
			count++
		}
		if count == 0 {
			return fmt.Errorf("jenkins: ca_cert must contain a PEM CA certificate")
		}
		transport.TLSClientConfig.RootCAs = pool
	}
	client := &http.Client{Transport: transport, Timeout: requestTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	endpoint.Path, endpoint.RawPath = prefix, ""
	j.baseURL, j.username, j.apiToken, j.client = endpoint.String(), username, token, client
	return nil
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

func (j *jenkins) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.client != nil {
		j.client.CloseIdleConnections()
	}
	j.client, j.baseURL, j.username, j.apiToken = nil, "", "", ""
	return nil
}

func (j *jenkins) Healthy(ctx context.Context) bool {
	result, err := j.Execute(ctx, "jenkins_list_jobs", map[string]any{"limit": 1})
	return err == nil && result != nil && !result.IsError
}

func (j *jenkins) Tools() []mcp.ToolDefinition { return tools }

func (j *jenkins) CompactSpec(name mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[name]
	return fields, ok
}

type handlerFunc func(context.Context, *jenkins, map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	"jenkins_list_jobs": listJobs,
	"jenkins_get_job":   getJob,
	"jenkins_get_build": getBuild,
}

func (j *jenkins) Execute(ctx context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	handler, ok := dispatch[name]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("jenkins: unknown tool"))
	}
	for key := range args {
		allowed := (name == "jenkins_list_jobs" && key == "folder_path") || (name != "jenkins_list_jobs" && key == "job_path") || (name != "jenkins_get_build" && (key == "offset" || key == "limit")) || (name == "jenkins_get_build" && key == "build_number")
		if !allowed {
			return mcp.ErrResult(fmt.Errorf("jenkins: unsupported argument"))
		}
	}
	return handler(ctx, j, args)
}

func (j *jenkins) get(ctx context.Context, path, tree string, target any) error {
	j.mu.RLock()
	base, username, token, client := j.baseURL, j.username, j.apiToken, j.client
	j.mu.RUnlock()
	if client == nil {
		return fmt.Errorf("jenkins: %w", mcp.ErrNotConfigured)
	}
	defer func() {
		j.mu.RLock()
		retired := j.client != client
		j.mu.RUnlock()
		if retired {
			client.CloseIdleConnections()
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	endpoint := base + path + "/api/json?" + url.Values{"tree": {tree}}.Encode()
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return fmt.Errorf("jenkins: unable to create fixed API request")
		}
		req.SetBasicAuth(username, token)
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("jenkins: request failed; check connectivity, verified TLS and context deadline")
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt == 0 {
			delay, valid := retryDelay(resp.Header.Get("Retry-After"), time.Now())
			_ = resp.Body.Close()
			if valid {
				timer := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return fmt.Errorf("jenkins: request canceled during rate-limit wait")
				case <-timer.C:
					continue
				}
			}
			return fmt.Errorf("jenkins: HTTP 429 rate limited; no bounded valid Retry-After")
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			switch {
			case resp.StatusCode >= 300 && resp.StatusCode < 400:
				return fmt.Errorf("jenkins: redirect denied; configure the final HTTPS installation URL")
			case resp.StatusCode == 401 || resp.StatusCode == 403:
				return fmt.Errorf("jenkins: HTTP %d authentication or read permission denied", resp.StatusCode)
			case resp.StatusCode == 404:
				return fmt.Errorf("jenkins: HTTP 404; item may be absent or hidden by permission/visibility restrictions")
			default:
				return fmt.Errorf("jenkins: HTTP %d API request failed", resp.StatusCode)
			}
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
		_ = resp.Body.Close()
		if len(data) > maxResponseBytes {
			return fmt.Errorf("jenkins: response exceeds 2 MiB limit")
		}
		if readErr != nil {
			return fmt.Errorf("jenkins: could not read bounded response")
		}
		mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			return fmt.Errorf("jenkins: expected JSON response; HTML login interception or unsupported proxy")
		}
		if len(data) == 0 || strings.TrimSpace(string(data)) == "null" || json.Unmarshal(data, target) != nil {
			return fmt.Errorf("jenkins: invalid JSON API response; possible HTML login interception")
		}
		return nil
	}
	return fmt.Errorf("jenkins: rate-limit retry exhausted")
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
