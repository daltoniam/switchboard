# Snipe-IT asset inventory

The native `snipeit` adapter targets Snipe-IT **v8.8.0**. It exposes three read-only hardware operations, not a general Snipe-IT API client. Other versions are unverified.

## Credentials and effective permissions

Create a dedicated, activated account outside Switchboard. Grant **assets.view** and **self.api** (for the operator to create its personal access token), without group-inherited admin or superuser permissions. Deny asset create/edit/delete, checkout/checkin/audit, files, licenses, user profiles and **assets.view.encrypted_custom_fields**. Personal access tokens inherit the account's permissions; the adapter cannot reduce a privileged token's authority.

When company isolation is required, enable Snipe-IT's full multiple-company support and assign the reader only to approved companies. Review the installation's null-company visibility setting and inherited group permissions. Verify effective restrictions against disposable records before connecting a production installation. Never grant administrative permissions to repair a failed health check.

The upstream token can still read fields and asset history permitted by `assets.view` directly through Snipe-IT, beyond the adapter's projection. Switchboard's output filter is not a token scope or a defense against a stolen token. Approve that residual read exposure before connecting.

On **Integrations > Snipeit**, enter:

| Credential | Environment variable | Value |
|---|---|---|
| `base_url` | `SNIPEIT_BASE_URL` | Trusted HTTPS installation URL; an installation subpath is preserved |
| `token` | `SNIPEIT_TOKEN` | Dedicated reader's personal access token |
| `ca_cert` | `SNIPEIT_CA_CERT` | Optional additional PEM CA certificate; literal `\n` is accepted |

Enable and save. Health checks request one hardware record with the same credential, so an empty but authorized register is healthy. TLS certificate and hostname verification remain enabled. HTTP, URL credentials, query strings, fragments, encoded prefixes, traversal prefixes and redirects are rejected. There is no insecure TLS option. The account, token and upstream permissions are provisioned externally; Switchboard never creates them.

## Tools

| Tool | Inputs | Result |
|---|---|---|
| `snipeit_list_assets` | Optional `search`, `status`, `limit`, `offset` | One page of visible hardware and `total` / `next_offset` |
| `snipeit_get_asset` | Exact positive numeric `id` from discovery | One hardware record |
| `snipeit_get_asset_by_tag` | Exact literal `tag` | Normalized `rows` candidates and `total` / `next_offset` |

Start with `snipeit_list_assets`, then inspect an ID or an exact tag. Search is Snipe-IT's upstream hardware search over tag, serial, name and any additional fields the server searches; it is not a local exact-match filter. The adapter never returns those additional fields. Default page size is 25; maximum 100. Pages use ascending numeric ID order. Offset is bounded to 1,000,000. Pass `next_offset` with the same filters; a null value means no further page was advertised. Inventory can change between requests, so offset pagination is not a snapshot. Empty/missing results can reflect permissions, not global absence.

Status values are `ready`, `pending`, `archived`, `undeployable`, `deployed` and `requestable`. They map to Snipe-IT's case-sensitive `RTD`, `Pending`, `Archived`, `Undeployable`, `Deployed` and `Requestable` selectors. In particular, `ready` means ready-to-deploy, excluding assigned assets.

Exact tags are 1..128 bytes, with no slash, backslash, percent, query, fragment, control characters or traversal segments. They are path-escaped as data. Single-object and collection responses normalize to candidates. At most 25 candidates are accepted; larger or incomplete collections fail rather than claiming a complete match. Each candidate is fetched again by ID before returning any metadata, enforcing the exact-record authorization boundary even if the upstream tag endpoint only checks list permission. Any denied or mismatched candidate fails the entire operation without partial results.

## Output and resource limits

Every read, including raw script output, is projected to:

- `id`, `name`, `asset_tag`, `serial`;
- `model`, `status_label` and `location`, each limited to `id` / `name`;
- `assigned_to`, limited to `type` / `id`.

No assignee name/email/profile, custom fields (encrypted or otherwise), notes, attachments, financial fields, purchase/license keys, executable content or history is returned. HTML entities in allowed text are decoded. Text exceeding 512 bytes or containing control characters fails closed rather than silently truncating an identifier. Unknown upstream fields are discarded before serialization, script access and compaction. Page compaction preserves the already-allowlisted `rows` array intact, retaining explicit null assignment/location references instead of dropping their absence.

Requests have a shared 15-second operation deadline and a 2 MiB decoded-response cap, including gzip responses. Tool output is capped at 256 KiB; reduce `limit` if it is exceeded. Calls do not retry automatically. HTTP 429 reports rate limiting and a valid numeric or HTTP-date `Retry-After` as a wait in seconds; wait before retrying and respect the installation's configured quota (v8.8.0's default is 120 API requests/minute, not a universal limit). Tag authorization reads consume additional requests. HTTP-200 error envelopes, malformed pages and non-success HTTP statuses fail without echoing upstream bodies, tokens or transport URLs.

## Acceptance

```sh
go test -race ./integrations/snipeit ./config ./server -run 'SnipeIT|TestConfigure|TestDispatchMap|TestAssets|TestInvalidArguments|TestUpstreamErrors|TestLifecycle|TestFieldCompaction|TestTagAuthorization|TestRedirect|TestTLS|TestDecoded|TestPageValidation|TestDefaultPage|TestRawProjection'
make test-live INTEGRATION=snipeit
make ci
```

`TestUISnipeIT` exercises a TLS fixture through the isolated real binary and Chromium: fresh fields, optional CA, invalid/valid/missing token, save/enable, health, restart persistence, MCP discovery and every safe read, disable, desktop/mobile evidence and browser console/network checks.

`web/live_snipeit_test.go` pins the vendor's `snipe/snipe-it:v8.8.0` and MariaDB `11.4.7` images by digest. It bootstraps a disposable installation, seeds inventory and a company-scoped `assets.view` / `self.api` reader through the product API, verifies denied writes, cross-company exact reads and an actually encrypted custom field (visible to the bootstrap admin but not the reader), then supplies only that reader PAT to Switchboard. A loopback TLS proxy fronts the real product; its CA is explicitly trusted, never skipped. Live cases cover search, pagination, ready/deployed filters, exact ID/tag, restart persistence, MCP execution and raw-script/compaction field parity. Evidence is written to `dist/live-artifacts/live-snipeit.json` and masked desktop/mobile screenshots.

Fixture success is not production authentication. The container run verifies the pinned product and disposable credential only; authentication to an operator's installation remains unverified until a dedicated authorized safe read succeeds there.
