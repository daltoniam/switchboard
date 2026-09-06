# Phase 7: Project-Scoped Gateway Convergence

## Goal

Make `/mcp/{project}` a thin project-policy adapter over the normal Switchboard search/execution pipeline. Remove stale captured definitions, cross-project administration from scoped endpoints, and policy bypasses while preserving the URL shape and project context behavior for existing clients.

## BDD Success Criteria

#### Scenario: Global policy is a hard upper bound

- **Given** a tool is allowed by project policy but excluded by global `IntegrationConfig.ToolGlobs`
- **When** a client searches or executes through `/mcp/{project}`
- **Then** the tool is neither advertised nor executable.

#### Scenario: Project policy narrows global authority

- **Given** a globally available tool denied by the effective project policy
- **When** the client searches or executes it
- **Then** it is hidden/rejected
- **And** no model-supplied role parameter changes execution authority. Role-specific execution is explicitly deferred to future run-bound endpoints with trusted role bindings.

#### Scenario: Scoped execution uses the normal pipeline

- **Given** a permitted integration tool with schema, retry, breaker, result processing, limits, metrics, history, and pin behavior
- **When** it is executed through global and project-scoped endpoints
- **Then** both pass through the same underlying dispatcher/pipeline
- **And** only project defaults/policy differ.

#### Scenario: Project defaults are deterministic and subordinate to explicit arguments

- **Given** multiple matching default patterns
- **When** a scoped tool executes
- **Then** default precedence is deterministic
- **And** explicit agent arguments win
- **And** app-session defaults follow the explicitly documented precedence frozen by tests.

#### Scenario: Definition changes take effect on the next request

- **Given** a scoped endpoint has already served requests
- **When** the project's current revision changes or is deleted
- **Then** the next request uses the new snapshot or returns not found
- **And** no process restart or MCP reconnection is necessary.

#### Scenario: Scoped endpoint has no cross-project administration

- **Given** a client connected to `/mcp/acme`
- **When** it lists tools
- **Then** cross-project create/patch/delete tools are absent
- **And** current-project introspection/context remains available through resources or compatibility read tools.

#### Scenario: Scripts cannot bypass scope

- **Given** a script mode call targets a denied tool
- **When** project-scoped execute supports scripts
- **Then** every internal `api.call` passes through the same effective policy and defaults
- **Or** scripts remain explicitly unsupported with a tested error until safe convergence is implemented.

## Implementation Instructions

### Required files

- Create `server/executor.go` and `server/executor_test.go` by extracting from `server/server.go` only the shared internal services needed for:
  - Effective tool lookup using enabled integration plus global `ToolGlobs`.
  - Schema validation.
  - Retry budget/circuit breaker.
  - Tool execution.
  - Markdown/compaction/columnarization/max-byte processing.
  - Metrics/history/pins hooks already present in the global server.
- Create or refactor `server/search_service.go` so global and project-scoped search share one indexed catalog plus an additional predicate. Avoid duplicating TF-IDF/scoring code.
- Refactor `server/server.go` handlers to call these shared services without changing global behavior.
- Refactor `server/project_server.go`:
  - Resolve a fresh catalog snapshot per request using the bound project ID.
  - Compute effective project rule per request; role-specific execution remains unavailable in this phase.
  - Apply project rules as an additional narrowing predicate after global policy.
  - Delegate execution to shared executor.
  - Remove `addProjectManagementTools`; canonical administration lives on `/project-catalog/mcp`, compatibility administration on global ProjectInterop integration.
  - Keep or replace `project_context`, `project_tools`, and `project_defaults` with current-project-only compatibility reads; they may not accept another project name.
- Fix `project/scope.go` and tests:
  - Replace nondeterministic map iteration for matching defaults with explicit specificity ordering (exact > more literal characters/fewer wildcards > lexical tie-break) or migrate the schema to an ordered rule list with backward-compatible decoding. Freeze one choice in tests.
  - Define monotonic merge: global policy hard-bounds project; repo-local policy cannot widen user-level allow; deny is additive.
