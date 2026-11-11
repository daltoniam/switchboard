# Apache Druid diagnostic integration

The first slice targets Apache Druid 38.0.0 with its basic-security extension. It exposes five GET tools: datasource discovery, datasource segment summaries, segment load percentages, recent task summaries and exact task status. It does not execute SQL, read task payloads or logs, or change ingestion, supervisors, segments, configuration or security.

## Deployment and credentials

Configure a trusted **HTTPS Router** URL, including any installation proxy prefix. The Router must have management proxy routing enabled for Coordinator and Overlord APIs. TLS hostname and certificate validation are mandatory. Set `ca_cert` to a PEM CA certificate for private deployments; in the single-line UI field enter line breaks as literal `\n` sequences (actual PEM newlines also work in configuration files). Never disable verification. Redirects are rejected, including same-origin redirects: configure the final endpoint.

An administrator must enable basic-security and provision a dedicated diagnostic identity with only:

- `DATASOURCE READ` on each explicitly approved datasource. Resource names in Druid permission definitions are regular expressions: escape literal names and do not use a catch-all pattern.
- `STATE READ` for the resource named `STATE`, for Coordinator diagnostics.

Grant no `DATASOURCE WRITE`, `CONFIG`, `EXTERNAL`, or security administration permissions. Check the union of all assigned roles. Do not use `admin`, `druid_system`, default administrator passwords, anonymous access or a personal administrator account. The adapter rejects built-in administrator usernames but cannot certify an arbitrary user's effective permissions. Verify denied ingestion submission, shutdown, configuration writes and security reads against a disposable deployment before production use.

Druid credentials are not endpoint-scoped. These grants also permit upstream reads outside the five-tool catalog (including SQL reads of approved datasource contents). `STATE READ` exposes cluster state outside these tools. The catalog is not a credential isolation boundary. Keep the Router and credentials within the operator's trusted network and use a gateway when additional upstream restrictions are required.

Open **Integrations > druid**, enter `base_url`, `username`, `password` and optional `ca_cert`, enable and save. A successful health check reads the datasource list; it does not prove that every datasource or Coordinator permission is granted. Credentials persist through the normal Switchboard configuration mechanism.

Environment equivalents:

| Credential | Environment variable |
|---|---|
| `base_url` | `DRUID_BASE_URL` |
| `username` | `DRUID_USERNAME` |
| `password` | `DRUID_PASSWORD` |
| `ca_cert` | `DRUID_CA_CERT` |

## Workflow and bounds

1. Search for Druid ingestion or segment diagnosis, then call `druid_list_datasources`.
2. Pass an exact returned datasource name to `druid_get_datasource` and optionally `druid_get_load_status`.
3. Call `druid_list_tasks` for that datasource. Default: `state=complete`, the last 24 hours, at most 50 tasks. Valid states are `running`, `complete`, `waiting` and `pending`. `FAILED` is a returned `statusCode`, not an accepted state filter.
4. Inspect a returned task identifier with `druid_get_task_status`.

`interval` accepts two RFC3339 timestamps separated by `_`, for example `2026-01-01T00:00:00Z_2026-01-02T00:00:00Z`. A slash-delimited interval is rejected. Upstream `max` applies only to complete tasks. Active tasks and metadata endpoints are capped locally, not paginated. Limits are 1..100; default 50. A datasource filter on load status is local. ACL-filtered lists never constitute a complete cluster inventory; responses explicitly disclose partial visibility and local truncation. Completed-task bounds may omit older tasks even when local truncation is false.

Each operation has a ten-second overall deadline, including at most two 429 retries. Retry-After is honored only within the bounded two-second per-retry window; longer waits return a retryable error. Decompressed response bodies exceeding 2 MiB fail before decoding. Malformed, unexpected null and unsupported response shapes fail explicitly. HTTP status is preserved in sanitized errors; upstream error bodies are withheld. Structured outputs are allowlisted before either scripts or compaction. Failure messages redact the configured password, paths and URLs, remove control characters and are capped. They may still contain business-sensitive text, so only grant visibility that the operator approves.

## Acceptance

`make test-live INTEGRATION=druid` starts digest-pinned official Apache Druid 38.0.0, PostgreSQL and ZooKeeper containers on an isolated Docker network. It enables real basic-security, seeds successful and failing ingestion through the product API, waits for the seeded segment to load, and grants the UI credential only the two read permissions above. It checks denied task submission/shutdown, configuration writes and security reads. A local TLS reverse proxy terminates verified HTTPS in front of the real Router; it does not replace product responses or authorization. All five tools run through MCP and raw script replay with compaction-field validation. Disposable bootstrap credentials never enter the Switchboard setup page.

`TestUIDruid` separately verifies fresh credential fields, HTTPS with a private CA, invalid and missing credentials, health, restart persistence, discovery/read, disable persistence and masked desktop/mobile screenshots against a local fixture. Fixture success is not production authentication certification. Live artifacts are written to `dist/live-artifacts/live-druid.json`; screenshots contain masked inputs.
