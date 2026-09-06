# Phase 2: Revisioned Project Catalog Core

## Goal

Turn `project/` into a transport-neutral catalog with immutable effective snapshots, canonical revisions, source provenance, diagnostics, root-based resolution, and optimistic writes. This phase deliberately exposes no new MCP surface; it stabilizes the domain contract first so every later adapter consumes identical behavior.

## BDD Success Criteria

#### Scenario: Resolve a project at its default repository

- **Given** a valid user-level project definition with no repository-local overlay
- **When** a caller resolves it by project ID
- **Then** it receives an effective snapshot with a deterministic `sha256:` revision
- **And** provenance identifies the user definition
- **And** repeated resolution without file changes returns the same revision.

#### Scenario: Resolve against an explicit worktree root

- **Given** a user definition points at a base repository and an explicit worktree contains its own `.project.json`
- **When** the caller resolves with that worktree's canonical `file://` `rootUri`
- **Then** the worktree-local overlay is applied rather than the base repository's overlay
- **And** provenance identifies both layers in precedence order
- **And** context entries resolve from the explicit worktree.

#### Scenario: Reject an unrelated or unsafe root

- **Given** a `rootUri` that is not a `file://` URI, escapes allowed paths, points through a symlink outside the supplied root, or resolves to a different project
- **When** resolution is attempted
- **Then** it fails with a typed diagnostic/error
- **And** no outside file is read.

#### Scenario: Detect an optimistic concurrency conflict

- **Given** user-layer `sourceRevision` S1 was read and the user definition changes to S2, including a change hidden by a higher-precedence repo overlay
- **When** a patch or delete supplies `expectedSourceRevision=S1`
- **Then** the operation returns `revision_conflict` with current source revision S2
- **And** the user definition remains byte-for-byte unchanged.

#### Scenario: Preserve layer ownership and unknown fields

- **Given** user and repo-local definitions include unknown top-level fields
- **When** a valid patch updates the user branch
- **Then** unknown user fields are preserved
- **And** the repo-local file is unchanged
- **And** the returned snapshot reapplies repo-local precedence.

#### Scenario: Reject project rename through patch

- **Given** an existing project
- **When** a patch attempts to change or delete `name`
- **Then** the catalog rejects it before writing
- **And** no second file or stale map key is created.

#### Scenario: Surface invalid source diagnostics

- **Given** a malformed user definition or repository-local overlay
- **When** the catalog lists or resolves projects
- **Then** the invalid source is not silently represented as a valid effective project
- **And** a diagnostic includes source URI and a value-free validation message.

## Implementation Instructions

### Required files and contracts

- Create `project/catalog.go`:
  - Define read port `Catalog` and write port `CatalogWriter`.
  - Define `ProjectID`, `Revision`, `Snapshot`, source-independent `RevisionSnapshot`, `ProjectSummary`, `InvalidProjectSummary`, `DiagnosticsEnvelope`, `Source`, `Diagnostic`, `ResolveRequest`, `SearchRequest`, `Page`, `CreateRequest`, `PatchRequest`, and `DeleteRequest`.
  - Return immutable value snapshots/deep copies; do not expose mutable store pointers.
- Create `project/revision.go` and `project/revision_test.go`:
  - Canonicalize effective and user-layer definitions with RFC 8785 JSON Canonicalization Scheme (JCS), including nested unknown `json.RawMessage`, normalized numbers/string escaping, and recursive key ordering; reject values JCS cannot represent.
  - Hash effective JCS bytes as `revision` and user-layer JCS bytes as `sourceRevision`; provenance, diagnostics, and `rootUri` belong to neither hash.
  - Format revisions as `sha256:<64 lowercase hex>`.
