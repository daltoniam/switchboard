# Switchboard PR #307 — Sandbox E2E proof (GitHub `github_trigger_workflow` inputs)

- Generated: 2026-10-09 00:24 CDT
- PR: https://github.com/daltoniam/switchboard/pull/307 (branch `feat/github-dispatch-inputs`)
- Head SHA: `f4d7153ee61ed9ad925d065f1ee13f362cf35869`
- Status: **WAITING_FOR_SANDBOX_ACCOUNT**
- Sandbox repo: (not created — no sandbox account)
- Transport: local Switchboard (PR build) over MCP HTTP, isolated HOME/SWITCHBOARD_DEV_HOME, ports 13850 (offline, dummy token) / 13851 (live). Native REST via go-github → api.github.com.

## Summary
- Unit tests: `go test ./integrations/github/...` PASS; `go build ./cmd/server` + `go vet` PASS.
- Offline MCP phase (dummy token): 9 PASS / 0 FAIL / 1 INFO.
  - `search` exposes `github_trigger_workflow` with the new `inputs` parameter.
  - 5 malformed inputs (truncated JSON string, JSON-array string, number, array, bool) all return `parameter "inputs": ...` adapter errors with **no HTTP** — whereas the 3 valid controls (no inputs / object / JSON string) reach `POST api.github.com/.../dispatches` and get `401 Bad credentials`. That contrast is the "fails before the API" proof.
- Live: **WAITING_FOR_SANDBOX_ACCOUNT** — no non-daltoniam GitHub PAT on the box.

## Results
| Phase | Step | Tool | Result |
|---|---|---|---|
| build | go test ./integrations/github/... | go test | PASS — incl. TestTriggerWorkflow_Inputs (4) + _InvalidInputs (3) |
| build | go build ./cmd/server + go vet | go | PASS |
| offline | search | `search` | PASS — github_trigger_workflow listed with `inputs` param |
| offline | invalid_malformed_json | `github_trigger_workflow` | PASS — rejected client-side, no HTTP (no 401 from GitHub) |
| offline | invalid_json_array_string | `github_trigger_workflow` | PASS — rejected client-side, no HTTP (no 401 from GitHub) |
| offline | invalid_number | `github_trigger_workflow` | PASS — rejected client-side, no HTTP (no 401 from GitHub) |
| offline | invalid_array | `github_trigger_workflow` | PASS — rejected client-side, no HTTP (no 401 from GitHub) |
| offline | invalid_bool | `github_trigger_workflow` | PASS — rejected client-side, no HTTP (no 401 from GitHub) |
| offline | control_valid_object | `github_trigger_workflow` | PASS — reached api.github.com (401 with dummy token) — contrast with invalid cases |
| offline | control_valid_json_string | `github_trigger_workflow` | PASS — reached api.github.com (401 with dummy token) — contrast with invalid cases |
| offline | control_no_inputs | `github_trigger_workflow` | PASS — reached api.github.com (401 with dummy token) — contrast with invalid cases |
| offline | info_non_string_values_object | `github_trigger_workflow` | INFO — object with number/nested values is NOT validated client-side — forwarded to GitHub (401 reached API) |
| live | trigger_no_inputs | — | WAITING — no sandbox GitHub PAT |
| live | trigger_object_inputs | — | WAITING — no sandbox GitHub PAT |
| live | trigger_json_string_inputs | — | WAITING — no sandbox GitHub PAT |
| live | invalid_* ×5 + invalid_no_new_runs (REST count unchanged) | — | WAITING — no sandbox GitHub PAT |
| live | list_workflow_runs (run-name shows inputs) | — | WAITING — no sandbox GitHub PAT |
| live | get_run / list_jobs / job_logs ×3 (echoed inputs match) | — | WAITING — no sandbox GitHub PAT |

## Caveats
- Objects whose values are numbers/nested objects (e.g. `{"greeting":5}`) are **not** rejected client-side; only non-object `inputs` and unparseable/non-object JSON strings are. Such payloads are forwarded to GitHub (see `info_non_string_values_object`). Matches the PR's stated scope ("invalid inputs" = not an object); flagging for reviewer awareness.
- Blank-string `inputs` is treated as "no inputs" (unit-tested).
- Signed log-download URLs and all tokens are redacted in every artifact.

## Files
REPORT.md, pass-fail.md, responses.json, mcp-responses.json, handoff-snippet.md, proof.mp4, screenshots/, harness-run.log, go-test.log, build.log, workflow `dispatch.yml`.
