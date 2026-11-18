# Gatus endpoint health inspection

The Gatus adapter supports the released v5.37.0 HTTP API with three read-only tools. It does not configure monitors, execute probes, publish external results, read configuration, inspect suites, or change announcements.

## Supported deployment and credentials

Use one administrator-approved HTTPS Gatus installation with **built-in Basic authentication and OIDC disabled**. OIDC takes precedence over Basic when both are configured. Browser cookies, bearer tokens, anonymous access, arbitrary gateway headers and insecure TLS are not supported. A reverse-proxy subpath is preserved, but redirects are rejected, including same-origin redirects.

Configure the disabled-by-default integration through **Integrations > Gatus**:

| Credential | Environment override | Requirement |
|---|---|---|
| `base_url` | `GATUS_BASE_URL` | Trusted HTTPS installation URL, optional subpath, no URL credentials/query/fragment/encoded paths |
| `username` | `GATUS_USERNAME` | Built-in Basic username, no colon or control characters |
| `password` | `GATUS_PASSWORD` | Complete Basic password, no control characters |
| `ca_cert` | `GATUS_CA_CERT` | Optional PEM CA certificate; literal `\n` is accepted, certificate verification remains enabled |

Basic authentication exposes installation-wide protected status information, not per-endpoint ACLs. Use a dedicated status credential where the deployment supports it and explicitly accept installation-wide topology visibility. Never supply an external-result bearer token: it is a separate mutation credential and is not needed by this adapter. Basic credentials cannot submit external results on the released product.

**Raw uptime routes are public in Gatus v5.37.0 even when Basic authentication is enabled.** Confidential uptime requires a network or whole-site access boundary. Configuring this adapter does not secure the Gatus server. Health verification reads the protected status inventory; public uptime alone is never treated as proof of working authentication. An OIDC redirect/HTML page or HTTP 401/403 fails verification.

## Tools and response contract

| Tool | Inputs | Output |
|---|---|---|
| `gatus_list_endpoints` | Optional exact `group`, case-insensitive `name` substring, boolean `only_unhealthy`, local `limit` 1..50 (default 20) | Endpoint key/name/group and latest observed timestamp/success/status/duration_ms; null latest means unknown. Includes total_fetched, matching_count, returned_count and truncated |
| `gatus_get_endpoint` | Exact discovered `key`, history `page` 1..1000000 (default 1), `page_size` 1..20 (default 10) | Endpoint identity and ordered probe timestamp/success/status/duration_ms, with requested history page/size |
| `gatus_get_uptime` | Discovered `key`, `duration` 1h/24h/7d/30d (default 24h) | Finite ratio between 0 and 1 and the requested duration, never a percentage |

Start with list, then pass an opaque key from its output to history or uptime. Keys are bounded to 256 bytes and reject controls, traversal, path separators, percent encodings, query/fragment delimiters and plus signs (the upstream handler query-decodes path keys). HTTP 404 is an explicit error. Monitored URLs are never followed by the adapter.

The inventory request uses `page=1&pageSize=1`. These parameters paginate **probe results within each endpoint**, not endpoint inventory. Every endpoint is still fetched upstream; filters and limit are local. The adapter selects the latest timestamp in each returned page rather than assuming array order, and sorts history newest-first within the requested page. `only_unhealthy` excludes unknown endpoints instead of inventing health or an outage.

Gatus caches inventory for ten seconds; experimental remote aggregation can silently omit failed remote instances. Counts describe only the fetched response, not guaranteed global completeness. History pages do not imply a stable snapshot or a complete history. Uptime zero can mean no observations; the upstream `1h` calculation actually starts two hours back because storage is hourly.

## Bounds and privacy

Each tool makes one logical GET request, without fan-out or polling. HTTP 429 and transient 500/502/503/504 responses allow at most two retries, honoring supported Retry-After seconds/dates inside a ten-second operation budget. There is no documented universal Gatus quota. Non-transient errors, redirects and transport failures are not retried automatically.

Responses are capped at 2 MiB **after decompression**. Decoding caps inventory at 2,000 endpoint objects and each endpoint at 20 probe results; oversized or malformed responses fail explicitly. Identity strings are bounded to 256 UTF-8 bytes and controls are rejected rather than silently truncating identifiers. Durations are converted from upstream nanoseconds to milliseconds, preserving fractional milliseconds. Missing required probe timestamps/success/duration fail rather than presenting an invented failed observation.

Small typed allowlists are applied before returning either normal MCP or raw/script results. Hostnames, probe errors, condition expressions/results, events, monitored URLs and unknown upstream fields never enter the response. Compaction preserves the same bounded projection and its caveats. Provider errors and credentials are not echoed in failures.

## Verification and development commands

```bash
go test -race -cover ./integrations/gatus
go test -v ./server -run 'TestGatusSearchDiscoverability|TestSearchBenchmark|TestSmoke_SearchResponseShape'
make test-live INTEGRATION=gatus
make ci
```

`TestUIGatus` drives the real development binary with Go Playwright/Chromium in an isolated HOME/config directory against an HTTPS fixture. It checks fresh fields, text/password types and placeholders, invalid/missing/valid passwords, save/enable, health, restart persistence, discovery followed by all three reads, and disable. Desktop/mobile screenshots mask all inputs; console and unexpected HTTP errors fail the test.

`web/live_gatus_test.go` pins the official `twinproduction/gatus:v5.37.0` image by digest. Gatus has no monitor-creation API in this slice: the disposable provider starts with explicit monitor configuration, then seeds alternating success/failure and duration/error data through its own external-result API with a **bootstrap-only bearer token**. A real loopback HTTP monitor supplies a successful HTTP status. Both SQLite and memory storage are checked. Switchboard receives only Basic status credentials and a CA for an isolated TLS frontend, never the seeding token. Negative checks prove protected status rejects anonymous access and Basic cannot publish external results; a public-uptime check verifies the documented deployment limitation.

The live harness enters credentials through the setup UI, restarts Switchboard, calls every tool via MCP search/execute, replays reads through raw scripts, and checks every compaction field against real responses. Evidence is written to `dist/live-artifacts/live-gatus.json` and masked desktop/mobile screenshots. This proves the pinned local product contract and Basic authentication, not production vendor credentials or arbitrary future versions.
