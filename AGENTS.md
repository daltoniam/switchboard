# AGENTS.md

## Overview

- Go MCP server aggregating integrations (GitHub, Datadog, Linear, Sentry, Slack, Notion, AWS, GCP, PostHog, Postgres, ClickHouse, and more) behind one endpoint
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
- Core: `mcp.go` (types + port interfaces), `compact.go` (field compaction engine), `args.go` (shared arg extraction)
- Markdown: `markdown/` package — `Builder`, `FromHTML`, `ApplyMarks`, `WriteTable` (see [docs/markdown-rendering.md](docs/markdown-rendering.md))
- Server: `server/server.go`, composition root: `cmd/server/main.go`
- Every integration implements the `Integration` interface (see [docs/architecture.md](docs/architecture.md))

## Key Conventions

- **Unexported structs, exported constructors**: `type github struct{...}` / `func New() mcp.Integration`
- **Tool naming**: prefixed with integration name (`github_list_issues`, `datadog_search_logs`)
- **Dispatch map test parity** (MUST): `TestDispatchMap_AllToolsCovered` + `TestDispatchMap_NoOrphanHandlers` in every adapter
- **Compaction spec tests** (MUST): every adapter with `compact.yaml` has parity + shape tests. Cross-adapter strict-mode validation lives in `compact/all_adapters_test.go`.
- **Shared result helpers** (MUST): use `mcp.JSONResult(v)`, `mcp.RawResult(data)`, `mcp.ErrResult(err)` — never define per-adapter copies
- **Shared arg helpers** (MUST): use `mcp.NewArgs(args)` reader or standalone `mcp.ArgStr`/`mcp.ArgInt`/etc. from `args.go` — never define local `argStr`/`argInt` in adapters. All return `(value, error)`. Use `r.OptInt("page", 1)` for pagination defaults. See `args.go` for the full API and [docs/go-anti-patterns.md](docs/go-anti-patterns.md) for extraction pitfalls that cause silent errors.
- **Args parity test** (MUST): `TestNewArgs_ErrCheckParity` in `args_test.go` walks all adapters verifying every `NewArgs` call has a matching `.Err()` check — new adapters are covered automatically
- **Parse at boundary, not throughout** (MUST): JSON is unmarshalled once at ingress and marshalled once at egress. Use `CompactAny`/`ColumnarizeAny` for already-parsed data — never re-serialize to `[]byte` just to call `CompactJSON`/`ColumnarizeJSON`. Redundant marshal/unmarshal cycles are the #1 performance regression to guard against in the response pipeline.
- **Entry-point guidance** (MUST): Every integration's primary tool description must include "Start here" text for wayfinding. See `add-integration` skill for pattern.
- **Semantic types** (MUST): `mcp.ToolName` for all tool name parameters, `mcp.Markdown` for markdown return values — never cast back to `string` in the dispatch chain. `string()` only at I/O boundaries.
- **Markdown rendering**: When a tool returns document content (pages, emails, comments), implement `mcp.MarkdownIntegration` with types from the `markdown` package — see [docs/markdown-rendering.md](docs/markdown-rendering.md) for decision framework and implementation checklist. Parity test `TestRenderMarkdown_ToolsCovered` is MUST.
- **Script scope**: `api.call()` in scripts can only call integration tools — the `search` and `execute` meta-tools are not callable from scripts. Scripts skip `processResult` entirely (no compaction, no markdown rendering) so tools always return raw JSON for programmatic access.

## Reference Docs

| Doc | When to read |
|-----|-------------|
| [docs/architecture.md](docs/architecture.md) | Project structure, interfaces, adding integrations |
| [docs/field-compaction.md](docs/field-compaction.md) | Writing/editing compaction specs, tool descriptions |
| [docs/response-optimizations.md](docs/response-optimizations.md) | Modifying server response pipeline |
| [docs/adapter-reference.md](docs/adapter-reference.md) | Working on a specific integration adapter |
| [docs/go-anti-patterns.md](docs/go-anti-patterns.md) | Writing/reviewing handler arg extraction (prevents silent error swallowing) |
| [docs/tool-search.md](docs/tool-search.md) | Search scoring engine, synonym groups, tool description guidelines, benchmarking |
| [docs/notion-v3-transactions.md](docs/notion-v3-transactions.md) | Debugging Notion v3 400 errors, transaction patterns, ID disambiguation |
| [docs/markdown-rendering.md](docs/markdown-rendering.md) | Adding markdown rendering to an integration (decision framework, shared utilities, implementation checklist) |
| [docs/web-ui.md](docs/web-ui.md) | Modifying the web config UI |

## Skills

| Skill | When to use |
|-------|-------------|
| `add-integration` | Adding a new external API integration adapter |
| `optimize-integration` | Improving an existing adapter's LLM usability |
| `mcp-benchmark` | Running live benchmark sequences against integrations |
| `search-benchmark` | Cross-model search quality benchmark after scoring changes |
| `pr-review` | Reviewing a pull request |
| `pr-comments` | Submitting inline PR review comments |
