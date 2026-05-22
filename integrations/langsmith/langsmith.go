// Package langsmith proxies LangSmith's hosted Remote MCP server
// (https://api.smith.langchain.com/mcp, OAuth 2.1 + Dynamic Client
// Registration) as a Switchboard integration. Tool discovery and execution
// are delegated to remotemcp; this wrapper exists so the integration has
// a stable Name(), participates in the OAuth setup flow from the web UI
// (via MCPServerURL), and persists refreshed tokens back to the config
// service so headless production deployments don't need to re-authorize
// when access tokens expire.
package langsmith

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/remotemcp"
)

// defaultMCPServerURL is the LangSmith Cloud GCP US region. remotemcp
// appends "/mcp" automatically when connecting. Override via the optional
// constructor argument to point at eu., apac., or aws. regions.
const defaultMCPServerURL = "https://api.smith.langchain.com"

// Regional endpoints documented at
// https://docs.langchain.com/langsmith/langsmith-remote-mcp.
const (
	RegionUS   = "https://api.smith.langchain.com"
	RegionEU   = "https://eu.api.smith.langchain.com"
	RegionAPAC = "https://apac.api.smith.langchain.com"
	RegionAWS  = "https://aws.api.smith.langchain.com"
)

var _ mcp.Integration = (*langsmith)(nil)

type langsmith struct {
	cfg          mcp.ConfigService
	mcpServerURL string
	remote       mcp.Integration
}

// New creates a LangSmith integration. cfg is required so refreshed access
// tokens get persisted back to the config store — without it, the in-memory
// refresh works but a process restart loses the rotated token (if upstream
// rotates) and falls back to the original refresh token. Pass an optional
// mcpServerURL to override the default GCP US endpoint (tests, EU/APAC/AWS
// regions, or self-hosted LangSmith).
func New(cfg mcp.ConfigService, mcpServerURL ...string) mcp.Integration {
	url := defaultMCPServerURL
	if len(mcpServerURL) > 0 && mcpServerURL[0] != "" {
		url = mcpServerURL[0]
	}
	l := &langsmith{cfg: cfg, mcpServerURL: url}
	l.remote = remotemcp.New("langsmith", url, remotemcp.WithTokenSink(l.persistTokens))
	return l
}

// MCPServerURL returns the configured remote MCP server URL, or empty. Used
// by the web layer to discover that this integration speaks remote MCP and
// to drive the OAuth start endpoint. Mirrors linear.MCPServerURL.
func MCPServerURL(i mcp.Integration) string {
	if l, ok := i.(*langsmith); ok {
		return l.mcpServerURL
	}
	return ""
}

func (l *langsmith) Name() string { return "langsmith" }

// Configure accepts the credentials persisted by the web OAuth callback:
// mcp_access_token (required), mcp_refresh_token + client_id + client_secret
// (optional, but required to recover from access-token expiry without a
// fresh browser flow). The mcp_ prefix matches the convention established
// by the Linear integration; remotemcp internally uses the unprefixed names.
func (l *langsmith) Configure(ctx context.Context, creds mcp.Credentials) error {
	delegate := mcp.Credentials{"access_token": creds["mcp_access_token"]}
	if v := creds["mcp_refresh_token"]; v != "" {
		delegate["refresh_token"] = v
	}
	if v := creds[mcp.CredKeyClientID]; v != "" {
		delegate["client_id"] = v
	}
	if v := creds[mcp.CredKeyClientSecret]; v != "" {
		delegate["client_secret"] = v
	}
	return l.remote.Configure(ctx, delegate)
}

func (l *langsmith) Healthy(ctx context.Context) bool {
	return l.remote.Healthy(ctx)
}

func (l *langsmith) Tools() []mcp.ToolDefinition {
	return l.remote.Tools()
}

func (l *langsmith) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	return l.remote.Execute(ctx, toolName, args)
}

// persistTokens is the TokenSink callback wired into remotemcp. When the
// transport refreshes an access token (and possibly receives a rotated
// refresh token), this writes the new values back to the config store
// under the mcp_-prefixed keys that the web UI and Overmind expect.
func (l *langsmith) persistTokens(refreshed mcp.Credentials) {
	if l.cfg == nil {
		return
	}
	ic, _ := l.cfg.GetIntegration(l.Name())
	if ic == nil {
		return
	}
	if v := refreshed["access_token"]; v != "" {
		ic.Credentials["mcp_access_token"] = v
	}
	if v := refreshed["refresh_token"]; v != "" {
		ic.Credentials["mcp_refresh_token"] = v
	}
	_ = l.cfg.SetIntegration(l.Name(), ic)
}
