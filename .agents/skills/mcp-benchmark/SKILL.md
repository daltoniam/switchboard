---
name: mcp-benchmark
description: >
  Live benchmark protocol for Switchboard's MCP server. Runs real tool-calling
  sequences against enabled integrations, tracks failure metrics, and identifies
  impediments to successful LLM tool usage.
  Use when: "benchmark", "test the MCP", "run user stories", "smoke test
  integrations", after adding/changing integrations or tools, after changing
  compaction specs or search logic, before releases.
  Not for unit testing (use make test) or load testing.
metadata:
  author: switchboard
  version: "1.0"
---

# MCP Benchmark

Run live tool-calling sequences against enabled Switchboard integrations. Measure
failure rates, identify silent data loss, and report impediments to LLM tool usage.

## When to Use

- After adding or modifying an integration adapter
- After changing compaction specs, search logic, or server error handling
- Before releases
- When evaluating MCP quality or comparing before/after changes

## Prerequisites

- Switchboard running locally or accessible via MCP
- At least one integration enabled with valid credentials
- Access to `search` and `execute` MCP tools

## Hard Rules (apply to ALL phases)

1. **Discovery before identity**: NEVER pass an org name, team slug, channel ID, project name, or any entity identifier to a tool unless you discovered it from a prior list/search call in this session. Use authenticated-user tools (e.g., `github_list_user_repos`) as safe entry points.
2. **Read-only**: NEVER call create/update/delete/send tools. All scripts must be read-only.
3. **Record everything**: Every call gets a row in the results table, even failures.
4. **`{}` is failure**: An empty object response means compaction stripped all fields. Flag as Critical.

## Protocol

Execute phases in order. Record every result. Do not skip phases.

### Phase 1: Discovery (read-only)

1. Call `search({"limit": 0})` to get total tool count and enabled integrations
2. For each enabled integration, call `search({"query": "<integration_name>", "limit": 5})` to sample tool definitions
3. Record: total tools, enabled integrations, tools per integration

**Output table:**

| Integration | Tools | Status |
|-------------|-------|--------|
| github      | 100   | enabled |
| slack       | 42    | enabled |
| ...         | ...   | ...    |

### Phase 2: Single-Tool Smoke Tests

For each enabled integration, run ONE read-only list/search tool. Choose the
safest entry point (list, not create/update/delete).

**Suggested smoke tests per integration:**

| Integration | Tool | Args |
|-------------|------|------|
| github | `github_list_user_repos` (safe — uses authenticated user; `github_list_org_repos` requires a known org name — discover first via `github_list_user_orgs`) | `{per_page: 3}` |
| linear | `linear_list_issues` | `{first: 5}` |
| sentry | `sentry_list_org_issues` | `{query: "is:unresolved"}` |
| slack | `slack_list_conversations` | `{limit: 5}` |
| notion | `notion_search` | `{query: "<any known term>", limit: 3}` |
| metabase | `metabase_list_databases` | `{}` |
| datadog | `datadog_search_logs` | `{query: "*", from: "-1h"}` |
| aws | `aws_sts_get_caller_identity` | `{}` |
| posthog | `posthog_list_projects` | `{}` |
| postgres | `postgres_list_schemas` | `{}` |
| clickhouse | `clickhouse_list_databases` | `{}` |
| pganalyze | `pganalyze_list_servers` | `{}` |
| rwx | `rwx_list_workspaces` | `{}` |
| gmail | `gmail_list_messages` | `{max_results: 5}` |
| homeassistant | `homeassistant_list_states` | `{}` |
| ynab | `ynab_list_budgets` | `{}` |
| gcp | `gcp_list_projects` | `{}` |

For any integration not listed, use the first list/search tool found via
`search({"integration": "<name>"})`.

**For each call, record:**

| Field | How to check |
|-------|-------------|
| Tool name | What you called |
| Response shape | Array, columnar (`columns`/`rows`/`constants`), object, or `{}` |
| Columnar format | If 8+ items, verify columnar shape with `columns`+`rows`. Check `constants` for lifted uniform values |
| Empty object `{}` | **FLAG as compaction shape mismatch (Critical)** |
| Error | Record error message verbatim |
| Approximate size | Eyeball response length; flag if approaching 50KB |

### Phase 3: Search Discoverability Tests

Test natural-language queries that an LLM would realistically use. These validate
that search AND-matching, descriptions, and synonyms work correctly.

