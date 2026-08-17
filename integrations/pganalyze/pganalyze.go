package pganalyze

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

const defaultMCPEndpoint = "https://app.pganalyze.com/mcp"

var (
	_ mcp.Integration          = (*pganalyze)(nil)
	_ mcp.PlainTextCredentials = (*pganalyze)(nil)
)

type pganalyze struct {
	apiKey  string
	mcpURL  string
	client  *http.Client
	proxy   *proxyClient
	started bool
}

func New() mcp.Integration {
	return &pganalyze{client: &http.Client{}}
}

func (p *pganalyze) Name() string { return "pganalyze" }

func (p *pganalyze) Configure(_ context.Context, creds mcp.Credentials) error {
	p.apiKey = creds["api_key"]
	if p.apiKey == "" {
		return fmt.Errorf("pganalyze: api_key is required")
	}
	p.mcpURL = defaultMCPEndpoint
	if v := creds["base_url"]; v != "" {
		v = strings.TrimRight(v, "/")
		if strings.HasSuffix(v, "/mcp") {
			p.mcpURL = v
		} else {
			p.mcpURL = v + "/mcp"
		}
	}
	return nil
}

func (p *pganalyze) Healthy(ctx context.Context) bool {
	if p.apiKey == "" {
		return false
	}
	if !p.started {
		p.startProxy(ctx)
	}
	return p.proxy != nil && len(p.proxy.tools) > 0
}

func (p *pganalyze) Tools() []mcp.ToolDefinition {
	if p.proxy != nil {
		return p.proxy.toolDefinitions()
	}
	return staticTools
}

func (p *pganalyze) PlainTextKeys() []string {
	return []string{"base_url"}
}

func (p *pganalyze) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	if !p.started {
		p.startProxy(ctx)
	}
	if p.proxy == nil {
		return mcp.ErrResult(fmt.Errorf("pganalyze MCP proxy not initialized"))
	}
	return p.proxy.execute(ctx, toolName, args)
}

func (p *pganalyze) startProxy(ctx context.Context) {
	p.started = true
	proxy := newProxyClient(p.mcpURL, p.apiKey, p.client)
	if err := proxy.initialize(ctx); err != nil {
		return
	}
	if err := proxy.fetchTools(ctx); err != nil {
		return
	}
	p.proxy = proxy
}
