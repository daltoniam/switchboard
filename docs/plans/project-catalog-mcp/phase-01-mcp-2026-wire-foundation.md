# Phase 1: MCP 2026-07-28 Wire Foundation

## Goal

Upgrade Switchboard to a stable Go MCP SDK that implements the complete published `2026-07-28` surface needed by this plan and lock actual wire behavior before domain/resource implementation. This phase belongs first because all later resource, cache, structured-result, and subscription work depends on a proven modern SDK and stateless request model.

The server must continue supporting existing pre-2026 clients during the migration window. Switchboard application sessions (`X-Switchboard-Session-Id`) remain a product feature, not an MCP transport session.

## BDD Success Criteria

#### Scenario: A modern client discovers Switchboard without initialization

- **Given** Switchboard is running through its production Streamable HTTP handler
- **When** a client sends `server/discover` with protocol version `2026-07-28` in request `_meta`
- **Then** the response advertises `2026-07-28`, server instructions, and its real capabilities
- **And** the response has `resultType: "complete"`, `ttlMs`, and `cacheScope`
- **And** no `Mcp-Session-Id` is required or returned for the modern flow.

#### Scenario: A modern request is self-describing and stateless

- **Given** two independent HTTP POST requests without transport affinity
- **When** both carry valid per-request protocol metadata and invoke `tools/list` or a tool
- **Then** both succeed without initialize/initialized state
- **And** tool availability does not vary because of an earlier request on either connection.

#### Scenario: A legacy client remains compatible

- **Given** a client using the currently supported initialize/initialized sequence
- **When** it connects to `/mcp` and calls `search`
- **Then** the existing response shape remains valid
- **And** `TestSmoke_SearchResponseShape` still passes.

#### Scenario: App sessions do not depend on removed MCP sessions

- **Given** two calls carrying the same valid `X-Switchboard-Session-Id`
- **When** the first stores Switchboard context or a pin and the second retrieves it
- **Then** the state is shared by the Switchboard application session
- **And** omitting `Mcp-Session-Id` does not change the behavior.

#### Scenario: Invalid modern protocol metadata fails explicitly

- **Given** a request with an unsupported protocol version or missing required modern `_meta`
- **When** it reaches the production handler
- **Then** the server returns the SDK's protocol error
- **And** it does not silently reinterpret the request as another tenant, app session, or project.

## Implementation Instructions

### Required files

- Create `server/mcp_sdk_feasibility_test.go` and `docs/plans/project-catalog-mcp/evidence/phase-01-sdk.md` as a fail-closed spike **before selecting the SDK version**. The compile/run proof must cover: `server/discover`; stateless modern + legacy compatibility; typed structured tool output; dynamic resource add/delete reflected in list; deterministic opaque-cursor paging; caller-controlled private cache scope for resource list/templates/read; `subscriptions/listen`; resource-specific updates; and disconnect/reconnect. Go SDK v1.7.0 is known not to satisfy dynamic/private-cache requirements without changes.
- Modify `go.mod` and `go.sum` to pin the first stable upstream SDK that passes the complete spike. If none exists, use only a narrowly scoped upstream-compatible patch/replace with exact commit provenance and conformance tests; do not proceed to Phase 2 with unresolved SDK feasibility.
- Modify `server/server.go`:
  - Remove the deprecated logging capability from the modern advertised capability set.
  - Keep `tools.listChanged=false` because Switchboard's tool list is static during one process lifetime.
  - Ensure `server/discover` reports accurate capabilities from real registered features.
  - Make the production HTTP handler stateless so the SDK advertises/accepts `2026-07-28`; retain legacy initialize handling through the SDK's stateless compatibility path.
- Modify `cmd/server/main.go` and create a testable `buildHTTPMux` (or equivalently named) composition helper so production and E2E tests mount exactly the same `/mcp` handler. The production route must use the modern-capable stateless handler, not `Handler()`. Preserve explicit Switchboard app sessions via middleware.
- Modify `server/session_context.go` and `server/session_context_test.go`:
  - Keep `X-Switchboard-Session-Id` as the primary explicit app-state key.
  - Retain `Mcp-Session-Id` only as a documented legacy fallback; add no new dependency on it.
