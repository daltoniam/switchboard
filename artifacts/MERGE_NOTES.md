# Airflow integration E2E proof (PR #274)

**Branch:** `feat/airflow-integration`  
**Proof log:** [`artifacts/airflow_e2e_proof.log`](airflow_e2e_proof.log)  
**Sandbox:** [`artifacts/airflow-e2e/`](airflow-e2e/) (Docker Compose + tiny DAG)

## Summary

| Area | Result |
|------|--------|
| Auth | `POST /auth/token` with JSON `username` / `password` → JWT `access_token` |
| API | Airflow **3.0.6** public REST `/api/v2/*` with `Authorization: Bearer …` |
| Switchboard adapter | All six tools exercised against live Airflow (native HTTP in `integrations/airflow`, **not** `remotemcp`) |
| Trigger | `airflow_trigger_dag` on `switchboard_proof` created run `manual__2026-10-05T15:37:25…` |
| Task instances | `airflow_list_task_instances` returned task `done` for that run |

**Merge recommendation:** **Approve** — adapter matches Airflow 3 JWT + v2 REST behavior in this sandbox; no code changes required on the branch for E2E.

## What was exercised

1. `airflow_list_dags` — found `switchboard_proof`
2. `airflow_get_dag` — metadata for `switchboard_proof`
3. `airflow_trigger_dag` — manual run with `conf: {"proof":"switchboard-e2e"}` (`conf` redacted on trigger response per adapter design; visible on `list_dag_runs` / `get_dag_run`)
4. `airflow_list_dag_runs` / `airflow_get_dag_run` — run state `queued` → scheduler progressed
5. `airflow_list_task_instances` — task `done` listed for the triggered run

Credentials path: `Configure()` → `POST {base_url}/auth/token` → Bearer on all `/api/v2/...` calls (same as unit tests in `airflow_test.go`).

## Reproduce (from repo root)

**Prerequisites:** Docker, Go 1.26+, `jq`, `curl`. On Linux cloud VMs you may need `sudo dockerd` running.

```bash
# 1) Airflow sandbox (Airflow 3 standalone in one container)
cd artifacts/airflow-e2e
chmod 666 config/simple_auth_manager_passwords.json   # airflow uid must write lock file
sudo docker compose up -d
# wait until: curl -sf http://localhost:8080/api/v2/version

# 2) Credentials (local only — do not commit overrides)
export AIRFLOW_BASE_URL=http://127.0.0.1:8080
export AIRFLOW_USERNAME=admin
export AIRFLOW_PASSWORD=switchboard-e2e   # matches config/simple_auth_manager_passwords.json

# 3) Full proof (auth smoke + adapter tools)
cd ../..
./artifacts/airflow-e2e/run_e2e.sh
# log: artifacts/airflow_e2e_proof.log
```

**Switchboard config (optional, for MCP `execute`):** enable `airflow` in `~/.config/switchboard/config.json` and set the same `AIRFLOW_*` env vars (see `config/config.go` env mapping). The proof script calls the registered integration via `go run ./artifacts/airflow-e2e/run_proof.go`, which is the same code path the server uses.

## Sandbox auth notes

- This proof uses Airflow’s **Simple Auth Manager** with a fixed password file so JWT login is deterministic. Production deployments often use **FAB**; the adapter still uses the same **`POST /auth/token`** contract documented for Airflow 3.
- Default compose user: `admin` / role `ADMIN`; password file: `artifacts/airflow-e2e/config/simple_auth_manager_passwords.json` (`switchboard-e2e`).
- DAG: `artifacts/airflow-e2e/dags/switchboard_proof.py` (`EmptyOperator` task `done`).

## Known limitations (sandbox only)

- `airflow standalone` is dev-only; not a production topology.
- FAB + `_AIRFLOW_WWW_USER_*` was not used here because standalone sqlite/FAB user bootstrap was flaky in this environment; Simple Auth matches the adapter’s JWT flow.

## Teardown

```bash
cd artifacts/airflow-e2e && sudo docker compose down -v
```
