### Sandbox E2E proof

**Sandbox E2E PARTIAL on `f4d7153` — offline/validation proven; live dispatch pending sandbox GitHub account.** Head `f4d7153ee61ed9ad925d065f1ee13f362cf35869`.

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

Artifacts: `sandbox-e2e-proofs` → `github-307/` (REPORT.md, proof.mp4, responses.json, mcp-responses.json, screenshots/).
Caveat: object values aren't type-checked client-side (numbers/nested objects are forwarded to GitHub).
