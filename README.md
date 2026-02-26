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
