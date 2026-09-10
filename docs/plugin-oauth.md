# Native OAuth for WASM integrations

Switchboard owns the authorization-code S256 PKCE exchange, user verification, token refresh, and credential persistence in-process. A browser is needed only for initial login or explicit reauthorization. There is no companion service, CDP connection, browser token extraction, client secret, or browser-based refresh.

## Configuration

OAuth settings live in the owning integration's `credentials` map, alongside the plugin's existing settings. For Primer:

```json
{
  "oauth_issuer": "https://clerk.primerlms.com",
  "oauth_client_id": "YOUR_REGISTERED_PUBLIC_CLIENT_ID",
  "oauth_scopes": "openid profile email offline_access",
  "oauth_token_key": "tasks_api_key",
  "oauth_subject": "EXPECTED_USER_SUBJECT",
  "oauth_email": "expected@example.com"
}
```

| Credential | Meaning |
| --- | --- |
| `oauth_issuer` | HTTPS authorization server issuer; no userinfo, query, or fragment in the URL |
| `oauth_client_id` | Registered **public** OAuth application's client ID |
| `oauth_scopes` | Space-separated scopes; defaults to `openid profile email offline_access` |
| `oauth_token_key` | Plugin credential receiving the access token, such as `tasks_api_key`; cannot start with `oauth_` |
| `oauth_subject` | Required expected userinfo `sub`; exact match |
| `oauth_email` | Required expected userinfo `email`; exact match |
| `oauth_access_token` | Host-managed access token |
| `oauth_refresh_token` | Host-managed refresh token, replaced when the issuer rotates it |
| `oauth_expires_at` | Host-managed access expiry in RFC3339 UTC |
| `oauth_settings_binding` | Host-managed fingerprint binding tokens to their OAuth settings |

Configure permits valid OAuth settings without tokens. Until authorization completes, Execute fails with an authorization-required result and Healthy returns false. Partial nonempty OAuth settings fail closed. Empty optional OAuth fields do not change ordinary credential handling.

The Loader injects ConfigService before Configure, including the startup path. Tokens are saved through that service into the integration's normal configuration entry. Enabled state, tool restrictions, identities, and unrelated credentials are preserved. Connecting **does not enable** a disabled integration.

All `oauth_*` values, including refresh tokens, are excluded from guest configuration. Only the access token is copied to `oauth_token_key`; other plugin credentials remain available. The guest then makes its ordinary HTTP requests. Access tokens may be opaque: Switchboard does not decode JWT claims, use an ID token as an access token, or synthesize a session ID.

## Public client registration and discovery

Register an application with authorization-code and refresh-token grants, public-client authentication (`none`), and S256 PKCE. For an integration named `primer` on port 3847, register this exact redirect:

```text
http://127.0.0.1:3847/api/integrations/primer/oauth/callback
```

Switchboard derives the redirect from its configured port, not request Host or forwarded headers. Change the registered redirect when changing ports or the integration's configured name.

Discovery begins at `<issuer>/.well-known/oauth-authorization-server`. If userinfo or JWKS metadata is absent, Switchboard also reads `<issuer>/.well-known/openid-configuration`, requiring matching issuer, authorization endpoint, and token endpoint. Clerk's production issuer above currently needs this OIDC fallback for userinfo. Authorization, token, userinfo, and JWKS URLs must be HTTPS and on the issuer's exact host/port. Provider redirects are refused, requests have a 15-second timeout, and response bodies are capped at 1 MiB.

Userinfo is called with each newly exchanged access token, including refreshes and tokens first loaded after restart. Both configured identity pins must match before a token is persisted or sent to the guest. JWKS is discovered and its URL validated, but no JWKS fetch or ID-token verification is needed because identity is checked directly at authenticated userinfo. Token responses must contain a Bearer access token and a positive integer `expires_in`. The initial authorization-code exchange must also return a nonempty refresh token; otherwise authorization fails. Later refresh responses may omit it and retain the current refresh token.

The configured issuer is a **trusted-operator-only setting**. HTTPS private-network and loopback issuers are supported for self-hosted deployments, not blocked as a class. Discovery and authenticated userinfo intentionally contact that issuer. Only a trusted operator may choose it or the plugin API destination; HTTPS and same-host endpoint checks are not a substitute for trusting those services.

## Initial browser setup

Use the local UI through **`http://127.0.0.1:<port>`**, not `localhost`, a proxy hostname, or a forwarded endpoint. OAuth endpoints require a loopback peer and the exact loopback Host; start and credential mutations additionally reject foreign Origin and non-same-origin browser fetch requests. The same guard applies to generic form saves and credential PUTs, even before OAuth settings are present. No CORS permission is granted.

If OAuth settings are already saved, the integration detail page offers **Connect OAuth**. It POSTs to start and navigates the same browser to the returned authorization URL.

