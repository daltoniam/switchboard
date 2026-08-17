# Go Anti-Patterns

Each pattern below has caused a production bug where arg extraction errors were silently swallowed, leading to zero-value parameters reaching external APIs.

## 1. Args scanner extractions after `r.Err()` guard

The `Args` scanner accumulates the first error — extractions after `r.Err()` silently return zero values with no error path. ALL extractions must happen BEFORE the single `r.Err()` check.

```go
// BAD: r.Int64 inside switch is after the guard — errors silently swallowed
r := mcp.NewArgs(args)
idType := r.Str("type")
if err := r.Err(); err != nil { return mcp.ErrResult(err) }
switch idType {
case "id":
    monID := r.Int64("monitor_id") // ← error never checked
}

// GOOD: extract everything up front, guard once
r := mcp.NewArgs(args)
idType := r.Str("type")
monID := r.Int64("monitor_id")
tags := r.StrSlice("tags")
if err := r.Err(); err != nil { return mcp.ErrResult(err) }
switch idType { ... }

// ALSO GOOD: use standalone calls inside conditional branches
switch idType {
case "id":
    monID, err := mcp.ArgInt64(args, "monitor_id")
    if err != nil { return mcp.ErrResult(err) }
}
```

## 2. Scanner for single extractions — use standalone instead

The scanner pattern (`NewArgs` + `r.Err()`) is for batches. For 1 parameter, use the standalone function directly.

```go
// BAD: scanner overhead for single extraction, easy to forget r.Err()
r := mcp.NewArgs(args)
resp, _, err := api.GetSLO(ctx, r.Str("id"), opts)

// GOOD: standalone call with explicit error check
id, err := mcp.ArgStr(args, "id")
if err != nil { return mcp.ErrResult(err) }
resp, _, err := api.GetSLO(ctx, id, opts)
```

## 3. Don't mix scanner + standalone in the same handler

Pick one pattern per function. Mixing creates confusion about which parameters are covered by `r.Err()` and which have their own error checks.

**Exception**: "set-if-present" update handlers may use the scanner for required fields and standalone calls for optional fields that should only be set when non-zero. This is acceptable when the scanner cannot distinguish "missing" from "zero value" and the handler needs per-field conditional control.

```go
// BAD: reader for some args, standalone for others
r := mcp.NewArgs(args)
owner := r.Str("owner")
if err := r.Err(); err != nil { return mcp.ErrResult(err) }
labels, err := mcp.ArgStrSlice(args, "labels") // separate error path
milestone, err := mcp.ArgStr(args, "milestone") // yet another

// GOOD: all scanner
r := mcp.NewArgs(args)
owner := r.Str("owner")
labels := r.StrSlice("labels")
milestone := r.Str("milestone")
if err := r.Err(); err != nil { return mcp.ErrResult(err) }

// ALSO GOOD: all standalone (when conditional logic needs per-field control)
owner, err := mcp.ArgStr(args, "owner")
if err != nil { return mcp.ErrResult(err) }
labels, err := mcp.ArgStrSlice(args, "labels")
if err != nil { return mcp.ErrResult(err) }
```
