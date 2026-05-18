// Package gdocs is the Switchboard adapter for Google Docs v1.
//
// It mirrors the structure of the gmail, gcal, and gdrive adapters: raw HTTP
// against docs.googleapis.com/v1 using a Bearer token, with OAuth refresh
// handled transparently on 401 responses. The OAuth flow itself lives in
// the shared googleoauth package.
//
// The Google Docs API surface is intentionally small — get, create, and
// batchUpdate cover everything — so this adapter exposes those three core
// tools plus a handful of convenience wrappers (insert/append/replace text)
// that compile to batchUpdate requests for the common edit patterns.
package gdocs

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

var compactResult = compact.MustLoadWithOverlay("gdocs", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type gdocs struct {
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
	_ mcp.FieldCompactionIntegration = (*gdocs)(nil)
	_ mcp.MarkdownIntegration        = (*gdocs)(nil)
	_ mcp.PlainTextCredentials       = (*gdocs)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*gdocs)(nil)
)

func (g *gdocs) PlainTextKeys() []string {
	return []string{"base_url"}
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

// New returns a fresh, unconfigured gdocs integration.
func New() mcp.Integration {
	return &gdocs{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://docs.googleapis.com/v1",
	}
}

// SetConfigService wires the registry's config service into the integration
// so refreshed access tokens can be persisted across restarts.
func SetConfigService(i mcp.Integration, svc mcp.ConfigService) {
	if g, ok := i.(*gdocs); ok {
		g.mu.Lock()
		g.configSvc = svc
		g.mu.Unlock()
	}
}

func (g *gdocs) Name() string { return "gdocs" }

func (g *gdocs) Configure(_ context.Context, creds mcp.Credentials) error {
	g.accessToken = creds["access_token"]
	g.refreshToken = creds["refresh_token"]
	g.clientID = creds[mcp.CredKeyClientID]
	g.clientSecret = creds[mcp.CredKeyClientSecret]
	if g.accessToken == "" {
		return fmt.Errorf("gdocs: access_token is required")
	}
	if v := creds["base_url"]; v != "" {
		g.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

// Healthy probes the Docs API. There's no `/about`-style endpoint on Docs
// itself, so we issue a deliberately-invalid `documents/HEALTHCHECK` request.
// Anything other than a 401/403 (which would indicate a credential problem)
// counts as healthy — even a 404 means the token is valid and the request
// reached the API.
func (g *gdocs) Healthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", g.baseURL+"/documents/HEALTHCHECK_PROBE", nil)
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

func (g *gdocs) Tools() []mcp.ToolDefinition {
	return tools
}

func (g *gdocs) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, g, args)
}

func (g *gdocs) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (g *gdocs) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

// --- HTTP helpers ---

func (g *gdocs) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	return g.doRequestInner(ctx, method, g.baseURL+path, body, "application/json", true)
}

func (g *gdocs) buildBody(body any) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (g *gdocs) doRequestInner(ctx context.Context, method, fullURL string, body any, contentType string, canRetry bool) (json.RawMessage, error) {
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
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("gdocs API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gdocs API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (g *gdocs) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return g.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (g *gdocs) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, "POST", path, body)
}

func (g *gdocs) persistToken(token string) {
	if g.configSvc == nil {
		return
	}
	ic, ok := g.configSvc.GetIntegration("gdocs")
	if !ok || ic == nil {
		return
	}
	ic.Credentials["access_token"] = token
	_ = g.configSvc.SetIntegration("gdocs", ic)
}

// --- Handler types ---

type handlerFunc func(ctx context.Context, g *gdocs, args map[string]any) (*mcp.ToolResult, error)

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("gdocs_get_document"):    getDocument,
	mcp.ToolName("gdocs_create_document"): createDocument,
	mcp.ToolName("gdocs_batch_update"):    batchUpdate,
	mcp.ToolName("gdocs_insert_text"):     insertText,
	mcp.ToolName("gdocs_append_text"):     appendText,
	mcp.ToolName("gdocs_replace_text"):    replaceText,
	mcp.ToolName("gdocs_delete_content"):  deleteContent,
}
