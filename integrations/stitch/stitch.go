package stitch

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
	_ mcp.Integration                = (*stitch)(nil)
	_ mcp.FieldCompactionIntegration = (*stitch)(nil)
	_ mcp.PlainTextCredentials       = (*stitch)(nil)
)

type stitch struct {
	token   string
	client  *http.Client
	baseURL string
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

func New() mcp.Integration {
	return &stitch{
		client:  &http.Client{Timeout: 120 * time.Second},
		baseURL: "https://stitchapiserver-pa.googleapis.com/v1",
	}
}

func (s *stitch) Name() string { return "stitch" }

func (s *stitch) Configure(_ context.Context, creds mcp.Credentials) error {
	s.token = creds["access_token"]
	if s.token == "" {
		return fmt.Errorf("stitch: access_token is required")
	}
	if v := creds["base_url"]; v != "" {
		s.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (s *stitch) Healthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", s.baseURL+"/projects?filter="+url.QueryEscape("view=owned"), nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	resp, err := s.client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == 200
}

func (s *stitch) Tools() []mcp.ToolDefinition {
	return tools
}

func (s *stitch) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, s, args)
}

func (s *stitch) CompactSpec(toolName string) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (s *stitch) PlainTextKeys() []string {
	return []string{"base_url"}
}

// --- HTTP helpers ---

func (s *stitch) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
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
	req.Header.Set("Authorization", "Bearer "+s.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
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
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("stitch API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("stitch API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (s *stitch) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return s.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (s *stitch) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return s.doRequest(ctx, "POST", path, body)
}

func (s *stitch) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return s.doRequest(ctx, "PATCH", path, body)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, s *stitch, args map[string]any) (*mcp.ToolResult, error)

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
	// Projects
	"stitch_list_projects":  listProjects,
	"stitch_create_project": createProject,
	"stitch_get_project":    getProject,

	// Screens
	"stitch_list_screens":              listScreens,
	"stitch_get_screen":                getScreen,
	"stitch_generate_screen_from_text": generateScreenFromText,
	"stitch_edit_screens":              editScreens,
	"stitch_generate_variants":         generateVariants,

	// Design Systems
	"stitch_list_design_systems":  listDesignSystems,
	"stitch_create_design_system": createDesignSystem,
	"stitch_update_design_system": updateDesignSystem,
	"stitch_apply_design_system":  applyDesignSystem,
}
