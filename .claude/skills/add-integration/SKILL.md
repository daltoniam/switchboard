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
- Org/workspace slug injection (see `sentry/org()`)
- Entity ID resolution by name (see `linear/resolveTeamID()`)
- Query string building from optional params (see `sentry/queryEncode()`)

Note: arg helpers (`argStr`, `argInt`, `argBool`) are intentionally duplicated *across* adapters — see `AGENTS.md > Gotchas`. Raw-HTTP adapters also duplicate result helpers (`rawResult`, `errResult`). Do not extract shared utilities.

## 4. Testing Requirements

Every adapter must have these test categories (see existing `*_test.go` files):

- [ ] **Constructor**: `New()` returns valid integration, `Name()` matches
- [ ] **Configure success**: Valid credentials accepted
- [ ] **Configure failures**: One test per required credential, verifying error message
- [ ] **Tools metadata**: All have Name + Description, prefix matches `Name()`, no duplicates
- [ ] **Dispatch parity (non-negotiable)**:
  - `TestDispatchMap_AllToolsCovered` — every `Tools()` entry has a dispatch handler
  - `TestDispatchMap_NoOrphanHandlers` — every dispatch key has a `ToolDefinition`
- [ ] **Execute unknown tool**: Returns `IsError: true`, `"unknown tool"` in Data
- [ ] **HTTP helpers**: `httptest.NewServer` for success, API errors (>=400), 204 no-content
- [ ] **Arg helpers**: Unit tests for type coercion (`float64→int`, `string→bool`)

## 5. Wiring and Verification

Follow `AGENTS.md > Adding a New Integration` steps 6-7 (register + config defaults), then verify:

1. `go build ./...` && `go test ./...` && `go vet ./...` && `golangci-lint run`
2. Smoke test: start server, call `search` for new integration tools, `execute` one

## Anti-Patterns

| Mistake | Correct approach |
|---------|-----------------|
| Defaulting missing credentials | Return error from `Configure()` |
| Returning Go error for API failures | Use `ToolResult{IsError: true}`, nil Go error |
| Skipping dispatch parity tests | Non-negotiable — tests catch tool/handler drift |
| Pre-building helpers before duplication | Wait for 3+ uses, then extract |
| Duplicating AGENTS.md content in handlers | Read AGENTS.md for conventions |
| Adding OAuth before basic auth works | Get token-based auth working first, add OAuth flow after |
