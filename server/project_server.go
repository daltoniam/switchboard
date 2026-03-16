package server

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/project"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultServerID = "switchboard"

// ProjectRouter serves project-scoped MCP endpoints at /mcp/{project}.
type ProjectRouter struct {
	services *mcp.Services
	store    *project.Store
	serverID string

	mu      sync.RWMutex
	servers map[string]*projectMCPServer
}

type projectMCPServer struct {
	mcpSrv *mcpsdk.Server
	def    *project.Definition
}

// NewProjectRouter creates a router that dispatches /mcp/{project} requests
// to per-project MCP servers with tool scoping and context delivery.
func NewProjectRouter(services *mcp.Services, store *project.Store, serverID string) *ProjectRouter {
	if serverID == "" {
		serverID = defaultServerID
	}
	return &ProjectRouter{
		services: services,
		store:    store,
		serverID: serverID,
		servers:  make(map[string]*projectMCPServer),
	}
}

// Handler returns an http.Handler for /mcp/{project} routes.
func (pr *ProjectRouter) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		projectName := r.PathValue("project")
		if projectName == "" {
			http.Error(w, "missing project name", http.StatusBadRequest)
			return
		}

		srv, err := pr.getOrCreate(projectName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		handler := mcpsdk.NewStreamableHTTPHandler(
			func(_ *http.Request) *mcpsdk.Server {
				return srv.mcpSrv
			},
			&mcpsdk.StreamableHTTPOptions{
				Stateless: true,
				Logger:    slog.Default(),
			},
		)
		handler.ServeHTTP(w, r)
	})
}

func (pr *ProjectRouter) getOrCreate(projectName string) (*projectMCPServer, error) {
	pr.mu.RLock()
	srv, ok := pr.servers[projectName]
	pr.mu.RUnlock()
	if ok {
		return srv, nil
	}

	def, exists := pr.store.Get(projectName)
	if !exists {
		return nil, fmt.Errorf("project %q not found", projectName)
	}

	srv = pr.buildServer(def)

	pr.mu.Lock()
	pr.servers[projectName] = srv
	pr.mu.Unlock()

	return srv, nil
}