| Query | Expected Tool | Why |
|-------|--------------|-----|
| `send message slack` | `slack_send_message` | Common verb "send" |
| `post message` | `slack_send_message` | Synonym "post" |
| `list issues` | github/linear/sentry list issues | Cross-integration |
| `create ticket` | `linear_create_issue` | Synonym "ticket" for "issue" |
| `run query` | metabase/postgres execute query | Ambiguous intent |
| `get pull request` | `github_get_pull` | Noun phrase |

**For each query, record:**

| Field | How to check |
|-------|-------------|
| Total results | Count returned tools |
| 0 results | **FLAG as search gap (High)** |
| Expected tool missing from first page | **FLAG as ranking issue (Medium)** |

### Phase 4: Cross-Integration Scripts

Run 2-3 scripts that chain tools across integrations. Use `api.tryCall` for
resilience so partial results are preserved.

**IMPORTANT: Read-only scripts only.** Do NOT send Slack messages, create issues,
or perform any write operations during benchmarking. Scripts should read and
cross-reference data, not mutate state.

**Template script pattern:**

```javascript
// Discover data from one integration, cross-reference another.
var data1 = api.call('<integration1_list_tool>', {<args>});
var data2 = api.tryCall('<integration2_tool>', {<args_from_data1>});
({source: data1, cross_ref: data2});
```

**Example scripts (adapt to enabled integrations):**

1. **Sentry+Linear cross-ref**: List unresolved Sentry issues, search Linear for matching issue titles
2. **Linear+GitHub cross-ref**: List Linear issues, search GitHub PRs matching issue IDs
3. **Notion search + page content**: Search Notion, get full page content for top result

**Record per script:**

| Field | How to check |
|-------|-------------|
| Number of api.call()s | Count calls in script |
| Errors | Which call failed and error message |
| Script rewritten? | Did you have to modify the script to get results? Why? |
| Execution time | Did it approach 30s timeout? |

### Phase 5: Metrics & Report

Calculate metrics from all phases:

| Metric | Formula |
|--------|---------|
| Script rewrite rate | Scripts rewritten / scripts attempted |
| Integration error rate | Upstream API errors / total calls |
| Server error rate | Tool-not-found + 50KB exceeded / total calls |
| Silent data loss | Responses returning `{}` when data expected |
| Search miss rate | 0-result queries / total search queries |
| Search false-negative rate | Expected tool not in results / total queries |

**Report template:**

```
## MCP Benchmark Report - [date]

### Environment
- Enabled integrations: [list]
- Total tools: [count]

### Results
| Metric | Value | Target | Status |
|--------|-------|--------|--------|
| Script rewrite rate | X% | <10% | PASS/FAIL |
| Integration errors | X | 0 | PASS/FAIL |
| Server errors | X | 0 | PASS/FAIL |
| Silent data loss | X | 0 | PASS/FAIL |
| Search miss rate | X% | 0% | PASS/FAIL |

### Smoke Test Results
| Integration | Tool | Shape | Status |
|-------------|------|-------|--------|
| ... | ... | ... | PASS/FAIL |

### Search Discoverability Results
| Query | Results | Expected Tool Found? | Status |
|-------|---------|---------------------|--------|
| ... | ... | ... | PASS/FAIL |

### Cross-Integration Script Results
| Script | Calls | Errors | Rewritten? | Status |
|--------|-------|--------|------------|--------|
| ... | ... | ... | ... | PASS/FAIL |

### Findings
[List each failure: tool name, what happened, severity, suggested fix]

### Comparison to Previous
[If previous benchmark exists, note improvements/regressions]
```

## Common Mistakes

- **Writing during benchmarks**: NEVER send messages, create issues, or mutate
  state. All scripts must be read-only.
- **Guessing org/repo/channel names**: NEVER pass a name to a tool that requires
  an org, team, channel, or project unless you discovered it from a prior list call.
  `github_list_user_repos` is safe (uses authenticated user); `github_list_org_repos`
  requires a known org — discover via `github_list_user_orgs` first.
- **Using `api.call` for optional steps**: Use `api.tryCall` for cross-integration
  calls where partial results are acceptable.
- **Not checking for `{}`**: An empty object response looks like success but means
  compaction stripped everything. Always check response shape.
- **AND-matching gotcha**: Search requires ALL words to match. Use fewer, more
  specific words. `"slack send"` is better than `"slack send message channel"`.
- **Ignoring parameter defaults**: If a tool ignores your `per_page`/`limit`,
  that's a bug worth flagging.
- **Skipping discovery**: Don't assume which integrations are enabled. Run Phase 1
  first.
