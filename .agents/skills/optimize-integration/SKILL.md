---
name: optimize-integration
description: >
  Improve an existing Switchboard integration adapter's LLM usability — tool
  description enrichment, field compaction refinement, and response tuning.
  Use when: "optimize integration", "improve tool descriptions", "extend
  compaction", "make integration better for LLMs", after user story mapping,
  or when an LLM is making wrong tool choices or passing wrong IDs.
  Not for adding new integrations (use add-integration) or fixing bugs.
metadata:
  author: switchboard
  version: "1.0"
---

# Optimize Integration

Improve an existing adapter's LLM usability. Three phases, each independent — run whichever applies.

See `AGENTS.md` for interface contracts, project structure, and conventions.

## Prerequisites

Before optimizing, you need signal on what to optimize. Ideal inputs:

- **User story mapping**: Which tools cover which user stories? Which tools are entry points?
- **LLM error patterns**: Wrong tool selected, wrong ID passed, unnecessary drill-downs
- **Token profiling**: Which tools produce oversized responses?

If none exist, start by mapping the top 10 user stories to tools — this reveals the high-value tools and the common mistakes.

## Phase 1: Tool Description Enrichment

Tool descriptions are the only context an LLM gets for tool selection. The goal is correct routing on first attempt.

### Step 1: Tier the tools

Classify every tool into tiers based on user story coverage:

| Tier | Criteria | Description treatment |
|------|----------|-----------------------|
| 1 | Covers 80%+ of user stories (typically 3-5 tools) | Workflow routing: "Start here", "returns the data needed by X" |
| 2 | Used in multi-step workflows but not entry points | Chaining hints: "Use before X to understand the schema" |
| 3 | Primitives subsumed by higher-level tools | Prefer-over hints: "For full detail, prefer X" |
| 4 | Identity/utility tools with clear names | Keep as-is |

### Step 2: Enrich descriptions

For each tool, apply these patterns where applicable:

- **Workflow entry points** (Tier 1): Tell the LLM this is where to start and what to chain next
- **Prefer-over hints** (Tier 3): Explicitly name the better alternative — LLMs won't infer this
- **Gotcha prevention**: Surface ID confusion, parameter constraints, and common mistakes directly in the description AND parameter strings
- **Mutual exclusion**: When two parameters are alternatives, say which to use when and which to omit

Keep descriptions to 1-3 sentences. No paragraphs.

**Example — before:**
```
"List issues for a repository"
```

**Example — after:**
```
"List issues for a repository. Returns open issues by default. For searching across repos, prefer search_issues."
```

### Step 3: Enrich parameter descriptions

Focus on parameters where the wrong value is a common mistake:

- ID parameters that accept a specific ID type (not the obvious one)
- Parameters that are mutually exclusive with another parameter
- Parameters where a prerequisite tool provides the needed information

### Verification

- `make ci` passes (descriptions are data, not logic)
- Dispatch parity tests still pass
- `search({"query": "<integration>"})` returns tools with enriched descriptions

## Phase 2: Field Compaction Refinement

Extend or tune compaction specs to reduce token waste. The `add-integration` skill covers initial compaction setup — this phase covers refinement of an existing spec set.

### When to extend compaction

- **Single-record get tools** returning raw API records with noise fields (permissions, version, internal IDs) — these need specs just like list tools
- **Search results** missing routing fields needed for follow-up queries (e.g., an ID the LLM needs for the next call)
- **List tools** where the LLM consistently drills into every item (spec is missing a routing field)

### Handler vs compaction boundary

Handlers and compaction have distinct jobs. Mixing them causes specs and handlers to drift independently — field additions require changes in two places, and handler-level filtering is invisible to spec reviewers.

- **Handlers**: structural transformation only — unwrap API envelopes, merge split responses (e.g., search `results` + `recordMap`), resolve double-wrapped formats, tree-build from flat records
- **Compaction specs**: all noise/context reduction — strip internal fields, drop low-value records, whitelist routing fields

A handler that selects specific fields during a merge loop duplicates what the compaction spec already declares. A handler that filters record types for "relevance" makes a context decision that belongs in compaction. Both cause the same failure: the compaction spec becomes an incomplete description of the tool's output shape, and changes to what the LLM sees require editing two files instead of one.

### When NOT to extend compaction

- Mutation tools returning small confirmation objects
- Tools where the full response is already small (<1KB)

### Shared field slices pattern

When list and get tools return the same record type but with different noise profiles, use shared field slices with list/single variance:

```go
// List context: compact, just essentials for routing
var userFields = []string{"id", "name", "email"}

// Single-record context: more detail is acceptable (1 object, not N)
var singleUserFields = []string{"id", "name", "email", "profile_photo"}
```

How these get applied to specs depends on the adapter's `compact_specs.go` conventions. Check the existing adapter (or `integrations/github/compact_specs.go` as canonical reference).

### SDK vs raw HTTP response shapes

- **Typed SDK adapters** (GitHub, Datadog, AWS): SDK structs may include fields not populated by list endpoints (phantom fields). Check the actual API response, not just the SDK struct definition.
- **Raw HTTP adapters** (Notion, Sentry, Metabase): Raw JSON — what you see is what you compact. Watch for internal fields (version, space_id, CRDT data) that APIs return but SDKs would normally hide.
- **GraphQL adapters** (Linear): Response already shaped by the query — compaction may not be needed if the query already selects only needed fields.

