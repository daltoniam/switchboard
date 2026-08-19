# Project work model (AWM subset)

Switchboard implements a **minimal Agent Work Model** subset in the internal `awm` package so projects, resources, resource bindings, work profiles, work sessions, and agent profiles share one store tree. It is available through both the MCP tools below and the strongly typed [AWM gRPC API](awm-grpc.md).

Canonical vocabulary: `~/work/projects/agent-work-model/model/terms/`.

## Terms in package `awm`

| AWM term | Type | Tools |
|---|---|---|
| **Project** | `awm.Project` | `project_list` / `project_get` / `project_create` / `project_update` / `project_delete` (catalog surface) |
| **Resource** | `awm.Resource` | `project_resource_*` |
| **ResourceBinding** | `awm.ResourceBinding` | `project_resource_binding_*` |
| **WorkProfile** | `awm.WorkProfile` | `project_work_profile_*` |
| **WorkSession** | `awm.WorkSession` | `project_work_session_*` |
| **AgentProfile** | `awm.AgentProfile` | `project_agent_profile_*` |

`project_*` catalog tools and `awm.Store` project APIs share `~/.config/switchboard/projects/*.project.json`.

## Storage (Switchboard config root only)

```text
~/.config/switchboard/
  projects/<project_id>.project.json    # Project (awm + catalog)
  awm/
    resources/<resource_id>.json
    resource_bindings/<resource_binding_id>.json
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

## Default WorkProfile

On startup Switchboard seeds an idempotent WorkProfile with
`work_profile_id: "default"`, `display_name: "default"`, version `1`, and empty
default policy / intended resources. Repeated initialization never overwrites
an operator-modified existing `default` record. Clients that omit
`work_profile_id` resolve only to this exact ID.

## WorkSession

May reference `project_id` (must exist), `project_revision`, `project_snapshot_id`,
`work_profile_id`, `agent_profile_ids`.

Project-bound sessions pin one exact immutable Project revision archive:

- When `project_revision` is omitted, the live Project revision is resolved and pinned.
- `project_snapshot_id` is deterministic:
  `project://registry/projects/{project_id}/revisions/{project_revision}`.
- Mismatched project / revision / snapshot triples are rejected with
  `invalid_reference`.
- Fabricated revisions (e.g. `sha256:abc`) fail archive lookup.

WorkProfile eligibility: empty `project_ids` means globally applicable; otherwise
the session's `project_id` must be listed.

Policy may only narrow WorkProfile `default_policy` and Project `policy`.
Switchboard's closed v1 policy shape is `capability name -> boolean allowed`;
child policy can change `true` to `false`, never `false` to `true`. Arbitrary
JSON policy values and arbitrary AgentProfile constraint objects are rejected.

Lifecycle: `proposed` → `open` → `paused` / `closed` / `aborted`.

Create is idempotent for the same `work_session_id` with compatible fields.

## Resource and ResourceBinding

A Resource is independently addressable and remains under its native provider's
authority. `resource_id` and `uri` are names, not credentials or grants. Both
`uri` and a binding's optional `resolved_locator` must be absolute URIs.

A ResourceBinding belongs to exactly one WorkSession and references exactly one
Resource. Its boolean capability `grant` must narrow both WorkSession and
Project policy. Lifecycle is `proposed` → `bound` → `revoked`, with direct
`proposed` → `revoked` also allowed. A `bound` binding requires a
`resolved_locator`. Bindings cannot be created for terminal WorkSessions.

## Referential integrity

Deleting a Project or WorkProfile while a retained WorkSession references it
fails with typed code `referenced`. Deleting a Resource or WorkSession while a
retained ResourceBinding references it also fails with `referenced`, including
for revoked bindings. Nothing is cascade-deleted; remove the binding explicitly
before removing either endpoint. Immutable revision history is retained.

## Typed errors

Work-model tool errors expose stable codes in structured content:

| Code | Meaning |
|---|---|
| `not_found` | Entity missing |
| `already_exists` | Duplicate identity |
| `invalid_input` | Schema / validation |
| `invalid_reference` | Missing or mismatched foreign key / pin |
| `invalid_transition` | Illegal lifecycle move |
| `referenced` | Delete blocked by a retained WorkSession or ResourceBinding |
| `policy_broadening` | WorkSession policy or ResourceBinding grant widens a parent |
| `conflict` | CAS / concurrent mutation |
| `missing_default_profile` | Exact-ID `default` profile absent |
| `unavailable` | Dependency failure |
| `lock_timeout` | Filesystem lock |
| `write_disabled` | Mutations refused because `project_catalog.writes_enabled` is false |

## Invariants

- WorkSession ≠ MCP connection or host chat  
- WorkProfile ≠ live WorkSession  
- AgentProfile ≠ running instance  
- ResourceBinding ≠ Resource authority
- Resource IDs and URIs do not grant access
- No credentials in these records
- One mutable authority per entity (Switchboard config root only)  