- Expand `server/project_server_test.go` to cover parity, global ToolGlobs, fresh snapshots, current-project-only introspection, scripts, and deterministic defaults.
- Update `docs/architecture.md`, `docs/response-optimizations.md`, and `docs/tool-search.md` for the shared pipeline.

### Explicit precedence

Freeze and test:

1. Global deployment/integration permission decides whether a tool can ever be visible/called.
2. Effective project rule narrows that set.
3. Role-specific execution is unavailable in this release; model-supplied role values are ignored/rejected for authorization.
4. Project defaults are injected.
5. Switchboard app-session defaults are injected only for still-missing keys.
6. Explicit call arguments override all defaults.

If current global precedence differs, record and migrate it explicitly rather than silently changing behavior.

### TDD sequence

1. Add a failing E2E test demonstrating current global `ToolGlobs` bypass through ProjectRouter.
2. Extract global executor under existing tests without behavior changes.
3. Route project execute through it and turn the bypass test green.
4. Extract shared search and prove global/project scoring parity over the same permitted set.
5. Add stale update/delete tests after first route use.
6. Add deterministic overlapping-default tests.
7. Remove scoped cross-project management only after canonical/compatibility alternatives are green.
8. Decide script support through tests; never enable partial bypass-prone script execution.

## End-to-End Test Plan

1. Build real Services with two mock external integration adapters implementing actual `Integration` boundaries; configure enablement, credentials, and `ToolGlobs` through real config service behavior.
2. Serve global, catalog, and project-scoped handlers through the production mux.
3. Compare global/project `search` results and execute allowed/denied tools over Streamable HTTP.
4. Use an integration fixture that fails transiently then succeeds; prove project-scoped calls use retry/breaker/metrics rather than direct integration execution.
5. Return large structured JSON and verify identical result processing/limits globally and scoped.
6. Patch/delete project after first route call and verify immediate behavior.
7. Attempt cross-project CRUD and arbitrary role escalation through scoped tools; assert absence/rejection.
8. Exercise script denial or fully scoped execution according to the frozen decision.
9. Run:

```bash
go test -race ./project ./server ./cmd/server -run 'ProjectRouter|Executor|Scoped' -count=1
go test ./server -run TestSmoke_SearchResponseShape -count=1
```

First-party config/catalog must be real. External integrations may be deterministic test adapters because the boundary under test is Switchboard routing/policy, not third-party APIs.

## Anti-Cheating Audit

- Search `server/project_server.go` for direct `integration.Execute`; it must be absent.
- Verify both search and execute apply global `ToolAllowed`, not only one path.
- Confirm ProjectRouter stores only project ID/dependencies, never captured `*Definition`, `ScopeRule`, or stale MCP server per definition revision.
- Ensure retries/metrics assertions observe real executor effects, not mocked method-call counters only.
- Confirm result processing parity asserts output, max-size behavior, and metrics.
- Inspect role source; model-supplied role must not authorize execution.
- Confirm scoped management tools cannot target another name and canonical alternatives are tested first.
- Verify default ordering does not depend on Go map iteration.
- Ensure scripts cannot call a lower-level unscoped executor.

## Completion Gate

- [ ] RED bypass test was observed before convergence.
- [ ] Global and scoped handlers use one executor and one search implementation.
- [ ] Global policy and project narrowing are enforced in order; role-specific execution is absent and cannot be selected by model input.
- [ ] Deterministic defaults and explicit-argument precedence pass.
- [ ] Fresh revision/delete behavior passes after route reuse.
- [ ] Cross-project administration is absent from scoped endpoints.
- [ ] Script behavior is explicit and bypass-safe.
- [ ] Smoke and focused race tests pass.
- [ ] Audit finds no direct project-router integration execution or captured definition.
- [ ] `make ci` passes for this phase commit; focused tests and RED→GREEN evidence are recorded additionally.
