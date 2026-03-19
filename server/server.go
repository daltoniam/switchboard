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

// searchToolInfo represents a tool in search results.
type searchToolInfo struct {
	Integration string            `json:"integration"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters"`
	Required    []string          `json:"required,omitempty"`
}

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
- Filter by integration: {"integration": "github"}
- Search by action: {"query": "list issues"}
- Combined filter: {"integration": "slack", "query": "send message"}
- Search specific tool: {"query": "datadog_search_logs"}
- Page through results: {"query": "github", "offset": 20, "limit": 20}
- Count all tools: {"limit": 0}`,
		InputSchema: objectSchema(map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query to filter tools. Leave empty to list all available tools. Matches against tool names, descriptions, and integration names.",
			},
			"integration": map[string]any{
				"type":        "string",
				"description": "Filter by integration name (e.g., \"github\", \"slack\", \"linear\"). When set, only returns tools from that integration.",
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
  api.call(toolName, args[, opts]) — call any tool, returns parsed JSON. Throws on error (kills script).
    Optional opts object with fields key applies server-side field projection: {fields: ["id", "title", "user.login"]}.
    Uses dot-notation: {fields: ["id", "title", "user.login", "labels[].name"]}. Only specified fields are kept.
  api.tryCall(toolName, args[, opts]) — like call, but returns {ok: true, data: ...} or {ok: false, error: "..."}.
    Also supports the optional opts with field projection. Prefer tryCall for cross-integration scripts where partial results are useful.
  console.log(...) — debug logging (included in output on error)

Scripts can call tools from ANY integration — chain GitHub, Linear, Sentry, Datadog, Slack, etc. in one script.

List and search responses are automatically compacted to essential fields.
Use single-item get tools (e.g., github_get_issue) for full detail.
Responses over 50KB return an error — use filters, lower limit/per_page, or fetch individual items.
Script output is also capped at 50KB — return only the fields you need, not entire API responses.

Use search first to discover available tools and their parameter schemas.

Script examples:

Fetch a GitHub PR with field projection (only title, state, and branch refs returned):
  {"script": "var pr = api.call('github_get_pull', {owner: 'o', repo: 'r', pull_number: 42}, {fields: ['title', 'state', 'body', 'base.ref', 'head.ref']}); var diff = api.call('github_get_pull_diff', {owner: 'o', repo: 'r', pull_number: 42}); ({pr: pr, diff: diff});"}

Create a Linear issue then open a GitHub PR referencing it:
  {"script": "var issue = api.call('linear_create_issue', {team_id: 'TEAM-ID', title: 'Fix auth bug', description: 'Details...'}); var pr = api.call('github_create_pull', {owner: 'o', repo: 'r', title: issue.identifier + ': ' + issue.title, head: 'fix-auth', base: 'main', body: 'Resolves ' + issue.url}); ({issue: issue.identifier, pr_url: pr.html_url});"}

Look up a Sentry error, find the responsible deploy, and notify Slack:
  {"script": "var issue = api.call('sentry_get_issue', {issue_id: '12345'}); var deploys = api.call('sentry_list_deploys', {organization_slug: 'org', version: issue.firstRelease.version}); api.call('slack_post_message', {channel: '#alerts', text: 'Sentry issue ' + issue.title + ' introduced in deploy ' + deploys[0].environment}); ({sentry: issue.shortId, deploy: deploys[0].environment});"}

Cross-integration correlation with tryCall and field projection:
  {"script": "var pr = api.call('github_get_pull', {owner: 'o', repo: 'r', pull_number: 42}, {fields: ['title', 'state']}); var linear = api.tryCall('linear_search_issues', {query: pr.title}, {fields: ['issues.nodes[].identifier', 'issues.nodes[].title']}); ({pr: pr, linear: linear.ok ? linear.data : {error: linear.error}});"}

List issues with server-side projection (only id, title, labels — no manual .map() needed):
  {"script": "api.call('github_list_issues', {owner: 'o', repo: 'r', state: 'open'}, {fields: ['number', 'title', 'labels[].name']});"}`,
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
				"description": "ES5 JavaScript code to execute server-side. Use var (not let/const), function() (not =>), string + concatenation (not template literals). Use api.call(toolName, args, {fields: [...]}) to invoke tools with optional field projection. Return the final result. (mutually exclusive with tool_name)",
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
			if ic.Enabled {
				ic.Enabled = false
				_ = s.services.Config.SetIntegration(name, ic)
			}
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
		Query       string `json:"query"`
		Integration string `json:"integration"`
		Limit       *int   `json:"limit"`
		Offset      int    `json:"offset"`
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

	type toolInfo = searchToolInfo

	enabled := s.services.Config.EnabledIntegrations()
	var all []toolInfo

	for _, name := range enabled {
		if args.Integration != "" && name != args.Integration {
			continue
		}
		integration, ok := s.services.Registry.Get(name)
		if !ok {
			continue
		}

		for _, tool := range integration.Tools() {
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
		Summary          string            `json:"summary"`
		ScriptHint       string            `json:"script_hint,omitempty"`
		SharedParameters map[string]string `json:"shared_parameters,omitempty"`
		Total            int               `json:"total"`
		Offset           int               `json:"offset"`
		Limit            int               `json:"limit"`
		HasMore          bool              `json:"has_more"`
		Integrations     []string          `json:"integrations"`
		Tools            []toolInfo        `json:"tools"`
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

	shared := extractSharedParameters(page)

	data, _ := json.Marshal(response{
		Summary:          summary,
		ScriptHint:       scriptHint,
		SharedParameters: shared,
		Total:            total,
		Offset:           offset,
		Limit:            limit,
		HasMore:          limit > 0 && offset+limit < total,
		Integrations:     enabled,
		Tools:            page,
	})

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: columnarizeResult(string(data))},
		},
	}, nil
}

