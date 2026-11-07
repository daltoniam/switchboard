# Keycloak read-only investigation

The native `keycloak` adapter exposes four tools for one configured realm:

| Tool | Purpose |
|------|---------|
| `keycloak_list_users` | Start here: search a bounded page of user profiles and enabled/disabled account flags |
| `keycloak_get_user` | Read an exact user ID discovered from the user list |
| `keycloak_list_groups` | List top-level groups, or search group names, without recursive hierarchy expansion |
| `keycloak_list_events` | Read persisted `LOGIN` and `LOGIN_ERROR` events for a bounded date range |

No identity writes, password reset, impersonation, role assignment, memberships, sessions, token introspection, admin events, or generic HTTP endpoint are exposed. Listing groups does **not** establish which groups a user belongs to. An enabled account flag does not establish whether brute-force protection has temporarily locked an account.

## Operator setup

1. Use an authorized Keycloak deployment and a dedicated confidential client in the target realm. Enable client authentication and service accounts, not interactive browser login for this integration.
2. Grant only the realm-management read permissions required by the intended workflow: `view-users` and `query-users` for user reads, applicable group query/view permissions for group reads, and `view-events` for login events and their persistence configuration. Check effective composite roles, fine-grained admin permissions, and actual endpoint access on the deployed version. Do not grant `realm-admin`, manage-users, manage-events, or impersonation as a workaround for HTTP 403.
3. In the local Switchboard Web UI, open **Integrations → Keycloak**, enter the installation URL, realm **name** (not ID), client ID, and client secret, enable the integration, and save. All four fields are required. The generic credential form is the setup page; there is no interactive OAuth redirect or named-identity flow.
4. Verify health and perform a safe user list with the service account. Health checks only a one-item user list: it does not certify group/event access. Verify all enabled workflows with the least-privilege account on a disposable deployment before production use.

The adapter exchanges `client_credentials` at the configured installation's `/realms/{realm}/protocol/openid-connect/token`, caches the bearer token in memory, renews before expiry, and refreshes once after HTTP 401. Tokens are not written into configuration or returned by tools. Pre-issued tokens are not accepted. Reconfiguration replaces the in-memory credential/token state; an invalid configuration cannot reuse prior credentials.

The installation URL must use HTTPS, preserving any deployment subpath (for example `/auth`). HTTP is accepted only for literal loopback addresses for isolated local fixtures. TLS verification is always enabled; use the host's trusted certificate roots for a private CA. There is no insecure TLS switch. URL userinfo, query strings, fragments, unsafe paths, and **all redirects** are rejected. Realm names, installation path components, and user IDs support literal ASCII letters, digits, hyphens, underscores, dots, and tildes, excluding `.` and `..`; unusual names outside this supported subset require a later compatibility extension. Do not include `/admin/realms` in the installation URL.

Environment overrides (the integration remains disabled until explicitly enabled):

| Credential | Environment variable |
|------------|----------------------|
| `base_url` | `KEYCLOAK_BASE_URL` |
| `realm` | `KEYCLOAK_REALM` |
| `client_id` | `KEYCLOAK_CLIENT_ID` |
| `client_secret` | `KEYCLOAK_CLIENT_SECRET` |

## Read boundaries

- Lists default to `first=0`, `max=20`. `first` is an integer from 0 through 1,000,000; `max` is 1 through 100. One page per tool call, no automatic crawling. Search is at most 512 characters. Unknown parameters, arbitrary realms, URLs, and query expressions are not accepted.
- List responses carry `items`, `first`, `max`, `returned_count`, and `has_more_possible`. The latter is true only when the page is full; it is not an upstream total or a guarantee of another page. Permissions, user federation, upstream search semantics, and concurrent changes can affect visibility and ordering. Group lists omit subgroup expansion; an unsearched list covers top-level groups only.
- Events require explicit inclusive UTC `date_from` and `date_to` (`YYYY-MM-DD`) spanning at most seven calendar days. `type` is `LOGIN` or `LOGIN_ERROR`, defaulting to both; `user` optionally restricts an exact user ID. Out-of-filter event responses fail closed. The adapter first reads `/events/config` under the same `view-events` permission. Disabled or unavailable persistence is an error, not an empty-history claim. Enabled persistence does not prove collection, retention, or that both event types are saved.
- User output is a typed allowlist: `id`, `username`, `enabled`, `email`, `emailVerified`, `firstName`, `lastName`, and `createdTimestamp` when available. Compact user lists retain the core lookup/status fields. Group output contains only `id`, `name`, `path`, `parentId`, and `subGroupCount` when available. Event output contains only `time`, `type`, `userId`, `clientId`, and `error`.
- Credentials, custom attributes, required actions, group roles/subgroups/members, event details, IP addresses, session IDs, and unknown fields are removed **before** both raw/script and compacted returns. Allowed profile fields are still sensitive personal information and untrusted data, not instructions; authorize their use and retention.
- Each tool has a shared 15-second deadline including token exchange and its optional single 401 refresh. Each decoded response is capped at 2 MiB (including gzip expansion); final raw output is capped at 64 KiB. Oversized or malformed responses fail rather than silently truncating. Reduce page size or narrow the search after overflow.
- HTTP status errors are sanitized without upstream bodies, secrets, tokens, or request URLs. HTTP 403 and 429 do not trigger retries, permission escalation, or alternate authentication. Numeric/date Retry-After is presented with a maximum one-minute advisory, not automatically slept or retried. There is no universal rate-limit claim for self-hosted installations.

## Compatibility and verification

Initial contract research used the published Admin REST reference and **Keycloak 26.8.1** released Java endpoint contracts, independently implemented with `net/http` and typed JSON DTOs. The small four-tool surface does not justify adopting the larger community Go SDK. No first-party Go SDK is required.

Fixture-backed verification covers token exchange/expiry/401 refresh/concurrency, fixed paths and query encoding, error sanitization, redirects, TLS verification, gzip/byte/output limits, event-persistence state, filter boundaries, dispatch/compaction parity, raw/script projection, search discovery, and fresh browser setup/save/health/restart/disable.

**Live authentication and least-privilege deployment acceptance: unverified without dedicated test credentials.** Fixtures establish adapter behavior, not a certification of production permissions, Keycloak versions, custom providers, or event collection. Do not perform destructive permission-denial probes against production.
