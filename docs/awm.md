# Project work model (AWM subset)

Switchboard implements a **minimal Agent Work Model** subset in the internal `awm` package so projects, work profiles, work sessions, and agent profiles share one store tree.

Canonical vocabulary: `~/work/projects/agent-work-model/model/terms/`.

## Terms in package `awm`

| AWM term | Type | Tools |
|---|---|---|
| **Project** | `awm.Project` | `project_list` / `project_get` / `project_create` / `project_update` / `project_delete` (catalog surface) |
| **WorkProfile** | `awm.WorkProfile` | `project_work_profile_*` |
| **WorkSession** | `awm.WorkSession` | `project_work_session_*` |
| **AgentProfile** | `awm.AgentProfile` | `project_agent_profile_*` |

`project_*` catalog tools and `awm.Store` project APIs share `~/.config/switchboard/projects/*.project.json`.

## Storage (Switchboard config root only)

```text
~/.config/switchboard/
  projects/<project_id>.project.json    # Project (awm + catalog)
  awm/
    work_profiles/<work_profile_id>.json
    agent_profiles/<agent_profile_id>.json
    work_sessions/<work_session_id>.json
```

Never under `project-interop`.

## Project schema

```json
{ "version": "1", "name": "switchboard", "description": "optional" }
```

`project_id` is the filename stem / `name` field. Extra unknown JSON fields are tolerated on read for legacy files.

## WorkSession

May reference `project_id` (must exist), `project_revision`, `work_profile_id`, `agent_profile_ids`.  
Lifecycle: `proposed` → `open` → `paused` / `closed` / `aborted`.

## Invariants

- WorkSession ≠ MCP connection or host chat  
- WorkProfile ≠ live WorkSession  
- AgentProfile ≠ running instance  
- No credentials in these records  
