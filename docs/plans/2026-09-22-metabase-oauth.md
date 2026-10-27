# Metabase OAuth (hosted MCP) with API-key fallback

## Problem

The `metabase` adapter only speaks Metabase's REST API with an `x-api-key`. Metabase now ships a
hosted MCP server at `<site>/api/metabase-mcp` behind its own embedded OAuth 2.1 server (the same
one the claude.ai Metabase connector uses). Users who cannot mint API keys, or whose admins prefer
per-user OAuth grants, cannot use the adapter today.

## Facts (verified against metabase/metabase master, 2026-09)

| Item | Value |
| --- | --- |
| MCP endpoint | `<site>/api/metabase-mcp` (legacy alias `/api/mcp`), Streamable HTTP |
| AS metadata | `<site>/.well-known/oauth-authorization-server` (RFC 8414) |
| Resource metadata | `<site>/.well-known/oauth-protected-resource/api/metabase-mcp` (RFC 9728) |
| Registration | RFC 7591 dynamic; `application_type` defaults to `native`, HTTP loopback (`localhost`, `127.0.0.1`, `[::1]`) redirects accepted; `token_endpoint_auth_method: none` |
| PKCE | S256 |
| Scopes | `agent:content:read agent:content:write agent:query:run agent:sql:run agent:delivery:write agent:resource:read`; baseline (auto-ticked) is read + query:run + resource:read, the rest are opt-in on the consent screen |
| Access token TTL | 3600 s |
| Refresh token TTL | 30 days, rotated on every refresh |
| Instance setting | `mcp-enabled?` (Admin > AI > MCP > "MCP server"), `:visibility :public` → readable unauthenticated at `GET /api/session/properties`; also gates dynamic registration. Disabled → MCP endpoint answers 403 "MCP server is not enabled." |

## Requirements

1. `metabase` stays one integration. Credentials: `url` (required in both modes), `api_key`
   (fallback), `mcp_access_token` + `mcp_refresh_token` + `mcp_client_id` (OAuth), `token_source`.
2. OAuth mode proxies the hosted MCP through `remotemcp`; tool names are `metabase_<remote tool>`.
   API-key mode keeps the existing REST tools untouched.
3. Mode selection: `token_source` decides when set (`oauth` | `api_key`); otherwise an OAuth token
   wins over an API key. Connecting via OAuth never deletes a saved API key (it is the fallback).
   Saving an API key never deletes OAuth tokens; it just sets `token_source=api_key`.
4. Expired access tokens are refreshed transparently (on 401, via the go-sdk `auth.OAuthHandler`
   hook) and the rotated refresh token is persisted through `mcp.ConfigService`.
5. Field compaction and max-bytes specs (written for REST shapes) are disabled in OAuth mode.
6. The setup page probes the Metabase instance's public `mcp-enabled?` setting and says whether
   "Sign in with Metabase" is available or the admin must enable the MCP server first.
7. Every new behavior has tests; `make ci` passes.

## Design

### `remotemcp`
- `Options{EndpointPath, OptionalToken, OnTokenRefresh}` + `NewWithOptions`. `New` and
  `NewOptionalToken` delegate. Default endpoint path stays `/mcp`.
- `remote` implements `auth.OAuthHandler`: `TokenSource` serves the current access token,
  `Authorize` refreshes on 401 when a refresh token is configured, else returns an
  authorization-required error. Replaces `bearerTransport`.
- `Configure` also reads `refresh_token`, `client_id`, `client_secret`.
- `HandleOAuthCallback` captures `refresh_token`; new `PollOAuthTokens` returns a `TokenSet`.
  `PollOAuth` is unchanged.

### `integrations/metabase`
- Dual mode modelled on `linear`, guarded by an `RWMutex` like `notionmcp`.
- `SetConfigService`, `MCPServerURL`, `IsRemoteMCP`, `MCPServerEnabled(ctx, client, url)`.
- "Start here" text injected on `metabase_search` in OAuth mode.

### `web`
- `remoteOAuthProfiles["metabase"]` with `ResourcePath: "/api/metabase-mcp"`, all six scopes,
  `PersistRefresh: true` (saves `mcp_refresh_token` + `mcp_client_id`).
- `/integrations/metabase/setup` page (URL + API key form, OAuth card with instance probe),
  `POST /api/metabase/save-credentials`. Detail page redirects to setup.

### Out of scope
- Using Metabase OAuth tokens against the REST API (`mb:full` scope is deliberately not advertised
  by Metabase). Env-var injection of OAuth tokens. RFC 9728 discovery fallback in `remotemcp`.
