# Phase 4: Resources-First Catalog MCP

## Goal

Expose projects through canonical MCP resources and resource templates on a dedicated `/project-catalog/mcp` endpoint. Reads become standard, cacheable MCP data access; tools remain for query and mutation in the next phase.

## BDD Success Criteria

#### Scenario: Discover the catalog resources

- **Given** projects exist in the shared filesystem catalog
- **When** a modern client calls `resources/list`
- **Then** an authenticated caller receives `project://registry/catalog` and one current-definition resource per project in deterministic project-ID order
- **And** the result includes private cache metadata and an opaque next cursor when required.

#### Scenario: Discover resource templates

- **Given** the dedicated catalog endpoint
- **When** a client calls `resources/templates/list`
- **Then** it sees templates for current definitions, immutable revisions, context manifests, and nested context paths
- **And** the templates use valid RFC 6570 and MIME declarations.

#### Scenario: Read current and immutable definitions

- **Given** project revision R1 exists and the current definition later becomes R2
- **When** a client reads the current definition URI and each revision URI
- **Then** the current URI returns a new source revision and effective revision
- **And** the R1 URI still returns the exact effective R1 definition
- **And** the current envelope reports current provenance while immutable envelopes contain only project ID, revision, and effective definition as frozen in `contracts.md`.

#### Scenario: Read context progressively

- **Given** a project has multiple context entries
- **When** the client reads the context manifest and then one context URI
- **Then** it receives only the selected content with the declared MIME type
- **And** a traversal or symlink escape returns resource-not-found/invalid-params without leaking content.

#### Scenario: Missing resources use modern MCP errors

- **Given** a nonexistent project, revision, or context path
- **When** a client calls `resources/read`
- **Then** the server returns JSON-RPC `-32602` with `data.uri`
- **And** it never returns an ambiguous empty `contents` array.

#### Scenario: Resource reads are independent of connection history

- **Given** two separate modern HTTP requests with the configured bearer token
- **When** both read the same URI
- **Then** they receive the same representation regardless of prior calls or app-session headers.

#### Scenario: Remote callers are rejected before catalog dispatch

- **Given** a request with a missing/invalid bearer token, including one from loopback or a local reverse proxy with forged forwarding headers
- **When** it targets `/project-catalog/mcp`
- **Then** the endpoint returns HTTP 401 with `WWW-Authenticate: Bearer`
- **And** forwarding/app-session headers do not grant read or write access.

## Implementation Instructions

### Required files

- Create `server/project_catalog_server.go`:
  - Implement `ProjectCatalogServer` over injected `project.Catalog`/validator/writer ports.
  - Enforce the bearer capability policy from `contracts.md` before MCP dispatch using constant-time comparison; ignore forwarding/app-session headers for authorization.
  - Construct an `mcpsdk.Server` implementation named `switchboard-project-catalog`, with accurate resources/tools capabilities and instructions that projects are declarative—not sessions/workspaces. Do not assert that 2026 `server/discover` carries implementation identity; Go SDK v1.7's `DiscoverResult` does not expose it.
  - Expose `Handler()` using production Streamable HTTP stateless mode and `RunStdio` only if needed by composition; do not tie resources to Switchboard app sessions.
- Create `server/project_catalog_resources.go`:
  - Centralize URI parsing/building; use URL path escaping and reject ambiguous/double-decoded forms.
  - Register `project://registry/catalog`, the diagnostics resource, and the definition/revision/context templates frozen in `contracts.md`, including explicit optional `rootUri` query expansion on context templates.
  - Add resources/templates with assistant/user audience, priority, and `lastModified` where known.
  - Return `application/json` for catalog/definition/manifest and the resolved MIME type for context content.
- Create `server/project_catalog_server_test.go` and `server/project_catalog_resources_test.go`; include exact diagnostics envelope/listing behavior for valid warnings and invalid sources.
- Use Phase 2 `Catalog.Diagnostics(context.Context, ProjectID, *url.URL)`; nil root returns global/default diagnostics, explicit root returns that worktree overlay/context diagnostics. Resource handlers must not parse invalid files independently of the catalog.
- Modify `mcp.go`, `config/config.go`, and `config/config_test.go` to add the exact top-level `ProjectCatalogConfig` from `contracts.md`, with backward-compatible safe defaults (`enabled=false`, `writes_enabled=false`, no token). Phase 4 uses `enabled` to mount or omit the endpoint; Phase 5 begins honoring `writes_enabled` for canonical tools.
- Modify `cmd/server/main.go`:
  - Mount `/project-catalog/mcp` only when `project_catalog.enabled=true`, before `/mcp/{project}`; in Go `ServeMux`, the literal route is more specific, but add routing/config tests to lock this.
