# Switchboard PR #303 — Cloudflare Access + tunnels live proof

**Date:** 2026-10-08 ~14:50 CDT (America/Chicago)  
**Repo:** https://github.com/daltoniam/switchboard  
**PR:** https://github.com/daltoniam/switchboard/pull/303  
**Branch:** `feat/cloudflare-access-write`  
**Head SHA:** `9a85a2b15159c6676e4a058326231506b8db3922` (`9a85a2b`)  
**Machine:** box (Linux) — native REST only, no remotemcp  
**Do not merge.**

## Status: LIVE_RUN_COMPLETE — tunnel PASS; Access BLOCKED (DALTON_BLOCKER_PAYMENT)

Live MCP harness exercised against a free Cloudflare sandbox account. Tunnel create/config/token/delete all PASS. Access/Zero Trust tools return `access.api.error.not_enabled` because Free Zero Trust checkout requires a payment method. No zone → DNS step skipped. Tunnel token redacted everywhere.

## Sandbox

| Item | Value |
|------|-------|
| Type | Free Cloudflare account (Zero Trust **not** enabled — payment gate) |
| Account ID | `c730354fa3222b757c4ffb96a9664c41` |
| API | `https://api.cloudflare.com/client/v4` (native REST) |
| Creds | `CLOUDFLARE_API_TOKEN` + `CLOUDFLARE_ACCOUNT_ID` via env overlay — **not** written into config.json |
| Secrets path | `/home/box/.sb-sandbox-secrets/cloudflare-303.env` (chmod 600) |
| Zone | none (DNS skipped; do not buy domains) |
| Token scope | Access Edit + Tunnel Edit (verify id `145ef48ff2a0cfd843898461889c0e73`) |

## Switchboard under test

| Item | Value |
|------|-------|
| Binary | `/workspace/sb-cloudflare-proof-20261008/switchboard/dist/switchboard` |
| Version | `switchboard v2026.1002.0-1-g9a85a2b (commit: 9a85a2b)` |
| Isolated home | `HOME=/workspace/sb-cloudflare-proof-20261008/switchboard-home` |
| Port | `13849` → MCP `http://127.0.0.1:13849/mcp` |
| Tools registered | **112** Cloudflare tools |

## PASS / FAIL / BLOCKED

| Step | Tool | Result |
|------|------|--------|
| search | `search` | PASS — 9/9 new tools visible |
| validation_reject_scheme | `cloudflare_create_access_app` | PASS — domain with `https://` rejected locally |
| create_tunnel | `cloudflare_create_tunnel` | PASS — id `34380120-998d-425f-8703-df4411c95ba6` |
| update_tunnel_config | `cloudflare_update_tunnel_config` | PASS — hostname+service ingress |
| get_tunnel_config | `cloudflare_get_tunnel_config` | PASS — `http_status:404` catch-all present |
| get_tunnel_token | `cloudflare_get_tunnel_token` | PASS — token retrieved; **[REDACTED]** in all artifacts |
| create_dns_record | `cloudflare_create_dns_record` | SKIPPED — no ZONE_ID / no zone |
| create_access_app | `cloudflare_create_access_app` | BLOCKED — `access.api.error.not_enabled` (403); Free ZT needs payment |
| create_access_app_policy_allow | `cloudflare_create_access_app_policy` | BLOCKED — same |
| create_access_app_policy_bypass | `cloudflare_create_access_app_policy` | BLOCKED — same |
| get_access_app | `cloudflare_get_access_app` | BLOCKED — same |
| delete_access_app_policy | `cloudflare_delete_access_app_policy` | BLOCKED — same (placeholder ids; Access not enabled) |
| delete_access_app | `cloudflare_delete_access_app` | BLOCKED — same (placeholder id) |
| delete_tunnel | `cloudflare_delete_tunnel` | PASS — `deleted_at` set; 0 active tunnels |

## What was proven

1. **Search / discovery:** All 9 new PR tools are registered and searchable.
2. **Local validation:** `cloudflare_create_access_app` rejects domains with a scheme (`https://…`) before hitting Cloudflare.
3. **Tunnel write path:** create → update ingress → get confirms adapter-appended `http_status:404` catch-all → get token (redacted) → delete.
4. **Access path (negative):** With ZT disabled, every Access tool correctly surfaces Cloudflare’s `access.api.error.not_enabled` 403 through Switchboard’s MCP error channel (not a silent success).
5. **Cleanup:** Proof tunnel deleted; no leftover Access apps (none could be created); no DNS records.

## Caveats

- **DALTON_BLOCKER_PAYMENT:** Enabling Cloudflare Access / Zero Trust Free still requires a payment method at checkout. Until that (or an alternate account with ZT already on) is available, Access create/get/policy/delete cannot be proven live.
- **No zone:** Proxied CNAME via `cloudflare_create_dns_record` was not exercised. Adding a free zone without purchase was not available; do not buy domains for this proof.
- **Token list/revoke:** Current scoped API token cannot list/revoke user tokens (`9109 Unauthorized`). Optional cleanup of any leftover Workers-template token from signup **not performed** — needs a token with User Tokens permission or dashboard action.
- **create_tunnel response** includes Cloudflare’s `token` / `TunnelSecret` fields; harness redacts them in `responses.json` / `mcp-responses.json` / logs. `get_tunnel_token` is stored only as `[REDACTED]`.
- Harness patch (local only, not committed to Switchboard): Access `not_enabled` is recorded as **BLOCKED** and the run continues through remaining Access probes + tunnel cleanup (`run-e2e.py` under `/workspace/sb-cloudflare-proof-20261008/`).

## Leftover resources

| Resource | State |
|----------|-------|
| Tunnel `34380120-998d-425f-8703-df4411c95ba6` | Deleted (`deleted_at` set); 0 active tunnels |
| Access apps / policies | None created |
| DNS | None |
| Switchboard on :13849 | Stopped after harness |
| Secrets / account | Left for CoS at `/home/box/.sb-sandbox-secrets/cloudflare-303.env` |

## Dalton actions (if Access E2E is required)

1. Add a payment method and enable Zero Trust Free on account `c730354…`, **or** hand a sandbox account that already has Access enabled.
2. Optionally add a zone you control (no purchase on this proof account) and set `ZONE_ID` + `DOMAIN` in the secrets file to unlock DNS + a real Access hostname.
3. Optional: revoke any leftover Workers-template API token from the dashboard (scoped token cannot list tokens).

## Merge readiness (one-liner)

**READY WITH CAVEATS** — tunnel + validation + error-path proven live at `9a85a2b`; Access happy-path blocked on Zero Trust Free payment method; DNS skipped (no zone). Do not merge; CoS/Dalton review.

## Artifacts

```
/workspace/sb-sandbox-proofs/cloudflare-303/
  REPORT.md
  pass-fail.md
  handoff-snippet.md
  responses.json
  mcp-responses.json
  proof.mp4                 (~20s slideshow)
  screenshots/01..08.png
  live-run-stdout.txt
  harness-run.log
  switchboard.log
  build.log
  smoke-skip-live/          (earlier SKIP_LIVE smoke)

/workspace/sb-cloudflare-proof-20261008/
  switchboard/ @ 9a85a2b
  run-e2e.sh / run-e2e.py   (local harness; Access BLOCKED patch)
  switchboard-home/
  artifacts/                (mirror of live outputs)
```
