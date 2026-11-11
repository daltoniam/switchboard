// Package obsidian is an adapter for the Obsidian Local REST API community
// plugin (github.com/coddingtonbear/obsidian-local-rest-api), which serves a
// running Obsidian vault over HTTPS on 127.0.0.1:27124 (and optionally plain
// HTTP on 27123).
package obsidian

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
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("obsidian", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

const (
	defaultBaseURL  = "https://127.0.0.1:27124"
	maxResponseSize = 10 * 1024 * 1024
	requestTimeout  = 30 * time.Second
)

type obsidian struct {
	baseURL    string
	apiKey     string
	client     *http.Client
	configured bool
}

var (
	_ mcp.Integration                = (*obsidian)(nil)
	_ mcp.FieldCompactionIntegration = (*obsidian)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*obsidian)(nil)
	_ mcp.MarkdownIntegration        = (*obsidian)(nil)
	_ mcp.PlainTextCredentials       = (*obsidian)(nil)
	_ mcp.PlaceholderHints           = (*obsidian)(nil)
	_ mcp.OptionalCredentials        = (*obsidian)(nil)
)

func New() mcp.Integration {
	return &obsidian{}
}

func (o *obsidian) Name() string { return "obsidian" }

func (o *obsidian) PlainTextKeys() []string {
	return []string{"base_url", "ca_cert", "insecure_skip_tls_verify"}
}

func (o *obsidian) OptionalKeys() []string {
	return []string{"base_url", "ca_cert", "insecure_skip_tls_verify"}
}

func (o *obsidian) Placeholders() map[string]string {
	return map[string]string{
		"base_url":                 "https://127.0.0.1:27124 (default; http://127.0.0.1:27123 if the plugin's non-encrypted server is on)",
		"api_key":                  "API key from Obsidian → Settings → Local REST API",
		"ca_cert":                  "PEM certificate from <base_url>/obsidian-local-rest-api.crt (one line, or newlines as \\n), to trust the plugin's self-signed HTTPS certificate",
		"insecure_skip_tls_verify": "true to skip HTTPS certificate checks (not recommended; prefer ca_cert)",
	}
}

func (o *obsidian) Configure(_ context.Context, creds mcp.Credentials) error {
	o.configured = false
	key := strings.TrimSpace(creds["api_key"])
	if key == "" {
		return fmt.Errorf("obsidian: api_key is required (Obsidian → Settings → Local REST API)")
	}
	base := strings.TrimRight(strings.TrimSpace(creds["base_url"]), "/")
	if base == "" {
		base = defaultBaseURL
	}
	parsed, err := url.Parse(base)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("obsidian: base_url must be an http(s) URL such as %s", defaultBaseURL)
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if v := strings.TrimSpace(creds["insecure_skip_tls_verify"]); v != "" {
		skip, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("obsidian: insecure_skip_tls_verify must be true or false")
		}
		tlsConfig.InsecureSkipVerify = skip //nolint:gosec // explicit user opt-in for the plugin's self-signed certificate
	}
	if pemData := normalizePEM(creds["ca_cert"]); pemData != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(pemData)) {
			return fmt.Errorf("obsidian: ca_cert must be a PEM certificate (download it from %s/obsidian-local-rest-api.crt)", base)
		}
		tlsConfig.RootCAs = pool
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	o.client = &http.Client{Timeout: requestTimeout, Transport: transport}
	o.baseURL = base
	o.apiKey = key
	o.configured = true
	return nil
}

// normalizePEM restores the line breaks a single-line web form input drops:
// escaped \n sequences, or none at all around the BEGIN/END markers.
func normalizePEM(raw string) string {
	s := strings.TrimSpace(strings.ReplaceAll(raw, `\n`, "\n"))
	if s == "" || strings.Contains(s, "\n") {
		return s
	}
	s = strings.ReplaceAll(s, "-----BEGIN CERTIFICATE-----", "-----BEGIN CERTIFICATE-----\n")
	return strings.ReplaceAll(s, "-----END CERTIFICATE-----", "\n-----END CERTIFICATE-----\n")
}

