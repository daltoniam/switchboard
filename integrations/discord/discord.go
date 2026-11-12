package discord

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/daltoniam/switchboard/version"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("discord", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

const (
	defaultBaseURL  = "https://discord.com/api/v10"
	maxResponseSize = 10 * 1024 * 1024 // 10 MB
)

// discord talks to the Discord REST API as a bot. It never accepts user
// tokens: automating a user account (self-botting) breaks Discord's terms.
type discord struct {
	token   string
	baseURL string
	client  *http.Client
}

var (
	_ mcp.Integration                = (*discord)(nil)
	_ mcp.FieldCompactionIntegration = (*discord)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*discord)(nil)
	_ mcp.PlainTextCredentials       = (*discord)(nil)
	_ mcp.OptionalCredentials        = (*discord)(nil)
	_ mcp.PlaceholderHints           = (*discord)(nil)
)

func New() mcp.Integration { return &discord{} }

func (*discord) Name() string            { return "discord" }
func (*discord) PlainTextKeys() []string { return []string{"base_url"} }
func (*discord) OptionalKeys() []string  { return []string{"base_url"} }
func (*discord) Placeholders() map[string]string {
	return map[string]string{
		"bot_token": "Bot token from Developer Portal → your app → Bot (never a user token)",
		"base_url":  defaultBaseURL,
	}
}

func (d *discord) Configure(_ context.Context, creds mcp.Credentials) error {
	token := strings.TrimSpace(creds["bot_token"])
	token = strings.TrimSpace(strings.TrimPrefix(token, "Bot "))
	if token == "" || token == "Bot" {
		return fmt.Errorf("discord: bot_token is required")
	}
	if strings.ContainsAny(token, "\r\n") {
		return fmt.Errorf("discord: bot_token must not contain newlines")
	}
	d.token = token
	d.baseURL = defaultBaseURL
	if v := strings.TrimSpace(creds["base_url"]); v != "" {
		d.baseURL = strings.TrimRight(v, "/")
	}
	d.client = &http.Client{
		Timeout: 30 * time.Second,
		// The bot token must never follow a redirect to another host.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return nil
}

func (d *discord) Healthy(ctx context.Context) bool {
	if d.client == nil {
		return false
	}
	_, err := d.doRequest(ctx, http.MethodGet, "/users/@me", nil, nil)
	return err == nil
}

func (*discord) Tools() []mcp.ToolDefinition { return tools }

func (d *discord) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("discord: unknown tool: %s", toolName))
	}
	if d.client == nil {
		return mcp.ErrResult(fmt.Errorf("discord: integration is not configured"))
	}
	return fn(ctx, d, args)
}

func (*discord) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (*discord) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (d *discord) doRequest(ctx context.Context, method, path string, query url.Values, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}
	target := d.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bot "+d.token)
	req.Header.Set("User-Agent", "DiscordBot (https://github.com/daltoniam/switchboard, "+version.String()+")")
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		apiErr := fmt.Errorf("discord API error (%d): %s", resp.StatusCode, errorDetail(data))
		if retryable(method, resp.StatusCode) {
			return nil, &mcp.RetryableError{StatusCode: resp.StatusCode, Err: apiErr, RetryAfter: retryAfter(resp.Header, data)}
		}
		return nil, apiErr
	}
	if resp.StatusCode == http.StatusNoContent || len(bytes.TrimSpace(data)) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

// retryable reports whether the server may safely call the tool again. A 429
// means Discord did not process the request. A server error on a send may
// still have delivered the message, so only reads retry on 5xx.
func retryable(method string, status int) bool {
	if status == http.StatusTooManyRequests {
		return true
	}
	return method == http.MethodGet && status >= 500
}

// retryAfter prefers the precise retry_after (float seconds) from a 429 body
// over the whole-second Retry-After header.
func retryAfter(h http.Header, data []byte) time.Duration {
	var limited struct {
		RetryAfter float64 `json:"retry_after"`
	}
	if err := json.Unmarshal(data, &limited); err == nil && limited.RetryAfter > 0 {
		return time.Duration(limited.RetryAfter * float64(time.Second))
	}
	return mcp.ParseRetryAfter(h.Get("Retry-After"))
}

// errorDetail renders Discord's {"message", "code"} error body, falling back
// to the raw body.
func errorDetail(data []byte) string {
	var e struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	}
	if err := json.Unmarshal(data, &e); err == nil && e.Message != "" {
		if e.Code != 0 {
			return fmt.Sprintf("%s (code %d)", e.Message, e.Code)
		}
		return e.Message
	}
	if detail := strings.TrimSpace(string(data)); detail != "" {
		return detail
	}
	return "no error detail"
}
