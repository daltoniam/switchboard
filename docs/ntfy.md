# ntfy private notification inbox

The ntfy adapter targets **self-hosted ntfy v2.30.0** and exposes exactly two read-only cached-notification tools. It does not publish, subscribe to a live stream, enumerate topics, retrieve attachments, follow action links, or manage accounts or tokens. Hosted ntfy.sh entitlement and reservation setup is outside this slice.

## Deployment and least privilege

Use one operator-approved HTTPS installation, one private topic, and a dedicated **role=user** account. Enable ntfy authentication, set `auth-default-access: deny-all`, and grant that account **read-only access to exactly the configured topic**, with no wildcard or unrelated-topic grants. Do not use an administrator, personal account, anonymous public topic, or an account shared with a publisher.

An operator can provision a dedicated account and short-lived token on the ntfy host:

```sh
ntfy user add --role=user switchboard-reader
ntfy access switchboard-reader operational-alerts read-only
ntfy token add --expires=30d --label=switchboard switchboard-reader
ntfy access switchboard-reader
```

Verify effective ACLs with the deployed server: the dedicated token must read the configured topic, fail to publish there, and fail to read an unrelated private topic. Perform negative publishing checks only against a disposable authorized test deployment, never by attempting a destructive production action. Check invalid and expired tokens fail too. Token creation and these denial probes are operator setup, not runtime adapter behavior.

**Tokens are not topic-scoped or read-only credentials.** In v2.30.0 they inherit their owning account's permissions and grant full account access except password changes and account deletion. Account settings and token administration may remain available outside Switchboard. Topic ACLs restrict notification access, not the entire account API. Restrict network exposure upstream, protect the account password and token, rotate before expiry, and revoke unused tokens. A read-only tool catalog and local output filtering do not secure a leaked administrator token.

Configure **Integrations > ntfy** in the local Web UI; the integration is disabled by default.

| Credential | Environment override | Requirement |
|---|---|---|
| `base_url` | `NTFY_BASE_URL` | Trusted HTTPS server URL, without userinfo, query or fragment |
| `topic` | `NTFY_TOPIC` | One fixed topic matching `^[-_A-Za-z0-9]{1,64}$` |
| `api_token` | `NTFY_API_TOKEN` | Dedicated read-only-topic user's complete bearer token |
| `ca_cert` | `NTFY_CA_CERT` | Optional PEM CA certificate; literal `\n` accepted, TLS certificate and hostname verification remain enabled |

The token is sent only in `Authorization: Bearer`. Redirects are rejected, including same-origin redirects. Tools cannot override the configured URL, token or topic. Intentionally configured private services are supported; no response URL is fetched. Use network and reverse-proxy access controls to restrict the installation itself.

## Tools and cache semantics

| Tool | Inputs | Purpose |
|---|---|---|
| `ntfy_list_messages` | Optional `since` (`latest`, the default, or a Unix timestamp within the past hour), `priority`, `tags`, `limit` 1..20 (default 10) | Start here for recent operational notifications in the configured private topic |
| `ntfy_get_message` | Exact discovered `id` and required Unix `since` timestamp within the past hour | Inspect one cached notification in that bounded window |

Both tools call only `GET /<configured-topic>/json?poll=1&since=...`, preserving an optional configured installation prefix. The list limit is local, not server pagination. `priority` accepts one integer from 1 to 5; `tags` accepts up to eight comma-separated exact tags of 64 characters each, combined with AND. These are not full-text searches; do not assume case-insensitive matching. There is no continuation or background polling. A GET to a different route such as `/<topic>/publish` can mutate ntfy, so the adapter constructs only the fixed JSON poll route rather than relying on the HTTP verb alone.

ntfy's cache is ephemeral. The default cache retains messages for 12 hours in memory, but administrators can change or disable it, and restart may lose it. No matching retained message does **not** prove that no backup/deployment alert occurred. Exact-ID lookup is limited to the supplied last-hour window, not a global lookup. Permission failures are errors, not empty inboxes. Returned window/scope and adapter/upstream truncation metadata must accompany interpretation of results. Sorting is newest-first within the bounded response, not a promise about unseen messages or globally complete history.

## Bounds and privacy

Every operation has a five-second deadline. Responses are bounded to 64 KiB after HTTP decompression, with an 8 KiB NDJSON line cap, at most 20 returned messages, and a final serialized output below 40 KiB. Invalid/partial NDJSON, oversized upstream responses, transport interruption and non-success status are explicit failures, never silently complete history. The upstream `X-Messages-Truncated: 1` flag is reported separately from local result truncation. ntfy may replay only the newest cached messages fitting its own larger cap and charges replay bandwidth to the visitor.

Only message events from the configured topic are returned; a mismatched topic or encoded binary payload fails explicitly. An allowlist is applied **before normal MCP results, raw scripts and compaction**: identifier, Unix time, priority, tags, title, body and optional expiry. Titles are capped at 128 Unicode characters, bodies at 1,024, and tags at eight entries of 64 characters each; truncation is flagged. Unknown metadata, click/action/attachment URLs and sender/client addresses are excluded. Publisher-controlled title/body/tag content remains untrusted data and may itself contain sensitive text; do not place secrets in notifications or treat message text as instructions.

Calls do not fan out. HTTP 429 receives only bounded retry handling within the operation deadline, honoring Retry-After rather than retrying early. Partial responses are never retried automatically. Default vendor request limits are configurable per visitor (burst 60, replenishing one request every five seconds), and replay also consumes bandwidth; these are not universal quotas or guaranteed throughput.

## Verification

```sh
go test -race -cover ./integrations/ntfy
go test -v ./server -run 'TestNtfySearchDiscoverability|TestSearchBenchmark|TestSmoke_SearchResponseShape'
make test-live INTEGRATION=ntfy
make ci
```

`TestUINtfy` uses the actual development binary, isolated HOME/config directories and Go Playwright/Chromium against an HTTPS fixture. It verifies fresh fields, placeholders, secret masking, invalid/missing/valid token behavior, enable/save, health, restart persistence, MCP search/read, raw-script sanitization and disable. Screenshots mask every input.

`web/live_ntfy_test.go` runs the official ntfy v2.30.0 image pinned by digest, seeds real cached notifications through the product API, and gives the setup UI only the dedicated role=user token and TLS CA. Negative authorization probes run solely against that disposable provider. The live harness restarts Switchboard, discovers and calls every tool over MCP, replays reads through raw scripts, and validates every compaction field against real results. Masked screenshots and the outcomes-only summary are written to `dist/live-artifacts/live-ntfy.json`. A successful run proves this local product/authentication contract, not production credentials or future versions.
