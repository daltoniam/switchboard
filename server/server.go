package server

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultSearchLimit = 20

// Server wraps the MCP SDK server and exposes search/execute tools.
type Server struct {
	mcpServer *mcpsdk.Server
	services  *mcp.Services
}

// New creates a Server that exposes two MCP tools — search and execute —
// following the Cloudflare "code mode" pattern for progressive discovery
// and efficient tool execution.
func New(services *mcp.Services) *Server {
	mcpServer := mcpsdk.NewServer(
		&mcpsdk.Implementation{
			Name:    "switchboard",
			Version: "0.2.0",
		},
		nil,
	)

	s := &Server{
		mcpServer: mcpServer,
		services:  services,
	}

	s.registerTools()
	return s
}

func (s *Server) registerTools() {
	s.configureIntegrations()

	searchTool := &mcpsdk.Tool{
		Name: "search",
		Description: `Search available tools across all configured integrations (GitHub, Datadog, Linear, Sentry, etc.).
Use this to discover what operations are available before calling execute.

You can filter by integration name, tool name, or keyword. Returns tool definitions
with their parameters and descriptions. Results are paginated (default limit: 20).

Examples:
- Search by integration: {"query": "github"}
- Search by action: {"query": "list issues"}
- Search specific tool: {"query": "datadog_search_logs"}
- Page through results: {"query": "github", "offset": 20, "limit": 20}
- Count all tools: {"limit": 0}`,
		InputSchema: objectSchema(map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query to filter tools. Leave empty to list all available tools. Matches against tool names, descriptions, and integration names.",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of tools to return. Defaults to 20. Set to 0 to return only the total count.",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "Number of results to skip for pagination. Defaults to 0.",
			},
		}, nil),
	}

	executeTool := &mcpsdk.Tool{
		Name: "execute",
		Description: `Execute a tool by name with the given arguments.
Use the search tool first to discover available tools and their parameters.

The tool_name must exactly match a tool returned by search. Arguments are
passed as a JSON object matching the tool's parameter schema.

Examples:
- {"tool_name": "github_list_issues", "arguments": {"owner": "golang", "repo": "go", "state": "open"}}
- {"tool_name": "datadog_search_logs", "arguments": {"query": "service:nginx status:error", "from": "now-1h"}}
- {"tool_name": "linear_get_issue", "arguments": {"id": "ENG-123"}}
- {"tool_name": "sentry_list_projects", "arguments": {}}`,
		InputSchema: objectSchema(map[string]any{
			"tool_name": map[string]any{
				"type":        "string",
				"description": "The exact name of the tool to execute (e.g., 'github_search_repos', 'datadog_search_logs')",
			},
			"arguments": map[string]any{
				"type":        "object",
				"description": "Arguments to pass to the tool, matching the parameter schema returned by search",
			},
		}, []string{"tool_name"}),
	}

	s.mcpServer.AddTool(searchTool, s.handleSearch)
	s.mcpServer.AddTool(executeTool, s.handleExecute)
}

func (s *Server) configureIntegrations() {
	for _, integration := range s.services.Registry.All() {
		name := integration.Name()
		ic, exists := s.services.Config.GetIntegration(name)
		if !exists {
			continue
		}

		// Respect explicit disable from config toggle.
		if exists && !ic.Enabled && !hasCredentials(ic.Credentials) {
			continue
		}

		if err := integration.Configure(ic.Credentials); err != nil {
			log.Printf("WARN: failed to configure %q: %v", name, err)
			continue
		}

		// Auto-enable in config if Configure succeeded.
		if !ic.Enabled {
			ic.Enabled = true
			_ = s.services.Config.SetIntegration(name, ic)
		}

		log.Printf("Configured integration %q with %d tools", name, len(integration.Tools()))
	}
}

func hasCredentials(creds mcp.Credentials) bool {
	for k, v := range creds {
		if v == "" {
			continue
		}
		switch k {
		case "client_id", "client_secret", "token_source":
			continue
		default:
			return true
		}
	}
	return false
}

