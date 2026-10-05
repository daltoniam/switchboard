# Switchboard PR #269 — Metabase OAuth/remotemcp + REST proof

**Date:** 2026-10-05 (America/Chicago)  
**PR:** https://github.com/daltoniam/switchboard/pull/269  
**Branch/SHA:** `esjay/wayne/metabase-oauth` @ `f5a7e9d1c49e460468a9d536ec8d15c522295935`  
**Machine:** MacBookPro.lan (`9f64e4bf-deb7-4d88-ae42-8bd6731d5b86`) Docker Desktop  
**Do not merge from this proof.**

## Mode flags (read this first)

| Mode | Label | Status in this proof |
|------|-------|----------------------|
| **native REST (API key)** | Switchboard `token_source=api_key` + Metabase `x-api-key` | **PROVEN end-to-end** (preferred path) |
| **remotemcp (hosted MCP)** | Metabase `<site>/api/metabase-mcp` (+ OAuth when using Switchboard OAuth mode) | **Infrastructure PROVEN** on OSS Docker; hosted MCP tools exercised with API key. **Full browser OAuth consent → Switchboard remotemcp proxy NOT completed** (interactive Metabase login required). |

Dalton preference: native REST over remotemcp — REST path is the primary proof.

## Sandbox

- **Type:** Local Docker OSS Metabase (`metabase/metabase:latest` → **v0.63.19.1** / hash `d59b5c8`, image created 2026-10-01)
- **Container:** `sb-metabase-proof` on `0.0.0.0:3000`
- **Data:** Sample Database (sqlite id=1) shipped with Metabase
- **Admin:** local-only `proof-admin@localhost.local` (not a Dalton account; no email/phone verification)
- **Switchboard:** isolated `HOME` / `SWITCHBOARD_DEV_HOME=/tmp/sb-metabase-proof-20261005/switchboard-home`, port **13848** (avoids Airflow proof 13847)
- **No Metabase Cloud trial / no Dalton email click required**

## Honest note on OSS Docker vs hosted MCP

Contrary to a possible expectation that only Metabase Cloud exposes hosted MCP:

- Public `GET /api/session/properties` reports **`mcp-enabled?: true`** and **`mcp-execute-sql-enabled: true`** on this OSS image.
- `POST /api/metabase-mcp` works (Streamable HTTP) after auth.
- RFC 8414 `/.well-known/oauth-authorization-server` returns 200 after setup/`site-url`.
- Dynamic client registration at `/oauth/register` succeeds.
- Switchboard setup UI text: **"This instance reports its MCP server is enabled."**

OAuth authorization-code + PKCE consent in a browser was **not** completed in this unattended proof.

## What was exercised

### A) native REST (API key) — primary

Direct Metabase REST:

- `GET /api/database` → Sample Database
- `GET /api/search?q=Orders` → tables/cards/metrics

Go harness (`integrations/metabase` `Configure` + `Execute`, `token_source=api_key`):

| Tool | Result |
|------|--------|
| Healthy | PASS |
| Tools (23 REST tools) | PASS |
| metabase_list_databases | PASS |
| metabase_search | PASS |
| metabase_get_database | PASS |
| metabase_list_tables | PASS |
| metabase_list_cards | PASS |
| metabase_list_dashboards | PASS |
| metabase_execute_query (`SELECT COUNT(*) FROM ORDERS`) | PASS |

Isolated Switchboard HTTP MCP (`http://127.0.0.1:13848/mcp`) via progressive `search` → `execute`:

| Call | Result |
|------|--------|
| search "metabase list databases" | PASS (23 metabase tools) |
| execute metabase_list_databases | PASS |
| execute metabase_get_database | PASS |
| execute metabase_list_tables | PASS |
| execute metabase_execute_query | PASS |
| execute metabase_search | returned `{}` via MCP HTTP (harness REST search PASS — likely execute/arguments quirk; not a Metabase outage) |

