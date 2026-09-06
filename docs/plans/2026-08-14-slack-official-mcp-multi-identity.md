# Slack Official MCP Multi-Identity Integration Plan

> **For Hermes:** Execute this plan with strict RED→GREEN TDD and review the completed diff for both spec compliance and code quality.

**Goal:** Add a separate `slackmcp` integration that proxies Slack's hosted official MCP server and routes every upstream tool call through an explicitly selected configured identity.

**Architecture:** Keep the existing `slack` session-token adapter unchanged. Extend `IntegrationConfig` with generic named identities (`credentials` plus non-secret `metadata`) and add an optional multi-identity configuration hook so existing adapters remain source-compatible. The new adapter maintains one authenticated remote MCP client/session per identity, unions scope-dependent upstream tool catalogs, injects a required `identity_id` into every proxied tool schema, and exposes one identity discovery tool.

**Important authentication correction:** Slack's hosted official MCP endpoint is `https://mcp.slack.com/mcp`. Slack documents confidential OAuth and a **user OAuth access token** for MCP calls; an app/bot may host the AI agent, but an `xoxb-` bot token does not authenticate the hosted MCP endpoint. Configure each Switchboard identity with the resulting user `access_token` (typically `xoxp-...`).

**Tech stack:** Go 1.26, `github.com/modelcontextprotocol/go-sdk/mcp`, Switchboard `remotemcp`, JSON config, `testify`.

---

### Task 1: Add generic multi-identity configuration support

**Objective:** Persist named identity credentials without changing the existing `Integration.Configure` contract for single-identity adapters.

**Files:**
- Modify: `mcp.go`
- Modify: `mcp_test.go`
- Modify: `config/config.go`
- Modify: `config/config_test.go`
- Modify: `server/server.go`
- Modify: `server/server_test.go`
- Modify: `web/health_cache.go`
- Modify: `web/health_cache_test.go`
- Modify: `web/web.go`
- Modify: `web/web_test.go`

**Data model:**
- Add `IntegrationIdentity` with `Credentials Credentials` and optional `Metadata map[string]string`.
- Add `Identities map[string]IntegrationIdentity` to `IntegrationConfig` with `json:"identities,omitempty"`.
- Add optional `MultiIdentityIntegration` with `ConfigureIdentities(context.Context, map[string]IntegrationIdentity) error`.
- Add a shared configuration helper that first calls normal `Configure` with integration-level credentials, then calls `ConfigureIdentities` when implemented.

**TDD acceptance cases:**
1. JSON round-trip preserves multiple identities, their credentials, and metadata.
2. Merging a user config with defaults preserves identities.
3. Generic web saves and credential hot reloads preserve existing identities.
4. Startup and health refresh use the shared helper and can configure an adapter whose only usable credentials are in `Identities`.
5. Disabled integrations with configured identities are eligible for startup auto-configuration, while existing single-identity behavior is unchanged.

**RED command:** `go test ./... -run 'Test.*(Identity|Identities|ConfigureIntegration)'`

**GREEN command:** `go test ./... -run 'Test.*(Identity|Identities|ConfigureIntegration)'`

---

### Task 2: Implement the Slack hosted MCP identity multiplexer

**Objective:** Add an adapter that dynamically proxies Slack's official MCP tools through a caller-selected identity.

**Files:**
- Create: `integrations/slackmcp/slackmcp.go`
- Create: `integrations/slackmcp/slackmcp_test.go`

**Required behavior:**
- Integration name: `slackmcp`.
- Production endpoint base: `https://mcp.slack.com` (the shared remote client appends `/mcp`). Allow an integration-level `base_url` override for tests/proxies.
- Identity credential key: `access_token`. Reject empty identity IDs and identities without tokens. Never return or log token values.
- Preserve/reuse per-identity remote clients when possible; reset sessions when tokens change.
- Build a deterministic union of tools returned by all configured identities because Slack's catalog is scope-dependent.
- Translate upstream names from `slack_*` to Switchboard names `slackmcp_*` (avoid `slackmcp_slack_*`).
- Inject `identity_id` into every proxied tool's parameter map and required list, without forwarding that synthetic argument upstream.
- Expose exactly one non-proxied discovery tool named `slackmcp_list_available_identites` (retain the requested spelling). Its description must include **Start here** and its result may expose only identity ID, metadata, and non-secret capability/tool information.
- All tools except `slackmcp_list_available_identites` must fail clearly when `identity_id` is missing or unknown.
- Unknown tool and upstream errors return normal Switchboard error results.
- Healthy if at least one configured identity can list tools.
- Implement `PlainTextCredentials` for `base_url` and `OptionalCredentials` because no integration-level secret is required.

**TDD acceptance cases (use an `httptest` Streamable HTTP MCP server):**
1. Two identities receive distinct bearer tokens on proxied calls.
2. The union includes tools available to only one identity.
3. Every dynamic schema requires `identity_id`; the identity-list tool does not.
4. `identity_id` is removed before the upstream `tools/call` request.
5. Missing/unknown identity and missing token produce deterministic safe errors.
6. Identity listing returns metadata but never access tokens.
7. Reconfiguration removes stale identities and updates changed tokens.
8. Tool names are translated exactly once.

**RED command:** `go test ./integrations/slackmcp -count=1`

**GREEN command:** `go test ./integrations/slackmcp -count=1`

---

### Task 3: Wire configuration, registration, and documentation

**Objective:** Make `slackmcp` available from a fresh Switchboard install and document correct identity setup.

**Files:**
- Modify: `config/config.go`
- Modify: `config/config_test.go`
- Modify: `cmd/server/main.go`
- Modify: `README.md`
- Modify if needed: `docs/architecture.md`
- Modify if needed: `docs/adapter-reference.md`

**Required behavior:**
- Add disabled default config for `slackmcp` with optional `base_url` and an empty identities map.
- Register `slackmcp.New()` separately from the existing `slack` adapter.
- Update exact default-integration count/name assertions.
- Document a JSON example with at least two identity IDs and non-secret labels/workspace metadata.
- Explicitly document that the hosted MCP bearer token is a Slack user OAuth token; `xoxb-` bot tokens are not accepted by Slack's hosted MCP server.
- Document the spelling and role of `slackmcp_list_available_identites`, and that all other `slackmcp_*` calls require `identity_id`.

**RED command:** `go test ./config ./cmd/server ./... -run 'Test.*slackmcp'`

**GREEN command:** `go test ./config ./cmd/server ./... -run 'Test.*slackmcp'`

---

### Task 4: Format and run all repository gates

**Objective:** Verify the implementation against project-wide quality, race, lint, and security gates.

**Steps:**
1. Run `make fmt`.
2. Run targeted tests: `go test ./integrations/slackmcp ./config ./server ./web -count=1`.
3. Run `make ci`.
4. Inspect `git diff --check`, `git status --short`, and the final diff for accidental secret values or unrelated changes.
5. Do not commit or push unless explicitly requested.

**Acceptance:** All commands pass; no secrets, generated junk, or unrelated modifications are present.