func (s *Server) handleSearch(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	var args struct {
		Query  string `json:"query"`
		Limit  *int   `json:"limit"`
		Offset int    `json:"offset"`
	}
	if req.Params.Arguments != nil {
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments: " + err.Error()), nil
		}
	}

	limit := defaultSearchLimit
	if args.Limit != nil {
		limit = max(*args.Limit, 0)
	}
	if args.Offset < 0 {
		args.Offset = 0
	}

	query := strings.ToLower(args.Query)

	type toolInfo struct {
		Integration string            `json:"integration"`
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Parameters  map[string]string `json:"parameters"`
		Required    []string          `json:"required,omitempty"`
	}

	enabled := s.services.Config.EnabledIntegrations()
	var all []toolInfo

	for _, name := range enabled {
		integration, ok := s.services.Registry.Get(name)
		if !ok {
			continue
		}

		for _, tool := range integration.Tools() {
			if query == "" || matches(tool, name, query) {
				all = append(all, toolInfo{
					Integration: name,
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
					Required:    tool.Required,
				})
			}
		}
	}

	slices.SortFunc(all, func(a, b toolInfo) int {
		if c := cmp.Compare(a.Integration, b.Integration); c != 0 {
			return c
		}
		return cmp.Compare(a.Name, b.Name)
	})

	total := len(all)

	// Clamp offset.
	offset := args.Offset
	if offset > total {
		offset = total
	}

	// Slice the window. limit=0 intentionally yields an empty page (count-only mode).
	end := offset + limit
	if end > total {
		end = total
	}
	page := all[offset:end]

	type response struct {
		Summary      string     `json:"summary"`
		Total        int        `json:"total"`
		Offset       int        `json:"offset"`
		Limit        int        `json:"limit"`
		HasMore      bool       `json:"has_more"`
		Integrations []string   `json:"integrations"`
		Tools        []toolInfo `json:"tools"`
	}

	summary := fmt.Sprintf("Found %d tools", total)
	if query != "" {
		summary += fmt.Sprintf(" matching %q", args.Query)
	}

	data, _ := json.Marshal(response{
		Summary:      summary,
		Total:        total,
		Offset:       offset,
		Limit:        limit,
		HasMore:      limit > 0 && offset+limit < total,
		Integrations: enabled,
		Tools:        page,
	})

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: string(data)},
		},
	}, nil
}

func (s *Server) handleExecute(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	var args struct {
		ToolName  string         `json:"tool_name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error()), nil
	}

	if args.ToolName == "" {
		return errorResult("tool_name is required"), nil
	}
	if args.Arguments == nil {
		args.Arguments = map[string]any{}
	}

	for _, name := range s.services.Config.EnabledIntegrations() {
		integration, ok := s.services.Registry.Get(name)
		if !ok {
			continue
		}

		for _, tool := range integration.Tools() {
			if tool.Name == args.ToolName {
				result, err := integration.Execute(ctx, args.ToolName, args.Arguments)
				if err != nil {
					return errorResult(err.Error()), nil
				}
				return &mcpsdk.CallToolResult{
					Content: []mcpsdk.Content{
						&mcpsdk.TextContent{Text: result.Data},
					},
					IsError: result.IsError,
				}, nil
			}
		}
	}

	return errorResult(fmt.Sprintf("tool %q not found. Use the search tool to discover available tools.", args.ToolName)), nil
}

// Handler returns an http.Handler that serves MCP over streamable HTTP transport.
func (s *Server) Handler() http.Handler {
	return mcpsdk.NewStreamableHTTPHandler(
		func(r *http.Request) *mcpsdk.Server {
			return s.mcpServer
		},
		&mcpsdk.StreamableHTTPOptions{
			Logger: slog.Default(),
		},
	)
}

// RunStdio starts the MCP server over stdio transport.
func (s *Server) RunStdio(ctx context.Context) error {
	return s.mcpServer.Run(ctx, &mcpsdk.StdioTransport{})
}

func matches(tool mcp.ToolDefinition, integrationName, query string) bool {
	searchable := strings.ToLower(tool.Name + " " + tool.Description + " " + integrationName)
	for _, word := range strings.Fields(query) {
		if !strings.Contains(searchable, word) {
			return false
		}
	}
	return true
}

func errorResult(text string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: text},
		},
		IsError: true,
	}
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	s := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}
