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

Every adapter with a static `dispatch` map **must** have two tests enforcing bidirectional parity between `Tools()` definitions and the `dispatch` map:

- `TestDispatchMap_AllToolsCovered` — every tool returned by `Tools()` has a handler in `dispatch`
- `TestDispatchMap_NoOrphanHandlers` — every key in `dispatch` has a corresponding `ToolDefinition`

When adding a new tool: add both the `ToolDefinition` in `tools.go` **and** the handler entry in the `dispatch` map. Tests will fail if either is missing.

Dynamic adapters without a static dispatch map (e.g. `slackmcp`) must instead prove routing parity with tests that every tool returned by `Tools()` is executable through the dynamic path.

### Error Handling
- Integration errors: return `&mcp.ToolResult{Data: err.Error(), IsError: true}, nil`
- Only return a non-nil Go error for truly exceptional failures
- The server layer wraps these into MCP error results

### HTTP Client / SDK Pattern
Each adapter uses either a typed SDK or raw HTTP. Auth varies:
- **GitHub**: `google/go-github/v68` typed SDK. Auth via `oauth2` token transport.
- **Forgejo**: `codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3` typed SDK (`v3.0.0`), authenticated with a personal access token. Required credentials: `base_url` (instance URL including any deployment subpath, without `/api/v1`) and `token`; environment overrides: `FORGEJO_BASE_URL`, `FORGEJO_TOKEN`. Disabled by default. The generic web UI uses the adapter's `PlainTextKeys` and `Placeholders` metadata for credential fields; no custom setup page.
  - **Scope**: 30 tools: 23 reads and 7 mutations. Covers repository search/list/get; authenticated user and organizations; issues and comments; pull requests, diffs, files, reviews, and merge; branches, commits, content, releases, and labels. Start with `forgejo_list_user_repos` or `forgejo_search_repos`; issue/PR follow-ups take repository-local `number`, not global `id`.
  - **PAT permissions**: grant only the read access needed for repositories, the authenticated user, organizations, and issues used by your workflow. Check the instance's API/token settings for exact scope names and endpoint requirements; add write access only for intended mutations, not for read-only discovery or smoke tests.
  - **Pagination**: paginated tools default to `page=1`, `per_page=30`, maximum `per_page=50`. `forgejo_list_commits` with a nonempty `path` rejects `per_page`: Forgejo supplies server-sized chunks, advanced with `page`, without local slicing. Directory and issue/PR conversation-comment lists are unpaginated and reject `page`/`per_page`. Narrow comments by update time using `since`/`before`: nonzero whole-second RFC3339 timestamps with a timezone, no fractional seconds; `since` must precede `before`.
  - **Retries and context**: safe reads expose retryable HTTP 429/5xx failures to the server retry policy. Writes never retry automatically, including in scripts: an error can arrive after a mutation persisted (for example, failed mention processing after comment creation). Verify remote state before retrying an ambiguous outcome. Each call constructs a fresh SDK client with its request context while sharing the HTTP client, so cancellation/deadlines do not leak across calls.
  - **Response shaping**: `forgejo_get_repo` preserves `permissions.admin`, `permissions.push`, and `permissions.pull` during compaction; repository lists omit permissions. Conversation-comment bodies are fenced as literal Markdown (with collision-safe fences); `get_issue` renders ordinary Markdown, while `get_pull` retains structured JSON with its Markdown body. File contents retain encoding/base64 content; pull diffs are `{diff: string}`, not base64; both have a 1 MiB response budget.
  - **Search verification**: discovery reads local tool metadata, not the Forgejo API. Disabled tools are hidden unless `discoverAll` is enabled, when they are marked unconfigured. Short queries such as `Forgejo repo`, `Forgejo issue`, and `Forgejo file content` cover common entry points. `Forgejo pull review` is intentionally broad: finding PRs, inspecting diffs, and submitting reviews are useful results. For existing reviews use `{"integration":"forgejo","query":"list pull reviews"}` rather than tuning ranking to force the review list above other workflow tools. `server/forgejo_test.go` checks the real catalog, top-3/top-5 discovery, stable ordering, and disabled visibility without upstream HTTP.
