# Project Catalog MCP Implementation Plan

> **For Goose:** Implement this plan phase-by-phase with parent-side `delegate` calls. Each phase is a self-contained delivery and review unit. Children cannot spawn subagents. Require RED→GREEN evidence, independent verification, and an allowlisted commit for every phase.

**Outcome:** Switchboard exposes one revisioned, resources-first Project Catalog over MCP, uses that same catalog for the existing `projectinterop_*` compatibility tools and project-scoped gateway, and speaks both the MCP 2026-07-28 stateless protocol and the currently supported legacy protocol during migration.

**Architecture:** The `project/` package becomes the transport-neutral catalog core and filesystem adapter. A dedicated MCP adapter at `/project-catalog/mcp` exposes canonical `project://registry/...` resources and typed `project.*` tools. `integrations/projectinterop/` and `server/project_server.go` become compatibility/consumer adapters over the same injected catalog; neither owns another store.

**Tech stack:** Go 1.26.6, the stable upstream `github.com/modelcontextprotocol/go-sdk` release—or narrowly pinned upstream-compatible patch—selected by Phase 1's complete `2026-07-28` feasibility gate, JSON Schema 2020-12, Streamable HTTP, RFC 6570 resource templates, RFC 7396 merge patch, SHA-256 content revisions, XDG filesystem storage.

---

## Base branch choice

| Candidate | SHA | Decision |
|---|---|---|
| `main` in `/home/aleks/work/projects/switchboard/worktrees/slack-official-mcp` | `85f3ed65473e382e80b12e2abfb7d2e72e2155f9` | **Selected.** This is current `origin/main` and already contains merged Project Interop work from PR #215. |
| `aleks/project-interop` | `1c106b907221bc3eeff218368a453f04d3ad9c7e` | Rejected as base because it is one commit behind `main`; its feature is already merged. |
| `/home/aleks/work/projects/switchboard/repo` | `6bb7576b8eae0300c55954798ffec8e1fae9f89f` | Rejected because it is an older unrelated branch with user changes. |

Plan worktree: `/home/aleks/work/projects/switchboard/worktrees/impl-project-catalog-mcp` on `impl/project-catalog-mcp`.

## Current-state summary

Evidence at the selected base:

- `project/project.go` has a typed v1 definition, RFC 7396 patching, atomic user-file writes, and user + repository-local merging.
- `project/json.go` preserves unknown top-level user fields.
- `project/context.go` assembles context manifests and contains lexical path checks, but project snapshots have no revision, source provenance, immutable history, or explicit worktree-root resolution contract.
- `integrations/projectinterop/projectinterop.go` exposes six functional `projectinterop_*` tools but constructs its own `project.Store` in `Configure`.
- `cmd/server/main.go` constructs a second `project.Store` for `server.ProjectRouter`.
- `server/project_server.go` caches definitions and reimplements search/execution instead of using `server.Server`'s validation, global `ToolGlobs`, retry, breaker, result processing, history, and pin pipeline.
- Project data is exposed only through tools; no Project Catalog MCP resources or resource templates exist.
- `go.mod` pins `github.com/modelcontextprotocol/go-sdk v1.5.0`. Stable v1.7.0 is available and contains `server/discover`, `resultType`, cache fields, `subscriptions/listen`, resource subscriptions, and backward-compatible legacy handling.
- Focused baseline passes: `go test ./project ./integrations/projectinterop ./server ./cmd/server`.

## Scope boundaries

### In scope

- A single injected Project Catalog authority inside one Switchboard process.
- Effective project snapshots with revisions, provenance, diagnostics, and explicit `rootUri` resolution.
- Immutable definition snapshots addressable by revision.
- Canonical Project Catalog resources, templates, and typed tools.
- Optimistic concurrency for canonical patch/delete operations.
- Deterministic listing, pagination, private cache metadata, and MCP change notifications.
- MCP 2026-07-28 support with compatibility for pre-2026 clients.
- A bearer-authenticated, default-disabled first-release security boundary and explicit default-off canonical writes, as frozen in [`contracts.md`](./contracts.md).
- Existing `projectinterop_*` tool names and successful result shapes during migration.
- Project-scoped Switchboard execution through the normal global execution controls.

