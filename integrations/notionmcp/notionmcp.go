package notionmcp

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
	defaultBaseURL = "https://mcp.notion.com"
	toolPrefix     = "notion-mcp_"
)

var (
	_ mcp.Integration          = (*notionmcp)(nil)
	_ mcp.PlainTextCredentials = (*notionmcp)(nil)
	_ mcp.OptionalCredentials  = (*notionmcp)(nil)
	_ io.Closer                = (*notionmcp)(nil)
)

type notionmcp struct {
	mu      sync.RWMutex
	baseURL string
	remote  mcp.Integration
}

func New() mcp.Integration {
	return &notionmcp{baseURL: defaultBaseURL}
}

func (n *notionmcp) Name() string { return "notion-mcp" }

func (n *notionmcp) PlainTextKeys() []string { return []string{"base_url"} }

func (n *notionmcp) OptionalKeys() []string { return []string{"base_url"} }

func MCPServerURL(integration mcp.Integration) string {
	n, ok := integration.(*notionmcp)
	if !ok || n == nil {
		return ""
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.baseURL
}

func (n *notionmcp) Configure(ctx context.Context, creds mcp.Credentials) error {
	token := strings.TrimSpace(creds["mcp_access_token"])
	if token == "" {
		return fmt.Errorf("notion-mcp: mcp_access_token is required")
	}
	baseURL, err := normalizeBaseURL(creds["base_url"])
	if err != nil {
		return err
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	credentials := mcp.Credentials{"access_token": token}
	if n.remote != nil && n.baseURL == baseURL {
		return n.remote.Configure(ctx, credentials)
	}
	remote := remotemcp.New("notion-mcp", baseURL)
	if err := remote.Configure(ctx, credentials); err != nil {
		return fmt.Errorf("notion-mcp: configure remote: %w", errors.Join(err, closeRemote(remote)))
	}
	previous := n.remote
	n.remote = remote
	n.baseURL = baseURL
	return closeRemote(previous)
}

func normalizeBaseURL(baseURL string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return defaultBaseURL, nil
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("notion-mcp: invalid base_url: must be an HTTP(S) URL with a host")
	}
	parsed.Path = strings.TrimSuffix(strings.TrimRight(parsed.Path, "/"), "/mcp")
	parsed.RawPath = strings.TrimSuffix(strings.TrimRight(parsed.RawPath, "/"), "/mcp")
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.RawFragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (n *notionmcp) Healthy(ctx context.Context) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.remote != nil && n.remote.Healthy(ctx)
}

func (n *notionmcp) Tools() []mcp.ToolDefinition {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.remote == nil {
		return nil
	}
	tools := slices.Clone(n.remote.Tools())
	for i := range tools {
		tool := &tools[i]
		tool.Parameters = maps.Clone(tool.Parameters)
		tool.Required = slices.Clone(tool.Required)
		if tool.Name == "notion-mcp_notion-search" {
			tool.Description += " Start here to search Notion pages and discover content before fetching or editing it."
		}
	}
	return tools
}

func (n *notionmcp) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.remote == nil {
		return mcp.ErrResult(mcp.ErrNotConfigured)
	}
	if len(toolName) > len(toolPrefix) && toolName[:len(toolPrefix)] == toolPrefix {
		for _, tool := range n.remote.Tools() {
			if tool.Name == toolName {
				return n.remote.Execute(ctx, toolName, args)
			}
		}
	}
	return mcp.ErrResult(fmt.Errorf("unknown tool: %s", toolName))
}

func (n *notionmcp) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	remote := n.remote
	n.remote = nil
	return closeRemote(remote)
}

func closeRemote(remote mcp.Integration) error {
	if closer, ok := remote.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
