# Project Catalog

Project Catalog is a local registry of named projects. Each project is identified by an **id** (`name`) and optional **description**.

It is **not** a work session, workspace, AgentRun runtime, or task queue.

## Schema

```json
{
  "version": "1",
  "name": "switchboard",
  "description": "Switchboard and related work",
  "known_resource_ids": ["awesometree.switchboard.repo"]
}
```

- `version` must be `"1"`
- `name` is the project id (filename stem): `^[A-Za-z0-9][A-Za-z0-9._-]*$`
- `description` is optional human text
- `known_resource_ids` is the typed AWM observation of Resource IDs (optional)

## Filesystem authority

Source of truth:

- `$configRoot/projects/<name>.project.json`
- Immutable archives: `$configRoot/revisions/<name>/<sha256-hex>.json`

Default `$configRoot` is `~/.config/switchboard` (or `$XDG_CONFIG_HOME/switchboard`).  
This is **isolated** from any `project-interop` config directory.

## Endpoint

Catalog tools and `project://` resources are on the main `/mcp` endpoint (enabled by default).

### Writes / exposure

- `project_catalog.writes_enabled` defaults to **on** (`null`/`omitted` ⇒ true) for local development.
- Catalog mutation tools (`project_create`, `project_update`, `project_patch`, `project_delete`) **and** work-model mutations (`project_work_*`, `project_resource_*`, and `project_resource_binding_*`) share `writes_enabled` and the main `/mcp` surface with **no built-in bearer token**.
- TCP listening defaults to loopback (`127.0.0.1`). Use `--listen-host` only when you intentionally want another bind address.
- `--grpc-socket` serves native AWM gRPC only. It does not expose `/mcp` or the web UI.
- If Switchboard is reachable beyond loopback, either set `"project_catalog": { "writes_enabled": false }` or put `/mcp` behind an external auth layer. Do not expose unauthenticated catalog writes on the public internet.

### Tools

- `project_list` / `project_search`
- `project_get` / `project_resolve` (both may materialize a content-addressed revision archive under `revisions/`)
- `project_validate`
- `project_create`
- `project_update` / `project_patch`
- `project_delete`

### Resources

- `project://registry/catalog`
- `project://registry/projects/{id}`
- `project://registry/projects/{id}/definition`
- `project://registry/projects/{id}/revisions/{revision}`

## Web UI

Browse projects at `/projects` (list + detail).

## Related

- Work profiles, work sessions, agent profiles (nested `project_*` tools): [awm.md](awm.md)

Projects are also modeled as `awm.Project` in the internal AWM package and share this authority with resources, resource bindings, work profiles, and work sessions (see [awm.md](awm.md)).
