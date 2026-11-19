## Sandbox E2E proof

**Ready on the sandbox side, pending human review.** On `41f207e`, all 28 tools pass end to end through Switchboard MCP (native REST over HTTPS) against a real self-hosted GrowthBook 5.2.0 with a live warehouse analysis. The 3 write tools only touched sandbox data. Configuration and auth guards behave as documented, and secrets stay scrubbed. No bugs found; open items are INFO only. Human review is still required, and nothing was merged.

**Head caveat:** Sandbox proof ran at `41f207e07389bb64f634b4970c9940ad60f5e5b3`; PR head is now `477a946329ec23fca0c18e6ffda78bcd7fc2c1ae`. Re-run on current head if you need byte-for-byte alignment with the proof commit.

Proof head `41f207e07389bb64f634b4970c9940ad60f5e5b3`. Self-hosted GrowthBook **5.2.0+ffe6dab** (the official image digest the PR's live test pins, run without Docker), MongoDB 8.0.4, PostgreSQL 17 warehouse, HTTPS via a sandbox CA (`ca_cert`).

**78 PASS / 0 FAIL / 4 INFO** (build + MCP E2E + config/auth + UI cross-check)

| Tool | Access | Result | Calls | Evidence |
|---|---|---|---|---|
| `growthbook_list_features` | read | PASS | 5 | 4 seeded flags + rules/saved group; project filter, limit/offset paging (hasMore), limit=500 clamped to 100, limit=0 rejected |
| `growthbook_get_feature` | read | PASS | 5 | boolean flag and string flag with force (saved group) + 10% rollout rules and prerequisite; 404 and `../` id rejected client-side |
| `growthbook_get_feature_staleness` | read | PASS | 2 | per-env staleByEnv for 2 flags; 101 ids rejected client-side |
| `growthbook_list_feature_revisions` | read | PASS | 2 | published history ("Created via REST API"); default drafts → empty |
| `growthbook_list_experiments` | read | PASS | 3 | running + stopped (won) experiments; status=running/stopped filters exact |
| `growthbook_get_experiment` | read | PASS | 3 | hypothesis, variations, linked flag, "Banner won" analysis; unknown id 404 |
| `growthbook_get_experiment_results` | read | PASS | 1 | live Postgres analysis: 400 users, SRM healthy, Express chanceToBeatControl 0.999998; matches UI |
| `growthbook_refresh_experiment_results` | write | PASS | 1 | started snapshot snp_… against sandbox warehouse |
| `growthbook_get_snapshot` | read | PASS | 2 | snapshot from refresh reaches success; unknown id 404 |
| `growthbook_list_fact_metrics` | read | PASS | 2 | Purchase Rate; fact_table_id filter |
| `growthbook_get_fact_metric` | read | PASS | 1 | proportion metric details |
| `growthbook_list_metrics` | read | PASS | 1 | legacy binomial metric (met_) |
| `growthbook_list_fact_tables` | read | PASS | 1 | Purchases with columns from warehouse |
| `growthbook_list_projects` | read | PASS | 1 | SB Growth project |
| `growthbook_list_environments` | read | PASS | 1 | production + staging |
| `growthbook_list_sdk_connections` | read | PASS | 1 | SB Web sdk- key; encryption/proxy keys scrubbed |
| `growthbook_list_saved_groups` | read | PASS | 1 | Beta Testers |
| `growthbook_get_saved_group` | read | PASS | 2 | list values sb-user-1/2; unknown id 404 |
| `growthbook_list_attributes` | read | PASS | 1 | country attribute with description |
| `growthbook_list_data_sources` | read | PASS | 1 | SB Warehouse + assignment query; connection params (password) scrubbed |
| `growthbook_list_segments` | read | PASS | 1 | Big Spenders SQL segment |
| `growthbook_list_audit_logs` | read | PASS | 4 | feature.toggle with reason; before=nextCursor paging; missing entity_type rejected |
| `growthbook_list_code_refs` | read | PASS | 1 | acme/web src/checkout.ts |
| `growthbook_get_feature_code_refs` | read | PASS | 1 | line 42 ref for sb-checkout-express |
| `growthbook_get_settings` | read | PASS | 1 | bayesian/confidenceLevel; secureAttributeSalt scrubbed |
| `growthbook_list_members` | read | PASS | 1 | admin member |
| `growthbook_toggle_feature` | write | PASS | 6 | production off→on→off verified by direct REST, staging untouched; unknown env/feature errors; missing env rejected |
| `growthbook_create_feature` | write | PASS | 6 | boolean/string/json flags created disabled in all envs with no rules (verified by direct REST); duplicate, bad type, bad key rejected |

| Config / auth check | Result | Evidence |
|---|---|---|
| sdk- client key rejected at configure | PASS | configured=False; True: growthbook: integration is not configured |
| bogus secret key -> 401 surfaced | PASS | configured=True; True: growthbook API error (401): {"message":"Invalid API key"} |
| plain http base_url rejected | PASS | configured=False; True: growthbook: integration is not configured |
| base_url with credentials rejected | PASS | configured=False; True: growthbook: integration is not configured |
| no ca_cert -> TLS verification fails (stays on) | PASS | configured=True; True: growthbook: GET /v1/projects: tls: failed to verify certificate: x509: certificate signed by unknown authority |
| base_url already ending in /api/ (multi-line PEM) | PASS | configured=True; False: {"hasMore":false,"projects":[{"description":"Sandbox checkout growth experiments","id":"prj_2ChD5kgpAZdu9q6y2XCTaZ","name":"SB Growth","owner":"","publicId":"my-first-project"}],"total":1} pinned  |
| env overrides GROWTHBOOK_API_KEY/BASE_URL/CA_CERT | PASS | configured=True; False: {"hasMore":false,"projects":[{"description":"Sandbox checkout growth experiments","id":"prj_2ChD5kgpAZdu9q6y2XCTaZ","name":"SB Growth","owner":"","publicId":"my-first-project"}],"total":1} pinned  |
| garbage ca_cert rejected at configure | PASS | configured=False; True: growthbook: integration is not configured |

### Caveats (INFO — not failures)

- **Hosting:** official `growthbook/growthbook:latest@sha256:f7a708cc…` image (the digest the PR's live test pins; build `5.2.0+ffe6dab`), layers pulled from Docker Hub and run under `chroot` on the box (no Docker daemon). MongoDB 8.0.4 tarball, PostgreSQL 17 warehouse, a local TLS proxy with a sandbox CA in front of the API (the adapter requires HTTPS).
- **Admin key (INFO):** self-hosted OSS only issues admin-role Secret Keys (`readonly` → HTTP 402), so the proof used an admin key, as the PR docs say.
- **Not live-tested (INFO):** the `pending_confirmation` write path (Cloud/commercial) and redirect refusal (no redirecting host). Unit tests and the TestUIGrowthBook fixture cover both.
- **Minor (INFO, not a bug):** `growthbook_toggle_feature` with `enabled:"true"` (a string) is accepted and coerced. Toggling an environment to the state it already has returns `status:"updated"` with an unchanged revision version.
- **Secrets:** a leak scan of every MCP response and Switchboard log found no API key, admin password, warehouse password, `encryptionKey`, `proxySigningKey` or `secureAttributeSalt`. The proxy log shows the key only in the bearer header, never in a query string.

Artifacts on branch `sandbox-e2e-proofs` at `3df88f6b368ebff2e2b6fcf69039f52b1f47600b` (not in this PR diff): [proof.mp4](https://github.com/daltoniam/switchboard/blob/sandbox-e2e-proofs/docs/sandbox-proofs/growthbook-397/proof.mp4) · [REPORT.md](https://github.com/daltoniam/switchboard/blob/sandbox-e2e-proofs/docs/sandbox-proofs/growthbook-397/REPORT.md)

<details><summary>Full report</summary>

# Switchboard PR #397: GrowthBook native REST sandbox E2E proof

**Date:** 2026-11-19 16:47 CST  
**PR:** https://github.com/daltoniam/switchboard/pull/397 ("Add GrowthBook feature flag and experiment integration", by Dalton)  
**Head SHA:** `41f207e07389bb64f634b4970c9940ad60f5e5b3` (single commit on main `9cab5d5`)  
**Machine:** Grok Bot box (Debian 13, no Docker/podman)

**Ready on the sandbox side, pending human review.** On `41f207e`, all 28 tools pass end to end through Switchboard MCP (native REST over HTTPS) against a real self-hosted GrowthBook 5.2.0 with a live warehouse analysis. The 3 write tools only touched sandbox data. Configuration and auth guards behave as documented, and secrets stay scrubbed. No bugs found; open items are INFO only. Human review is still required, and nothing was merged.

**Totals (build + MCP E2E + config/auth + UI cross-check):** 78 PASS / 0 FAIL / 4 INFO

## Environment

| Item | Value |
|---|---|
| GrowthBook | official image `growthbook/growthbook:latest@sha256:f7a708cce78ea53e7b80d787de919fccab151188dbac85f89b916c87986b9e07` (same digest as `web/live_growthbook_test.go`), build **5.2.0+ffe6dab** (commit ffe6dab161ef, built 2026-11-19), layers pulled from Docker Hub and run via `sudo chroot` (no Docker) |
| GrowthBook API / UI | `http://127.0.0.1:3100` / `http://localhost:3000` (self-hosted OSS, org "Switchboard Sandbox 397") |
| HTTPS front | local Go reverse proxy `https://localhost:3443` → :3100, leaf cert signed by a sandbox CA, CA passed as `ca_cert` (literal `\n` form in the main run, multi-line PEM and env-var form in config checks) |
| MongoDB | 8.0.4 (official tarball) on 127.0.0.1:27117 |
| Warehouse | PostgreSQL 17 on 127.0.0.1:55432; 400 exposures (`experiment_viewed`) + 173 purchases, the same seed SQL as the PR's live test |
| API key | admin-role Secret Key `secret_…` (redacted; stored in a 600-mode file outside the artifacts) |
| Switchboard | `go build ./cmd/server` at `41f207e`, isolated `HOME`/`SWITCHBOARD_DEV_HOME` per instance, MCP at `http://127.0.0.1:13897/mcp` (config checks on 13898–13905) |
| Log line | `Configured integration "growthbook" with 28 tools` |
| Transport | native REST (`growthbook_*` adapter, bearer auth); remotemcp not used |

## Seeded sandbox data

Project `SB Growth`, environments production + staging, data source `SB Warehouse` (Postgres, assignment query "Experiment Viewed"), fact table `Purchases`, fact metric `Purchase Rate`, legacy metric `Legacy Purchases`, segment `Big Spenders`, saved group `Beta Testers`, attribute `country`, SDK connection `SB Web`. Flags: `sb-checkout-express` (boolean, staging on), `sb-beta-banner` (string, prerequisite, force rule for the saved group, 10% rollout, toggled on in production with reason "sandbox launch"), and experiment-ref flags `sb-checkout-flow` / `sb-banner-flow`. Experiments: `SB Checkout Test` (running, analyzed against the warehouse) and `SB Banner Test` (stopped, won). Plus a code ref. Ids are in `harness/ids.json`.

Environment note: mid-run, mongod (dbpath under /workspace) aborted on box snapshot files (`*.box-store-snap-*`) appearing in its WiredTiger dir. The data was copied to `/var/tmp/gb-mongo-data` and mongod restarted with nothing lost. This happened after both MCP runs and before the last two UI screenshots, and has nothing to do with the PR.

## Per-tool results (28 tools, via Switchboard MCP `execute`)

| Tool | Access | Result | Calls | Evidence |
|---|---|---|---|---|
| `growthbook_list_features` | read | PASS | 5 | 4 seeded flags + rules/saved group; project filter, limit/offset paging (hasMore), limit=500 clamped to 100, limit=0 rejected |
| `growthbook_get_feature` | read | PASS | 5 | boolean flag and string flag with force (saved group) + 10% rollout rules and prerequisite; 404 and `../` id rejected client-side |
| `growthbook_get_feature_staleness` | read | PASS | 2 | per-env staleByEnv for 2 flags; 101 ids rejected client-side |
| `growthbook_list_feature_revisions` | read | PASS | 2 | published history ("Created via REST API"); default drafts → empty |
| `growthbook_list_experiments` | read | PASS | 3 | running + stopped (won) experiments; status=running/stopped filters exact |
| `growthbook_get_experiment` | read | PASS | 3 | hypothesis, variations, linked flag, "Banner won" analysis; unknown id 404 |
| `growthbook_get_experiment_results` | read | PASS | 1 | live Postgres analysis: 400 users, SRM healthy, Express chanceToBeatControl 0.999998; matches UI |
| `growthbook_refresh_experiment_results` | write | PASS | 1 | started snapshot snp_… against sandbox warehouse |
| `growthbook_get_snapshot` | read | PASS | 2 | snapshot from refresh reaches success; unknown id 404 |
| `growthbook_list_fact_metrics` | read | PASS | 2 | Purchase Rate; fact_table_id filter |
| `growthbook_get_fact_metric` | read | PASS | 1 | proportion metric details |
| `growthbook_list_metrics` | read | PASS | 1 | legacy binomial metric (met_) |
| `growthbook_list_fact_tables` | read | PASS | 1 | Purchases with columns from warehouse |
| `growthbook_list_projects` | read | PASS | 1 | SB Growth project |
| `growthbook_list_environments` | read | PASS | 1 | production + staging |
| `growthbook_list_sdk_connections` | read | PASS | 1 | SB Web sdk- key; encryption/proxy keys scrubbed |
| `growthbook_list_saved_groups` | read | PASS | 1 | Beta Testers |
| `growthbook_get_saved_group` | read | PASS | 2 | list values sb-user-1/2; unknown id 404 |
| `growthbook_list_attributes` | read | PASS | 1 | country attribute with description |
| `growthbook_list_data_sources` | read | PASS | 1 | SB Warehouse + assignment query; connection params (password) scrubbed |
| `growthbook_list_segments` | read | PASS | 1 | Big Spenders SQL segment |
| `growthbook_list_audit_logs` | read | PASS | 4 | feature.toggle with reason; before=nextCursor paging; missing entity_type rejected |
| `growthbook_list_code_refs` | read | PASS | 1 | acme/web src/checkout.ts |
| `growthbook_get_feature_code_refs` | read | PASS | 1 | line 42 ref for sb-checkout-express |
| `growthbook_get_settings` | read | PASS | 1 | bayesian/confidenceLevel; secureAttributeSalt scrubbed |
| `growthbook_list_members` | read | PASS | 1 | admin member |
| `growthbook_toggle_feature` | write | PASS | 6 | production off→on→off verified by direct REST, staging untouched; unknown env/feature errors; missing env rejected |
| `growthbook_create_feature` | write | PASS | 6 | boolean/string/json flags created disabled in all envs with no rules (verified by direct REST); duplicate, bad type, bad key rejected |

MCP `search` for "growthbook" returns all 28 tools; targeted searches surface `growthbook_get_experiment_results` and `growthbook_toggle_feature`.

## Configuration / auth checks (separate Switchboard instances)

| Config / auth check | Result | Evidence |
|---|---|---|
| sdk- client key rejected at configure | PASS | configured=False; True: growthbook: integration is not configured |
| bogus secret key -> 401 surfaced | PASS | configured=True; True: growthbook API error (401): {"message":"Invalid API key"} |
| plain http base_url rejected | PASS | configured=False; True: growthbook: integration is not configured |
| base_url with credentials rejected | PASS | configured=False; True: growthbook: integration is not configured |
| no ca_cert -> TLS verification fails (stays on) | PASS | configured=True; True: growthbook: GET /v1/projects: tls: failed to verify certificate: x509: certificate signed by unknown authority |
| base_url already ending in /api/ (multi-line PEM) | PASS | configured=True; False: {"hasMore":false,"projects":[{"description":"Sandbox checkout growth experiments","id":"prj_2ChD5kgpAZdu9q6y2XCTaZ","name":"SB Growth","owner":"","publicId":"my-first-project"}],"total":1} pinned  |
| env overrides GROWTHBOOK_API_KEY/BASE_URL/CA_CERT | PASS | configured=True; False: {"hasMore":false,"projects":[{"description":"Sandbox checkout growth experiments","id":"prj_2ChD5kgpAZdu9q6y2XCTaZ","name":"SB Growth","owner":"","publicId":"my-first-project"}],"total":1} pinned  |
| garbage ca_cert rejected at configure | PASS | configured=False; True: growthbook: integration is not configured |

## Caveats

- **Hosting:** official `growthbook/growthbook:latest@sha256:f7a708cc…` image (the digest the PR's live test pins; build `5.2.0+ffe6dab`), layers pulled from Docker Hub and run under `chroot` on the box (no Docker daemon). MongoDB 8.0.4 tarball, PostgreSQL 17 warehouse, a local TLS proxy with a sandbox CA in front of the API (the adapter requires HTTPS).
- **Admin key:** self-hosted OSS only issues admin-role Secret Keys (`readonly` → HTTP 402), so the proof used an admin key, as the PR docs say.
- **Not reachable here:** the `pending_confirmation` write path (Cloud/commercial) and redirect refusal (no redirecting host). Unit tests and the TestUIGrowthBook fixture cover both.
- **Minor (INFO, not a bug):** `growthbook_toggle_feature` with `enabled:"true"` (a string) is accepted and coerced. Toggling an environment to the state it already has returns `status:"updated"` with an unchanged revision version.
- **Secrets:** a leak scan of every MCP response and Switchboard log found no API key, admin password, warehouse password, `encryptionKey`, `proxySigningKey` or `secureAttributeSalt`. The proxy log shows the key only in the bearer header, never in a query string.

## Merge readiness

**Ready on the sandbox side, pending human review.** On `41f207e`, all 28 tools pass end to end through Switchboard MCP (native REST over HTTPS) against a real self-hosted GrowthBook 5.2.0 with a live warehouse analysis. The 3 write tools only touched sandbox data. Configuration and auth guards behave as documented, and secrets stay scrubbed. No bugs found; open items are INFO only. Human review is still required, and nothing was merged.

## Artifacts

`/workspace/sb-sandbox-proofs/growthbook-397/`: REPORT.md, pass-fail.md, handoff-snippet.md, proof.mp4, responses.json (MCP E2E steps), responses-config.json, responses.json.all (every step), mcp-responses.json (raw JSON-RPC), responses/NN-*.json (one per call), screenshots/*.png, logs/ (go vet/test, harness runs, Switchboard logs, TLS proxy request log), harness/ (seed.py, run-e2e.py, run-config-neg.py, start-sb.sh, mcpc.py, finalize.py, ids.json).

No commits, pushes, PR comments or merges were made.


</details>
