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
	"github.com/daltoniam/switchboard/version"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultSearchLimit = 20
const defaultMaxResponseBytes = 50 * 1024 // 50KB

// responseLimitFor returns the effective response byte cap for an integration.
// Integrations that implement MaxResponseBytesIntegration may declare a higher
// cap; values at or below the default are ignored.
func responseLimitFor(integration mcp.Integration) int {
	if mri, ok := integration.(mcp.MaxResponseBytesIntegration); ok {
		if v := mri.MaxResponseBytes(); v > defaultMaxResponseBytes {
			return v
		}
	}
	return defaultMaxResponseBytes
}

// searchToolInfo represents a tool in search results.
type searchToolInfo struct {
	Integration string            `json:"integration"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters"`
	Required    []string          `json:"required,omitempty"`
	Configured  *bool             `json:"configured,omitempty"` // nil = omitted (configured); false = not yet configured
}

// searchableIntegration pairs an integration with its name for iteration.
type searchableIntegration struct {
	name        string
	integration mcp.Integration
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
	sessionStore     SessionStore
	retryBackoff     time.Duration
	breakers         map[string]*breaker
	breakerThreshold int
	breakerCooldown  time.Duration
	idf              map[string]float64
	synMap           map[string][]string
	allTools         []toolWithIntegration // pre-indexed tools with token sets
	discoverAll      bool
}

// Option configures optional Server behavior.
type Option func(*Server)

// WithDiscoverAll makes search return tools from all registered
// integrations, not just enabled ones. Unconfigured tools are marked
// with configured=false so LLMs know they can't be executed yet.
func WithDiscoverAll(v bool) Option {
	return func(s *Server) { s.discoverAll = v }
}

// WithSessionTTL configures how long idle sessions are kept before eviction.
// A zero or negative value uses DefaultSessionTTL (1h).
func WithSessionTTL(ttl time.Duration) Option {
	return func(s *Server) { s.sessionStore = NewMemorySessionStore(ttl) }
}

// WithSessionStore replaces the default in-memory session store with a custom
// implementation. Use this to plug in durable storage (Postgres, file-backed, etc.).
func WithSessionStore(store SessionStore) Option {
	return func(s *Server) { s.sessionStore = store }
}

// New creates a Server that exposes two MCP tools — search and execute —
// following the Cloudflare "code mode" pattern for progressive discovery
// and efficient tool execution.
func New(services *mcp.Services, opts ...Option) *Server {
	mcpServer := mcpsdk.NewServer(
		&mcpsdk.Implementation{
			Name:    "switchboard",
			Version: version.String(),
		},
		&mcpsdk.ServerOptions{
			Instructions: "Switchboard aggregates tools from multiple integrations " +
				"behind two meta-tools: search and execute. " +
				"Always search before calling execute — do not guess tool names. " +
				"Search uses synonym matching, so short keyword queries work best " +
				"(e.g., {\"query\": \"get page\"} finds notion_retrieve_page). " +
				"Use the session tool to set context (e.g., owner/repo) once — " +
				"subsequent execute calls auto-inject those values as defaults. " +
				"Use the history tool to review prior calls after context compression. " +
				"Every execute result is auto-pinned ($1, $2, ...) — use pin to retrieve or " +
				"pass handles like $1.field in execute arguments to reference previous results.",
		},
	)

	s := &Server{
		mcpServer:        mcpServer,
		services:         services,
		sessionStore:     NewMemorySessionStore(DefaultSessionTTL),
		retryBackoff:     500 * time.Millisecond,
		breakers:         make(map[string]*breaker),
		breakerThreshold: defaultBreakerThreshold,
		breakerCooldown:  defaultBreakerCooldown,
	}
	for _, opt := range opts {
		opt(s)
	}
	s.scriptEngine = script.New(&toolExecutor{server: s})

	s.registerTools()
	return s
}

