---
name: add-integration
description: >
  Use when adding a new external API integration to Switchboard, scaffolding an
  integration adapter, or deciding between SDK vs raw HTTP for a new service.
  Not for modifying existing integrations or fixing bugs in current adapters.
metadata:
  author: switchboard
  version: "1.0"
---

# Add Integration

Full lifecycle for adding a new integration adapter to Switchboard.
See `AGENTS.md` for interface contracts, project structure, and conventions referenced below.

## 1. Research the Target API

Before writing code, answer these questions:

- [ ] **Auth model**: API key, OAuth (which grant type?), session tokens, other?
- [ ] **API shape**: REST, GraphQL, or mixed?
- [ ] **Go SDK**: Does a maintained, well-typed Go SDK exist? Does it cover the endpoints you need?
- [ ] **Rate limits**: Documented? Per-endpoint or global? Headers for remaining quota?
- [ ] **Pagination**: Cursor-based, offset, or link-header? Consistent across endpoints?
- [ ] **Error format**: Structured JSON errors or plain text? Status code conventions?
- [ ] **Scope**: Which API resources/operations are needed? Group by domain (e.g., issues, projects, users)
- [ ] **Parent types**: Do entities have polymorphic parent relationships? Do different parent types require different mutation commands? (e.g., Notion block parents use `listAfter`, collection parents use `setParent`)

**Undocumented APIs**: When working with internal or undocumented APIs (no OpenAPI spec, no public docs), intercept the first-party client's network traffic to discover expected payload shapes. For web apps: monkey-patch `window.fetch` in Chrome DevTools to log request bodies. For mobile apps: use a MITM proxy. This is often the only way to discover commands, required fields, and transaction formats.

## 2. Design Decisions

### SDK vs Raw HTTP

| Criteria | Use Typed SDK | Use Raw HTTP |
|----------|--------------|--------------|
| Go SDK exists and maintained | Yes | - |
| SDK covers needed endpoints | Yes | - |
| No Go SDK available | - | Yes |
| SDK exists but poorly typed or incomplete | - | Yes |
| API is GraphQL | - | Yes (hand-rolled queries) |

**Existing precedent**: GitHub, Datadog, Slack use typed SDKs. Linear, Sentry, Metabase use raw HTTP.

### Tool Granularity and File Organization

One tool per API operation. Follow naming and dispatch conventions in `AGENTS.md > Conventions and Patterns`.

| Tool count | Structure |
|-----------|-----------|
| < 30 | 1-2 handler files + `tools.go` + `<name>.go` + `<name>_test.go` |
| 30-60 | 3-5 handler files split by domain (see `sentry/`) |
| 60+ | 5+ handler files (see `github/`, `datadog/`) |

### Tool Description Quality (Search Discoverability)

Tool descriptions are scored by a TF-IDF search engine with synonym expansion.
They're the ONLY text an LLM sees when deciding which tool to use. Write them
for discoverability, not just accuracy.

**Three-tier pattern** (from the GitHub adapter — the gold standard):

1. **Entry points** — tools users search for first:
   ```
   "List error and exception issues for a project. Start here for error tracking, debugging, and finding unresolved bugs or crashes."
   ```

2. **Drill-down tools** — used after entry points:
   ```
   "Get details of a specific error issue, including stacktrace and debugging context. Use after list_issues."
   ```

3. **Action tools** — mutations with chaining hints:
   ```
   "Update an error issue (resolve, assign, triage). Use after list_issues or get_issue."
   ```

**Rules:**

- **Include domain keywords users would search for.** "List issues" is invisible to someone searching "find bugs" or "error tracking". Add the words: "List error and exception issues... bugs, crashes, unresolved problems."
- **Include synonym-group words in descriptions.** The search engine expands "ticket" to {"ticket","issue","task","bug"}, but the tool still needs at least one variant in its description to score. Don't rely on synonyms alone.
- **Scope descriptions to prevent false positives.** HomeAssistant tools should say "smart home" not just "state". CI/CD tools should say "CI/CD pipeline" not just "run" or "logs". Generic words cause tools to surface as noise in unrelated queries.
- **Add workflow hints.** "Start here for..." on entry points. "Use after X..." on drill-down tools. "Preferred over X because..." when tools overlap.
- **Don't pad with stop words.** Words like "a", "the", "to", "for" are filtered by the search engine. Every word in the description should carry meaning.
- **Include both singular and plural forms.** The search engine does exact token matching with NO stemming. "errors" ≠ "error". If users might search for either form, include both: "List errors and exceptions" covers both "sentry errors" and "sentry error". Check the synonym groups in `server/search.go` — common plurals are covered there, but new domain words need explicit plural coverage either in the description or as a synonym group.

