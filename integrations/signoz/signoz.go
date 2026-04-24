package signoz

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*signoz)(nil)
	_ mcp.FieldCompactionIntegration = (*signoz)(nil)
)

type signoz struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func New() mcp.Integration {
	return &signoz{
		client: &http.Client{},
	}
}

func (s *signoz) Name() string { return "signoz" }

func (s *signoz) Configure(_ context.Context, creds mcp.Credentials) error {
	s.apiKey = creds["api_key"]
	if s.apiKey == "" {
		return fmt.Errorf("signoz: api_key is required")
	}
	s.baseURL = creds["base_url"]
	if s.baseURL == "" {
		return fmt.Errorf("signoz: base_url is required")
	}
	s.baseURL = strings.TrimRight(s.baseURL, "/")
	if creds["skip_verify"] == "true" {
		s.client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec G402 -- user-configured skip_verify for self-signed certs
			},
		}
	}
	return nil
}

func (s *signoz) Healthy(ctx context.Context) bool {
	if s.client == nil || s.apiKey == "" {
		return false
	}
	data, err := s.get(ctx, "/api/v1/version")
	if err != nil {
		return false
	}
	var v struct {
		Version string `json:"version"`
	}
	_ = json.Unmarshal(data, &v)
	return v.Version != ""
}

func (s *signoz) Tools() []mcp.ToolDefinition {
	return tools
}

func (s *signoz) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (s *signoz) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, s, args)
}

// --- HTTP helpers ---

func (s *signoz) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("SIGNOZ-API-KEY", s.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("signoz API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("signoz API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (s *signoz) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return s.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (s *signoz) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return s.doRequest(ctx, "POST", path, body)
}

func (s *signoz) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return s.doRequest(ctx, "PUT", path, body)
}

func (s *signoz) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return s.doRequest(ctx, "DELETE", fmt.Sprintf(pathFmt, args...), nil)
}

// --- Dispatch map ---

type handlerFunc func(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	// Services
	mcp.ToolName("signoz_list_services"):          listServices,
	mcp.ToolName("signoz_get_service_overview"):   getServiceOverview,
	mcp.ToolName("signoz_top_operations"):         topOperations,
	mcp.ToolName("signoz_top_level_operations"):   topLevelOperations,
	mcp.ToolName("signoz_entry_point_operations"): entryPointOperations,

	// Query (logs, traces, metrics)
	mcp.ToolName("signoz_search_logs"):   searchLogs,
	mcp.ToolName("signoz_search_traces"): searchTraces,
	mcp.ToolName("signoz_query_metrics"): queryMetrics,
	mcp.ToolName("signoz_get_trace"):     getTrace,

	// Dashboards
	mcp.ToolName("signoz_list_dashboards"):  listDashboards,
	mcp.ToolName("signoz_get_dashboard"):    getDashboard,
	mcp.ToolName("signoz_create_dashboard"): createDashboard,
	mcp.ToolName("signoz_update_dashboard"): updateDashboard,
	mcp.ToolName("signoz_delete_dashboard"): deleteDashboard,

	// Alerts (Rules)
	mcp.ToolName("signoz_list_alerts"):  listAlerts,
	mcp.ToolName("signoz_get_alert"):    getAlert,
	mcp.ToolName("signoz_create_alert"): createAlert,
	mcp.ToolName("signoz_update_alert"): updateAlert,
	mcp.ToolName("signoz_delete_alert"): deleteAlert,

	// Saved Views
	mcp.ToolName("signoz_list_saved_views"):  listSavedViews,
	mcp.ToolName("signoz_get_saved_view"):    getSavedView,
	mcp.ToolName("signoz_create_saved_view"): createSavedView,
	mcp.ToolName("signoz_update_saved_view"): updateSavedView,
	mcp.ToolName("signoz_delete_saved_view"): deleteSavedView,

	// Notification Channels
	mcp.ToolName("signoz_list_channels"): listChannels,

	// Extras
	mcp.ToolName("signoz_get_version"): getVersion,
}
