# Notion v3 Transaction Debugging Guide

How to diagnose and fix 404/400/500 errors from Notion v3 writes. All cookie-backed write handlers call the internal `submitTransaction` helper, which posts to `/api/v3/saveTransactions`. OAuth-backed v1 writes are separate.

## Write Endpoint and Envelope

A `404 Cannot POST /api/v3/submitTransaction` indicates an unavailable route, not an invalid operation or database ID. Changing only the URL is insufficient: `saveTransactions` expects a batch envelope rather than top-level `operations`.

```json
{
  "requestId": "<request-uuid>",
  "transactions": [
    {
      "id": "<transaction-uuid>",
      "spaceId": "<configured-workspace-id>",
      "operations": [
        {"command": "set", "pointer": {"table": "block", "id": "<block-id>", "spaceId": "<configured-workspace-id>"}, "path": ["type"], "args": "page"}
      ]
    }
  ]
}
```

Switchboard generates distinct request and transaction UUIDs for each helper call, sets the transaction's `spaceId` from the configured workspace, and keeps all operations in one transaction. At submission, legacy flat operations are copied into pointer format with the configured workspace; existing pointer operations are preserved. Requests include `x-notion-active-user-header` and `x-notion-space-id` when configured. It does not retry against alternate write endpoints: an ambiguous write result must not trigger a second mutation.

Public implementation evidence checked on 2026-11-24, without issuing live writes:

