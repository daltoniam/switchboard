# Agent Work Model gRPC API

Switchboard exposes a native gRPC API for its implemented Agent Work Model (AWM) subset. The protobuf contract is [`api/awm/v1/awm.proto`](../api/switchboard/awm/v1/awm.proto).

## Design review

The canonical AWM 0.1 model is protocol-neutral and defines semantic terms, ownership, relationships, lifecycle, and invariants. It does **not** define a transport schema. In particular, `PolicyDocument`, `constraints: object`, and `resolved_definition: object` are semantic placeholders rather than interoperable wire types.

The MCP adapter weakens those boundaries further by accepting generic JSON objects and returning structured content whose shape is known only at runtime. That is convenient for an LLM, but awkward for project/tool integrations that need generated clients, compile-time field names, stable enums, and standard transport errors.

The gRPC v1 contract therefore makes these deliberate choices:

- one RPC per MCP operation instead of `tool_name` plus an arguments object;
- generated request and response messages for every operation;
- closed `WorkSessionState` and `ResourceBindingState` enums;
- boolean capability policies (`PolicyDocument`) instead of arbitrary JSON values;
- explicit project eligibility constraints instead of arbitrary agent constraint objects;
- a closed Project projection (`version`, `name`, `display_name`, `description`, and boolean capability `policy`) instead of arbitrary project extensions;
- typed gRPC status details (`ErrorDetail`) carrying stable AWM/catalog error codes;
- no `google.protobuf.Struct`, `Value`, `Any`, raw JSON, or opaque bytes.

This is an AWM projection, not a second authority. Both MCP and gRPC adapters use the same `project.Store` and `awm.Store` instances.

## Services and MCP parity

| gRPC service | RPCs | MCP surface |
|---|---|---|
| `ProjectCatalogService` | `ListProjects`, `SearchProjects`, `GetProject`, `ResolveProject`, `ValidateProject`, `CreateProject`, `UpdateProject`, `PatchProject`, `DeleteProject` | `project_list`, `project_search`, `project_get`, `project_resolve`, `project_validate`, `project_create`, `project_update`, `project_patch`, `project_delete` |
| `ProjectCatalogService` | `GetProjectRevision`, `GetProjectDiagnostics` | `project://.../revisions/...` and diagnostics resources |
| `WorkProfileService` | list/get/put/delete | `project_work_profile_*` |
| `AgentProfileService` | list/get/put/delete | `project_agent_profile_*` |
| `WorkSessionService` | list/get/create/transition/patch/delete | `project_work_session_*` |
| `ResourceService` | list/get/put/delete | `project_resource_*` |
| `ResourceBindingService` | list/get/create/transition/patch/delete | `project_resource_binding_*` |

MCP remains available for LLM clients. gRPC is the preferred integration boundary for compiled applications.

## Endpoint

Native gRPC is served over HTTP/2 cleartext (h2c) on the same address as the existing HTTP server (default `localhost:3847`). Standard gRPC method paths are used, for example:

```text
/switchboard.awm.v1.WorkSessionService/CreateWorkSession
/switchboard.awm.v1.ResourceBindingService/CreateResourceBinding
```

The Project Catalog service follows `project_catalog.enabled`; the five work-model services remain available whenever the HTTP server is running. Mutation RPCs follow the same `project_catalog.writes_enabled` switch as MCP mutations.

The gRPC surface has the same deployment trust boundary as `/mcp`: there is no
built-in caller authentication. On a non-loopback deployment, disable writes or
put the shared port behind an authenticating TLS proxy. h2c is intended for
local/direct use.

## Errors

Failures use idiomatic non-OK gRPC statuses. The status includes a typed `switchboard.awm.v1.ErrorDetail` detail with a stable `ErrorCode`. Clients should branch on the detail code and use the human message only for display.

Broad status mapping:

- invalid input/reference/definition: `InvalidArgument`
- not found: `NotFound`
- already exists: `AlreadyExists`
- revision conflict: `Aborted`
- invalid transition, referenced entity, or policy broadening: `FailedPrecondition`
- writes disabled / permission denied: `PermissionDenied`
- lock timeout: `DeadlineExceeded`
- unavailable dependency: `Unavailable`
- unexpected failures: `Internal`

## Compatibility boundary

The filesystem can contain legacy Project extensions or older AWM records with non-boolean policy values. gRPC v1 does not serialize those as an untyped escape hatch. Project reads return the closed AWM Project projection; AWM records with policy/constraint values outside the v1 protobuf contract fail with `FailedPrecondition` rather than silently coercing data.
