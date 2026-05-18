// Package gsheets is the Switchboard adapter for Google Sheets v4.
//
// It mirrors the structure of the gmail, gcal, gdrive, and gdocs adapters:
// raw HTTP against sheets.googleapis.com/v4 using a Bearer token, with OAuth
// refresh handled transparently on 401 responses. The OAuth flow itself
// lives in the shared googleoauth package.
//
// The Google Sheets API exposes two main surfaces:
//
//  1. spreadsheets.*  — get/create the spreadsheet container (sheets,
//     properties, banding, charts).
//  2. spreadsheets.values.* — read/write the actual cell values
//     using A1-notation ranges.
//
// We expose the core verbs (get/create, get-values, update, append, clear,
// batchGet, batchUpdate, values.batchUpdate) and rely on tool descriptions
// plus markdown rendering to make spreadsheet round-tripping ergonomic.
package gsheets

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("gsheets", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type gsheets struct {
	accessToken  string
	refreshToken string
	clientID     string
	clientSecret string
	client       *http.Client
	baseURL      string
	configSvc    mcp.ConfigService
	mu           sync.Mutex
}

var (
	_ mcp.FieldCompactionIntegration = (*gsheets)(nil)
	_ mcp.MarkdownIntegration        = (*gsheets)(nil)
	_ mcp.PlainTextCredentials       = (*gsheets)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*gsheets)(nil)
)

func (g *gsheets) PlainTextKeys() []string {
	return []string{"base_url"}
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

// New returns a fresh, unconfigured gsheets integration.
func New() mcp.Integration {
	return &gsheets{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://sheets.googleapis.com/v4",
	}
}

// SetConfigService wires the registry's config service into the integration
// so refreshed access tokens can be persisted across restarts.
func SetConfigService(i mcp.Integration, svc mcp.ConfigService) {
	if g, ok := i.(*gsheets); ok {
		g.mu.Lock()
		g.configSvc = svc
		g.mu.Unlock()
	}
}

func (g *gsheets) Name() string { return "gsheets" }

func (g *gsheets) Configure(_ context.Context, creds mcp.Credentials) error {
	g.accessToken = creds["access_token"]
	g.refreshToken = creds["refresh_token"]
	g.clientID = creds[mcp.CredKeyClientID]
	g.clientSecret = creds[mcp.CredKeyClientSecret]
	if g.accessToken == "" {
		return fmt.Errorf("gsheets: access_token is required")
	}
	if v := creds["base_url"]; v != "" {
		g.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

// Healthy probes the Sheets API. There's no `/about` endpoint, so we issue
// a deliberately-invalid `spreadsheets/HEALTHCHECK_PROBE` request. Anything
// other than a 401/403 (which would indicate a credential problem) counts
// as healthy — even a 404 means the token is valid and the request reached
// the API.
func (g *gsheets) Healthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", g.baseURL+"/spreadsheets/HEALTHCHECK_PROBE", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+g.accessToken)
	resp, err := g.client.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return resp.StatusCode != 401 && resp.StatusCode != 403
}

func (g *gsheets) Tools() []mcp.ToolDefinition {
	return tools
}

func (g *gsheets) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, g, args)
}

func (g *gsheets) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (g *gsheets) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

// --- HTTP helpers ---

func (g *gsheets) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	return g.doRequestInner(ctx, method, g.baseURL+path, body, "application/json", true)
}

func (g *gsheets) buildBody(body any) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (g *gsheets) doRequestInner(ctx context.Context, method, fullURL string, body any, contentType string, canRetry bool) (json.RawMessage, error) {
	bodyReader, err := g.buildBody(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+g.accessToken)
	if body != nil && contentType != "" {
		req.Header.Set("Content-Type", contentType)
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
	if resp.StatusCode == 401 && canRetry && g.refreshToken != "" && g.clientID != "" && g.clientSecret != "" {
		g.mu.Lock()
		currentToken := g.accessToken
		g.mu.Unlock()

		newToken, rerr := RefreshAccessToken(g.clientID, g.clientSecret, g.refreshToken)
		if rerr == nil {
			g.mu.Lock()
			if g.accessToken == currentToken {
				g.accessToken = newToken
				g.persistToken(newToken)
			}
			g.mu.Unlock()
			return g.doRequestInner(ctx, method, fullURL, body, contentType, false)
		}
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("gsheets API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gsheets API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (g *gsheets) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return g.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (g *gsheets) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, "POST", path, body)
}

func (g *gsheets) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, "PUT", path, body)
}

func (g *gsheets) persistToken(token string) {
	if g.configSvc == nil {
		return
	}
	ic, ok := g.configSvc.GetIntegration("gsheets")
	if !ok || ic == nil {
		return
	}
	ic.Credentials["access_token"] = token
	_ = g.configSvc.SetIntegration("gsheets", ic)
}

// --- Handler types ---

type handlerFunc func(ctx context.Context, g *gsheets, args map[string]any) (*mcp.ToolResult, error)

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("gsheets_get_spreadsheet"):     getSpreadsheet,
	mcp.ToolName("gsheets_create_spreadsheet"):  createSpreadsheet,
	mcp.ToolName("gsheets_get_values"):          getValues,
	mcp.ToolName("gsheets_batch_get_values"):    batchGetValues,
	mcp.ToolName("gsheets_update_values"):       updateValues,
	mcp.ToolName("gsheets_append_values"):       appendValues,
	mcp.ToolName("gsheets_clear_values"):        clearValues,
	mcp.ToolName("gsheets_batch_update_values"): batchUpdateValues,
	mcp.ToolName("gsheets_batch_update"):        batchUpdate,
}