func (s *Server) registerTools() {
	s.configureIntegrations()
	s.buildSearchIndex()

	searchTool := &mcpsdk.Tool{
		Name: "search",
		Description: `Search available tools across all integrations (GitHub, Datadog, Linear, Sentry, Slack, etc.).

IMPORTANT: Always search before calling execute. Do NOT guess tool names.

Query format — use 1-2 keywords, not full sentences. Fewer words = better results:
- {"query": "create ticket"} — synonym matching finds linear_create_issue
- {"query": "slack send"} — integration name + verb is ideal
- {"integration": "sentry", "query": "errors"} — or use the integration filter
Avoid 4+ word queries — they return fewer results. Search twice with short queries instead of once with a long query.`,
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
  api.call(toolName, args[, opts]) — call any integration tool, returns parsed JSON. Throws on error (kills script).
    Optional opts object with fields key applies server-side field projection: {fields: ["id", "title", "user.login"]}.
    Uses dot-notation: {fields: ["id", "title", "user.login", "labels[].name"]}. Only specified fields are kept.
  api.tryCall(toolName, args[, opts]) — like call, but returns {ok: true, data: ...} or {ok: false, error: "..."}.
    Also supports the optional opts with field projection. Prefer tryCall for cross-integration scripts where partial results are useful.
  console.log(...) — debug logging (included in output on error)

Scripts can call integration tools — chain GitHub, Linear, Sentry, Datadog, Slack, etc. in one script.
Scripts CANNOT call the search or execute meta-tools. Use search before writing a script to discover tool names.

List and search responses are automatically compacted to essential fields.
Use single-item get tools (e.g., github_get_issue) for full detail.
Responses over 50KB return an error — use filters, lower limit/per_page, or fetch individual items.
Script output is also capped at 50KB — return only the fields you need, not entire API responses.

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
			"dry_run": map[string]any{
				"type":        "boolean",
				"description": "If true, validate arguments and show what would happen without executing. Works with tool_name only (not scripts).",
			},
		}, nil),
	}

	sessionTool := &mcpsdk.Tool{
		Name: "session",
		Description: `Manage session-scoped context to avoid repeating parameters.

Set context once (e.g., owner/repo) and all subsequent execute calls auto-inject those values as defaults.
Explicit arguments always override session context.

Actions:
- "set": Upsert key-value pairs into session context
- "get": Return current session context
- "clear": Reset session context to empty

Example: {"action": "set", "context": {"owner": "daltoniam", "repo": "switchboard"}}
Then: execute({tool_name: "github_list_issues", arguments: {state: "open"}}) — owner/repo injected automatically.`,
		InputSchema: objectSchema(map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": `The action to perform: "set", "get", or "clear".`,
				"enum":        []string{"set", "get", "clear"},
			},
			"context": map[string]any{
				"type":        "object",
				"description": "Key-value pairs to set in session context (only for \"set\" action).",
			},
		}, []string{"action"}),
	}

	historyTool := &mcpsdk.Tool{
		Name: "history",
		Description: `Retrieve a compact log of tool calls made in this session.

Useful after context compression to recover what was already fetched without re-executing.
Returns: [{seq, tool, args, summary, is_error, timestamp}] ordered by time.`,
		InputSchema: objectSchema(map[string]any{
			"last_n": map[string]any{
				"type":        "integer",
				"description": "Number of recent entries to return (default 20, max 200).",
			},
			"tool": map[string]any{
				"type":        "string",
				"description": "Filter breadcrumbs to a specific tool name.",
			},
		}, nil),
	}

	pinTool := &mcpsdk.Tool{
		Name: "pin",
		Description: `Manage pinned results from previous execute calls.

Every successful execute auto-pins its result with a handle ($1, $2, ...).
Use handles in execute arguments to reference previous results without re-fetching:
  execute({tool_name: "github_get_issue", arguments: {owner: "$1.owner.login", issue_number: "$2.number"}})

Actions:
- "list": Show all pinned handles with tool name and size
- "get": Retrieve a pinned result by handle, optionally extracting a sub-field via path
- "unpin": Free memory by removing a pinned result`,
		InputSchema: objectSchema(map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": `The action to perform: "list", "get", or "unpin".`,
				"enum":        []string{"list", "get", "unpin"},
			},
			"handle": map[string]any{
				"type":        "string",
				"description": "The handle to operate on (e.g. \"$1\"). Required for get and unpin.",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Dot-notation path to extract a sub-field (e.g. \"user.login\"). Only for get.",
			},
		}, []string{"action"}),
	}

	s.mcpServer.AddTool(searchTool, s.handleSearch)
	s.mcpServer.AddTool(executeTool, s.handleExecute)
	s.mcpServer.AddTool(sessionTool, s.handleSession)
	s.mcpServer.AddTool(historyTool, s.handleHistory)
	s.mcpServer.AddTool(pinTool, s.handlePin)
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
		case mcp.CredKeyClientID, mcp.CredKeyClientSecret, mcp.CredKeyTokenSource:
			continue
		default:
			return true
		}
	}
	return false
}

// searchableIntegrationNames returns the list of integration names included in search.
func (s *Server) searchableIntegrationNames() []string {
	if s.discoverAll {
		return s.services.Registry.Names()
	}
	return s.services.Config.EnabledIntegrations()
}

