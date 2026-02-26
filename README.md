# Unified MCP Server

A local MCP server written in Go that aggregates multiple integrations
(Datadog, Linear, Sentry, GitHub, etc.) behind a single MCP endpoint,
with a web UI for easy configuration.

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
                                     │  │  ├─ Datadog      │  │
                                     │  │  ├─ Linear       │  │
                                     │  │  ├─ Sentry       │  │
                                     │  │  └─ GitHub       │  │
                                     │  └─────────────────┘  │
                                     └──────────────────────┘
```

## Quick Start

```bash
# Build
go build -o switchboard ./cmd/server

# Run (stdio mode for Cursor/Claude)
./switchboard

# Run with web UI
./switchboard --web --port 3847

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

## Adding to Cursor / Claude Desktop

Add to your MCP client config:

```json
{
  "mcpServers": {
    "switchboard": {
      "command": "/path/to/switchboard",
      "args": []
    }
  }
}
```

## Project Structure

```
cmd/server/         — Entry point
internal/
  config/           — Config loading, saving, defaults
  adapter/          — Integration adapter interface + registry
  adapters/
    github/         — GitHub integration
    datadog/        — Datadog integration
    linear/         — Linear integration
    sentry/         — Sentry integration
  server/           — MCP server wiring
  web/              — Web UI (config dashboard)
    static/         — HTML/CSS/JS assets
```
