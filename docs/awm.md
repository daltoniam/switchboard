# Agent Work Model (minimal) in Switchboard

Switchboard holds a **minimal subset** of the [Agent Work Model](https://github.com/) vocabulary so multiple tools can share projects and work episodes.

Canonical AWM source (reviewed): `~/work/projects/agent-work-model/model/terms/`.

## Terms implemented

| AWM term | Switchboard role | Notes |
|---|---|---|
| **Project** | Existing Project Catalog (`project.*`) | Durable id + description under `~/.config/switchboard/projects/` |
| **WorkProfile** | `awm.work_profile.*` | Session **blueprint** (product alias: “session profile”) |
| **WorkSession** | `awm.work_session.*` | Bounded work episode — **not** an MCP transport session |
| **AgentProfile** | `awm.agent_profile.*` | Eligible agent **kind**, not a running process |

Not implemented yet: AgentRun, ResourceBinding, Workspace, Task, Artifact, HostConversation, Principal.

## Storage

All AWM records live under the Switchboard config root (default `~/.config/switchboard`):

```text
~/.config/switchboard/awm/
  work_profiles/<work_profile_id>.json
  agent_profiles/<agent_profile_id>.json
  work_sessions/<work_session_id>.json
```

Never under `~/.config/project-interop`.

## MCP tools (main `/mcp`)

### WorkProfile (session blueprint)

- `awm.work_profile.list` / `get` / `put` / `delete`

### AgentProfile

- `awm.agent_profile.list` / `get` / `put` / `delete`

### WorkSession

- `awm.work_session.list` (optional `state`, `project_id`)
- `awm.work_session.get`
- `awm.work_session.create` — may set `project_id`, `project_revision`, `work_profile_id`, `agent_profile_ids`
- `awm.work_session.transition` — `proposed` → `open` → `paused`/`closed`/`aborted`
- `awm.work_session.patch` — display name / agent profiles / policy (non-terminal only)
- `awm.work_session.delete`

## Invariants honored

- WorkSession is not an MCP connection or host chat thread
- WorkProfile is not a live WorkSession
- AgentProfile is not an AgentInstance or AgentRun
- No credentials in portable fields
- Project catalog remains the project authority; sessions only reference `project_id` / revision
