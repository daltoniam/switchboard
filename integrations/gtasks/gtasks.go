// Package gtasks is the Switchboard adapter for the Google Tasks API v1.
//
// It mirrors the structure of the gmail, gcal, gdrive, gdocs, gsheets,
// gslides, and gforms adapters: raw HTTP against tasks.googleapis.com/tasks/v1
// using a Bearer token, with OAuth refresh handled transparently on 401
// responses. The OAuth flow itself lives in the shared googleoauth package.
//
// The Google Tasks API has two resources:
//
//  1. Tasklists — containers for tasks (e.g. "Work", "Groceries"). Accessed
//     under /users/@me/lists.
//  2. Tasks — individual todo items within a tasklist. Accessed under
//     /lists/{tasklist}/tasks. Tasks support hierarchy (parent task) and
//     ordering (previous sibling), surfaced via the `move` endpoint.
package gtasks

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

var compactResult = compact.MustLoadWithOverlay("gtasks", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type gtasks struct {
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
	_ mcp.FieldCompactionIntegration = (*gtasks)(nil)
	_ mcp.MarkdownIntegration        = (*gtasks)(nil)
	_ mcp.PlainTextCredentials       = (*gtasks)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*gtasks)(nil)
)

func (g *gtasks) PlainTextKeys() []string {
	return []string{"base_url"}
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

// New returns a fresh, unconfigured gtasks integration.
func New() mcp.Integration {
	return &gtasks{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://tasks.googleapis.com/tasks/v1",
	}
}

// SetConfigService wires the registry's config service into the integration
// so refreshed access tokens can be persisted across restarts.
func SetConfigService(i mcp.Integration, svc mcp.ConfigService) {
	if g, ok := i.(*gtasks); ok {
		g.mu.Lock()
		g.configSvc = svc
		g.mu.Unlock()
	}
}

func (g *gtasks) Name() string { return "gtasks" }

func (g *gtasks) Configure(_ context.Context, creds mcp.Credentials) error {
	g.accessToken = creds["access_token"]
	g.refreshToken = creds["refresh_token"]
	g.clientID = creds[mcp.CredKeyClientID]
	g.clientSecret = creds[mcp.CredKeyClientSecret]
	if g.accessToken == "" {
		return fmt.Errorf("gtasks: access_token is required")
	}
	if v := creds["base_url"]; v != "" {
		g.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

// Healthy probes the Tasks API by listing tasklists with maxResults=1.
// The /users/@me/lists endpoint is cheap, idempotent, and validates the
// token. Anything other than 401/403 counts as healthy.
func (g *gtasks) Healthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", g.baseURL+"/users/@me/lists?maxResults=1", nil)
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

func (g *gtasks) Tools() []mcp.ToolDefinition {
	return tools
}

func (g *gtasks) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, g, args)
}

func (g *gtasks) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (g *gtasks) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

// --- HTTP helpers ---

func (g *gtasks) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	return g.doRequestInner(ctx, method, g.baseURL+path, body, "application/json", true)
}

func (g *gtasks) buildBody(body any) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (g *gtasks) doRequestInner(ctx context.Context, method, fullURL string, body any, contentType string, canRetry bool) (json.RawMessage, error) {
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
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("gtasks API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gtasks API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (g *gtasks) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return g.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (g *gtasks) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, "POST", path, body)
}

func (g *gtasks) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, "PATCH", path, body)
}

func (g *gtasks) delete(ctx context.Context, path string) (json.RawMessage, error) {
	return g.doRequest(ctx, "DELETE", path, nil)
}

func (g *gtasks) persistToken(token string) {
	if g.configSvc == nil {
		return
	}
	ic, ok := g.configSvc.GetIntegration("gtasks")
	if !ok || ic == nil {
		return
	}
	ic.Credentials["access_token"] = token
	_ = g.configSvc.SetIntegration("gtasks", ic)
}

// --- Handler types ---

type handlerFunc func(ctx context.Context, g *gtasks, args map[string]any) (*mcp.ToolResult, error)

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("gtasks_list_tasklists"):  listTasklists,
	mcp.ToolName("gtasks_create_tasklist"): createTasklist,
	mcp.ToolName("gtasks_delete_tasklist"): deleteTasklist,
	mcp.ToolName("gtasks_list_tasks"):      listTasks,
	mcp.ToolName("gtasks_get_task"):        getTask,
	mcp.ToolName("gtasks_create_task"):     createTask,
	mcp.ToolName("gtasks_update_task"):     updateTask,
	mcp.ToolName("gtasks_delete_task"):     deleteTask,
	mcp.ToolName("gtasks_move_task"):       moveTask,
	mcp.ToolName("gtasks_clear_completed"): clearCompleted,
}
