package figma

import (
	"context"
	"fmt"
	"maps"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/remotemcp"
)

const defaultBaseURL = "https://mcp.figma.com"

var toolDescriptions = map[mcp.ToolName]string{
	"figma_get_figjam":       "Read a FigJam whiteboard as structured XML with node IDs, positions, sizes, and screenshots. Start here to inspect an existing FigJam board before editing it or implementing reviewed decisions.",
	"figma_use_figma":        "Create, inspect, edit, or delete native editable FigJam content including sections, sticky notes, connectors, shapes, text, tables, code blocks, labels, and annotations. Use after figma_get_figjam when modifying an existing board.",
	"figma_generate_diagram": "Generate an editable FigJam flowchart, Gantt chart, state diagram, sequence diagram, architecture diagram, or ERD from Mermaid syntax or a natural-language description.",
	"figma_create_new_file":  "Create a blank FigJam whiteboard in Figma Drafts. Use before figma_use_figma when a workflow needs a new planning board.",
	"figma_upload_assets":    "Upload PNG, JPG, GIF, or WebP images into a FigJam whiteboard, either as new layers or as fills on existing nodes.",
	"figma_get_screenshot":   "Capture a screenshot of a FigJam node for visual inspection. Use figma_get_figjam first to discover a valid node ID.",
	"figma_whoami":           "Get the authenticated Figma user, plans, and seat types to verify account access before creating or editing a FigJam whiteboard.",
}

var (
	_ mcp.Integration          = (*figma)(nil)
	_ mcp.PlainTextCredentials = (*figma)(nil)
	_ mcp.OptionalCredentials  = (*figma)(nil)
)

type figma struct {
	baseURL   string
	remote    mcp.Integration
	newRemote func(string) mcp.Integration
}

func New() mcp.Integration {
	return &figma{
		baseURL: defaultBaseURL,
		newRemote: func(baseURL string) mcp.Integration {
			return remotemcp.New("figma", baseURL)
		},
	}
}

func (f *figma) Name() string { return "figma" }

func (f *figma) PlainTextKeys() []string { return []string{"base_url"} }

func (f *figma) OptionalKeys() []string { return []string{"base_url"} }

func MCPServerURL(integration mcp.Integration) string {
	if configured, ok := integration.(*figma); ok {
		return configured.baseURL
	}
	return ""
}

func (f *figma) Configure(ctx context.Context, creds mcp.Credentials) error {
	token := strings.TrimSpace(creds["mcp_access_token"])
	if token == "" {
		return fmt.Errorf("figma: mcp_access_token is required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(creds["base_url"]), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if f.remote == nil || f.baseURL != baseURL {
		factory := f.newRemote
		if factory == nil {
			factory = func(serverURL string) mcp.Integration {
				return remotemcp.New("figma", serverURL)
			}
		}
		f.remote = factory(baseURL)
	}
	f.baseURL = baseURL
	return f.remote.Configure(ctx, mcp.Credentials{"access_token": token})
}

func (f *figma) Healthy(ctx context.Context) bool {
	return f.remote != nil && f.remote.Healthy(ctx)
}

func (f *figma) Tools() []mcp.ToolDefinition {
	if f.remote == nil {
		return nil
	}
	upstream := f.remote.Tools()
	tools := make([]mcp.ToolDefinition, 0, len(toolDescriptions))
	for _, tool := range upstream {
		description, ok := toolDescriptions[tool.Name]
		if !ok {
			continue
		}
		tool.Description = description
		tools = append(tools, tool)
	}
	return tools
}

func (f *figma) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	if _, ok := toolDescriptions[toolName]; !ok {
		return mcp.ErrResult(fmt.Errorf("unknown tool: %s", toolName))
	}
	if f.remote == nil {
		return mcp.ErrResult(mcp.ErrNotConfigured)
	}
	forwarded := make(map[string]any, len(args)+1)
	maps.Copy(forwarded, args)
	if toolName == "figma_use_figma" {
		if _, ok := forwarded["skillNames"]; !ok {
			forwarded["skillNames"] = []any{"figma-use-figjam"}
		}
	}
	return f.remote.Execute(ctx, toolName, forwarded)
}
