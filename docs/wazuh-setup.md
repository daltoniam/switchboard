# Wazuh manager agent health

The native `wazuh` integration inspects endpoint coverage through the Wazuh **manager API**, tested against 4.14.8. It does not connect to the indexer or search alerts, vulnerabilities, logs, process inventory, or configuration content. All three tools are read-only; there are no enrollment, restart, scan, security-administration, or active-response tools.

## Provision a least-privilege reader

An operator must provision the identity outside Switchboard:

1. Keep the manager's production-recommended RBAC **white** mode (deny by default). Do not change a production deployment's global authorization mode just to connect this integration.
2. Create a dedicated non-administrator API user with `allow_run_as=false`. Do not use `wazuh`, `wazuh-wui`, `admin`, or `administrator`; configuration rejects these account names.
3. Create one custom policy with action `agent:read`, effect `allow`, and explicit approved resources such as `agent:id:001`. Link that policy to a dedicated role and assign only that role to the API user. Do not grant security management, wildcard administration, enrollment, configuration writes, restart, or active response. Include `agent:id:000` only if the manager's own inventory is approved for reading.
4. Review the user's complete effective role union. A non-default account name does not prove least privilege. Switchboard checks connectivity and a bounded agent read, not the complete effective RBAC policy; a Connected badge is not a privilege certification.
5. On an authorized disposable deployment, verify the approved agent reads and scoped summary, hidden agents, and denied enrollment/restart/delete/active-response/security changes. Never probe destructive permissions on production.

An `agent:read` credential is not an endpoint-scoped or metadata-only upstream credential. Other manager read endpoints may be available outside this three-tool catalog. Hostnames and operating-system details remain sensitive; approve the entire read visibility of the identity and protect its password accordingly. The adapter never grants roles, changes RBAC, mints accounts, or changes token expiry.

Vendor references: [released API contract](https://github.com/wazuh/wazuh/blob/v4.14.8/api/api/spec/spec.yaml), [RBAC provisioning](https://github.com/wazuh/wazuh-documentation/blob/main/source/user-manual/api/rbac/configuration.rst), and [RBAC semantics](https://github.com/wazuh/wazuh-documentation/blob/main/source/user-manual/api/rbac/how-it-works.rst).

## Configure

Open **Integrations > Wazuh** in a fresh Switchboard Web UI. The integration starts disabled. Fill the standard form, enable it, and save:

| Credential | Environment override | Meaning |
|---|---|---|
| `base_url` | `WAZUH_BASE_URL` | Fixed trusted HTTPS manager installation URL, normally port 55000; an installation prefix is preserved |
| `username` | `WAZUH_USERNAME` | Dedicated scoped reader |
| `password` | `WAZUH_PASSWORD` | Reader password, displayed as a password field |
| `ca_cert` | `WAZUH_CA_CERT` | Optional PEM CA certificate; literal `\n` is accepted for the one-line form |

Certificate chain and hostname verification always remain enabled, with TLS 1.2 or later. For the official container's self-signed certificate, trust the certificate explicitly and use its certified hostname (`localhost` in the disposable live spec). HTTP, credentials in URLs, query strings, fragments, encoded/traversal prefixes, and redirects are rejected. There is no insecure TLS option.

Authentication is `POST /security/user/authenticate` with HTTP Basic over verified HTTPS. Subsequent reads use its JWT as a bearer token. JWTs stay in memory and are renewed under synchronization before expiry or once after an HTTP 401. Default upstream expiry is 900 seconds; the adapter does not change it. Invalid passwords, revocation, 401/403, and TLS failures produce a failed health check, not an empty fleet.

## Search and execute

| Tool | Inputs | Result |
|---|---|---|
| `wazuh_list_agents` | Optional `search`, `status`, `limit`, `offset` | One scoped page with `affected_items`, totals, sanitized failures, `partial`, and `next_offset` |
| `wazuh_get_agent` | Required `agent_id` copied from a list result, preserving leading zeros | One exact agent in the same safe page envelope; missing/forbidden is an error |
| `wazuh_get_agent_status_summary` | None | RBAC-scoped `connection` and `configuration` count objects |

Start with search query `wazuh disconnected endpoint agents`, execute `wazuh_list_agents`, then use an observed `id` with `wazuh_get_agent`. Exact reads use `/agents?agents_list=<id>`, not an `/agents/<id>` route. Summary totals exclude manager agent `000`, even when the reader can list it.

Lists default to 25, maximum 100; `offset` is zero-based, at most 2147483647. Follow a non-null `next_offset` with the same filters. A null cursor marks the last page in raw output; compaction omits null fields, so an absent compacted `next_offset` also means no next page. There is no automatic pagination or fanout. Stable ID ordering reduces page churn, but offset pagination is not a snapshot if the fleet changes. Status is one of `active`, `disconnected`, `never_connected`, or `pending`. Search text is limited to 256 UTF-8 bytes.

Agent output is allowlisted before both raw scripts and compaction: `id`, `name`, `status`, `version`, `os.name`, `os.version`, `lastKeepAlive`, and `group_config_status`. Missing fields stay absent and unknown status strings remain unchanged. IPs, OS uname, groups, checksums, keys, upstream error messages, remediation text, and unexpected fields are never forwarded. Retained strings are limited to 256 bytes; oversized or malformed responses fail explicitly. Partial upstream failures retain counts, failed agent IDs, numeric error codes, and `partial=true`; a wholly failed read is an error rather than a successful empty list.

Each call has a 15-second network deadline, a 2 MiB decompressed response ceiling, and serialized access to avoid a refresh stampede. HTTP 429 retries at most twice with bounded backoff; `Retry-After` is honored only when its wait fits the two-second retry budget, otherwise return an actionable rate-limit error without retrying early. No retries on 403 or server failures. Calls are on demand; there is no background fleet polling.

## Verification

```bash
go test -race ./integrations/wazuh ./config ./server ./web
make test-live INTEGRATION=wazuh
make ci
```

`TestUIWazuh` drives the real development binary in isolated Chromium with a TLS fixture: fresh fields, disabled discovery, invalid/missing/valid credentials, health, save, restart persistence, MCP search/read, and disable. Masked desktop/mobile evidence is under `dist/ui-artifacts/wazuh-*.png`.

`web/live_wazuh_test.go` starts the official 4.14.8 manager image pinned to `sha256:412f665c77af5497780d29d0d9f069d7a169c5eca45e9a3a47a587d9cfc99b55`. Bootstrap credentials are used only to seed 26 agents, one scoped role, and a dedicated reader. Switchboard receives only the reader and verified TLS trust. The spec proves hidden-agent exclusion, scoped counts, upstream mutation/security denials, pagination, setup, restart persistence, all tools over MCP, and raw-script/compaction field parity. It uses a real manager without an indexer; indexer warnings do not affect the manager-only contract. No live cases are skipped. Results are written to `dist/live-artifacts/live-wazuh.json`; production authentication remains unverified.
