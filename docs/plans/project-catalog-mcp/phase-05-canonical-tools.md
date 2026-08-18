# Phase 5: Canonical Typed Project Tools

## Goal

Add a small canonical tool surface to the dedicated Project Catalog endpoint. Read tools search, resolve, and validate; administrative tools create, patch, and delete user definitions with explicit revisions and authorization. Every tool has JSON Schema 2020-12 input/output and typed `structuredContent`.

## BDD Success Criteria

#### Scenario: Search projects with deterministic pagination

- **Given** several projects visible to the caller
- **When** `project_search` receives a case-insensitive query and optional cursor
- **Then** it returns deterministic compact summaries and resource links
- **And** cursor replay continues the same query/order without duplication.

#### Scenario: Resolve by ID and explicit root

- **Given** a worktree-specific overlay
- **When** `project_resolve` is called with `projectId` and `rootUri`
- **Then** it returns the effective project ID, revision, resource URIs, root, provenance, and diagnostics
- **And** no connection state or MCP Roots call is required.

#### Scenario: Resolve by root only

- **Given** a root inside exactly one registered project
- **When** `project_resolve` receives only `rootUri`
- **Then** it resolves that project
- **And** equal/ambiguous candidates return a structured `ambiguous_project` error.

#### Scenario: Validate without mutation

- **Given** an invalid candidate definition
- **When** `project_validate` is called
- **Then** it returns value-free structured diagnostics
- **And** neither user files nor revision files are created.

#### Scenario: Administration is disabled by default

- **Given** the dedicated Catalog endpoint is running with no explicit write enablement
- **When** a caller lists and calls `project_create`, `project_patch`, or `project_delete`
- **Then** the tools remain listed with stable schemas but each call returns structured `write_disabled`
- **And** read tools/resources continue to work.

#### Scenario: Guarded patch succeeds once

- **Given** write administration is enabled and current source revision is S1
- **When** `project_patch` supplies S1 and a valid RFC 7396 patch
- **Then** it returns a new source revision and effective revision and a resource link
- **And** repeating the same request with S1 returns `revision_conflict` without another write.

#### Scenario: Delete has no runtime cascade

- **Given** a user definition, repo-local overlay, context files, and revision history
- **When** authenticated `project_delete` succeeds with writes explicitly enabled and the current source revision
- **Then** only the user-level file is removed
- **And** context, repo-local file, and immutable revisions remain.

#### Scenario: Tool metadata describes risk accurately

- **Given** `tools/list`
- **When** the client inspects canonical tools
- **Then** read tools advertise read-only hints
- **And** mutations advertise conservative destructive/idempotency hints
- **And** every tool has an output schema matching its structured result.

## Implementation Instructions

### Required files

- Create `server/project_catalog_tools.go`:
  - Define typed input/output structs and register tools with generic `mcpsdk.AddTool` so schemas and `structuredContent` are generated/validated.
  - Use exact names: `project_search`, `project_resolve`, `project_validate`, `project_create`, `project_patch`, `project_delete`.
  - Return resource links in `content` where the SDK permits while preserving typed output.
  - Advertise each output as `oneOf` the frozen success schema and `{error: ToolError}`; test that typed/generic SDK registration does not overwrite this union and that both success and `IsError=true` structured failures validate.
- Create `server/project_catalog_tools_test.go` with schema, annotation, success, error, and no-write validation tests.
- Modify `server/project_catalog_server.go` to accept the typed `ProjectCatalogConfig`; writes default false. The endpoint remains bearer-capability protected and disabled by default. Do not add spoofable identity headers or claim hosted/multi-principal authorization.
- Reuse the Phase 4 top-level `ProjectCatalogConfig`; begin honoring `writes_enabled` in canonical handlers. Keep it independent of the ordinary ProjectInterop integration and keep ordinary `projectinterop_*` tools controlled by existing integration enablement/`ToolGlobs`.
- Extend config clone/save/equality and web serialization tests only if Phase 4 did not cover a path; a UI control is optional and out of scope.
- Modify `cmd/server/main.go` to pass the explicit option.
- Update `README.md` and `docs/architecture.md` with tool semantics and write safety.

### Exact tool contracts