// extractSharedParameters finds parameters with identical name+description
// across 3+ tools in the page, moves them to a shared map, and removes them
// from per-tool parameters. This deduplicates common params like "owner" and
// "repo" that appear verbatim across dozens of tools in an integration.
func extractSharedParameters(tools []searchToolInfo) map[string]string {
	const minCount = 3

	type paramKey struct{ name, desc string }
	counts := map[paramKey]int{}
	for _, t := range tools {
		for name, desc := range t.Parameters {
			counts[paramKey{name, desc}]++
		}
	}

	// For each param name, collect all descriptions that meet the threshold.
	// A name with multiple qualifying descriptions is ambiguous — skip it.
	candidates := map[string]string{} // name → description
	conflicted := map[string]bool{}
	for pk, count := range counts {
		if count < minCount {
			continue
		}
		if conflicted[pk.name] {
			continue
		}
		if prev, exists := candidates[pk.name]; exists && prev != pk.desc {
			delete(candidates, pk.name)
			conflicted[pk.name] = true
			continue
		}
		candidates[pk.name] = pk.desc
	}

	if len(candidates) == 0 {
		return nil
	}

	for i := range tools {
		for name, desc := range tools[i].Parameters {
			if candidates[name] == desc {
				delete(tools[i].Parameters, name)
			}
		}
	}

	return candidates
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

	integration, result, err := s.executeTool(ctx, args.ToolName, args.Arguments)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	if result.IsError {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: result.Data}},
			IsError: true,
		}, nil
	}
	result.Data = processResult(integration, args.ToolName, result.Data)
	if len(result.Data) > maxResponseBytes {
		return errorResult(fmt.Sprintf(
			"Response exceeded %dKB (actual: %dKB). Use more specific filters, lower limit/per_page, or fetch individual items.",
			maxResponseBytes/1024,
			len(result.Data)/1024,
		)), nil
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: result.Data},
		},
	}, nil
}

const maxScriptRetries = 10

func (s *Server) handleScriptExecute(ctx context.Context, source string) (*mcpsdk.CallToolResult, error) {
	ctx = withRetryBudget(ctx, maxScriptRetries)
	result, err := s.scriptEngine.Run(ctx, source)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	if result.IsError {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: result.Data}},
			IsError: true,
		}, nil
	}
	result.Data = columnarizeResult(result.Data)
	if len(result.Data) > maxResponseBytes {
		return errorResult(fmt.Sprintf(
			"Script output exceeded %dKB (actual: %dKB). Return only the fields you need from each api.call() result.",
			maxResponseBytes/1024,
			len(result.Data)/1024,
		)), nil
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: result.Data},
		},
	}, nil
}

// processResult applies compaction and columnarization in a single parse/serialize cycle.
// Parses JSON once, applies compact specs (if any), columnarizes arrays, and serializes once.
func processResult(integration mcp.Integration, toolName string, data string) string {
	trimmed := strings.TrimLeft(data, " \t\n\r")
	if len(trimmed) == 0 || (trimmed[0] != '[' && trimmed[0] != '{') {
		return data
	}
	originalLen := len(data)

	var parsed any
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		slog.Warn("processResult: unmarshal failed", "tool", toolName, "err", err)
		return data
	}

	if pi, ok := integration.(mcp.FieldCompactionIntegration); ok {
		if fields, ok := pi.CompactSpec(toolName); ok {
			parsed = mcp.CompactAny(parsed, fields)
		}
	}

	parsed = mcp.ColumnarizeAny(parsed)

	result, err := json.Marshal(parsed)
	if err != nil {
		slog.Warn("processResult: marshal failed", "tool", toolName, "err", err)
		return data
	}

	if slog.Default().Handler().Enabled(context.Background(), slog.LevelDebug) {
		savings := 0
		if originalLen > 0 {
			savings = 100 - 100*len(result)/originalLen
		}
		slog.Debug("processResult: applied",
			"tool", toolName,
			"before_bytes", originalLen,
			"after_bytes", len(result),
			"savings_pct", savings,
		)
	}

	return string(result)
}

