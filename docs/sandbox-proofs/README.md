# Sandbox E2E proof artifacts

This directory holds **live end-to-end smoke proof** from local Docker sandboxes against open integration PRs. These files are for **PR reviewers** who want screen recordings, reports, and raw API/MCP captures—not product documentation.

| Folder | PR | Integration |
|--------|-----|-------------|
| [`airflow-274/`](airflow-274/) | [#274](https://github.com/daltoniam/switchboard/pull/274) | Airflow native REST |
| [`metabase-269/`](metabase-269/) | [#269](https://github.com/daltoniam/switchboard/pull/269) | Metabase OAuth / REST |
| [`cloudflare-303/`](cloudflare-303/) | [#303](https://github.com/daltoniam/switchboard/pull/303) | Cloudflare Access + tunnels |
| [`github-307/`](github-307/) | [#307](https://github.com/daltoniam/switchboard/pull/307) | GitHub `github_trigger_workflow` optional `inputs` |
| [`imessage-320/`](imessage-320/) | [#320](https://github.com/daltoniam/switchboard/pull/320) | iMessage (SQLite read + AppleScript send) |

**Branch:** `sandbox-e2e-proofs` (published separately so integration PR diffs stay focused on code).

Each subfolder typically includes:

- `REPORT.md` — what was run, sandbox setup, and honest limitations
- `proof.mp4` — short screen recording
- JSON/PNG extras — MCP or REST response captures and UI screenshots where applicable

Do not treat content here as user-facing docs; refer to the main `docs/` tree for Switchboard behavior and setup.
