# Stateful Features for Switchboard (Open Source)

Implementation plan for six stateful features derived from LLM usage benchmarking.
All features target the open-source project; the hosted SaaS can extend them (e.g. org-wide workflow storage).

---

## Priority Order

| # | Feature | Impact | Effort | Why this order |
|---|---------|--------|--------|----------------|
| 1 | Session Context / Working Set | Highest | Medium | Reduces token waste on *every* multi-call interaction |
| 2 | Breadcrumb Trail / Audit Log | High | Low-Med | Natural extension of session — unlocks LLM context recovery |
| 3 | Result Pinning / References | High | Medium | Builds on session + audit log; enables chained calls without re-fetch |
| 4 | Dry-Run Mode | High | Medium | Independent of session; unblocks LLM willingness to act |
| 5 | Favorites / Bookmarks (Workflows) | Medium | Medium | Builds on all above; most useful once sessions exist |
| 6 | Cross-Integration Joins | Medium | High | Most complex; scripts already cover 80% of this use case |

---

## Feature 1: Session Context / Working Set

### Problem
Every `execute` call is stateless. The LLM re-specifies `owner/repo`, `project_id`, `team_id` on every call.
`shared_parameters` in search deduplicates parameter *descriptions* but doesn't inject *values*.

### Current State
- `Server` struct holds no per-session state — same `*mcpsdk.Server` returned for all requests.
- `ProjectRouter` sets `Stateless: true` on its `StreamableHTTPHandler`.
- `project.ScopeRule.Defaults` already injects default arg values per-tool-pattern at the project level (`project/resolve.go`), proving the pattern works.
- Stdio transport is inherently single-session (one process = one client).
- The go-sdk's `StreamableHTTPHandler` maintains `sessions map[string]*sessionInfo` keyed by `Mcp-Session-Id` header when stateful mode is enabled.

### Design

#### Core Type
```go
// server/session.go (new file)
type Session struct {
    ID        string
    Context   map[string]any    // working set: "owner" → "daltoniam", "repo" → "switchboard"
    CreatedAt time.Time
    LastUsed  time.Time
    mu        sync.RWMutex
}

type SessionStore struct {
    mu       sync.RWMutex
    sessions map[string]*Session  // session ID → session
    ttl      time.Duration        // default 1h, configurable
}
```

#### New Meta-Tool: `session`
Expose a third meta-tool alongside `search`/`execute`:

```
session — Manage session-scoped context. 
  Actions: "set" (upsert key-value pairs), "get" (read current context), "clear" (reset).
  Parameters:
    action: "set" | "get" | "clear"
    context: map of key→value pairs (for "set")
  Returns: current session context after mutation.
```

LLM sets context once: `session({action: "set", context: {owner: "daltoniam", repo: "switchboard"}})`.
All subsequent `execute` calls auto-merge session context as default args (lowest priority — explicit args override).

#### Injection Point
In `server.executeTool()`, after `findTool()` and before `validateArgs()`:

```go
// Merge session defaults (lowest priority)
if sess := s.sessionFromCtx(ctx); sess != nil {
    for k, v := range sess.Context {
        if _, exists := args[k]; !exists {
            args[k] = v
        }
    }
}
```

#### Transport Binding
- **HTTP**: Enable stateful mode on `StreamableHTTPHandler` (remove `Stateless: true`). Use `Mcp-Session-Id` header from go-sdk. Map SDK session ID → our `Session`.
- **Stdio**: Single implicit session (keyed by a constant ID like `"stdio"`). Created on first request, lives for process lifetime.
- **ProjectRouter**: Session context merges *after* project defaults, so project defaults take precedence over session context but explicit args override both.

#### Implementation Steps
1. [ ] Create `server/session.go` — `Session`, `SessionStore` with TTL-based expiry (background goroutine or lazy eviction)
2. [ ] Add `sessionStore` field to `Server` struct
3. [ ] Register `session` meta-tool in `registerTools()`
4. [ ] Implement `handleSession()` handler (set/get/clear)
5. [ ] Add session context injection in `executeTool()` before `validateArgs()`
6. [ ] Wire session ID extraction for HTTP (from SDK session) and stdio (constant ID)
7. [ ] Add `SessionTTL` option to `server.Option` set
8. [ ] Tests: session CRUD, arg merge precedence (explicit > project defaults > session context), TTL expiry, concurrent access
9. [ ] Update `search` response to include active session context so LLMs know what's already set

