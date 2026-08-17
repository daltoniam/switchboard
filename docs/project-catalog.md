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
| Global Switchboard | `/mcp` | search/execute, `project.*` catalog tools, `project://` resources, and compatibility `projectinterop_*` |
| Project-scoped gateway | `/mcp/{project}` | current-project search/execute; no cross-project admin |

Catalog tools and resources live on the **main** `/mcp` endpoint. There is no separate bearer-gated catalog URL and no access token.

## Config

Omitted `project_catalog` defaults to **enabled** with **writes enabled**:

```json
{
  "project_catalog": {
    "enabled": true,
    "writes_enabled": true
  }
}
```

Disable either flag explicitly with `false` if needed. Canonical write tools still honor `writes_enabled`.

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

The six `projectinterop_*` tools remain on `/mcp` with their existing success shapes. They use last-write-wins compatibility methods on the same catalog.

## Migration

1. Prefer `project.search` / resources on `/mcp` for discovery.
2. Mutate with `project.create` / `project.patch` / `project.delete` (writes on by default).
3. Keep `projectinterop_*` only while older clients still call them.