- [Mikubill/transfer](https://github.com/Mikubill/transfer/blob/9227a05701322bfd00f64d557554818e25248c25/apis/public/notion/client.go): `SubmitTransaction` posts the UUID/space/batch envelope to `https://www.notion.so/api/v3/saveTransactions`.
- [FSB Notion handler](https://github.com/fullselfbrowsing/FSB/blob/df541df6638ff660f8633a383a7f4191e76d2fe6/catalog/handlers/notion.js): `saveTransactions` uses the same envelope. Its [2026-06-29 UAT record](https://github.com/fullselfbrowsing/FSB/blob/df541df6638ff660f8633a383a7f4191e76d2fe6/.planning/milestones/v1.0.0-phases/41-depth-2-remaining-hand-ports-guarded-writes/41-HUMAN-UAT.md) reports verified page/database/row writes against that route on `app.notion.com`.
- [webclipper/web-clipper](https://github.com/webclipper/web-clipper/blob/33a3f3a09b02d114d61fdc84e465f310ecf39e7c/src/common/backend/services/notion/service.ts): uses `/api/v3/saveTransactionsFanout` with the same envelope and flat operations. This supports the envelope shape but is not a reason to add a blind fanout fallback.

- [cmux append implementation](https://github.com/manaflow-ai/cmux/blob/9fb414f372c07a1a199632ea4091d4cb3d8caa84/Resources/browser-repl/sites/notion.js): uses pointer-format `set` and `listAfter` operations for `saveTransactions` and an active-user header. The page's `space_id` provides the pointers' workspace.
- [OpenTabs Notion transport](https://github.com/opentabs-dev/opentabs/blob/de9200b1231cae419d1a437e410114a9e2fe8eca/plugins/notion/src/notion-api.ts): sends active-user and workspace headers when available.

The public sources are not an official API stability guarantee. During hot-reloaded verification on 2026-11-24, the parent agent reported a successful append after supplying a hyphenated page UUID with the new endpoint, pointer operations, and routing headers. Read-back verification remains a separate deployment check.

### Compact IDs can resemble authentication failures

Live verification found that a 32-character compact page ID caused `UserValidationError` on reads and generic `ValidationError` on writes, while the same ID in hyphenated UUID form read and appended successfully. Supply canonical hyphenated UUIDs to v3 handlers; automatic compact-ID normalization is outside this change. Before attributing a 400 to expired credentials or workspace selection, compare the ID format. If validation persists, confirm the target page's `space_id` matches the configured workspace (configuration currently selects the first workspace deterministically).

## ID Disambiguation

Search results for databases return three distinct IDs. Using the wrong one is the #1 cause of 400 errors.

| ID Type | v3 Table | Source | Used By |
|---------|----------|--------|---------|
| Block wrapper ID | `block` | search `id` field, page URLs | `retrieve_data_source`, `get_page_content`, `retrieve_database` |
| Collection ID | `collection` | search `collection_id` field | `query_data_source` (resolved internally), `parent.database_id` for row creation |
| Collection view ID | `collection_view` | block's `view_ids[0]` | View-specific ops (`page_sort` ordering) |

**Rule**: `id` for reads, `collection_id` for writes that target the database.

## Transaction Patterns

### Page under a page (block parent)

```
set block [pageID]          — create page record (parent_id, parent_table in args)
update block [pageID]       — set timestamps
listAfter block [parentID] ["content"]  — append to parent's content list
```

### Row in a database (collection parent)

```
set block [pageID]          — create page record (parent_id, parent_table in args)
update block [pageID]       — set timestamps
setParent block [pageID]    — link to collection via pointer format
```

**Do NOT** use `listAfter block [collectionID] ["content"]`. Collections are not blocks and have no content list. This causes a 400 with no useful error detail.

Optional: `listBefore collection_view [viewID] ["page_sort"]` controls row ordering in a view. Not required for the row to exist.

### Moving between parent types

| From | To | Remove ops | Add ops |
|------|----|-----------|---------|
| Block | Block | `listRemove block [old] ["content"]` | `listAfter block [new] ["content"]` |
| Block | Collection | `listRemove block [old] ["content"]` | `setParent` |
| Collection | Block | *(none — no content list)* | `listAfter block [new] ["content"]` |
| Collection | Collection | *(none)* | `setParent` |

Always `set` the `parent_id` and `parent_table` fields on the block regardless of parent type.

### Comment/discussion creation

```
set discussion [discID]     — with resolved: false (REQUIRED)
listAfter block [pageID] ["discussions"]
set comment [commentID]
listAfter discussion [discID] ["comments"]
```

Missing `resolved: false` causes `PostgresNullConstraintError` (400).

## `setParent` Command Shape

```json
{
  "command": "setParent",
  "pointer": {"table": "block", "id": "<page-id>", "spaceId": "<space-id>"},
  "path": [],
  "args": {"parentId": "<collection-id>", "parentTable": "collection"}
}
```

- `pointer` identifies the record being re-parented (the new page/row)
- `args.parentId` / `args.parentTable` identify the target parent (camelCase)
- Included inside `transactions[0].operations` by the shared write helper
- Builders can mix pointer and flat operations; submission converts all flat operations to pointers before sending

## Common 400 Error Causes

### `listAfter`/`listRemove` targeting wrong table

- **Symptom**: 400 with `"ValidationError: Something went wrong."` — no detail.
- **Cause**: Op targets `table: "block"` with a collection ID (or vice versa).
- **Debug**: Log the ops array before submission. For each `listAfter`/`listRemove`, verify the `table` field matches the actual record type of the `id`.
- **Fix**: Use `setParent` for collection parents. See `buildParentLinkOps` in `pages.go`.

### Missing `resolved: false` on discussion

- **Symptom**: 400 `PostgresNullConstraintError`.
- **Cause**: `set discussion` op missing `"resolved": false`.
- **Fix**: Always include `"resolved": false` in discussion creation args.

### Wrong ID type passed

- **Symptom**: 400 `ValidationError` from `queryCollection` or `query_data_source`.
- **Cause**: Block wrapper ID passed where collection ID expected (or vice versa).
- **Debug**: Check search results — `id` is the block, `collection_id` is the database.

### Generic 400 with no error body

- **Symptom**: `notion API error (400):` with empty or generic message.
- **Debug**: Binary-search the ops array. Split the transaction, submit each half to isolate the failing op. The v3 API validates all ops server-side with no per-op error detail.

## Transaction Format Reference

### Legacy flat operation format (internal builders only)

```json
{"command": "set", "table": "block", "id": "...", "path": [], "args": {"type": "page"}}
```

### Pointer operation format (wire format for every command)

```json
{"command": "setParent", "pointer": {"table": "block", "id": "...", "spaceId": "..."}, "path": [], "args": {"parentId": "...", "parentTable": "collection"}}
```

Operation builders are unchanged by the endpoint migration. Legacy flat operations are converted to pointers at submission, preserving command, arguments, path, and order inside the same transaction's `operations` array; they are not the top-level request body. Tests cover mixed formats and validate the batch envelope for every write handler.

## Known Limitations

- **`deleteBlock` with collection parent**: `blocks.go` sends `listRemove` with `parentTable` from the fetched block. If `parentTable` is `"collection"`, the `listRemove` targets the collection table which may not work. The `alive: false` set op still processes, so deletion functionally works. TODO: skip `listRemove` for collection parents (same pattern as `buildParentUnlinkOps`).
