package gong

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("gong", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type gong struct {
	accessKey       string
	accessKeySecret string
	client          *http.Client
	baseURL         string
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

var (
	_ mcp.Integration                = (*gong)(nil)
	_ mcp.FieldCompactionIntegration = (*gong)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*gong)(nil)
	_ mcp.PlainTextCredentials       = (*gong)(nil)
	_ mcp.PlaceholderHints           = (*gong)(nil)
	_ mcp.OptionalCredentials        = (*gong)(nil)
)

func (g *gong) PlainTextKeys() []string { return []string{"base_url"} }

func (g *gong) Placeholders() map[string]string {
	return map[string]string{
		"access_key":        "Gong API access key",
		"access_key_secret": "Gong API access key secret",
		"base_url":          "https://api.gong.io (default) or workspace-specific host",
	}
}

func (g *gong) OptionalKeys() []string { return []string{"base_url"} }

func New() mcp.Integration {
	return &gong{
		client:  &http.Client{Timeout: 60 * time.Second},
		baseURL: "https://api.gong.io",
	}
}

func (g *gong) Name() string { return "gong" }

func (g *gong) Configure(_ context.Context, creds mcp.Credentials) error {
	g.accessKey = creds["access_key"]
	g.accessKeySecret = creds["access_key_secret"]
	if g.accessKey == "" {
		return fmt.Errorf("gong: access_key is required")
	}
	if g.accessKeySecret == "" {
		return fmt.Errorf("gong: access_key_secret is required")
	}
	if v := creds["base_url"]; v != "" {
		g.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (g *gong) Healthy(ctx context.Context) bool {
	_, err := g.get(ctx, "/v2/users")
	return err == nil
}

func (g *gong) Tools() []mcp.ToolDefinition {
	return tools
}

func (g *gong) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (g *gong) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (g *gong) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, g, args)
}

func (g *gong) authHeader() string {
	token := base64.StdEncoding.EncodeToString([]byte(g.accessKey + ":" + g.accessKeySecret))
	return "Basic " + token
}

func (g *gong) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, g.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", g.authHeader())
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("gong API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gong API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (g *gong) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return g.doRequest(ctx, http.MethodGet, fmt.Sprintf(pathFmt, args...), nil)
}

func (g *gong) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, http.MethodPost, path, body)
}

type handlerFunc func(ctx context.Context, g *gong, args map[string]any) (*mcp.ToolResult, error)

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

func cursorParam(args map[string]any) map[string]string {
	r := mcp.NewArgs(args)
	cursor := r.Str("cursor")
	_ = r.Err()
	return map[string]string{"cursor": cursor}
}

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("gong_list_calls"):             listCalls,
	mcp.ToolName("gong_get_call"):               getCall,
	mcp.ToolName("gong_list_calls_extensive"):   listCallsExtensive,
	mcp.ToolName("gong_get_transcripts"):        getTranscripts,
	mcp.ToolName("gong_list_users"):             listUsers,
	mcp.ToolName("gong_get_user"):               getUser,
	mcp.ToolName("gong_list_users_extensive"):   listUsersExtensive,
	mcp.ToolName("gong_list_workspaces"):        listWorkspaces,
	mcp.ToolName("gong_list_library_folders"):   listLibraryFolders,
	mcp.ToolName("gong_get_library_folder"):     getLibraryFolder,
	mcp.ToolName("gong_list_stats_activity"):    listStatsActivity,
	mcp.ToolName("gong_list_stats_interaction"): listStatsInteraction,
	mcp.ToolName("gong_list_stats_scorecards"):  listStatsScorecards,
	mcp.ToolName("gong_list_logs"):              listLogs,
	mcp.ToolName("gong_list_data_privacy"):      listDataPrivacy,
}
