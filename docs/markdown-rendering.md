# Markdown Rendering

Document-oriented tools can return Markdown instead of JSON, reducing LLM context overhead and eliminating thinking tokens spent interpreting nested structures.

## When to Use Markdown

**Use when the tool returns content meant to be read** — documents, pages, emails, comments, descriptions. The LLM consumes this as natural language, not as data to manipulate programmatically.

| Content Type | Example Tools | Use Markdown? |
|---|---|---|
| Page/document body | `notion_get_page_content`, `confluence_get_page` | Yes |
| Issue descriptions | `jira_get_issue` (description field is ADF) | Yes |
| Email bodies | `gmail_get_message`, `gmail_get_thread` | Yes |
| Comments/discussions | `notion_retrieve_comments`, `confluence_list_comments` | Yes |
| Blog posts | `confluence_get_blog_post` | Yes |

**Do not use when the tool returns structured data meant to be filtered, sorted, or chained** — search results, lists, database queries, metrics. The LLM needs field access for these.

| Content Type | Example Tools | Use Markdown? |
|---|---|---|
| Search results | `notion_search`, `jira_search_issues` | No — needs field access |
| List endpoints | `github_list_issues`, `slack_list_conversations` | No — needs columnar format |
| Database queries | `notion_query_data_source`, `postgres_query` | No — tabular data |
| Metadata/status | `github_get_pull` (status, labels, reviewers) | No — structured fields |
| Metrics/logs | `datadog_search_logs`, `sentry_list_issue_events` | No — structured data |

**The litmus test**: Would the LLM read this response like a human reads a document? If yes, markdown. If the LLM needs to extract specific fields for a follow-up call, JSON.

## How It Works

### Pipeline

```
Handler returns JSON (as always)
    ↓
processResult checks MarkdownIntegration
    ↓
RenderMarkdown(toolName, jsonBytes) → (Markdown, true)
    ↓
Compaction + columnarization SKIPPED
    ↓
Markdown returned to LLM
```

Scripts (`api.call()`) bypass `processResult` entirely via `toolExecutor` — they always get raw JSON for programmatic access.

### Interface

```go
// mcp.go
type MarkdownIntegration interface {
    RenderMarkdown(toolName ToolName, data []byte) (Markdown, bool)
}
```

Return `(md, true)` for tools that render markdown. Return `("", false)` for tools that return JSON normally. The server falls through to compaction for `false` tools.

### Implementing for a New Integration

1. **Create `markdown.go`** in the integration package with:
   - Semantic domain types for the rendering layer (e.g., `renderedPage`, `renderedComment`)
   - Raw JSON parse types (private, used only at the parse boundary)
   - `RenderMarkdown` method with a switch on tool names
   - Per-tool parse functions that unmarshal JSON → typed structs → `markdown.Builder` output
   - Import `"github.com/daltoniam/switchboard/markdown"` for shared utilities

2. **Add compile-time assertion** in the adapter's main `.go` file:
   ```go
   var _ mcp.MarkdownIntegration = (*myAdapter)(nil)
   ```

3. **Add parity test** `TestRenderMarkdown_ToolsCovered` — verifies tool names in `RenderMarkdown` match actual tools (mirrors dispatch map test pattern).

4. **Keep compact specs** as fallback — `RenderMarkdown` takes priority at runtime, but specs document the JSON shape and serve as fallback if markdown is bypassed.

### Shared Utilities (`markdown` package)

All shared markdown code lives in `markdown/` (`github.com/daltoniam/switchboard/markdown`).

| Utility | Import As | Use When |
|---|---|---|
| `markdown.Builder` | `markdown.NewBuilder()` | Assembling any markdown document (metadata headers, headings, attribution, dividers) |
| `markdown.FromHTML` | `markdown.FromHTML(html)` | Converting HTML/XHTML to markdown (Confluence storage format, Gmail HTML bodies) |
| `markdown.ApplyMarks` | `markdown.ApplyMarks(sb, ...)` | Inline formatting from mark flags (bold/italic/code/strike/link) — used by Notion rich text and Jira ADF |
| `markdown.WriteTable` | `markdown.WriteTable(sb, rows)` | Rendering `[][]string` as a markdown table — used by HTML converter and Jira ADF |
| `markdown.NoComments` | | Standard empty-comments response (`"No comments.\n"`) |
| `markdown.Markdown` | | Semantic type for rendered markdown content |