// SearchIndex returns the pre-computed search index for sharing with
// ProjectRouter. The returned data is read-only after init.
func (s *Server) SearchIndex() SearchIndex {
	return SearchIndex{IDF: s.idf, SynMap: s.synMap, AllTools: s.allTools}
}

// buildSearchIndex builds the synonym map and IDF index for scored search.
// When discoverAll is true, indexes all registered integrations (not just enabled).
func (s *Server) buildSearchIndex() {
	s.synMap = buildSynonymMap(synonymGroups)

	var tools []toolWithIntegration
	if s.discoverAll {
		for _, integration := range s.services.Registry.All() {
			name := integration.Name()
			for _, tool := range integration.Tools() {
				tools = append(tools, toolWithIntegration{Integration: name, Tool: tool})
			}
		}
	} else {
		for _, name := range s.services.Config.EnabledIntegrations() {
			integration, ok := s.services.Registry.Get(name)
			if !ok {
				continue
			}
			for _, tool := range integration.Tools() {
				tools = append(tools, toolWithIntegration{Integration: name, Tool: tool})
			}
		}
	}
	s.idf = computeIDF(tools) // also populates tools[i].tokens
	s.allTools = tools
}

func (s *Server) handleSearch(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if s.services.Metrics != nil {
		s.services.Metrics.RecordSearch()
	}
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

	// Build the set of enabled integrations for configured/unconfigured tagging.
	enabledList := s.services.Config.EnabledIntegrations()
	enabledSet := make(map[string]bool, len(enabledList))
	for _, name := range enabledList {
		enabledSet[name] = true
	}

	// Collect searchable integrations: enabled only, or all registered if discoverAll.
	var searchable []searchableIntegration
	if s.discoverAll {
		for _, i := range s.services.Registry.All() {
			searchable = append(searchable, searchableIntegration{name: i.Name(), integration: i})
		}
	} else {
		for _, name := range enabledList {
			if i, ok := s.services.Registry.Get(name); ok {
				searchable = append(searchable, searchableIntegration{name: name, integration: i})
			}
		}
	}

	var all []toolInfo

	if query != "" {
		all = s.scoredSearch(query, args.Integration)
	} else {
		all = s.unrankedSearch(args.Integration, searchable)
	}

	// Tag unconfigured tools when discoverAll is active.
	if s.discoverAll {
		for i := range all {
			if !enabledSet[all[i].Integration] {
				f := false
				all[i].Configured = &f
			}
		}
	}

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

	data, err := json.Marshal(response{
		Summary:          summary,
		ScriptHint:       scriptHint,
		SharedParameters: shared,
		Total:            total,
		Offset:           offset,
		Limit:            limit,
		HasMore:          limit > 0 && offset+limit < total,
		Integrations:     s.searchableIntegrationNames(),
		Tools:            page,
	})
	if err != nil {
		return errorResult("marshal search response: " + err.Error()), nil
	}

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: columnarizeResult(string(data))},
		},
	}, nil
}

// scoredSearch returns tools ranked by TF-IDF + synonym relevance,
// respecting integration filter and ABAC tool globs.
func (s *Server) scoredSearch(query, integration string) []searchToolInfo {
	var candidates []toolWithIntegration
	for _, ti := range s.allTools {
		if integration != "" && ti.Integration != integration {
			continue
		}
		ic, _ := s.services.Config.GetIntegration(ti.Integration)
		if ic != nil && !ic.ToolAllowed(mcp.ToolName(ti.Tool.Name)) {
			continue
		}
		candidates = append(candidates, ti)
	}

	scored := scoreTools(query, candidates, s.idf, s.synMap)
	all := make([]searchToolInfo, len(scored))
	for i, r := range scored {
		all[i] = toToolInfo(r)
	}
	return all
}