- Create `project/lock.go` and `project/lock_test.go` around a maintained cross-platform advisory locking library. Every create existence check, canonical patch/delete CAS, compatibility mutation, and revision archive write must execute under the same catalog-root cross-process lock; define timeout/cancellation and typed lock errors.
- Refactor `project/project.go`:
  - Rename or adapt `Store` to implement the new ports without changing the XDG layout.
  - `Load` must rebuild its index rather than retaining deleted entries.
  - Reads must observe on-disk changes; choose one of two explicit implementations and test it: reload affected files per operation, or maintain a validated cache keyed by file metadata/content digest. Do not leave a permanently stale startup snapshot.
  - Create must apply an existing repo-local overlay in the returned snapshot.
  - Patch requires `expectedSourceRevision`. Delete requires exactly one of valid `expectedSourceRevision` or malformed-file `expectedRawSourceRevision`, using the recovery semantics in `contracts.md`.
  - Preserve atomic write-rename and `0700`/`0600` modes.
- Create `project/resolve.go` and `project/resolve_test.go`:
  - Accept project ID and/or explicit canonical `file://` root URI.
  - Implement the Git-common-directory association algorithm frozen in `contracts.md`; do not assume an out-of-tree worktree is contained beneath the base repository path.
  - If only root is given, require exactly one registered repository with the same canonical Git common directory; return `project_not_found` or `ambiguous_project` otherwise.
  - Load `<root>/.project.json` for invocation-specific overlay.
  - Return provenance in low-to-high precedence order.
- Refactor `project/context.go` and tests:
  - Assemble manifest/content against `Snapshot.RootURI`, not always `Definition.Repo`.
  - Introduce a resolved context entry carrying canonical source path internally so reads do not re-resolve store-first and misreport provenance.
  - Reject lexical and symlink escapes and non-regular files.
- Extend `project/json.go` tests to preserve unknown fields through canonicalization and patches.
- Create `project/errors.go` for typed domain codes and conflict details; adapters must not string-match errors.
- Create `project/catalog_test.go` for filesystem-backed contract tests.

### Required interface shape

The exact Go spelling may follow repository conventions, but consumers must be able to depend on separate read/write ports equivalent to:

```go
type Catalog interface {
    List(context.Context, string) (Page, error)
    Search(context.Context, SearchRequest) (Page, error)
    Get(context.Context, ProjectID) (Snapshot, error)
    GetRevision(context.Context, ProjectID, Revision) (RevisionSnapshot, error)
    Diagnostics(context.Context, ProjectID, *url.URL) (DiagnosticsEnvelope, error)
    Resolve(context.Context, ResolveRequest) (Snapshot, error)
}

type CatalogValidator interface {
    ValidateJSON(context.Context, json.RawMessage, *url.URL) []Diagnostic
}

type CatalogWriter interface {
    Create(context.Context, CreateRequest) (Snapshot, error)
    Patch(context.Context, PatchRequest) (Snapshot, error)
    Delete(context.Context, DeleteRequest) error
}

type CompatibilityWriter interface {
    CreateCompatibility(context.Context, CreateRequest) (PersistedUserDefinition, Snapshot, error)
    PatchCompatibility(context.Context, ProjectID, json.RawMessage) (Snapshot, error)
    DeleteCompatibility(context.Context, ProjectID) error
}
```

Use opaque cursor strings for list/search. Sort by project ID before pagination. A cursor should bind to ordering/query and reject malformed or mismatched input.

### Revision retention

Create `$configRoot/revisions/<projectId>/<digest-hex>.json` only after a successful create/patch or resolution of a new effective revision; the filename omits the semantic `sha256:` prefix for cross-platform safety. Write atomically with `0600`. Treat it as an immutable cache/archive derived from canonical source files. Use the cross-process lock and mutation sequencing frozen in `contracts.md`:

- Existing matching content is a no-op.
- Existing different content at the same hash is an internal integrity error.
- Use the archive-first durable mutation and failure semantics frozen in `contracts.md`; inject filesystem fault points in tests to prove archive failure leaves canonical source untouched and canonical-write failure leaves old source authoritative.
- Project delete does not remove revisions, and `GetRevision` must remain readable after delete and catalog/process reconstruction.
- No API to mutate revisions is introduced.

