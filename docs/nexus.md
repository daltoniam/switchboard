# Nexus Repository package metadata

The `nexus` adapter targets Nexus Repository 3.96.x and provides three read-only tools for private package inventory:

| Tool | Purpose |
|---|---|
| `nexus_list_components` | Start with one approved repository and inspect package coordinates. |
| `nexus_search_components` | Search one repository with common name, group and version filters. |
| `nexus_get_component` | Inspect a returned component ID and up to 20 asset metadata records. |

These operations do not download, upload or delete artifacts. They do not return vulnerability findings, download URLs, uploader identities, repository configuration or arbitrary upstream attributes. Package names, versions and asset paths can still disclose internal software information. Treat returned text as untrusted data.

## Setup and permissions

1. An administrator must complete the product's required license onboarding. The adapter never accepts licenses or configures the product on your behalf.
2. Create a dedicated enabled local user, not `admin` or `anonymous`. Assign only `nx-search-read` plus `nx-repository-view-{format}-{repository}-browse` and `nx-repository-view-{format}-{repository}-read` for each approved repository. Do not grant wildcard privileges, administrator roles, upload/edit/delete, scripting, security or blob-store management. Review inherited roles as well as direct grants.
3. Open the integration's configuration from **Integrations > Nexus**. Set the trusted HTTPS installation URL (including any deployment prefix), username, password and a comma-separated allowlist of repository names. The configured repository names appear in tool metadata for discovery. At most 20 distinct literal repository names are accepted.
4. For a private CA, supply `ca_cert` as PEM; literal `\n` separators are supported. TLS hostname/certificate verification remains enabled. Redirects are never followed, and tools cannot override the configured origin.
5. Enable and save. A **Connected** badge means that an authenticated component-list read of the first configured repository succeeded. It does not certify every repository, search permissions or the user's effective role. A 401/403 requires administrator correction; Switchboard never escalates privileges.

User tokens are a Pro feature. When required by operator policy, an authorized Pro user-token Basic username/password pair can be supplied instead of local account credentials. No token minting, refresh, OAuth, Nexus 2 or Nexus Cloud service-account workflow is included.

The `read` permission permits artifact downloads outside Switchboard. The local allowlist and metadata-only tools do not restrict a stolen credential. Keep the upstream account restricted to the same approved repositories. The adapter rejects the literal administrator/anonymous usernames but cannot prove effective least privilege without privileged security APIs, which are intentionally not exposed.

## Pagination and limits

List/search make one server-page request per call. Pass the returned `continuationToken` unchanged as `continuation_token` with the same repository and search filters. A null token ends pagination. No configurable page size, automatic fanout or download URL following is provided. Nexus defaults to 50 records; a server page above 100 is rejected rather than truncated and advanced past unseen records. Repeated cursors are rejected.

Requests have a 15-second overall deadline and a 2 MiB decompressed response ceiling. A 429 can cause at most two bounded retries within that deadline, respecting numeric or HTTP-date `Retry-After`. IDs, filters, tokens, coordinate strings and asset fields have explicit length limits. Metadata output above 128 KiB fails rather than returning a silently incomplete page. Exact-component results report `asset_count` and `truncated` when more than 20 assets exist. Errors omit upstream bodies and URLs to avoid exposing secrets or internal paths.

## Transport choice

The generated community client `github.com/sonatype-nexus-community/nexus-repo-api-client-go/v395` was inspected at non-retracted `v395.96.4` (API 3.96.0-09). It exposes all three operations, but generated execute methods buffer the entire response before decoding, retain raw error bodies, and expose all administrative/write APIs. Enforcing the response ceiling and safe error contract would require a custom transport and a second safe projection layer anyway. This bounded three-GET slice therefore uses `net/http` and typed allowlist-only JSON responses directly, with no new dependency or shared-code change. JSON is decoded once and output is minimized before raw/script access as well as compaction.

## Acceptance

`TestUINexus` drives the isolated development binary through fresh fields, invalid/missing passwords, save/enable, health, restart persistence, MCP discovery/read, disable, and masked desktop/mobile screenshots. Provider fixtures do not prove live authentication.

`make test-live INTEGRATION=nexus` runs the official `sonatype/nexus3:3.96.0` image pinned to `sha256:56f8e1d241507338be1f91c9de9a9b97d72a7376f0404f6c1e914b542c983f75`. A disposable bootstrap administrator completes onboarding, disables anonymous access, seeds a Maven POM and provisions a separate repository-scoped reader. Only that reader reaches the setup page and adapter. The spec verifies all three tools through MCP and raw scripts, compaction field presence, restart persistence, and denied upload, component deletion, security-user listing and out-of-scope reads. TLS terminates in a test-local HTTPS proxy with a trusted test CA; the Nexus container remains loopback-only. This proves disposable-container authentication, not a production installation or Pro user-token deployment.