---

## Feature 2: Breadcrumb Trail / Audit Log

### Problem
When the LLM's context window compresses mid-conversation, it loses track of what it already fetched.
Re-executing calls wastes tokens, API quota, and time.

### Current State
- `Metrics` in `metrics.go` tracks aggregate counts/latencies per tool — no per-call history.
- No persistent or session-scoped call log exists.

### Design

#### Core Type
```go
// server/session.go (extend)
type Breadcrumb struct {
    Seq       int        // monotonic sequence within session
    Timestamp time.Time
    Tool      ToolName
    Args      map[string]any     // what was passed
    Summary   string             // first 200 chars of result, or error message
    IsError   bool
    Handle    string             // "$1", "$2" — for result pinning (Feature 3)
}

// On Session struct:
type Session struct {
    // ...existing fields...
    Breadcrumbs []Breadcrumb   // append-only, capped at ~200 entries
    nextSeq     int
}
```

#### New Meta-Tool: `history`
```
history — Retrieve a compact log of tool calls made in this session.
  Parameters:
    last_n: number of recent entries (default 20, max 200)
    tool:   filter by tool name (optional)
  Returns: [{seq, tool, args_summary, result_summary, timestamp}]
```

The response is compacted — args are key-only (no values) unless small, results are truncated to first 200 chars.
This gives the LLM a "table of contents" for what it already did.

#### Recording Point
In `executeTool()`, after the retry loop returns (success or final failure), append a `Breadcrumb` to the session.
For scripts, record one breadcrumb per `api.call()` within the script + one for the script itself.

#### Implementation Steps
1. [ ] Add `Breadcrumb` type and breadcrumb slice to `Session`
2. [ ] Record breadcrumbs in `executeTool()` (single tool path) and `toolExecutor.Execute()` (script path)
3. [ ] Register `history` meta-tool, implement `handleHistory()`
4. [ ] Add summary truncation helper (`truncateResult(data string, maxLen int) string`)
5. [ ] Cap breadcrumb list at 200 entries (FIFO drop oldest)
6. [ ] Tests: breadcrumb recording, history retrieval with filters, cap enforcement
7. [ ] Columnarize history output for token efficiency

---

## Feature 3: Result Pinning / References

### Problem
LLMs chain calls: "get PR → get its comments → get linked issue." Each intermediate result disappears.
If results had session-scoped handles, scripts and follow-up calls could reference them.

### Current State
- Script engine keeps intermediate results in goja VM heap — but only within a single script execution.
- No cross-call result persistence.

### Design

#### Extension to Session
```go
// server/session.go (extend)
type PinnedResult struct {
    Handle    string         // "$1", "$2", etc.
    Tool      ToolName
    Data      json.RawMessage  // raw result (compacted)
    PinnedAt  time.Time
    SizeBytes int
}

type Session struct {
    // ...existing fields...
    Pinned     map[string]*PinnedResult  // handle → result
    nextHandle int
    pinnedSize int                        // total bytes, capped at 5MB
}
```

#### How It Works
1. Every `execute` result is auto-assigned a handle (`$1`, `$2`, ...) and stored in the session.
2. The handle is returned in the execute response metadata: `{"handle": "$3", "data": {...}}`.
3. In subsequent calls, args can reference handles: `{"issue_id": "$3.number"}` — the server resolves `$3` from the session and extracts `.number` via JSON path.
4. In scripts, a `api.ref("$3")` function returns the pinned result without re-fetching.
5. Memory cap: 5MB total pinned data per session. LRU eviction when exceeded. LLM can also explicitly unpin.