func (pr *ProjectRouter) buildServer(def *project.Definition) *projectMCPServer {
	mcpSrv := mcpsdk.NewServer(
		&mcpsdk.Implementation{
			Name:    "switchboard",
			Version: "0.2.0",
		},
		&mcpsdk.ServerOptions{
			Instructions: fmt.Sprintf(
				"Project-scoped MCP server for %q. Use the search tool to discover available operations and project_context to retrieve project context.",
				def.Name,
			),
			Logger: slog.Default(),
		},
	)

	ps := &projectMCPServer{mcpSrv: mcpSrv, def: def}

	scopeRule := project.GetEffectiveRule(def, pr.serverID, "")

	searchTool := &mcpsdk.Tool{
		Name: "search",
		Description: `Search available tools scoped to this project.
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
				"description": "Search query to filter tools.",
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
		Description: `Execute a tool scoped to this project. Default arguments from the project definition are injected automatically.

Mode 1 — Single tool (provide tool_name + arguments):
  {"tool_name": "github_list_issues", "arguments": {"state": "open"}}

Mode 2 — Script (provide script):
  Write JavaScript that calls api.call(toolName, args) to invoke tools.

Use search first to discover available tools and their parameter schemas.`,
		InputSchema: objectSchema(map[string]any{
			"tool_name": map[string]any{
				"type":        "string",
				"description": "The exact name of the tool to execute (mutually exclusive with script)",
			},
			"arguments": map[string]any{
				"type":        "object",
				"description": "Arguments to pass to the tool (mutually exclusive with script)",
			},
			"script": map[string]any{
				"type":        "string",
				"description": "JavaScript code to execute server-side. Use api.call(toolName, args) to invoke tools. Return the final result. (mutually exclusive with tool_name)",
			},
		}, nil),
	}

	mcpSrv.AddTool(searchTool, pr.makeSearchHandler(scopeRule))
	mcpSrv.AddTool(executeTool, pr.makeExecuteHandler(def, scopeRule))

	pr.addContextTool(mcpSrv, def)
	pr.addProjectManagementTools(mcpSrv, def)

	return ps
}

func (pr *ProjectRouter) makeSearchHandler(scopeRule *project.ScopeRule) mcpsdk.ToolHandler {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
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

		enabled := pr.services.Config.EnabledIntegrations()
		var all []toolInfo

		for _, name := range enabled {
			integration, ok := pr.services.Registry.Get(name)
			if !ok {
				continue
			}

			permitted := project.FilterTools(integration.Tools(), scopeRule)
			for _, tool := range permitted {
				if query == "" || matches(tool, name, query) {
					params := make(map[string]string, len(tool.Parameters))
					for k, v := range tool.Parameters {
						params[k] = v
					}
					all = append(all, toolInfo{
						Integration: name,
						Name:        tool.Name,
						Description: tool.Description,
						Parameters:  params,
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
		offset := args.Offset
		if offset > total {
			offset = total
		}
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
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(data)}},
		}, nil
	}
}

func (pr *ProjectRouter) makeExecuteHandler(def *project.Definition, scopeRule *project.ScopeRule) mcpsdk.ToolHandler {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		var args struct {
			ToolName  string         `json:"tool_name"`
			Arguments map[string]any `json:"arguments"`
			Script    string         `json:"script"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments: " + err.Error()), nil
		}

		if args.Script != "" {
			return errorResult("scripts are not supported on project-scoped endpoints"), nil
		}

		if args.ToolName == "" {
			return errorResult("tool_name is required"), nil
		}

		if !project.IsToolPermitted(args.ToolName, scopeRule) {
			return errorResult(fmt.Sprintf("tool %q is denied by project scoping rules", args.ToolName)), nil
		}

		if args.Arguments == nil {
			args.Arguments = map[string]any{}
		}
		args.Arguments = project.ResolveDefaults(args.ToolName, scopeRule, args.Arguments)

		integration, found := pr.findIntegration(args.ToolName)
		if !found {
			return errorResult(fmt.Sprintf("tool %q not found. Use the search tool to discover available tools.", args.ToolName)), nil
		}

		result, err := integration.Execute(ctx, args.ToolName, args.Arguments)
		if err != nil {
			return errorResult(err.Error()), nil
		}

		if !result.IsError {
			result.Data = processResult(integration, args.ToolName, result.Data)

			if len(result.Data) > maxResponseBytes {
				return errorResult(fmt.Sprintf(
					"Response exceeded %dKB (actual: %dKB). Use more specific filters, lower limit/per_page, or fetch individual items.",
					maxResponseBytes/1024, len(result.Data)/1024,
				)), nil
			}
		}

		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: result.Data}},
			IsError: result.IsError,
		}, nil
	}
}

func (pr *ProjectRouter) findIntegration(toolName string) (mcp.Integration, bool) {
	for _, name := range pr.services.Config.EnabledIntegrations() {
		integration, ok := pr.services.Registry.Get(name)
		if !ok {
			continue
		}
		for _, tool := range integration.Tools() {
			if tool.Name == toolName {
				return integration, true
			}
		}
	}
	return nil, false
}

func (pr *ProjectRouter) addContextTool(mcpSrv *mcpsdk.Server, def *project.Definition) {
	contextTool := &mcpsdk.Tool{
		Name:        "project_context",
		Description: "Search and retrieve project context files. Call with no arguments to list available context entries, with a query to search, or with a path to fetch a specific file's content.",
		InputSchema: objectSchema(map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query to filter context entries by path. If omitted, returns a manifest of all available context.",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Exact path of a context file to retrieve its full content.",
			},
			"role": map[string]any{
				"type":        "string",
				"description": "Role name to apply context overrides.",
			},
		}, nil),
	}

	mcpSrv.AddTool(contextTool, pr.makeContextHandler(def))
}

func (pr *ProjectRouter) makeContextHandler(def *project.Definition) mcpsdk.ToolHandler {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		var args struct {
			Query string `json:"query"`
			Path  string `json:"path"`
			Role  string `json:"role"`
		}
		if req.Params.Arguments != nil {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return errorResult("invalid arguments: " + err.Error()), nil
			}
		}

		configDir := pr.store.ConfigDir()

		if args.Path != "" {
			content, err := project.ReadContextFile(def, configDir, args.Path)
			if err != nil {
				return &mcpsdk.CallToolResult{
					Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: err.Error()}},
				}, nil
			}
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: content}},
			}, nil
		}

		entries := project.AssembleManifestWithRole(def, configDir, args.Role)

		if args.Query != "" {
			q := strings.ToLower(args.Query)
			var filtered []project.ContextEntry
			for _, e := range entries {
				if strings.Contains(strings.ToLower(e.Path), q) {
					filtered = append(filtered, e)
				}
			}
			entries = filtered
		}

		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return errorResult("failed to marshal context manifest: " + err.Error()), nil
		}
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(data)}},
		}, nil
	}
}

