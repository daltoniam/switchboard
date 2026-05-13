package fly

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
	"github.com/daltoniam/switchboard/compactyaml"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compactyaml.MustLoadWithOverlay("fly", compactYAML, compactyaml.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type fly struct {
	token   string
	client  *http.Client
	baseURL string
}

var (
	_ mcp.FieldCompactionIntegration = (*fly)(nil)
	_ mcp.PlainTextCredentials       = (*fly)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*fly)(nil)
)

func (f *fly) PlainTextKeys() []string {
	return []string{"base_url"}
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

func New() mcp.Integration {
	return &fly{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://api.machines.dev/v1",
	}
}

func (f *fly) Name() string { return "fly" }

func (f *fly) Configure(_ context.Context, creds mcp.Credentials) error {
	f.token = creds["api_token"]
	if f.token == "" {
		return fmt.Errorf("fly: api_token is required")
	}
	if v := creds["base_url"]; v != "" {
		f.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (f *fly) Healthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", f.baseURL+"/apps?org_slug=personal", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+f.token)
	resp, err := f.client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == 200
}

func (f *fly) Tools() []mcp.ToolDefinition {
	return tools
}

func (f *fly) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, f, args)
}

func (f *fly) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (f *fly) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

// --- HTTP helpers ---

func (f *fly) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, f.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+f.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("fly API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("fly API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || resp.StatusCode == 202 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (f *fly) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return f.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (f *fly) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return f.doRequest(ctx, "POST", path, body)
}

func (f *fly) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return f.doRequest(ctx, "PUT", path, body)
}

func (f *fly) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return f.doRequest(ctx, "PATCH", path, body)
}

func (f *fly) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return f.doRequest(ctx, "DELETE", fmt.Sprintf(pathFmt, args...), nil)
}

func (f *fly) delWithQuery(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
	return f.doRequest(ctx, "DELETE", path+queryEncode(params), nil)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error)

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
	// Apps
	mcp.ToolName("fly_list_apps"):  listApps,
	mcp.ToolName("fly_get_app"):    getApp,
	mcp.ToolName("fly_create_app"): createApp,
	mcp.ToolName("fly_delete_app"): deleteApp,

	// Machines
	mcp.ToolName("fly_list_machines"):   listMachines,
	mcp.ToolName("fly_get_machine"):     getMachine,
	mcp.ToolName("fly_create_machine"):  createMachine,
	mcp.ToolName("fly_update_machine"):  updateMachine,
	mcp.ToolName("fly_delete_machine"):  deleteMachine,
	mcp.ToolName("fly_start_machine"):   startMachine,
	mcp.ToolName("fly_stop_machine"):    stopMachine,
	mcp.ToolName("fly_restart_machine"): restartMachine,
	mcp.ToolName("fly_signal_machine"):  signalMachine,
	mcp.ToolName("fly_wait_machine"):    waitMachine,
	mcp.ToolName("fly_exec_machine"):    execMachine,

	// Volumes
	mcp.ToolName("fly_list_volumes"):          listVolumes,
	mcp.ToolName("fly_get_volume"):            getVolume,
	mcp.ToolName("fly_create_volume"):         createVolume,
	mcp.ToolName("fly_update_volume"):         updateVolume,
	mcp.ToolName("fly_delete_volume"):         deleteVolume,
	mcp.ToolName("fly_list_volume_snapshots"): listVolumeSnapshots,

	// Secrets
	mcp.ToolName("fly_list_secrets"):  listSecrets,
	mcp.ToolName("fly_set_secrets"):   setSecrets,
	mcp.ToolName("fly_unset_secrets"): unsetSecrets,
}
