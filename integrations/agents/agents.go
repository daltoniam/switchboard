// Package agents provides a native Go integration for the ARP (Agent Registry
// & Proxy) daemon. It communicates over two transports:
//   - gRPC via HTTP/2 cleartext (h2c) for agent lifecycle management
//   - HTTP/1.1 JSON for the A2A proxy endpoints
//
// The gRPC and HTTP endpoints may run on different ports (configured via
// base_url and a2a_url credentials). This resolves the empty-response issue
// where the WASM plugin used a single port for both transports.
package agents

import (
	"context"
	"fmt"
	"strings"
	"sync"

	mcp "github.com/daltoniam/switchboard"
)

type integration struct {
	mu      sync.RWMutex
	grpc    *grpcClient
	http    *httpClient
	baseURL string
	a2aURL  string
	token   string
}

var _ mcp.PlainTextCredentials = (*integration)(nil)
var _ mcp.PlaceholderHints = (*integration)(nil)
var _ mcp.OptionalCredentials = (*integration)(nil)

// New creates a new agents integration.
func New() mcp.Integration {
	return &integration{}
}

func (a *integration) Name() string { return "agents" }

func (a *integration) PlainTextKeys() []string { return []string{"base_url", "a2a_url"} }
func (a *integration) OptionalKeys() []string  { return []string{"token", "a2a_url"} }
func (a *integration) Placeholders() map[string]string {
	return map[string]string{
		"base_url": "http://localhost:9098",
		"a2a_url":  "http://localhost:9099 (defaults to base_url with port+1)",
		"token":    "arp-bearer-token (optional for localhost)",
	}
}

func (a *integration) Configure(_ context.Context, creds mcp.Credentials) error {
	baseURL := strings.TrimRight(creds["base_url"], "/")
	if baseURL == "" {
		return fmt.Errorf("agents: base_url is required")
	}

	a2aURL := strings.TrimRight(creds["a2a_url"], "/")
	if a2aURL == "" {
		a2aURL = deriveA2AURL(baseURL)
	}

	token := creds["token"]

	a.mu.Lock()
	defer a.mu.Unlock()

	a.baseURL = baseURL
	a.a2aURL = a2aURL
	a.token = token
	a.grpc = newGRPCClient(baseURL, token)
	a.http = newHTTPClient(a2aURL, token)

	return nil
}

func (a *integration) Healthy(ctx context.Context) bool {
	a.mu.RLock()
	g := a.grpc
	a.mu.RUnlock()

	if g == nil {
		return false
	}

	// Try listing projects as a health check.
	_, err := g.call(ctx, "arp.v1.ProjectService", "ListProjects", map[string]any{})
	return err == nil
}

func (a *integration) Tools() []mcp.ToolDefinition {
	return tools
}

func (a *integration) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}

	a.mu.RLock()
	grpc := a.grpc
	http := a.http
	a.mu.RUnlock()

	if grpc == nil || http == nil {
		return &mcp.ToolResult{Data: "agents: not configured", IsError: true}, nil
	}

	return fn(ctx, grpc, http, args)
}

type handlerFunc func(ctx context.Context, grpc *grpcClient, http *httpClient, args map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	// ProjectService
	"agents_project_list":       handleProjectList,
	"agents_project_register":   handleProjectRegister,
	"agents_project_unregister": handleProjectUnregister,
	// WorkspaceService
	"agents_workspace_create":  handleWorkspaceCreate,
	"agents_workspace_list":    handleWorkspaceList,
	"agents_workspace_get":     handleWorkspaceGet,
	"agents_workspace_destroy": handleWorkspaceDestroy,
	// AgentService — lifecycle
	"agents_agent_spawn":   handleAgentSpawn,
	"agents_agent_list":    handleAgentList,
	"agents_agent_status":  handleAgentStatus,
	"agents_agent_stop":    handleAgentStop,
	"agents_agent_restart": handleAgentRestart,
	// AgentService — messaging
	"agents_agent_message":     handleAgentMessage,
	"agents_agent_task":        handleAgentTask,
	"agents_agent_task_status": handleAgentTaskStatus,
	// DiscoveryService
	"agents_discover": handleDiscover,
	// A2A proxy (HTTP)
	"agents_proxy_list":         handleProxyList,
	"agents_agent_card":         handleAgentCard,
	"agents_proxy_send_message": handleProxySendMessage,
	"agents_proxy_get_task":     handleProxyGetTask,
	"agents_proxy_cancel_task":  handleProxyCancelTask,
	"agents_route_message":      handleRouteMessage,
}

// deriveA2AURL computes the A2A HTTP URL from the gRPC base URL by
// incrementing the port number by 1. For example, http://localhost:9098
// becomes http://localhost:9099.
func deriveA2AURL(baseURL string) string {
	// Strip scheme prefix to find the host:port portion.
	scheme := ""
	rest := baseURL
	if idx := strings.Index(baseURL, "://"); idx != -1 {
		scheme = baseURL[:idx+3]
		rest = baseURL[idx+3:]
	}

	// Find the port after the last colon in the host portion.
	// Stop at any path separator.
	hostPort := rest
	var pathSuffix string
	if idx := strings.Index(rest, "/"); idx != -1 {
		hostPort = rest[:idx]
		pathSuffix = rest[idx:]
	}

	lastColon := strings.LastIndex(hostPort, ":")
	if lastColon == -1 {
		// No port specified, return original.
		return baseURL
	}

	host := hostPort[:lastColon]
	portStr := hostPort[lastColon+1:]

	port := 0
	for _, c := range portStr {
		if c >= '0' && c <= '9' {
			port = port*10 + int(c-'0')
		} else {
			// Not a valid port, return original.
			return baseURL
		}
	}

	if portStr == "" {
		return baseURL
	}

	return fmt.Sprintf("%s%s:%d%s", scheme, host, port+1, pathSuffix)
}