- **Datadog**: `DataDog/datadog-api-client-go/v2` typed SDK. Auth via `context.WithValue(ctx, datadog.ContextAPIKeys, ...)`. Site via `ContextServerVariables`. SDK has V1 and V2 API packages (`datadogV1`, `datadogV2`). Incidents API requires `cfg.SetUnstableOperationEnabled("v2.XxxIncident", true)`.
- **Linear**: Hand-rolled GraphQL over `net/http`. Auth via `Authorization: <api_key>` (no Bearer prefix).
- **Sentry**: Hand-rolled REST over `net/http` (no typed Go SDK for Sentry management API — `getsentry/sentry-go` is for error capture only). Auth via `Authorization: Bearer <auth_token>`. Base URL defaults to `https://sentry.io/api/0`. Organization slug configured once and injected into paths via `org(args)` helper.
- **Slack**: `slack-go/slack` typed SDK with **session token auth** (`xoxc-*` token + `xoxd-*` cookie)
  - `cookieTransport` (`http.RoundTripper`) injects `Cookie: d=<xoxd-cookie>`
  - Token priority: (1) config, (2) `~/.slack-mcp-tokens.json`, (3) Chrome disk-read (macOS)
  - Chrome extraction: LevelDB (`xoxc-*`) + encrypted SQLite cookies (`xoxd-*`, AES-128-CBC via Keychain)
  - Background refresh every 4h (`refresh.go`). Mutex-protected client (`s.getClient()`)
  - OAuth v2 flow (`oauth.go`) for web UI setup
- **Slack MCP (`slackmcp`)**: separate multi-identity proxy to Slack's **official hosted** Streamable HTTP MCP (`https://mcp.slack.com`, client appends `/mcp`). One `remotemcp` client per named identity; each identity requires a user OAuth `access_token` (not bot `xoxb-`). Dynamically unions scope-dependent upstream tools as `slackmcp_*`, injects required `identity_id` on every proxied tool, and exposes discovery tool `slackmcp_list_available_identites`. No static dispatch map — routing parity covered by dynamic execute tests.
- **Figma (`figma`)**: focused proxy to Figma's official hosted Streamable HTTP MCP (`https://mcp.figma.com/mcp`). OAuth uses dynamic client registration, PKCE, the `mcp:connect` scope, and RFC 8707 resource binding. Exposes only the FigJam planning workflow (`get_figjam`, `use_figma`, `generate_diagram`, `create_new_file`, `upload_assets`, `get_screenshot`, `whoami`) and injects `figma-use-figjam` into `use_figma` calls when omitted.
- **AWS**: `aws-sdk-go-v2` official typed SDK. Auth via static credentials or default credential chain. Region defaults to `us-east-1`. Each service gets typed client via `<service>.NewFromConfig(cfg)`. Import aliased as `awsInt`
- **Notion**: Hand-rolled v3 internal API over `net/http`. Auth via `Cookie: token_v2=<token>` (session cookie starting with `v03:`). Base URL `https://www.notion.so`. All endpoints are POST to `/api/v3/<endpoint>`. No version header. HTTP client: 30s timeout, redirect blocking (prevents token leaking on 3xx), 512KB response cap (largest real responses ~230KB, keeps worst-case at ~125K tokens). 24 tools covering databases, data sources, pages, blocks, search, users, comments + 2 convenience tools (`getPageContent` single-call page tree, `createPageWithContent` atomic transaction). `spaceID` and `userID` resolved at `Configure()` time via `getSpaces`.
  - **Reads**: `loadCachedPageChunkV2` (blocks, pages, databases, data sources, comments, children, page content), `syncRecordValuesMain` with pointer format (users), `queryCollection` with source+reducer format (data source queries), `getSpaces` (user list), `search` (hybrid search). `getRecordValues` NOT used — broken by shard isolation.
  - **Writes**: `submitTransaction` with client-generated UUIDs. Atomic multi-op transactions.
  - **v3 gotchas**: `queryCollection` double-wraps blocks (`block[id].value.value.*`) and `recordMap` contains `__version__` (number) alongside table maps — parse as `map[string]any`; `collection_view.parent_table` must be `"block"` not `"collection"`; comments are bundled in `loadCachedPageChunkV2` recordMap (no dedicated endpoint); search results split between `results` (id, highlight) and `recordMap` (block data) — handler normalizes into flat array.
  - **Parent type branching**: Block parents use `listAfter block [parentID] ["content"]`; collection (database) parents use `setParent` with pointer format. Collections have no content list — `listAfter` on a collection ID returns 400. See `buildParentLinkOps` in `pages.go`. Full debugging guide: [notion-v3-transactions.md](notion-v3-transactions.md).
  - **ID types in search results**: `id` = block wrapper ID (for reads); `collection_id` = collection ID (for `database_id` in write tools). Passing block ID as `database_id` causes 400.
