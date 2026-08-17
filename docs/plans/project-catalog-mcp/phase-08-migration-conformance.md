# Phase 8: Migration, Conformance, and Release Gates

## Goal

Finish the delivery with cumulative modern/legacy protocol evidence, compatibility locks, operator documentation, migration telemetry, and full repository gates. No legacy surface is deleted in this phase; removals require a later release after observed migration.

## BDD Success Criteria

#### Scenario: Existing agents continue to use ProjectInterop tools

- **Given** a client using the six preexisting `projectinterop_*` tool names and arguments
- **When** it connects through global Switchboard
- **Then** its successful operations and result shapes remain compatible
- **And** all changes are visible through canonical resources/tools.

#### Scenario: A modern client uses the complete catalog flow

- **Given** an empty temporary catalog and modern MCP client
- **When** an authenticated caller with canonical writes explicitly enabled creates, lists, resolves, subscribes, patches, reads immutable/current resources, validates, and deletes a project
- **Then** every operation succeeds with the frozen schemas and notifications
- **And** a caller with a missing/invalid bearer token is denied before catalog dispatch.

#### Scenario: A project-scoped client respects converged policy

- **Given** the project created through canonical tools
- **When** a client connects to `/mcp/{project}`
- **Then** it sees the effective permitted tool set and context
- **And** global denials and project denials remain enforced.

#### Scenario: Restart preserves catalog truth

- **Given** projects, revisions, and context created before process shutdown
- **When** Switchboard restarts against the same config root
- **Then** current and immutable resources are consistent
- **And** no transport/app session is needed to recover project state.

#### Scenario: Current and legacy clients coexist

- **Given** one modern 2026-07-28 client and one legacy initialized client
- **When** both operate concurrently
- **Then** neither changes the other's tool/resource lists through connection state
- **And** both receive protocol-valid results.

#### Scenario: Migration is diagnosable

- **Given** clients use canonical or compatibility surfaces
- **When** operators inspect structured logs/metrics
- **Then** they can distinguish canonical reads/writes, compatibility tool use, revision conflicts, invalid source diagnostics, and dropped/coalesced invalidations
- **And** no project contents or credentials are recorded.

## Implementation Instructions

### Required files

- Create `server/project_catalog_e2e_test.go` as the cumulative public-boundary harness using real production mux, filesystem catalog, official modern client, raw JSON-RPC assertions, and legacy client.
- Add or update `server/testdata/` wire goldens only for stable protocol envelopes. Provide an explicit update flag; never regenerate silently during normal tests.
- Modify `integrations/projectinterop/projectinterop_test.go` to make old success payloads a compatibility wire lock.
- Add `docs/project-catalog.md` covering:
  - Project ontology and non-goals.
  - Filesystem authority and precedence.
  - Endpoint/resources/templates/tools.
  - Revision/provenance/conflict semantics.
  - Modern MCP behavior and legacy compatibility.
  - Security/cache/notification behavior.
  - Migration examples from `projectinterop_*` to canonical resources/tools.
- Update `README.md`, `docs/architecture.md`, and `docs/adapter-reference.md` with links and ownership boundaries.
- Update `metrics.go`, `metrics_test.go`, and metrics serialization docs with these exact bounded instruments and increment points:
  - Counter `project_catalog_resource_reads_total{outcome=success|not_found|denied|error}` increments once per `resources/read` completion.
  - Counter `project_catalog_tool_calls_total{tool=search|resolve|validate|create|patch|delete,outcome=success|domain_error|denied|error}` increments once per canonical tool completion.
  - Counter `projectinterop_compat_calls_total{tool=list|get|context|create|update|delete,outcome=success|error}` increments once per compatibility completion.
  - Counter `project_catalog_revision_conflicts_total` increments once per canonical source-CAS conflict.
  - Gauge `project_catalog_invalid_sources` is set after each catalog reconciliation to current invalid-source count.
  - Counter `project_catalog_invalidations_total{outcome=emitted|coalesced|dropped}` increments at the event fanout decision.
  - Structured event names are exactly `project_catalog.read`, `project_catalog.tool`, `project_catalog.compat`, `project_catalog.conflict`, `project_catalog.diagnostic`, and `project_catalog.invalidation`, with bounded outcome/kind fields only.
  - Do not use project IDs, roots, paths, definitions, or tokens as metric labels or log fields. Add public-boundary assertions that each operation increments only its expected instruments.
- Add a release note/changelog entry if the repository's release process has an established location; do not invent a changelog file otherwise.

### Compatibility and rollout

- Keep global `projectinterop_*` tools enabled according to existing config.
- Mark them as compatibility aliases in descriptions only after canonical surface is shipped; do not break search discoverability abruptly.
- Keep `/mcp/{project}` URL stable.
- Add `/project-catalog/mcp` as the recommended registry endpoint.
- Do not alias MCP Registry terminology; call the feature Project Catalog consistently.
- Do not remove legacy initialize support while supported Goose/Hermes/Crush clients still require it.
- Record future removal criteria: at least one release with compatibility-use telemetry available and all first-party clients migrated. This plan does not schedule deletion.