// Healthy calls the server root with the API key. The root answers 200 with
// or without a valid key, so the reported "authenticated" flag is what
// proves the key works.
func (o *obsidian) Healthy(ctx context.Context) bool {
	if !o.configured {
		return false
	}
	data, err := o.do(ctx, request{method: http.MethodGet, path: "/"})
	if err != nil {
		return false
	}
	var status struct {
		Authenticated bool `json:"authenticated"`
	}
	return json.Unmarshal(data, &status) == nil && status.Authenticated
}

func (o *obsidian) Tools() []mcp.ToolDefinition { return tools }

func (o *obsidian) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (o *obsidian) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (o *obsidian) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	if !o.configured {
		return mcp.ErrResult(fmt.Errorf("obsidian: integration is not configured; set api_key"))
	}
	return fn(ctx, o, args)
}

type request struct {
	method      string
	path        string // already escaped
	query       url.Values
	accept      string
	contentType string
	headers     map[string]string
	body        []byte
}

// apiErrorBody is the error shape every Local REST API route returns.
type apiErrorBody struct {
	Message   string `json:"message"`
	ErrorCode int    `json:"errorCode"`
}

func (o *obsidian) do(ctx context.Context, r request) ([]byte, error) {
	target := o.baseURL + r.path
	if len(r.query) > 0 {
		target += "?" + r.query.Encode()
	}
	var body io.Reader
	if r.body != nil {
		body = bytes.NewReader(r.body)
	}
	req, err := http.NewRequestWithContext(ctx, r.method, target, body)
	if err != nil {
		return nil, err
	}
	accept := r.accept
	if accept == "" {
		accept = "application/json"
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	if r.contentType != "" {
		req.Header.Set("Content-Type", r.contentType)
	}
	for k, v := range r.headers {
		req.Header.Set(k, v)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := readLimited(resp.Body, maxResponseSize)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		apiErr := apiError(resp.StatusCode, data)
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: apiErr}
			re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
			return nil, re
		}
		return nil, apiErr
	}
	return data, nil
}

func apiError(status int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	var e apiErrorBody
	if json.Unmarshal(body, &e) == nil && e.Message != "" {
		msg = e.Message
		if e.ErrorCode != 0 {
			msg = fmt.Sprintf("%s (error %d)", msg, e.ErrorCode)
		}
	}
	if len(msg) > 2000 {
		msg = msg[:2000] + "..."
	}
	err := fmt.Errorf("obsidian API error (%d): %s", status, msg)
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		err = fmt.Errorf("%w; check api_key against Obsidian → Settings → Local REST API", err)
	case http.StatusNotFound:
		err = fmt.Errorf("%w; the file, folder, or section was not found (paths are relative to the vault root; see obsidian_list_files)", err)
	}
	return &statusError{status: status, err: err}
}

// statusError carries the HTTP status so handlers can branch on it without
// parsing messages.
type statusError struct {
	status int
	err    error
}

func (e *statusError) Error() string { return e.err.Error() }
func (e *statusError) Unwrap() error { return e.err }

func isStatus(err error, status int) bool {
	var se *statusError
	return errors.As(err, &se) && se.status == status
}

// readLimited fails rather than truncating so a partial note is never handed
// back as if it were complete.
func readLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("obsidian: response body exceeds %d bytes; read a section with heading or block instead", limit)
	}
	return data, nil
}

type handlerFunc func(ctx context.Context, o *obsidian, args map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("obsidian_list_files"):       listFiles,
	mcp.ToolName("obsidian_read_note"):        readNote,
	mcp.ToolName("obsidian_get_document_map"): getDocumentMap,
	mcp.ToolName("obsidian_search"):           search,
	mcp.ToolName("obsidian_query"):            query,
	mcp.ToolName("obsidian_list_tags"):        listTags,
	mcp.ToolName("obsidian_list_commands"):    listCommands,
	mcp.ToolName("obsidian_get_active_note"):  getActiveNote,
	mcp.ToolName("obsidian_write_note"):       writeNote,
	mcp.ToolName("obsidian_append_note"):      appendNote,
	mcp.ToolName("obsidian_patch_note"):       patchNote,
	mcp.ToolName("obsidian_delete_note"):      deleteNote,
	mcp.ToolName("obsidian_execute_command"):  executeCommand,
}
