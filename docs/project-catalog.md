# Project Catalog

Project Catalog is a local registry of named projects. Each project is identified by an **id** (`name`) and optional **description**.

It is **not** a work session, workspace, AgentRun runtime, or task queue.

## Schema

```json
{
  "version": "1",
  "name": "switchboard",
  "description": "Switchboard and related work"
}
```

- `version` must be `"1"`
- `name` is the project id (filename stem): `^[A-Za-z0-9][A-Za-z0-9._-]*$`
- `description` is optional human text

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
- Catalog mutation tools (`project_create`, `project_update`, `project_patch`, `project_delete`) **and** work-model mutations (`project_work_*` put/delete/create/transition/patch) share `writes_enabled` and the main `/mcp` surface with **no built-in bearer token**.
- If Switchboard is reachable beyond loopback, either set `"project_catalog": { "writes_enabled": false }` or put `/mcp` behind an external auth layer. Do not expose unauthenticated catalog writes on the public internet.

### Tools

- `project_list` / `project_search`
- `project_get` / `project_resolve` (resolve may materialize a content-addressed revision archive under `revisions/`)
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

Projects are also modeled as `awm.Project` in the internal AWM package and share this on-disk path with work profiles/sessions (see [awm.md](awm.md)).
