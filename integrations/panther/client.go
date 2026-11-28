package panther

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

const (
	// maxResponseBytes bounds one API response; data lake result pages are
	// the largest legitimate payloads.
	maxResponseBytes  = 16 << 20
	maxErrorBodyChars = 500
	tokenSettingsHint = "check the token's permissions in Panther under Settings > Developer Tools > API Tokens"
	userAgent         = "switchboard-panther"
)

// client calls Panther's REST and GraphQL APIs with an API token sent in the
// X-API-Key header.
type client struct {
	token string
	inst  instance
	http  *http.Client
	// pollInterval is the first wait between data lake status checks; it
	// grows to maxPollInterval. Tests shorten it.
	pollInterval    time.Duration
	maxPollInterval time.Duration
}

func newClient(token string, inst instance) *client {
	return &client{
		token: token,
		inst:  inst,
		http: &http.Client{
			Timeout: 60 * time.Second,
			// A redirect could carry the API token to another host.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		pollInterval:    time.Second,
		maxPollInterval: 5 * time.Second,
	}
}

// rest calls a REST endpoint and returns the raw JSON body; 204 returns nil.
func (c *client) rest(ctx context.Context, method, path string, query url.Values, body any) (json.RawMessage, error) {
	target := c.inst.apiBase + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, err
	}
	data, err := c.do(req, method+" "+path)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("panther %s %s: response is not JSON: %s", method, path, truncate(string(data)))
	}
	return data, nil
}

func (c *client) do(req *http.Request, label string) ([]byte, error) {
	req.Header.Set("X-API-Key", c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("panther %s: %w", label, err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	// Status is classified before body problems, so a throttled or failing
	// response stays retryable even when its body is cut short.
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, &mcp.RetryableError{
			StatusCode: resp.StatusCode,
			Err:        statusError(label, resp.StatusCode, data),
			RetryAfter: mcp.ParseRetryAfter(resp.Header.Get("Retry-After")),
		}
	}
	if resp.StatusCode >= 300 {
		return nil, statusError(label, resp.StatusCode, data)
	}
	if readErr != nil {
		return nil, fmt.Errorf("panther %s: read response: %w", label, readErr)
	}
	if len(data) > maxResponseBytes {
		return nil, fmt.Errorf("panther %s: response is larger than %d bytes; narrow the request with filters or a smaller page size", label, maxResponseBytes)
	}
	return data, nil
}

func statusError(label string, status int, body []byte) error {
	detail := strings.TrimSpace(string(body))
	var apiErr struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &apiErr) == nil && apiErr.Message != "" {
		detail = apiErr.Message
	}
	detail = truncate(detail)
	switch {
	case status == http.StatusUnauthorized:
		return fmt.Errorf("panther API error (401) on %s: %s; the API token is missing, expired, or not valid for this instance, %s", label, detail, tokenSettingsHint)
	case status == http.StatusForbidden:
		return fmt.Errorf("panther API error (403) on %s: %s; the API token lacks the permission this call needs (panther_get_permissions lists what it has), %s", label, detail, tokenSettingsHint)
	case status >= 300 && status < 400:
		return fmt.Errorf("panther API error (%d) on %s: redirects are not followed; check instance_url points at your Panther API", status, label)
	}
	return fmt.Errorf("panther API error (%d) on %s: %s", status, label, detail)
}

func truncate(text string) string {
	if len(text) > maxErrorBodyChars {
		return text[:maxErrorBodyChars] + "... (truncated)"
	}
	return text
}

type graphqlError struct {
	Message string `json:"message"`
	Path    []any  `json:"path"`
}

// graphql runs one operation and decodes its data into out. GraphQL reports
// failures in an errors array, often with HTTP 200, so any error fails the
// call even when partial data came back.
func (c *client) graphql(ctx context.Context, operation, query string, variables map[string]any, out any) error {
	payload, err := json.Marshal(map[string]any{"query": query, "variables": variables, "operationName": operation})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.inst.graphqlURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	data, err := c.do(req, "GraphQL "+operation)
	if err != nil {
		return err
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []graphqlError  `json:"errors"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("panther GraphQL %s: invalid JSON response: %s", operation, truncate(string(data)))
	}
	if len(envelope.Errors) > 0 {
		messages := make([]string, 0, len(envelope.Errors))
		for _, e := range envelope.Errors {
			messages = append(messages, e.Message)
		}
		text := truncate(strings.Join(messages, "; "))
		if strings.Contains(strings.ToLower(text), "permission") || strings.Contains(strings.ToLower(text), "unauthorized") {
			text += "; " + tokenSettingsHint
		}
		return fmt.Errorf("panther GraphQL %s failed: %s", operation, text)
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return fmt.Errorf("panther GraphQL %s: response has no data", operation)
	}
	return json.Unmarshal(envelope.Data, out)
}

func (c *client) healthy(ctx context.Context) bool {
	_, err := c.rest(ctx, http.MethodGet, "/api-tokens/self", nil, nil)
	return err == nil
}

// pathID escapes one path segment and refuses values that would change the
// path instead of naming a record.
func pathID(name, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	if value == "." || value == ".." {
		return "", fmt.Errorf("%s %q is not a valid id", name, value)
	}
	return url.PathEscape(value), nil
}

var errNoCredential = errors.New("panther: api_token (API token) or mcp_access_token (Sign in with Panther) is required")
