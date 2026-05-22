// Package intercom proxies Intercom's official remote MCP server
// (https://mcp.intercom.com) as a Switchboard integration. All tool
// discovery and execution is delegated to remotemcp; this wrapper exists
// so the integration has a stable Name(), a single registration point in
// cmd/server/main.go, and a seam for future native handlers if Intercom's
// MCP coverage proves insufficient.
package intercom

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/remotemcp"
)

// defaultMCPServerURL is Intercom's hosted MCP endpoint. remotemcp
// appends "/mcp" automatically when connecting.
const defaultMCPServerURL = "https://mcp.intercom.com"

var _ mcp.Integration = (*intercom)(nil)

type intercom struct {
	remote mcp.Integration
}

// New creates an Intercom integration backed by Intercom's official MCP
// server. An optional URL override is accepted to support tests and
// custom endpoints; production callers should pass nothing.
func New(mcpServerURL ...string) mcp.Integration {
	url := defaultMCPServerURL
	if len(mcpServerURL) > 0 && mcpServerURL[0] != "" {
		url = mcpServerURL[0]
	}
	return &intercom{remote: remotemcp.New("intercom", url)}
}

func (i *intercom) Name() string { return "intercom" }

func (i *intercom) Configure(ctx context.Context, creds mcp.Credentials) error {
	return i.remote.Configure(ctx, creds)
}

func (i *intercom) Healthy(ctx context.Context) bool {
	return i.remote.Healthy(ctx)
}

func (i *intercom) Tools() []mcp.ToolDefinition {
	return i.remote.Tools()
}

func (i *intercom) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	return i.remote.Execute(ctx, toolName, args)
}
