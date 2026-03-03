---
name: pr-comments
description: Submit a PR review as inline GitHub comments on specific files and lines using the gh CLI.
---

# PR Comments

Post review findings as inline comments on specific diff lines via the GitHub Pull Request Reviews API and `gh` CLI.

## When to Use

- After `pr-review` produces findings and user wants them posted to the PR
- User asks to "add comments", "submit the review", "comment on the lines", or "do an actual review"

## Workflow

### Step 1: Gather Data

1. Get owner, repo, PR number
2. Get head commit SHA: `gh api repos/<owner>/<repo>/pulls/<number> --jq '.head.sha'`
3. Get the diff: `gh pr diff <number> --repo <owner>/<repo>`
4. Check existing comments: `gh api repos/<owner>/<repo>/pulls/<number>/comments` — don't duplicate

### Step 2: Map Findings to Diff Lines

The API only accepts lines that appear in the diff. For each finding, confirm the target line is in a `+` or context line. If not, use the nearest line in the same hunk.

### Step 3: Build Payload with Python

Always use a Python script to build the JSON — avoids shell escaping nightmares with Markdown and code fences:

```python
import json

comments = [
    {
        "path": "pkg/foo.go",
        "line": 42,
        "side": "RIGHT",
        "body": "`format` is user-provided and goes straight into the SQL string via Sprintf — something like `text) DROP TABLE users; --` would work. Should validate against an allow-list.\n\n```go\nvar validFormats = map[string]bool{\"text\": true, \"json\": true, \"xml\": true, \"yaml\": true}\n\nif !validFormats[strings.ToLower(format)] {\n    return errResult(fmt.Errorf(\"invalid format: %s\", format))\n}\n```"
    },
]

payload = {
    "commit_id": "<sha>",
    "event": "COMMENT",
    "body": "Looks good overall, nice job on X. Few things inline.",
    "comments": comments,
}

with open("/tmp/review_payload.json", "w") as f:
    json.dump(payload, f)
```

Then submit:
```bash
gh api repos/<owner>/<repo>/pulls/<number>/reviews \
  --method POST \
  --input /tmp/review_payload.json \
  --jq '.html_url'
```

### Step 4: Handle Errors

- **422 line not in diff**: Use nearest diff line in the same hunk
- **422 validation failed**: Re-fetch head SHA and retry — branch may have been updated
- **403 not accessible**: Fall back to `gh pr comment <number> --repo <owner>/<repo> --body "..."`

## Comment Voice and Style

Write comments like a peer reviewer who's read the code carefully and is being helpful, not like an automated tool generating a report. Match the user's communication style from conversation history.

**Key rules:**

- **No severity titles or labels.** Don't start comments with "Must Fix —", "Should Fix —", "Nit —", or any bolded header/title. Just talk about the issue directly.
- **Conversational, not formulaic.** Write like you'd talk in a PR review — varied sentence structure, no repeated patterns across comments. Each comment should feel like its own thought, not a template fill-in.
- **Lead with the problem, not a category.** Instead of "**Security — SQL injection**" just say what's wrong: "`format` goes straight into the SQL string here — a value like `text); DROP TABLE ...` would work."
- **Keep it tight.** 2-4 sentences explaining the issue is usually enough. Don't over-explain things the author likely already understands.
- **Code suggestions are inline.** When suggesting a fix, just drop the code block naturally after explaining the issue. No "**Suggestion:**" header.
- **Skip the preamble.** Don't start every comment with "Great work but..." or "Nice job here, however...". Just get to the point.
- **Use "we" and "this" not "you should".** "We should validate this" or "this needs a read-only tx" reads better than "you need to add validation here."
- **One idea per comment.** Don't combine unrelated issues. Each comment lives on the line it's about.

**Good example:**
```
`format` is user-provided and goes straight into the SQL string here — something like `text) DROP TABLE users; --` would work. Worth validating against an allow-list and wrapping in a read-only tx like `queryTool` does.

\```go
var validFormats = map[string]bool{"text": true, "json": true, "xml": true, "yaml": true}
\```
```

**Bad example:**
```
**Must Fix — SQL injection via unsanitized `format` parameter**

The `format` argument is user-provided and interpolated directly into SQL via `fmt.Sprintf`. A malicious value like `text) DROP TABLE users; --` would produce valid destructive SQL.

**Suggestion:** Validate `format` against an allow-list:
```

The bad example reads like a security scanner output. The good example reads like a human who noticed something.

**Overall review body:** Keep the `body` field (the top-level review summary) to 1-2 natural sentences. Lead with something positive if warranted, mention roughly how many things to look at. Don't list categories or use bullet points.

**Event type:** Choose the event based on the review findings from `pr-review`:

- `"APPROVE"` — if there are **no "Must Fix (Blocking)"** items in the review. Non-blocking suggestions and nits are fine alongside an approval.
- `"REQUEST_CHANGES"` — if the user explicitly asks to block the PR.
- `"COMMENT"` — if there **are** blocking items but the user hasn't explicitly asked to block. Blocking items warrant inline comments but not a hard gate unless requested.
