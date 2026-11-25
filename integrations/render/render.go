// Package render proxies Render's hosted MCP server (https://mcp.render.com/mcp)
// for Render cloud hosting: services, deploys, logs, metrics, Postgres, Key
// Value, Blueprints, and Workflows, authenticated with a Render API key.
package render

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"sync"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/daltoniam/switchboard/remotemcp"
)

const (
	integrationName = "render"
	defaultBaseURL  = "https://mcp.render.com"
	toolPrefix      = integrationName + "_"
	workspaceParam  = "workspaceId"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay(integrationName, compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

var (
	_ mcp.Integration                = (*render)(nil)
	_ mcp.PlainTextCredentials       = (*render)(nil)
	_ mcp.OptionalCredentials        = (*render)(nil)
	_ mcp.PlaceholderHints           = (*render)(nil)
	_ mcp.FieldCompactionIntegration = (*render)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*render)(nil)
	_ io.Closer                      = (*render)(nil)
)

var workspaceIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// cursorPageTools return a JSON array followed by "\n\n cursor: <value>"
// upstream, which is not JSON, so neither compaction nor scripts could read it.
var cursorPageTools = map[mcp.ToolName]bool{
	"render_list_deploys": true,
	"render_list_events":  true,
}

type render struct {
	mu          sync.RWMutex
	baseURL     string
	workspaceID string
	remote      mcp.Integration
}

// New returns an unconfigured Render integration.
func New() mcp.Integration {
	return &render{baseURL: defaultBaseURL}
}

func (r *render) Name() string { return integrationName }

func (r *render) PlainTextKeys() []string { return []string{"base_url", "workspace_id"} }

func (r *render) OptionalKeys() []string { return []string{"base_url", "workspace_id"} }

func (r *render) Placeholders() map[string]string {
	return map[string]string{
		"api_key":      "rnd_... from Render Dashboard > Account Settings > API Keys",
		"workspace_id": "Default workspace (tea-... or usr-...); blank to pass workspaceId per call",
		"base_url":     defaultBaseURL + " (hosted Render MCP)",
	}
}

func serverURL(integration mcp.Integration) string {
	r, ok := integration.(*render)
	if !ok || r == nil {
		return ""
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.baseURL
}

func (r *render) Configure(ctx context.Context, creds mcp.Credentials) error {
	apiKey := strings.TrimSpace(creds["api_key"])
	if apiKey == "" {
		return fmt.Errorf("render: api_key is required")
	}
	baseURL, err := normalizeBaseURL(creds["base_url"])
	if err != nil {
		return err
	}
	workspaceID := strings.TrimSpace(creds["workspace_id"])
	if workspaceID != "" && !workspaceIDPattern.MatchString(workspaceID) {
		return fmt.Errorf("render: invalid workspace_id: use the tea-... or usr-... ID from render_list_workspaces")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	remoteCreds := mcp.Credentials{"access_token": apiKey}
	if r.remote != nil && r.baseURL == baseURL {
		if err := r.remote.Configure(ctx, remoteCreds); err != nil {
			return fmt.Errorf("render: configure remote: %w", err)
		}
		r.workspaceID = workspaceID
		return nil
	}
	remote := remotemcp.New(integrationName, baseURL)
	if err := remote.Configure(ctx, remoteCreds); err != nil {
		return fmt.Errorf("render: configure remote: %w", errors.Join(err, closeRemote(remote)))
	}
	previous := r.remote
	r.remote = remote
	r.baseURL = baseURL
	r.workspaceID = workspaceID
	return closeRemote(previous)
}

// normalizeBaseURL accepts the origin or the full /mcp endpoint; remotemcp
// appends /mcp itself.
func normalizeBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultBaseURL, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("render: invalid base_url: must be an HTTP(S) URL with a host")
	}
	parsed.Path = strings.TrimSuffix(strings.TrimRight(parsed.Path, "/"), "/mcp")
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.RawFragment = ""
	parsed.User = nil
	return strings.TrimRight(parsed.String(), "/"), nil
}

// Healthy calls list_workspaces rather than only listing tools: the hosted
// server accepts any bearer token at the transport and forwards it to the
// Render API, so only a real API call proves the key works.
func (r *render) Healthy(ctx context.Context) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.remote == nil {
		return false
	}
	result, err := r.remote.Execute(ctx, "render_list_workspaces", map[string]any{})
	return err == nil && result != nil && !result.IsError
}

// Tools returns the static catalog so search works before an API key is set
// and when Render is unreachable.
func (r *render) Tools() []mcp.ToolDefinition {
	return tools
}

func (r *render) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	upstream, ok := dispatch[toolName]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("unknown tool: %s", toolName))
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.remote == nil {
		return mcp.ErrResult(mcp.ErrNotConfigured)
	}
	args = r.withDefaultWorkspace(toolName, args)
	result, err := r.remote.Execute(ctx, mcp.ToolName(toolPrefix+upstream), args)
	if err != nil || result == nil || result.IsError || !cursorPageTools[toolName] {
		return result, err
	}
	result.Data = cursorPage(result.Data)
	return result, nil
}

// withDefaultWorkspace returns a copy of args with the configured workspace
// for workspace-scoped tools, leaving an explicit workspaceId untouched.
func (r *render) withDefaultWorkspace(toolName mcp.ToolName, args map[string]any) map[string]any {
	if r.workspaceID == "" || !scopedTools[toolName] {
		return args
	}
	if _, set := args[workspaceParam]; set {
		return args
	}
	out := make(map[string]any, len(args)+1)
	for k, v := range args {
		out[k] = v
	}
	out[workspaceParam] = r.workspaceID
	return out
}

var scopedTools = func() map[mcp.ToolName]bool {
	scoped := make(map[mcp.ToolName]bool, len(tools))
	for _, tool := range tools {
		if _, ok := tool.Parameters[workspaceParam]; ok {
			scoped[tool.Name] = true
		}
	}
	return scoped
}()

// cursorPage turns "<json array>\n\n cursor: <c>" into
// {"items": [...], "next_cursor": "<c>"}, with next_cursor null when the
// upstream cursor is empty. Anything else is returned as is.
func cursorPage(data string) string {
	body, cursor, found := strings.Cut(data, "\n\n cursor: ")
	body = strings.TrimSpace(body)
	if !found || !strings.HasPrefix(body, "[") || !json.Valid([]byte(body)) {
		return data
	}
	page := struct {
		Items      json.RawMessage `json:"items"`
		NextCursor *string         `json:"next_cursor"`
	}{Items: json.RawMessage(body)}
	if cursor = strings.TrimSpace(cursor); cursor != "" && cursor != `""` {
		page.NextCursor = &cursor
	}
	out, err := json.Marshal(page)
	if err != nil {
		return data
	}
	return string(out)
}

func (r *render) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (r *render) MaxBytes(toolName mcp.ToolName) (int, bool) {
	limit, ok := maxBytesByTool[toolName]
	return limit, ok
}

func (r *render) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	remote := r.remote
	r.remote = nil
	return closeRemote(remote)
}

func closeRemote(remote mcp.Integration) error {
	if closer, ok := remote.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
