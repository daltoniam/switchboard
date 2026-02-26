package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

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
with their parameters and descriptions.

Examples:
- Search all tools: {"query": ""}
- Search by integration: {"query": "github"}
- Search by action: {"query": "list issues"}
- Search specific tool: {"query": "datadog_search_logs"}`,
		InputSchema: objectSchema(map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query to filter tools. Leave empty to list all available tools. Matches against tool names, descriptions, and integration names.",
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
	for _, name := range s.services.Config.EnabledIntegrations() {
		integration, ok := s.services.Registry.Get(name)
		if !ok {
			log.Printf("WARN: integration %q enabled but no adapter found", name)
			continue
		}

		ic, _ := s.services.Config.GetIntegration(name)
		if err := integration.Configure(ic.Credentials); err != nil {
			log.Printf("WARN: failed to configure %q: %v", name, err)
			continue
		}

		log.Printf("Configured integration %q with %d tools", name, len(integration.Tools()))
	}
}

func (s *Server) handleSearch(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	var args struct {
		Query string `json:"query"`
	}
	if req.Params.Arguments != nil {
		_ = json.Unmarshal(req.Params.Arguments, &args)
	}

	query := strings.ToLower(args.Query)

	type toolInfo struct {
		Integration string            `json:"integration"`
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Parameters  map[string]string `json:"parameters"`
		Required    []string          `json:"required,omitempty"`
	}

	var results []toolInfo

	for _, name := range s.services.Config.EnabledIntegrations() {
		integration, ok := s.services.Registry.Get(name)
		if !ok {
			continue
		}

		for _, tool := range integration.Tools() {
			if query == "" || matches(tool, name, query) {
				results = append(results, toolInfo{
					Integration: name,
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
					Required:    tool.Required,
				})
			}
		}
	}

	data, _ := json.MarshalIndent(results, "", "  ")

	summary := fmt.Sprintf("Found %d tools", len(results))
	if query != "" {
		summary += fmt.Sprintf(" matching %q", args.Query)
	}

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: summary + "\n\n" + string(data)},
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