### Nested object compaction

The compaction engine supports both array-element whitelisting (`items[].name`) and nested object whitelisting (`page.id`, `commit.message`). When 2+ specs share a root segment, the engine groups them into a nested output object:

```
specs: ["page.id", "page.type", "page.properties"]
input: {"page": {"id":"x", "type":"page", "properties":{...}, "crdt_data":"noise", "version":42}}
output: {"page": {"id":"x", "type":"page", "properties":{...}}}
```

A single spec with a dot (e.g., `user.login`) stays flat — `{"user.login": "alice"}` — preserving backward compatibility. Grouping only activates at 2+ members.

### Verification

- `TestFieldCompactionSpecs_NoOrphanSpecs` passes
- `TestFieldCompactionSpecs_NoMutationTools` passes (if present)
- Spot-check: execute a compacted tool, verify noise fields stripped and routing fields preserved

## Phase 3: Response Tuning

Adjust response size caps and HTTP client behavior for the adapter's actual response profile.

### Response cap sizing

Only applies to raw HTTP adapters that read response bodies directly. SDK adapters handle this internally.

Measure the largest real response the adapter produces (use the heaviest list/search tool with maximum page size). Set the cap at ~2x the observed maximum.

```go
const maxResponseSize = 512 << 10 // example: 512 KB for an adapter whose largest real response is ~230KB
```

### What to check

- **All adapters**: `http.Client.Timeout` set (prevents hanging on slow APIs)
- **Raw HTTP adapters**: `io.LimitReader` cap on response bodies (prevents unbounded reads)
- **Cookie/session-auth adapters** (Slack, Notion): `CheckRedirect` blocks redirects (prevents token leaking on 3xx)

## Execution Order

1. Phase 1 first — description enrichment is highest-impact, lowest-risk
2. Phase 2 second — compaction requires understanding the response shapes
3. Phase 3 last — response tuning requires measuring real responses
4. Run `make ci` after each phase
5. Commit each phase separately (descriptions, compaction, tuning)

## Columnar Format & Automatic Optimizations

These optimizations are applied automatically at the MCP response boundary — no per-adapter work needed. Understanding them helps when debugging response shapes or writing scripts.

### Columnar JSON

Arrays of 8+ objects in execute and search responses are reshaped from `[{k:v},{k:v}]` to `{"columns":[...],"rows":[[...],...],"constants":{...}}`. This eliminates per-record key repetition (28%+ savings at 8+ items).

- **Constant lifting**: Uniform columns move to `"constants"` map (e.g., filtered list where all items have `state:"open"`)
- **Density ordering**: Dense columns appear before sparse ones — LLMs see important data first
- **Threshold**: Only arrays of 8+ items are columnarized. Below 8, per-record format preserved

### Search shared parameter deduplication

Params with identical name+description across 3+ tools on the search page are extracted to `shared_parameters`. Common params like `owner`/`repo` appear once instead of N times.

### Script field projection

Scripts can project fields via third arg: `api.call(tool, args, {fields: ["id","title"]})`. Uses `CompactAny` under the hood. Use this in scripts that only need a few fields from large responses.

### Glob exclusion specs

Use `"-*_url"` to exclude all fields matching a glob pattern. Valid in exclusion mode only. **Caveat**: catches future fields — prefer targeted exclusions when the field set is small. Invalid patterns rejected at parse time.

## Anti-Patterns

| Mistake | Failure it causes | Correct approach |
|---------|-------------------|-----------------|
| Handler-level field whitelist in merge/transform loops | Spec and handler drift independently — field additions require two-file edits, reviewers miss the handler's hidden filter | Handler passes all fields through; compaction spec is the single source of truth for what the LLM sees |
| Handler-level record filtering for context reduction (e.g., skipping "noisy" record types) | Filtering logic invisible to spec reviewers; no way to audit what the LLM never sees without reading handler code | Handler passes all records through; if a record type is pure noise, add a compaction spec that strips its fields down to just `id` and `type` |
| Long paragraph descriptions | LLM skips or misinterprets; wastes context | 1-3 sentences max |
| Describing implementation details (internal endpoint names, SDK methods) | Leaks internals that confuse LLMs into constructing raw API calls | Describe behavior and value to the caller |
| Compaction specs on mutation tools | Spec maintenance cost with no token savings (mutations return small confirmations) | No spec needed |
| Same field set for list and get tools | List context wastes tokens on fields only useful in single-record context (N x noise) | Use shared slices with list/single variance |
| Adding every field "just in case" | Every field costs tokens x N items — unjustified fields compound across pagination | Justify each field by the query it enables |
| Enriching Tier 4 tools that are already clear | Description churn with no routing improvement | Don't touch what doesn't need touching |
| Broad glob exclusions like `-*_url` | Silently excludes future upstream API fields that match the glob | Use targeted exclusions when field set is small and stable |
| Aliasing `ToolDefinition.Parameters` map in search | Progressive silent corruption — `extractSharedParameters` deletes from shared map | Always deep-copy the Parameters map when building `searchToolInfo` |