- **ClickHouse**: `ClickHouse/clickhouse-go/v2` typed native driver. Auth via `ch.Auth{Username, Password}`. Supports TLS (`secure`/`skip_verify` config). Connection pooling built into driver. Supports multiple configured clusters through `connections` JSON; tools use the `connection` alias parameter while `database` remains the ClickHouse database/schema context. Dynamic column scanning via `reflect` for generic query results.
- **pganalyze**: Hand-rolled GraphQL over `net/http`. Auth via `Authorization: Token <api_key>`. Base URL defaults to `https://app.pganalyze.com/graphql`; configurable via `base_url`. Organization slug required.
- **RWX**: Hand-rolled REST over `net/http`. Auth via `Authorization: Bearer <access_token>`. Base URL hardcoded to `https://cloud.rwx.com`. Read paths for runs, results, logs, and artifacts use `/mint/api/*` directly. CLI-backed tools are explicitly labeled in descriptions (`launch`, `dispatch`, workflow validation, docs, vaults, and `verify_cli`). Includes proxy client that forwards tools from `rwx mcp serve` when available.
- **Gmail**: Hand-rolled REST over `net/http` against Google Gmail API. Auth via `Authorization: Bearer <access_token>` with OAuth2 refresh token support. Base URL defaults to `https://gmail.googleapis.com`. Requires OAuth2 client credentials for token refresh.
- **Home Assistant**: Hand-rolled REST over `net/http`. Auth via `Authorization: Bearer <token>`. Base URL required from config (varies per installation). ~17 tools covering states, services, history, events, config, areas, devices.
- **YNAB**: Hand-rolled REST over `net/http`. Auth via `Authorization: Bearer <api_key>` (personal access token). Base URL defaults to `https://api.ynab.com/v1`. ~25 tools covering user, budgets, accounts, categories, payees, months, transactions, scheduled transactions. Amounts in milliunits (1000 = $1.00). `budget_id` defaults to `"last-used"`. Rate limit: 200 requests/hour.
- **Gong**: Hand-rolled REST over `net/http` against `https://api.gong.io/v2`. Auth via HTTP Basic (`access_key:access_key_secret` base64). ~15 tools covering calls, extensive call details, transcripts, users, workspaces, library folders, activity/interaction/scorecard stats, logs, and data privacy. Cursor pagination via `cursor`.
- **Zendesk**: Hand-rolled REST over `net/http` against Support API v2 (`https://{subdomain}.zendesk.com/api/v2`). Auth via OAuth bearer token or API token Basic (`{email}/token:{api_token}`). ~26 tools covering tickets, comments, audits, users, organizations, groups, views, macros, Help Center articles, tags, and CSAT ratings. Cursor pagination via `page[size]` / `page[after]`. Markdown rendering for ticket, comment, and article bodies.
- **HubSpot**: Hand-rolled REST over `net/http` against CRM v3 (`https://api.hubapi.com`). Auth via `Authorization: Bearer <access_token>` (private app token). ~31 tools covering contacts, companies, deals, tickets, generic objects, associations, owners, pipelines, and properties. Cursor pagination via `after`.
- **Front**: Hand-rolled REST over `net/http` against Core API (`https://api2.frontapp.com`). Auth via `Authorization: Bearer <access_token>` (API token or OAuth). ~20 tools covering conversation search/list/get, messages, comments, assignment/status, inboxes, teammates, tags, channels, contacts, accounts, drafts, and sending. Cursor pagination via `page_token` / `_pagination.next`. Markdown rendering for conversations and message threads.
- **Okta**: Hand-rolled REST over `net/http` against `{org_url}/api/v1`. Auth via `Authorization: SSWS <api_token>`. ~34 tools covering users (list/get/create/update + activate/deactivate/suspend/unlock/reset password), groups and membership, apps and assignments, MFA factors, policies/rules, system log, and org metadata. Cursor pagination via `after` from the `Link` header (`next_after` in list responses).
- **Ramp**: Hand-rolled REST over `net/http` against the Developer API (`/developer/v1/*`). Auth via `Authorization: Bearer <access_token>` (OAuth client-credentials or dashboard token). Base URL defaults to `https://api.ramp.com`. ~26 tools covering transactions, reimbursements, bills, users, virtual/physical cards, receipts, departments, locations, merchants, vendors, entities, and bank accounts. Cursor pagination via `start` + `page_size`.
- **NetSuite**: Hand-rolled REST over `net/http` against SuiteTalk REST Web Services. Auth via OAuth 2.0 bearer token or Token-Based Auth (OAuth 1.0 HMAC-SHA256). Base URL defaults to `https://{accountId}.suitetalk.api.netsuite.com`. ~25 tools covering SuiteQL, generic record CRUD, customers, vendors, invoices, vendor bills, POs, sales orders, employees, subsidiaries, departments, journal entries, and metadata catalog.
- **Microsoft 365**: Hand-rolled REST over `net/http` against Microsoft Graph `v1.0`. Auth via `Authorization: Bearer <access_token>` with OAuth2 refresh (Azure AD / Entra ID, PKCE). Base URL defaults to `https://graph.microsoft.com/v1.0`. ~35 tools covering Outlook mail, calendar, OneDrive/SharePoint files, Teams chats/channels, To Do tasks, and directory users. Graph list pages unwrap `@odata.nextLink` to `next_link`.
- **ServiceNow**: Hand-rolled REST over `net/http` against the Table API (`/api/now/table/*`) and Aggregate API (`/api/now/stats/*`). Auth via HTTP Basic (`username:password`) or `Authorization: Bearer <access_token>`. Instance URL required (`https://{instance}.service-now.com`). ~25 tools covering incidents, problems, change requests, catalog requests, knowledge articles, users, groups, CMDB CIs, generic table CRUD, aggregates, journal comments, and attachments. Offset pagination via `sysparm_limit` + `sysparm_offset`. Encoded queries via `sysparm_query`.
- **DigitalOcean**: `digitalocean/godo` typed SDK. Auth via `oauth2.StaticTokenSource` with personal access token. Base URL defaults to `https://api.digitalocean.com`. ~45 tools covering droplets, Kubernetes, managed databases, domains, DNS, load balancers, firewalls, VPCs, volumes, App Platform, regions, sizes, images, SSH keys, snapshots, projects, billing, CDN, certificates, container registry, tags. Pagination via `godo.ListOptions` with default `PerPage: 200`.

