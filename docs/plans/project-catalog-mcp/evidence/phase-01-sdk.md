# Phase 1 SDK feasibility evidence

## Provenance

| Item | Value |
|---|---|
| Base pin before this phase | `github.com/modelcontextprotocol/go-sdk v1.5.0` |
| Selected pin | `github.com/modelcontextprotocol/go-sdk v1.7.0` |
| Module source | official upstream, `proxy.golang.org` |
| Companion bump | `github.com/google/jsonschema-go v0.4.2` → `v0.4.3` (transitive) |
| Patch / replace | none |

v1.7.0 is the first stable upstream release that implements the published `2026-07-28` surface (`server/discover`, `resultType`, cache fields, `subscriptions/listen`, resource subscriptions, stateless Streamable HTTP, legacy initialize fallback). No later stable tag exists at the time of this gate.

## RED (v1.5.0)

Command:

```bash
go test ./server -run 'TestMCPSDKFeasibility' -count=1
```

Result: **build failed**. v1.5.0 has no `CacheScope` / `TTLMs` on `ListResourcesResult`, `ListResourceTemplatesResult`, or `ReadResourceResult`, and no `server/discover` types. This is the expected fail-closed proof that the previously pinned SDK cannot implement the plan.

## GREEN (v1.7.0)

Command:

```bash
go test ./server -run 'TestMCPSDKFeasibility|TestMCPWire_20260728' -count=1
```

Result: pass after:

- sending required 2026 headers (`Mcp-Method`, `Mcp-Name`) on raw JSON-RPC posts
- treating `tools.listChanged` omitempty-absence as false
- sending legacy `notifications/initialized` without a JSON-RPC id
- accepting HTTP 400 + JSON-RPC `-32602` for a malformed cursor
- accepting the SDK's HTTP 400 "Unsupported protocol version" for an unknown `Mcp-Protocol-Version`

## Capability notes (not blockers)

- Default `Cacheable.CacheScope` is `"public"`; private scope is caller-controlled via receiving middleware / result fields. Phase 4/5 catalog handlers must set `cacheScope: "private"` themselves.
- Dynamic `AddResource` / `RemoveResources` is reflected by the next `resources/list` on the same `*mcp.Server`.
- Opaque cursors are SDK-owned (base64 page tokens). Catalog list generation-binding in Phase 4 is application-level, not an SDK gap.
- `subscriptions/listen` and `ResourceUpdated` work on stateless Streamable HTTP when `SubscribeHandler` is set.

No unresolved SDK feasibility remains that would block Phase 2.