### Verification matrix

| Surface | Modern client | Legacy client | Restart | Access boundary | Concurrency |
|---|---:|---:|---:|---:|---:|
| Catalog resources | Required | Best effort if supported | Required | Invalid token denied | Read consistency |
| Canonical tools | Required | Required through SDK fallback if possible | Required | Required | Patch conflict |
| Compatibility tools | Required | Required | Required | Global config | Existing behavior |
| Project-scoped gateway | Required | Required | Required | Global + project policy | Fresh snapshot |
| Subscriptions | Required | SDK legacy compatibility only | Reconnect | Filtered | Multiple listeners |

### Final test order

1. Focused catalog domain tests.
2. Catalog MCP E2E tests.
3. Project-scoped convergence tests.
4. Existing smoke and adapter parity tests.
5. Race suite.
6. Full CI.

Do not hide a flaky test with retries. Fix nondeterminism in event timing using explicit acknowledgments/deadlines and bounded polling of observable state.

## End-to-End Test Plan

The cumulative test must:

1. Create an isolated XDG/config root and real repo/worktree/context layout.
2. Start the actual `cmd/server` mux construction in-process with bearer authentication and project writes explicitly enabled; separately exercise missing/invalid token rejection from direct and local-proxy-style requests with forged forwarding headers.
3. Connect a modern official client; call `server/discover`, list resources/templates/tools, and open subscriptions.
4. Create a project canonically and follow returned resource links.
5. Resolve against a worktree; verify provenance/context.
6. Patch with expected source revision, race a stale patch—including a user-layer change hidden by a repo overlay—and verify conflict/update notifications.
7. Call the six compatibility tools and compare frozen successful shapes.
8. Exercise `/mcp/{project}` global/project policy and normal execution pipeline.
9. Stop and reconstruct the server/catalog on the same files; reread current and immutable revisions.
10. Delete canonically; verify no context/revision/repo-local cascade, reconstruct the process/catalog, and read pre-delete immutable revision URIs successfully.
11. Connect a legacy client in the same test or a dedicated companion test and call search/execute.
12. Through raw wire JSON, assert every empty collection is `[]`, never `null`: catalog `projects`/`invalidProjects`, current definition `sources`/`diagnostics`, resolve `sources`/`diagnostics`, standalone diagnostics-envelope `diagnostics`, context `entries`, search `projects`, validation `diagnostics`, and tool-error `diagnostics` when present.
13. Assert the exact observability counters/events for canonical read/write, compatibility use, conflict, invalid source, and emitted/coalesced/dropped invalidations without sensitive labels.
14. Run:

```bash
go test ./project ./integrations/projectinterop ./server ./cmd/server -count=1
go test ./server -run TestSmoke_SearchResponseShape -count=1
go test -race -coverprofile=coverage.out ./...
make ci
```

## Anti-Cheating Audit

- Confirm cumulative tests use production mux and public MCP calls; no direct handler invocation may stand in for E2E steps.
- Inspect test fixtures for hard-coded expected project results disconnected from files created during setup.
- Ensure restart reconstructs new server/catalog objects and does not reuse pointers.
- Verify the endpoint is absent by default; enabled startup rejects a missing/weak configured token; access tests use valid, missing, and invalid bearer tokens from direct and local-proxy-style requests; forged forwarding/app-session headers must not bypass denial; config/log/web serialization redacts the token.
- Check compatibility tests assert complete successful payload shapes, not only names/status.
- Ensure notification assertions wait for acknowledgment and then validate subscription IDs and reread truth.
- Search for skipped tests, environment-gated PASS, sleeps without bounded observable conditions, and broad retries.
- Search production code for a second `project.NewStore`, captured project definitions, direct scoped `integration.Execute`, and test-only branches.
- Confirm documentation does not claim MCP Tasks, OAuth, work sessions, or AgentRuns are implemented by the catalog.
- Review logs/metrics tests for absence of definition/context values and credential material.

## Completion Gate

- [ ] Requirement-to-phase traceability in `index.md` is fully satisfied.
- [ ] Modern end-to-end catalog lifecycle passes.
- [ ] Legacy protocol and six compatibility tools pass.
- [ ] Restart, authorization, optimistic concurrency, policy, and subscription scenarios pass.
- [ ] Wire/schema goldens update only through explicit opt-in and are reviewed.
- [ ] Documentation and migration guidance are complete and terminology is consistent.
- [ ] No legacy surface is prematurely removed.
- [ ] Final anti-cheating searches are clean.
- [ ] `make ci` passes from a clean worktree.
