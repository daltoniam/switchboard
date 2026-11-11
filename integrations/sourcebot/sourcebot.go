package sourcebot

import (
	"bytes"
	"context"
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

var compactResult = compact.MustLoadWithOverlay("sourcebot", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

const (
	maxResponseSize = 10 * 1024 * 1024
	requestTimeout  = 30 * time.Second
	// Ask Sourcebot runs a full agent loop before answering, which routinely
	// takes 30-60 seconds and longer for broad questions.
	askTimeout   = 3 * time.Minute
	clientSource = "switchboard"
)

type sourcebot struct {
	baseURL    string
	apiKey     string
	client     *http.Client
	askClient  *http.Client
	configured bool
}

var (
	_ mcp.Integration                = (*sourcebot)(nil)
	_ mcp.FieldCompactionIntegration = (*sourcebot)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*sourcebot)(nil)
	_ mcp.PlainTextCredentials       = (*sourcebot)(nil)
	_ mcp.PlaceholderHints           = (*sourcebot)(nil)
	_ mcp.OptionalCredentials        = (*sourcebot)(nil)
)

func New() mcp.Integration {
	return &sourcebot{
		client:    newHTTPClient(requestTimeout),
		askClient: newHTTPClient(askTimeout),
	}
}

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, CheckRedirect: sameOriginRedirect}
}

// sameOriginRedirect stops redirects to another origin. Go forwards custom
// headers such as X-Sourcebot-Api-Key on every redirect, so following one
// off the configured instance would hand the API key to that host. An
// upgrade from http to https on the same host is allowed.
func sameOriginRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("sourcebot: stopped after 10 redirects")
	}
	from, to := via[0].URL, req.URL
	sameOrigin := from.Scheme == to.Scheme && from.Host == to.Host
	upgrade := from.Scheme == "http" && to.Scheme == "https" && from.Hostname() == to.Hostname()
	if !sameOrigin && !upgrade {
		return fmt.Errorf("sourcebot: refusing to follow redirect from %s://%s to %s://%s; set base_url to the address Sourcebot is served from", from.Scheme, from.Host, to.Scheme, to.Host)
	}
	return nil
}

func (s *sourcebot) Name() string { return "sourcebot" }

func (s *sourcebot) PlainTextKeys() []string { return []string{"base_url"} }

func (s *sourcebot) Placeholders() map[string]string {
	return map[string]string{
		"base_url": "https://sourcebot.example.com (your Sourcebot instance URL)",
		"api_key":  "Sourcebot API key (sbk_... or sourcebot-...) from Settings → API Keys. Leave blank if anonymous access is enabled",
	}
}

func (s *sourcebot) OptionalKeys() []string { return []string{"api_key"} }

func (s *sourcebot) Configure(_ context.Context, creds mcp.Credentials) error {
	s.configured = false
	base := strings.TrimSpace(creds["base_url"])
	if base == "" {
		return fmt.Errorf("sourcebot: base_url is required")
	}
	base = strings.TrimRight(base, "/")
	base = strings.TrimSuffix(base, "/api")
	parsed, err := url.Parse(base)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("sourcebot: base_url must be an http(s) URL such as https://sourcebot.example.com")
	}
	s.baseURL = base
	s.apiKey = strings.TrimSpace(creds["api_key"])
	if s.client == nil {
		s.client = newHTTPClient(requestTimeout)
	}
	if s.askClient == nil {
		s.askClient = newHTTPClient(askTimeout)
	}
	s.configured = true
	return nil
}

func (s *sourcebot) Healthy(ctx context.Context) bool {
	if !s.configured {
		return false
	}
	// /api/health and /api/version answer without authentication, so they
	// would report a wrong or missing API key as healthy.
	_, _, err := s.do(ctx, s.client, http.MethodGet, "/api/repos?perPage=1", nil)
	return err == nil
}

func (s *sourcebot) Tools() []mcp.ToolDefinition { return tools }

func (s *sourcebot) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (s *sourcebot) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (s *sourcebot) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	if !s.configured {
		return mcp.ErrResult(fmt.Errorf("sourcebot: integration is not configured; set base_url"))
	}
	return fn(ctx, s, args)
}

// serviceError is the error body every Sourcebot API route returns.
type serviceError struct {
	StatusCode int    `json:"statusCode"`
	ErrorCode  string `json:"errorCode"`
	Message    string `json:"message"`
}

func (s *sourcebot) do(ctx context.Context, client *http.Client, method, path string, body any) ([]byte, http.Header, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, bodyReader)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Org-Domain", "~")
	req.Header.Set("X-Sourcebot-Client-Source", clientSource)
	if s.apiKey != "" {
		req.Header.Set("X-Sourcebot-Api-Key", s.apiKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := readLimited(resp.Body, maxResponseSize)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode >= 400 {
		apiErr := apiError(resp.StatusCode, data, s.apiKey != "")
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: apiErr}
			re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
			return nil, nil, re
		}
		return nil, nil, apiErr
	}
	return data, resp.Header, nil
}

func apiError(status int, body []byte, hasKey bool) error {
	var se serviceError
	msg := strings.TrimSpace(string(body))
	if json.Unmarshal(body, &se) == nil && se.Message != "" {
		msg = se.Message
		if se.ErrorCode != "" {
			msg = se.ErrorCode + ": " + msg
		}
	}
	if len(msg) > 2000 {
		msg = msg[:2000] + "..."
	}
	err := fmt.Errorf("sourcebot API error (%d): %s", status, msg)
	if status == http.StatusUnauthorized {
		if hasKey {
			return fmt.Errorf("%w (the api_key was rejected; create a new key under Settings → API Keys)", err)
		}
		return fmt.Errorf("%w (set api_key, or enable anonymous access on the Sourcebot instance)", err)
	}
	return err
}

// readLimited fails rather than truncating so a partial file or result set
// is never handed back as if it were complete.
func readLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("sourcebot: response body exceeds %d bytes; narrow the request", limit)
	}
	return data, nil
}

func (s *sourcebot) getJSON(ctx context.Context, path string, out any) (http.Header, error) {
	data, header, err := s.do(ctx, s.client, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if err := decode(data, out); err != nil {
		return nil, err
	}
	return header, nil
}

func (s *sourcebot) postJSON(ctx context.Context, client *http.Client, path string, body, out any) error {
	data, _, err := s.do(ctx, client, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	return decode(data, out)
}

func decode(data []byte, out any) error {
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("sourcebot: unexpected response: %w", err)
	}
	return nil
}

func totalCount(header http.Header) (int, bool) {
	v := header.Get("X-Total-Count")
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}
	return n, true
}

type handlerFunc func(ctx context.Context, s *sourcebot, args map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("sourcebot_search_code"):          searchCode,
	mcp.ToolName("sourcebot_list_repos"):           listRepos,
	mcp.ToolName("sourcebot_read_file"):            readFile,
	mcp.ToolName("sourcebot_list_tree"):            listTree,
	mcp.ToolName("sourcebot_list_commits"):         listCommits,
	mcp.ToolName("sourcebot_find_references"):      findReferences,
	mcp.ToolName("sourcebot_find_definitions"):     findDefinitions,
	mcp.ToolName("sourcebot_list_language_models"): listLanguageModels,
	mcp.ToolName("sourcebot_ask_codebase"):         askCodebase,
}
