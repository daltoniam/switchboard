# InfluxDB OSS 2.8 metadata diagnostics

This adapter supports InfluxDB **OSS 2.8**, tested against 2.8.0. It is not an InfluxDB 1, InfluxDB 3, Cloud Serverless, SQL, Flux query, or processing-engine trigger integration. InfluxDB 3 has its own official MCP server; this adapter covers the different OSS 2 task/run control plane.

## Credentials

Open **Integrations > Influxdb** on a fresh Switchboard installation. Enter:

| Field | Value |
| --- | --- |
| `base_url` | Fixed trusted HTTPS server URL, including any reverse-proxy installation prefix. No query, fragment, embedded credentials, or redirects. |
| `org_id` | Nonzero 16-character lowercase hexadecimal organization ID. Every task and bucket is checked against this organization. |
| `api_token` | Administrator-created custom token with only the read permissions below. Sent as `Authorization: Token ...`, never Bearer. |
| `ca_cert` | Optional trusted PEM CA certificate. In the single-line form, replace certificate newlines with literal `\n`. TLS certificate and hostname verification cannot be disabled. |

An administrator must create a dedicated custom authorization granting:

- `read` on `tasks` in the configured organization, without `write`.
- `read` on the approved `buckets`, preferably restricted to each selected bucket ID, without `write`.
- No authorizations, secrets, user management, operator, all-access, or other resource permissions.

Use the vendor's [authentication guide](https://docs.influxdata.com/influxdb/v2/api/authentication/) and [custom authorization reference](https://docs.influxdata.com/influxdb/v2/reference/cli/influx/auth/create/). The `--read-tasks` permission is separate from `--write-tasks`. Do not give Switchboard the administrator credential used to provision the token.

**Credential exposure risk:** upstream bucket-read permission also permits point-data reads, and task-read permission exposes task definitions and potentially logs upstream. The adapter exposes only allowlisted metadata, but it cannot turn a stolen upstream token into an endpoint-scoped token. Org-scoped task permission may reveal every task in that organization. Choose an organization whose full task visibility is acceptable.

Enable and save. A **Connected** badge requires a successful `/health` response identifying 2.8.x and an authenticated org-scoped task list. Unsupported versions, invalid tokens, or an untrusted certificate remain **Not Verified**. Configuration validation rejects HTTP and invalid credential formats. Health verification does not certify all effective upstream permissions; provision and audit the custom token separately.

## Tools and pagination

| Tool | Purpose |
| --- | --- |
| `influxdb_list_buckets` | Start here for IDs, bucket names, and retention-rule durations. |
| `influxdb_list_tasks` | Task IDs, names, organization, active state, schedules, last run status, and latest completion. Optional exact name and active/inactive filters. |
| `influxdb_get_task` | Exact task summary after parent-organization validation. |
| `influxdb_list_task_runs` | Recent run IDs, state, and timestamps; defaults to the last 24 hours with a maximum seven-day window. |
| `influxdb_get_task_run` | Exact task/run state and timestamps after validating the task organization. |

Each list call fetches one page with a default limit of 50 and a maximum of 100. Buckets use `offset`/`next_offset`; tasks and runs use hexadecimal `after`/`next_after` IDs. `has_more` conservatively indicates that a full page may have a successor, not that another record is guaranteed. `page_only` is always true. Continue until a short or empty page; the adapter never follows upstream links or automatically fetches every page. A missing next cursor is represented as null in raw script output.

Run `after_time` and `before_time` use RFC3339. With only `before_time`, the default lower bound is 24 hours earlier. With only `after_time`, the upper bound is now; the entire interval must still be at most seven days. Status filtering is local to the returned page and does not alter its cursor, even when no items match.

Requests have a ten-second overall deadline, a 1 MiB decompressed response ceiling, and at most two retries for HTTP 429. Retry-After delays longer than two seconds are surfaced instead of blocking. There is no background polling. API error bodies, URLs, Flux, descriptions, labels, authorization IDs, last-run error text, and logs are never returned. Null optional values are preserved in raw output; compaction may omit nulls. No query, mutation, trigger, retry, cancellation, or arbitrary REST tools exist.

OSS 2.8.0 reports several denied task mutations as HTTP 500 with an `unauthorized` message rather than 401/403. The disposable live test asserts those actual denials and verifies the task remains active; the adapter does not expose those mutation endpoints.

## Verification

```sh
go test -race ./integrations/influxdb ./config
make test-live INTEGRATION=influxdb
make ci
```

The live spec pins the official `influxdb:2.8` image by digest, enables verified TLS, provisions a bucket and a failed Flux task run through the product API, and uses a custom read-tasks/one-bucket token through the browser setup page. It checks denied task creation/update/deletion/run/retry and authorization creation, rejects out-of-org reads, and verifies secret-sentinel exclusion before MCP compaction and in raw scripts. All five tools run over MCP, with restart persistence, field-spec validation, and masked screenshots under `dist/live-artifacts/`.

Fixture-backed `TestUIInfluxDB` additionally covers fresh credential defaults, required/optional fields, TLS trust failure, invalid token health, save/enable/disable, MCP discovery, and restart persistence. Live acceptance proves disposable OSS 2.8 authentication, not a production account or vendor SaaS sandbox.