### Config
- File: `~/.config/switchboard/config.json`
- Auto-created with defaults if missing
- `Credentials` is `map[string]string`
- Thread-safe (`sync.RWMutex`)
- File permissions: dir `0700`, file `0600`

## Gotchas

- **Arg helpers are shared** in `args.go` — NEVER create local copies. Use `mcp.NewArgs(args)` or standalone `mcp.ArgStr`/`mcp.ArgInt`/etc.
- **All adapters use dispatch maps** (`var dispatch map[string]handlerFunc`). Tool counts: GitHub ~100, AWS ~65, Datadog ~60, Linear ~60, Sentry ~55, GCP ~55, PostHog ~50, DigitalOcean ~45, Gmail ~44, Slack ~40, YNAB ~37, Postgres ~25, Notion ~24, Metabase ~22, ClickHouse ~20, Home Assistant ~17, RWX ~11, pganalyze ~3
- **Linear is the only GraphQL adapter**. `gql()` helper, entity resolution (`resolveTeamID`, `resolveIssueID`), field fragment constants (`issueFields`, `projectFields`)
- **AWS adapter uses `aws-sdk-go-v2`** — 11 typed service clients (S3, EC2, Lambda, IAM, CloudWatch, STS, ECS, SNS, SQS, DynamoDB, CloudFormation). Custom `unmarshalDynamoJSON` for DynamoDB AttributeValue marshalling. S3 `GetObject` capped at 10MB via `io.LimitReader`
- **PostHog adapter uses hand-rolled REST HTTP**. ~50 tools covering projects, feature flags, cohorts, insights, persons, groups, annotations, dashboards, actions, events, experiments, and surveys. Auth via `Authorization: Bearer <api_key>` (personal API key starting with `phx_`). Base URL defaults to `https://us.posthog.com`; configurable for EU or self-hosted. Most deletes are soft deletes (PATCH with `deleted: true`).
- **PostgreSQL adapter uses `database/sql` with `lib/pq`**. ~25 tools. Auth via `connection_string` or individual host/port/user/password/database/sslmode. Read-only queries wrapped in read-only transactions. `sanitizeIdentifier` prevents SQL injection. Handlers split across `databases.go`, `queries.go`, `management.go`
- **YNAB adapter uses hand-rolled REST HTTP**. ~25 tools covering user, budgets, accounts, categories, payees, months, transactions, and scheduled transactions. Auth via `Authorization: Bearer <api_key>` (personal access token). Base URL defaults to `https://api.ynab.com/v1`. All monetary amounts in milliunits (1000 = $1.00). `budget(args)` helper defaults `budget_id` to `"last-used"`. Rate limit: 200 requests/hour per token.
- **GCP adapter uses official `cloud.google.com/go` client libraries** — 17 typed clients (Storage, Compute Instances/Disks/Networks/Subnetworks/Firewalls, Functions, IAM via `google.golang.org/api/iam/v1`, Monitoring/AlertPolicy, Cloud Run Services/Revisions, Pub/Sub, Firestore, Logging/ConfigClient, ResourceManager Projects/Folders). Auth via Application Default Credentials or `credentials_json`. GCS `GetObject` capped at 10MB via `io.LimitReader`
- **`search` returns `ToolDefinition` metadata**, not raw API specs
