# Semaphore UI metadata integration

This adapter supports **Semaphore UI v2.19.12**, not Semaphore CI SaaS. It exposes five read-only tools for one configured project: project identity, template list/get, and task list/get. It cannot run infrastructure automation or read task logs.

## Provision a dedicated guest

1. An administrator creates a dedicated **non-admin** user.
2. Add that user as **guest** in the single approved project. Remove manager, owner, task-runner, custom-role and template-specific run grants. Review every membership and template grant; the project role alone does not certify all effective permissions.
3. Sign in as that user and create an API token using Semaphore UI's account interface. The adapter never issues tokens or accepts login passwords.
4. On an authorized disposable deployment, verify project/template/task reads succeed and task creation, stopping and template editing return HTTP 403. Do not probe production mutations.
5. Review network access and token lifetime, then configure the integration through Switchboard's Integrations page.

A token inherits the user's full access. **Guest is not an endpoint-scoped or globally read-only token.** Guest GET access can expose upstream environments, inventories, logs and other project data outside these five tools, and account self-service can remain available. A leaked token bypasses Switchboard's output allowlist. Only approve a project whose complete guest visibility is acceptable, isolate the service, and never use an administrator's token.

Setup and every tool execution verify `/api/user` is explicitly non-admin, `/api/info` identifies v2.19.12, and `/api/project/{project_id}/role` reports `guest` with `permissions=0`. Rejected reconfiguration clears the previous connection. Permission changes between verification and a read cannot be made atomic by this API; upstream access control and operator review remain required.

## Credential fields

| Field | Environment override | Meaning |
| --- | --- | --- |
| `base_url` | `SEMAPHOREUI_BASE_URL` | Trusted HTTPS installation URL, including its deployment prefix, without `/api` |
| `api_token` | `SEMAPHOREUI_API_TOKEN` | Dedicated guest API token without `Bearer ` |
| `project_id` | `SEMAPHOREUI_PROJECT_ID` | One positive numeric project ID, maximum 2147483647 |
| `ca_cert` | `SEMAPHOREUI_CA_CERT` | Optional PEM root CA; literal `\n` line separators are accepted |

Certificate and hostname verification are mandatory. HTTP, URL credentials, query strings, fragments and encoded/traversal paths are rejected. Redirects are never followed. No per-call URL or project override is accepted. A health check verifies the same identity and configured project read used by the tools. Other versions fail explicitly until tested.

## Tools and bounds

| Tool | Inputs | Output |
| --- | --- | --- |
| `semaphoreui_get_project` | None | Configured project ID/name; start here |
| `semaphoreui_list_templates` | None | At most 100 ID/name/app/type summaries and last-task ID/status when available, plus `truncated` |
| `semaphoreui_get_template` | `template_id` | The same safe template summary |
| `semaphoreui_list_tasks` | Optional `count`, `before`, `template_id`, `status` | One page of task IDs, template/project IDs, status and available timestamps, plus pagination metadata |
| `semaphoreui_get_task` | `task_id` | One task's safe status/timestamp summary |

Task pages default to 50, maximum 100. `before` is the previous response's `next_before`, the minimum ID on the original unfiltered page, not a page number. Template/status filters apply locally to that one page. An empty filtered page can still have `has_next=true`; keep paging with its cursor. Only explicit `X-Has-Next: true/false` establishes completeness. Missing/unsupported headers yield `pagination_known=false` and omit `has_next`; they do not imply the last page. Template lists have no server cursor: `truncated=true` means only the first 100 summaries were returned.

Each operation, including identity checks and bounded 429 retries, has a 15-second deadline. Each decompressed response is limited to 2 MiB before JSON decoding. Oversized, malformed, redirected, unauthorized or mismatched-project responses fail explicitly. There is no automatic page traversal. HTTP 429 honors bounded numeric or HTTP-date `Retry-After`, at most two retries; 401/403 never escalates privileges. Error bodies and transport URLs are not returned.

Allowlists run in handlers **before both raw scripts and compaction**. Playbooks, arguments, environments, secrets, inventories, repository paths, parameters, survey variables, vaults, artifacts, messages, commit messages and logs are excluded. Missing/null optional fields remain absent. Last-task objects contain only ID/status, and compaction preserves that nesting. No write tools or arbitrary REST proxy exist.

## Verification

```bash
go test -race ./integrations/semaphoreui ./config
make test-live INTEGRATION=semaphoreui
make ci
```

`TestUISemaphoreUI` drives the real Switchboard binary with an isolated TLS fixture: fresh fields and placeholders, disabled discovery, invalid/missing credentials, save/enable, health, all five MCP reads, restart persistence and disable. Masked desktop/mobile screenshots are in `dist/ui-artifacts/`.

The live spec pins the official Semaphore UI image by digest, seeds a local Git repository and real successful/failed Bash tasks through the product API, and creates a non-admin guest token. It verifies denied task creation/stop/template edits, raw-output sentinel exclusion, an empty filtered page's cursor, all tools through MCP and scripts, every compaction field, and restart persistence. A local verified-TLS reverse proxy fronts only the disposable container's loopback HTTP port. No mock API replaces the live product. Evidence is written to `dist/live-artifacts/live-semaphoreui.json` and masked desktop/mobile screenshots. This verifies disposable-product authentication, not a production account.
