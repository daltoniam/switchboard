package notion

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/url"
	"slices"
	"strings"
	"sync"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/remotemcp"
)

const (
	defaultHostedBaseURL = "https://mcp.notion.com"
	hostedToolPrefix     = "notion_"
	hostedSearchTool     = mcp.ToolName("notion_notion-search")
)

// hostedMCP proxies Notion's official hosted MCP server. It is the backend the
// notion integration uses when signed in through OAuth (mcp_access_token); the
// tool catalog is discovered from Notion rather than declared here.
type hostedMCP struct {
	mu      sync.RWMutex
	baseURL string
	remote  mcp.Integration
}

func newHostedMCP() *hostedMCP {
	return &hostedMCP{baseURL: defaultHostedBaseURL}
}

// MCPServerURL returns the hosted Notion MCP origin used for OAuth discovery,
// or empty when integration is not the notion adapter.
func MCPServerURL(integration mcp.Integration) string {
	n, ok := integration.(*notion)
	if !ok || n == nil || n.hosted == nil {
		return ""
	}
	n.hosted.mu.RLock()
	defer n.hosted.mu.RUnlock()
	return n.hosted.baseURL
}

func (h *hostedMCP) configure(ctx context.Context, token, rawBaseURL string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("notion: mcp_access_token is required")
	}
	baseURL, err := normalizeHostedBaseURL(rawBaseURL)
	if err != nil {
		return err
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	credentials := mcp.Credentials{"access_token": token}
	if h.remote != nil && h.baseURL == baseURL {
		return h.remote.Configure(ctx, credentials)
	}
	remote := remotemcp.New("notion", baseURL)
	if err := remote.Configure(ctx, credentials); err != nil {
		return fmt.Errorf("notion: configure hosted MCP: %w", errors.Join(err, closeRemote(remote)))
	}
	previous := h.remote
	h.remote = remote
	h.baseURL = baseURL
	return closeRemote(previous)
}

func normalizeHostedBaseURL(baseURL string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return defaultHostedBaseURL, nil
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("notion: invalid mcp_base_url: must be an HTTP(S) URL with a host")
	}
	parsed.Path = strings.TrimSuffix(strings.TrimRight(parsed.Path, "/"), "/mcp")
	parsed.RawPath = strings.TrimSuffix(strings.TrimRight(parsed.RawPath, "/"), "/mcp")
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.RawFragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

// active is nil-safe because tests build the notion struct without a
// hosted backend to exercise the native paths.
func (h *hostedMCP) active() bool {
	if h == nil {
		return false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.remote != nil
}

func (h *hostedMCP) healthy(ctx context.Context) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.remote != nil && h.remote.Healthy(ctx)
}

func (h *hostedMCP) tools() []mcp.ToolDefinition {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.remote == nil {
		return nil
	}
	tools := slices.Clone(h.remote.Tools())
	for i := range tools {
		tool := &tools[i]
		tool.Parameters = maps.Clone(tool.Parameters)
		tool.Required = slices.Clone(tool.Required)
		if tool.Name == hostedSearchTool {
			tool.Description += " Start here to search Notion pages and discover content before fetching or editing it."
		}
	}
	return tools
}

func (h *hostedMCP) execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.remote == nil {
		return mcp.ErrResult(mcp.ErrNotConfigured)
	}
	if len(toolName) > len(hostedToolPrefix) && strings.HasPrefix(string(toolName), hostedToolPrefix) {
		for _, tool := range h.remote.Tools() {
			if tool.Name == toolName {
				return h.remote.Execute(ctx, toolName, args)
			}
		}
	}
	return mcp.ErrResult(fmt.Errorf("unknown tool: %s", toolName))
}

func (h *hostedMCP) close() error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	remote := h.remote
	h.remote = nil
	return closeRemote(remote)
}

func closeRemote(remote mcp.Integration) error {
	if closer, ok := remote.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
