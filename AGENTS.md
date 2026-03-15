# AGENTS.md

## Overview

- Go MCP server aggregating 17 integrations (GitHub, Datadog, Linear, Sentry, Slack, Notion, AWS, GCP, PostHog, Postgres, ClickHouse, and more) behind one endpoint
- Two meta-tools only: **search** (discover operations) and **execute** (run them)
- Hexagonal architecture (ports and adapters) — see [docs/architecture.md](docs/architecture.md)

## Commands

| Target | Command | Make shortcut |
|--------|---------|---------------|
| Build | `go build -o switchboard ./cmd/server` | `make build` |
| Test | `go test ./...` | `make test` |
| Test + race | `go test -race -coverprofile=coverage.out ./...` | `make test-race` |
| Vet | `go vet ./...` | `make vet` |
| Lint | `go tool golangci-lint run` | `make lint` |
| Format | `gofmt -w .` | `make fmt` |
| **All CI checks** | build + vet + test-race + lint + security | **`make ci`** |

## Requirements Before Completing Code Changes

1. **Run `make ci`** — must pass
2. **New code must include tests**
3. **TDD**: write failing test before implementation, verify it fails for the right reason, then write minimal code to pass
4. **Table-driven tests**: use `t.Run` subtests when 3+ cases share the same assert structure
5. **MCP smoke test** — `TestSmoke_SearchResponseShape` in `server/server_test.go` validates the full response contract
6. **Go files must be `gofmt`'d** — run `make fmt` or `gofmt -w <file>` after editing `.go` files

## Git Workflow

- Branch from `main` for all changes
- CI runs on PRs: build, test (race), lint, security — all must pass
- Commit messages: imperative mood (e.g., "Add Linear OAuth flow", "Fix token refresh race")

## Commit Attribution

AI commits MUST include:
```
Co-Authored-By: <agent model name> <noreply@anthropic.com>
```

## Architecture

- Root package: `package mcp` — import as `mcp "github.com/daltoniam/switchboard"`
- Core: `mcp.go` (types + port interfaces), `compact.go` (field compaction engine)
- Server: `server/server.go`, composition root: `cmd/server/main.go`
- Every integration implements the `Integration` interface (see [docs/architecture.md](docs/architecture.md))

## Key Conventions

- **Unexported structs, exported constructors**: `type github struct{...}` / `func New() mcp.Integration`
- **Tool naming**: prefixed with integration name (`github_list_issues`, `datadog_search_logs`)
- **Dispatch map test parity** (MUST): `TestDispatchMap_AllToolsCovered` + `TestDispatchMap_NoOrphanHandlers` in every adapter
- **Compaction spec tests** (MUST): every adapter with `compact_specs.go` has parity + shape tests
- **Error handling**: `return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil`

## Reference Docs

| Doc | When to read |
|-----|-------------|
| [docs/architecture.md](docs/architecture.md) | Project structure, interfaces, adding integrations |
| [docs/field-compaction.md](docs/field-compaction.md) | Writing/editing compaction specs, tool descriptions |
| [docs/response-optimizations.md](docs/response-optimizations.md) | Modifying server response pipeline |
| [docs/adapter-reference.md](docs/adapter-reference.md) | Working on a specific integration adapter |
| [docs/web-ui.md](docs/web-ui.md) | Modifying the web config UI |

## Skills

| Skill | When to use |
|-------|-------------|
| `add-integration` | Adding a new external API integration adapter |
| `optimize-integration` | Improving an existing adapter's LLM usability |
| `mcp-benchmark` | Running live benchmark sequences against integrations |
| `pr-review` | Reviewing a pull request |
| `pr-comments` | Submitting inline PR review comments |
