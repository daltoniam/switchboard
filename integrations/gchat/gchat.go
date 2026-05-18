// Package gchat is the Switchboard adapter for the Google Chat API v1.
//
// It mirrors the structure of the gmail, gcal, gdrive, gdocs, gsheets,
// gslides, gforms, and gtasks adapters: raw HTTP against
// chat.googleapis.com/v1 using a Bearer token, with OAuth refresh handled
// transparently on 401 responses. The OAuth flow itself lives in the
// shared googleoauth package.
//
// The Google Chat API exposes:
//
//  1. Spaces — rooms, group DMs, and direct messages. Listed under
//     /spaces, fetched at /spaces/{space}.
//  2. Messages — text and card messages within a space. Listed under
//     /spaces/{space}/messages, individual paths /spaces/{space}/messages/{message}.
//  3. Members — humans and apps that belong to a space. Listed under
//     /spaces/{space}/members.
//
// Note: user-credential (OAuth) authentication only covers a subset of the
// Chat API — most spaces.create / spaces.setup endpoints require service
// account auth with domain-wide delegation. This adapter focuses on the
// read + post surface that user OAuth supports.
package gchat

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

var compactResult = compact.MustLoadWithOverlay("gchat", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type gchat struct {
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
	_ mcp.FieldCompactionIntegration = (*gchat)(nil)
	_ mcp.MarkdownIntegration        = (*gchat)(nil)
	_ mcp.PlainTextCredentials       = (*gchat)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*gchat)(nil)
)

func (g *gchat) PlainTextKeys() []string {
	return []string{"base_url"}
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

// New returns a fresh, unconfigured gchat integration.
func New() mcp.Integration {
	return &gchat{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://chat.googleapis.com/v1",
	}
}

// SetConfigService wires the registry's config service into the integration
// so refreshed access tokens can be persisted across restarts.
func SetConfigService(i mcp.Integration, svc mcp.ConfigService) {
	if g, ok := i.(*gchat); ok {
		g.mu.Lock()
		g.configSvc = svc
		g.mu.Unlock()
	}
}

func (g *gchat) Name() string { return "gchat" }

func (g *gchat) Configure(_ context.Context, creds mcp.Credentials) error {
	g.accessToken = creds["access_token"]
	g.refreshToken = creds["refresh_token"]
	g.clientID = creds[mcp.CredKeyClientID]
	g.clientSecret = creds[mcp.CredKeyClientSecret]
	if g.accessToken == "" {
		return fmt.Errorf("gchat: access_token is required")
	}
	if v := creds["base_url"]; v != "" {
		g.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

// Healthy probes the Chat API by listing spaces with pageSize=1. The
// /spaces endpoint is cheap, idempotent, and validates the user OAuth
// token (as long as the chat.spaces.readonly or chat.spaces scope was
// granted). Anything other than 401/403 counts as healthy.
func (g *gchat) Healthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", g.baseURL+"/spaces?pageSize=1", nil)
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

func (g *gchat) Tools() []mcp.ToolDefinition {
	return tools
}

func (g *gchat) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, g, args)
}

func (g *gchat) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (g *gchat) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

// --- HTTP helpers ---

func (g *gchat) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	return g.doRequestInner(ctx, method, g.baseURL+path, body, "application/json", true)
}

func (g *gchat) buildBody(body any) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (g *gchat) doRequestInner(ctx context.Context, method, fullURL string, body any, contentType string, canRetry bool) (json.RawMessage, error) {
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
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("gchat API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gchat API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (g *gchat) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return g.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (g *gchat) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, "POST", path, body)
}

func (g *gchat) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, "PATCH", path, body)
}

func (g *gchat) delete(ctx context.Context, path string) (json.RawMessage, error) {
	return g.doRequest(ctx, "DELETE", path, nil)
}

func (g *gchat) persistToken(token string) {
	if g.configSvc == nil {
		return
	}
	ic, ok := g.configSvc.GetIntegration("gchat")
	if !ok || ic == nil {
		return
	}
	ic.Credentials["access_token"] = token
	_ = g.configSvc.SetIntegration("gchat", ic)
}

// --- Handler types ---

type handlerFunc func(ctx context.Context, g *gchat, args map[string]any) (*mcp.ToolResult, error)

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("gchat_list_spaces"):    listSpaces,
	mcp.ToolName("gchat_get_space"):      getSpace,
	mcp.ToolName("gchat_list_messages"):  listMessages,
	mcp.ToolName("gchat_get_message"):    getMessage,
	mcp.ToolName("gchat_create_message"): createMessage,
	mcp.ToolName("gchat_update_message"): updateMessage,
	mcp.ToolName("gchat_delete_message"): deleteMessage,
	mcp.ToolName("gchat_list_members"):   listMembers,
}