**Anti-patterns:**
- `"List issues for a project"` — too generic, no domain keywords
- `"Get a specific message by ID"` — no "email", "mail", or "read"
- `"List events with optional filters"` — what kind of events? For what purpose?

**Verify with benchmark:** After adding tools, run `/search-benchmark` to check
that your tools surface for natural-language queries users would actually type.

### Auth Pattern

| Auth type | Pattern | Example adapter |
|-----------|---------|-----------------|
| API key / token | Header in `doRequest` | `metabase/` (`x-api-key`), `sentry/` (`Bearer`) |
| OAuth token via SDK | SDK transport/config | `github/` (`oauth2`), `datadog/` (context keys) |
| Session token + cookie | Custom `http.RoundTripper` | `slack/` (`cookieTransport`) |
| OAuth setup flow | Separate `oauth.go` file | `github/`, `linear/`, `sentry/`, `slack/` |

Add an OAuth flow when the API supports it *and* you want guided credential setup in the Web UI. Get basic token auth working first. Grant type depends on the API: Device Flow for headless, PKCE for browser-redirect. Add a corresponding setup page in `web/templates/pages/`.

## 3. Implementation

Reference `AGENTS.md > Adding a New Integration` for the 7-step mechanical checklist.
Focus here on judgment calls:

### Configure as Defensive Validation Boundary

`Configure()` is where you reject invalid state. Validate eagerly, fail on missing credentials — never let an unconfigured adapter reach `Execute()`.

```go
func (x *myapi) Configure(creds mcp.Credentials) error {
    x.apiKey = creds["api_key"]
    if x.apiKey == "" {
        return fmt.Errorf("myapi: api_key is required")
    }
    // For services with a fixed base URL, hardcode a default (see sentry/)
    // For services where URL varies, require it (see metabase/)
    if v := creds["base_url"]; v != "" {
        x.baseURL = strings.TrimRight(v, "/")
    }
    return nil
}
```

### Healthy() Check

Implement a lightweight API call that verifies credentials work (e.g., "get current user" or "list with limit=1"). Must handle the case where `Configure()` hasn't been called yet (nil client) — return `false`, don't panic.

### Error Handling

Follow `AGENTS.md > Error Handling`. Key judgment: surface errors to the caller — never swallow them, never add fallback defaults.

### When to Add Custom Helpers

Add integration-specific helpers when a pattern repeats 3+ times *within* an adapter:
- Org/workspace slug injection (see `integrations/sentry/org()`)
- Entity ID resolution by name (see `integrations/linear/resolveTeamID()`)
- Query string building from optional params (see `integrations/sentry/queryEncode()`)

Note: arg helpers are **shared** from `args.go` — use `mcp.NewArgs(args)` reader for bulk extraction or standalone `mcp.ArgStr`/`mcp.ArgInt`/etc. for conditional fields. NEVER define local `argStr`/`argInt` in adapters. Use `r.OptInt("page", 1)` for pagination defaults. See [docs/go-anti-patterns.md](docs/go-anti-patterns.md) for extraction pitfalls. Result constructors (`mcp.JSONResult`, `mcp.RawResult`, `mcp.ErrResult`) are shared from the root package. Some adapters wrap `mcp.ErrResult` in a local `errResult` to inject retry semantics.

## 4. Testing Requirements

Every adapter must have these test categories (see existing `*_test.go` files):

- [ ] **Constructor**: `New()` returns valid integration, `Name()` matches
- [ ] **Configure success**: Valid credentials accepted
- [ ] **Configure failures**: One test per required credential, verifying error message
- [ ] **Tools metadata**: All have Name + Description, prefix matches `Name()`, no duplicates. Descriptions follow the three-tier pattern (see "Tool Description Quality" above)
- [ ] **Search discoverability**: Run `/search-benchmark` — new tools surface for natural-language queries. Check synonym coverage with existing `synonymGroups` in `server/search.go`
- [ ] **Dispatch parity (non-negotiable)**:
  - `TestDispatchMap_AllToolsCovered` — every `Tools()` entry has a dispatch handler
  - `TestDispatchMap_NoOrphanHandlers` — every dispatch key has a `ToolDefinition`
- [ ] **Execute unknown tool**: Returns `IsError: true`, `"unknown tool"` in Data
- [ ] **HTTP helpers**: `httptest.NewServer` for success, API errors (>=400), 204 no-content
- [ ] **Arg extraction**: Uses shared `mcp.NewArgs(args)` reader with `r.Err()` check — type coercion is tested in root `args_test.go`. `TestNewArgs_ErrCheckParity` automatically covers new adapters

