package likec4excalidraw

import (
	"context"
	_ "embed"
	"fmt"
	"net/url"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/daltoniam/switchboard/remotemcp"
)

const integrationName = "likec4excalidraw"

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay(integrationName, compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

var (
	_ mcp.Integration                = (*integration)(nil)
	_ mcp.FieldCompactionIntegration = (*integration)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*integration)(nil)
	_ mcp.PlainTextCredentials       = (*integration)(nil)
	_ mcp.OptionalCredentials        = (*integration)(nil)
	_ mcp.PlaceholderHints           = (*integration)(nil)
)

type integration struct {
	remote mcp.Integration
}

func New() mcp.Integration {
	return &integration{}
}

func (i *integration) Name() string { return integrationName }

func (i *integration) PlainTextKeys() []string { return []string{"base_url"} }

func (i *integration) OptionalKeys() []string { return []string{"mcp_token"} }

func (i *integration) Placeholders() map[string]string {
	return map[string]string{"base_url": "http://127.0.0.1:4242"}
}

func (i *integration) Configure(ctx context.Context, credentials mcp.Credentials) error {
	baseURL := strings.TrimSpace(credentials["base_url"])
	if baseURL == "" {
		return fmt.Errorf("likec4excalidraw: base_url is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("likec4excalidraw: invalid base_url")
	}
	parsed.Path = strings.TrimSuffix(strings.TrimRight(parsed.Path, "/"), "/mcp")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	remote := remotemcp.NewOptionalToken(integrationName, strings.TrimRight(parsed.String(), "/"))
	if err := remote.Configure(ctx, mcp.Credentials{"access_token": strings.TrimSpace(credentials["mcp_token"])}); err != nil {
		return err
	}
	closeRemote(i.remote)
	i.remote = remote
	return nil
}

func (i *integration) Healthy(ctx context.Context) bool {
	return i.remote != nil && i.remote.Healthy(ctx)
}

func (i *integration) Tools() []mcp.ToolDefinition {
	return tools
}

func (i *integration) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	if _, ok := supportedTools[toolName]; !ok {
		return mcp.ErrResult(fmt.Errorf("unknown tool: %s", toolName))
	}
	if i.remote == nil {
		return mcp.ErrResult(fmt.Errorf("likec4excalidraw: integration is not configured"))
	}
	return i.remote.Execute(ctx, toolName, args)
}

func (i *integration) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (i *integration) MaxBytes(toolName mcp.ToolName) (int, bool) {
	maxBytes, ok := maxBytesByTool[toolName]
	return maxBytes, ok
}

func (i *integration) Close() error {
	closeRemote(i.remote)
	i.remote = nil
	return nil
}

func closeRemote(remote mcp.Integration) {
	if closer, ok := remote.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
}
