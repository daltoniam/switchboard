# Tool Search

## Overview

Switchboard exposes two meta-tools: **search** discovers operations, **execute** runs them. An LLM never calls integration APIs directly — it searches for the right tool first, then executes it by name.

```
search({"query": "create ticket"}) → [{name: "linear_create_issue", ...}]
execute({"tool_name": "linear_create_issue", "arguments": {...}}) → result
```

Search scores ~850 tools across integrations using synonym-expanded TF-IDF. The index is built once at startup; queries are zero-allocation beyond the result slice.

## How Scoring Works

Each tool's name, description, and integration name are tokenized into words. At startup, the engine computes an **IDF (inverse document frequency)** weight for every word: words that appear in fewer tools are worth more. "github" appears in 100+ tools, so it's nearly worthless. "deploy" appears in a handful, so it's highly valuable.

When a query arrives:

1. **Tokenize** the query — split on whitespace/underscores/punctuation, lowercase, filter stop words.
2. **Expand** each query word through the synonym map (e.g., "ticket" expands to `["ticket", "issue", "issues", "task", "bug"]`).
3. **Score** each tool — for each query word, find the best-scoring synonym variant that exists in the tool's token set. Sum the IDF weights. Higher sum = better match.
4. **Sort** by score descending, then by integration name and tool name for stable tiebreaking.
5. **Filter** zero-score tools — if no query word (or synonym) matched, the tool is excluded.

The formula: `IDF(word) = log(totalTools / toolsContainingWord)`. A word appearing in 1 of 850 tools scores ~6.7; a word in 425 of 850 scores ~0.7.

## Synonym Groups

Synonym groups define equivalence sets of words that match interchangeably. Defined in `server/search.go` as `synonymGroups`:

```go
// Nouns
{"ticket", "issue", "issues", "task", "bug"},
{"table", "tables"},
{"label", "labels", "tag", "tags"},
{"diff", "patch", "changes"},
{"database", "databases", "db"},
// ... and more (see server/search.go for full list)

// Verbs
{"create", "add", "new", "make", "generate"},
{"get", "retrieve", "fetch", "read", "show", "view", "describe"},
{"list", "ls", "enumerate"},
{"deploy", "deploys", "deployment", "deployments", "release", "releases", "rollout", "ship"},
```

**IDF dilution warning**: Noun synonym groups where the union covers >60 tools can degrade search quality. The MAX-per-word scoring makes verb synonyms inherently safe (rare variants like "retrieve" carry the score), but noun groups with many common words dilute the IDF signal. Test with the search benchmark before adding high-union noun groups.

At startup, `buildSynonymMap` expands these into a bidirectional lookup map where each word maps to its full group (including itself). Groups must be disjoint — no word in multiple groups.

**To add a new synonym**: append to the appropriate existing group, or create a new slice if no group fits.

**Plural/stemming caveat**: the tokenizer does exact matching with no stemming. "errors" and "error" are different words. Flatten both forms into the same group. Only create a standalone pair like `{"metric", "metrics"}` when the word isn't in any other group.

## Stop Words

Stop words are common English function words (articles, prepositions, conjunctions) that carry no semantic signal. Defined in `server/search.go` as `stopWords`.

Why they're needed: tool descriptions are short (10-30 words). In longer documents, IDF naturally zeroes out function words because they appear everywhere. With terse descriptions, words like "for" and "the" may appear in only a fraction of tools and receive artificially high IDF scores. The stop-word list prevents this.

The list is a closed linguistic class — English doesn't gain new prepositions. It's write-once, not a maintenance burden.

## Query Formulation

The search tool's description coaches LLMs on query strategy:

```
Query format — use 2-3 keywords, not full sentences:
- {"query": "create ticket"} — synonym matching finds linear_create_issue
- {"query": "slack send message"} — always include the integration name
- {"integration": "sentry", "query": "errors"} — or use the integration filter
```

