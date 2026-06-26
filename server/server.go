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
	"sort"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/daltoniam/switchboard/script"
	"github.com/daltoniam/switchboard/server/prompts"
	"github.com/daltoniam/switchboard/version"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultSearchLimit = 20
const defaultMaxResponseBytes = 50 * 1024 // 50KB

// responseLimitFor returns the effective response byte cap for an integration
// and tool. Tools whose integration implements PerToolMaxResponseBytesIntegration
// and returns a value above the default take priority; otherwise the
// integration-wide MaxResponseBytesIntegration override applies; otherwise the
// server default. Values at or below the default are always ignored so the
// safety net cannot be lowered by an integration.
func responseLimitFor(integration mcp.Integration, toolName mcp.ToolName) int {
	if pti, ok := integration.(mcp.PerToolMaxResponseBytesIntegration); ok {
		if v, ok := pti.MaxResponseBytesForTool(toolName); ok && v > defaultMaxResponseBytes {
			return v
		}
	}
	if mri, ok := integration.(mcp.MaxResponseBytesIntegration); ok {
		if v := mri.MaxResponseBytes(); v > defaultMaxResponseBytes {
			return v
		}
	}
	return defaultMaxResponseBytes
}

// searchToolInfo represents a tool in search results.
//
// Required is parsed once at construction (toToolInfo / toolDefToInfo) from
// the source ToolDefinition's Parameters. The field is the parse-don't-validate
// proof of which parameters this tool requires — downstream code never
// re-derives required-ness from the Parameters slice. extractSharedParameters
// mutates Parameters (filtering shared params out for the wire response) but
// MUST NOT touch Required: the wire's required[] is the snapshot, not a
// derivation from the mutated slice.
//
// Permanent wire-shape contract: this type's JSON shape is documented in the
// meta-tool description. There is no inverse deserializer — the LLM consumes
// it but does not round-trip back into a searchToolInfo. Do not reconstruct
// from the wire shape.
type searchToolInfo struct {
	Integration string          `json:"integration"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  []mcp.Parameter `json:"parameters"`
	Required    []string        `json:"required,omitempty"`   // snapshotted at construction; never re-derived
	Configured  *bool           `json:"configured,omitempty"` // nil = omitted (configured); false = not yet configured
}

// MarshalJSON preserves the search-response wire format. Parameters serialize
// as a JSON object {name: description}, required as a separate sorted array —
// the shape LLM consumers of the search meta-tool depend on. Internal
// processing uses []mcp.Parameter for type-system consistency with
// ToolDefinition; this method bridges the two.
//
// Required is emitted from the snapshot field, not re-derived from Parameters.
// This preserves required-ness for parameters that extractSharedParameters has
// moved to shared_parameters (where the map[string]string wire shape would
// otherwise drop the required flag).
func (s searchToolInfo) MarshalJSON() ([]byte, error) {
	params := make(map[string]string, len(s.Parameters))
	for _, p := range s.Parameters {
		params[string(p.Name)] = p.Description
	}
	required := slices.Clone(s.Required)
	sort.Strings(required)

	type wire struct {
		Integration string            `json:"integration"`
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Parameters  map[string]string `json:"parameters"`
		Required    []string          `json:"required,omitempty"`
		Configured  *bool             `json:"configured,omitempty"`
	}
	return json.Marshal(wire{
		Integration: s.Integration,
		Name:        s.Name,
		Description: s.Description,
		Parameters:  params,
		Required:    required,
		Configured:  s.Configured,
	})
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
	searchMu         sync.RWMutex
	idf              map[string]float64
	synMap           map[string][]string
	allTools         []toolWithIntegration // pre-indexed tools with token sets
	catalogBytes     int64                 // byte size of full tool catalog (for savings accounting)
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
		Name:        "search",
		Description: prompts.Meta.Search(),
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
		Name:        "execute",
		Description: prompts.Meta.Execute(),
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

	sessionTool := &mcpsdk.Tool{
		Name:        "session",
		Description: prompts.Meta.Session(),
		InputSchema: objectSchema(map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": `"set" upserts key-value pairs into session context; "get" returns the current context; "clear" resets context to empty.`,
				"enum":        []string{"set", "get", "clear"},
			},
			"context": map[string]any{
				"type":        "object",
				"description": "Key-value pairs to set in session context (only for \"set\" action).",
			},
		}, []string{"action"}),
	}

	historyTool := &mcpsdk.Tool{
		Name:        "history",
		Description: prompts.Meta.History(),
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
		Name:        "pin",
		Description: prompts.Meta.Pin(),
		InputSchema: objectSchema(map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": `"list" shows all pinned handles with tool name and size; "get" retrieves a pinned result by handle, optionally extracting a sub-field via path; "unpin" frees memory by removing a pinned result.`,
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
		if exists && !ic.Enabled && !integrationHasCredentials(integration, ic.Credentials) {
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

func integrationHasCredentials(integration mcp.Integration, creds mcp.Credentials) bool {
	if detector, ok := integration.(mcp.CredentialDetector); ok {
		return detector.HasCredentials(creds)
	}
	return hasCredentials(creds)
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
	s.searchMu.RLock()
	defer s.searchMu.RUnlock()
	return SearchIndex{IDF: s.idf, SynMap: s.synMap, AllTools: s.allTools}
}

func (s *Server) RefreshSearchIndex() {
	s.buildSearchIndex()
}

// buildSearchIndex builds the synonym map and IDF index for scored search.
// When discoverAll is true, indexes all registered integrations (not just enabled).
func (s *Server) buildSearchIndex() {
	synMap := buildSynonymMap(synonymGroups)

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
	idf := computeIDF(tools) // also populates tools[i].tokens
	catalogBytes := computeCatalogBytes(tools)

	s.searchMu.Lock()
	s.idf = idf
	s.synMap = synMap
	s.allTools = tools
	s.catalogBytes = catalogBytes
	s.searchMu.Unlock()
}

// computeCatalogBytes returns the byte size of a faithful tools/list payload
// for the given tool set. This is the baseline an LLM client would have to
// receive on every turn if it connected directly to each vendor's MCP server
// instead of going through Switchboard's two-tool surface (search + execute).
// Each entry is rendered as {name, description, inputSchema{type, properties, required}}
// — the same shape vendor MCPs emit. Errors fall back to zero (i.e., no credit
// claimed) rather than failing the whole index build.
func computeCatalogBytes(tools []toolWithIntegration) int64 {
	type wireProp struct {
		Type        string `json:"type"`
		Description string `json:"description"`
	}
	type schema struct {
		Type       string              `json:"type"`
		Properties map[string]wireProp `json:"properties,omitempty"`
		Required   []string            `json:"required,omitempty"`
	}
	type listing struct {
		Name        mcp.ToolName `json:"name"`
		Description string       `json:"description"`
		InputSchema schema       `json:"inputSchema"`
	}
	entries := make([]listing, 0, len(tools))
	for _, t := range tools {
		props := make(map[string]wireProp, len(t.Tool.Parameters))
		var required []string
		for _, p := range t.Tool.Parameters {
			props[string(p.Name)] = wireProp{Type: "string", Description: p.Description}
			if p.Required {
				required = append(required, string(p.Name))
			}
		}
		sort.Strings(required)
		entries = append(entries, listing{
			Name:        t.Tool.Name,
			Description: t.Tool.Description,
			InputSchema: schema{
				Type:       "object",
				Properties: props,
				Required:   required,
			},
		})
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return 0
	}
	return int64(len(data))
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

	summary := prompts.SearchSummary(prompts.Context{}, total, args.Query)

	var scriptHint string
	if total > 1 {
		seen := map[string]bool{}
		for _, t := range page {
			seen[t.Integration] = true
		}
		if len(seen) >= 2 {
			scriptHint = prompts.SearchHintMulti(prompts.Context{})
		} else if total > 1 {
			scriptHint = prompts.SearchHintSingle(prompts.Context{})
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

	// Credit this search call with catalog-avoidance savings: every turn a
	// vanilla MCP client would have re-shipped the full catalog; here we only
	// shipped the matching slice. We use the columnarized response size since
	// that is what the LLM actually receives.
	columnarized := columnarizeResult(string(data))
	s.searchMu.RLock()
	catalogBytes := s.catalogBytes
	s.searchMu.RUnlock()
	if s.services.Metrics != nil && catalogBytes > 0 {
		avoided := catalogBytes - int64(len(columnarized))
		s.services.Metrics.RecordCatalogAvoidance(avoided)
	}

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: columnarized},
		},
	}, nil
}

// scoredSearch returns tools ranked by TF-IDF + synonym relevance,
// respecting integration filter and ABAC tool globs.
func (s *Server) scoredSearch(query, integration string) []searchToolInfo {
	s.searchMu.RLock()
	allTools := s.allTools
	idf := s.idf
	synMap := s.synMap
	s.searchMu.RUnlock()

	var candidates []toolWithIntegration
	for _, ti := range allTools {
		if integration != "" && ti.Integration != integration {
			continue
		}
		ic, _ := s.services.Config.GetIntegration(ti.Integration)
		if ic != nil && !ic.ToolAllowed(mcp.ToolName(ti.Tool.Name)) {
			continue
		}
		candidates = append(candidates, ti)
	}

	scored := scoreTools(query, candidates, idf, synMap)
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
// AND identical Required flag across 3+ tools in the page, moves them to a
// shared map, and removes them from per-tool parameters. This deduplicates
// common params like "owner" and "repo" that appear verbatim across dozens
// of tools in an integration.
//
// Required is part of the dedup key because it is part of the parameter's
// semantic identity. A param with `required:true` in some tools and
// `required:false` in others is two distinct parameters that happen to share
// a name, and collapsing them would emit a misleading shared entry. When the
// two variants both meet the count threshold, the name is conflicted and not
// extracted; it stays per-tool.
//
// Required-ness of extracted shared params survives the wire boundary via
// searchToolInfo.Required, which is snapshotted at construction. This
// function only mutates Parameters; it does not touch Required.
//
// Each searchToolInfo.Parameters is a clone (via slices.Clone in toToolInfo /
// toolDefToInfo), so filtering in place here does not corrupt the original
// ToolDefinition stored in the search index.
func extractSharedParameters(tools []searchToolInfo) map[string]string {
	const minCount = 3

	type paramKey struct {
		name     string
		desc     string
		required bool
	}
	counts := map[paramKey]int{}
	for _, t := range tools {
		for _, p := range t.Parameters {
			counts[paramKey{string(p.Name), p.Description, p.Required}]++
		}
	}

	// For each param name, collect all (desc, required) pairs that meet the
	// threshold. A name with multiple qualifying pairs is ambiguous — skip it.
	type candidate struct {
		desc     string
		required bool
	}
	candidates := map[string]candidate{}
	conflicted := map[string]bool{}
	for pk, count := range counts {
		if count < minCount {
			continue
		}
		if conflicted[pk.name] {
			continue
		}
		c := candidate{desc: pk.desc, required: pk.required}
		if prev, exists := candidates[pk.name]; exists && prev != c {
			delete(candidates, pk.name)
			conflicted[pk.name] = true
			continue
		}
		candidates[pk.name] = c
	}

	if len(candidates) == 0 {
		return nil
	}

	for i := range tools {
		filtered := tools[i].Parameters[:0]
		for _, p := range tools[i].Parameters {
			if c, ok := candidates[string(p.Name)]; ok && c == (candidate{desc: p.Description, required: p.Required}) {
				continue
			}
			filtered = append(filtered, p)
		}
		tools[i].Parameters = filtered
	}

	shared := make(map[string]string, len(candidates))
	for name, c := range candidates {
		shared[name] = c.desc
	}
	return shared
}

func (s *Server) handleExecute(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	var args struct {
		ToolName  mcp.ToolName   `json:"tool_name"`
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
	if args.ToolName == "search" || args.ToolName == "execute" || args.ToolName == "session" || args.ToolName == "history" || args.ToolName == "pin" {
		return errorResult(fmt.Sprintf(
			"tool %q is a meta-tool — use it directly as an MCP tool call, not through execute",
			args.ToolName)), nil
	}
	if args.Arguments == nil {
		args.Arguments = map[string]any{}
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
	applyResultProcessing(integration, args.ToolName, compact.ParseViewArgs(args.Arguments), result, s.services.Metrics)
	limit := responseLimitFor(integration, args.ToolName)
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
	// Record script byte flow regardless of error state: even an errored
	// script may have already issued api.call() invocations whose bytes
	// stayed server-side. The "final" portion is the actual ToolResult.Data
	// the LLM ends up seeing — for scripts returning tabular data,
	// columnarization can cut output 30-50%, so credit the post-columnar
	// size on the happy path.
	if result.IsError {
		if s.services.Metrics != nil {
			s.services.Metrics.RecordScriptSavings(result.IntermediateBytes, result.FinalBytes)
		}
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: result.Data}},
			IsError: true,
		}, nil
	}
	result.Data = columnarizeResult(result.Data)
	if s.services.Metrics != nil {
		s.services.Metrics.RecordScriptSavings(result.IntermediateBytes, int64(len(result.Data)))
	}
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
	maxBytes func(mcp.ToolName) (int, bool)
	views    func(mcp.ToolName) (compact.ViewSet, bool)
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
	if tm, ok := integration.(mcp.ToolMaxBytesIntegration); ok {
		rp.maxBytes = tm.MaxBytes
	}
	if tv, ok := integration.(compact.ToolViewsIntegration); ok {
		rp.views = tv.Views
	}
	return rp
}

// applyResultProcessing runs the integration's response pipeline on a successful
// tool result, mutating result.Data in place. No-op on IsError or nil integration.
//
// view carries the LLM's view/format selection, parsed at the request boundary
// via compact.ParseViewArgs. Passing a typed compact.ViewArgs (not a raw args
// map) is the structural defense against the "caller forgot to thread args"
// leak: the parameter has no nil — every callsite must parse explicitly, and
// "no selection" is the well-defined zero value (which means "use defaults").
func applyResultProcessing(integration mcp.Integration, toolName mcp.ToolName, view compact.ViewArgs, result *mcp.ToolResult, metrics *mcp.Metrics) {
	if integration == nil || result == nil || result.IsError {
		return
	}
	rp := buildResultProcessor(integration)
	result.Data = processResult(rp, toolName, view, result.Data, metrics)
}

// processResult applies markdown rendering, compaction, and columnarization.
// Markdown rendering takes priority — if the processor has a markdown function
// and it returns rendered content for this tool, compaction and columnarization
// are skipped. Otherwise, parses JSON once, applies compact specs (if any),
// columnarizes arrays, and serializes once.
//
// If the tool declares multi-view config (rp.views returns a ViewSet), the
// pipeline dispatches via processViewsResult instead — view.View and
// view.Format select the projection and renderer.
func processResult(rp resultProcessor, toolName mcp.ToolName, view compact.ViewArgs, data string, metrics *mcp.Metrics) string {
	trimmed := strings.TrimLeft(data, " \t\n\r")
	if len(trimmed) == 0 || (trimmed[0] != '[' && trimmed[0] != '{') {
		return data
	}

	// Multi-view path takes priority when declared.
	if rp.views != nil {
		if viewSet, ok := rp.views(toolName); ok {
			return processViewsResult(viewSet, toolName, view, data, metrics)
		}
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

	if rp.compact != nil {
		if fields, ok := rp.compact(toolName); ok {
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

	if metrics != nil && len(result) < originalLen {
		metrics.RecordCompaction(toolName, originalLen, len(result))
	}

	if rp.maxBytes != nil {
		if limit, ok := rp.maxBytes(toolName); ok && limit > 0 && len(result) > limit {
			return tooLargeEnvelope(toolName, len(result), limit)
		}
	}

	return string(result)
}

// processViewsResult handles the multi-view dispatch path. The tool's
// ViewSet was resolved at load time; here we parse the LLM's selection
// from args, apply the chosen view's spec, render in the chosen format,
// and enforce the per-view max_bytes cap.
//
// Unsupported (view, format) combos return a structured error envelope
// rather than silently falling back — the YAML is the contract, what's
// not declared is not available.
func processViewsResult(viewSet compact.ViewSet, toolName mcp.ToolName, view compact.ViewArgs, data string, metrics *mcp.Metrics) string {
	// Parse errors from the boundary surface here as a view envelope. The
	// parse boundary (compact.ParseViewArgs) catches type errors like
	// view=123; ViewSet-relative validation (unknown view, undeclared format)
	// happens in resolveSelection below.
	if err := view.Err(); err != nil {
		return viewErrorEnvelope(toolName, err)
	}

	selection, err := resolveSelection(view, viewSet)
	if err != nil {
		return viewErrorEnvelope(toolName, err)
	}

	renderer := viewSet.Renderers[selection.View][selection.Format]
	if renderer == nil {
		// Loader validated this combo at parse, so a miss here is a bug.
		return viewErrorEnvelope(toolName, fmt.Errorf("internal: no renderer for (view=%s, format=%s)", selection.View, selection.Format))
	}

	originalLen := len(data)

	var parsed any
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		slog.Warn("processViewsResult: unmarshal failed", "tool", toolName, "err", err)
		return data
	}

	parsedView := viewSet.Views[selection.View]
	if len(parsedView.Spec) > 0 {
		parsed = mcp.CompactAny(parsed, parsedView.Spec)
	}

	out, err := renderer(parsed)
	if err != nil {
		slog.Warn("processViewsResult: render failed", "tool", toolName, "view", selection.View, "format", selection.Format, "err", err)
		return viewErrorEnvelope(toolName, fmt.Errorf("render failed: %w", err))
	}

	out = appendMoreHint(out, viewSet, selection)

	if parsedView.MaxBytes > 0 && len(out) > parsedView.MaxBytes {
		return tooLargeEnvelope(toolName, len(out), parsedView.MaxBytes)
	}

	// Match the flat path's contract: only record when output actually shrank.
	// appendMoreHint can grow the response past input; recording every call
	// would feed negative savings into compaction-rate monitoring.
	if metrics != nil && len(out) < originalLen {
		metrics.RecordCompaction(toolName, originalLen, len(out))
	}

	return string(out)
}

// resolveSelection takes the typed-but-unvalidated ViewArgs from the request
// boundary and fills in defaults from the ViewSet, then validates the result
// against the registered renderers. Empty View/Format fields fall back to
// viewSet.Default. Unknown view names or undeclared formats produce an error
// that the caller wraps in a viewErrorEnvelope.
//
// This is the second half of the old parseViewSelection — type-parsing now
// happens at the request boundary in compact.ParseViewArgs, leaving this
// function to do only ViewSet-relative resolution.
func resolveSelection(view compact.ViewArgs, viewSet compact.ViewSet) (compact.ViewSelection, error) {
	selection := viewSet.Default
	if view.View != "" {
		if _, exists := viewSet.Views[view.View]; !exists {
			return selection, fmt.Errorf("unknown view %q (available: %s)", view.View, listViewNames(viewSet))
		}
		selection.View = view.View
	}
	if view.Format != "" {
		selection.Format = view.Format
	}
	if _, ok := viewSet.Renderers[selection.View][selection.Format]; !ok {
		return selection, fmt.Errorf("view %q does not declare format %q (available formats: %s)",
			selection.View, selection.Format, listFormats(viewSet, selection.View))
	}
	return selection, nil
}

// Map iteration order in Go is non-deterministic. Sort the output so error
// messages are stable across runs — anything else flakes tests that match
// against the message string.
func listViewNames(vs compact.ViewSet) string {
	names := make([]string, 0, len(vs.Views))
	for v := range vs.Views {
		names = append(names, string(v))
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func listFormats(vs compact.ViewSet, view compact.ViewName) string {
	formats := make([]string, 0)
	for f := range vs.Renderers[view] {
		formats = append(formats, string(f))
	}
	sort.Strings(formats)
	return strings.Join(formats, ", ")
}

// appendMoreHint adds a `_more` envelope listing alternate views the LLM
// could ask for. The hint teaches by response, not by tool description:
// the LLM learns the affordance from the bytes it just received.
//
// JSON-only for v1. Markdown responses are returned unchanged — appending
// a JSON `_more` envelope to markdown would pollute the format. A markdown
// footer hint is a future addition; for now the markdown response is what
// the LLM asked for, no metadata.
//
// Object root: `_more` is added at root as a sibling key.
// Array root: wrapped into `{"data": [...], "_more": {...}}`. This shape
// change for array-returning tools is the cost of discoverability.
// No alternates available: returns out unchanged (no envelope).
func appendMoreHint(out []byte, viewSet compact.ViewSet, selection compact.ViewSelection) []byte {
	if selection.Format != compact.FormatJSON {
		return out
	}

	alternates := make(map[string]string, len(viewSet.Views))
	for vn, vc := range viewSet.Views {
		if vn == selection.View {
			continue
		}
		alternates[string(vn)] = vc.Hint
	}
	if len(alternates) == 0 {
		return out
	}

	var parsed any
	if err := json.Unmarshal(out, &parsed); err != nil {
		// Can't augment what we can't parse; preserve the original.
		return out
	}

	more := map[string]any{"views": alternates}

	var augmented any
	switch v := parsed.(type) {
	case map[string]any:
		v["_more"] = more
		augmented = v
	case []any:
		augmented = map[string]any{"data": v, "_more": more}
	default:
		// Scalar root — leave alone.
		return out
	}

	result, err := json.Marshal(augmented)
	if err != nil {
		slog.Warn("appendMoreHint: marshal failed; returning unaugmented", "err", err)
		return out
	}
	return result
}

// viewErrorEnvelope structures a view-dispatch failure as JSON the LLM
// can read and recover from. Distinct from response_too_large; this
// signals contract mismatch (bad view/format), not size overrun.
func viewErrorEnvelope(toolName mcp.ToolName, err error) string {
	envelope := map[string]any{
		"error":   "view_dispatch_failed",
		"tool":    string(toolName),
		"message": err.Error(),
	}
	out, mErr := json.Marshal(envelope)
	if mErr != nil {
		slog.Error("viewErrorEnvelope: marshal failed", "tool", toolName, "err", mErr)
		return fmt.Sprintf(`{"error":"view_dispatch_failed","tool":%q,"message":%q}`, toolName, err.Error())
	}
	return string(out)
}

// tooLargeEnvelope returns a structured JSON error replacing an oversized response.
// LLM-observable; preserves parseability where a raw truncation would corrupt JSON.
func tooLargeEnvelope(toolName mcp.ToolName, size, limit int) string {
	slog.Warn("processResult: response exceeded per-tool max_bytes",
		"tool", toolName, "size", size, "limit", limit)
	envelope := map[string]any{
		"error": "response_too_large",
		"tool":  string(toolName),
		"size":  size,
		"limit": limit,
		"hint":  prompts.ResponseTooLargeHint(prompts.Context{}),
	}
	out, err := json.Marshal(envelope)
	if err != nil {
		// Should be impossible given the static shape; preserve current data on the off-chance.
		slog.Error("processResult: envelope marshal failed", "tool", toolName, "err", err)
		return fmt.Sprintf(`{"error":"response_too_large","tool":%q,"size":%d,"limit":%d}`, toolName, size, limit)
	}
	return string(out)
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

	if err := validateArgs(toolDef, args, reservedArgsFor(integration, toolName)); err != nil {
		return nil, &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
	}

	cb := s.getBreaker(integration.Name())
	if !cb.allow() {
		if s.services.Metrics != nil {
			s.services.Metrics.RecordCircuitBreak(mcp.IntegrationName(integration.Name()))
		}
		return nil, &mcp.ToolResult{
			Data:    prompts.CircuitBreaker(prompts.Context{}, integration.Name(), int(s.breakerCooldown.Seconds())),
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

// reservedArgsFor returns the argument names that bypass the tool's
// declared schema for this (integration, tool) pair. View-aware tools
// reserve "view" and "format" so they reach parseViewSelection instead
// of failing the validator as unknown parameters. Returns nil unless the
// integration both implements compact.ToolViewsIntegration and reports a
// ViewSet for the named tool — non-view tools that receive a stray view
// arg still get a clear "unknown parameter" error.
func reservedArgsFor(integration mcp.Integration, toolName mcp.ToolName) []string {
	vi, ok := integration.(compact.ToolViewsIntegration)
	if !ok {
		return nil
	}
	if _, hasViews := vi.Views(toolName); !hasViews {
		return nil
	}
	return compact.ReservedArgs()
}

// validateArgs checks that all required parameters are present and all provided
// parameters are declared in the tool's schema. Returns a descriptive error
// naming the offending parameter and suggesting corrections for typos.
//
// allowedExtras names additional argument keys that are valid for this tool
// even though they aren't declared in tool.Parameters. View-aware tools pass
// compact.ReservedArgs() via reservedArgsFor so view/format reach
// parseViewSelection instead of failing as unknown parameters.
func validateArgs(tool mcp.ToolDefinition, args map[string]any, allowedExtras []string) error {
	paramByName := make(map[string]mcp.Parameter, len(tool.Parameters))
	for _, p := range tool.Parameters {
		paramByName[string(p.Name)] = p
	}
	for _, p := range tool.Parameters {
		if !p.Required {
			continue
		}
		if _, ok := args[string(p.Name)]; !ok {
			return fmt.Errorf("missing required parameter %q for tool %q", string(p.Name), tool.Name)
		}
	}
	if len(tool.Parameters) == 0 {
		return nil // no schema declared — skip unknown-param check
	}
	extras := make(map[string]struct{}, len(allowedExtras))
	for _, k := range allowedExtras {
		extras[k] = struct{}{}
	}
	for key := range args {
		if _, ok := paramByName[key]; ok {
			continue
		}
		if _, ok := extras[key]; ok {
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
func closestParam(key string, params []mcp.Parameter) string {
	best := ""
	bestDist := len(key)/2 + 1 // threshold: half the key length
	for _, p := range params {
		name := string(p.Name)
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
func paramNames(params []mcp.Parameter) []string {
	names := make([]string, 0, len(params))
	for _, p := range params {
		names = append(names, string(p.Name))
	}
	slices.Sort(names)
	return names
}

// toolExecutor bridges the script.Executor interface to the server's tool dispatch.
type toolExecutor struct {
	server *Server
}

// checkMetaTool rejects meta-tools from script calls.
func (te *toolExecutor) checkMetaTool(toolName mcp.ToolName) *mcp.ToolResult {
	if toolName == "search" || toolName == "execute" || toolName == "session" || toolName == "history" || toolName == "pin" {
		return &mcp.ToolResult{
			Data: fmt.Sprintf(
				"tool %q is a meta-tool and cannot be called from scripts. "+
					"Use the search MCP tool before writing a script to discover tool names.",
				toolName),
			IsError: true,
		}
	}
	return nil
}

func (te *toolExecutor) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	if r := te.checkMetaTool(toolName); r != nil {
		return r, nil
	}
	// Integration is discarded: script-path calls intentionally skip per-tool
	// compaction so scripts can access all fields by name before projecting.
	_, result, err := te.server.executeTool(ctx, toolName, args)
	return result, err
}

// ExecuteRendered applies the same markdown/compaction/columnarization pipeline
// as the execute meta-tool, returning LLM-readable output. Used by api.callRendered().
func (te *toolExecutor) ExecuteRendered(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	if r := te.checkMetaTool(toolName); r != nil {
		return r, nil
	}
	integration, result, err := te.server.executeTool(ctx, toolName, args)
	if err != nil {
		return nil, err
	}
	applyResultProcessing(integration, toolName, compact.ParseViewArgs(args), result, te.server.services.Metrics)
	return result, nil
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

// StatelessHandler returns an http.Handler that serves MCP over streamable HTTP
// transport in stateless mode. Every request is handled independently with no
// session state retained across calls, which makes it safe to deploy behind a
// load balancer with multiple replicas (no session affinity required).
func (s *Server) StatelessHandler() http.Handler {
	return mcpsdk.NewStreamableHTTPHandler(
		func(r *http.Request) *mcpsdk.Server {
			return s.mcpServer
		},
		&mcpsdk.StreamableHTTPOptions{
			Stateless: true,
			Logger:    slog.Default(),
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