- Modify `cmd/server/main_test.go` to test the production mux, endpoint precedence, default-disabled decoding, and explicit disabled behavior.
- Update `docs/architecture.md` and `README.md` with the endpoint and URI inventory.

### Resource envelopes

- Catalog envelope: valid `projects` use the exact `ProjectSummary`; malformed sources appear only in separate `invalidProjects` entries without revision/definition/context URI and link to `project://registry/projects/{projectId}/diagnostics`, as frozen in `contracts.md`. Do not include full context or secrets.
- Current definition: effective `Definition`, revision, current source provenance, and diagnostics. Immutable revision: only project ID, revision, and effective definition—never invocation-specific provenance/diagnostics. Exact envelopes are frozen in `contracts.md`. Current definitions may include local paths; therefore always `cacheScope=private`.
- Context manifest: project ID, effective revision, entries with path, resource URI, source kind, MIME type, and size.
- Context file: exactly one `TextResourceContents`; binary files remain unsupported by Project Interop v1.

### Dynamic resources, pagination, and caching

Phase 1 already selected and pinned an SDK source proven to support the entire required surface. Phase 4 must consume that proven mechanism; it must not reopen SDK selection or weaken private-cache/dynamic-list assertions.

- In Phase 4, compute a catalog generation digest from the sorted valid/invalid catalog entries and their current revisions/diagnostic source digests before every `resources/list` page. Reconcile SDK registrations from that truth before pagination; no event dependency is introduced yet. Phase 6 adds semantic events for efficient synchronization. Notifications alone are never sufficient.
- Encode query/list kind, generation digest, and last project ID in an authenticated/validated opaque cursor. Before serving a subsequent page, recompute generation; if compatibility tools or external files changed it, reject the cursor with invalid params and require restart from page one. Add tests for add/update/delete/malformed changes between pages through compatibility tools and direct atomic file edits.
- `resources/list`, templates, current reads: short TTL (plan default 10 seconds).
- Immutable revision reads: long TTL (plan default 1 hour).
- All are private.

### TDD sequence

1. Add route-precedence and `server/discover` capability tests.
2. Add raw wire tests for resources/list and templates/list; prove RED before registering resources.
3. Add current-definition read, then immutable revision read.
4. Add manifest/file read and escape failures.
5. Add pagination/cache field assertions on raw wire JSON.
6. Run modern and legacy tests to ensure adding resources does not alter old tool lists.

## End-to-End Test Plan

1. Create two projects and context files in a real temporary catalog root.
2. Start the actual combined mux in `httptest.Server`.
3. Use the Phase 1 pinned Go MCP client over Streamable HTTP to discover/list/read resources and templates.
4. Use raw JSON-RPC to assert wire-only fields (`resultType`, `ttlMs`, `cacheScope`, cursor) that high-level SDK helpers may abstract.
5. Mutate one project through the catalog core, then prove current alias advances and immutable R1 remains readable. Delete the project, reconstruct the catalog/server, and prove R1 remains readable by revision URI.
6. Page `resources/list`, mutate via compatibility MCP and external atomic edit before the next page, and assert the old cursor is rejected; restart listing and assert complete deterministic truth.
7. Request malformed URI encodings, nonexistent resources, traversal, and symlink escape.
8. Hit `/project-catalog/mcp` and prove it remains disjoint from every `/mcp/{project}` name, including a real `project-catalog` project.
9. Run:

```bash
go test ./server ./cmd/server -run 'ProjectCatalog|ProjectCatalogRoute' -count=1
go test -race ./project ./server ./cmd/server -count=1
```

## Anti-Cheating Audit

- Ensure resource handlers read the injected catalog/revision store, not fixture maps or startup snapshots.
- Inspect route registration order/patterns and the E2E request target.
- Confirm revision resources survive reconstruction and current mutation.
- Verify resource-not-found is a protocol error `-32602`, not a successful tool-style error.
- Ensure tests inspect content and provenance, not only URI presence/status code.
- Confirm context resource handlers use canonical resolved entries rather than joining arbitrary URI path strings.
- Check no resource includes runtime session, workspace, AgentRun, PID, port, token, or conversation data.
- Verify cache scope is private on the wire.

## Completion Gate

- [ ] Dedicated endpoint advertises the resources capability through modern discovery.
- [ ] Catalog, definition, revision, manifest, and context resources are readable.
- [ ] Diagnostics plus definition, revision, and both root-aware context templates are discoverable and valid.
- [ ] Pagination/deterministic ordering/cache fields pass raw wire tests.
- [ ] Missing and unsafe reads return `-32602` without leakage.
- [ ] Current alias and immutable revision behavior pass across catalog reconstruction.
- [ ] Route precedence is locked by E2E test.
- [ ] Focused race tests pass.
- [ ] `make ci` passes for this phase commit; focused tests and RED→GREEN evidence are recorded additionally.
