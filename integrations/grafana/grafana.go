package grafana

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
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("grafana", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

// sleepingStackHint explains the most common reason Grafana answers with
// something other than its JSON API: a Grafana Cloud free stack that paused
// after inactivity and now serves a wake-up page.
const sleepingStackHint = "if this is a Grafana Cloud free stack it is probably asleep; open the stack in a browser to wake it, then retry. Otherwise check that base_url is the Grafana root URL"

type grafana struct {
	baseURL string
	token   string
	client  *http.Client
}

var (
	_ mcp.Integration                = (*grafana)(nil)
	_ mcp.FieldCompactionIntegration = (*grafana)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*grafana)(nil)
	_ mcp.PlainTextCredentials       = (*grafana)(nil)
	_ mcp.PlaceholderHints           = (*grafana)(nil)
)

func New() mcp.Integration { return &grafana{} }

func (*grafana) Name() string                { return "grafana" }
func (*grafana) Tools() []mcp.ToolDefinition { return tools }
func (*grafana) PlainTextKeys() []string     { return []string{"base_url"} }
func (*grafana) Placeholders() map[string]string {
	return map[string]string{
		"base_url":              "https://<stack>.grafana.net or your self-hosted Grafana root URL",
		"service_account_token": "Service account token (glsa_...) from Administration > Service accounts",
	}
}

func (g *grafana) Configure(_ context.Context, creds mcp.Credentials) error {
	g.client = nil
	raw := strings.TrimSpace(creds["base_url"])
	if raw == "" {
		return fmt.Errorf("grafana: base_url is required")
	}
	token := strings.TrimSpace(creds["service_account_token"])
	if token == "" {
		return fmt.Errorf("grafana: service_account_token is required")
	}
	if strings.ContainsAny(token, "\r\n") {
		return fmt.Errorf("grafana: service_account_token is invalid")
	}
	parsed, err := url.Parse(strings.TrimRight(raw, "/"))
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return fmt.Errorf("grafana: base_url must be an http or https URL, such as https://acme.grafana.net")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return fmt.Errorf("grafana: base_url must not contain a user name, query or fragment")
	}
	g.baseURL = parsed.String()
	g.token = token
	g.client = &http.Client{
		Timeout: 30 * time.Second,
		// A sleeping Grafana Cloud stack redirects to a wake-up page on
		// another host; never follow, so the token stays with Grafana.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return nil
}

// Healthy reads the token's organization, which needs a valid token but no
// particular role.
func (g *grafana) Healthy(ctx context.Context) bool {
	if g.client == nil {
		return false
	}
	_, err := g.doRequest(ctx, http.MethodGet, "/api/org", nil, nil, true)
	return err == nil
}

func (g *grafana) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("grafana: unknown tool: %s", toolName))
	}
	if g.client == nil {
		return mcp.ErrResult(fmt.Errorf("grafana: integration is not configured"))
	}
	return fn(ctx, g, args)
}

func (*grafana) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (*grafana) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

// apiError is a Grafana error that may carry the parsed response body, so
// callers can report per-query errors from /api/ds/query.
type apiError struct {
	status int
	body   any
	msg    string
}

func (e *apiError) Error() string { return e.msg }

// doRequest calls the Grafana HTTP API and returns the decoded JSON body.
// Numbers decode as json.Number so dashboard ids and versions round-trip
// exactly. retrySafe marks requests that only read, which may be retried
// after a server error; any request may retry after a rate limit.
func (g *grafana) doRequest(ctx context.Context, method, path string, query url.Values, body any, retrySafe bool) (any, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("grafana: cannot encode request: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	target := g.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, fmt.Errorf("grafana: cannot build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := g.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("grafana: cannot reach %s: %w", g.baseURL, errors.Unwrap(err))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return nil, fmt.Errorf("grafana: %s redirected to %s instead of answering the API; %s", g.baseURL, redirectTarget(resp), sleepingStackHint)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		if resp.StatusCode >= 400 {
			// The status alone says whether the request can be retried.
			msg := fmt.Sprintf("grafana API error (%d): incomplete response: %v", resp.StatusCode, err)
			return nil, statusError(resp, nil, msg, retrySafe)
		}
		return nil, fmt.Errorf("grafana: incomplete response: %w", err)
	}
	if len(data) > maxResponseSize {
		return nil, fmt.Errorf("grafana: response is larger than %d MB; narrow the search or query", maxResponseSize/(1024*1024))
	}
	if looksLikeHTML(resp.Header.Get("Content-Type"), data) {
		msg := fmt.Sprintf("grafana: %s answered with a web page (HTTP %d) instead of the API; %s", g.baseURL, resp.StatusCode, sleepingStackHint)
		if resp.StatusCode < 400 {
			return nil, errors.New(msg)
		}
		return nil, statusError(resp, nil, msg, retrySafe)
	}

	var parsed any
	if len(bytes.TrimSpace(data)) > 0 {
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		if err := dec.Decode(&parsed); err != nil {
			parsed = nil
			if resp.StatusCode < 300 {
				return nil, fmt.Errorf("grafana: invalid JSON response (HTTP %d)", resp.StatusCode)
			}
		}
	}

	if resp.StatusCode >= 400 {
		msg := fmt.Sprintf("grafana API error (%d): %s", resp.StatusCode, errorDetail(parsed, data))
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			msg += "; check the service account token and that its role allows this action"
		}
		return nil, statusError(resp, parsed, msg, retrySafe)
	}
	return parsed, nil
}

// statusError types a failed response. A rate limit is always retryable,
// since Grafana did not process the request; a server error only when the
// request is safe to repeat.
func statusError(resp *http.Response, parsed any, msg string, retrySafe bool) error {
	apiErr := &apiError{status: resp.StatusCode, body: parsed, msg: msg}
	if resp.StatusCode == http.StatusTooManyRequests || (retrySafe && resp.StatusCode >= 500) {
		return &mcp.RetryableError{StatusCode: resp.StatusCode, Err: apiErr, RetryAfter: mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))}
	}
	return apiErr
}

// redirectTarget names where a redirect points without echoing its query,
// which may carry session state.
func redirectTarget(resp *http.Response) string {
	loc, err := resp.Location()
	if err != nil {
		return "another page"
	}
	return loc.Scheme + "://" + loc.Host + loc.Path
}

func looksLikeHTML(contentType string, data []byte) bool {
	if strings.HasPrefix(strings.ToLower(contentType), "text/html") {
		return true
	}
	head := strings.ToLower(string(bytes.TrimSpace(data[:min(len(data), 64)])))
	return strings.HasPrefix(head, "<!doctype html") || strings.HasPrefix(head, "<html")
}

// errorDetail extracts Grafana's {"message": ...} error text, falling back to
// a short excerpt of the raw body.
func errorDetail(parsed any, data []byte) string {
	if obj, ok := parsed.(map[string]any); ok {
		if msg, ok := obj["message"].(string); ok && msg != "" {
			return msg
		}
	}
	detail := strings.TrimSpace(string(data))
	if detail == "" {
		return "no error detail"
	}
	if len(detail) > 300 {
		detail = detail[:300] + "..."
	}
	return detail
}
