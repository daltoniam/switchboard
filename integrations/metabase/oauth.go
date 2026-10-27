package metabase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/remotemcp"
)

// MCPEndpointPath is where Metabase serves its hosted MCP server, relative to the site URL.
const MCPEndpointPath = "/api/metabase-mcp"

const searchTool = mcp.ToolName("metabase_search")

// SetConfigService lets the adapter persist rotated OAuth tokens.
func SetConfigService(i mcp.Integration, svc mcp.ConfigService) {
	if m, ok := i.(*metabase); ok {
		m.cfgMu.Lock()
		m.configSvc = svc
		m.cfgMu.Unlock()
	}
}

// MCPServerURL returns the configured Metabase site URL, which is also the OAuth issuer.
func MCPServerURL(i mcp.Integration) string {
	m, ok := i.(*metabase)
	if !ok {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.baseURL
}

// IsRemoteMCP reports whether the adapter currently proxies Metabase's hosted MCP server.
func IsRemoteMCP(i mcp.Integration) bool {
	m, ok := i.(*metabase)
	if !ok {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.useRemote && m.remote != nil
}

// MCPServerEnabled reads Metabase's public "MCP server" admin setting (Admin > AI > MCP).
func MCPServerEnabled(ctx context.Context, client *http.Client, baseURL string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/session/properties", nil)
	if err != nil {
		return false, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("metabase: read session properties: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("metabase: session properties returned %d", resp.StatusCode)
	}
	var props struct {
		MCPEnabled *bool `json:"mcp-enabled?"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&props); err != nil {
		return false, fmt.Errorf("metabase: parse session properties: %w", err)
	}
	if props.MCPEnabled == nil {
		return false, errors.New("metabase: this instance does not report an MCP server setting; it may predate the hosted MCP server")
	}
	return *props.MCPEnabled, nil
}

func (m *metabase) remoteFor(baseURL string) (mcp.Integration, error) {
	if m.remote != nil && remotemcp.ServerURL(m.remote) == baseURL {
		return m.remote, nil
	}
	previous := m.remote
	m.remote = remotemcp.NewWithOptions("metabase", baseURL, remotemcp.Options{
		EndpointPath:   MCPEndpointPath,
		OnTokenRefresh: m.persistTokens,
	})
	return m.remote, closeRemote(previous)
}

func (m *metabase) activeRemote() mcp.Integration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.useRemote {
		return nil
	}
	return m.remote
}

func remoteTools(remote mcp.Integration) []mcp.ToolDefinition {
	tools := slices.Clone(remote.Tools())
	for i := range tools {
		tool := &tools[i]
		tool.Parameters = maps.Clone(tool.Parameters)
		tool.Required = slices.Clone(tool.Required)
		if tool.Name == searchTool {
			tool.Description += " Start here to find Metabase questions, dashboards, models, and tables before querying or editing them."
		}
	}
	return tools
}

func (m *metabase) persistTokens(set remotemcp.TokenSet) {
	m.cfgMu.Lock()
	svc := m.configSvc
	m.cfgMu.Unlock()
	if svc == nil {
		return
	}
	ic, ok := svc.GetIntegration("metabase")
	if !ok || ic == nil {
		return
	}
	if ic.Credentials == nil {
		ic.Credentials = mcp.Credentials{}
	}
	ic.Credentials["mcp_access_token"] = set.AccessToken
	ic.Credentials["mcp_refresh_token"] = set.RefreshToken
	if set.ClientID != "" {
		ic.Credentials["mcp_client_id"] = set.ClientID
	}
	_ = svc.SetIntegration("metabase", ic)
}

func (m *metabase) Close() error {
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
