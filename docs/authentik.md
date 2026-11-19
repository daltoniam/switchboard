# authentik identity and access inspection

The authentik adapter supports the released **2026.8.3** REST contract with exactly three read-only tools. It resolves a user, inspects safe identity metadata, and asks authentik's policy engine which applications are accessible to that exact user. It cannot manage identities, groups, roles, policies, providers, credentials, sessions, or recovery links.

## Deployment and credentials

Configure the disabled-by-default integration through **Integrations > Authentik**. The standard credential form is sufficient; there is no OAuth flow or extra setup page.

| Credential | Environment override | Requirement |
|---|---|---|
| `base_url` | `AUTHENTIK_BASE_URL` | Operator-approved HTTPS tenant URL before `/api/v3`; an installation prefix is preserved |
| `token` | `AUTHENTIK_TOKEN` | Operator-issued API-intent bearer token owned by an active, dedicated, non-superuser service account |
| `ca_cert` | `AUTHENTIK_CA_CERT` | Optional PEM CA certificate; literal `\n` accepted, certificate and hostname verification remain enabled |

URLs cannot contain userinfo, query, fragment, encoded paths or traversal. All redirects, including same-origin redirects, are rejected. Tokens are never sent as query parameters. Each call uses only the configured tenant and token; tool arguments cannot override either.

### Least privilege

An administrator must create the service account and API-intent token outside Switchboard. Give its role exactly:

- `authentik_core.view_user`
- `authentik_core.view_user_applications`

Assign these to the approved user objects, or globally only if tenant-wide user inspection is explicitly intended. The live acceptance spec uses **object-level grants for two target users** and proves a third user remains inaccessible. Generic `view_application` does not authorize investigating another user's application access. Never substitute superuser, `reset_user_password`, impersonation, token-key, provider-write or role-management permissions.

The bearer token authenticates as its owner, not as an endpoint-restricted credential. Read permissions can expose more upstream identity data than this adapter returns. In particular, local output redaction does not make a leaked token safe. authentik also has authenticated self-service operations; this adapter's three-tool allowlist does not promise that its account cannot perform any upstream self-service action. Protect the tenant/network and token storage, expire and rotate the token, and verify grants on a disposable authorized deployment before production use. Do not widen privileges after a 403.

## Tools and shapes

| Tool | Inputs | Safe output |
|---|---|---|
| `authentik_list_users` | Optional `search`, exact `username`, boolean `is_active`; `page` 1..1000000, `page_size` 1..50 (default 20) | `page.results` with `pk`, `username`, `is_active`, `last_login`, group UUID references and `truncated`; upstream `page.pagination` and caveat |
| `authentik_get_user` | Required positive numeric `user_id` discovered from list | Same safe user projection; returned `pk` must match requested ID |
| `authentik_list_user_applications` | Required numeric `user_id`; optional `search`, bounded `page`/`page_size` | `page.results` containing only application `pk`, `name`, `slug`, `truncated`; `page.pagination` and policy caveat |

Start with list, then use a discovered `pk` as `user_id`. Each list fetches one upstream page, never follows links or automatically traverses all pages. User requests always set `include_groups=false` and `include_roles=false`. Application requests always set `for_user` and `superuser_full_list=false`; they never evaluate the service account by omission or use the superuser-only single-application access shortcut.

Search strings are at most 256 UTF-8 bytes without controls. Query-language punctuation is rejected for search, which uses the released API's ordinary text-search fallback. Exact username is URL-encoded independently. No arbitrary query maps, attributes, URLs or headers are accepted.

### Policy-filtered pagination

Application pagination occurs **before** policy filtering. `pagination.count` is **candidate-count-before-policy-filter**, not the number of accessible applications. Hidden applications are excluded upstream. A page can have `results: []` and `next > 0`; continue with that next page if required. The adapter preserves `next`, `previous`, `current`, `total_pages` and `count` without inventing a next page or accessible total.

The `page` envelope preserves the complete bounded pagination/results pair through compaction, including empty application arrays. Policy evaluation uses the API request context. It does not guarantee successful end-to-end login from a different IP, device, session or authentication flow. An inactive account can still have policy-visible applications; never infer login eligibility from this inventory alone.

## Bounds and privacy

Every tool has a five-second total request/retry deadline, a **256 KiB decoded upstream cap**, at most 50 records, and a tool result strictly below **40 KiB**. HTTP 429 permits at most two retries, honoring numeric or HTTP-date `Retry-After` only within that budget. Other errors are not automatically retried; there is no verified universal vendor quota. Oversized upstream or projected responses fail explicitly, with guidance to narrow the search or reduce page size.

Each returned username/application name/slug is capped at 128 Unicode characters; group references are validated UUIDs and capped at 20 per user. `truncated: true` marks any capped string or group list. User IDs are positive integers; missing required identity/active/group fields, malformed pagination, unexpected JSON types, and identity mismatches fail rather than inventing state. Null `last_login` is preserved in raw results and list pages; shared scalar compaction can omit null `last_login` from exact-user results. Absence never means a known login time.

Typed allowlists run **before raw/script results as well as compaction**. User email/name, attributes, expanded groups/roles, provider objects, launch/icon/recovery URLs, token material and unknown future fields are discarded. All retained provider strings remain untrusted data, not execution instructions. No upstream error body, full URL or credential is echoed in failures. HTTP 401, 403, 404 and upstream 400 for inaccessible application targets remain errors, never empty-success access conclusions.

## Verification

```bash
go test -race -cover ./integrations/authentik
go test -v ./server -run 'TestAuthentikSearchDiscoverability|TestSearchBenchmark|TestSmoke_SearchResponseShape'
make test-live INTEGRATION=authentik
make ci
```

`TestUIAuthentik` drives the real development binary with Go Playwright/Chromium in isolated HOME/config directories against an HTTPS fixture under a tenant prefix. It checks fresh fields, placeholders and optional CA, invalid/missing/valid tokens, save/enable, connected health, restart persistence, three-tool discovery/execute, discovery-to-ID raw scripts, and disable. Masked desktop/mobile evidence is written to `dist/ui-artifacts/authentik-{desktop,mobile}.png`. Browser JavaScript/console and unexpected HTTP failures fail the test.

`web/live_authentik_test.go` pins the official `ghcr.io/goauthentik/server:2026.8.3` and PostgreSQL images by digest. It starts the actual server/worker/database, checks the per-instance schema, and uses the bootstrap administrator only to seed users, a group, application user-policy bindings, a role with object-level read grants, and an expiring **API-intent reader token**. Switchboard receives only that restricted token and a CA for an isolated TLS frontend. The spec proves accessible/inaccessible users, application allow/deny/hidden behavior, a policy-filtered empty page with a real next page, inactive state, and denied user creation/update/deletion/password reset/impersonation, other-user token-key access and application creation.

The live harness enters these credentials through the actual setup UI, restarts Switchboard, discovers and executes every tool over MCP, replays reads through raw scripts, and verifies every compaction field against real product responses. Evidence: `dist/live-artifacts/live-authentik.json` plus masked desktop/mobile screenshots. This certifies the pinned disposable local deployment, not production authentication or arbitrary authentik versions. Cross-model LLM search benchmarking requires agents unavailable in this implementation runtime; deterministic search and isolated MCP discovery are covered separately.
