# Switchboard PR #274 — Airflow native REST live proof

**Date:** 2026-10-05 (America/Chicago)  
**Repo:** https://github.com/daltoniam/switchboard  
**PR:** https://github.com/daltoniam/switchboard/pull/274  
**Branch:** `feat/airflow-integration`  
**Head SHA:** `81342d07fcd1a1d06d7edf7b7631bc8be4712508`  
**Machine:** MacBookPro.lan (Docker Desktop)

## Sandbox

| Item | Value |
|------|-------|
| Type | Self-hosted official Airflow docker-compose |
| Image | `apache/airflow:3.0.6` |
| API URL | `http://127.0.0.1:18080` (host remap; container still `:8080`) |
| Version probe | `GET /api/v2/version` → `{"version":"3.0.6"}` |
| Example DAGs | 81 (`AIRFLOW__CORE__LOAD_EXAMPLES=true`) |
| Credentials pattern | `base_url` + `username`/`password` → `POST /auth/token` JWT (or optional `access_token`) |
| Creds used | local compose defaults overridden to username `admin` / password `admin` (sandbox only; not committed) |

Port **18080** used because host **8080** was already bound by a local llama server.

## Switchboard under test

- Checkout: `/tmp/sb-airflow-proof-20261005/switchboard` @ `81342d07`
- Isolated home: `SWITCHBOARD_DEV_HOME=/tmp/sb-airflow-proof-20261005/switchboard-home` (never touched host Switchboard config)
- Server: `HOME=$SWITCHBOARD_DEV_HOME ./switchboard --port 13847`
- Config path: `$SWITCHBOARD_DEV_HOME/.config/switchboard/config.json` (Airflow only)
- Log line: `Configured integration "airflow" with 6 tools`

## Tools exercised

### A) Direct PR adapter harness (`integrations/airflow`)

| Step | Result |
|------|--------|
| Healthy | PASS |
| airflow_list_dags | PASS |
| airflow_get_dag | PASS |
| airflow_list_dag_runs | PASS |
| airflow_get_dag_run | PASS |
| airflow_list_task_instances | PASS |
| airflow_trigger_dag | PASS (created `manual__2026-10-05T15:25:23.077128+00:00_ktW6x7IT` on `asset1_producer`, state queued) |

### B) Switchboard MCP (`http://127.0.0.1:13847/mcp`)

Execute arg shape: `{ "tool_name": "...", "arguments": { ... } }`

| Step | Result |
|------|--------|
| search (airflow) | PASS (6 tools) |
| airflow_list_dags | PASS |
| airflow_get_dag | PASS |
| airflow_list_dag_runs | PASS |
| airflow_get_dag_run | PASS |
| airflow_list_task_instances | PASS |

## Artifacts (Mac)

```
/tmp/sb-airflow-proof-20261005/artifacts/REPORT.md
/tmp/sb-airflow-proof-20261005/artifacts/responses.json
/tmp/sb-airflow-proof-20261005/artifacts/mcp-responses.json
/tmp/sb-airflow-proof-20261005/artifacts/drilldown.json
/tmp/sb-airflow-proof-20261005/artifacts/harness-run.log
/tmp/sb-airflow-proof-20261005/artifacts/proof.mp4          (~10s slideshow)
/tmp/sb-airflow-proof-20261005/artifacts/screenshots/*.png
```

Secrets redacted from auth token dumps. Desktop `screencapture` blocked by macOS permissions; proof video is a step slideshow from generated frames.

## 5-line summary (what worked / what's broken)

1. **Worked:** Official Airflow 3.0.6 docker-compose API healthy on loopback; JWT auth via `/auth/token` succeeded.  
2. **Worked:** PR adapter Healthy + all 6 tools (list/get DAGs, runs, task instances, trigger) against live sandbox.  
3. **Worked:** Isolated Switchboard on `:13847` configured Airflow and MCP `search`/`execute` returned real compacted results.  
4. **Caveat:** Host `:8080` busy — used `:18080`; triggered run stayed `queued` briefly (worker warm-up), still visible via list/get APIs.  
5. **Caveat:** macOS Screen Recording permission blocked live UI screencapture; slideshow `proof.mp4` used instead.

## Merge readiness

**READY WITH CAVEATS** — live native REST path proven end-to-end; do **not** merge without human review (per task constraint). No Dalton email/phone verification needed for this sandbox.

## Dalton actions needed

None for auth/verification. Optional cleanup: `docker compose -f /tmp/sb-airflow-proof-20261005/airflow-docker/docker-compose.yaml down -v` and remove `/tmp/sb-airflow-proof-20261005` when finished. Stop isolated Switchboard PID on port 13847.