### Out of scope

- Work sessions, workspaces, AgentRuns, agent processes, task queues, or shared runtime state.
- Making Switchboard the owner of Awesometree state.
- An MCP Tasks extension for normal catalog CRUD; all planned catalog operations are bounded and synchronous.
- Remote/hosted Project Catalog exposure, OAuth, trusted-edge caller resolution, and per-project visibility. The endpoint is disabled by default and requires an explicit bearer capability when enabled; hosted/multi-principal exposure remains out of scope.
- Automatic clone, checkout, worktree creation, or repository mutation.
- A project editor UI.
- Deleting context, repository files, workspaces, sessions, or immutable revisions when deleting a user-level project.
- Removing legacy `projectinterop_*`, `/mcp/{project}`, or project-scoped `project_*` tools in this delivery. They are deprecated only after replacement evidence exists.

## Global constraints

1. **One authority:** the composition root constructs one catalog. The integration, catalog MCP adapter, and project-scoped router receive the same object.
2. **Filesystem remains authoritative:** in-memory indexes and watchers may accelerate reads or notifications but may not become the source of truth.
3. **Stateless MCP:** no current project, revision, role, or root may be inferred from an MCP transport connection or `Mcp-Session-Id`.
4. **Frozen contracts:** JSON shapes, errors, access policy, worktree identity, root-specific context URIs, compatibility projections, and cross-process locking are normative in [`contracts.md`](./contracts.md).
5. **Explicit context:** `project.resolve` accepts `projectId` and/or an explicit `rootUri`. Roots are not added as a new dependency because MCP Roots is deprecated in 2026-07-28.
6. **Revision semantics:** `revision = "sha256:" + lowercase hex(SHA-256(canonical effective definition JSON))`. Immutable revision resources return only the effective definition envelope; invocation-specific root/provenance remains in the resolve result.
7. **Write ownership:** canonical create/patch/delete mutate only `$configRoot/projects/<name>.project.json`. Repo-local `.project.json`, context, and revision snapshots are never changed or deleted by those operations.
8. **Optimistic concurrency:** canonical patch/delete require `expectedSourceRevision`; mismatch returns a structured `revision_conflict` tool error without writing.
9. **Name immutability:** patches may not change `name`; rename is not smuggled through merge patch.
10. **Private caching:** project lists and reads return `cacheScope: "private"`. Current aliases use short TTLs; immutable revisions use long TTLs.
11. **Monotonic authorization:** global integration enablement and `ToolGlobs` are a hard upper bound. Effective project policy may narrow but never widen global authority; role-based execution policy is deferred to future trusted run-bound endpoints.
12. **Compatibility:** old tools retain names and current success payloads. New canonical tools use typed `structuredContent` and JSON Schema 2020-12.
13. **TDD:** write focused failing tests, prove the intended failure, implement minimally, then run focused and broader gates.
14. **No parse churn:** operate on parsed Go values; do not add marshal→unmarshal cycles to call JSON-only helpers in the execution pipeline.
15. **Security:** canonicalize and contain all `file://` roots/context paths, reject symlink escape, preserve file modes `0700`/`0600`, and never log file content or credentials.
16. **CI per phase:** every phase commit must pass `make ci`; focused tests are additional evidence, not substitutes. Record RED command/failure and GREEN commands in the phase PR/commit notes or `docs/plans/project-catalog-mcp/evidence/phase-NN.md`.

## Canonical surface frozen by this plan

### Endpoint

- Dedicated catalog: `/project-catalog/mcp`
- Existing global Switchboard: `/mcp`
- Existing project scope: `/mcp/{project}` retained as compatibility surface

### Resources

