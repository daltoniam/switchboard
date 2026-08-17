# Project Catalog Frozen Contracts

This document removes schema and authority ambiguity from the phase files. Implementation may add optional fields only after updating contract tests and this plan; it may not rename or repurpose fields.

## Access contract

The first release is a **local Project Catalog**:

- `/project-catalog/mcp` is disabled by default and is mounted only when `project_catalog.enabled=true` and a `project_catalog.access_token` meeting the exact 32-byte minimum below is configured.
- Every HTTP request requires `Authorization: Bearer <access_token>`; compare in constant time. Missing/invalid credentials receive HTTP 401 with `WWW-Authenticate: Bearer` before MCP dispatch. The token is a local deployment capability, not a user identity.
- Do not trust socket loopback, `Forwarded`, `X-Forwarded-For`, `X-Real-IP`, `X-Switchboard-Session-Id`, or `Mcp-Session-Id` as authorization. A local reverse proxy does not bypass the bearer check.
- Canonical writes are listed but fail with tool error `write_disabled` unless `project_catalog.writes_enabled=true`; they still require the bearer token.
- Remote/multi-principal and hosted exposure are out of scope. Hosted deployments must leave this endpoint unexposed until a separate design defines trusted caller resolution and per-project visibility; this plan adds no `CallerResolver` extension point.
- Full definitions can contain local paths, `launch.env`, unknown fields, and arbitrary extensions. They are intentionally exposed only to the local OS user in this release. Documentation must warn that project files must not contain secrets intended to be hidden from local agents.

Typed config added at top level:

```go
type ProjectCatalogConfig struct {
    Enabled       bool   `json:"enabled"`
    WritesEnabled bool   `json:"writes_enabled"`
    AccessToken   string `json:"access_token,omitempty"`
}
```

Defaults: `Enabled=false`, `WritesEnabled=false`. Startup/config validation rejects `Enabled=true` unless the token has at least 32 bytes (measured after UTF-8 encoding); operators should generate 32 random bytes and encode them as 64 lowercase hexadecimal characters. Treat the token as a secret: redact it from logs/UI responses and preserve it through config updates. This config is independent of whether the ordinary `projectinterop` integration is enabled.

## Project identity and source authority

- Canonical user filename is `<projectId>.project.json`; `projectId` is the validated filename stem.
- For a valid user file, embedded `name` is required and must equal the filename stem exactly (case-sensitive). Mismatch is `invalid_definition`; it is never indexed under the embedded name.
- Each filename stem is one catalog entry, valid or invalid. The directory cannot contain duplicate stems; case-fold collisions are rejected on case-insensitive filesystems and reported as `duplicate_project_id` diagnostics.
- Repo-local overlays never establish identity: embedded `name` may be omitted or must equal the selected user project ID. A mismatch invalidates only that resolution/root overlay, never creates another project.
- Create derives filename from the validated definition name and fails if any canonical/case-colliding path exists. Patch cannot add/change/delete `name`. External rename is observed as remove-old/add-new, subject to validation.
- For malformed user JSON where embedded name cannot be read, the filename stem remains the diagnostic/recovery identity.

## Domain types and JSON shapes

### Project summary

```json
{
  "projectId": "awesometree",
  "title": "awesometree",
  "repo": "/home/aleks/work/projects/awesometree/repo",
  "branch": "master",
  "revision": "sha256:<64-lowercase-hex>",
  "sourceRevision": "sha256:<64-lowercase-hex>",
  "definitionUri": "project://registry/projects/awesometree/definition",
  "contextUri": "project://registry/projects/awesometree/context",
  "diagnosticCount": 0
}
```

Required for valid projects: `projectId`, `title`, `revision`, `sourceRevision`, `definitionUri`, `contextUri`, `diagnosticCount`. `sourceRevision` is the hash of the canonical user-layer definition and is the optimistic concurrency token for user-layer patch/delete; `revision` remains the effective merged snapshot identity. Invalid-source entries use a separate `InvalidProjectSummary` shape `{projectId, title, rawSourceRevision, diagnosticCount, diagnosticsUri}` with no valid effective revision/definition/context URI and are returned in the catalog envelope's separate `invalidProjects` array. `rawSourceRevision` is SHA-256 of the exact malformed user-file bytes and is only a recovery CAS token, not a project revision. `repo` and `branch` are omitted when empty. All arrays are `[]`, never `null`; invalid sources never masquerade as valid projects or mint revisions.

