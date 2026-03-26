# Adapter Reference

## Conventions

### Unexported Structs, Exported Constructors
```go
type github struct { ... }           // unexported
func New() mcp.Integration { ... }   // returns interface
```

### Import Aliases

Only `slack`, `aws`, `notion`, and `gcp` require aliases to avoid collision with standard/SDK package names. Other packages are imported directly.

| Package | Alias | Used In |
|---------|-------|---------|
| `github.com/daltoniam/switchboard` | `mcp` | All consumers |
| `.../switchboard/integrations/slack` | `slackInt` | `cmd/server/main.go`, `web/web.go` |
| `.../switchboard/integrations/aws` | `awsInt` | `cmd/server/main.go` |
| `.../switchboard/integrations/notion` | `notionInt` | `cmd/server/main.go` |
| `.../switchboard/integrations/github` | `ghInt` | `web/web.go` |
| `.../switchboard/integrations/linear` | `linearInt` | `web/web.go` |
| `.../switchboard/integrations/sentry` | `sentryInt` | `web/web.go` |
| `.../switchboard/integrations/gcp` | `gcpInt` | `cmd/server/main.go` |

### Tool Naming
Tools are prefixed with integration name: `github_search_repos`, `datadog_search_logs`, `linear_list_issues`, `sentry_list_issues`.

### Argument Parsing
Use shared helpers from `args.go`. NEVER define local arg helpers in adapters.

**Bulk extraction** (2+ args at handler start — preferred):
```go
r := mcp.NewArgs(args)
owner := r.Str("owner")
repo := r.Str("repo")
if err := r.Err(); err != nil {
    return mcp.ErrResult(err)
}
```

**Conditional extraction** (inside if-blocks):
```go
if v, err := mcp.ArgStr(args, "project"); err != nil {
    return mcp.ErrResult(err)
} else if v != "" {
    // resolve project...
}
```

Available: `Str`, `Int`, `Int32`, `Int64`, `Float64`, `Bool`, `StrSlice`, `Map` — on both `Args` reader and as standalone `mcp.Arg*` functions. All return `(value, error)`.

**Pagination defaults** (reader only):
```go
page := r.OptInt("page", 1)
perPage := r.OptInt("per_page", 10)
```
`OptInt` returns the default when the value is missing, zero, or negative. Type coercion errors are silently ignored (returns default).

### Dispatch Map Test Parity

Every adapter **must** have two tests enforcing bidirectional parity between `Tools()` definitions and the `dispatch` map:

- `TestDispatchMap_AllToolsCovered` — every tool returned by `Tools()` has a handler in `dispatch`
- `TestDispatchMap_NoOrphanHandlers` — every key in `dispatch` has a corresponding `ToolDefinition`

When adding a new tool: add both the `ToolDefinition` in `tools.go` **and** the handler entry in the `dispatch` map. Tests will fail if either is missing.

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
- **Slack**: `slack-go/slack` typed SDK with **session token auth** (`xoxc-*` token + `xoxd-*` cookie)
  - `cookieTransport` (`http.RoundTripper`) injects `Cookie: d=<xoxd-cookie>`
  - Token priority: (1) config, (2) `~/.slack-mcp-tokens.json`, (3) Chrome disk-read (macOS)
  - Chrome extraction: LevelDB (`xoxc-*`) + encrypted SQLite cookies (`xoxd-*`, AES-128-CBC via Keychain)
  - Background refresh every 4h (`refresh.go`). Mutex-protected client (`s.getClient()`)
  - OAuth v2 flow (`oauth.go`) for web UI setup
- **AWS**: `aws-sdk-go-v2` official typed SDK. Auth via static credentials or default credential chain. Region defaults to `us-east-1`. Each service gets typed client via `<service>.NewFromConfig(cfg)`. Import aliased as `awsInt`
- **Notion**: Hand-rolled v3 internal API over `net/http`. Auth via `Cookie: token_v2=<token>` (session cookie starting with `v03:`). Base URL `https://www.notion.so`. All endpoints are POST to `/api/v3/<endpoint>`. No version header. HTTP client: 30s timeout, redirect blocking (prevents token leaking on 3xx), 512KB response cap (largest real responses ~230KB, keeps worst-case at ~125K tokens). 24 tools covering databases, data sources, pages, blocks, search, users, comments + 2 convenience tools (`getPageContent` single-call page tree, `createPageWithContent` atomic transaction). `spaceID` and `userID` resolved at `Configure()` time via `getSpaces`.
  - **Reads**: `loadCachedPageChunkV2` (blocks, pages, databases, data sources, comments, children, page content), `syncRecordValuesMain` with pointer format (users), `queryCollection` with source+reducer format (data source queries), `getSpaces` (user list), `search` (hybrid search). `getRecordValues` NOT used — broken by shard isolation.
  - **Writes**: `submitTransaction` with client-generated UUIDs. Atomic multi-op transactions.
  - **v3 gotchas**: `queryCollection` double-wraps blocks (`block[id].value.value.*`) and `recordMap` contains `__version__` (number) alongside table maps — parse as `map[string]any`; `collection_view.parent_table` must be `"block"` not `"collection"`; comments are bundled in `loadCachedPageChunkV2` recordMap (no dedicated endpoint); search results split between `results` (id, highlight) and `recordMap` (block data) — handler normalizes into flat array.
  - **Parent type branching**: Block parents use `listAfter block [parentID] ["content"]`; collection (database) parents use `setParent` with pointer format. Collections have no content list — `listAfter` on a collection ID returns 400. See `buildParentLinkOps` in `pages.go`. Full debugging guide: [notion-v3-transactions.md](notion-v3-transactions.md).
  - **ID types in search results**: `id` = block wrapper ID (for reads); `collection_id` = collection ID (for `database_id` in write tools). Passing block ID as `database_id` causes 400.