func (pr *ProjectRouter) addProjectManagementTools(mcpSrv *mcpsdk.Server, boundDef *project.Definition) {
	listTool := &mcpsdk.Tool{
		Name:        "project_list",
		Description: "List all project names and summaries.",
		InputSchema: objectSchema(nil, nil),
	}

	getTool := &mcpsdk.Tool{
		Name:        "project_get",
		Description: "Return the fully merged project definition. Name defaults to the current project.",
		InputSchema: objectSchema(map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Project name. Optional when served at a project-scoped URL.",
			},
		}, nil),
	}

	createTool := &mcpsdk.Tool{
		Name:        "project_create",
		Description: "Create a new project definition in the user-level store.",
		InputSchema: objectSchema(map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Project name.",
			},
			"repo": map[string]any{
				"type":        "string",
				"description": "Path to source repository.",
			},
			"branch": map[string]any{
				"type":        "string",
				"description": "Default branch.",
			},
		}, []string{"name"}),
	}

	updateTool := &mcpsdk.Tool{
		Name:        "project_update",
		Description: "Apply a JSON merge patch to the project definition. Name defaults to the current project.",
		InputSchema: objectSchema(map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Project name. Optional when served at a project-scoped URL.",
			},
			"patch": map[string]any{
				"type":        "object",
				"description": "JSON merge patch (RFC 7396) to apply.",
			},
		}, []string{"patch"}),
	}

	deleteTool := &mcpsdk.Tool{
		Name:        "project_delete",
		Description: "Delete the project definition from the user-level store. Name defaults to the current project.",
		InputSchema: objectSchema(map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Project name. Optional when served at a project-scoped URL.",
			},
		}, nil),
	}

	toolsTool := &mcpsdk.Tool{
		Name:        "project_tools",
		Description: "Return the resolved tool manifest after allow/deny/role filtering.",
		InputSchema: objectSchema(map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Project name. Optional when served at a project-scoped URL.",
			},
			"role": map[string]any{
				"type":        "string",
				"description": "Role name to apply tool overrides.",
			},
		}, nil),
	}

	defaultsTool := &mcpsdk.Tool{
		Name:        "project_defaults",
		Description: "Return the resolved default arguments for a specific tool.",
		InputSchema: objectSchema(map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Project name. Optional when served at a project-scoped URL.",
			},
			"tool_name": map[string]any{
				"type":        "string",
				"description": "Tool name to resolve defaults for.",
			},
		}, []string{"tool_name"}),
	}

	mcpSrv.AddTool(listTool, pr.handleProjectList)
	mcpSrv.AddTool(getTool, pr.makeProjectGetHandler(boundDef))
	mcpSrv.AddTool(createTool, pr.handleProjectCreate)
	mcpSrv.AddTool(updateTool, pr.makeProjectUpdateHandler(boundDef))
	mcpSrv.AddTool(deleteTool, pr.makeProjectDeleteHandler(boundDef))
	mcpSrv.AddTool(toolsTool, pr.makeProjectToolsHandler(boundDef))
	mcpSrv.AddTool(defaultsTool, pr.makeProjectDefaultsHandler(boundDef))
}

func (pr *ProjectRouter) handleProjectList(_ context.Context, _ *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	type summary struct {
		Name   string `json:"name"`
		Repo   string `json:"repo,omitempty"`
		Branch string `json:"branch,omitempty"`
	}
	all := pr.store.All()
	var summaries []summary
	for _, def := range all {
		summaries = append(summaries, summary{
			Name:   def.Name,
			Repo:   def.Repo,
			Branch: def.Branch,
		})
	}
	slices.SortFunc(summaries, func(a, b summary) int {
		return cmp.Compare(a.Name, b.Name)
	})
	data, _ := json.MarshalIndent(summaries, "", "  ")
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(data)}},
	}, nil
}