#### New Meta-Tool: `pin`
```
pin — Manage pinned results from previous calls.
  Actions: "list" (show all handles), "get" (retrieve a specific result), "unpin" (free memory).
  Parameters:
    action:  "list" | "get" | "unpin"
    handle:  "$N" (for get/unpin)
    path:    optional JSON path to extract a sub-field (for get)
```

#### Reference Resolution
In `executeTool()`, before `validateArgs()`, walk the args map:
```go
for k, v := range args {
    if s, ok := v.(string); ok && strings.HasPrefix(s, "$") {
        resolved, err := sess.ResolveRef(s)
        if err == nil {
            args[k] = resolved
        }
    }
}
```

#### Implementation Steps
1. [ ] Add `PinnedResult` type and pinned map to `Session`
2. [ ] Auto-pin results in `executeTool()` after successful execution
3. [ ] Add handle to execute response envelope
4. [ ] Implement reference resolution in arg processing (with JSON path support)
5. [ ] Add `api.ref()` function to script engine VM
6. [ ] Register `pin` meta-tool, implement `handlePin()`
7. [ ] Add memory cap + LRU eviction
8. [ ] Tests: pin/unpin, reference resolution with paths, memory cap, script api.ref()

---

## Feature 4: Dry-Run Mode

### Problem
Mutations (create issue, merge PR, post comment) are irreversible. LLMs hedge and ask for confirmation.
A `dry_run: true` flag would make LLMs more willing to act by showing what *would* happen.

### Current State
- No mutation metadata on `ToolDefinition` — mutation vs. read is convention-based (name prefix).
- Compact spec tests in each adapter already classify tools as mutation/read by name prefix.
- GitHub API supports preview headers (`Accept: application/vnd.github.v3+json` with preview features).
- AWS Lambda supports `DryRun` invocation type (already plumbed through).
- Most other APIs (Linear, Notion, Slack) have no dry-run support.

### Design

#### Tiered Approach
Three tiers based on upstream API support:

| Tier | Behavior | Examples |
|------|----------|---------|
| **Native** | Pass `dry_run` to upstream API | AWS Lambda, GitHub (some endpoints) |
| **Simulated** | Validate args + return the request payload that *would* be sent | Linear, Notion, Slack mutations |
| **Unsupported** | Return error: "dry-run not available for this tool" | Complex multi-step operations |

#### New Optional Interface
```go
// mcp.go
type DryRunIntegration interface {
    // DryRun returns what would happen if Execute were called.
    // Returns (preview, true) for supported tools, (nil, false) otherwise.
    DryRun(ctx context.Context, toolName ToolName, args map[string]any) (*ToolResult, bool)
}
```

#### Framework-Level Simulated Dry-Run
For integrations that don't implement `DryRunIntegration`, the server can simulate it:

1. Validate all args (same as real execution).
2. Return a result like:
```json
{
    "dry_run": true,
    "tool": "linear_create_issue",
    "validated_args": {"title": "Fix auth bug", "team_id": "TEAM-123"},
    "would_call": "POST https://api.linear.app/graphql",
    "note": "Simulated — this tool does not support native dry-run. Args are valid."
}
```

This is less useful than native dry-run but still confirms the call *would* succeed (valid args, healthy integration, circuit breaker open, etc.).

#### Arg Addition
Add `dry_run` as a **reserved parameter** on the `execute` meta-tool (not per-integration):

```go
// In handleExecute:
if dryRun, _ := mcp.ArgBool(args, "dry_run"); dryRun {
    return s.handleDryRun(ctx, toolName, args)
}
```

#### ToolDefinition Metadata (Optional Enhancement)
Add an optional `Annotations` field to `ToolDefinition` to expose mutation classification:
```go
type ToolDefinition struct {
    // ...existing fields...
    Annotations map[string]any `json:"annotations,omitempty"` // e.g., {"readOnlyHint": false, "destructiveHint": true}
}
```
This aligns with the MCP spec's `ToolAnnotations` concept and lets LLMs know which tools are mutations *before* calling them.

