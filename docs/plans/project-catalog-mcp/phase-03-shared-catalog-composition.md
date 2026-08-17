# Phase 3: One Catalog in the Composition Root

## Goal

Eliminate the two independent project stores and inject one catalog into every existing project consumer while preserving the six `projectinterop_*` tools. After this phase, a mutation through any existing project adapter is immediately visible to every other adapter without restart.

## BDD Success Criteria

#### Scenario: Existing tool mutation is visible to the scoped router

- **Given** Switchboard starts with one shared catalog and an existing project
- **When** `projectinterop_update_project` changes it through global `search`/`execute`
- **Then** the next project-scoped read sees the new effective revision and definition
- **And** no server restart or explicit reload is required.

#### Scenario: Existing tool success shapes remain compatible

- **Given** a client using the six merged `projectinterop_*` names
- **When** it lists, gets, creates, updates, reads context, and deletes projects
- **Then** the successful payloads remain compatible with the tests at the base revision
- **And** mutations still affect only user-level definitions.

#### Scenario: Disabled ProjectInterop administration remains disabled

- **Given** the `projectinterop` integration is disabled or its `ToolGlobs` exclude a write tool
- **When** a caller searches for or executes that write tool through `/mcp`
- **Then** the normal Switchboard policy hides or rejects it
- **And** injection of the shared catalog does not bypass configuration.

#### Scenario: Project-scoped reads do not use stale closures

- **Given** a project endpoint has already served one request
- **When** the underlying definition changes through the shared catalog
- **Then** the next request resolves a current snapshot
- **And** deletion makes the next request fail rather than serving cached data.

## Implementation Instructions

### Required files

- Modify `mcp.go`:
  - Add project read/write ports to `Services` only if doing so does not create a root-package import cycle. Preferred solution: define minimal ports in root `mcp.go` using neutral DTOs or place a `ProjectCatalog` aggregate dependency in `cmd/server` constructors. Do not make the domain `project` package depend on adapters.
  - Document catalog ownership as process-wide.
- Modify `integrations/projectinterop/projectinterop.go`:
  - Change `New()` to accept an injected catalog (or add `NewWithCatalog` while retaining `New` only for tests/backward API compatibility).
  - `Configure` must not allocate/load another store when a catalog was injected.
  - Replace local argument/result helpers with `mcp.NewArgs`, `mcp.JSONResult`, `mcp.RawResult`, and `mcp.ErrResult` as required by repository conventions.
  - Map typed catalog errors to existing tool errors without changing successful compatibility shapes.
- Modify `integrations/projectinterop/projectinterop_test.go` **before refactoring constructors**:
  - Add full wire-shape locks/goldens for all six successful tools, including empty list as `[]` (not `null`), unknown fields, deterministic context-manifest ordering, raw context text, update projection, and exact delete text.
  - Add a create golden where an existing repo-local overlay differs from the user definition; the compatibility result must remain the persisted user definition rather than the effective overlay.
  - Keep dispatch parity and CRUD/context behavior assertions.
  - Add a test proving the injected catalog is the real mutation path.
- Modify `cmd/server/main.go` and `cmd/server/main_test.go`:
  - Resolve `projectConfigRoot` once.
  - Construct/load one filesystem catalog before registering ProjectInterop and creating project routing.
  - Inject the same instance into both.
  - Fail startup on a catalog root/integrity error; malformed individual definitions remain visible as diagnostics and do not crash unrelated integrations.
- Refactor `server/project_server.go` constructor to accept the read/write interfaces, not concrete `*project.Store`.
- Refactor `server/project_server_test.go` setup helpers to use a filesystem catalog fixture and prove mutation visibility.
- Update `docs/architecture.md` composition diagram and project ownership notes.

### Compatibility mapping

Keep these exact names and current successful shapes:

- `projectinterop_list_projects`
- `projectinterop_get_project`
- `projectinterop_create_project`
- `projectinterop_update_project`
- `projectinterop_delete_project`
- `projectinterop_get_context`

For compatibility only, old update/delete calls without a caller-supplied CAS token use dedicated catalog compatibility methods frozen in `contracts.md`: acquire the cross-process lock, reread current source, apply the operation, atomically persist/archive, and release. They preserve legacy last-write-wins behavior instead of exposing new `revision_conflict` failures. Canonical Phase 5 tools require caller-supplied revisions. Compatibility update explicitly projects the effective catalog snapshot into its frozen shape. Compatibility create returns the validated persisted user-definition request only after the dedicated catalog create succeeds; it must not project the effective snapshot when a repo overlay differs.

Do not add canonical `project.*` aliases to the ordinary integration in this phase; those belong on the dedicated Catalog MCP surface.

### TDD sequence

1. Add a composition/integration test that constructs one catalog, one ProjectInterop adapter, and one ProjectRouter; prove it fails because constructors currently allocate/capture separate state.
2. Introduce injected constructors and update registration.
3. Add stale-delete and stale-update tests after the route was first used.
4. Re-run existing ProjectInterop tests unchanged before adapting internals.
5. Run focused packages after each constructor migration.

## End-to-End Test Plan

1. Build production-like services with a real temporary config root and register the actual ProjectInterop integration.
2. Serve `/mcp` and `/mcp/{project}` from the same `http.ServeMux` used by `cmd/server/main.go` (extract a testable mux builder if needed; do not duplicate routing in tests).
3. Call `projectinterop_update_project` through `/mcp`'s public `execute` tool.
4. Read the project through `/mcp/{project}` and assert the update is visible.
5. Delete through `/mcp`; call the already-used project route again and assert not found.
6. Exclude `projectinterop_delete_project` with real `IntegrationConfig.ToolGlobs`; assert global search and execute both enforce it.
7. Run:

```bash
go test ./integrations/projectinterop ./server ./cmd/server -count=1
go test -race ./project ./integrations/projectinterop ./server ./cmd/server -count=1
```

## Anti-Cheating Audit

- Search for `project.NewStore(`. Outside tests, exactly one construction is allowed in the composition root/filesystem factory.
- Confirm ProjectInterop `Configure` does not overwrite an injected catalog.
- Confirm ProjectRouter does not retain `*Definition` or policy closures across requests.
- Ensure the cross-adapter E2E test uses HTTP/MCP calls, not direct catalog mutation followed by direct handler calls.
- Verify old success-shape assertions were not relaxed to substring checks.
- Confirm disabled integration and `ToolGlobs` are enforced by normal Switchboard lookup, not only by client-side discovery.
- Ensure no global singleton is introduced to simulate sharing.

## Completion Gate

- [ ] One catalog is constructed in production wiring.
- [ ] All existing project consumers use injected ports.
- [ ] Existing `projectinterop_*` successful response tests remain green.
- [ ] Cross-adapter update/delete visibility passes without restart.
- [ ] Disabled/filtered administration remains inaccessible.
- [ ] `rg 'project.NewStore\(' --glob '*.go'` shows only approved composition/test sites.
- [ ] Focused race tests pass.
- [ ] Architecture documentation names the catalog as the sole project authority.
- [ ] `make ci` passes for this phase commit; focused tests and RED→GREEN evidence are recorded additionally.
