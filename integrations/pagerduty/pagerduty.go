package pagerduty

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

var compactResult = compact.MustLoadWithOverlay("pagerduty", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type pagerduty struct {
	apiToken  string
	fromEmail string
	client    *http.Client
	baseURL   string
}

const (
	maxResponseSize = 10 * 1024 * 1024 // 10 MB
	defaultBaseURL  = "https://api.pagerduty.com"
	apiAccept       = "application/vnd.pagerduty+json;version=2"
	defaultLimit    = 25
	maxLimit        = 100
)

var (
	_ mcp.Integration                = (*pagerduty)(nil)
	_ mcp.FieldCompactionIntegration = (*pagerduty)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*pagerduty)(nil)
	_ mcp.PlainTextCredentials       = (*pagerduty)(nil)
	_ mcp.PlaceholderHints           = (*pagerduty)(nil)
	_ mcp.OptionalCredentials        = (*pagerduty)(nil)
)

func (p *pagerduty) PlainTextKeys() []string { return []string{"from_email", "base_url"} }

func (p *pagerduty) Placeholders() map[string]string {
	return map[string]string{
		"api_token":  "PagerDuty REST API user or account token",
		"from_email": "PagerDuty user email required for notes, acknowledge, and resolve",
		"base_url":   "https://api.pagerduty.com (default)",
	}
}

func (p *pagerduty) OptionalKeys() []string { return []string{"from_email", "base_url"} }

func New() mcp.Integration {
	return &pagerduty{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: defaultBaseURL,
	}
}

func (p *pagerduty) Name() string { return "pagerduty" }

func (p *pagerduty) Configure(_ context.Context, creds mcp.Credentials) error {
	p.apiToken = creds["api_token"]
	if p.apiToken == "" {
		return fmt.Errorf("pagerduty: api_token is required")
	}
	p.fromEmail = creds["from_email"]
	if v := creds["base_url"]; v != "" {
		p.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (p *pagerduty) Healthy(ctx context.Context) bool {
	if p.client == nil || p.apiToken == "" {
		return false
	}
	_, err := p.get(ctx, "/incidents?limit=1")
	return err == nil
}

func (p *pagerduty) Tools() []mcp.ToolDefinition {
	return tools
}

func (p *pagerduty) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (p *pagerduty) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (p *pagerduty) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, p, args)
}

func (p *pagerduty) doRequest(ctx context.Context, method, path, from string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token token="+p.apiToken)
	req.Header.Set("Accept", apiAccept)
	if from != "" {
		req.Header.Set("From", from)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("pagerduty API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("pagerduty API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (p *pagerduty) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return p.doRequest(ctx, http.MethodGet, fmt.Sprintf(pathFmt, args...), "", nil)
}

func (p *pagerduty) post(ctx context.Context, path, from string, body any) (json.RawMessage, error) {
	return p.doRequest(ctx, http.MethodPost, path, from, body)
}

func (p *pagerduty) put(ctx context.Context, path, from string, body any) (json.RawMessage, error) {
	return p.doRequest(ctx, http.MethodPut, path, from, body)
}

type handlerFunc func(ctx context.Context, p *pagerduty, args map[string]any) (*mcp.ToolResult, error)

func queryEncode(params map[string]string) string {
	vals := url.Values{}
	for k, v := range params {
		if v != "" {
			vals.Set(k, v)
		}
	}
	if len(vals) == 0 {
		return ""
	}
	return "?" + vals.Encode()
}

func addCSV(vals url.Values, key, raw string) {
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			vals.Add(key, part)
		}
	}
}

func clampLimit(n int) int {
	if n > maxLimit {
		return maxLimit
	}
	return n
}

func paginationQuery(limit, offset int) url.Values {
	vals := url.Values{}
	vals.Set("limit", strconv.Itoa(clampLimit(limit)))
	vals.Set("offset", strconv.Itoa(offset))
	return vals
}

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("pagerduty_list_incidents"):       listIncidents,
	mcp.ToolName("pagerduty_get_incident"):         getIncident,
	mcp.ToolName("pagerduty_list_oncalls"):         listOncalls,
	mcp.ToolName("pagerduty_list_services"):        listServices,
	mcp.ToolName("pagerduty_add_incident_note"):    addIncidentNote,
	mcp.ToolName("pagerduty_acknowledge_incident"): acknowledgeIncident,
	mcp.ToolName("pagerduty_resolve_incident"):     resolveIncident,
}