// columnarizeResult applies columnar formatting only (no compaction).
// Used by script and search paths where per-tool compaction doesn't apply.
func columnarizeResult(data string) string {
	trimmed := strings.TrimLeft(data, " \t\n\r")
	if len(trimmed) == 0 || (trimmed[0] != '[' && trimmed[0] != '{') {
		return data
	}
	var parsed any
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		slog.Warn("columnarizeResult: unmarshal failed", "err", err)
		return data
	}
	result, err := json.Marshal(mcp.ColumnarizeAny(parsed))
	if err != nil {
		slog.Warn("columnarizeResult: marshal failed", "err", err)
		return data
	}
	return string(result)
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
// Returns the owning integration alongside the result so callers can
// apply post-processing (e.g. compaction) without a redundant findTool call.
// Retries automatically on RetryableError (5xx, 429) with exponential backoff.
// Respects per-integration circuit breakers to avoid hammering down services.
func (s *Server) executeTool(ctx context.Context, toolName string, args map[string]any) (mcp.Integration, *mcp.ToolResult, error) {
	integration, toolDef, found := s.findTool(toolName)
	if !found {
		return nil, &mcp.ToolResult{
			Data:    fmt.Sprintf("tool %q not found. Use the search tool to discover available tools.", toolName),
			IsError: true,
		}, nil
	}

	if err := validateArgs(toolDef, args); err != nil {
		return nil, &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
	}

	cb := s.getBreaker(integration.Name())
	if !cb.allow() {
		return nil, &mcp.ToolResult{
			Data: fmt.Sprintf(
				"integration %q temporarily unavailable (circuit breaker open, try again in ~%ds). Other integrations still work.",
				integration.Name(), int(s.breakerCooldown.Seconds()),
			),
			IsError: true,
		}, nil
	}

	var lastErr error
	for attempt := range maxRetries {
		result, err := integration.Execute(ctx, toolName, args)
		if err == nil {
			cb.recordSuccess()
			return integration, result, nil
		}

		if !mcp.IsRetryable(err) {
			return nil, result, err
		}
		lastErr = err
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
			return nil, nil, ctx.Err()
		case <-time.After(backoff):
		}
	}

	// All retries exhausted — record one failure per call (not per attempt).
	cb.recordFailure()
	return nil, &mcp.ToolResult{Data: lastErr.Error(), IsError: true}, nil
}

// findTool returns the integration and tool definition that owns toolName, or false if not found.
func (s *Server) findTool(toolName string) (mcp.Integration, mcp.ToolDefinition, bool) {
	for _, name := range s.services.Config.EnabledIntegrations() {
		integration, ok := s.services.Registry.Get(name)
		if !ok {
			continue
		}
		for _, tool := range integration.Tools() {
			if tool.Name == toolName {
				return integration, tool, true
			}
		}
	}
	return nil, mcp.ToolDefinition{}, false
}

// validateArgs checks that all required parameters are present and all provided
// parameters are declared in the tool's schema. Returns a descriptive error
// naming the offending parameter and suggesting corrections for typos.
func validateArgs(tool mcp.ToolDefinition, args map[string]any) error {
	for _, req := range tool.Required {
		if _, ok := args[req]; !ok {
			return fmt.Errorf("missing required parameter %q for tool %q. Required: %v", req, tool.Name, tool.Required)
		}
	}
	if len(tool.Parameters) == 0 {
		return nil // no schema declared — skip unknown-param check
	}
	for key := range args {
		if _, ok := tool.Parameters[key]; ok {
			continue
		}
		return unknownParamError(key, tool)
	}
	return nil
}

func unknownParamError(key string, tool mcp.ToolDefinition) error {
	valid := paramNames(tool.Parameters)
	suggestion := closestParam(key, tool.Parameters)
	if suggestion != "" {
		return fmt.Errorf("unknown parameter %q for tool %q, did you mean %q? Valid parameters: %v",
			key, tool.Name, suggestion, valid)
	}
	return fmt.Errorf("unknown parameter %q for tool %q. Valid parameters: %v",
		key, tool.Name, valid)
}

// closestParam returns the parameter name closest to key by edit distance,
// or empty string if no parameter is within a reasonable threshold.
func closestParam(key string, params map[string]string) string {
	best := ""
	bestDist := len(key)/2 + 1 // threshold: half the key length
	for name := range params {
		d := editDistance(key, name)
		if d < bestDist {
			bestDist = d
			best = name
		}
	}
	return best
}

// editDistance computes the Levenshtein distance between two strings.
func editDistance(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr := make([]int, lb+1)
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev = curr
	}
	return prev[lb]
}

// paramNames returns sorted parameter names for error messages.
func paramNames(params map[string]string) []string {
	names := make([]string, 0, len(params))
	for name := range params {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// toolExecutor bridges the script.Executor interface to the server's tool dispatch.
type toolExecutor struct {
	server *Server
}

func (te *toolExecutor) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	// Integration is discarded: script-path calls intentionally skip per-tool
	// compaction so scripts can access all fields by name before projecting.
	_, result, err := te.server.executeTool(ctx, toolName, args)
	return result, err
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