// unrankedSearch returns all tools sorted alphabetically, respecting ABAC globs.
func (s *Server) unrankedSearch(integration string, searchable []searchableIntegration) []searchToolInfo {
	var all []searchToolInfo
	for _, si := range searchable {
		if integration != "" && si.name != integration {
			continue
		}
		ic, _ := s.services.Config.GetIntegration(si.name)
		for _, tool := range si.integration.Tools() {
			if ic != nil && !ic.ToolAllowed(tool.Name) {
				continue
			}
			all = append(all, toolDefToInfo(si.name, tool))
		}
	}
	slices.SortFunc(all, func(a, b searchToolInfo) int {
		if c := cmp.Compare(a.Integration, b.Integration); c != 0 {
			return c
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return all
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
		ToolName  mcp.ToolName   `json:"tool_name"`
		Arguments map[string]any `json:"arguments"`
		Script    string         `json:"script"`
		DryRun    bool           `json:"dry_run"`
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
	if args.ToolName == "search" || args.ToolName == "execute" || args.ToolName == "session" || args.ToolName == "history" || args.ToolName == "pin" {
		return errorResult(fmt.Sprintf(
			"tool %q is a meta-tool — use it directly as an MCP tool call, not through execute",
			args.ToolName)), nil
	}
	if args.Arguments == nil {
		args.Arguments = map[string]any{}
	}

	if args.DryRun {
		return s.handleDryRun(ctx, args.ToolName, args.Arguments)
	}

	sess := sessionFromCtx(ctx)
	if sess == nil {
		sess = s.sessionStore.GetOrCreate(sessionIDFromReq(req.Session))
	}
	resolveRefs(sess, args.Arguments)
	args.Arguments = sess.MergeDefaults(args.Arguments)

	integration, result, err := s.executeTool(ctx, args.ToolName, args.Arguments)
	if err != nil {
		sess.AddBreadcrumb(args.ToolName, args.Arguments, err.Error(), true)
		_ = s.sessionStore.Save(sess)
		return errorResult(err.Error()), nil
	}
	var handle string
	if !result.IsError {
		handle = sess.PinResult(args.ToolName, result.Data)
	}
	sess.AddBreadcrumb(args.ToolName, args.Arguments, result.Data, result.IsError)
	_ = s.sessionStore.Save(sess)
	if result.IsError {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: result.Data}},
			IsError: true,
		}, nil
	}
	result.Data = processResult(buildResultProcessor(integration), args.ToolName, result.Data, s.services.Metrics)
	limit := responseLimitFor(integration)
	if len(result.Data) > limit {
		if s.services.Metrics != nil {
			s.services.Metrics.RecordTruncation()
		}
		return errorResult(fmt.Sprintf(
			"Response exceeded %dKB (actual: %dKB). Use more specific filters, lower limit/per_page, or fetch individual items.",
			limit/1024,
			len(result.Data)/1024,
		)), nil
	}
	text := result.Data
	if handle != "" {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: text},
				&mcpsdk.TextContent{Text: "pinned as " + handle},
			},
		}, nil
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: text},
		},
	}, nil
}

const maxScriptRetries = 10

func (s *Server) handleScriptExecute(ctx context.Context, source string) (*mcpsdk.CallToolResult, error) {
	if s.services.Metrics != nil {
		s.services.Metrics.RecordScript()
	}
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
	if len(result.Data) > defaultMaxResponseBytes {
		if s.services.Metrics != nil {
			s.services.Metrics.RecordTruncation()
		}
		return errorResult(fmt.Sprintf(
			"Script output exceeded %dKB (actual: %dKB). Return only the fields you need from each api.call() result.",
			defaultMaxResponseBytes/1024,
			len(result.Data)/1024,
		)), nil
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: result.Data},
		},
	}, nil
}

// resultProcessor encapsulates an integration's response processing capabilities.
// Built from an integration via buildResultProcessor; decouples processResult
// from knowledge of which optional interfaces exist.
type resultProcessor struct {
	markdown func(mcp.ToolName, []byte) (mcp.Markdown, bool)
	compact  func(mcp.ToolName) ([]mcp.CompactField, bool)
}

// buildResultProcessor inspects an integration's optional interfaces once
// and captures them as function fields. The returned processor is safe to
// reuse for the lifetime of the integration (capabilities are static).
func buildResultProcessor(integration mcp.Integration) resultProcessor {
	var rp resultProcessor
	if mr, ok := integration.(mcp.MarkdownIntegration); ok {
		rp.markdown = mr.RenderMarkdown
	}
	if fc, ok := integration.(mcp.FieldCompactionIntegration); ok {
		rp.compact = fc.CompactSpec
	}
	return rp
}