#### Implementation Steps
1. [ ] Add `Annotations` field to `ToolDefinition` (non-breaking, `omitempty`)
2. [ ] Define `DryRunIntegration` optional interface in `mcp.go`
3. [ ] Add `dry_run` parameter to `execute` meta-tool definition
4. [ ] Implement `handleDryRun()` in server — checks `DryRunIntegration` first, falls back to simulated
5. [ ] Implement simulated dry-run: arg validation + request preview
6. [ ] Implement native dry-run in GitHub adapter (use preview headers where supported)
7. [ ] Implement native dry-run in AWS adapter (already has `DryRun` invocation type)
8. [ ] Populate `Annotations` on mutation tools across adapters (can be incremental)
9. [ ] Tests: native dry-run, simulated dry-run, non-mutation tools (should just execute normally)

---

## Feature 5: Favorites / Bookmarks (Workflows)

### Problem
LLMs doing repeated workflows (standup, PR triage, incident response) re-discover the same tools every session.
A saved "workflow" — a named sequence of tool calls with template parameters — would skip the search-then-chain dance.

### Current State
- No workflow concept exists in Switchboard.
- `project.ScopeRule.Defaults` proves the pattern of stored arg defaults.
- Script engine already supports multi-step tool call sequences.
- The hosted SaaS will want to store these org-wide; the OSS version should use local file storage.

### Design

#### Core Type
```go
// workflow/workflow.go (new package)
type Workflow struct {
    Name        string            `json:"name"`
    Description string            `json:"description"`
    Steps       []Step            `json:"steps"`
    Parameters  map[string]string `json:"parameters"` // template params: "repo" → "Target repository"
    CreatedAt   time.Time         `json:"created_at"`
    UpdatedAt   time.Time         `json:"updated_at"`
}

type Step struct {
    Tool   ToolName       `json:"tool"`
    Args   map[string]any `json:"args"`   // can contain "{{repo}}" template refs
    Assign string         `json:"assign"` // variable name to store result
}
```

#### Storage
- OSS: `~/.config/switchboard/workflows/{name}.json` — same pattern as project store.
- Hosted: pluggable `WorkflowStore` interface (file-backed for OSS, API-backed for SaaS).

#### New Meta-Tool: `workflow`
```
workflow — Manage saved tool call sequences.
  Actions: "list", "get", "run", "create", "update", "delete".
  Parameters:
    action: "list" | "get" | "run" | "create" | "update" | "delete"
    name:   workflow name
    params: template parameter values (for "run")
    definition: Workflow JSON (for "create"/"update")
  
  "run" expands templates, executes steps sequentially (like a script),
  and returns the final result.
```

Running a workflow is essentially syntactic sugar over the script engine — the server generates a script from the workflow steps and feeds it to `script.Engine.Run()`.

#### LLM Workflow
1. LLM does a multi-step task manually (search → execute → execute → ...).
2. LLM (or user) calls `workflow({action: "create", name: "morning-standup", ...})` with the steps.
3. Next session: `workflow({action: "run", name: "morning-standup", params: {team: "platform"}})`.

#### Implementation Steps
1. [ ] Create `workflow/` package with `Workflow`, `Step`, `Store` types
2. [ ] Implement file-based `Store` (load/save/list/delete in `~/.config/switchboard/workflows/`)
3. [ ] Add `WorkflowStore` to `Services` struct (or as a server option)
4. [ ] Register `workflow` meta-tool, implement `handleWorkflow()`
5. [ ] Implement workflow-to-script compilation (template expansion → ES5 script source)
6. [ ] Wire workflow execution through existing `script.Engine`
7. [ ] Tests: CRUD, template expansion, execution, error handling mid-workflow
8. [ ] Add workflow suggestions: after history shows repeated patterns, the server could hint "save as workflow?"

---

## Feature 6: Cross-Integration Joins

### Problem
Getting "GitHub PRs enriched with their linked Linear tickets" requires the LLM to write a script loop.
A declarative join could do this server-side.

### Current State
- Script engine handles cross-integration calls — LLMs write `api.call()` loops.
- No field-level linking metadata between integrations.
- Common patterns: GitHub PR body mentions `LIN-123`, Sentry issue links to GitHub commit, Slack message links to Linear ticket.

