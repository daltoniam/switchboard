# Project Catalog

Project Catalog is a local, revisioned registry of project definition files. It is **not** a work session, workspace, AgentRun runtime, or task queue.

## Filesystem authority

The source of truth is JSON files:

- User layer: `$configRoot/projects/<projectId>.project.json`
- Optional repository overlay: `<root>/.project.json`
- Immutable archives: `$configRoot/revisions/<projectId>/<sha256-hex>.json`

In-memory indexes and watchers accelerate reads and notifications. They never replace the files.

`config_root` defaults to `$XDG_CONFIG_HOME/switchboard` (or `~/.config/switchboard`). It is intentionally isolated from any `project-interop` path.

## Definition shape

Projects use a multi-resource definition:

```json
{
  "version": "1",
  "name": "switchboard",
  "description": "Switchboard and related projects",
  "resources": {
    "switchboard": { "type": "repo", "path": "/path/to/switchboard", "branch": "main" },
    "architecture": { "type": "file", "repo": "switchboard", "path": "docs/architecture.md" },
    "project-specs": {
      "type": "files",
      "repo": "switchboard",
      "include": ["docs/specs/**/*.md"]
    }
  }
}
```

Resource types: `repo`, `file`, `files`. File/files resources may reference a repo resource id; Switchboard validates those refs after schema checks.

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

Minimal surface:

- `project://registry/catalog`
- `project://registry/projects/{project}`
- `project://registry/projects/{project}/resources`
- `project://registry/projects/{project}/resources/{resource}`
- `project://registry/projects/{project}/resources/{resource}/content`
- `project://registry/projects/{project}/resources/{resource}/files/{urlencoded path}`

Compatibility / advanced:

- `project://registry/projects/{project}/definition` (alias of project envelope)
- `project://registry/projects/{project}/diagnostics{?rootUri}`
- `project://registry/projects/{project}/revisions/{revision}`
- `project://registry/projects/{project}/context{?rootUri}`
- `project://registry/projects/{project}/context/{+path}{?rootUri}`

Reading a repo resource returns metadata JSON `{id,type,path,branch}`.  
Reading a file resource `/content` returns text.  
Reading a files resource `/content` returns a manifest with progressive file URIs.

## Canonical tools

Clean names:

- `project.list`, `project.get`, `project.create`, `project.update`, `project.delete`

Transition aliases (same handlers):

- `project.search` → `project.list`
- `project.patch` → merge-patch update
- `project.resolve`, `project.validate` remain available

`project.update` accepts `{projectId, expectedSourceRevision, patch|definition}`.  
Patch/delete require a CAS token (`expectedSourceRevision` or raw-source recovery). Name cannot be changed by patch.

## Compatibility

The six `projectinterop_*` tools remain on `/mcp` with their existing success shapes. They use last-write-wins compatibility methods on the same catalog.

## Migration

1. Prefer `project.list` / `project.get` / resources on `/mcp` for discovery.
2. Mutate with `project.create` / `project.update` / `project.delete` (writes on by default).
3. Keep `project.search` / `project.patch` aliases while older clients still call them.
4. Keep `projectinterop_*` only while older clients still call them.
