# Switchboard

A unified MCP server written in Go that aggregates multiple integrations
(GitHub, Datadog, Linear, Sentry, Slack, Metabase) behind a single MCP endpoint,
with a web UI for easy configuration.

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
                                     │  │  ├─ Sentry       │  │
                                     │  │  ├─ Slack        │  │
                                     │  │  └─ Metabase     │  │
                                     │  └─────────────────┘  │
                                     └──────────────────────┘
```

## Context Optimization

API responses are large. A single GitHub issue carries ~100 fields (nested users, permissions, node IDs, avatar URLs) when an LLM needs ~10 to decide what to do next. Multiply by 30 issues per page and a list call can consume 150KB of context for information the model will never use.

Switchboard solves this automatically. Integrations declare **compaction specs** that describe which fields matter for each tool. The server strips everything else after every `execute` call, before responses reach the LLM.

List and search responses are compact by default. When the LLM identifies a specific item and calls a single-item `get` tool, it gets the full response back for drill-down.

## Quick Start

```bash
# Run (default — HTTP server with MCP + web UI on port 3847)
switchboard

# Custom port
switchboard --port 8080

# Stdio mode (for Cursor/Claude Desktop)
switchboard --stdio

# Check version
switchboard --version

# Open config UI
open http://localhost:3847
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

### Environment Variables

Switchboard automatically reads environment variables from your shell (fish, zsh, bash, etc.) and overlays them on top of the JSON config. If an env var is set, it takes precedence over the corresponding value in `config.json`. Env-sourced values are never written back to disk.

Any integration with credentials provided via env vars will auto-enable without needing to toggle it in the web UI.

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
| Metabase | `api_key` | `METABASE_API_KEY` |
| Metabase | `url` | `METABASE_URL` |
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
| DigitalOcean | `api_token` | `DIGITALOCEAN_TOKEN` |

### OAuth Setup

Some integrations support OAuth flows through the web UI at `http://localhost:3847`. This is the easiest way to get tokens for integrations that don't use simple API keys.

| Integration | Auth Method | Setup |
|---|---|---|
| GitHub | OAuth Device Flow | Web UI → GitHub → Setup, or set `GITHUB_TOKEN` |
| Linear | OAuth (PKCE) | Web UI → Linear → Setup, or set `LINEAR_API_KEY` |
| Sentry | OAuth Device Flow | Web UI → Sentry → Setup, or set `SENTRY_AUTH_TOKEN` |
| Slack | Session Token | Web UI → Slack → Setup (auto-extracts from Chrome), or set `SLACK_TOKEN` |
| Datadog | API + App Key | Set `DD_API_KEY` and `DD_APP_KEY` env vars or enter in web UI |
| AWS | IAM Credentials | Set `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` env vars, or uses default credential chain |
| Metabase | API Key | Set `METABASE_API_KEY` and `METABASE_URL` env vars or enter in web UI |
| PostHog | Personal API Key | Set `POSTHOG_API_KEY` env var or enter in web UI |
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