// processResult applies markdown rendering, compaction, and columnarization.
// Markdown rendering takes priority — if the processor has a markdown function
// and it returns rendered content for this tool, compaction and columnarization
// are skipped. Otherwise, parses JSON once, applies compact specs (if any),
// columnarizes arrays, and serializes once.
func processResult(rp resultProcessor, toolName mcp.ToolName, data string, metrics *mcp.Metrics) string {
	trimmed := strings.TrimLeft(data, " \t\n\r")
	if len(trimmed) == 0 || (trimmed[0] != '[' && trimmed[0] != '{') {
		return data
	}

	// Try markdown rendering first — replaces JSON compaction entirely.
	if rp.markdown != nil {
		if md, rendered := rp.markdown(toolName, []byte(data)); rendered {
			if metrics != nil {
				metrics.RecordMarkdownRender(toolName, len(data), len(md))
			}
			return string(md)
		}
	}

	originalLen := len(data)

	var parsed any
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		slog.Warn("processResult: unmarshal failed", "tool", toolName, "err", err)
		return data
	}

	compacted := false
	if rp.compact != nil {
		if fields, ok := rp.compact(toolName); ok {
			parsed = mcp.CompactAny(parsed, fields)
			compacted = true
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

	if compacted && metrics != nil {
		metrics.RecordCompaction(toolName, originalLen, len(result))
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
func (s *Server) executeTool(ctx context.Context, toolName mcp.ToolName, args map[string]any) (mcp.Integration, *mcp.ToolResult, error) {
	integration, toolDef, err := s.findTool(toolName)
	if err != nil {
		return nil, &mcp.ToolResult{
			Data:    err.Error(),
			IsError: true,
		}, nil
	}

	if err := validateArgs(toolDef, args); err != nil {
		return nil, &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
	}

	cb := s.getBreaker(integration.Name())
	if !cb.allow() {
		if s.services.Metrics != nil {
			s.services.Metrics.RecordCircuitBreak(mcp.IntegrationName(integration.Name()))
		}
		return nil, &mcp.ToolResult{
			Data: fmt.Sprintf(
				"integration %q temporarily unavailable (circuit breaker open, try again in ~%ds). Other integrations still work.",
				integration.Name(), int(s.breakerCooldown.Seconds()),
			),
			IsError: true,
		}, nil
	}

	var lastErr error
	retries := 0
	var callDuration time.Duration
	for attempt := range maxRetries {
		callStart := time.Now()
		result, err := integration.Execute(ctx, toolName, args)
		callDuration += time.Since(callStart)
		if err == nil {
			cb.recordSuccess()
			if s.services.Metrics != nil {
				s.services.Metrics.RecordExecution(mcp.IntegrationName(integration.Name()), toolName, callDuration, false, retries)
			}
			return integration, result, nil
		}

		if !mcp.IsRetryable(err) {
			if s.services.Metrics != nil {
				s.services.Metrics.RecordExecution(mcp.IntegrationName(integration.Name()), toolName, callDuration, true, retries)
			}
			return nil, result, err
		}
		lastErr = err
		if attempt >= maxRetries-1 {
			break
		}
		if !consumeRetry(ctx) {
			break // script retry budget exhausted
		}
		retries++
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
	if s.services.Metrics != nil {
		s.services.Metrics.RecordExecution(mcp.IntegrationName(integration.Name()), toolName, callDuration, true, retries)
	}
	return nil, &mcp.ToolResult{Data: lastErr.Error(), IsError: true}, nil
}

// findTool returns the integration and tool definition that owns toolName.
// Respects ABAC tool glob restrictions. Returns a descriptive error when
// the tool exists but its integration is not configured.
func (s *Server) findTool(toolName mcp.ToolName) (mcp.Integration, mcp.ToolDefinition, error) {
	for _, name := range s.services.Config.EnabledIntegrations() {
		integration, ok := s.services.Registry.Get(name)
		if !ok {
			continue
		}
		ic, _ := s.services.Config.GetIntegration(name)
		for _, tool := range integration.Tools() {
			if tool.Name == toolName {
				if ic != nil && !ic.ToolAllowed(tool.Name) {
					continue
				}
				return integration, tool, nil
			}
		}
	}

	// When discoverAll is active, check if the tool exists in an unconfigured
	// integration. This gives LLMs an actionable "not configured" error instead
	// of a generic "not found" that triggers a retry loop.
	if s.discoverAll {
		for _, integration := range s.services.Registry.All() {
			for _, tool := range integration.Tools() {
				if tool.Name == toolName {
					return nil, mcp.ToolDefinition{}, fmt.Errorf(
						"tool %q exists but integration %q is not configured. Configure it via the web UI or config file",
						toolName, integration.Name())
				}
			}
		}
	}

	return nil, mcp.ToolDefinition{}, fmt.Errorf(
		"tool %q not found. Use the search tool to discover available tools", toolName)
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

func (te *toolExecutor) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	if toolName == "search" || toolName == "execute" || toolName == "session" || toolName == "history" || toolName == "pin" {
		return &mcp.ToolResult{
			Data: fmt.Sprintf(
				"tool %q is a meta-tool and cannot be called from scripts. "+
					"Use the search MCP tool before writing a script to discover tool names.",
				toolName),
			IsError: true,
		}, nil
	}
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
	searchable := strings.ToLower(string(tool.Name) + " " + tool.Description + " " + integrationName)
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
