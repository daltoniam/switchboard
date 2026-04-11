package homeassistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

var (
	_ mcp.Integration                = (*homeassistant)(nil)
	_ mcp.FieldCompactionIntegration = (*homeassistant)(nil)
	_ mcp.PlainTextCredentials       = (*homeassistant)(nil)
)

type homeassistant struct {
	token   string
	client  *http.Client
	baseURL string
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

func New() mcp.Integration {
	return &homeassistant{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (h *homeassistant) Name() string { return "homeassistant" }

func (h *homeassistant) PlainTextKeys() []string {
	return []string{"base_url"}
}

func (h *homeassistant) Configure(_ context.Context, creds mcp.Credentials) error {
	h.token = creds["token"]
	if h.token == "" {
		return fmt.Errorf("homeassistant: token is required")
	}
	h.baseURL = creds["base_url"]
	if h.baseURL == "" {
		return fmt.Errorf("homeassistant: base_url is required")
	}
	h.baseURL = strings.TrimRight(h.baseURL, "/")
	return nil
}

func (h *homeassistant) Healthy(ctx context.Context) bool {
	if h.token == "" || h.baseURL == "" {
		return false
	}
	_, err := h.get(ctx, "/api/")
	return err == nil
}

func (h *homeassistant) Tools() []mcp.ToolDefinition {
	return tools
}

func (h *homeassistant) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, h, args)
}

func (h *homeassistant) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

// --- HTTP helpers ---

func (h *homeassistant) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, h.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+h.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("homeassistant API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("homeassistant API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (h *homeassistant) doRequestRaw(ctx context.Context, method, path string, body any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, h.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+h.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("homeassistant API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("homeassistant API error (%d): %s", resp.StatusCode, string(data))
	}
	return data, nil
}

func (h *homeassistant) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return h.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (h *homeassistant) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return h.doRequest(ctx, "POST", path, body)
}

func (h *homeassistant) postf(ctx context.Context, body any, pathFmt string, args ...any) (json.RawMessage, error) {
	return h.doRequest(ctx, "POST", fmt.Sprintf(pathFmt, args...), body)
}

func (h *homeassistant) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return h.doRequest(ctx, "DELETE", fmt.Sprintf(pathFmt, args...), nil)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error)

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

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	// States / Entities
	mcp.ToolName("homeassistant_list_states"):  listStates,
	mcp.ToolName("homeassistant_get_state"):    getState,
	mcp.ToolName("homeassistant_set_state"):    setState,
	mcp.ToolName("homeassistant_delete_state"): deleteState,

	// Services
	mcp.ToolName("homeassistant_list_services"): listServices,
	mcp.ToolName("homeassistant_call_service"):  callService,

	// Events
	mcp.ToolName("homeassistant_list_events"): listEvents,
	mcp.ToolName("homeassistant_fire_event"):  fireEvent,

	// History
	mcp.ToolName("homeassistant_get_history"): getHistory,

	// Logbook
	mcp.ToolName("homeassistant_get_logbook"): getLogbook,

	// Config
	mcp.ToolName("homeassistant_get_config"):   getConfig,
	mcp.ToolName("homeassistant_check_config"): checkConfig,

	// Template
	mcp.ToolName("homeassistant_render_template"): renderTemplate,

	// Error Log
	mcp.ToolName("homeassistant_get_error_log"): getErrorLog,

	// Calendars
	mcp.ToolName("homeassistant_list_calendars"):      listCalendars,
	mcp.ToolName("homeassistant_get_calendar_events"): getCalendarEvents,

	// Intents
	mcp.ToolName("homeassistant_handle_intent"): handleIntent,

	// Automations
	mcp.ToolName("homeassistant_get_automation"):    getAutomation,
	mcp.ToolName("homeassistant_save_automation"):   saveAutomation,
	mcp.ToolName("homeassistant_delete_automation"): deleteAutomation,

	// Scenes
	mcp.ToolName("homeassistant_get_scene"):    getScene,
	mcp.ToolName("homeassistant_save_scene"):   saveScene,
	mcp.ToolName("homeassistant_delete_scene"): deleteScene,

	// Scripts
	mcp.ToolName("homeassistant_get_script"):    getScript,
	mcp.ToolName("homeassistant_save_script"):   saveScript,
	mcp.ToolName("homeassistant_delete_script"): deleteScript,
}
