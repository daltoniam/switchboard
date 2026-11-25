# Render

The `render` integration proxies Render's official hosted MCP server at `https://mcp.render.com/mcp` (source: [render-oss/render-mcp-server](https://github.com/render-oss/render-mcp-server)). Switchboard sends your Render API key as a bearer token; nothing runs locally.

## Setup

1. In the [Render Dashboard](https://dashboard.render.com), open **Account Settings → API Keys** and create a key (`rnd_...`).
2. In Switchboard, open **Integrations → render**, paste the key into `api_key`, enable the integration, and save. The badge turns **Connected** once `render_list_workspaces` succeeds with the key.
3. Optional: set `workspace_id` to the workspace (`tea-...` for a team, `usr-...` for a personal workspace) Switchboard should use when a call does not pass `workspaceId`. Find IDs with `render_list_workspaces`.

| Credential | Environment variable | Notes |
|---|---|---|
| `api_key` | `RENDER_API_KEY` | Required |
| `workspace_id` | `RENDER_WORKSPACE_ID` | Optional default workspace for scoped tools |
| `base_url` | `RENDER_MCP_BASE_URL` | Optional, default `https://mcp.render.com`; a trailing `/mcp` is accepted |

Render API keys have the full access of the account that created them in every workspace it belongs to. Render has no read-only or workspace-scoped keys, so use a dedicated account if you need least privilege. Write and destructive tools (`Access` write or destructive) are marked so hosts can gate them.

## Tools

48 tools mirror the hosted catalog: services and cron jobs, deploys, service events, logs and log labels, metrics, environment variables, Postgres (including read-only SQL as `render_query_postgres`), Key Value, Blueprints (early access upstream), and Workflows. Start with `render_list_services` or `render_list_logs`.

- `render_list_deploys` and `render_list_events` return `{items, next_cursor}`; pass `next_cursor` back as `cursor` for older entries.
- To restart a service, redeploy it with `render_trigger_deploy`.
- Render's MCP server cannot suspend, resume, roll back, delete services, or manage custom domains, disks, env groups, or projects. Use the Dashboard or REST API for those.
- `select_workspace` and `get_selected_workspace` are not exposed: they store the workspace in upstream session state that Switchboard would share between callers.

## Live acceptance

`web/live_render_test.go` is a read-only sandbox spec. It runs when `RENDER_LIVE_API_KEY`, `RENDER_LIVE_WORKSPACE_ID`, `RENDER_LIVE_SERVICE_ID` (a Git-backed web service that deployed, and so wrote build logs, in the last 7 days; the log and event cases read a 7-day window), `RENDER_LIVE_POSTGRES_ID`, and `RENDER_LIVE_KEY_VALUE_ID` are set, and skips otherwise.
