package airflow

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("airflow", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs

var (
	_ mcp.Integration                = (*airflow)(nil)
	_ mcp.FieldCompactionIntegration = (*airflow)(nil)
	_ mcp.PlainTextCredentials       = (*airflow)(nil)
	_ mcp.PlaceholderHints           = (*airflow)(nil)
)

type airflow struct {
	baseURL  string
	username string
	password string
	client   *http.Client
	mu       sync.Mutex
	token    string
}

func New() mcp.Integration {
	return &airflow{client: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return fmt.Errorf("airflow: redirect refused")
	}}}
}

func (a *airflow) Name() string { return "airflow" }

func (a *airflow) PlainTextKeys() []string { return []string{"base_url", "username"} }

func (a *airflow) Placeholders() map[string]string {
	return map[string]string{
		"base_url":     "https://airflow.example.com (Airflow 3 API; loopback HTTP allowed)",
		"username":     "Airflow auth manager username for /auth/token",
		"password":     "Airflow auth manager password",
		"access_token": "Optional pre-issued JWT (cannot be renewed)",
	}
}

func (a *airflow) Configure(_ context.Context, creds mcp.Credentials) error {
	base := strings.TrimRight(creds["base_url"], "/")
	if base == "" {
		return fmt.Errorf("airflow: base_url is required")
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return fmt.Errorf("airflow: invalid base_url")
	}
	if parsed.Scheme == "http" {
		ip := net.ParseIP(parsed.Hostname())
		if !strings.EqualFold(parsed.Hostname(), "localhost") && (ip == nil || !ip.IsLoopback()) {
			return fmt.Errorf("airflow: base_url must use https unless loopback")
		}
	}
	if creds["access_token"] == "" && (creds["username"] == "" || creds["password"] == "") {
		return fmt.Errorf("airflow: username and password or access_token are required")
	}
	if a.client == nil {
		a.client = New().(*airflow).client
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.baseURL = base
	a.username = creds["username"]
	a.password = creds["password"]
	a.token = creds["access_token"]
	return nil
}

func (a *airflow) Healthy(ctx context.Context) bool {
	if a.client == nil || a.baseURL == "" {
		return false
	}
	_, err := a.request(ctx, http.MethodGet, "/api/v2/dags?limit=1", nil)
	return err == nil
}

func (a *airflow) Tools() []mcp.ToolDefinition { return tools }

func (a *airflow) CompactSpec(name mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[name]
	return fields, ok
}

func (a *airflow) Execute(ctx context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	handler, ok := dispatch[name]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("unknown tool: %s", name))
	}
	return handler(ctx, a, args)
}

func (a *airflow) authenticate(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.token != "" {
		return a.token, nil
	}
	body, err := json.Marshal(map[string]string{"username": a.username, "password": a.password})
	if err != nil {
		return "", err
	}
	data, err := a.send(ctx, http.MethodPost, "/auth/token", body, "")
	if err != nil {
		return "", err
	}
	var response struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(data, &response); err != nil || response.AccessToken == "" {
		return "", fmt.Errorf("airflow: auth response missing access_token")
	}
	a.token = response.AccessToken
	return a.token, nil
}

func (a *airflow) request(ctx context.Context, method, path string, body any) ([]byte, error) {
	if a.client == nil || a.baseURL == "" {
		return nil, fmt.Errorf("airflow: integration not configured")
	}
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("airflow: encode request: %w", err)
		}
	}
	token, err := a.authenticate(ctx)
	if err != nil {
		return nil, err
	}
	data, err := a.send(ctx, method, path, payload, token)
	if err != nil && strings.Contains(err.Error(), "401 Unauthorized") && a.username != "" && a.password != "" {
		a.mu.Lock()
		if a.token == token {
			a.token = ""
		}
		a.mu.Unlock()
		token, authErr := a.authenticate(ctx)
		if authErr != nil {
			return nil, authErr
		}
		return a.send(ctx, method, path, payload, token)
	}
	return data, err
}