### B) remotemcp (hosted MCP) — secondary / documented

Against Metabase hosted MCP with **API key** (not OAuth token):

- initialize → `serverInfo.name=metabase`
- tools/list → **15** remote tools: `search`, `query`, `execute_query`, `execute_sql`, `read_resource`, create/update dashboard/question/metric, etc.
- `search` with `term_queries: ["Orders"]` → real ORDERS table + metrics
- `search` with `semantic_queries` → example questions
- OAuth discovery + dynamic registration OK
- Switchboard UI probes MCP enabled and offers OAuth **or** API key

**Not done:** browser OAuth grant → Switchboard `mcp_access_token` / remotemcp proxy mode live tool calls.

## Five lines — what worked / what's broken

1. **WORKED:** native REST API-key path through Switchboard PR HEAD — list/search/get/query against Sample Database.
2. **WORKED:** OSS Docker Metabase v0.63.19.1 exposes `mcp-enabled?`, hosted `/api/metabase-mcp`, and OAuth discovery/registration.
3. **WORKED:** Hosted MCP `search` (term/semantic) returns real content when called directly with API key.
4. **GAP:** Full OAuth interactive consent → Switchboard remotemcp proxy mode not live-proven (needs browser login as Metabase user).
5. **MINOR:** Switchboard MCP HTTP `execute metabase_search` returned empty `{}` once; direct adapter Execute search succeeded — investigate argument forwarding if OAuth merge depends on MCP HTTP search.

## Merge readiness

- **REST fallback:** ready from this proof’s perspective (smoke-tested live).
- **OAuth/remotemcp:** code + fixture tests exist in PR; live OAuth E2E still incomplete here.
- **Recommendation:** merge-ready for **REST-first** users if CI is green; treat full OAuth E2E as follow-up or require a short interactive sign-in on a Metabase with MCP toggle on (this sandbox already has MCP on).
- **Do not merge from this agent.**

## Dalton action needed?

- **None required** for REST proof (local Docker admin + API key created in sandbox).
- **Optional follow-up:** open `http://127.0.0.1:13848/integrations/metabase/setup` and click **Sign in with Metabase** while `sb-metabase-proof` is still running, to finish OAuth remotemcp proof. Sandbox may still be up on ports 3000 / 13848.
- No Metabase Cloud email verification was needed.

## Artifact paths (Mac)

```
/tmp/sb-metabase-proof-20261005/artifacts/REPORT.md
/tmp/sb-metabase-proof-20261005/artifacts/responses-rest.json
/tmp/sb-metabase-proof-20261005/artifacts/harness-rest.log
/tmp/sb-metabase-proof-20261005/artifacts/sb-mcp-http-summary.json
/tmp/sb-metabase-proof-20261005/artifacts/remotemcp-tools-list.json
/tmp/sb-metabase-proof-20261005/artifacts/remotemcp-search-hit.json
/tmp/sb-metabase-proof-20261005/artifacts/oauth-discovery.json
/tmp/sb-metabase-proof-20261005/artifacts/oauth-dynreg.redacted.json
/tmp/sb-metabase-proof-20261005/artifacts/metabase-version.json
/tmp/sb-metabase-proof-20261005/artifacts/proof.mp4
/tmp/sb-metabase-proof-20261005/artifacts/metabase-home.png
/tmp/sb-metabase-proof-20261005/artifacts/sb-metabase-setup.png
/tmp/sb-metabase-proof-20261005/screenshots/*.png
```

Private (not for commit): `/tmp/sb-metabase-proof-20261005/config/metabase-api-key.txt`, Switchboard config under `switchboard-home/`.

## Process notes

- Switchboard PID listening on 13848; Metabase container `sb-metabase-proof`.
- Clone: `/tmp/sb-metabase-proof-20261005/switchboard` @ f5a7e9d.
- No commits of secrets; no PR merge; no user/CoS messaging.
