# Script Engine

Sandboxed ES5.1 JavaScript runtime (goja) that lets LLMs chain multiple Switchboard tool calls in one MCP invocation.

## Architecture

```
server/server.go → engine.Run(ctx, source) → goja VM → api.call()/api.tryCall() → executor.Execute() → integration adapter
```

- Fresh VM per execution — no state leaks between runs
- `Executor` interface decouples from server layer — tests use `mockExecutor`
- JS return value is JSON-serialized as `ToolResult.Data`

## JS API Surface

| Function | Behavior on error | Returns |
|----------|-------------------|---------|
| `api.call(toolName, args?)` | Throws (kills script) | Parsed JSON result |
| `api.tryCall(toolName, args?)` | Returns error envelope | `{ok: bool, data/error: any}` |
| `console.log(...args)` | N/A | Collected; included in result on error or when script returns `undefined` |

### goja panic pattern

`panic(vm.NewGoError(...))` is goja's equivalent of `throw new Error()` in JS. goja recovers it internally during `vm.RunString()` — the Go process never sees an unrecovered panic. This is why `api.call()` uses panic for errors.

## Safety Limits

| Limit | Default | Constant |
|-------|---------|----------|
| Execution timeout | 30s | `DefaultTimeout` |
| Max tool calls per script | 50 | `DefaultMaxCalls` |
| Max script size | 64KB | `MaxScriptSize` |
| Max console.log entries | 100 | `MaxLogEntries` |

- `api.call()` and `api.tryCall()` share the same call counter
- Timeout enforced via `context.WithTimeout` + `vm.Interrupt()` goroutine
- Context cancellation kills the script for both call and tryCall — nothing can execute anyway
- maxCalls exceeded: `call` kills the script (panic), `tryCall` returns `{ok: false, error: "exceeded maximum..."}` — preserves partial results

## Conventions

- Options pattern: `WithTimeout(d)`, `WithMaxCalls(n)`
- `parseCallArgs()` extracts tool name + args from goja function calls — shared by both call and tryCall
- `parseResult()` JSON-parses ToolResult.Data into a goja value — shared by both call paths
- Tests use `mockExecutor` with a `results map[string]*mcp.ToolResult` for per-tool response stubs
