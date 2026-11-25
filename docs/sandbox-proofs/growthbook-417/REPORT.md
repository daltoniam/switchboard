# Switchboard PR #417: GrowthBook writes (rules + experiments) sandbox E2E proof

**Date:** 2026-11-25 12:00 CST  
**PR:** https://github.com/daltoniam/switchboard/pull/417 (branch `feat/growthbook-rules-experiments`, "Let GrowthBook tools edit flag rules and run experiments")  
**Tested SHA:** `58681b8a24a7240384074c67d3c6f907f9331b7f` (one commit on `08e9a12`)  
**Machine:** Grok Bot box (Debian 13, no Docker)

**READY TO MERGE from the sandbox side.** On `58681b8`, all 37 tools pass end to end through Switchboard MCP (native REST over HTTPS) against self-hosted GrowthBook 5.2.0. The 9 new write tools cover the full draft → publish and discard rule lifecycle and the experiment create → update → start (409 checklist, then success) → stop-with-rollout flow. Every gate override, ramp schedule, status change and unscoped new rule is refused before any request leaves Switchboard. All writes hit sandbox data only. Findings: 2 low (L1 update can unscope a rule, which is inert; L2 the audit comment isn't visible through list_audit_logs) and 1 nit, none blocking. Human review is still required, and nothing was merged.

**Totals (build + MCP E2E + UI cross-check + caveats):** 210 PASS / 0 FAIL / 6 INFO

## Environment

| Item | Value |
|---|---|
| GrowthBook | official image `growthbook/growthbook@sha256:f7a708cc…`, build **5.2.0+ffe6dab**, run via `sudo chroot` (rootfs `/workspace/gb-sandbox/rootfs`, `/dev` `/proc` `/sys` rbind-mounted) |
| API / UI | `http://127.0.0.1:3100` / `http://localhost:3000` (OSS org "Switchboard Sandbox 417") |
| HTTPS front | `https://localhost:3443` Go reverse proxy, sandbox CA passed as `ca_cert`; request log in `logs/tlsproxy-requests.log` |
| MongoDB | 8.0.4, dbpath `/var/tmp/gb-mongo-data`, 127.0.0.1:27117 |
| Warehouse | PostgreSQL 17, `/workspace/gb-sandbox/pg-data`, 127.0.0.1:55432 (400 exposures, 173 purchases) |
| API key | admin Secret Key `secret_…` (redacted), in `/home/box/.sb-sandbox-secrets/growthbook-417.env` (mode 600) |
| Switchboard | `go build ./cmd/server` at `58681b8`, isolated `HOME`/`SWITCHBOARD_DEV_HOME` (`/workspace/gb-sandbox/homes/a`), MCP `http://127.0.0.1:13897/mcp`; log `Configured integration "growthbook" with 37 tools` |

Box note: a box replacement since #397 had wiped the rootfs `dist/` folders and Postgres's empty directories. The rootfs was re-extracted from the cached image layers, and the empty `pg_*` dirs were recreated (the warehouse data was intact). Postgres 17 server binaries were reinstalled with apt.

## What the run did

1. `tools/list` and `search`: all 37 `growthbook_*` tools are discoverable, and targeted searches find the rule and experiment write tools.
2. Ran all 26 read tools, plus the toggle, create and refresh write tools as regression checks.
3. **Rule lifecycle** on `sb-beta-banner`: add a force rule to a new draft (titled "US market launch") → add a rollout to the same draft → update the condition to US+CA and the value to `na-on` → change the rollout coverage → delete the rollout → list the draft → publish v3 with the comment "Sandbox E2E PR 417: ship the North America force rule". The live rule was checked by direct REST and in the UI.
4. **Discard:** a draft rule on `sb-checkout-express` (sent as a JSON string) → discard → live revision unchanged, and the discarded draft can't be published.
5. **Guardrails:** checked new rule scope, all 7 gate overrides (`ignoreWarnings`, `skipSchemaValidation`, `skipHooks`, `skipChecklist`, `bypassApproval`, `allowUnscopedRules`, `bypassDuplicateKeyCheck`) on add, update, create_experiment and update_experiment, `rampSchedule` and `schedule`, and status changes through update. Every one was refused, with 0 requests reaching GrowthBook.
6. **Experiment** `SB 417 Market Test`: create a draft → start returns **409 checklist_incomplete** (goal metric + linked change) and the experiment stays a draft → update metrics, hypothesis and description → add an experiment-ref rule to a draft of a new flag → start (**running**; GrowthBook publishes the linked draft) → update while running → stop as **won** with `enable_temporary_rollout` of NA Banner (resultSummary.releasedVariationId set, excludeFromPayload false).
7. Edge probes, then **reran every read tool** and checked that the writes show up (new rule, revisions, the stopped experiment, audit events).

## Per-tool results (37 tools, via Switchboard MCP `execute`)

| Tool | Access | Result | Calls | Evidence |
|---|---|---|---|---|
| `growthbook_list_features` | read | PASS | 8 | new flags visible after writes |
| `growthbook_get_feature` | read | PASS | 9 | published NA rule visible after writes; discarded rule absent |
| `growthbook_get_feature_staleness` | read | PASS | 4 | same checks as #397, run before and after the writes |
| `growthbook_list_feature_revisions` | read | PASS | 8 | draft listed with rules before publish; published v3, discarded v4 and the start-published experiment flag revision listed after |
| `growthbook_list_experiments` | read | PASS | 8 | new experiment listed; status=stopped filter includes it |
| `growthbook_get_experiment` | read | PASS | 6 | won, released variation, analysis and linked flag after stop |
| `growthbook_get_experiment_results` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_refresh_experiment_results` | write | PASS | 2 | snapshot started against the warehouse (twice) and reached success |
| `growthbook_get_snapshot` | read | PASS | 3 | same checks as #397, run before and after the writes |
| `growthbook_list_fact_metrics` | read | PASS | 4 | same checks as #397, run before and after the writes |
| `growthbook_get_fact_metric` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_list_metrics` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_list_fact_tables` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_list_projects` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_list_environments` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_list_sdk_connections` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_list_saved_groups` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_get_saved_group` | read | PASS | 3 | same checks as #397, run before and after the writes |
| `growthbook_list_attributes` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_list_data_sources` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_list_segments` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_list_audit_logs` | read | PASS | 10 | feature.publish + feature.revision.update events and experiment start/stop listed; cursor paging; publish comment not in compact output (finding L2) |
| `growthbook_list_code_refs` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_get_feature_code_refs` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_get_settings` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_list_members` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_toggle_feature` | write | PASS | 6 | production off→on→off (direct REST) |
| `growthbook_create_feature` | write | PASS | 6 | flags created disabled, duplicate rejected |
| `growthbook_add_feature_rule` | write (new) | PASS | 21 | new draft (title/comment) force rule; 2nd rule into same version; JSON-string rule; experiment-ref rule; draft not live (direct REST). Rejected locally with 0 requests to GrowthBook: no environments / [] / allEnvironments:false, bad type, all 7 gate overrides, rampSchedule, schedule, non-object rule. GrowthBook 400s surfaced: unknown env, published version |
| `growthbook_update_feature_rule` | write (new) | PASS | 15 | condition/value/description change; rollout coverage 0.25→0.5. Rejected: 7 overrides + rampSchedule, type change, empty patch, version 0, edit of a published version. Edge: environments:[] accepted (finding L1) |
| `growthbook_delete_feature_rule` | write (new) | PASS | 3 | temp rollout removed from the draft and never went live; version "abc" and `../` rule_id rejected |
| `growthbook_publish_feature_revision` | write (new) | PASS | 5 | v3 published with audit comment; rule live in production (direct REST + UI); blank comment, discarded draft, unknown ignoreWarnings arg rejected; comment stored in audit details.context |
| `growthbook_discard_feature_revision` | write (new) | PASS | 1 | draft v4 discarded, live revision unchanged (direct REST + UI), discarded draft cannot be published |
| `growthbook_create_experiment` | write (new) | PASS | 11 | draft with project/datasource/assignmentQuery/variations; status:running, 7 overrides, 1 variation, missing trackingKey rejected with 0 requests to GrowthBook |
| `growthbook_update_experiment` | write (new) | PASS | 14 | goal + guardrail metric, hypothesis, description persisted (direct REST + UI); edit while running; status running/stopped/draft, linkedChanges, 7 overrides, empty changes rejected with 0 requests |
| `growthbook_start_experiment` | write (new) | PASS | 4 | 409 checklist_incomplete (goal metric + linked flag) surfaced, experiment stays draft; after update + experiment-ref draft, start → running and publishes the linked draft (publishedFeatureDrafts); extra skipChecklist arg rejected; unknown id 404 |
| `growthbook_stop_experiment` | write (new) | PASS | 4 | won + winner + enable_temporary_rollout of NA Banner → stopped, resultSummary.releasedVariationId set, excludeFromPayload false; invalid results, rollout without released id, `../` id rejected |

## Bugs / findings

No blocking bugs. Two low-severity findings and one nit, none of which let an agent bypass a GrowthBook gate:

- **L1 (low) `growthbook_update_feature_rule` doesn't re-check rule scope.** `add_feature_rule` requires `environments` or `allEnvironments: true`, but an update patch of `{"environments": []}` is forwarded. GrowthBook 5.2.0 accepts it, and the draft publishes with no `allowUnscopedRules` gate. The published rule is inert: it isn't in any environment's SDK definition, so production isn't targeted. It does break the "rule must name its environments" rule the tool states. Repro: add a staging rule, call update_feature_rule with `rule: {"environments": []}`, then publish. Suggested fix: reject a patch that leaves `environments` empty without `allEnvironments: true`, matching the add check.
- **L2 (low, compact spec) the publish audit comment can't be read back through MCP.** GrowthBook records the publish `comment` in the `feature.publish` audit event under `details.context.comment` (checked by direct REST). The `growthbook_list_audit_logs` compact spec keeps id, event, entity, user.name, reason and dateCreated and drops `details`, so neither the comment nor the rule.add/update/delete context shows. The spec predates this PR, but the new publish tool says the comment is "recorded in the audit log". Suggested fix: surface `details.context` (it is small) without the large pre/post blobs.
- **Nit:** the 409 from `growthbook_start_experiment` passes GrowthBook's text through word for word, including `(retry with "skipChecklist": true)`. The tool can't send that flag (it rejects it as an unknown parameter), so the agent may be confused. Adding "a person must resolve this in GrowthBook" to the error would match the tool description.

## Caveats

- **Hosting:** the official `growthbook/growthbook` image `5.2.0+ffe6dab` (the digest the PR's live test pins), with its layers re-extracted from the cached Docker Hub blobs and run under `chroot` (the box has no Docker). Also MongoDB 8.0.4, a PostgreSQL 17 warehouse, and a local TLS proxy with a sandbox CA (`ca_cert`). Native REST only; remotemcp was not used.
- **Fresh state:** Mongo was wiped and re-seeded for the final run (`harness/reset-and-seed.sh`). Warehouse timestamps were moved to the last 24 hours so the seeded experiment window covers them, because the #397 data had aged out of the 5-day phase.
- **Admin key:** self-hosted OSS only issues admin-role Secret Keys (`readonly` → HTTP 402).
- **Not reachable on OSS (INFO):** the `pending_confirmation` 202 path, required reviews and approvals. The gate-as-error behavior was shown live through the launch checklist 409, and the unit tests cover the 202 path.
- **GrowthBook limitation (INFO):** revision titles are stored (the UI shows "US market launch") but GrowthBook's REST revision payload leaves them out, so the adapter can't echo them.
- **Guardrails are client-side:** for every rejected override, ramp, status and scope call, the TLS proxy log shows **0** requests reaching GrowthBook. Unknown top-level arguments (such as `skipChecklist` on start, or `ignoreWarnings` on publish) are rejected by Switchboard's parameter validation.
- **Secrets:** the artifacts are scrubbed (`secret_…`, `sdk-…`, passwords). See leak-scan.txt.

## Merge readiness

**READY TO MERGE from the sandbox side.** On `58681b8`, all 37 tools pass end to end through Switchboard MCP (native REST over HTTPS) against self-hosted GrowthBook 5.2.0. The 9 new write tools cover the full draft → publish and discard rule lifecycle and the experiment create → update → start (409 checklist, then success) → stop-with-rollout flow. Every gate override, ramp schedule, status change and unscoped new rule is refused before any request leaves Switchboard. All writes hit sandbox data only. Findings: 2 low (L1 update can unscope a rule, which is inert; L2 the audit comment isn't visible through list_audit_logs) and 1 nit, none blocking. Human review is still required, and nothing was merged.

Human blockers: **none**. No email or phone verification, paid plan, or Dalton personal Google/GitHub account was needed.

## Sandbox left running / cleanup

GrowthBook, MongoDB, Postgres, the TLS proxy and Switchboard (PR build, :13897) are still running. Sandbox data created: flags `sb-agent-flag-29342`, `sb-417-market-flag-29342`, `sb-edge-unscoped-*`; experiment `exp_4w9jq1zx7mwwfck9f`; revisions on `sb-beta-banner` and `sb-checkout-express`. To clean up:

```bash
# stop Switchboard (PR build), GrowthBook, TLS proxy, MongoDB, Postgres
kill $(cat /workspace/gb-sandbox/homes/a/pid)
sudo pkill -f run-apps.js
pkill -f 'tlsproxy/tlsproxy'
/workspace/gb-sandbox/mongodb-linux-x86_64-debian12-8.0.4/bin/mongod --dbpath /var/tmp/gb-mongo-data --shutdown
/usr/lib/postgresql/17/bin/pg_ctl -D /workspace/gb-sandbox/pg-data stop
for m in sys proc dev; do sudo umount -R /workspace/gb-sandbox/rootfs/$m; done
# optional: remove state and secrets
rm -rf /var/tmp/gb-mongo-data /workspace/gb-sandbox/homes/a
rm -f /home/box/.sb-sandbox-secrets/growthbook-417.env /home/box/.sb-sandbox-secrets/growthbook-417-config.json
git -C /workspace/sb-growthbook-397 worktree remove /workspace/sb-growthbook-417
# restart instead: harness/reset-and-seed.sh (needs Postgres + TLS proxy up and rootfs mounts, see REPORT "Environment")
```

## Artifacts

`/workspace/sb-sandbox-proofs/growthbook-417/`: REPORT.md, pass-fail.md (every step), handoff-snippet.md, proof.mp4, responses.json (MCP E2E steps), responses.json.all (build + E2E + UI + caveats), mcp-responses.json (raw JSON-RPC), responses/NNN-*.json (one per call), screenshots/ (GrowthBook UI pages 01–07, terminal frames 10–16), logs/ (go vet/test, harness run, exploratory edge probes, Switchboard log, TLS proxy request log), harness/ (seed.py, reset-and-seed.sh, start-sb.sh, run-e2e.py, mcpc.py, finalize.py, scrub.py, leak-scan.py, ids.json, run-ids.json), leak-scan.txt.

No commits, pushes, PR comments, merges or publishing were done.
