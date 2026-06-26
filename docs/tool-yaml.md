# tool-yaml.md — Tool Definition Authoring Reference

Each adapter ships a `tools.yaml` file with its tool descriptions and parameter specs. The Go side embeds the YAML at compile time and parses it into typed values once at startup. Downstream code never sees the raw YAML. It works with the typed `[]ToolDefinition`.

This doc covers the schema, the rules, and the migration pattern.

## File Location

```
integrations/<name>/tools.yaml
integrations/<name>/tools.go    # 3-line embed-and-load; no inline literals
```

Every adapter also has a `compact.yaml` next to its `tools.yaml`. They sit next to each other but they answer different questions. `tools.yaml` is what the model sees when it lists or executes a tool. `compact.yaml` controls which fields come back from an API call. Don't merge them.

## Schema

```yaml
version: 1
tools:
  <tool_name>:
    description: "<one to three sentences>"
    parameters:
      <param_name>:
        description: "<what it is and how to get a valid value>"
        required: true    # only `true` is permitted; absence means optional
      <param_name>:
        description: "<...>"
```

### Fields

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `version` | int | yes | Must be `1` |
| `tools` | map | yes | Keyed by fully-prefixed tool name |
| `tools.<name>.description` | string | yes | The tool's LLM-facing description |
| `tools.<name>.parameters` | map | no | Absent or `{}` means no parameters |
| `tools.<name>.parameters.<p>.description` | string | yes (if param present) | Parameter's LLM-facing description |
| `tools.<name>.parameters.<p>.required` | bool | no | Only `true` is permitted (required parameter). Absence means optional. `required: false` is rejected as a parse error — it is redundant with absence. |

### Tool naming

Tool names must be fully prefixed with the integration name: `github_list_issues`, `datadog_search_logs`. This matches the `AGENTS.md` naming convention and the dispatch map keys.

### Example

```yaml
version: 1
tools:
  myapi_list_items:
    description: "List items in the project. Start here for item discovery and triage."
    parameters:
      project_id:
        description: "The project ID (from myapi_list_projects)."
        required: true
      limit:
        description: "Maximum items to return (default 50, max 200)."
  myapi_get_item:
    description: "Get a single item by ID. Use after myapi_list_items."
    parameters:
      item_id:
        description: "The item ID (from myapi_list_items)."
        required: true
```

## Strict-Mode Validation

The loader rejects unknown keys at every nesting level. A typo like `descripton:` instead of `description:` fails the load. So does `requird: true` inside a parameter block, or any other key the schema doesn't recognize.

`MustLoadToolsYAML` panics on failure, so the server crashes at startup if any adapter ships a malformed `tools.yaml`. We chose loud-fail-at-init over silent-blank-prose-to-the-model.

The same strictness applies to the values that key names map to. `required: false` is rejected. Absence already conveys optional, so the explicit `false` is redundant and the author should omit the key. The error message tells them so.

## Parameter Declaration Order

Whatever order you write parameters in YAML is the order they come back from the loader, the order the dispatch sees, and the order that shows up in the wire output. The loader walks the YAML's underlying node tree directly so iteration is deterministic; Go's map iteration would have randomized it.

This matters because the model reads parameter lists top to bottom. Put the most important parameter first.

## Wire-Format Normalization

One thing the loader doesn't preserve verbatim: the `required: [...]` array in each tool's `inputSchema`. We sort that array alphabetically at the wire boundary before emitting it. JSON Schema treats `required` as a set, so sort order carries no meaning to the consumer.

The reason we sort is mechanical, not semantic. With sorted output we can lock the live wire response and compare against it byte-for-byte. That lock lives at `server/tools_list.lock.json`, and the test that enforces it is `TestToolsList_MatchesWireLock` in `server/wire_test.go`. It is a lock file in the same sense as `go.sum`: a generated artifact derived from the `tools.yaml` files, committed so CI can catch drift. You don't hand-edit it. After an intentional change to a tool's name, description, parameters, or required flags, regenerate it with `go test ./server -run TestToolsList_MatchesWireLock -update` and commit the updated lock alongside the YAML change. As long as that test stays green, nothing has silently changed what the model sees.

## tools.go Wiring

Every adapter's `tools.go` is a 3-line file:

```go
package myapi

import (
    _ "embed"
    mcp "github.com/daltoniam/switchboard"
)

//go:embed tools.yaml
var toolsYAML []byte

var tools = mcp.MustLoadToolsYAML(toolsYAML)
```

`//go:embed` bundles the YAML into the binary at compile time. One build step covers both the Go and the YAML, so prose-only updates to `tools.yaml` rebuild the same way code changes do.

## Parity Test Pattern (Migration Only)

When we migrate an existing adapter from inline Go literals to YAML, we copy the old definitions verbatim into a `legacyTools` fixture and assert the YAML-loaded `tools` equals it. This is the rope that catches conversion mistakes: silent prose loss, dropped parameters, wrong required flag.

New adapters starting YAML-first skip this. There's nothing to compare against.

```go
//go:build parity

package myapi

import (
    "testing"
    mcp "github.com/daltoniam/switchboard"
    "github.com/stretchr/testify/require"
)

// legacyTools is a verbatim snapshot of the pre-YAML inline definitions.
// Removed in Phase 3 cleanup after all adapters migrate.
var legacyTools = []mcp.ToolDefinition{
    {
        Name:        mcp.ToolName("myapi_list_items"),
        Description: "<verbatim from old tools.go>",
        Parameters: []mcp.Parameter{
            {Name: mcp.ParamName("project_id"), Description: "<verbatim>", Required: true},
        },
    },
}

func TestToolsYAML_ParityWithLegacy(t *testing.T) {
    require.Equal(t, legacyTools, tools)
}
```

Run with: `go test -tags parity ./integrations/<name>/...`

The build tag keeps the `legacyTools` fixture out of normal builds. After every adapter has migrated, Phase 3 deletes these files in one sweep.

## Variant Story

The broader prompt-variant pattern (operational vs per-session, per-model tuning) lives in [`docs/prompts.md`](prompts.md). When tool description variants land, they'll follow the same approach. We kept `version: 1` at the top of every file so we can grow into structured variant extensions without breaking existing YAML.
