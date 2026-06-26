# Prompts

`server/prompts/` owns all LLM-facing prose in this repo — meta-tool descriptions and
runtime-assembled messages. Everything in the package is authored in `.md.tmpl` files and
accessed through typed Go methods. This doc covers where files live, naming rules, body
conventions, and how to add new accessors.

---

## Where Prompts Live

Two directories, two lifetimes:

| Directory | Content | Rendered when |
|-----------|---------|---------------|
| `server/prompts/meta/` | Meta-tool descriptions: `search`, `execute`, `session`, `history`, `pin` | Once at server startup; passed as `Tool.Description` to the MCP SDK |
| `server/prompts/dynamic/` | Runtime messages: search-result summaries, circuit-breaker errors, response-too-large hints | Per-request, from dynamic wrappers in `wrappers.go` |

Meta descriptions are fixed at registration time. The MCP Go SDK registers tools globally at
boot and returns identical descriptions for every session — one server process, one variant.
(See [Variant story](#variant-story-operational-vs-per-session) for what changes this.)

---

## File and Accessor Naming

### Meta accessors

File: `server/prompts/meta/<accessor>.md.tmpl`
Go method: `Meta.Accessor()` (PascalCase, on `metaAccessors`)

| File | Method |
|------|--------|
| `meta/search.md.tmpl` | `prompts.Meta.Search()` |
| `meta/execute.md.tmpl` | `prompts.Meta.Execute()` |
| `meta/session.md.tmpl` | `prompts.Meta.Session()` |
| `meta/history.md.tmpl` | `prompts.Meta.History()` |
| `meta/pin.md.tmpl` | `prompts.Meta.Pin()` |

### Dynamic wrappers

File: `server/prompts/dynamic/<snake_case_name>.md.tmpl`
Go function: `SnakeCaseAsCamel(ctx Context, ...)` in `wrappers.go`

| File | Function |
|------|----------|
| `dynamic/search_summary.md.tmpl` | `SearchSummary(ctx, total, query)` |
| `dynamic/circuit_breaker.md.tmpl` | `CircuitBreaker(ctx, integration, cooldownSeconds)` |
| `dynamic/response_too_large_hint.md.tmpl` | `ResponseTooLargeHint(ctx)` |

### Why `.md.tmpl` everywhere

Every file is parsed by `text/template` at init — even files whose v1 body is pure markdown
with no `<% %>` directives. The extension signals this uniformly. When a future variant adds
a directive, callers see no change; there is no engine-swap moment.

---

## Template Body Conventions

### Line 1: trim-marker header comment

Every template starts with:

```
<%- /* filename.md.tmpl — v1, no variants */ -%>
```

The `-` trim markers on both sides strip surrounding whitespace. Without them the comment
emits a leading newline before the description body — visible as a stray blank line at the
top of every rendered description.

### Custom delimiters `<%` `%>`

Both parsers (`metaTmpl`, `dynamicTmpl`) use `<%` `%>` instead of the default `{{` `}}`.

Rationale: `execute.md.tmpl` contains nine `}}` sequences inside JSON examples (e.g.,
`{"arguments": {"owner": "x"}}`). With default delimiters the template parser panics at
init. Custom delimiters eliminate the collision permanently and apply to all files in both
directories — no per-file opt-in needed.

**Never use `{{` `}}` in any file under `server/prompts/`.**

### Trailing newline

Markdown files end with `\n` by editor default. The original backtick strings in `server.go`
did not. The `render()` helper in `embed.go` calls `strings.TrimRight(out, "\n")` so every
accessor returns a string that matches the Go literal equivalent byte-for-byte.

Do not manually strip the trailing newline from template files — `render()` handles it.

### Total failure mode

`template.Must` in `embed.go` wraps `ParseFS` for both parsers. If a template file is
missing or malformed, the process panics at **init**, not at the first call to an accessor.
This keeps the failure loud and immediate — a mis-named file surfaces in the first test run,
not in production on the first request that happens to call that accessor.

Every accessor always returns a `string`. The function signature is total. There is no
"returns an error on missing file" path — the seam is enforced at startup.

---

## Adding a New Meta-Tool Description

Use `summarize` as a hypothetical example.

**Step 1 — Add the accessor stub in `meta.go`:**

```go
func (metaAccessors) Summarize() string {
    return render(metaTmpl, "summarize.md.tmpl", nil)
}
```

**Step 2 — Create the template file:**

```
server/prompts/meta/summarize.md.tmpl
```

```
<%- /* summarize.md.tmpl — v1, no variants */ -%>
Summarize tool results or documents into a compact digest.

[prose body here]
```

**Step 3 — Register the accessor in the trim-newline test:**

Add one line to `TestRender_TrimsTrailingNewline`'s cases table in `render_test.go`:

```go
{"Meta.Summarize", Meta.Summarize},
```

That table-driven test covers the new accessor's two routine failure modes — typo'd template
name (would panic at first call), and stray trailing newline (would diverge from a Go literal
at the wire boundary). No per-accessor test file is needed; the package logic is the same
for every meta-tool.

Avoid writing per-template byte-identity tests (`require.Equal(t, raw_string, accessor())`).
With pure-markdown templates and no `<% %>` directives, those assert "string == string" —
the template body is duplicated into a Go constant that has to be edited in lockstep on
every prose change. Reach for them only when a template grows a directive (real logic to
exercise) or when variant selection lands.

**Step 4 — Wire the call site in `server.go`:**

```go
summarizeTool := &mcpsdk.Tool{
    Name:        "summarize",
    Description: prompts.Meta.Summarize(),
    InputSchema: objectSchema(map[string]any{ /* ... */ }, nil),
}
```

### Parse-don't-validate at the call site

`prompts.Meta.Summarize()` is the parse boundary for the accessor name. A typo in the
method name is a compile error. The alternative — `prompts.Meta.Get("summarize")` — pushes
validity to runtime: a mis-spelled string compiles fine, fails at first call with "template
not found." Typed methods eliminate that class of error entirely.

The struct-with-methods shape also serves namespacing: `prompts.Meta.` in IDE autocomplete
lists exactly the five (or six, or N) meta-tools. `prompts.MetaSummarize()` as a free
function works but doesn't group.

---

## Adding a New Dynamic-Prose Accessor

Dynamic accessors mirror meta accessors but live in `wrappers.go` and `dynamic/`.

**Differences from meta:**

- First parameter is always `ctx Context`. The struct is currently empty and reserved for
  future per-client variants. Pass it through to the template data even if unused.
- Additional runtime parameters follow `ctx` for values known only at call time (e.g.,
  `integration string`, `cooldownSeconds int` in `CircuitBreaker`).
- The template data struct is defined inline in the wrapper function — no named type needed
  for call-site clarity.

**Example — `CircuitBreaker`:**

```go
func CircuitBreaker(ctx Context, integration string, cooldownSeconds int) string {
    return render(dynamicTmpl, "circuit_breaker.md.tmpl", struct {
        Ctx             Context
        Integration     string
        CooldownSeconds int
    }{ctx, integration, cooldownSeconds})
}
```

Template fields are accessed as `<% .Integration %>`, `<% .CooldownSeconds %>`.

Testing for dynamic wrappers differs from meta accessors — dynamic templates DO carry
directives (parameter interpolation, conditional branches), so per-wrapper tests in
`prompts_test.go` exercise real logic: that `printf "%q"` quoting matches `fmt.Sprintf`'s
quoting byte-exact, that conditional branches produce the right output for each input case,
that table-driven inputs cover empty/unicode/quoted edge cases. Mirror the existing tests
for `SearchSummary` and `CircuitBreaker` when adding a new dynamic wrapper.

---

## Variant Story: Operational vs Per-Session

There are two distinct paths to per-client variants. They have different scope and different
prerequisites.

### Path 1 — Operational variants (in scope, not v1)

Server reads an env var at startup (e.g., `SWITCHBOARD_CLIENT_FAMILY=claude-code`). A
`renderMeta` helper selects between sibling template files:

```
server/prompts/meta/
├── execute.md.tmpl              # default (always present)
├── execute.claude-code.md.tmpl  # operational variant
└── execute.cursor.md.tmpl
```

One variant per server process. Deploy one process per client family. No SDK changes
required. Accessor signatures stay `() string` — the variant selection is internal to
`render` or `renderMeta`.

### Path 2 — Per-session variants (out of scope, requires SDK work)

Requires `modelcontextprotocol/go-sdk` to expose a `ListToolsHandler` (or equivalent) in
`ServerOptions` so each `tools/list` response can vary by session metadata. Today the SDK
registers tools globally at boot; `listTools` returns identical descriptions for every
session.

Two sub-paths: upstream a PR to the SDK, or maintain a fork. Open SDK issues #666 and #745
point in this direction but are "needs investigation." Neither sub-path is in scope for v1.

### What changes when Path 2 lands

Accessor signatures widen from `Meta.Execute() string` to `Meta.Execute(Context) string`.
All five call sites in `server.go` update simultaneously to pass a `Context` built from
session state. Any other consumer of `Tool.Description` (search indexes, telemetry, etc.)
must be touched too — easy to miss in a "small migration" claim. The SDK work is a separate
prerequisite effort.

This migration is bounded but not trivial. v1 defers it intentionally — there is no
concrete per-session need yet.

---

## Constraint-Driven Framing (for Prose Authors)

Every instruction in a meta-tool description should name the failure it prevents, not just
assert a rule. The mechanism produces compliance; the megaphone produces resentment — and
models ignore both equally when the reason is absent.

**Before:**
```
IMPORTANT: Always search before calling execute. Do NOT guess tool names.
```

**After:**
```
Tool names shift across integration versions. A guessed name returns "tool not found" with
no fallback — search first to get the live name.
```

**Before:**
```
PREFER scripts when a task requires 2+ tool calls or crosses integrations
```

**After:**
```
Two or more tool calls without a script means each intermediate result lands in the
conversation context — usually 5–10x the bytes you actually need. Use scripts to keep
intermediates server-side.
```

Rule: **every instruction names the failure it prevents**. Reach for the mechanism, not the
megaphone.

---

## Counterweights

When you write a one-direction nudge, include the opposite case. Without it the model
over-applies the rule:

> PREFER scripts when a task requires 2+ tool calls or crosses integrations.
> For two calls where the second doesn't read from the first, call directly — script
> overhead isn't worth it.

Counterweights without removal triggers become permanent cruft. When telemetry can measure
the inversion rate (e.g., "user corrects script to direct call N% of the time"), document a
removal threshold in the commit body so a future author knows when to simplify.

---

## Anthropic Prompt-Cache Discipline

Meta-tool descriptions register at server boot and don't change per session. Anthropic API
clients that cache tool registrations pay the token cost once at startup — educational
content, worked examples, and counterweights in `meta/*.md.tmpl` are amortized across the
session.

Verify with downstream clients before assuming generosity is free. If a client doesn't cache
tool registrations, the full description is paid per-request, and the description needs
trimming. This argues for keeping `meta/*.md.tmpl` bodies generous on examples by default
and leaning them out only where measurement shows runaway context use.

---

## Deferred Audit Items

### Discovery rule duplication (Workstream A item 1)

The "always search before execute," session-context, and auto-pin protocols appear in both
`ServerOptions.Instructions` (server.go) and the individual meta-tool descriptions. Every
session reads the same rule twice.

**Status:** deferred to a follow-up workstream.

**Why deferred:** picking between Option A (consolidate to `Instructions`, slim descriptions)
and Option B (remove `Instructions`, descriptions stand alone) requires measurement. Not all
MCP clients forward `Instructions` to the model — some strip or summarize it. Without
telemetry on client behavior, choosing A risks losing the rule for clients that drop
`Instructions`; choosing B leaves a per-session token cost on every description. Pick one
when measurement is available.

### MCP Resources for execute's JSON examples (Workstream A item 5)

The seven worked JSON examples in `execute.md.tmpl` account for ~30% of its tokens.
MCP Resources could deliver them on-demand instead of carrying them in every session prompt.

**Status:** deferred to a follow-up workstream.

**Why deferred:** the tradeoff is discoverability (gone if externalized — the model no longer
sees examples passively) vs context cost. Justified only if measurement shows examples bloat
real-workload context beyond an acceptable threshold. Land when a measurement target exists.

### Downstream-client cache verification (Workstream A item 9)

The Prompt-Cache Discipline section above assumes Anthropic API clients cache MCP tool
registrations between sessions. If they do, educational content in `meta/*.md.tmpl` is paid
once at startup and amortized; if they don't, every session pays the full description.

**Status:** deferred. Requires inspecting downstream-client internals (Claude Code, Cursor,
others) or running an instrumented session to observe cache headers.

**Question to answer:** for each downstream Anthropic-using MCP client, does the
`tools/list` response land in the prompt-cache prefix, or is it re-injected per request?

**Implication if false:** the descriptions are paid per-session, and the generosity in
worked examples + counterweights becomes a measurable token cost. The first action would be
trimming `execute.md.tmpl`'s 7 JSON examples — which is exactly what item 5 (MCP Resources)
would land. The two deferrals share a trigger: if real-workload measurement shows description
cost is a problem, address them together.
