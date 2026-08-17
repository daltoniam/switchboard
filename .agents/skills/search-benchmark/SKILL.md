---
name: search-benchmark
description: >
  Cross-model search quality benchmark for Switchboard's tool discovery.
  Dispatches identical search scenarios to opus, sonnet, and haiku in parallel,
  compiles a comparison table, and identifies optimization opportunities.
  Use when: "benchmark search", "test search quality", "run search benchmark",
  after changing scoring logic, synonyms, stop words, IDF, or tool descriptions,
  after adding new integrations, or when evaluating Phase 2 tag impact.
  Also use when the user mentions "search hit rate", "search recall", or
  "did search get better/worse". Not for full MCP smoke tests (use mcp-benchmark)
  or unit testing (use make test).
metadata:
  author: switchboard
  version: "1.0"
---

# Search Quality Benchmark

Measures how well LLMs discover tools through Switchboard's search, across
model tiers. The goal: prove that scoring changes translate to real LLM
behavior improvement, not just synthetic test improvements.

## Mental Model

We're optimizing **call efficiency**: how many search calls does an LLM need
to find the N relevant tools?

- Best case: `x = 1` — one search finds everything
- Target: `1 ≤ x < N`
- Worst case: `x ≥ N` — one call per tool, or worse (retries on vocabulary misses)

Two dimensions of improvement:
1. **Fewer misses per tool** — synonym expansion, stop-word filtering
2. **More tools per hit** — cross-integration discovery, keyword tags

## Two Benchmarks

### 1. Synthetic (Go test)

Fast, deterministic, no LLM involved. Tests the scoring algorithm in isolation.

```bash
go test -v -run TestSearchBenchmark ./server/
```

Reports recall@K for both old (substring AND) and new (synonym+TF-IDF) against
46 curated test cases (17 single-tool + 29 multi-tool intent across 9 personas).

Run this first — it's instant and catches regressions.

### 2. Live Cross-Model (this skill's main output)

Tests the full loop: LLM picks query terms → search returns results → we evaluate
whether the right tools surfaced. Reveals how different model tiers interact with
the scoring engine differently.

## Protocol

### Step 1: Verify Connection

```
search({"limit": 0})
```

Check total tools and integrations. With `--discover-all`, expect ~898 tools
across 21 integrations. Without it, only enabled integrations are searchable.

### Step 2: Dispatch 3 Agents in Parallel

Launch opus, sonnet, and haiku **simultaneously** using the Agent tool with
`model` parameter and `run_in_background: true`.

Each agent gets identical instructions:

```
You are benchmarking Switchboard's search tool across {TOTAL} tools and
{INTEGRATIONS} integrations. For each scenario below, use the
mcp__switchboard__search tool to find relevant tools. DO NOT execute any
tools — search only. Make exactly ONE search call per scenario.

For EACH scenario, record: the exact query you used, total results, and the
top 3 tool names with their integrations.

Scenarios:
1. "I need to create a Linear ticket"
2. "Send a Slack message to the team"
3. "Look up Sentry errors from today"
4. "Find GitHub pull requests for review"
5. "Investigate a production error across logging and error tracking"
6. "What deployed recently and did anything break?"
7. "Slow database queries need investigation"
8. "Draft a follow-up email about what we agreed to fix"

After all 8 scenarios, output ONLY this JSON (no other text):
{
  "model": "{MODEL}",
  "scenarios": [
    {
      "scenario": 1,
      "query_used": "...",
      "total_results": N,
      "top_3": [{"name": "...", "integration": "..."}, ...]
    }
  ]
}
```

### Step 3: Compile Comparison Table

After all 3 complete, build the per-scenario table:

```markdown
| Scenario | Opus query (results) | Sonnet query (results) | Haiku query (results) | #1 Tool | Correct? |
|----------|---------------------|----------------------|---------------------|---------|----------|
```

### Step 4: Score Against Expected Results

| # | Scenario | Expected #1 Tool | Expected Integration |
|---|----------|-------------------|---------------------|
| 1 | Create Linear ticket | `linear_create_issue` | linear |
| 2 | Send Slack message | `slack_send_message` | slack |
| 3 | Sentry errors | `sentry_list_issues` or `sentry_list_org_issues` | sentry |
| 4 | GitHub PRs | `github_list_pulls` | github |
| 5 | Production error | `sentry_list_project_events` + `datadog_search_logs` | sentry, datadog |
| 6 | Recent deploys | `github_list_deployments` or `github_list_releases` | github |
| 7 | Slow DB queries | `pganalyze_get_query_stats` | pganalyze |
| 8 | Draft email | `gmail_create_draft` | gmail |

Report hit rate per model at multiple rank tiers:

```
### Hit Rate by Rank Tier
| Model  | Top-1 | Top-3 | Top-5 | Top-8 |
|--------|-------|-------|-------|-------|
| Opus   | X/8   | X/8   | X/8   | X/8   |
| Sonnet | X/8   | X/8   | X/8   | X/8   |
| Haiku  | X/8   | X/8   | X/8   | X/8   |
```

- **Top-1**: LLM doesn't need to choose — the best tool is first
- **Top-3**: LLM picks from a small set, almost always correct
- **Top-5**: Tool is visible in the default result window
- **Top-8**: Tool is present on the first page

The key metric is **Top-3** — if the correct tool is in the top 3, the LLM
has enough context to make the right choice. Top-1 is ideal but not required.

### Step 5: Identify Optimization Opportunities

Classify each miss into one of these categories — this tells you what to fix:

| Category | Scoring layer fix | Example |
|----------|-------------------|---------|
| Missing synonym | Add to `synonymGroups` in `server/search.go` | "errors" should match "issues" |
| Stop-word gap | Add to `stopWords` in `server/search.go` | Verbose query drowning signal |
| Wrong integration ranked first | Phase 2 tags (not yet implemented) | rwx above sentry for "production error" |
| Zero results | Vocabulary gap — no matching words exist | "deployed recently" has no tool matches |

### Step 6: Report

Output a summary block:

```
## Search Benchmark Results — {date}

Corpus: {N} tools, {M} integrations
Server flags: {--discover-all or default}

### Hit Rate by Rank Tier
| Model  | Top-1 | Top-3 | Top-5 | Top-8 |
|--------|-------|-------|-------|-------|
| Opus   | X/8   | X/8   | X/8   | X/8   |
| Sonnet | X/8   | X/8   | X/8   | X/8   |
| Haiku  | X/8   | X/8   | X/8   | X/8   |

### Synthetic Benchmark
Single-tool recall@5: X/19 (XX%)
Integration recall:   X/92 (XX%)

### Optimization Opportunities
[List from Step 5]

### Comparison to Previous
[If prior run data available, note improvements/regressions]
```

## Adding New Scenarios

The 8 scenarios above cover common personas (DevOps, PM, CS, CEO, analyst).
To add scenarios:

1. Add to the agent prompt template in Step 2
2. Add expected results to the table in Step 4
3. Update the hit rate denominator in Step 6
4. Consider adding matching cases to `server/search_benchmark_test.go` for
   the synthetic benchmark

Good scenarios are **natural language** (how a real person would phrase it,
not technical tool-speak) and **cross-integration** (need tools from 2+
integrations to fully address).
