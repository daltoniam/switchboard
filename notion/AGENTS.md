# Notion Integration

## API Version

- `Notion-Version: 2025-09-03` (data sources API)
- Auth: `Authorization: Bearer <integration_secret>` (internal integration secret starting with `ntn_`)

## Critical: Data Source ID != Database ID

A single Notion table has TWO different IDs:
- `database_id` — for `/v1/databases/*` endpoints
- `data_source_id` — from the `data_sources` array in database response, for `/v1/data_sources/*` endpoints

Confusing these returns 404.

## v2025-09-03 Gotchas

| What | Detail |
|------|--------|
| **Create** | `POST /v1/data_sources` returns 400. Use `POST /v1/databases` instead (`notion_create_database`) |
| **Description updates** | `PATCH /v1/data_sources/{id}` rejects `description` field. Only `title` and `properties` supported. Use `PATCH /v1/databases/{id}` for descriptions |
| **Search filter** | Filter value is `"data_source"` not `"database"`. Test enforces this: `TestSearchToolDescription_DocumentsDataSourceFilter` |

## url.PathEscape Inconsistency

- `get()` and `del()` use variadic args with `url.PathEscape`
- `patch()` and `post()` callers build paths with raw `fmt.Sprintf`
- Low risk (Notion IDs are UUIDs) but if adding handlers where user-controlled strings appear in `patch`/`post` paths, manually call `url.PathEscape`

## Recursive Block Fetching

`fetchBlocksRecursive` in `pages.go` has three safety bounds:
- `maxBlockFetches=100` shared counter via `*remaining` pointer
- `maxDepth` parameter (default 3)
- `ctx.Err()` check on every iteration

Returns a `truncated` bool — when the fetch limit is exhausted, `getPageContent` includes `"truncated":true` in the response JSON. Follow this pattern for any future recursive Notion API traversal.

## updateBlock type_content Merge

`type_content` parameter's map keys are merged directly into the request body (not nested). Pass `type_content: {"paragraph": {"rich_text": [...]}}` — handler flattens it.

## Testing

- `testNotion(t, handler)` creates httptest server + wired `*notion` + cleanup
- `okJSON(w, body)` writes JSON response in test handlers
- Test handlers directly (not through `Execute`) for focused coverage
- 64 tests with race detection

## No OAuth

Unlike GitHub/Linear/Sentry/Slack, Notion uses a simple internal integration secret. No OAuth dance, no token refresh, no callback routes. Web setup is a plain form at `POST /api/notion/save-token`.

## Pages Must Be Shared

Creating a Notion integration does NOT grant access to content. Two ways to grant access:
- **Bulk (recommended)**: Integration settings → **Access** tab → select pages
- **Per-page**: Open page → `···` → **Connections** → add integration

"Not found" errors usually mean the page wasn't shared, not that the token is bad.