## 5. Wiring and Verification

Follow `AGENTS.md > Adding a New Integration` steps 6-7 (register + config defaults), then verify:

1. `go build ./...` && `go test ./...` && `go vet ./...` && `go tool golangci-lint run`
2. Smoke test: start server, call `search` for new integration tools, `execute` one

## 6. Field Compaction

New adapters should implement `FieldCompactionIntegration` to keep list/search responses compact.

**Contract vs implementation**: The interface contract is `CompactSpec(toolName string) ([]CompactField, bool)` defined in `mcp.go`. How you build the specs is an implementation detail — `integrations/github/compact_specs.go` uses a raw string map parsed at init, but adapters can construct `CompactField` slices however they want.

### Token Budget Principle

Optimize specs for **fewest total tokens across the entire task workflow**, not smallest single response. A field that prevents an N+1 drill-down saves ~5KB per item even if it costs 50 bytes in the compacted list. Distribution of tokens across 1 or N calls doesn't matter as long as N is small enough that network latency doesn't dominate timing. The goal is a finite minimum token budget for any given workflow — get as close to it as possible.

**Example**: `requested_reviewers[].login` adds ~80 bytes per PR to the compacted list, but without it "which PRs need review?" requires a separate `list_requested_reviewers` call per PR (~3KB each). For 20 open PRs: +1.6KB in compacted list vs. +60KB in drill-down calls.

### Checklist

- [ ] Create `integrations/<name>/compact_specs.go` with `rawFieldCompactionSpecs` map and `mustBuildFieldCompactionSpecs` init (copy pattern from `integrations/github/compact_specs.go`)
- [ ] Design field compaction specs using the spec design questions below
- [ ] Add specs for all read tools (list, search, AND single-record get) — keep identifiers, names, states, dates, counts, URLs; drop nested full objects, permissions, avatars, node_ids, CRDT noise
- [ ] Implement `CompactSpec(toolName string) ([]CompactField, bool)` method on the adapter struct
- [ ] Add compile-time assertion: `var _ mcp.FieldCompactionIntegration = (*myapi)(nil)`
- [ ] Add `TestFieldCompactionSpecs_NoOrphanSpecs` — every spec key must exist in `dispatch`
- [ ] Unwrap SDK list responses to the inner slice (e.g., `resp.Items` not `resp`) so field compaction operates on the array directly
- [ ] Mutation tools (create/update/delete) should NOT have field compaction specs — return full confirmation responses

### Spec Design Questions

For each tool's spec, verify against these questions before finalizing:

1. **Routing sufficiency**: Can the LLM decide which item to drill into from the compacted list alone? If it must open every item to answer "which PR broke the build?", the spec is missing a field.
2. **Workflow gaps**: Trace common workflows (triage, review, debug CI). Does each workflow have the fields to complete without per-item get calls? Missing a field like `requested_reviewers` means "which PRs need review?" requires N extra calls.
3. **Dead weight**: Would you only look at this field after already deciding to open the full record? If yes, drop it — it's noise in a list context. Also watch for **phantom fields** — fields that exist in SDK structs but are only populated by Get endpoints (e.g., `additions`/`deletions` on GitHub's List PRs API return 0/null).
4. **Field dependencies**: Do included fields make sense alone? `status` without `conclusion` in CI runs is incomplete. `additions` without `deletions` in PRs is half the story. Include paired fields together or not at all.
5. **Follow-up keys**: Does the LLM have the identifiers it needs to make follow-up API calls? Verify that `id`, `number`, or `sha` — whatever the get tool requires — is included.

Canonical example: `integrations/github/compact_specs.go`

## Anti-Patterns

| Mistake | Correct approach |
|---------|-----------------|
| Defaulting missing credentials | Return error from `Configure()` |
| Returning Go error for API failures | Use `ToolResult{IsError: true}`, nil Go error |
| Skipping dispatch parity tests | Non-negotiable — tests catch tool/handler drift |
| Pre-building helpers before duplication | Wait for 3+ uses, then extract |
| Duplicating AGENTS.md content in handlers | Read AGENTS.md for conventions |
| Adding OAuth before basic auth works | Get token-based auth working first, add OAuth flow after |
| Returning full SDK wrapper for lists | Unwrap to inner slice, declare compaction specs |
| No default page size on list tools | Use sensible defaults (10-50 items) |
