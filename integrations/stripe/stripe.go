package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

type stripe struct {
	apiKey  string
	account string // optional Stripe-Account header (Connect)
	client  *http.Client
	baseURL string
}

var _ mcp.FieldCompactionIntegration = (*stripe)(nil)

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

func New() mcp.Integration {
	return &stripe{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://api.stripe.com/v1",
	}
}

func (s *stripe) Name() string { return "stripe" }

func (s *stripe) Configure(_ context.Context, creds mcp.Credentials) error {
	s.apiKey = creds["api_key"]
	if s.apiKey == "" {
		return fmt.Errorf("stripe: api_key is required")
	}
	if v := creds["base_url"]; v != "" {
		s.baseURL = strings.TrimRight(v, "/")
	}
	s.account = creds["account"]
	return nil
}

func (s *stripe) Healthy(ctx context.Context) bool {
	_, err := s.get(ctx, "/balance", nil)
	return err == nil
}

func (s *stripe) Tools() []mcp.ToolDefinition {
	return tools
}

func (s *stripe) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, s, args)
}

func (s *stripe) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

// --- HTTP helpers ---

// doRequest performs a Stripe API call. For GET/DELETE, params are query string.
// For POST, params are application/x-www-form-urlencoded body.
func (s *stripe) doRequest(ctx context.Context, method, path string, params map[string]any) (json.RawMessage, error) {
	form := encodeForm(params)

	fullURL := s.baseURL + path
	var bodyReader io.Reader
	switch method {
	case http.MethodGet, http.MethodDelete:
		if form != "" {
			if strings.Contains(fullURL, "?") {
				fullURL += "&" + form
			} else {
				fullURL += "?" + form
			}
		}
	default:
		bodyReader = strings.NewReader(form)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if s.account != "" {
		req.Header.Set("Stripe-Account", s.account)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("stripe API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("stripe API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (s *stripe) get(ctx context.Context, path string, params map[string]any) (json.RawMessage, error) {
	return s.doRequest(ctx, http.MethodGet, path, params)
}

func (s *stripe) post(ctx context.Context, path string, params map[string]any) (json.RawMessage, error) {
	return s.doRequest(ctx, http.MethodPost, path, params)
}

func (s *stripe) del(ctx context.Context, path string, params map[string]any) (json.RawMessage, error) {
	return s.doRequest(ctx, http.MethodDelete, path, params)
}

// --- Form encoding ---

// encodeForm flattens a params map into Stripe's bracket-notation
// application/x-www-form-urlencoded representation.
//   - {"metadata": {"k": "v"}}     → metadata[k]=v
//   - {"items": [{"price":"p_1"}]} → items[0][price]=p_1
//   - {"expand": ["customer"]}     → expand[0]=customer
//   - scalar values are stringified directly
//
// Keys with empty/nil values are skipped to avoid sending empty Stripe params.
func encodeForm(params map[string]any) string {
	if len(params) == 0 {
		return ""
	}
	vals := url.Values{}
	for k, v := range params {
		flatten(vals, k, v)
	}
	if len(vals) == 0 {
		return ""
	}
	// Stable ordering for tests/idempotency.
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		for j, val := range vals[k] {
			if i > 0 || j > 0 {
				b.WriteByte('&')
			}
			b.WriteString(url.QueryEscape(k))
			b.WriteByte('=')
			b.WriteString(url.QueryEscape(val))
		}
	}
	return b.String()
}

func flatten(out url.Values, key string, val any) {
	switch v := val.(type) {
	case nil:
		return
	case string:
		if v == "" {
			return
		}
		out.Add(key, v)
	case bool:
		out.Add(key, strconv.FormatBool(v))
	case int:
		out.Add(key, strconv.Itoa(v))
	case int32:
		out.Add(key, strconv.FormatInt(int64(v), 10))
	case int64:
		out.Add(key, strconv.FormatInt(v, 10))
	case float64:
		// Stripe amounts are integers in the smallest currency unit; JSON unmarshals
		// numbers as float64. Render as int when there is no fractional part.
		if v == float64(int64(v)) {
			out.Add(key, strconv.FormatInt(int64(v), 10))
		} else {
			out.Add(key, strconv.FormatFloat(v, 'f', -1, 64))
		}
	case json.Number:
		out.Add(key, v.String())
	case map[string]any:
		for k, sub := range v {
			flatten(out, key+"["+k+"]", sub)
		}
	case []any:
		for i, sub := range v {
			flatten(out, key+"["+strconv.Itoa(i)+"]", sub)
		}
	case []string:
		for i, sub := range v {
			out.Add(key+"["+strconv.Itoa(i)+"]", sub)
		}
	default:
		out.Add(key, fmt.Sprintf("%v", v))
	}
}

// --- Handler signature + dispatch map ---

type handlerFunc func(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error)
