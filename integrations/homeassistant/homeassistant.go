package homeassistant

import (
	"bytes"
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

var (
	_ mcp.Integration              = (*homeassistant)(nil)
	_ mcp.FieldCompactionIntegration = (*homeassistant)(nil)
	_ mcp.PlainTextCredentials     = (*homeassistant)(nil)
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

func (h *homeassistant) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, h, args)
}

func (h *homeassistant) CompactSpec(toolName string) ([]mcp.CompactField, bool) {
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

func rawResult(data json.RawMessage) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
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

// --- Dispatch map ---

var dispatch = map[string]handlerFunc{
	// States / Entities
	"homeassistant_list_states":  listStates,
	"homeassistant_get_state":    getState,
	"homeassistant_set_state":    setState,
	"homeassistant_delete_state": deleteState,

	// Services
	"homeassistant_list_services": listServices,
	"homeassistant_call_service":  callService,

	// Events
	"homeassistant_list_events": listEvents,
	"homeassistant_fire_event":  fireEvent,

	// History
	"homeassistant_get_history": getHistory,

	// Logbook
	"homeassistant_get_logbook": getLogbook,

	// Config
	"homeassistant_get_config":    getConfig,
	"homeassistant_check_config":  checkConfig,

	// Template
	"homeassistant_render_template": renderTemplate,

	// Error Log
	"homeassistant_get_error_log": getErrorLog,

	// Calendars
	"homeassistant_list_calendars":    listCalendars,
	"homeassistant_get_calendar_events": getCalendarEvents,

	// Intents
	"homeassistant_handle_intent": handleIntent,

	// Automations
	"homeassistant_get_automation":    getAutomation,
	"homeassistant_save_automation":   saveAutomation,
	"homeassistant_delete_automation": deleteAutomation,

	// Scenes
	"homeassistant_get_scene":    getScene,
	"homeassistant_save_scene":   saveScene,
	"homeassistant_delete_scene": deleteScene,

	// Scripts
	"homeassistant_get_script":    getScript,
	"homeassistant_save_script":   saveScript,
	"homeassistant_delete_script": deleteScript,
}