- Create `server/mcp_wire_test.go` for raw JSON-RPC Streamable HTTP tests covering `server/discover`, per-request metadata, headers, `resultType`, cache fields, and modern/legacy coexistence.
- Modify `server/server_test.go` only where existing fixtures must account for modern fields; preserve the smoke shape contract rather than weakening it.
- Update `docs/architecture.md` with a short protocol-version section distinguishing MCP transport from Switchboard app sessions.

### TDD sequence

1. Add the complete SDK feasibility suite and raw HTTP tests; run `go test ./server -run 'TestMCPSDKFeasibility|TestMCPWire_20260728' -count=1` and retain the expected failures under SDK v1.5.0/v1.7.0 where applicable.
2. Select/pin the proven SDK or narrow patch, record provenance and results, run `go mod tidy`, and make only compatibility changes required to compile.
3. Add/adjust the app-session test proving `X-Switchboard-Session-Id` works with no MCP session header.
4. Add the legacy initialize/search test before changing capability configuration.
5. Run focused tests, then `go test ./server ./cmd/server`.

### Contract decisions

- Modern requests use per-request `_meta`; no mutable project or application selection is stored in transport state.
- `X-Switchboard-Session-Id` remains supported for Switchboard history/context/pins, but it never selects a project and is not advertised as an MCP protocol session.
- Do not add MCP Roots. The latest spec deprecates it, and project roots will be explicit `rootUri` arguments in Phase 2/5.
- Do not add Tasks in this phase.

## End-to-End Test Plan

1. Start `httptest.NewServer(buildHTTPMux(...))` using the same composition helper and handler choices as production; asserting only `srv.StatelessHandler()` is insufficient.
2. POST raw JSON-RPC `server/discover` to production `/mcp` with 2026 request metadata. Assert exact protocol version support, capabilities, `resultType`, `ttlMs`, `cacheScope`, and no session header.
3. On a fresh HTTP client/connection, POST `tools/list` with modern metadata and assert `search` and `execute` exist.
4. Connect the Phase 1 pinned Go MCP client over Streamable HTTP and call `search` without legacy initialization being required by the server.
5. Connect using the SDK's legacy protocol configuration and call the same operation.
6. Through the real handler, set app-session context with `X-Switchboard-Session-Id`, close the first client, reconnect, and verify retrieval.
7. Run:

```bash
go test ./server ./cmd/server -count=1
go test ./server -run TestSmoke_SearchResponseShape -count=1
```

The test registry/config may be in-memory because the behavior under test is the MCP wire and production server wiring, not durable project storage.

## Anti-Cheating Audit

- Inspect `server/mcp_wire_test.go` to ensure requests traverse the production `buildHTTPMux` `/mcp` route, not a substituted handler or private method.
- Confirm `server/discover` is provided by the real SDK/server configuration, not a custom hard-coded mux response.
- Search for new reliance on `Mcp-Session-Id`; it may appear only in the existing legacy fallback and compatibility tests.
- Ensure modern tests do not call initialize before claiming sessionless operation.
- Ensure legacy compatibility tests are not skipped under environment flags.
- Check that capability assertions inspect the wire JSON, not only Go structs created before serving.
- Verify no production branch checks `testing`, `GO_TEST`, or a test-only protocol flag.

## Completion Gate

- [ ] RED evidence under the old SDK was observed for the modern wire test.
- [ ] `go.mod` pins a stable/proven SDK source that passes the complete dynamic-resource/private-cache/subscription/compatibility spike, with evidence and provenance recorded.
- [ ] Modern and legacy public-boundary tests pass.
- [ ] App-session continuity is proven without `Mcp-Session-Id`.
- [ ] `TestSmoke_SearchResponseShape` passes unchanged or with only additive modern fields.
- [ ] `gofmt` and `go mod tidy` are clean.
- [ ] `go test ./server ./cmd/server -count=1` passes.
- [ ] Anti-cheating audit finds no fake wire handler or implicit connection state.
- [ ] `make ci` passes for this phase commit; focused tests and RED→GREEN evidence are recorded additionally.
