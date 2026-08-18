# Project work model (AWM subset)

Switchboard holds a **minimal Agent Work Model subset** nested under the **project** tool namespace so multiple tools can share projects and work episodes.

Canonical vocabulary: `~/work/projects/agent-work-model/model/terms/`.

## Terms

| AWM term | Switchboard tools | Notes |
|---|---|---|
| **Project** | `project.list` / `get` / `create` / … | Durable id + description |
| **WorkProfile** | `project.work_profile.*` | Session blueprint (“session profile”) |
| **WorkSession** | `project.work_session.*` | Bounded episode — not an MCP transport session |
| **AgentProfile** | `project.agent_profile.*` | Eligible agent kind, not a running process |

## Storage

Under the Switchboard config root (default `~/.config/switchboard`):

```text
~/.config/switchboard/
  projects/<project_id>.project.json
  awm/
    work_profiles/<work_profile_id>.json
    agent_profiles/<agent_profile_id>.json
    work_sessions/<work_session_id>.json
```

Isolated from `project-interop`.

## Tools (main `/mcp`)

### Project (existing)

- `project.list` / `project.search`
- `project.get` / `project.resolve`
- `project.validate` / `project.create` / `project.update` / `project.delete`

### WorkProfile

- `project.work_profile.list` / `get` / `put` / `delete`

### AgentProfile

- `project.agent_profile.list` / `get` / `put` / `delete`

### WorkSession

- `project.work_session.list` (`state`, `project_id` filters)
- `project.work_session.get`
- `project.work_session.create`
- `project.work_session.transition` — `proposed` → `open` → `paused`/`closed`/`aborted`
- `project.work_session.patch`
- `project.work_session.delete`

## Invariants

- WorkSession ≠ MCP connection or host chat
- WorkProfile ≠ live WorkSession
- AgentProfile ≠ AgentInstance / AgentRun
- No credentials in these records
- Project catalog remains project authority; sessions only reference `project_id` / revision
