// Package gslides is the Switchboard adapter for Google Slides v1.
//
// It mirrors the structure of the gmail, gcal, gdrive, gdocs, and gsheets
// adapters: raw HTTP against slides.googleapis.com/v1 using a Bearer token,
// with OAuth refresh handled transparently on 401 responses. The OAuth
// flow itself lives in the shared googleoauth package.
//
// The Google Slides API exposes a tight surface compared to Sheets:
//
//  1. presentations.get/create — the presentation container, slides, and
//     page elements (shapes, text, tables, images, charts).
//  2. presentations.batchUpdate — the workhorse mutation endpoint;
//     virtually every structural change funnels through here.
//  3. presentations.pages.get / .getThumbnail — single-slide access and
//     thumbnail image generation.
package gslides

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

var compactResult = compact.MustLoadWithOverlay("gslides", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes
var viewSets = compactResult.Views

type gslides struct {
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
	_ mcp.FieldCompactionIntegration = (*gslides)(nil)
	_ mcp.PlainTextCredentials       = (*gslides)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*gslides)(nil)
	_ compact.ToolViewsIntegration   = (*gslides)(nil)
)

func (g *gslides) PlainTextKeys() []string {
	return []string{"base_url"}
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

// New returns a fresh, unconfigured gslides integration.
func New() mcp.Integration {
	return &gslides{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://slides.googleapis.com/v1",
	}
}

// SetConfigService wires the registry's config service into the integration
// so refreshed access tokens can be persisted across restarts.
func SetConfigService(i mcp.Integration, svc mcp.ConfigService) {
	if g, ok := i.(*gslides); ok {
		g.mu.Lock()
		g.configSvc = svc
		g.mu.Unlock()
	}
}

func (g *gslides) Name() string { return "gslides" }

func (g *gslides) Configure(_ context.Context, creds mcp.Credentials) error {
	g.accessToken = creds["access_token"]
	g.refreshToken = creds["refresh_token"]
	g.clientID = creds[mcp.CredKeyClientID]
	g.clientSecret = creds[mcp.CredKeyClientSecret]
	if g.accessToken == "" {
		return fmt.Errorf("gslides: access_token is required")
	}
	if v := creds["base_url"]; v != "" {
		g.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

// Healthy probes the Slides API. There's no `/about` endpoint, so we issue
// a deliberately-invalid `presentations/HEALTHCHECK_PROBE` request. Anything
// other than a 401/403 (which would indicate a credential problem) counts
// as healthy — even a 404 means the token is valid and the request reached
// the API.
func (g *gslides) Healthy(ctx context.Context) bool {
	_, err := g.get(ctx, "/presentations/HEALTHCHECK_PROBE")
	return err == nil || strings.Contains(err.Error(), "(404)")
}

func (g *gslides) Tools() []mcp.ToolDefinition {
	return tools
}

func (g *gslides) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, g, args)
}

func (g *gslides) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (g *gslides) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (g *gslides) Views(toolName mcp.ToolName) (compact.ViewSet, bool) {
	vs, ok := viewSets[toolName]
	return vs, ok
}

// --- HTTP helpers ---

func (g *gslides) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	return g.doRequestInner(ctx, method, g.baseURL+path, body, "application/json", true)
}

func (g *gslides) buildBody(body any) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (g *gslides) doRequestInner(ctx context.Context, method, fullURL string, body any, contentType string, canRetry bool) (json.RawMessage, error) {
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

		newToken, rerr := RefreshAccessToken(ctx, g.clientID, g.clientSecret, g.refreshToken)
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
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("gslides API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gslides API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (g *gslides) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return g.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (g *gslides) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, "POST", path, body)
}

func (g *gslides) persistToken(token string) {
	if g.configSvc == nil {
		return
	}
	ic, ok := g.configSvc.GetIntegration("gslides")
	if !ok || ic == nil {
		return
	}
	ic.Credentials["access_token"] = token
	_ = g.configSvc.SetIntegration("gslides", ic)
}

// --- Handler types ---

type handlerFunc func(ctx context.Context, g *gslides, args map[string]any) (*mcp.ToolResult, error)

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("gslides_get_presentation"):    getPresentation,
	mcp.ToolName("gslides_create_presentation"): createPresentation,
	mcp.ToolName("gslides_get_page"):            getPage,
	mcp.ToolName("gslides_get_page_thumbnail"):  getPageThumbnail,
	mcp.ToolName("gslides_batch_update"):        batchUpdate,
}