### Design

#### Link Registry
```go
// server/joins.go (new file)
type LinkPattern struct {
    Source      string   // "github"
    SourceTool  ToolName // "github_get_pull"
    SourceField string   // "body"
    Pattern     string   // `LIN-\d+` (regex to extract linked identifier)
    Target      string   // "linear"
    TargetTool  ToolName // "linear_get_issue"
    TargetArg   string   // "identifier"
}
```

A curated set of `LinkPattern` definitions encoding known cross-integration references.

#### Enrichment Parameter
Add an `enrich` parameter to `execute`:

```json
{
  "tool_name": "github_list_pulls",
  "arguments": {"owner": "daltoniam", "repo": "switchboard", "state": "open"},
  "enrich": ["linear"]
}
```

The server:
1. Executes the primary tool.
2. Scans results for matching `LinkPattern`s targeting the requested integrations.
3. Fans out `api.call()` for each matched link.
4. Merges enrichment data back into the result under an `_enriched` key.

#### Implementation Steps
1. [ ] Define `LinkPattern` type and curate initial set (GitHub↔Linear, GitHub↔Sentry, Slack↔Linear)
2. [ ] Add `enrich` parameter to `execute` meta-tool
3. [ ] Implement `enrichResult()` — pattern scan + fan-out + merge
4. [ ] Use `errgroup` for concurrent enrichment calls with timeout
5. [ ] Add enrichment to response: `{"data": {...}, "_enriched": {"linear": [...]}}`
6. [ ] Tests: pattern matching, fan-out, timeout handling, no-match passthrough
7. [ ] Consider making link patterns configurable (user can add custom patterns)

---

## Implementation Sequencing

```
Phase 1 — Foundation (Features 1 + 2)
├── Session infrastructure (store, TTL, transport binding)
├── Session context tool + arg injection
├── Breadcrumb recording + history tool
└── Tests for all session lifecycle

Phase 2 — Result Memory (Feature 3)
├── Result pinning + handle assignment
├── Reference resolution in args
├── api.ref() in script engine
└── pin meta-tool

Phase 3 — Safety (Feature 4)  [can parallelize with Phase 2]
├── DryRunIntegration interface
├── Simulated dry-run fallback
├── ToolDefinition.Annotations
└── Native dry-run for GitHub + AWS

Phase 4 — Automation (Feature 5)
├── Workflow package + file store
├── Template expansion + script compilation
├── workflow meta-tool
└── Integration with session context for template defaults

Phase 5 — Intelligence (Feature 6)
├── Link pattern registry
├── Enrichment fan-out engine
├── enrich parameter on execute
└── Initial link patterns (GitHub↔Linear, GitHub↔Sentry)
```

### Estimated Effort

| Phase | Features | Est. Days | Dependencies |
|-------|----------|-----------|--------------|
| 1 | Session + Audit Log | 3-4 | None |
| 2 | Result Pinning | 2-3 | Phase 1 |
| 3 | Dry-Run Mode | 2-3 | None (parallelizable) |
| 4 | Workflows | 3-4 | Phase 1 (for template defaults) |
| 5 | Cross-Integration Joins | 4-5 | None (but most useful after Phase 1) |

**Total: ~15-19 days of focused development**

---

## Open Questions

1. **Session persistence**: Should sessions survive server restarts? (In-memory is simpler but stdio sessions would lose context on restart. File-backed or SQLite would persist.)
2. **Multi-tenant sessions**: For the hosted SaaS, sessions need to be scoped per-user/org. Should the OSS design accommodate a pluggable `SessionStore` interface from the start?
3. **Workflow sharing**: In the hosted version, workflows are org-wide. Should the OSS file format be designed for easy import/export?
4. **Enrichment depth**: Should cross-integration joins be recursive? (PR → Linear issue → Linear project?) Or always single-hop?
5. **Session context in search**: Should `search` results be re-ranked based on session context? (e.g., if session has `owner=daltoniam`, boost GitHub tools.)
