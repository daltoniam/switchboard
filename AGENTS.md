# AGENTS.md

## Overview

Unified MCP Server is a Go application that aggregates multiple third-party integrations (GitHub, Datadog, Linear, Sentry, Slack, Metabase) behind a single [Model Context Protocol](https://modelcontextprotocol.io/) endpoint. AI clients connect via HTTP (streamable HTTP transport) and get access to all configured integrations through just two tools: **search** and **execute**. A web config UI is served on the same port.

The server follows a **hexagonal architecture** (ports and adapters) and uses the **search/execute pattern** inspired by [Cloudflare's Code Mode MCP](https://blog.cloudflare.com/code-mode-mcp/) — instead of exposing dozens of individual MCP tools, the server exposes only two meta-tools that allow progressive discovery and execution of any underlying integration operation.

## Commands

```bash
# Build
go build -o switchboard ./cmd/server

# Run (default — HTTP server with MCP + web UI on same port)
./switchboard
./switchboard --port 3847

# Run (stdio mode — legacy, for AI clients that need stdin/stdout)
./switchboard --stdio

# Run tests
go test ./...

# Run tests with race detection
go test -race ./...

# Vet
go vet ./...

# Lint (requires golangci-lint installed)
golangci-lint run

# Release (local snapshot for testing)
goreleaser release --snapshot --clean

# Release (production — triggered by pushing a git tag)
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
# CI (or manually): goreleaser release --clean
```

**Release tooling**: [GoReleaser](https://goreleaser.com/) is configured via `.goreleaser.yml`. It builds cross-platform binaries (darwin/linux/windows, amd64/arm64), produces `.deb`, `.rpm`, `.apk`, and `.archlinux` packages, and publishes to Homebrew (`daltoniam/homebrew-tap`), Scoop (`daltoniam/scoop-bucket`), and AUR (`switchboard-bin`). Version info is injected via ldflags (`main.version`, `main.commit`, `main.date`). The `--version` flag prints build metadata.

**Testing**: Uses [`github.com/stretchr/testify`](https://github.com/stretchr/testify) for assertions and test helpers. Tests exist for every package.

**Linting**: [golangci-lint](https://golangci-lint.run/) is configured via `.golangci.yml` with errcheck, govet, ineffassign, staticcheck, and unused linters enabled.

**CI/CD**: GitHub Actions workflow at `.github/workflows/ci.yml` runs on PRs and pushes to `main`. Jobs: build, test (with race detection), golangci-lint, gosec (security scanner), and govulncheck (vulnerability checker). All must pass before merging.

Go 1.25 with `github.com/modelcontextprotocol/go-sdk`, `github.com/google/go-github/v68`, `github.com/slack-go/slack`, `github.com/a-h/templ`, and `github.com/stretchr/testify` as direct dependencies.

## Requirements Before Completing Code Changes

All of the following **must pass** before any code change is considered complete:

1. **Build**: `go build -o switchboard ./cmd/server` must succeed with no errors.
2. **Tests**: `go test ./...` must pass with all tests green. Run `go test -race ./...` to check for race conditions.
3. **Lint**: `golangci-lint run` must pass with no errors.
4. **New code must include tests**: Any new feature, integration, or bug fix must include corresponding test coverage.

## Project Structure

```
mcp.go                       Domain types + port interfaces (the hexagonal core)
cmd/server/main.go           Composition root — wires adapters into Services, starts server
server/server.go             MCP server — exposes search/execute tools, routes to integrations
config/config.go             ConfigService adapter — JSON file at ~/.config/switchboard/config.json
registry/registry.go         Registry adapter — thread-safe integration lookup
github/
  github.go                  GitHub integration adapter (core, dispatch, helpers)
  tools.go                   GitHub tool definitions (~100 tools)
  repos.go                   Repos, releases, deploy keys, webhooks, rate limit handlers
  issues.go                  Issues, comments, labels, milestones handlers
  pulls.go                   Pull requests, reviews, merge handlers
  git.go                     Low-level git (commits, refs, trees, tags) handlers
  users_orgs.go              Users, followers, orgs, teams handlers
  actions.go                 Actions workflows, runs, jobs, secrets, checks handlers
  search.go                  Search (code, issues, users, commits) handlers
  extras.go                  Gists, activity, code/secret/dependabot scanning, copilot handlers
datadog/
  datadog.go                 Datadog integration adapter (core, dispatch, SDK client, helpers)
  tools.go                   Datadog tool definitions (~60 tools)
  logs.go                    Logs search and aggregation handlers
  metrics.go                 Metrics query, search, metadata handlers
  monitors.go                Monitors CRUD, search, mute handlers
  dashboards.go              Dashboards list, get, create, delete handlers
  events.go                  Events list, search, get, create handlers
  extras.go                  Hosts, tags, SLOs, downtimes, incidents, synthetics,
                             notebooks, users, spans, software catalog, IP ranges handlers
linear/
  linear.go                  Linear integration adapter (core, dispatch, GraphQL helpers)
  tools.go                   Linear tool definitions (~60 tools)
  issues.go                  Issues, comments, relations, labels, attachments handlers
  projects.go                Projects, project updates, milestones handlers
  teams.go                   Teams and users handlers
  extras.go                  Cycles, labels, workflow states, documents, initiatives,
                             favorites, webhooks, notifications, templates, org,
                             custom views, rate limit handlers
sentry/
  sentry.go                  Sentry integration adapter (core, dispatch, HTTP helpers)
  tools.go                   Sentry tool definitions (~55 tools)
  organizations.go           Organizations, members, teams, repos handlers
  issues.go                  Projects, issues, events, tags, stats handlers
  releases.go                Releases, deploys, commits, files handlers
  extras.go                  Alerts, monitors (cron), discover, replays handlers
slack/
  slack.go                   Slack integration adapter (core, dispatch, cookie transport, mutex-protected client)
  tokens.go                  Token store (persistence, Chrome disk-read extraction via LevelDB+SQLite+AES, background refresh)
  tools.go                   Slack tool definitions (~42 tools)
  conversations.go           Channels, DMs, history, threads handlers
  messages.go                Send, update, delete, search, reactions, pins handlers
  users.go                   Users, user groups, presence handlers
  extras.go                  Files, bookmarks, reminders, emoji, team info, auth handlers
  extract.go                 Exported helpers for web UI token extraction (Chrome, manual, snippet)
metabase/
  metabase.go                Metabase integration adapter (core, dispatch, HTTP helpers)
  tools.go                   Metabase tool definitions (~22 tools)
  databases.go               Database, table, field metadata handlers
  queries.go                 Native SQL query execution, card CRUD handlers
  dashboards.go              Dashboard CRUD, add-card-to-dashboard handlers
  collections.go             Collection CRUD, search handlers
web/
  web.go                     Web UI HTTP server for config dashboard + Slack token setup routes
  templates/                 Templ-based component templates (layouts, pages, components, slack_setup)
```

## Architecture

### Hexagonal Pattern

The root package (`mcp.go`) defines all domain types and port interfaces. Adapter packages satisfy those interfaces. Nothing in the core depends on infrastructure — dependencies point inward.

```
                    ┌─────────────────────────┐
                    │      mcp.go (core)       │
                    │                          │
                    │  Types:                  │
                    │    Config, Credentials   │
                    │    ToolDefinition        │
                    │    ToolResult            │
                    │                          │
                    │  Ports (interfaces):     │
                    │    Integration           │
                    │    ConfigService         │
                    │    Registry              │
                    │    Services (DI struct)   │
                    └────────┬────────────────┘
                             │ implements
          ┌──────────┬───────┼───────┬──────────┐
          │          │       │       │          │
      github/    datadog/ linear/ sentry/  slack/   config/
      (adapter)  (adapter) (adapter) (adapter) (adapter) (adapter)
```

### Search/Execute Pattern

Instead of registering every integration tool as a separate MCP tool (which bloats the AI's context window), the server exposes exactly **two tools**:

| MCP Tool | Purpose |
|----------|---------|
| `search` | Discover available tools across all enabled integrations. Filter by name, integration, or keyword. Returns tool definitions with parameters. |
| `execute` | Run a discovered tool by name with arguments. Routes to the correct integration adapter. |

**Flow:**
1. AI calls `search({"query": "github issues"})` → gets tool definitions with parameter schemas
2. AI calls `execute({"tool_name": "github_list_issues", "arguments": {"owner": "golang", "repo": "go"}})` → gets results

This keeps the MCP tool count fixed at 2 regardless of how many integrations or operations are added.

## Key Interface: `Integration`

Every integration adapter implements this interface defined in `mcp.go`:

```go
type Integration interface {
    Name() string
    Configure(creds Credentials) error
    Tools() []ToolDefinition
    Execute(ctx context.Context, toolName string, args map[string]any) (*ToolResult, error)
    Healthy(ctx context.Context) bool
}
```

- **`Name()`** — Lowercase identifier (e.g., `"github"`). Must match config key.
- **`Configure()`** — Receives `Credentials` (`map[string]string`). Validate and store.
- **`Tools()`** — Returns tool definitions for progressive discovery via the `search` MCP tool.
- **`Execute()`** — Dispatches to the correct handler by tool name. Returns `*ToolResult`.
- **`Healthy()`** — Lightweight API call to verify credentials.

## Other Port Interfaces

```go
type ConfigService interface {
    Load() error
    Save() error
    Get() *Config
    Update(cfg *Config) error
    GetIntegration(name string) (*IntegrationConfig, bool)
    SetIntegration(name string, ic *IntegrationConfig) error
    EnabledIntegrations() []string
}

type Registry interface {
    Register(i Integration) error
    Get(name string) (Integration, bool)
    All() []Integration
    Names() []string
}
```

## Services Struct (DI Container)

Following the chapterchamp pattern, a single struct aggregates all ports:

```go
type Services struct {
    Config   ConfigService
    Registry Registry
}
```

Constructed in `cmd/server/main.go` and passed to both `server.New()` and `web.New()`.

## Adding a New Integration

1. Create `<name>/<name>.go` in the repo root.
2. Define an unexported struct implementing `Integration`.
3. Export a `New()` constructor that returns `mcp.Integration`.
4. In `Tools()`, return `[]mcp.ToolDefinition` describing each operation.
5. In `Execute()`, switch on tool name and dispatch to private handler methods.
6. Register in `cmd/server/main.go` by adding to the integration list.
7. Add default credentials to `config.defaultConfig()` in `config/config.go`.

## Conventions and Patterns

### Unexported Structs, Exported Constructors
Following hexagonal convention, adapter types are unexported. Constructors return the domain interface:
```go
type github struct { ... }           // unexported
func New() mcp.Integration { ... }   // returns interface
```

### Tool Naming
Tools are prefixed with integration name: `github_search_repos`, `datadog_search_logs`, `linear_list_issues`, `sentry_list_issues`.

### Argument Parsing
Each adapter has a local `argStr` helper:
```go
func argStr(args map[string]any, key string) string {
    v, _ := args[key].(string)
    return v
}
```

### Error Handling
- Integration errors: return `&mcp.ToolResult{Data: err.Error(), IsError: true}, nil`
- Only return a non-nil Go error for truly exceptional failures
- The server layer wraps these into MCP error results

### HTTP Client / SDK Pattern
Each adapter uses either a typed SDK or raw HTTP. Auth varies:
- **GitHub**: `google/go-github/v68` typed SDK. Auth via `oauth2` token transport.
- **Datadog**: `DataDog/datadog-api-client-go/v2` typed SDK. Auth via `context.WithValue(ctx, datadog.ContextAPIKeys, ...)`. Site via `ContextServerVariables`. SDK has V1 and V2 API packages (`datadogV1`, `datadogV2`). Incidents API requires `cfg.SetUnstableOperationEnabled("v2.XxxIncident", true)`.
- **Linear**: Hand-rolled GraphQL over `net/http`. Auth via `Authorization: <api_key>` (no Bearer prefix).
- **Sentry**: Hand-rolled REST over `net/http` (no typed Go SDK for Sentry management API — `getsentry/sentry-go` is for error capture only). Auth via `Authorization: Bearer <auth_token>`. Base URL defaults to `https://sentry.io/api/0`. Organization slug configured once and injected into paths via `org(args)` helper.
- **Slack**: `slack-go/slack` typed SDK with **session token auth** (`xoxc-*` token + `xoxd-*` cookie). Auth uses a custom `cookieTransport` (`http.RoundTripper`) that injects `Cookie: d=<xoxd-cookie>` on every request, passed via `slack.OptionHTTPClient()`. Token loading priority: (1) config credentials, (2) persistent file `~/.slack-mcp-tokens.json`, (3) Chrome disk-read extraction (macOS only). Auto-extraction reads the `xoxc-*` token from Chrome's LevelDB localStorage (via `goleveldb`) and the `xoxd-*` cookie from Chrome's encrypted SQLite cookie DB (via `go-sqlite/sqlite3` for reading + AES-128-CBC decryption with Chrome Safe Storage password from macOS Keychain). No AppleScript or "Allow JavaScript from Apple Events" setting required. All Chrome profiles are scanned (`Default`, `Profile 1`, etc.). Background goroutine refreshes tokens every 4 hours. Client is mutex-protected (`s.getClient()`) since it gets rebuilt on refresh. Import aliased as `slackInt` in `cmd/server/main.go` to avoid collision with the package name.

### Config
- File: `~/.config/switchboard/config.json`
- Auto-created with defaults if missing
- `Credentials` is `map[string]string`
- Thread-safe (`sync.RWMutex`)
- File permissions: dir `0700`, file `0600`

### Web UI
- Embedded HTML string in `web/html.go`
- Default port: 3847
- Go 1.22+ method-pattern routing (`"GET /api/config"`, `"PUT /api/integrations/{name}"`)
- Slack setup page at `/integrations/slack/setup` — guided token extraction flow with:
  - **macOS auto-extract**: POST `/api/slack/extract-chrome` triggers AppleScript Chrome extraction server-side
  - **Manual browser extraction**: JavaScript snippet users run in their browser console, paste JSON result back
  - **Direct token entry**: Paste `xoxc-*` token and `xoxd-*` cookie manually
  - All methods save to `~/.slack-mcp-tokens.json` and update config simultaneously
  - Linked from the Slack integration detail page via "Setup Session Token" button

## Gotchas

- **`argStr` is duplicated** in every adapter package rather than shared. Follow this pattern for consistency. The GitHub adapter additionally has `argInt`, `argInt64`, `argBool`, `argStrSlice` helpers.
- **All six integrations use dispatch maps** (`var dispatch map[string]handlerFunc`) instead of switch statements for routing tool execution. GitHub ~100, Datadog ~60, Linear ~60, Sentry ~55, Slack ~40, Metabase ~22 tools.
- **Linear uses GraphQL** while all other adapters use REST. It has a `gql()` helper that handles query+variables, error parsing, and returns `json.RawMessage`. Auth is `Authorization: <api_key>` (no Bearer prefix). Handlers use `rawResult(data)` / `errResult(err)`. Entity resolution helpers (`resolveTeamID`, `resolveIssueID`) convert names/identifiers to UUIDs. GraphQL field fragments are defined as constants (`issueFields`, `projectFields`) and interpolated with `fmt.Sprintf`.
- **Datadog adapter uses `DataDog/datadog-api-client-go/v2`** — the official typed Go SDK. ~60 tools covering logs, metrics, monitors, dashboards, events, hosts, tags, SLOs, downtimes, incidents, synthetics, notebooks, users, spans, software catalog, and IP ranges. Uses both V1 and V2 SDK packages. Handlers are split across domain-specific files. Auth is injected per-request via `ctx()` helper using `datadog.ContextAPIKeys`.
- **Metabase adapter uses hand-rolled REST HTTP** (no typed Go SDK). ~22 tools covering databases, tables, fields, native SQL query execution, cards (saved questions) CRUD, dashboards CRUD, collections, and cross-content search. Auth via `x-api-key` header. Requires `url` (Metabase instance base URL) and `api_key` credentials.
- **Web UI uses templ templates** in `web/templates/`. Run `templ generate` after editing `.templ` files.
- **GitHub adapter uses `google/go-github/v68`** — the official typed Go client. ~100 tools covering repos, issues, PRs, actions, checks, releases, gists, search, orgs, teams, security scanning, copilot, and more. Handlers are split across domain-specific files.
- **Linear adapter uses hand-rolled GraphQL** (no SDK — no mature Go client exists). ~60 tools covering issues, projects, cycles, teams, users, labels, workflow states, documents, initiatives, favorites, webhooks, notifications, templates, organization, custom views, and rate limits. Handlers are split across domain-specific files.
- **Sentry adapter uses hand-rolled REST HTTP** (no typed Go SDK for the management API). ~55 tools covering organizations, projects, teams, issues, events, releases, alerts, cron monitors, discover queries, and replays. Uses `doRequest` helper for all HTTP methods. `queryEncode` builds optional query parameters. `org(args)` defaults to configured organization slug with per-request override.
- **Slack adapter uses `slack-go/slack`** — a mature typed Go client. ~42 tools covering conversations (channels/DMs), message CRUD, search, reactions, pins, scheduled messages, users, user groups, files, bookmarks, reminders, emoji, team info, and token management (`slack_token_status`, `slack_refresh_tokens`). Uses session token scraping (`xoxc-*`/`xoxd-*`) instead of bot tokens — ported from [jtalk22/slack-mcp-server](https://github.com/jtalk22/slack-mcp-server) (Node.js) and expanded. `tokens.go` handles Chrome disk-read extraction (LevelDB via `goleveldb` for token, encrypted SQLite via `go-sqlite/sqlite3` + AES-CBC decryption for cookie — no AppleScript needed), file persistence (`~/.slack-mcp-tokens.json`), and background refresh (4h ticker). Extraction scans all Chrome profiles. `extract.go` exports helpers (`ExtractFromChromeForWeb`, `SaveTokensForWeb`, `GetTokenInfoForWeb`, `ExtractionSnippet`) for the web UI setup page. All handler functions use `s.getClient()` (read-locked) since the client is rebuilt on token refresh under a write lock. Handlers are split across domain-specific files.
- **The search tool returns ToolDefinition metadata**, not raw API specs. If an integration adds many tools, consider adding categories or tags to `ToolDefinition`.
- **Go 1.25** is specified in `go.mod`. Uses Go 1.22+ features (method-pattern routing).