### Source

```json
{
  "kind": "user",
  "uri": "file:///home/aleks/.config/project-interop/projects/awesometree.project.json",
  "precedence": 0
}
```

`kind` is `user` or `repository`; `precedence` is ascending low-to-high. Source URIs are local and appear only on the bearer-authenticated endpoint.

### Diagnostics envelope

```json
{
  "projectId": "awesometree",
  "diagnostics": []
}
```

`projectId` and `diagnostics` are required; the array is never null. `project://registry/projects/{projectId}/diagnostics` is listed only for invalid-source entries and is also available through its template for valid projects with warnings.

### Diagnostic

```json
{
  "severity": "error",
  "code": "invalid_definition",
  "message": "unsupported version (must be 1)",
  "sourceUri": "file:///.../awesometree.project.json",
  "path": "/version"
}
```

Required: `severity`, `code`, `message`. Optional: `sourceUri`, JSON Pointer `path`. Severity is `warning` or `error`. Messages must be value-free: never embed definition field values, context content, environment values, or credentials.

### Current definition envelope

```json
{
  "projectId": "awesometree",
  "revision": "sha256:<digest>",
  "sourceRevision": "sha256:<user-layer-digest>",
  "definition": {"version":"1","name":"awesometree"},
  "sources": [],
  "diagnostics": []
}
```

This is the default/base-repository resolution. Provenance and diagnostics reflect the current read.

### Immutable revision envelope

```json
{
  "projectId": "awesometree",
  "revision": "sha256:<digest>",
  "definition": {"version":"1","name":"awesometree"}
}
```

It contains no invocation-specific root, sources, diagnostics, or timestamps. Revision storage filename is `<64-lowercase-hex>.json`; the semantic `sha256:` prefix exists only in APIs/content.

### Resolve result

```json
{
  "projectId": "awesometree",
  "revision": "sha256:<digest>",
  "definitionUri": "project://registry/projects/awesometree/revisions/sha256%3A<digest>",
  "rootUri": "file:///worktrees/feature",
  "contextManifestUri": "project://registry/projects/awesometree/context?rootUri=file%3A%2F%2F%2Fworktrees%2Ffeature",
  "sources": [],
  "diagnostics": []
}
```

`rootUri` is omitted for default resolution. There is no `profile` or model-supplied authorization role in this release.

### Context manifest

```json
{
  "projectId": "awesometree",
  "revision": "sha256:<digest>",
  "rootUri": "file:///worktrees/feature",
  "entries": [
    {
      "path": "AGENTS.md",
      "uri": "project://registry/projects/awesometree/context/AGENTS.md?rootUri=file%3A%2F%2F%2Fworktrees%2Ffeature",
      "source": "repository",
      "mimeType": "text/markdown",
      "sizeBytes": 100
    }
  ]
}
```

`rootUri` is omitted for default resolution. Entries are sorted by path and never null.

## Worktree association

For `project.resolve` with only `rootUri`:

1. Require a canonical absolute `file://` URI naming an existing directory.
2. Run bounded `git -C <root> rev-parse --path-format=absolute --git-common-dir` and canonicalize the result.
3. For each project repository, resolve its canonical Git common directory the same way.
4. Exactly one equal common-directory match selects the project.
5. Zero matches returns `project_not_found`; multiple matches returns `ambiguous_project`.
6. Non-Git projects require explicit `projectId`.

When both ID and root are supplied, the common-directory identity must match. The root's `.project.json` is the repository overlay. No containment assumption is made because Git worktrees may live outside the base repository.

## Resource URIs and templates

- `project://registry/catalog`
- `project://registry/projects/{projectId}/definition`
- `project://registry/projects/{projectId}/diagnostics{?rootUri}`
- `project://registry/projects/{projectId}/revisions/{revision}`
- `project://registry/projects/{projectId}/context{?rootUri}`
- `project://registry/projects/{projectId}/context/{+path}{?rootUri}`

`rootUri` is explicit invocation data. Context resources re-resolve current filesystem state at that root. They are not immutable merely because the definition revision is known.

## Canonical tool schemas

All tools are always listed when the authenticated catalog endpoint is enabled. Write calls return `write_disabled` unless explicitly enabled. Each tool `outputSchema` is `oneOf` its success object and `{error: ToolError}`; expected domain failures set `IsError=true` and return the error alternative in `structuredContent`, while protocol/input-schema failures remain JSON-RPC errors.

