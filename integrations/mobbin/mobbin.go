// Package mobbin proxies Mobbin's hosted MCP server (https://api.mobbin.com/mcp),
// a searchable library of real app screens, user flows, and website sections.
package mobbin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/remotemcp"
)

const (
	integrationName = "mobbin"
	defaultBaseURL  = "https://api.mobbin.com"
	toolPrefix      = integrationName + "_"
)

var (
	_ mcp.Integration                        = (*mobbin)(nil)
	_ mcp.PlainTextCredentials               = (*mobbin)(nil)
	_ mcp.OptionalCredentials                = (*mobbin)(nil)
	_ io.Closer                              = (*mobbin)(nil)
	_ mcp.PerToolMaxResponseBytesIntegration = (*mobbin)(nil)
)

// dispatch lists the upstream tools this adapter forwards. Every entry runs
// the same proxy call; the map exists so an unknown or newly added upstream
// tool is never forwarded without a reviewed ToolDefinition and access class.
var dispatch = map[mcp.ToolName]bool{
	"mobbin_search_screens":          true,
	"mobbin_search_flows":            true,
	"mobbin_search_sections":         true,
	"mobbin_list_collections":        true,
	"mobbin_get_collection_contents": true,
	"mobbin_create_collection":       true,
	"mobbin_save_content":            true,
	"mobbin_add_comment":             true,
}

type mobbin struct {
	mu      sync.RWMutex
	baseURL string
	remote  mcp.Integration
}

// New returns an unconfigured Mobbin integration.
func New() mcp.Integration {
	return &mobbin{baseURL: defaultBaseURL}
}

func (m *mobbin) Name() string { return integrationName }

func (m *mobbin) PlainTextKeys() []string { return []string{"base_url"} }

func (m *mobbin) OptionalKeys() []string { return []string{"base_url"} }

// MCPServerURL returns the Mobbin MCP origin used for OAuth discovery, or ""
// when integration is not a Mobbin adapter.
func MCPServerURL(integration mcp.Integration) string {
	m, ok := integration.(*mobbin)
	if !ok || m == nil {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.baseURL
}

func (m *mobbin) Configure(ctx context.Context, creds mcp.Credentials) error {
	token := strings.TrimSpace(creds["mcp_access_token"])
	if token == "" {
		return fmt.Errorf("mobbin: mcp_access_token is required")
	}
	baseURL, err := normalizeBaseURL(creds["base_url"])
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	remoteCreds := mcp.Credentials{"access_token": token}
	if m.remote != nil && m.baseURL == baseURL {
		return m.remote.Configure(ctx, remoteCreds)
	}
	remote := remotemcp.New(integrationName, baseURL)
	if err := remote.Configure(ctx, remoteCreds); err != nil {
		return fmt.Errorf("mobbin: configure remote: %w", errors.Join(err, closeRemote(remote)))
	}
	previous := m.remote
	m.remote = remote
	m.baseURL = baseURL
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
		return "", fmt.Errorf("mobbin: invalid base_url: must be an HTTP(S) URL with a host")
	}
	parsed.Path = strings.TrimSuffix(strings.TrimRight(parsed.Path, "/"), "/mcp")
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.RawFragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (m *mobbin) Healthy(ctx context.Context) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.remote != nil && m.remote.Healthy(ctx)
}

// Tools returns the static catalog so search works before sign-in and when
// Mobbin is unreachable.
func (m *mobbin) Tools() []mcp.ToolDefinition {
	return tools
}

func (m *mobbin) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	if !dispatch[toolName] {
		return mcp.ErrResult(fmt.Errorf("unknown tool: %s", toolName))
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.remote == nil {
		return mcp.ErrResult(mcp.ErrNotConfigured)
	}
	return m.remote.Execute(ctx, toolName, args)
}

// imageResultMaxBytes caps tools that return inline preview images. Three
// screens are about 85KB of image data, so the 50KB server default would
// reject almost every search; 2MiB covers the upstream maximum page sizes.
const imageResultMaxBytes = 2 << 20

var imageResultTools = map[mcp.ToolName]bool{
	"mobbin_search_screens":          true,
	"mobbin_search_flows":            true,
	"mobbin_search_sections":         true,
	"mobbin_get_collection_contents": true,
}

func (m *mobbin) MaxResponseBytesForTool(toolName mcp.ToolName) (int, bool) {
	if imageResultTools[toolName] {
		return imageResultMaxBytes, true
	}
	return 0, false
}

func (m *mobbin) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	remote := m.remote
	m.remote = nil
	return closeRemote(remote)
}

func closeRemote(remote mcp.Integration) error {
	if closer, ok := remote.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