### TDD sequence

Implement in small cycles: RFC 8785 effective/source revision determinism (nested unknown keys, equivalent numeric spellings, Unicode/escaping); immutable deep copies; provenance; explicit root; safe context; optimistic patch; optimistic delete; rename rejection; external file refresh; pagination. For each cycle, run the single test first and retain the intended RED reason before writing implementation.

## End-to-End Test Plan

Use real temporary directories and files, not an in-memory catalog fake:

1. Create `$tmp/config/projects/acme.project.json`, a base repo, and two worktrees with different `.project.json` overlays.
2. Resolve by ID and by each worktree `file://` URI. Assert different effective revisions and correct source/context provenance.
3. Restart by constructing a new catalog on the same config root. Read each recorded revision and assert identical content.
4. Read R1, externally rewrite the user file atomically to R2, then attempt patch/delete with S1. Assert conflict and unchanged file.
5. Add/delete/misname/case-collide project files outside the API and assert identity, diagnostics, and list behavior without restart. Corrupt a canonical file, prove raw-source revision stability, reject patch, delete with raw CAS, then recreate validly.
6. Create symlinks and traversal patterns escaping root; assert no content leaks.
7. Run concurrent goroutines patching one expected source revision; exactly one succeeds, others conflict, and resulting JSON validates.
8. Add `project/testhelper/catalog_worker` (or a test subprocess mode in `catalog_test.go`) and launch independent OS processes against one root. Cover concurrent same-name create, canonical CAS, compatibility last-write-wins update, archive creation, lock cancellation, and final JSON/mode/integrity validation. At most one same-name create succeeds; canonical stale writers conflict; compatibility writes serialize without corruption.
9. Add fault-injection tests for archive-write/fsync failure and canonical rename/remove failure, then delete a project, reconstruct the catalog, and prove `GetRevision` still reads every archived revision.
10. Run:

```bash
go test -race ./project -count=1
```

## Anti-Cheating Audit

- Verify `Revision` hashes canonical effective definitions, not timestamps, pointer addresses, filenames, or precomputed fixtures.
- Ensure revision files are written only after valid source resolution and are not treated as editable canonical definitions.
- Ensure tests create real user/overlay/context files and reconstruct the catalog to prove durability.
- Search for `os.ReadFile` on caller-controlled joined paths that bypasses containment checks.
- Verify symlink escape tests actually create a target outside the root.
- Confirm create existence checks, patch/delete conflicts, compatibility mutations, and archive writes are protected by the same cross-process lock; an in-process mutex or check-then-unlocked-write is insufficient.
- Inspect subprocess tests to ensure workers are independent processes with separate catalog instances, not goroutines sharing one lock object.
- Confirm `Load` and returned pointers cannot retain or mutate stale catalog state.
- Ensure invalid files yield diagnostics rather than being silently skipped.
- Confirm no workspace, session, AgentRun, PID, token, or port fields are added to the project schema.

## Completion Gate

- [ ] All specified domain types and read/write ports exist with doc comments.
- [ ] RED→GREEN evidence exists for revision, root, conflict, and escape cases.
- [ ] Revisions are deterministic and durable across catalog reconstruction.
- [ ] Explicit worktree resolution and context provenance are correct.
- [ ] External file changes are visible without restart.
- [ ] Goroutine and independent-process tests prove serialized create/CAS/compatibility/archive behavior with no corruption.
- [ ] Unknown fields and file ownership are preserved.
- [ ] `go test -race ./project -count=1` passes.
- [ ] Anti-cheating audit finds no in-memory-only authority or unsafe path seam.
- [ ] `make ci` passes for this phase commit; focused tests and RED→GREEN evidence are recorded additionally.