- `project://registry/catalog`
- `project://registry/projects/{projectId}/definition`
- `project://registry/projects/{projectId}/diagnostics{?rootUri}`
- `project://registry/projects/{projectId}/revisions/{revision}`
- `project://registry/projects/{projectId}/context{?rootUri}`
- `project://registry/projects/{projectId}/context/{+path}{?rootUri}`

### Canonical tools

- Read-only: `project.search`, `project.resolve`, `project.validate`
- Administrative: `project.create`, `project.patch`, `project.delete`

Canonical administration tools remain listed but return `write_disabled` by default; mutations are enabled only by independent top-level `project_catalog.writes_enabled=true`. The existing ordinary ProjectInterop integration remains separately governable by integration enablement and `ToolGlobs`.

## Phase overview

| Phase | Goal | Depends on |
|---|---|---|
| [Phase 1: MCP 2026-07-28 wire foundation](./phase-01-mcp-2026-wire-foundation.md) | Upgrade and wire-lock modern stateless MCP while preserving legacy clients | None |
| [Phase 2: Revisioned Project Catalog core](./phase-02-revisioned-catalog-core.md) | Add immutable snapshots, provenance, diagnostics, explicit-root resolution, and optimistic writes | Phase 1 |
| [Phase 3: One catalog in the composition root](./phase-03-shared-catalog-composition.md) | Inject one catalog into all existing project consumers and preserve old tools | Phase 2 |
| [Phase 4: Resources-first Catalog MCP](./phase-04-catalog-resources.md) | Expose canonical project resources and resource templates at a dedicated endpoint | Phase 3 |
| [Phase 5: Canonical typed project tools](./phase-05-canonical-tools.md) | Add search, resolve, validate, and guarded administration with structured output | Phase 4 |
| [Phase 6: Change subscriptions and external consistency](./phase-06-change-subscriptions.md) | Emit correct list/resource updates for API and out-of-process file changes | Phase 5 |
| [Phase 7: Project-scoped gateway convergence](./phase-07-scoped-gateway-convergence.md) | Use fresh catalog snapshots and the normal Switchboard policy/execution pipeline | Phases 3 and 5 |
| [Phase 8: Migration, conformance, and release gates](./phase-08-migration-conformance.md) | Prove compatibility and publish the supported contract and rollout path | Phases 6 and 7 |

## Requirement traceability

| Requirement | Primary phase(s) |
|---|---|
| Shared catalog core | 2, 3 |
| Resources-first MCP representation | 4 |
| `project.search`, `project.resolve`, `project.validate` | 5 |
| Administrative tools and optimistic writes | 2, 5 |
| Revisions, provenance, diagnostics | 2 |
| Explicit workspace/worktree root | 2, 5 |
| Current MCP stateless protocol | 1, 4, 6 |
| Existing tool compatibility | 3, 8 |
| Resource/list update notifications | 6 |
| Global policy remains upper bound; project policy narrows it | 7 |
| No project/session conflation | All; explicitly audited in 2, 4, 5 |
| End-to-end and anti-cheating evidence | Every phase; cumulative in 8 |

## Completion rule

The plan is complete only when:

- Every phase completion gate is checked with retained RED→GREEN and public-boundary evidence.
- A modern client negotiates `2026-07-28` through `server/discover`, while a legacy client still completes initialize and uses existing tools.
- Catalog resources, templates, canonical tools, optimistic conflicts, and subscriptions pass over real Streamable HTTP production wiring.
- All project consumers observe the same mutation without process restart.
- Project-scoped search and execution demonstrably enforce global configuration plus project narrowing and use the shared executor; role input is rejected/ignored for authorization and cannot widen access.
- Existing `projectinterop_*` success-shape tests remain green.
- The anti-cheating audit finds no second store, stale captured definition, hard-coded fixtures in production handlers, in-memory-only revision truth, or test-only protocol path.
- `make ci` passes from a clean worktree.
