# Response Optimizations

## Columnar JSON Format

Arrays of 8+ objects in execute/search responses are automatically reshaped to columnar format, eliminating per-record key repetition. Applied at the MCP response boundary (`processResult` for execute, `columnarizeResult` for scripts/search in `server/server.go`).

```json
// Per-record (< 8 items): [{...},{...}]
// Columnar (8+ items):
{"columns":["id","title","state"],"rows":[[1,"bug","open"],[2,"feat","closed"]],"constants":{"repo":"myrepo"}}
```

- **Constant lifting**: Columns where all values are identical (via `reflect.DeepEqual`) move to `"constants"` map and are removed from columns/rows
- **Density ordering**: Columns sorted by non-null count descending, alphabetical tiebreak — dense (important) columns appear first
- **Threshold**: `columnarMinItems = 8` in `compact.go`. Below 8, per-record format preserved for readability. At 8+, columnar saves 28%+ even on heterogeneous data. Applied to all non-error responses (execute, search, scripts) — mutation tools are unaffected because they return single objects, not arrays
- **Nested arrays**: `columnarizeNestedArrays` also converts nested `[]map[string]any` inside objects (e.g., `issues.nodes[]` inside a GraphQL envelope)
- **Implementation**: `buildColumnar` in `compact.go` — single function used by both top-level and nested array columnarization
- **Safe for LLMs**: Validated on Haiku, Sonnet, and o3 — all parse columnar format correctly

## Search Response Optimizations

Search responses (`handleSearch` in `server/server.go`) apply two additional optimizations:

- **Shared parameter deduplication**: Parameters with identical name+description across 3+ tools in the page are extracted to a top-level `shared_parameters` map and removed from per-tool `parameters`. Common params like `"owner": "Repository owner"` appear once instead of N times. Ambiguous names (same name, different descriptions) stay per-tool.
- **Search columnarization**: The `tools` array is columnarized (8+ results). Combined with constant lifting, single-integration searches get `constants: {"integration": "github"}`.
- **CRITICAL: deep-copy Parameters before mutation** — `searchToolInfo.Parameters` must be a fresh map, not a pointer to the integration's original `ToolDefinition.Parameters`. The `extractSharedParameters` function deletes keys from the map. Without the deep copy, searches progressively corrupt the integration's tool definitions.

## Script Field Projection

Scripts can project fields from `api.call()` / `api.tryCall()` results via an optional third argument:

```javascript
var issues = api.call("github_list_issues", {owner:"org",repo:"app"}, {fields:["number","title"]});
// issues = [{number:1,title:"bug"},{number:2,title:"feat"}] — only requested fields
```

- Third arg is optional — omitting it preserves existing behavior (full compacted result)
- `fields` array parsed as compact specs via `ParseCompactSpecs` → applied via `CompactAny` before JS parsing
- Compaction happens after the integration's own compaction (additive filtering, never expands)
- Implementation: `parseCallArgs` returns 3 values, `projectFields` in `script/engine.go`