- `project.search` input `{query?: string, cursor?: string}`; output `{projects: ProjectSummary[], nextCursor?: string}`.
- `project.resolve` input `{projectId?: string, rootUri?: string}` with at least one required; output `ResolveResult`.
- `project.validate` input `{definition: object, rootUri?: string}`; output `{valid: boolean, diagnostics: Diagnostic[]}`. The MCP handler must retain `definition` as lossless `json.RawMessage`/generic JSON and pass it to `CatalogValidator.ValidateJSON`; do not decode to typed `Definition` first. The outer arguments schema requires only that `definition` is a JSON object, while all project-schema/type/version/name/nested-field failures become successful validation output (`valid=false`) rather than JSON-RPC invalid params. Malformed JSON at the transport level remains a protocol parse/params error. If `rootUri` is present it must be an existing canonical absolute `file://` directory and is used only as the base for resolving relative `repo`, `launch.promptFile`, and context paths in the candidate; the root's `.project.json` is **not** merged, no registered-project/Git identity match is required, and validation performs no writes. Context references are checked for containment/type/existence and reported as diagnostics.
- `project.create` input `{definition: object}`; output `{project: ProjectSummary}`.
- `project.patch` input `{projectId: string, expectedSourceRevision: string, patch: object}`; output `{project: ProjectSummary}`. It operates only on valid user definitions; malformed sources return `invalid_definition` and must be recovered by `project.delete` with `expectedRawSourceRevision`, followed by normal `project.create`.
- `project.delete` input `{projectId: string, expectedSourceRevision?: string, expectedRawSourceRevision?: string}` with exactly one CAS token; output `{projectId: string, deleted: true}`. Valid sources require `expectedSourceRevision`; malformed sources require `expectedRawSourceRevision`. Recovery of a malformed source is explicit delete-by-raw-CAS followed by normal create; patch never operates on malformed JSON.

## Tool error shape

Expected domain failures are tool errors with both concise text and:

```json
{
  "error": {
    "code": "revision_conflict",
    "message": "project revision changed",
    "projectId": "awesometree",
    "expectedSourceRevision": "sha256:...",
    "currentSourceRevision": "sha256:..."
  }
}
```

Required: `code`, `message`. Optional by error: `projectId`, `expectedSourceRevision`, `currentSourceRevision`, `rootUri`, `diagnostics`. Stable codes:

- `project_not_found`
- `project_already_exists`
- `invalid_definition`
- `revision_conflict`
- `ambiguous_project`
- `invalid_root`
- `root_project_mismatch`
- `write_disabled`
- `permission_denied`
- `internal_error`

## Compatibility contract

The six `projectinterop_*` tools retain exact successful wire payloads locked before refactoring. Compatibility update/delete use dedicated catalog methods that hold the cross-process catalog lock, read the latest source, and mutate it atomically without caller CAS. They preserve legacy last-write-wins behavior. Canonical tools always require `expectedSourceRevision`.

Compatibility create returns the exact successfully persisted user-definition request shape it returns at the base revision even if the catalog also archives an effective overlaid snapshot. It must not reconstruct this payload from the effective snapshot. Implement a dedicated compatibility-create method/result (persisted user definition plus effective snapshot/event metadata) or have the adapter retain the validated request and return it only after catalog create succeeds. Compatibility update/delete likewise use dedicated compatibility methods. Add a golden where the repository overlay differs from the created user definition.

## Concurrency and persistence

- Use a cross-process lock file under the catalog root with a maintained cross-platform locking library.
- Create, canonical patch/delete, and compatibility mutations perform lock → reread/validate → compare the user-layer `sourceRevision` for canonical CAS (create checks absence; compatibility has no caller CAS) → compute effective snapshot and archive bytes → durably write the immutable archive first → durably atomic-replace the canonical user definition (or remove for delete) → fsync containing directories where supported → unlock. Effective `revision` is never a user-layer CAS token because a repository overlay may hide user changes.
- If archive persistence fails, the canonical source is untouched and the operation fails. If canonical replacement/removal fails after archive success, the operation fails, the old canonical source remains authoritative, and the harmless unreferenced immutable archive may remain. Startup reconciliation may retain such valid archives; it never promotes them to current state.
- Compatibility mutation uses the same sequence but rereads latest without caller CAS.
- Revision archives use digest-only filenames and `0600` mode.
- Project/context/revision directories use `0700`.