Keywords beat sentences because:
- Stop-word filtering discards most of a sentence ("find me the issues that are open" → "find", "issues", "open")
- TF-IDF rewards specificity — fewer query words means less noise diluting the signal
- Synonym expansion compensates for vocabulary mismatches, so exact tool names aren't required

The `integration` parameter pre-filters tools before scoring, useful when the LLM already knows which integration to target.

## Tool Description Guidelines

Tool descriptions are the only context an LLM gets for tool selection. The scoring engine indexes them alongside tool names and integration names. Guidelines:

- **Three-tier pattern**: workflow entry points get routing hints ("Start here for most workflows"), supporting tools get chaining hints, subsumed primitives get prefer-over hints
- **Domain keywords**: include the vocabulary an LLM is likely to search for. A deployment tool should mention "deploy", "release", "ship" in its description even if the tool name is `github_list_deployments`
- **Stemming awareness**: the tokenizer does exact matching. If users might search for both "error" and "errors", include both in the description or rely on synonym groups
- **Gotcha prevention**: surface ID/parameter confusion in descriptions and parameter strings

See [field-compaction.md](field-compaction.md) § "Tool Description Design" for the full style guide.

## Benchmarking

### Synthetic Benchmark (`go test`)

```bash
go test -v -run TestSearchBenchmark ./server/
```

`search_benchmark_test.go` defines 46 curated test cases across two categories:

- **Single-tool** (17 cases): tests vocabulary matching — does synonym expansion find `linear_create_issue` when the query is "create ticket"? Measured as recall@K (did the expected tool appear in the top K results?).
- **Multi-tool** (29 cases): tests cross-integration discovery across 9 personas (DevOps, PM, CS, CEO, analyst, security, sales, marketing, growth). Measured as integration recall — did results include tools from all expected integrations?

The test is reporting-only (no pass/fail assertions). It compares the current scoring engine against the old substring-AND approach and reports deltas.

### Live Cross-Model Benchmark (`/search-benchmark` skill)

Tests the full loop: LLM picks query terms, search returns results, results are evaluated against expected tools. Dispatches identical scenarios to Opus, Sonnet, and Haiku in parallel.

Key metric: **Top-3 hit rate** — if the correct tool is in the top 3 results, the LLM has enough context to make the right choice.

Reports hit rate at multiple rank tiers (Top-1, Top-3, Top-5, Top-8) per model, plus optimization opportunities classified by fix category (missing synonym, stop-word gap, wrong integration ranked first, zero results).

## Architecture

### File Layout

| File | Purpose |
|------|---------|
| `server/search.go` | Scoring engine — `tokenize`, `computeIDF`, `scoreTool`, `scoreTools`, `buildSynonymMap`, `synonymGroups`, `stopWords` |
| `server/search_benchmark_test.go` | Synthetic benchmark corpus (46 cases) + `TestSearchBenchmark` |
| `server/server.go` | `buildSearchIndex` (startup), `handleSearch` (request handler), search tool MCP definition |

### SearchIndex Struct

```go
type SearchIndex struct {
    IDF      map[string]float64       // word → IDF weight
    SynMap   map[string][]string      // word → synonym group (self-inclusive)
    AllTools []toolWithIntegration    // all indexed tools with pre-computed token sets
}
```

Shared as a read-only value between `Server` and `ProjectRouter` via `Server.SearchIndex()`.

### Startup: `buildSearchIndex`

1. `buildSynonymMap(synonymGroups)` — expands synonym groups into the bidirectional lookup map.
2. Collects all `ToolDefinition`s from enabled integrations (or all registered, if `--discover-all`).
3. `computeIDF(tools)` — tokenizes each tool's name + description + integration name, builds IDF weights, and stores pre-computed token sets on each `toolWithIntegration`.

### Request: `handleSearch`

1. Parse query, integration filter, limit, offset.
2. If query is non-empty: filter `allTools` by integration (if specified), call `scoreTools` with the pre-computed IDF and synonym maps. Token sets are already cached on each tool — no per-query tokenization.
3. If query is empty: return all tools sorted alphabetically.
4. Apply pagination (offset + limit) and columnarize the response if result count exceeds the threshold.
