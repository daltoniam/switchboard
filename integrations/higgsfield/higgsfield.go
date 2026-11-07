// Package higgsfield proxies a curated set of tools from Higgsfield's hosted
// MCP server (AI image, video, and audio generation).
//
// Higgsfield publishes no API-key REST API for account features (credits,
// history, projects), so the adapter speaks MCP to https://mcp.higgsfield.ai
// with an OAuth access token. The hosted server exposes ~150 tools, many of
// them chat-widget workflows; this adapter exposes a fixed allowlist with
// hand-written descriptions, access classes, and compaction specs, and
// refuses everything else before it reaches the network.
package higgsfield

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/url"
	"slices"
	"strings"
	"sync"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/daltoniam/switchboard/remotemcp"
)

const (
	integrationName = "higgsfield"
	defaultBaseURL  = "https://mcp.higgsfield.ai"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay(integrationName, compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

var (
	_ mcp.Integration                = (*higgsfield)(nil)
	_ mcp.PlainTextCredentials       = (*higgsfield)(nil)
	_ mcp.OptionalCredentials        = (*higgsfield)(nil)
	_ mcp.FieldCompactionIntegration = (*higgsfield)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*higgsfield)(nil)
	_ io.Closer                      = (*higgsfield)(nil)
)

type higgsfield struct {
	mu      sync.RWMutex
	baseURL string
	remote  mcp.Integration
}

// New returns the Higgsfield hosted MCP integration.
func New() mcp.Integration {
	return &higgsfield{baseURL: defaultBaseURL}
}

func (h *higgsfield) Name() string { return integrationName }

func (h *higgsfield) PlainTextKeys() []string { return []string{"base_url"} }

func (h *higgsfield) OptionalKeys() []string { return []string{"base_url"} }

// MCPServerURL returns the hosted MCP origin the web UI uses for OAuth
// discovery, or "" when integration is not a Higgsfield adapter.
func MCPServerURL(integration mcp.Integration) string {
	h, ok := integration.(*higgsfield)
	if !ok || h == nil {
		return ""
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.baseURL
}

func (h *higgsfield) Configure(ctx context.Context, creds mcp.Credentials) error {
	token := strings.TrimSpace(creds["mcp_access_token"])
	if token == "" {
		return fmt.Errorf("higgsfield: mcp_access_token is required")
	}
	baseURL, err := normalizeBaseURL(creds["base_url"])
	if err != nil {
		return err
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	credentials := mcp.Credentials{"access_token": token}
	if h.remote != nil && h.baseURL == baseURL {
		return h.remote.Configure(ctx, credentials)
	}
	remote := remotemcp.New(integrationName, baseURL)
	if err := remote.Configure(ctx, credentials); err != nil {
		return fmt.Errorf("higgsfield: configure remote: %w", errors.Join(err, closeRemote(remote)))
	}
	previous := h.remote
	h.remote = remote
	h.baseURL = baseURL
	return closeRemote(previous)
}

// normalizeBaseURL accepts the origin or the full /mcp endpoint; remotemcp
// appends /mcp itself.
func normalizeBaseURL(baseURL string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return defaultBaseURL, nil
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("higgsfield: invalid base_url: must be an HTTP(S) URL with a host")
	}
	parsed.Path = strings.TrimSuffix(strings.TrimRight(parsed.Path, "/"), "/mcp")
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.RawFragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (h *higgsfield) Healthy(ctx context.Context) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.remote != nil && h.remote.Healthy(ctx)
}

// Tools returns the curated catalog. It is static so search works before the
// hosted server is reachable; callers get a deep copy.
func (h *higgsfield) Tools() []mcp.ToolDefinition {
	out := slices.Clone(tools)
	for i := range out {
		out[i].Parameters = maps.Clone(out[i].Parameters)
		out[i].Required = slices.Clone(out[i].Required)
	}
	return out
}

func (h *higgsfield) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.remote == nil {
		return mcp.ErrResult(mcp.ErrNotConfigured)
	}
	upstream, ok := dispatch[toolName]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("unknown tool: %s", toolName))
	}
	// remotemcp strips its own "<name>_" prefix to recover the upstream name.
	return h.remote.Execute(ctx, mcp.ToolName(integrationName+"_"+upstream), args)
}

func (h *higgsfield) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	remote := h.remote
	h.remote = nil
	return closeRemote(remote)
}

func (h *higgsfield) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (h *higgsfield) MaxBytes(toolName mcp.ToolName) (int, bool) {
	limit, ok := maxBytesByTool[toolName]
	return limit, ok
}

func closeRemote(remote mcp.Integration) error {
	if closer, ok := remote.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
