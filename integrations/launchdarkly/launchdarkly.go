package launchdarkly

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
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

var compactResult = compact.MustLoadWithOverlay("launchdarkly", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type launchdarkly struct {
	accessToken string
	client      *http.Client
	baseURL     string
}

const (
	maxResponseSize          = 10 * 1024 * 1024 // 10 MB
	defaultBaseURL           = "https://app.launchdarkly.com"
	apiPrefix                = "/api/v2"
	apiVersion               = "20240415"
	semanticPatchContentType = "application/json; domain-model=launchdarkly.semanticpatch"
	defaultLimit             = 20
	maxLimit                 = 100
)

var (
	_ mcp.Integration                = (*launchdarkly)(nil)
	_ mcp.FieldCompactionIntegration = (*launchdarkly)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*launchdarkly)(nil)
	_ mcp.PlainTextCredentials       = (*launchdarkly)(nil)
	_ mcp.PlaceholderHints           = (*launchdarkly)(nil)
	_ mcp.OptionalCredentials        = (*launchdarkly)(nil)
)

func (l *launchdarkly) PlainTextKeys() []string { return []string{"base_url"} }

func (l *launchdarkly) Placeholders() map[string]string {
	return map[string]string{
		"access_token": "LaunchDarkly personal or service access token (api-...), not an SDK key",
		"base_url":     "https://app.launchdarkly.com (default); EU: https://app.eu.launchdarkly.com; Federal: https://app.launchdarkly.us",
	}
}

func (l *launchdarkly) OptionalKeys() []string { return []string{"base_url"} }

func New() mcp.Integration {
	return &launchdarkly{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: defaultBaseURL,
	}
}

func (l *launchdarkly) Name() string { return "launchdarkly" }

func (l *launchdarkly) Configure(_ context.Context, creds mcp.Credentials) error {
	l.accessToken = strings.TrimSpace(creds["access_token"])
	if l.accessToken == "" {
		return fmt.Errorf("launchdarkly: access_token is required")
	}
	if strings.HasPrefix(l.accessToken, "sdk-") || strings.HasPrefix(l.accessToken, "mob-") {
		return fmt.Errorf("launchdarkly: access_token must be a personal or service access token; SDK keys and mobile keys cannot call the REST API")
	}
	if v := creds["base_url"]; v != "" {
		l.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (l *launchdarkly) Healthy(ctx context.Context) bool {
	if l.client == nil || l.accessToken == "" {
		return false
	}
	_, err := l.get(ctx, "/projects?limit=1")
	return err == nil
}

func (l *launchdarkly) Tools() []mcp.ToolDefinition {
	return tools
}

func (l *launchdarkly) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (l *launchdarkly) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (l *launchdarkly) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, l, args)
}

func (l *launchdarkly) doRequest(ctx context.Context, method, path, contentType string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, l.baseURL+apiPrefix+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", l.accessToken)
	req.Header.Set("LD-API-Version", apiVersion)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		if contentType == "" {
			contentType = "application/json"
		}
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("launchdarkly API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("launchdarkly API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (l *launchdarkly) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return l.doRequest(ctx, http.MethodGet, fmt.Sprintf(pathFmt, args...), "", nil)
}

func (l *launchdarkly) semanticPatch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return l.doRequest(ctx, http.MethodPatch, path, semanticPatchContentType, body)
}

type handlerFunc func(ctx context.Context, l *launchdarkly, args map[string]any) (*mcp.ToolResult, error)

func clampLimit(n int) int {
	if n > maxLimit {
		return maxLimit
	}
	if n < 1 {
		return defaultLimit
	}
	return n
}

func paginationQuery(limit, offset int) url.Values {
	vals := url.Values{}
	vals.Set("limit", strconv.Itoa(clampLimit(limit)))
	vals.Set("offset", strconv.Itoa(offset))
	return vals
}

func setIfNotEmpty(vals url.Values, key, value string) {
	if value != "" {
		vals.Set(key, value)
	}
}

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("launchdarkly_list_projects"):      listProjects,
	mcp.ToolName("launchdarkly_list_environments"):  listEnvironments,
	mcp.ToolName("launchdarkly_list_flags"):         listFlags,
	mcp.ToolName("launchdarkly_get_flag"):           getFlag,
	mcp.ToolName("launchdarkly_list_flag_statuses"): listFlagStatuses,
	mcp.ToolName("launchdarkly_toggle_flag"):        toggleFlag,
}
