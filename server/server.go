package server

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/script"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultSearchLimit = 20
const maxResponseBytes = 50 * 1024 // 50KB

const (
	defaultBreakerThreshold = 5
	defaultBreakerCooldown  = 30 * time.Second
)

// Server wraps the MCP SDK server and exposes search/execute tools.
type Server struct {
	mcpServer        *mcpsdk.Server
	services         *mcp.Services
	scriptEngine     *script.Engine
	retryBackoff     time.Duration
	breakers         map[string]*breaker
	breakerThreshold int
	breakerCooldown  time.Duration
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
		mcpServer:        mcpServer,
		services:         services,
		retryBackoff:     500 * time.Millisecond,
		breakers:         make(map[string]*breaker),
		breakerThreshold: defaultBreakerThreshold,
		breakerCooldown:  defaultBreakerCooldown,
	}
	s.scriptEngine = script.New(&toolExecutor{server: s})

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
		Description: `Execute a tool or run a JavaScript script that chains multiple tool calls.

PREFER scripts when a task requires 2+ tool calls or crosses integrations — intermediate
results stay server-side and never enter the conversation, saving tokens dramatically.

Mode 1 — Script (provide script):
  Write ES5 JavaScript (var, function(){}, string + concatenation). No let/const, arrow functions, template literals, or destructuring.
  Call api.call(toolName, args) to invoke tools. Chain multiple calls, filter results, and return only what you need.

  {"script": "var issues = api.call('linear_search_issues', {query: 'BUG-1234'}); var email = issues[0].assignee.email; var user = api.call('postgres_execute_query', {query: 'SELECT * FROM users WHERE email = $1', params: [email]}); ({issue: issues[0], dbUser: user[0]});"}

Mode 2 — Single tool (provide tool_name + arguments):
  Use for one-off calls where scripting adds no benefit.

  {"tool_name": "github_list_issues", "arguments": {"owner": "golang", "repo": "go"}}

Script API:
  api.call(toolName, args) — call any tool, returns parsed JSON. Throws on error (kills script).
  api.tryCall(toolName, args) — like call, but returns {ok: true, data: ...} or {ok: false, error: "..."}. Prefer tryCall for cross-integration scripts where partial results are useful.
  console.log(...) — debug logging (included in output on error)

Scripts can call tools from ANY integration — chain GitHub, Linear, Sentry, Datadog, Slack, etc. in one script.

List and search responses are automatically compacted to essential fields.
Use single-item get tools (e.g., github_get_issue) for full detail.
Responses over 50KB return an error — use filters, lower limit/per_page, or fetch individual items.

Use search first to discover available tools and their parameter schemas.

Script examples:

Fetch a GitHub PR with its diff in a single call:
  {"script": "var pr = api.call('github_get_pull', {owner: 'o', repo: 'r', pull_number: 42}); var diff = api.call('github_get_pull_diff', {owner: 'o', repo: 'r', pull_number: 42}); ({title: pr.title, state: pr.state, body: pr.body, base: pr.base.ref, head: pr.head.ref, diff: diff});"}

Create a Linear issue then open a GitHub PR referencing it:
  {"script": "var issue = api.call('linear_create_issue', {team_id: 'TEAM-ID', title: 'Fix auth bug', description: 'Details...'}); var pr = api.call('github_create_pull', {owner: 'o', repo: 'r', title: issue.identifier + ': ' + issue.title, head: 'fix-auth', base: 'main', body: 'Resolves ' + issue.url}); ({issue: issue.identifier, pr_url: pr.html_url});"}

Look up a Sentry error, find the responsible deploy, and notify Slack:
  {"script": "var issue = api.call('sentry_get_issue', {issue_id: '12345'}); var deploys = api.call('sentry_list_deploys', {organization_slug: 'org', version: issue.firstRelease.version}); api.call('slack_post_message', {channel: '#alerts', text: 'Sentry issue ' + issue.title + ' introduced in deploy ' + deploys[0].environment}); ({sentry: issue.shortId, deploy: deploys[0].environment});"}`,
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
				"description": "ES5 JavaScript code to execute server-side. Use var (not let/const), function() (not =>), string + concatenation (not template literals). Use api.call(toolName, args) to invoke tools. Return the final result. (mutually exclusive with tool_name)",
			},
		}, nil),
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

		if err := integration.Configure(context.Background(), ic.Credentials); err != nil {
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
		ScriptHint   string     `json:"script_hint,omitempty"`
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

	var scriptHint string
	if total > 1 {
		seen := map[string]bool{}
		for _, t := range page {
			seen[t.Integration] = true
		}
		if len(seen) >= 2 {
			scriptHint = "These tools span multiple integrations. Use execute with a script to chain them in a single call — intermediate results stay server-side and never enter the conversation."
		} else if total > 1 {
			scriptHint = "Tip: if your task requires multiple tool calls, use execute with a script to chain them in a single call and reduce token usage."
		}
	}

	data, _ := json.Marshal(response{
		Summary:      summary,
		ScriptHint:   scriptHint,
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
		Script    string         `json:"script"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error()), nil
	}

	if args.Script != "" {
		return s.handleScriptExecute(ctx, args.Script)
	}

	if args.ToolName == "" {
		return errorResult("either tool_name or script is required"), nil
	}
	if args.Arguments == nil {
		args.Arguments = map[string]any{}
	}

	result, err := s.executeTool(ctx, args.ToolName, args.Arguments)
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

const maxScriptRetries = 10

func (s *Server) handleScriptExecute(ctx context.Context, source string) (*mcpsdk.CallToolResult, error) {
	ctx = withRetryBudget(ctx, maxScriptRetries)
	result, err := s.scriptEngine.Run(ctx, source)
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

const maxRetries = 3

// computeBackoff returns a jittered backoff duration for the given retry attempt.
// Uses equal jitter: half the exponential base plus a random value in [0, half].
// This guarantees a minimum wait while decorrelating concurrent retries.
func (s *Server) computeBackoff(attempt int) time.Duration {
	base := s.retryBackoff << attempt
	half := base / 2
	return half + time.Duration(rand.Int64N(int64(half)+1)) // #nosec G404 -- jitter, not security
}

// getBreaker returns the circuit breaker for the given integration, creating one if needed.
func (s *Server) getBreaker(integrationName string) *breaker {
	if b, ok := s.breakers[integrationName]; ok {
		return b
	}
	b := newBreaker(s.breakerThreshold, s.breakerCooldown)
	s.breakers[integrationName] = b
	return b
}

// executeTool finds and runs a single tool by name. Used by both direct
// execute and as the bridge for the script engine's api.call().
// Retries automatically on RetryableError (5xx, 429) with exponential backoff.
// Respects per-integration circuit breakers to avoid hammering down services.
func (s *Server) executeTool(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	integration, found := s.findIntegration(toolName)
	if !found {
		return &mcp.ToolResult{
			Data:    fmt.Sprintf("tool %q not found. Use the search tool to discover available tools.", toolName),
			IsError: true,
		}, nil
	}

	cb := s.getBreaker(integration.Name())
	if !cb.allow() {
		return &mcp.ToolResult{
			Data:    fmt.Sprintf("integration %q temporarily unavailable (circuit breaker open)", integration.Name()),
			IsError: true,
		}, nil
	}

	var lastErr error
	for attempt := range maxRetries {
		result, err := integration.Execute(ctx, toolName, args)
		if err == nil {
			cb.recordSuccess()
			if !result.IsError {
				result.Data = compactResult(integration, toolName, result.Data)
				if len(result.Data) > maxResponseBytes {
					return &mcp.ToolResult{
						Data: fmt.Sprintf(
							"Response exceeded %dKB (actual: %dKB). Use more specific filters, lower limit/per_page, or fetch individual items.",
							maxResponseBytes/1024,
							len(result.Data)/1024,
						),
						IsError: true,
					}, nil
				}
			}
			return result, nil
		}

		if !mcp.IsRetryable(err) {
			return result, err
		}
		lastErr = err
		cb.recordFailure()
		if attempt >= maxRetries-1 {
			break
		}
		if !consumeRetry(ctx) {
			break // script retry budget exhausted
		}
		backoff := s.computeBackoff(attempt)
		// Prefer server-suggested Retry-After over computed backoff.
		var re *mcp.RetryableError
		if errors.As(err, &re) && re.RetryAfter > 0 {
			backoff = re.RetryAfter
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}

	// All retries exhausted — convert last retryable error to ToolResult
	return &mcp.ToolResult{Data: lastErr.Error(), IsError: true}, nil
}

// findIntegration returns the integration that owns toolName, or false if not found.
func (s *Server) findIntegration(toolName string) (mcp.Integration, bool) {
	for _, name := range s.services.Config.EnabledIntegrations() {
		integration, ok := s.services.Registry.Get(name)
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

// toolExecutor bridges the script.Executor interface to the server's tool dispatch.
type toolExecutor struct {
	server *Server
}

func (te *toolExecutor) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	return te.server.executeTool(ctx, toolName, args)
}

// compactResult applies field compaction if the integration opts in.
// Returns the original data unchanged if the integration doesn't implement
// FieldCompactionIntegration, returns nil specs, or compaction fails.
func compactResult(integration mcp.Integration, toolName string, data string) string {
	pi, ok := integration.(mcp.FieldCompactionIntegration)
	if !ok {
		return data
	}
	fields, ok := pi.CompactSpec(toolName)
	if !ok {
		return data
	}
	originalLen := len(data)
	compacted, err := mcp.CompactJSON([]byte(data), fields)
	if err != nil {
		slog.Warn("compaction failed, returning full response", "tool", toolName, "err", err)
		return data
	}
	if slog.Default().Handler().Enabled(context.Background(), slog.LevelDebug) {
		compactedLen := len(compacted)
		savingsPct := 0
		if originalLen > 0 {
			savingsPct = 100 - 100*compactedLen/originalLen
		}
		slog.Debug("compaction applied",
			"tool", toolName,
			"before_bytes", originalLen,
			"after_bytes", compactedLen,
			"savings_pct", savingsPct,
		)
	}
	return string(compacted)
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