- **ClickHouse**: `ClickHouse/clickhouse-go/v2` typed native driver. Auth via `ch.Auth{Username, Password}`. Supports TLS (`secure`/`skip_verify` config). Connection pooling built into driver. Dynamic column scanning via `reflect` for generic query results.
- **pganalyze**: Hand-rolled GraphQL over `net/http`. Auth via `Authorization: Token <api_key>`. Base URL defaults to `https://app.pganalyze.com/graphql`; configurable via `base_url`. Organization slug required.
- **RWX**: Hand-rolled REST over `net/http`. Auth via `Authorization: Bearer <access_token>`. Base URL hardcoded to `https://cloud.rwx.com`. Includes proxy client that forwards tools from `rwx mcp serve` when available.
- **Gmail**: Hand-rolled REST over `net/http` against Google Gmail API. Auth via `Authorization: Bearer <access_token>` with OAuth2 refresh token support. Base URL defaults to `https://gmail.googleapis.com`. Requires OAuth2 client credentials for token refresh.
- **Home Assistant**: Hand-rolled REST over `net/http`. Auth via `Authorization: Bearer <token>`. Base URL required from config (varies per installation). ~17 tools covering states, services, history, events, config, areas, devices.
- **YNAB**: Hand-rolled REST over `net/http`. Auth via `Authorization: Bearer <api_key>` (personal access token). Base URL defaults to `https://api.ynab.com/v1`. ~25 tools covering user, budgets, accounts, categories, payees, months, transactions, scheduled transactions. Amounts in milliunits (1000 = $1.00). `budget_id` defaults to `"last-used"`. Rate limit: 200 requests/hour.

### Config
- File: `~/.config/switchboard/config.json`
- Auto-created with defaults if missing
- `Credentials` is `map[string]string`
- Thread-safe (`sync.RWMutex`)
- File permissions: dir `0700`, file `0600`

## Gotchas

- **Arg helpers are shared** in `args.go` — NEVER create local copies. Use `mcp.NewArgs(args)` or standalone `mcp.ArgStr`/`mcp.ArgInt`/etc.
- **All seventeen adapters use dispatch maps** (`var dispatch map[string]handlerFunc`). Tool counts: GitHub ~100, AWS ~65, Datadog ~60, Linear ~60, Sentry ~55, GCP ~55, PostHog ~50, Gmail ~44, Slack ~40, YNAB ~37, Postgres ~25, Notion ~24, Metabase ~22, ClickHouse ~20, Home Assistant ~17, RWX ~11, pganalyze ~3
- **Linear is the only GraphQL adapter**. `gql()` helper, entity resolution (`resolveTeamID`, `resolveIssueID`), field fragment constants (`issueFields`, `projectFields`)
- **AWS adapter uses `aws-sdk-go-v2`** — 11 typed service clients (S3, EC2, Lambda, IAM, CloudWatch, STS, ECS, SNS, SQS, DynamoDB, CloudFormation). Custom `unmarshalDynamoJSON` for DynamoDB AttributeValue marshalling. S3 `GetObject` capped at 10MB via `io.LimitReader`
- **PostHog adapter uses hand-rolled REST HTTP**. ~50 tools covering projects, feature flags, cohorts, insights, persons, groups, annotations, dashboards, actions, events, experiments, and surveys. Auth via `Authorization: Bearer <api_key>` (personal API key starting with `phx_`). Base URL defaults to `https://us.posthog.com`; configurable for EU or self-hosted. Most deletes are soft deletes (PATCH with `deleted: true`).
- **PostgreSQL adapter uses `database/sql` with `lib/pq`**. ~25 tools. Auth via `connection_string` or individual host/port/user/password/database/sslmode. Read-only queries wrapped in read-only transactions. `sanitizeIdentifier` prevents SQL injection. Handlers split across `databases.go`, `queries.go`, `management.go`
- **YNAB adapter uses hand-rolled REST HTTP**. ~25 tools covering user, budgets, accounts, categories, payees, months, transactions, and scheduled transactions. Auth via `Authorization: Bearer <api_key>` (personal access token). Base URL defaults to `https://api.ynab.com/v1`. All monetary amounts in milliunits (1000 = $1.00). `budget(args)` helper defaults `budget_id` to `"last-used"`. Rate limit: 200 requests/hour per token.
- **GCP adapter uses official `cloud.google.com/go` client libraries** — 17 typed clients (Storage, Compute Instances/Disks/Networks/Subnetworks/Firewalls, Functions, IAM via `google.golang.org/api/iam/v1`, Monitoring/AlertPolicy, Cloud Run Services/Revisions, Pub/Sub, Firestore, Logging/ConfigClient, ResourceManager Projects/Folders). Auth via Application Default Credentials or `credentials_json`. GCS `GetObject` capped at 10MB via `io.LimitReader`
- **`search` returns `ToolDefinition` metadata**, not raw API specs