func (a *airflow) send(ctx context.Context, method, path string, payload []byte, token string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, a.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("airflow: create request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("airflow: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024+1))
	if err != nil {
		return nil, fmt.Errorf("airflow: read response: %w", err)
	}
	if len(data) > 2*1024*1024 {
		return nil, fmt.Errorf("airflow: response exceeds 2 MiB")
	}
	if resp.StatusCode >= 400 {
		apiErr := fmt.Errorf("airflow API %s", resp.Status)
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			return nil, &mcp.RetryableError{StatusCode: resp.StatusCode, RetryAfter: mcp.ParseRetryAfter(resp.Header.Get("Retry-After")), Err: apiErr}
		}
		return nil, apiErr
	}
	if len(data) == 0 {
		return []byte(`{}`), nil
	}
	return data, nil
}

func listQuery(args map[string]any) (string, error) {
	r := mcp.NewArgs(args)
	limit := r.Int("limit")
	offset := r.Int("offset")
	if err := r.Err(); err != nil {
		return "", err
	}
	if _, present := args["limit"]; !present {
		limit = 25
	}
	if limit < 1 || limit > 100 {
		return "", fmt.Errorf("limit must be between 1 and 100")
	}
	if offset < 0 {
		return "", fmt.Errorf("offset must be non-negative")
	}
	return "?limit=" + strconv.Itoa(limit) + "&offset=" + strconv.Itoa(offset), nil
}

func requiredIDs(args map[string]any, keys ...string) ([]string, error) {
	r := mcp.NewArgs(args)
	ids := make([]string, len(keys))
	for index, key := range keys {
		ids[index] = r.Str(key)
	}
	if err := r.Err(); err != nil {
		return nil, err
	}
	for index, key := range keys {
		id := ids[index]
		if strings.TrimSpace(id) == "" || id == "." || id == ".." {
			return nil, fmt.Errorf("%s is required", key)
		}
		ids[index] = url.PathEscape(id)
	}
	return ids, nil
}

func read(path string, keys ...string) func(context.Context, *airflow, map[string]any) (*mcp.ToolResult, error) {
	return func(ctx context.Context, a *airflow, args map[string]any) (*mcp.ToolResult, error) {
		ids, err := requiredIDs(args, keys...)
		if err != nil {
			return mcp.ErrResult(err)
		}
		requestPath := path
		for _, id := range ids {
			requestPath = strings.Replace(requestPath, "{}", id, 1)
		}
		data, err := a.request(ctx, http.MethodGet, requestPath, nil)
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}
}

func list(path string, keys ...string) func(context.Context, *airflow, map[string]any) (*mcp.ToolResult, error) {
	return func(ctx context.Context, a *airflow, args map[string]any) (*mcp.ToolResult, error) {
		ids, err := requiredIDs(args, keys...)
		if err != nil {
			return mcp.ErrResult(err)
		}
		query, err := listQuery(args)
		if err != nil {
			return mcp.ErrResult(err)
		}
		requestPath := path
		for _, id := range ids {
			requestPath = strings.Replace(requestPath, "{}", id, 1)
		}
		data, err := a.request(ctx, http.MethodGet, requestPath+query, nil)
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}
}

func trigger(ctx context.Context, a *airflow, args map[string]any) (*mcp.ToolResult, error) {
	ids, err := requiredIDs(args, "dag_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	logicalDate := r.Str("logical_date")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if logicalDate != "" {
		if _, err := time.Parse(time.RFC3339, logicalDate); err != nil {
			return mcp.ErrResult(fmt.Errorf("logical_date must be RFC3339: %w", err))
		}
	}
	body := map[string]any{"logical_date": nil}
	if logicalDate != "" {
		body["logical_date"] = logicalDate
	}
	if conf, present := args["conf"]; present {
		object, ok := conf.(map[string]any)
		if !ok {
			return mcp.ErrResult(fmt.Errorf("conf must be a JSON object"))
		}
		body["conf"] = object
	}
	data, err := a.request(ctx, http.MethodPost, "/api/v2/dags/"+ids[0]+"/dagRuns", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
