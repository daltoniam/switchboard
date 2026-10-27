package pganalyze

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

const (
	defaultMCPEndpoint = "https://app.pganalyze.com/mcp"
	httpTimeout        = 30 * time.Second
	// initRetryInterval bounds how often a failed handshake is retried, so a
	// network blip at daemon start heals on a later call without hammering
	// pganalyze on every search or execute.
	initRetryInterval = 30 * time.Second
)

var (
	_ mcp.Integration          = (*pganalyze)(nil)
	_ mcp.PlainTextCredentials = (*pganalyze)(nil)
)

type pganalyze struct {
	client *http.Client
	now    func() time.Time

	// initMu serializes handshake attempts; mu guards the fields below and is
	// never held across network I/O, so Tools() stays responsive during init.
	initMu sync.Mutex
	mu     sync.RWMutex
	apiKey string
	mcpURL string
	proxy  *proxyClient
	// initErr and initAt record the last failed handshake for backoff.
	initErr error
	initAt  time.Time
}

func New() mcp.Integration {
	return &pganalyze{client: &http.Client{Timeout: httpTimeout}}
}

func (p *pganalyze) Name() string { return "pganalyze" }

func (p *pganalyze) Configure(_ context.Context, creds mcp.Credentials) error {
	apiKey := creds["api_key"]
	if apiKey == "" {
		return fmt.Errorf("pganalyze: api_key is required")
	}
	mcpURL := defaultMCPEndpoint
	if v := creds["base_url"]; v != "" {
		v = strings.TrimRight(v, "/")
		if strings.HasSuffix(v, "/mcp") {
			mcpURL = v
		} else {
			mcpURL = v + "/mcp"
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if apiKey != p.apiKey || mcpURL != p.mcpURL {
		p.proxy = nil
	}
	p.apiKey = apiKey
	p.mcpURL = mcpURL
	p.initErr = nil
	p.initAt = time.Time{}
	return nil
}

func (p *pganalyze) Healthy(ctx context.Context) bool {
	proxy, err := p.ensureProxy(ctx)
	return err == nil && len(proxy.toolDefinitions()) > 0
}

func (p *pganalyze) Tools() []mcp.ToolDefinition {
	p.mu.RLock()
	proxy := p.proxy
	p.mu.RUnlock()
	if proxy != nil {
		return proxy.toolDefinitions()
	}
	return staticTools
}

func (p *pganalyze) PlainTextKeys() []string {
	return []string{"base_url"}
}

func (p *pganalyze) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	proxy, err := p.ensureProxy(ctx)
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("pganalyze MCP proxy not initialized: %w", err))
	}
	return proxy.execute(ctx, toolName, args)
}

// ensureProxy returns a connected proxy, performing the MCP handshake on first
// use. A failed handshake is retried after initRetryInterval rather than
// latching the integration off until the daemon restarts.
func (p *pganalyze) ensureProxy(ctx context.Context) (*proxyClient, error) {
	if proxy, done, err := p.cachedProxy(); done {
		return proxy, err
	}

	p.initMu.Lock()
	defer p.initMu.Unlock()
	if proxy, done, err := p.cachedProxy(); done {
		return proxy, err
	}

	p.mu.RLock()
	apiKey, mcpURL := p.apiKey, p.mcpURL
	p.mu.RUnlock()

	proxy := newProxyClient(mcpURL, apiKey, p.client)
	err := proxy.initialize(ctx)
	if err == nil {
		err = proxy.fetchTools(ctx)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if apiKey != p.apiKey || mcpURL != p.mcpURL {
		return nil, fmt.Errorf("pganalyze credentials changed during initialization")
	}
	if err != nil {
		p.initErr = err
		p.initAt = p.clock()
		return nil, err
	}
	p.proxy = proxy
	p.initErr = nil
	return proxy, nil
}

// cachedProxy reports whether ensureProxy can answer without a handshake: a
// live proxy, a missing api key, or a recent failure still inside backoff.
func (p *pganalyze) cachedProxy() (*proxyClient, bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	switch {
	case p.proxy != nil:
		return p.proxy, true, nil
	case p.apiKey == "":
		return nil, true, fmt.Errorf("pganalyze: api_key is required")
	case p.initErr != nil && p.clock().Sub(p.initAt) < initRetryInterval:
		return nil, true, p.initErr
	}
	return nil, false, nil
}

func (p *pganalyze) clock() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}