- `project_search`: `{query?: string, cursor?: string}` → `{projects: ProjectSummary[], nextCursor?: string}`.
- `project_resolve`: require at least one of `{projectId, rootUri}` and use the Git-common-directory algorithm in `contracts.md`. No `profile` or model-supplied role exists in this release.
- `project_validate`: `{definition: object, rootUri?: string}` → `{valid: boolean, diagnostics: Diagnostic[]}`.
- `project_create`: `{definition: object}` → snapshot summary/resource links. The definition name is the project ID.
- `project_patch`: `{projectId, expectedSourceRevision, patch}` → new snapshot summary/resource links. Reject patching `name`.
- `project_delete`: `{projectId, expectedSourceRevision?, expectedRawSourceRevision?}` with exactly one token → `{projectId, deleted: true}`. Valid sources use source CAS; malformed-source recovery uses raw-byte CAS and can then recreate normally.

Use typed domain error output with stable codes. Input schema/type errors remain MCP invalid-params errors; expected domain failures are tool results with `IsError=true`, machine-readable structured error, and concise text for the model.

### Tool annotations

- Search/resolve/validate: `readOnlyHint=true`.
- Create: `readOnlyHint=false`, `destructiveHint=false`, `idempotentHint=false`, `openWorldHint=false`.
- Patch/delete: `readOnlyHint=false`, `destructiveHint=true`, `idempotentHint=false`, `openWorldHint=false`.

Annotations are hints only; authorization must be enforced in handlers.

### TDD sequence

1. Write tools/list schema/annotation tests before registration.
2. Add search with real catalog paging.
3. Add resolve by ID, root, both, ambiguous root, and unsafe root.
4. Add validate and prove zero filesystem writes.
5. Add default-off write visibility/rejection test.
6. Add create, patch conflict (including hidden user-layer change), rename rejection, malformed-source patch rejection, raw-CAS delete/recreate recovery, and delete non-cascade tests.
7. Add structured-content/output-schema round-trip tests using the official client.

## End-to-End Test Plan

1. Start the bearer-authenticated dedicated endpoint through the production mux with real filesystem catalog and writes disabled. Assert tools/list/read calls; write behavior is unavailable/denied.
2. Restart the authenticated test server with typed `project_catalog.writes_enabled=true`; separately prove missing/invalid tokens are rejected from both direct loopback and local-proxy-style requests despite forged forwarding headers.
3. Call all six tools using official Go MCP client methods, not private handlers.
4. Assert both success and structured `IsError=true` alternatives decode against each advertised `oneOf` output schema and resource links can be followed through `resources/read`.
5. Race two `project_patch` calls with the same expected source revision; one succeeds and one conflicts.
6. Call validate before/after and compare filesystem tree hashes to prove no mutation.
7. Delete and prove context/repo-local/revision files remain.
8. Run:

```bash
go test ./server ./config ./cmd/server -run 'ProjectCatalogTool|CatalogWrites' -count=1
go test -race ./project ./server ./config ./cmd/server -count=1
```

## Anti-Cheating Audit

- Confirm tests invoke `tools/call` over the actual endpoint and validate wire `structuredContent`.
- Ensure bearer authentication and typed write enablement run server-side for every call and are not inferred from tool visibility or annotations.
- Verify disabled writes return `write_disabled` even though stable tool definitions remain listed.
- Check validate cannot reach `Create`, `Patch`, revision persistence, or a shared mutable map.
- Inspect conflict tests for a real simultaneous race and final file validation.
- Ensure tool errors carry stable codes and do not emit raw file contents, secrets, or stack traces.
- Confirm `project_resolve` receives explicit root input and never reads app/MCP session state.
- Verify no MCP Tasks or runtime-session handles are introduced for bounded CRUD.

## Completion Gate

- [ ] Six canonical tools are exposed with frozen names.
- [ ] Input/output schemas and structured results pass official-client tests.
- [ ] Search/resolve/validate behavior and errors pass through public MCP.
- [ ] Writes are disabled by default, require explicit typed enablement, and remain bearer-authenticated.
- [ ] Optimistic patch/delete and non-cascade behavior pass.
- [ ] Tool annotations match the frozen risk table.
- [ ] Focused race tests pass.
- [ ] Documentation clearly distinguishes catalog projects from work sessions/AgentRuns.
- [ ] `make ci` passes for this phase commit; focused tests and RED→GREEN evidence are recorded additionally.