func (pr *ProjectRouter) makeProjectGetHandler(boundDef *project.Definition) mcpsdk.ToolHandler {
	return func(_ context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		name := pr.resolveProjectName(req, boundDef)
		def, ok := pr.store.Get(name)
		if !ok {
			return errorResult(fmt.Sprintf("project %q not found", name)), nil
		}
		data, _ := json.MarshalIndent(def, "", "  ")
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(data)}},
		}, nil
	}
}

func (pr *ProjectRouter) handleProjectCreate(_ context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	var args struct {
		Name   string `json:"name"`
		Repo   string `json:"repo"`
		Branch string `json:"branch"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error()), nil
	}
	if args.Name == "" {
		return errorResult("name is required"), nil
	}
	def := &project.Definition{
		Version: "1",
		Name:    args.Name,
		Repo:    args.Repo,
		Branch:  args.Branch,
	}
	if err := pr.store.Create(def); err != nil {
		return errorResult(err.Error()), nil
	}
	data, _ := json.MarshalIndent(def, "", "  ")
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(data)}},
	}, nil
}

func (pr *ProjectRouter) makeProjectUpdateHandler(boundDef *project.Definition) mcpsdk.ToolHandler {
	return func(_ context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		var args struct {
			Name  string          `json:"name"`
			Patch json.RawMessage `json:"patch"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments: " + err.Error()), nil
		}
		name := args.Name
		if name == "" && boundDef != nil {
			name = boundDef.Name
		}
		if name == "" {
			return errorResult("name is required"), nil
		}
		updated, err := pr.store.Update(name, args.Patch)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		data, _ := json.MarshalIndent(updated, "", "  ")
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(data)}},
		}, nil
	}
}

func (pr *ProjectRouter) makeProjectDeleteHandler(boundDef *project.Definition) mcpsdk.ToolHandler {
	return func(_ context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		name := pr.resolveProjectName(req, boundDef)
		if err := pr.store.Delete(name); err != nil {
			return errorResult(err.Error()), nil
		}
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("project %q deleted", name)}},
		}, nil
	}
}

func (pr *ProjectRouter) makeProjectToolsHandler(boundDef *project.Definition) mcpsdk.ToolHandler {
	return func(_ context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		var args struct {
			Name string `json:"name"`
			Role string `json:"role"`
		}
		if req.Params.Arguments != nil {
			_ = json.Unmarshal(req.Params.Arguments, &args)
		}
		name := args.Name
		if name == "" && boundDef != nil {
			name = boundDef.Name
		}
		def, ok := pr.store.Get(name)
		if !ok {
			return errorResult(fmt.Sprintf("project %q not found", name)), nil
		}

		rule := project.GetEffectiveRule(def, pr.serverID, args.Role)

		var toolNames []string
		for _, intName := range pr.services.Config.EnabledIntegrations() {
			integration, ok := pr.services.Registry.Get(intName)
			if !ok {
				continue
			}
			for _, tool := range project.FilterTools(integration.Tools(), rule) {
				toolNames = append(toolNames, tool.Name)
			}
		}

		slices.Sort(toolNames)
		data, _ := json.MarshalIndent(toolNames, "", "  ")
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(data)}},
		}, nil
	}
}

func (pr *ProjectRouter) makeProjectDefaultsHandler(boundDef *project.Definition) mcpsdk.ToolHandler {
	return func(_ context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		var args struct {
			Name     string `json:"name"`
			ToolName string `json:"tool_name"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments: " + err.Error()), nil
		}
		name := args.Name
		if name == "" && boundDef != nil {
			name = boundDef.Name
		}
		def, ok := pr.store.Get(name)
		if !ok {
			return errorResult(fmt.Sprintf("project %q not found", name)), nil
		}

		rule := project.GetEffectiveRule(def, pr.serverID, "")
		defaults := project.ResolveDefaults(args.ToolName, rule, nil)

		data, _ := json.MarshalIndent(defaults, "", "  ")
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(data)}},
		}, nil
	}
}

func (pr *ProjectRouter) resolveProjectName(req *mcpsdk.CallToolRequest, boundDef *project.Definition) string {
	var args struct {
		Name string `json:"name"`
	}
	if req.Params.Arguments != nil {
		_ = json.Unmarshal(req.Params.Arguments, &args)
	}
	if args.Name != "" {
		return args.Name
	}
	if boundDef != nil {
		return boundDef.Name
	}
	return ""
}
