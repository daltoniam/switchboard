# Project Catalog

Project Catalog is a local, revisioned registry of project definition files. It is **not** a work session, workspace, AgentRun runtime, or task queue.

## Filesystem authority

The source of truth is JSON files:

- User layer: `$configRoot/projects/<projectId>.project.json`
- Optional repository overlay: `<root>/.project.json`
- Immutable archives: `$configRoot/revisions/<projectId>/<sha256-hex>.json`

In-memory indexes and watchers accelerate reads and notifications. They never replace the files.

`config_root` defaults to `$XDG_CONFIG_HOME/project-interop` (or `~/.config/project-interop`). Override it with the `projectinterop` integration credential `config_root`.

## Endpoints

| Surface | Path | Role |
|---|---|---|
| Global Switchboard | `/mcp` | search/execute plus compatibility `projectinterop_*` tools |
| Project-scoped gateway | `/mcp/{project}` | current-project search/execute; no cross-project admin |
| Project Catalog | `/project-catalog/mcp` | resources-first catalog; disabled by default |

## Access

`/project-catalog/mcp` mounts only when:

```json
{
  "project_catalog": {
    "enabled": true,
    "writes_enabled": false,
    "access_token": "<at least 32 UTF-8 bytes>"
  }
}
```

Every request requires `Authorization: Bearer <access_token>`. Missing/invalid tokens receive HTTP 401 with `WWW-Authenticate: Bearer`. Loopback, `X-Forwarded-For`, and `Mcp-Session-Id` are not authorization. Canonical writes still fail with `write_disabled` unless `writes_enabled` is true.

Project files can contain local paths and unknown fields. Do not store secrets in them that local agents should not see.

## Resources

- `project://registry/catalog`
- `project://registry/projects/{projectId}/definition`
- `project://registry/projects/{projectId}/diagnostics{?rootUri}`
- `project://registry/projects/{projectId}/revisions/{revision}`
- `project://registry/projects/{projectId}/context{?rootUri}`
- `project://registry/projects/{projectId}/context/{+path}{?rootUri}`

## Canonical tools

- `project.search`, `project.resolve`, `project.validate`
- `project.create`, `project.patch`, `project.delete`

Patch/delete require a CAS token (`expectedSourceRevision` or raw-source recovery). Name cannot be changed by patch.

## Compatibility

The six `projectinterop_*` tools remain on `/mcp` with their existing success shapes. They use last-write-wins compatibility methods on the same catalog. Do not remove them in this release.

## Migration

1. Keep using `projectinterop_*` if needed.
2. Enable `/project-catalog/mcp` with a local bearer token.
3. Discover via `resources/list` / `project.search`.
4. Mutate with `project.create` / `project.patch` / `project.delete` only after setting `writes_enabled=true`.
