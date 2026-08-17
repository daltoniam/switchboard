# Notion v3 Transaction Debugging Guide

How to diagnose and fix 400/500 errors from `submitTransaction`.

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
- Works with both `submitTransaction` and `saveTransactionsFanout`
- Can coexist with flat-format ops (`set`, `update`, `listAfter`) in the same transaction

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

### Flat format (Switchboard default)

```json
{"operations": [
  {"command": "set", "table": "block", "id": "...", "path": [], "args": {...}}
]}
```

### Pointer format (used by `setParent` and Notion web app)

```json
{"operations": [
  {"command": "setParent", "pointer": {"table": "block", "id": "...", "spaceId": "..."}, "path": [], "args": {...}}
]}
```

Both formats work with `submitTransaction`. Flat and pointer ops can be mixed in a single transaction.

## Known Limitations

- **`deleteBlock` with collection parent**: `blocks.go` sends `listRemove` with `parentTable` from the fetched block. If `parentTable` is `"collection"`, the `listRemove` targets the collection table which may not work. The `alive: false` set op still processes, so deletion functionally works. TODO: skip `listRemove` for collection parents (same pattern as `buildParentUnlinkOps`).