For programmatic setup, open Switchboard in the intended browser and issue a same-origin request from that page:

```javascript
const response = await fetch('/api/integrations/primer/oauth/start', {
  method: 'POST',
  credentials: 'same-origin',
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify({credentials: {
    oauth_issuer: 'https://clerk.primerlms.com',
    oauth_client_id: 'YOUR_REGISTERED_PUBLIC_CLIENT_ID',
    oauth_scopes: 'openid profile email offline_access',
    oauth_token_key: 'tasks_api_key',
    oauth_subject: 'EXPECTED_USER_SUBJECT',
    oauth_email: 'expected@example.com'
  }})
});
if (!response.ok) throw new Error('OAuth start failed');
const {authorize_url} = await response.json();
window.location.assign(authorize_url);
```

An empty body or `{}` uses current saved credentials. Supplied credentials overlay the saved settings for this attempt only; they are not persisted until identity verification succeeds. The start API refuses supplied managed token fields, client secrets, unknown OAuth keys, non-JSON bodies, bodies larger than 64 KiB, and trailing JSON. Keep other required plugin settings in its existing configuration, or include them in the credentials overlay.

### API contract

- `POST /api/integrations/{name}/oauth/start`: returns only `{"authorize_url":"https://..."}` and sets a random, path-scoped HttpOnly SameSite=Lax cookie with a ten-minute lifetime. GET never starts authorization.
- `GET /api/integrations/{name}/oauth/callback`: consumes the matching one-shot state, cookie binding, and authorization code; checks an `iss` response parameter when present; exchanges with the native PKCE verifier; verifies userinfo; persists tokens; then redirects to `/integrations/{name}` without tokens or error details in the URL.
- Unsupported integrations return 404. Bad local-origin checks return 403. Invalid configuration, state, identity, provider, or persistence failures return generic errors without raw provider responses.

There is at most one pending login per module. Starting another replaces the previous attempt. State is integration-specific, expires after ten minutes, is not persisted, and is consumed before exchange. A callback must arrive in the browser that received the cookie; opening a URL returned to an unrelated command-line HTTP client will not work. Callback responses use no-store and no-referrer. The authorization protocol necessarily delivers the short-lived code and state in the incoming callback query; Switchboard does not echo or log that query, and the successful redirect strips it. Do not configure external request logging to capture callback query strings.

## Refresh and failure recovery

Before Execute or Healthy, the module serializes token acquisition and guest configuration with its guest-call lock. An access token with less than 60 seconds remaining is refreshed. A rotated refresh token is retained in memory immediately, checked through userinfo, and persisted before any guest call. There is no automatic retry of API calls on 401, including mutations.

If persistence fails, the operation reports an error and no guest call runs. Keep Switchboard running, repair storage, and retry Execute or Healthy: it retries the save with the retained rotated token, not the old refresh token. The same applies to initial callback save failures. Module close, reload, and uninstall first flush retained rotations and refuse replacement if verification or saving fails. Replacement reads fresh configuration only after the old module has finished. Do not restart the process until saving succeeds: unsaved token rotations cannot survive a process exit.

`invalid_grant` or an identity mismatch blocks the manager until explicit reauthorization. Raw token endpoint errors and storage errors are not exposed or logged. A transient userinfo failure quarantines the new token in memory for verification retry instead of replaying the previous refresh token. If that access token expires, Switchboard refreshes using the quarantined refresh token and still requires identity validation before publishing anything. **Connect OAuth intentionally abandons a quarantined rotation** after validating settings and completing discovery; use it only when you intend to replace that authorization. Abandoning the new browser login does not reactivate discarded tokens. A provider exchange interrupted before a complete response may already have rotated its token; if a subsequent attempt gets `invalid_grant`, reconnect.

The integration form hides host-managed tokens. Form saves and credential PUTs serialize with token refresh, ignore supplied managed tokens, and preserve the authoritative current tokens for unchanged settings. Changing issuer, client, scopes, identity pins, or token destination clears old tokens and both old/new guest token slots; reconnect afterwards. Persisted tokens carry a settings fingerprint checked on load, including after restart. Existing unbound configurations are accepted as trusted legacy input and acquire a binding on the next successful exchange. Do not manually transplant tokens or remove their binding.

Configuration saves create a same-directory mode-0600 temporary file, sync it, atomically rename it, and sync the directory. Existing symlink/nonregular config targets are rejected; use a regular file in an operator-controlled directory. A failure before rename leaves the old file intact. A directory-sync failure after rename reports failure although the new complete file may already be visible; keep the process running and retry to establish durability. Tokens remain sensitive native configuration data, not an encrypted vault. Protect the normal config file and backups. Only install trusted WASM plugins: the access token intentionally becomes available to the guest at its normal credential key.
