# OpenBao metadata diagnostics

Switchboard supports OpenBao **2.7.2**, tested against the official container image pinned in `web/live_openbao_test.go`. The integration exposes exactly three read tools:

| Tool | Purpose |
|---|---|
| `openbao_list_secret_keys` | Discover key names and folders inside one configured KV v2 prefix, one bounded page at a time. |
| `openbao_get_secret_metadata` | Inspect one key's version numbers, creation/update timestamps, and per-version deletion/destroyed state. |
| `openbao_get_health` | Inspect initialized, sealed and standby states, plus server version. |

There is no secret-value read, write, recursive scan, detailed-metadata list, token administration, policy management or arbitrary HTTP tool. Unexpected upstream fields, `custom_metadata`, annotations, warnings, cluster names and upstream error bodies are excluded before either normal MCP or raw script output.

## Credential setup

Open **Integrations → Openbao** in a fresh Switchboard Web UI. The generic integration configuration page provides these fields; no separate OAuth setup is needed:

| Field | Environment variable | Requirement |
|---|---|---|
| `base_url` | `OPENBAO_BASE_URL` | Trusted HTTPS server URL, optionally including an installation prefix. No userinfo, query, fragment, encoded path or traversal. |
| `token` | `OPENBAO_TOKEN` | Dedicated, short-lived operator-issued token with the policy below. Never use a root token. |
| `mount` | `OPENBAO_MOUNT` | One literal KV v2 mount name, for example `secret`. |
| `path_prefix` | `OPENBAO_PATH_PREFIX` | Required narrow relative prefix, for example `approved/app`. Do not use the whole mount. |
| `ca_cert` | `OPENBAO_CA_CERT` | Optional trusted PEM CA certificate for a private HTTPS server. For the single-line UI field, replace newlines with literal `\n`. Actual newlines also work in configuration/environment values. |

TLS certificate and hostname verification are mandatory. The adapter does not follow redirects, including same-origin redirects: configure the final trusted endpoint directly. Tokens are sent only in `X-Vault-Token`, not in URLs. The unauthenticated health read does not send the token.

The connection badge validates the configured mount as KV v2 using OpenBao's single-path `sys/internal/ui/mounts/<mount>/metadata/<prefix>/` preflight, then performs an authenticated bounded LIST under that exact prefix. Do not grant broad mount-listing or administration privileges for this preflight. OpenBao grants it based on the token's existing path capabilities; the internal endpoint has no vendor backwards-compatibility guarantee, so failures are closed rather than bypassed. `get_health` alone cannot prove a token is valid.

## Least-privilege policy

For mount `secret` and prefix `approved/app`, an operator can create this policy:

```hcl
path "secret/metadata/approved/app/*" {
  capabilities = ["read", "list"]
}
path "secret/data/*" {
  capabilities = ["deny"]
}
path "secret/detailed-metadata/*" {
  capabilities = ["deny"]
}
```

Issue a short-lived, nonrenewable token without the default policy and with only this policy. Review the union of all attached policies and identity/role grants: the adapter cannot certify effective upstream permissions and does not inspect or create tokens. Do not grant `scan`, `create`, `update`, `delete`, `patch`, `sudo`, administration, or unrelated-prefix access. Do not attach a policy allowing secret-value reads. Renew/reissue outside Switchboard when the token expires.

**Key names themselves are sensitive.** OpenBao does not policy-filter individual names returned by LIST. The entire configured prefix and every folder the operator permits beneath it must be acceptable for the model to see. If that exposure is unacceptable, do not enable the integration. Prefix restriction inside Switchboard is not a substitute for the upstream policy. Metadata-only tokens can still read arbitrary `custom_metadata` directly upstream; Switchboard deliberately does not return it.

## Bounded read behavior

- Paths are relative to the configured prefix. Traversal, absolute paths, percent escapes, control/whitespace characters and wildcard paths are rejected before any request.
- LIST defaults to 20 names and accepts integer limits from 1 through 100. It requests one additional entry to detect another page. Use `next_after` with the same folder; the final page returns `has_more: false` and `next_after: ""`. No automatic pagination or recursion occurs.
- Each operation has a ten-second deadline and a one-MiB decompressed response ceiling. Metadata responses with more than 1,000 retained versions fail explicitly instead of silently dropping history.
- Metadata versions are sorted numerically. Missing/null timestamps and flags stay absent, never invented as successful states.
- Upstream 401/403/404 remain errors, not empty listings. Health HTTP 429/501/503 are diagnostic states, not rate-limit retries. Other 429/5xx responses use Switchboard's shared retryable-error handling with `Retry-After`; the adapter does not add a second retry loop.
- OpenBao has no verified universal rate limit. Keep requests bounded and do not use this integration for bulk inventory or scans.

## Verification

```sh
go test -race -cover ./integrations/openbao
make test-live INTEGRATION=openbao
make ci
```

The live spec starts the real OpenBao 2.7.2 TLS server from a digest-pinned official image, seeds disposable secrets and metadata through the product CLI/API, issues a non-root metadata-only token, and hands that token to the Web UI. It verifies allowed metadata reads and denied secret reads, out-of-prefix reads, detailed listing, value/metadata writes and deletion. Both raw and compacted results exclude seeded secret sentinels. Every tool is discovered/executed over MCP; all six cases are replayed as scripts and compaction fields are checked against actual responses. Browser tests additionally cover fresh fields, missing/wrong tokens, health, persistence, MCP discovery/read and disable.

Masked desktop/mobile screenshots and `live-openbao.json` are saved under `dist/live-artifacts/`; integration-specific UI evidence is under `dist/ui-artifacts/`. These are disposable-container authentication results, not production credential certification.

API references: [KV v2](https://openbao.org/docs/api/secret/kv/kv-v2/), [policies](https://openbao.org/docs/concepts/policies/), [health](https://openbao.org/docs/api/system/health/).

Raw HTTP is intentional: the official Go client's listing helper supports pagination but normalizes some 404s to empty results and parses general secret envelopes. Three fixed route shapes need neither that semantic change nor a broad SDK dependency; bounded HTTP and allowlisted response types keep the secret-value boundary explicit.
