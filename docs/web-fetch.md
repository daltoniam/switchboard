# Spec: `fetch_url` Tool for Switchboard

## Motivation

Frontier models (Sonnet, Opus) have strong library and API knowledge baked into training and can
fall back to web search. Open-source models like Gemma 4 26B don't. The knowledge gap shows up most
clearly when a model needs to reason about a specific library version, an API reference, or a doc
page that postdates or was underrepresented in its training data — it will hallucinate method
signatures or miss version-specific behavior.

`fetch_url` closes that gap by letting the model pull the actual source of truth into context at
call time. It is also the minimum viable tool needed to make local models useful for the kind of
work switchboard is designed for: referencing runbooks, API docs, internal wikis, GitHub raw files,
and package documentation without copy-pasting.

---

## Tool Definition

**Name:** `fetch_url`

**Description (shown to model verbatim — keep this precise):**
> Fetches the content of a URL and returns it as plain text. Use this to read documentation, API
> references, README files, GitHub raw content, or any web page whose content you need to reason
> about. Returns extracted readable text, not raw HTML.

**Why the description matters:** Weak tool-callers like Gemma 4 rely heavily on the tool
description to decide when and how to call it. Vague descriptions ("get web content") cause missed
calls. Overly broad descriptions ("browse the internet") cause spurious ones. The description above
scopes it to reference/doc lookup, which is the actual use case.

---

## Input Schema

```go
type FetchURLArgs struct {
    URL     string `json:"url"      jsonschema:"required,description=The full URL to fetch (https only)"`
    Timeout int    `json:"timeout"  jsonschema:"description=Request timeout in seconds (default: 10 max: 30)"`
}
```

**Keep the schema flat and minimal.** Two fields max. Every additional parameter is a chance for a
weak model to get the call wrong. No headers, no auth, no output format selector. If those are
needed later, add a v2 tool rather than complicating this one.

---

## Output

Return a single plain-text string. The model receives this as the tool result content.

Structure:

```
Source: <url>
Fetched: <RFC3339 timestamp>

<extracted text content>
```

The `Source` header lets the model cite where the information came from. The timestamp is useful
when the model needs to reason about content freshness.

**Content extraction rules:**

1. Strip all HTML tags
2. Remove `<script>`, `<style>`, `<nav>`, `<footer>`, `<header>`, `<aside>` blocks before
   stripping — these are noise
3. Preserve code blocks: content inside `<pre>` and `<code>` tags should be kept verbatim with a
   blank line before and after
4. Collapse runs of whitespace (3+ blank lines → 2)
5. For plain text responses (`.txt`, `.md`, raw GitHub content), return as-is without extraction
6. Truncate at **40,000 characters** and append `[truncated — content exceeded limit]`. This keeps
   the tool useful for large docs without blowing the model's context window. Gemma 4 26B has 256K
   context but only ~20K is usable before performance degrades on the M1.

---

## Implementation Notes

### Allowed schemes

HTTPS only. Reject `http://`, `file://`, `ftp://`, and anything else with a clear error:

```
error: only https:// URLs are supported
```

### Localhost / private range blocking

Reject requests to localhost, `127.x.x.x`, `10.x.x.x`, `172.16-31.x.x`, `192.168.x.x`, and
`::1`. This prevents the tool from being used to probe internal infrastructure if the model is
ever fed a malicious prompt. Return:

```
error: requests to private/local addresses are not allowed
```

### User-Agent

Set a descriptive user agent:

```
switchboard-mcp/1.0 (fetch_url; +https://github.com/daltoniam/switchboard)
```

This is good hygiene — it lets site operators identify and rate-limit automated fetches rather than
seeing a scraper UA.

### Timeouts

Default 10 seconds. Allow the caller to set up to 30 via the `timeout` param. Enforce the cap
server-side regardless of what the model passes. Return a clear timeout error rather than hanging:

```
error: request timed out after 10s
```

### Redirect handling

Follow up to 5 redirects. After 5, return an error rather than looping. Log the final URL in the
`Source` field if it differs from the requested URL.

### Error messages

Return errors as plain-text tool results, not Go errors that bubble up as MCP protocol errors.
The model needs to read the error and decide what to do next. A JSON error response the model
can't see is useless.

```go
// Good — model sees this
return mcpResult("error: HTTP 404 — page not found at " + url), nil

// Bad — disappears into the MCP error channel
return nil, fmt.Errorf("HTTP 404")
```

---

## Config / Registration

This tool should be gated by a config flag so it can be selectively enabled. It is off by default
in the standard work config (where the existing integrations cover what's needed) and opt-in for
personal/local model configs.

Suggested config key: `tools.fetch_url.enabled` (bool, default `false`)

When registering tools on startup, check the flag:

```go
if cfg.Tools.FetchURL.Enabled {
    server.RegisterTool("fetch_url", fetchURLDescription, handleFetchURL)
}
```

This also keeps the tool out of the tool manifest when it's not needed, which matters for models
that enumerate all available tools at the start of a session — fewer tools means less context
burned on the manifest.

---

## What This Is Not

- Not a search tool. It does not query a search engine or discover URLs. The model must provide
  a specific URL. If search is needed later, that is a separate `web_search` tool.
- Not a browser. It does not execute JavaScript, handle SPAs, click elements, or fill forms.
  Sites that require JS rendering (some dashboards, some docs) will return incomplete content.
  This is acceptable — the primary targets (pkg.go.dev, GitHub raw, docs sites, README files)
  are static.
- Not authenticated. No cookie jars, no OAuth, no API key injection. Authenticated endpoints
  should be handled by the existing provider-specific tools already in switchboard.

---

## Usage Pattern for Local Models

When using Gemma 4 26B (or similar) with a minimal switchboard config, the recommended setup is:

```json
{
  "tools": {
    "fetch_url": { "enabled": true },
    "github":    { "enabled": true }
  }
}
```

Everything else disabled. Two tools is a tool manifest the model can reliably navigate. Add more
only if you find yourself needing them in practice.

The intended workflow:

1. Model needs to reason about a library or API
2. Model calls `fetch_url` with the relevant doc URL (pkg.go.dev, a GitHub README, a changelog)
3. Content is injected into context
4. Model answers based on actual current documentation rather than training data

This is also how you work around the Flash Attention / tool-calling bugs during the Gemma 4 early
period — even if agentic tool loops are flaky, a single explicit fetch call is simple enough to
work reliably once the parser is fixed.
