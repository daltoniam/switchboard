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

### Tools

- `project.list` / `project.search`
- `project.get` / `project.resolve`
- `project.validate`
- `project.create`
- `project.update` / `project.patch`
- `project.delete`

### Resources

- `project://registry/catalog`
- `project://registry/projects/{id}`
- `project://registry/projects/{id}/definition`
- `project://registry/projects/{id}/revisions/{revision}`

## Web UI

Browse projects at `/projects` (list + detail).
