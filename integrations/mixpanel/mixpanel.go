package mixpanel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

type mixpanel struct {
	username  string
	secret    string
	projectID string
	client    *http.Client
	baseURL   string
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*mixpanel)(nil)
	_ mcp.FieldCompactionIntegration = (*mixpanel)(nil)
)

func New() mcp.Integration {
	return &mixpanel{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://mixpanel.com/api/query",
	}
}

func (m *mixpanel) Name() string { return "mixpanel" }

func (m *mixpanel) Configure(_ context.Context, creds mcp.Credentials) error {
	m.username = creds["username"]
	m.secret = creds["secret"]
	m.projectID = creds["project_id"]
	if m.username == "" {
		return fmt.Errorf("mixpanel: username is required")
	}
	if m.secret == "" {
		return fmt.Errorf("mixpanel: secret is required")
	}
	if m.projectID == "" {
		return fmt.Errorf("mixpanel: project_id is required")
	}
	if v := creds["base_url"]; v != "" {
		m.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (m *mixpanel) Healthy(ctx context.Context) bool {
	// Lightweight check: query insights with a dummy bookmark_id.
	// A 401 means bad auth; anything else means the service is reachable.
	req, err := http.NewRequestWithContext(ctx, "GET", m.baseURL+"/insights?project_id="+url.QueryEscape(m.projectID)+"&bookmark_id=0", nil)
	if err != nil {
		return false
	}
	req.SetBasicAuth(m.username, m.secret)
	resp, err := m.client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode != 401
}

func (m *mixpanel) Tools() []mcp.ToolDefinition {
	return tools
}

func (m *mixpanel) CompactSpec(toolName string) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (m *mixpanel) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, m, args)
}

// --- HTTP helpers ---

func (m *mixpanel) doGet(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
	params["project_id"] = m.proj(params)

	u := m.baseURL + path + queryEncode(params)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(m.username, m.secret)

	return m.doHTTP(req)
}

func (m *mixpanel) doPostForm(ctx context.Context, path string, params map[string]string, form url.Values) (json.RawMessage, error) {
	// project_id goes as a query param even on POST
	params["project_id"] = m.proj(params)

	u := m.baseURL + path + queryEncode(params)
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(m.username, m.secret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return m.doHTTP(req)
}

func (m *mixpanel) doHTTP(req *http.Request) (json.RawMessage, error) {
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("mixpanel API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("mixpanel API error (%d): %s", resp.StatusCode, string(data))
	}
	if len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

// --- Argument helpers ---

func argStr(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func argInt(args map[string]any, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func argBool(args map[string]any, key string) bool {
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}

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

// proj returns the project_id from params, falling back to the configured default.
func (m *mixpanel) proj(params map[string]string) string {
	if v := params["project_id"]; v != "" {
		return v
	}
	return m.projectID
}

// projFromArgs returns the project_id from tool args, falling back to the configured default.
func (m *mixpanel) projFromArgs(args map[string]any) string {
	if v := argStr(args, "project_id"); v != "" {
		return v
	}
	return m.projectID
}

// --- Dispatch map ---

type handlerFunc func(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error)

var dispatch = map[string]handlerFunc{
	"mixpanel_query_insights":         queryInsights,
	"mixpanel_query_funnels":          queryFunnels,
	"mixpanel_query_retention":        queryRetention,
	"mixpanel_query_segmentation":     querySegmentation,
	"mixpanel_query_event_properties": queryEventProperties,
	"mixpanel_query_profiles":         queryProfiles,
}
