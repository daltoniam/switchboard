# Switchboard

A source-available MCP gateway that connects any MCP client to your tools
behind one endpoint. Run it locally with a built-in web UI, or use
[Switchboard Hosted](https://app.switchboard-mcp.com) for a private-data
tunnel and other team features.

Switchboard sits under Cursor, Claude Code, Codex, and any other MCP client.
Your agent searches for a capability and executes it. Compaction strips unused
fields before responses reach the model, typically cutting token usage by
about 90%.

![Local Switchboard dashboard with connected integrations, token savings, and activity](docs/images/ui-dashboard.webp)

*The local dashboard tracks connected tools and how much LLM context Switchboard kept off the wire.*

![Local Switchboard integrations page with Google Workspace, connected services, and available adapters](docs/images/ui-integrations.webp)

*Connect GitHub, Datadog, Linear, Slack, Google Workspace, and more from the local UI.*

## Switchboard Hosted

Need agents to reach Postgres or Kubernetes behind a firewall? [Switchboard
Hosted](https://app.switchboard-mcp.com) adds a private-data tunnel, plus
organizations, SSO, policies, audit logs, and dedicated runtimes. Same MCP
clients and integrations as this repo.

- **[Try Switchboard Hosted free](https://app.switchboard-mcp.com)** — no credit card required
- **[Product and pricing](https://switchboard-mcp.com)**

## Features

- **One MCP endpoint** for GitHub, Datadog, Linear, Slack, Google Workspace, AWS, and more
- **Local web UI** to connect integrations, check health, and watch token savings
- **Search + execute** so agents discover tools instead of loading every schema
- **~90% fewer tokens** via compaction, columnar reshape, and markdown rendering
- **Any MCP client, any model** — Cursor, Claude Code, Codex, Windsurf, and others
- **Bring your own integrations** with the Go `mcp.Integration` interface or Wasm plugins

## Installation

### Homebrew (macOS / Linux)

```bash
brew install daltoniam/tap/switchboard
```

### Scoop (Windows)

```powershell
scoop bucket add daltoniam https://github.com/daltoniam/scoop-bucket
scoop install switchboard
```

### Debian / Ubuntu (.deb)

```bash
# Download the latest .deb from GitHub releases
curl -LO "https://github.com/daltoniam/switchboard/releases/latest/download/switchboard_$(curl -s https://api.github.com/repos/daltoniam/switchboard/releases/latest | grep tag_name | cut -d '"' -f4 | tr -d v)_linux_amd64.deb"
sudo dpkg -i switchboard_*.deb
```

### Fedora / RHEL (.rpm)

```bash
# Download the latest .rpm from GitHub releases
curl -LO "https://github.com/daltoniam/switchboard/releases/latest/download/switchboard_$(curl -s https://api.github.com/repos/daltoniam/switchboard/releases/latest | grep tag_name | cut -d '"' -f4 | tr -d v)_linux_amd64.rpm"
sudo rpm -i switchboard_*.rpm
```

### Arch Linux (AUR)

```bash
yay -S switchboard-bin
```

### Alpine Linux (.apk)

```bash
# Download the latest .apk from GitHub releases
curl -LO "https://github.com/daltoniam/switchboard/releases/latest/download/switchboard_$(curl -s https://api.github.com/repos/daltoniam/switchboard/releases/latest | grep tag_name | cut -d '"' -f4 | tr -d v)_linux_amd64.apk"
sudo apk add --allow-untrusted switchboard_*.apk
```

### Go Install

```bash
go install github.com/daltoniam/switchboard/cmd/server@latest
```

### Download Binary

Pre-built binaries for macOS, Linux, and Windows (amd64/arm64) are available on
the [GitHub Releases](https://github.com/daltoniam/switchboard/releases) page.

## Context Optimization

API responses are large. A single GitHub issue carries ~100 fields (nested users, permissions, node IDs, avatar URLs) when an LLM needs ~10 to decide what to do next. Multiply by 30 issues per page and a list call can consume 150KB of context for information the model will never use.

Switchboard solves this automatically. Integrations declare **compaction specs** that describe which fields matter for each tool. The server strips everything else after every `execute` call, before responses reach the LLM.

List and search responses are compact by default. When the LLM identifies a specific item and calls a single-item `get` tool, it gets the full response back for drill-down.

## Quick Start

Project Catalog tools (`project.*`) and `project://` resources are on the main
`/mcp` endpoint by default. See [docs/project-catalog.md](docs/project-catalog.md).

Compiled tools can use the strongly typed [AWM gRPC API](docs/awm-grpc.md)
over loopback h2c (default `127.0.0.1:3847`) or an optional Unix-domain socket.
Its generated services mirror the Project Catalog and AWM MCP operations
without generic JSON arguments or results. The typed surface includes
Projects, Resources, ResourceBindings, WorkProfiles, AgentProfiles, and
WorkSessions. A Rust client crate lives in [`rust/switchboard-awm`](rust/switchboard-awm).

```bash
# Run (default — HTTP/MCP + native AWM gRPC on 127.0.0.1:3847)
switchboard

# Custom port, still loopback
switchboard --port 8080

# Opt in to a non-loopback TCP bind (also exposes HTTP/MCP)
switchboard --listen-host 0.0.0.0 --port 3847

# Native AWM gRPC only on a Unix socket (HTTP/MCP stay on TCP)
switchboard --grpc-socket /tmp/switchboard-awm.sock

# Stdio mode (for Cursor/Claude Desktop)
switchboard --stdio

# Check version
switchboard --version

# Debug logging (search/execute requests, compaction savings)
switchboard --verbose

# Open config UI
open http://localhost:3847
```

## Architecture

```
┌─────────────┐     stdio / SSE      ┌──────────────────────┐
│  AI Client   │ ◄──────────────────► │  Unified MCP Server   │
│ (Cursor, etc)│                      │                       │
└─────────────┘                      │  ┌─────────────────┐  │
                                     │  │  Tool Router     │  │
       ┌──────────────────┐          │  └────────┬────────┘  │
       │  Web UI (3847)   │◄─ HTTP ─►│           │           │
       │  config/creds    │          │  ┌────────▼────────┐  │
       └──────────────────┘          │  │  Adapters        │  │
                                     │  │  ├─ GitHub       │  │
                                     │  │  ├─ Datadog      │  │
                                     │  │  ├─ Linear       │  │
                                     │  │  ├─ Slack        │  │
                                     │  │  └─ more         │  │
                                     │  └─────────────────┘  │
                                     └──────────────────────┘
```

## Configuration

Config lives at `~/.config/switchboard/config.json`. The web UI is a
convenience layer over this file — you can also edit it by hand.

```json
{
  "integrations": {
    "github": {
      "enabled": true,
      "credentials": {
        "token": "ghp_..."
      }
    },
    "datadog": {
      "enabled": true,
      "credentials": {
        "api_key": "...",
        "app_key": "..."
      }
    }
  }
}
```

### Slack official hosted MCP (`slackmcp`)

Separate from the native `slack` session-token adapter. Proxies Slack's hosted MCP at `https://mcp.slack.com` (optional `credentials.base_url` override; Switchboard appends `/mcp`).

Each named identity needs a **user OAuth access token** (`access_token`, typically `xoxp-...`). An app/bot may host the agent, but **`xoxb-` bot tokens cannot authenticate** Slack's hosted MCP endpoint.

```json
{
  "integrations": {
    "slackmcp": {
      "enabled": true,
      "credentials": {
        "base_url": ""
      },
      "identities": {
        "work": {
          "credentials": { "access_token": "xoxp-..." },
          "metadata": { "label": "Work", "team": "T0123WORK" }
        },
        "personal": {
          "credentials": { "access_token": "xoxp-..." },
          "metadata": { "label": "Personal", "team": "T0456HOME" }
        }
      }
    }
  }
}
```

- Start with `slackmcp_list_available_identites` (spelling is intentional) — returns identity IDs, metadata, and non-secret tool capability info (never tokens).
- Every other `slackmcp_*` tool requires `identity_id` selecting which configured identity to use.
- Upstream tools named `slack_*` are exposed once as `slackmcp_*` (not `slackmcp_slack_*`).

### Environment Variables

Switchboard automatically reads environment variables from your shell (fish, zsh, bash, etc.) and overlays them on top of the JSON config. If an env var is set, it takes precedence over the corresponding value in `config.json`. Env-sourced values are never written back to disk.

Environment variables override credential values but do not change the durable enabled state. Enable the integration explicitly in config or the web UI; transient startup failures never rewrite that choice.

| Integration | Credential | Env Var |
|---|---|---|
| GitHub | `token` | `GITHUB_TOKEN` |
| Datadog | `api_key` | `DD_API_KEY` |
| Datadog | `app_key` | `DD_APP_KEY` |
| Datadog | `site` | `DD_SITE` |
| Linear | `api_key` | `LINEAR_API_KEY` |
| Sentry | `auth_token` | `SENTRY_AUTH_TOKEN` |
| Sentry | `organization` | `SENTRY_ORG` (optional — auto-detected from API) |
| Slack | `token` | `SLACK_TOKEN` |
| Slack | `cookie` | `SLACK_COOKIE` |
| Slack MCP (official hosted) | multi-identity `access_token` | configure via `identities` in JSON (see below) |
| Metabase | `api_key` | `METABASE_API_KEY` |
| Metabase | `url` | `METABASE_URL` |
| Paperless-ngx | `token` | `PAPERLESS_TOKEN` |
| Paperless-ngx | `url` | `PAPERLESS_URL` |
| Recoll WebUI | `base_url` | `RECOLL_URL` |
| AWS | `access_key_id` | `AWS_ACCESS_KEY_ID` |
| AWS | `secret_access_key` | `AWS_SECRET_ACCESS_KEY` |
| AWS | `session_token` | `AWS_SESSION_TOKEN` |
| AWS | `region` | `AWS_REGION` |
| PostHog | `api_key` | `POSTHOG_API_KEY` |
| PostHog | `project_id` | `POSTHOG_PROJECT_ID` |
| PostHog | `base_url` | `POSTHOG_URL` |
| Postgres | `connection_string` | `DATABASE_URL` |
| Postgres | `host` | `PGHOST` |
| Postgres | `port` | `PGPORT` |
| Postgres | `user` | `PGUSER` |
| Postgres | `password` | `PGPASSWORD` |
| Postgres | `database` | `PGDATABASE` |
| Postgres | `sslmode` | `PGSSLMODE` |
| Jira | `email` | `JIRA_EMAIL` |
| Jira | `api_token` | `JIRA_API_TOKEN` |
| Jira | `domain` | `JIRA_DOMAIN` |
| Readarr | `api_key` | `READARR_API_KEY` |
| Readarr | `base_url` | `READARR_URL` |
| DigitalOcean | `api_token` | `DIGITALOCEAN_TOKEN` |
| Vercel | `api_token` | `VERCEL_API_TOKEN` |
| Vercel | `team_id` | `VERCEL_TEAM_ID` (optional — default team scope) |
| Vercel | `team_slug` | `VERCEL_TEAM_SLUG` (optional — default team scope) |
| Vercel | `base_url` | `VERCEL_BASE_URL` (optional — override API endpoint, e.g. tests/proxies) |
| Stripe | `api_key` | `STRIPE_API_KEY` |
| Stripe | `account` | `STRIPE_ACCOUNT` (optional — `Stripe-Account` header for Connect) |
| Stripe | `base_url` | `STRIPE_BASE_URL` (optional — override API endpoint, e.g. stripe-mock) |
| Gong | `access_key` | `GONG_ACCESS_KEY` |
| Gong | `access_key_secret` | `GONG_ACCESS_KEY_SECRET` |
| Gong | `base_url` | `GONG_BASE_URL` (optional — default `https://api.gong.io`) |
| HubSpot | `access_token` | `HUBSPOT_ACCESS_TOKEN` |
| HubSpot | `base_url` | `HUBSPOT_BASE_URL` (optional — default `https://api.hubapi.com`) |
| Intercom | `access_token` | `INTERCOM_ACCESS_TOKEN` |
| Intercom | `base_url` | `INTERCOM_BASE_URL` (optional — default `https://api.intercom.io`; EU `https://api.eu.intercom.io`, AU `https://api.au.intercom.io`) |
| Ramp | `access_token` | `RAMP_ACCESS_TOKEN` |
| Ramp | `base_url` | `RAMP_BASE_URL` (optional — default `https://api.ramp.com`, use `https://demo-api.ramp.com` for sandbox) |
| NetSuite | `account_id` | `NETSUITE_ACCOUNT_ID` |
| NetSuite | `access_token` | `NETSUITE_ACCESS_TOKEN` (OAuth 2.0; alternative to TBA) |
| NetSuite | `consumer_key` | `NETSUITE_CONSUMER_KEY` (TBA) |
| NetSuite | `consumer_secret` | `NETSUITE_CONSUMER_SECRET` (TBA) |
| NetSuite | `token_id` | `NETSUITE_TOKEN_ID` (TBA) |
| NetSuite | `token_secret` | `NETSUITE_TOKEN_SECRET` (TBA) |
| NetSuite | `base_url` | `NETSUITE_BASE_URL` (optional — default `https://{account}.suitetalk.api.netsuite.com`) |

### OAuth Setup

Some integrations support OAuth flows through the web UI at `http://localhost:3847`. This is the easiest way to get tokens for integrations that don't use simple API keys.

| Integration | Auth Method | Setup |
|---|---|---|
| GitHub | OAuth Device Flow | Web UI → GitHub → Setup, or set `GITHUB_TOKEN` |
| Linear | OAuth (PKCE) | Web UI → Linear → Setup, or set `LINEAR_API_KEY` |
| Sentry | OAuth Device Flow | Web UI → Sentry → Setup, or set `SENTRY_AUTH_TOKEN` |
| Slack | Session Token | Web UI → Slack → Setup (auto-extracts from Chrome), or set `SLACK_TOKEN` |
| Slack MCP (official hosted) | User OAuth access tokens per identity | Edit `~/.config/switchboard/config.json` `slackmcp.identities` (see below). Bot `xoxb-` tokens are **not** accepted by Slack's hosted MCP endpoint. |
| Datadog | API + App Key | Set `DD_API_KEY` and `DD_APP_KEY` env vars or enter in web UI |
| AWS | IAM Credentials | Set `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` env vars, or uses default credential chain |
| Metabase | API Key | Set `METABASE_API_KEY` and `METABASE_URL` env vars or enter in web UI |
| Paperless-ngx | API Token | Set `PAPERLESS_TOKEN` and `PAPERLESS_URL` env vars or enter in web UI |
| Recoll WebUI | Base URL | Set `RECOLL_URL` to the Recoll WebUI root (for example `http://localhost:8080`) or enter it in the web UI |
| PostHog | Personal API Key | Set `POSTHOG_API_KEY` env var or enter in web UI |
| Vercel | Personal Access Token | Set `VERCEL_API_TOKEN` env var or enter in web UI |
| Postgres | Connection String | Set `DATABASE_URL` env var or enter in web UI |

## Adding to Cursor / Claude Desktop

Add to your MCP client config:

```json
{
  "mcpServers": {
    "switchboard": {
      "command": "switchboard",
      "args": []
    }
  }
}
```

## Building from Source

```bash
git clone https://github.com/daltoniam/switchboard.git
cd switchboard
go build -o switchboard ./cmd/server
```

### Development

Install [air](https://github.com/air-verse/air) for live-reload during development:

```bash
go install github.com/air-verse/air@latest
```

Install the Playwright Chromium driver (optional — enables browser-based integrations):

```bash
go run github.com/playwright-community/playwright-go/cmd/playwright install chromium
```

Then run with live-reload:

```bash
air
```

Host `air` / `make build` remain the non-Docker path. For a worktree-isolated
Docker Compose DEV stack (ephemeral loopback publish + optional Stacklane
FQDNs, no provider tokens required) see [docs/dev-compose.md](docs/dev-compose.md):

```bash
make compose-up
make compose-status
make compose-down
```

## License

Switchboard is source-available under the [Elastic License 2.0](LICENSE).

In plain English: you can freely use, modify, redistribute, and self-host
Switchboard — including for commercial and internal-business use. The only
restriction is that you cannot offer Switchboard to third parties as a hosted
or managed service that exposes a substantial portion of its functionality.
That is the business reserved for the official hosted Switchboard service.

If you have questions about licensing or want to discuss other arrangements,
open an issue or get in touch.