### Semantic Types

| Type | Package | Purpose |
|---|---|---|
| `mcp.ToolName` | `mcp` | Tool name parameter — flows through all interfaces without `string()` casts |
| `markdown.Markdown` | `markdown` | Rendered markdown content — flows from `Build()` through `RenderMarkdown` to `processResult` without casts |

### Output Format Conventions

- **Metadata header**: `<!-- integration:key=value -->` HTML comment on first line
- **Title**: `# Title` on second line
- **Attribution**: `*Author: X | Status: Y*` italic line via `b.Attribution(...)`
- **Block IDs**: `<!-- block:UUID -->` before addressable blocks (Notion) for edit operations
- **Comments**: `## Comments (N)` heading, `b.CommentAttribution(author, context, body)` per comment, `b.BlockquoteAttribution(author, ts, text)` for threaded discussions
- **Thread status**: `### Thread N (open|resolved)` for threaded discussions (Notion)

### Parse-Don't-Validate

`RenderMarkdown` is the parse boundary. Raw JSON enters as `[]byte`, typed structs exit. No `map[string]any` survives past the parse point in the rendering layer.

```
[]byte (JSON from handler)
    ↓  json.Unmarshal into raw* types
rawIssueResponse, rawPageResponse, etc.
    ↓  extract + convert at parse boundary
renderedIssue, renderedPage, etc.  (semantic types)
    ↓  markdown.Builder methods
markdown.Markdown  (returned to server)
```

For polymorphic formats like Jira ADF where full typing would require a custom unmarshaler, `map[string]any` is acceptable within the format converter (e.g., `adfToMarkdown`) but the outer rendering functions must accept typed structs.

### Input Format Converters

Each integration has its own input format that needs conversion:

| Integration | Input Format | Converter |
|---|---|---|
| Notion | v3 block tree with `properties.title: [["text"]]` rich text | `richTextToMarkdown` + `blocksToMarkdown` |
| Confluence | XHTML storage format with `ac:` namespace macros | `mcp.HTMLToMarkdown` (shared) |
| Jira | Atlassian Document Format (ADF) JSON tree | `adfToMarkdown` + `parseDescription` (handles string fallback) |
| Gmail | MIME multipart with base64url-encoded parts | `extractBody` (prefers text/plain, falls back to HTML via `mcp.HTMLToMarkdown`) |

### Resilience Patterns

- **Jira description**: `parseDescription` tries ADF map first, falls back to plain string — prevents silent data loss if description isn't ADF
- **Gmail text/plain**: `strings.TrimSpace(plain) != ""` check before preferring over HTML — prevents empty body when text/plain is whitespace-only
- **Notion API v3**: `unwrapRecordValue` handles both old `{value: {id,...}}` and new `{value: {value: {id,...}}}` recordMap formats
- **Unknown tool names**: `RenderMarkdown` returns `("", false)` — server falls through to JSON compaction gracefully

## Integrations Using Markdown

| Integration | Tools | Notes |
|---|---|---|
| Notion | `get_page_content`, `retrieve_comments` | Block tree → MD with block ID annotations for editing |
| Confluence | `get_page`, `get_blog_post`, `list_comments` | XHTML → MD via shared `HTMLToMarkdown` |
| Jira | `get_issue`, `list_comments` | ADF → MD with issue metadata header |
| Gmail | `get_message`, `get_thread`, `get_draft` | MIME extraction, text/plain preferred, HTML fallback |

## Adding Markdown to an Existing Integration

**Candidates**: Any integration that returns human-readable document content in a format the LLM has to interpret (HTML, ADF, rich text arrays, MIME).

**Not candidates**: Integrations returning pure structured data (Datadog metrics, PostgreSQL query results, GitHub PR metadata).

**Checklist**:
- [ ] Identify which tools return document content vs structured data
- [ ] Create `markdown.go` with semantic types + parse boundary
- [ ] Use `MarkdownBuilder` for document assembly
- [ ] Use shared utilities (`HTMLToMarkdown`, `ApplyMarks`, `WriteMarkdownTable`) where applicable
- [ ] Add compile-time interface assertion
- [ ] Add `TestRenderMarkdown_ToolsCovered` parity test
- [ ] Keep compact specs as fallback
- [ ] Handle format edge cases (empty content, null fields, format variants)
- [ ] TDD: write failing test before implementation
