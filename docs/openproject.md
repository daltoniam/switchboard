# OpenProject work-package discovery and inspection

The OpenProject integration provides exactly two read-only REST tools for one configured project: list/search work packages, then read an exact work package. It targets the released **OpenProject v17.9.2** API contract, including numeric workspace routing and semantic display IDs. Earlier releases, other workspace kinds and arbitrary future versions are not certified.

OpenProject has an official MCP server, including work-package reads and writes. In the targeted release, that server requires the enabled MCP configuration and its Enterprise entitlement. This adapter serves existing Community installations that can use the documented REST API but cannot use that vendor MCP deployment path. It does not bypass MCP entitlements or unlock Enterprise-only REST features. Operators remain responsible for their installation's license and terms.

## Least-privilege account and API token

1. Have an administrator enable the REST API and API tokens under **Administration > API and webhooks**.
2. Create a dedicated **non-administrator** account. In the intended project, grant **View work packages** and the project visibility required for that read. Do not grant add/edit/delete/move work packages, comments, attachments, time logging, exports, project administration or global administrative capabilities.
3. Audit the account's **effective permissions**, not merely a role called “read-only.” Grants are additive: inspect every direct project role, inherited/group membership, global role, the instance-wide **Standard** role and **Non-member** access to public projects. A restrictive role does not revoke permissions granted elsewhere. Public and descendant projects need explicit visibility review.
4. Sign in as that account and open **Account settings > Access tokens > API > + API Token**. Create a token dedicated to Switchboard and securely retain the value shown once. Rotate or revoke it there when no longer needed.
5. On an authorized **disposable test installation**, verify that the exact token reads the approved project and work packages, cannot read an unauthorized private project or its work packages, and cannot create, edit, delete, comment, log time, export or perform administrative actions. Verify descendant-project visibility separately. Mutation-denial probes are operator/product acceptance work; never run them against production data.

The token inherits the account's permissions; it is not intrinsically read-only or limited to a workspace. The configured workspace, local cross-project filtering and two-tool catalog are **not upstream authorization boundaries**. A leaked token may have wider capabilities if the account's grants are wider. Fixture-backed tests do not certify an operator's real account restrictions.

Authentication uses HTTP Basic with the literal username **`apikey`** and the API token as the password. Do not enter a human login/password. There is no Bearer fallback, OAuth flow, browser-session support, token creation or entitlement bypass in this adapter.

## Switchboard setup

Open **Integrations > openproject**. This uses the standard integration detail form, not a separate setup or sign-in page. Fill the fields, enable the integration and select **Save Configuration**. Fresh installations show all four fields and leave the integration disabled until enabled.

| Credential | Environment override | Requirement |
|---|---|---|
| `base_url` | `OPENPROJECT_BASE_URL` | Required trusted HTTPS installation URL, including any deployment subpath; for example `https://projects.example.com/openproject` |
| `api_token` | `OPENPROJECT_API_TOKEN` | Required API token for the dedicated least-privilege account; entered as a password field |
| `workspace_id` | `OPENPROJECT_WORKSPACE_ID` | Required positive numeric ID of the selected **project** workspace, not a project slug, display ID or URL |
| `ca_cert` | `OPENPROJECT_CA_CERT` | Optional PEM CA certificate(s); literal `\n` separators are accepted |

Use an administrator-verified numeric project ID, such as `7`, not a work-package ID. Tools cannot change this configured project. Semantic work-package display IDs such as `DEMO-42` are returned for recognition, but exact reads use the positive numeric `id` discovered in the list.

HTTPS is mandatory. Certificate and hostname verification remain enabled with custom CAs; there is no insecure mode. Supply the private CA chain in `ca_cert` when the installation is not trusted by the system trust store. Set `base_url` to the installation root, **not** `/api/v3` or `/mcp`. The adapter retains its subpath when constructing requests. URL credentials, query strings, fragments, encoded paths, traversal and malformed path segments are rejected. All redirects, including same-origin redirects, are rejected: configure the final HTTPS URL instead.

A **Connected** badge means a bounded authenticated list read of the configured workspace succeeded. It does not prove denied-write permissions, exclusion from other upstream projects, or exhaustive compatibility. **Not Verified** can indicate missing/invalid credentials, denied project visibility, an untrusted certificate, an unsupported response or a failed request. The integrations list's **Re-check all** action refreshes health. A reverse proxy returning an HTML sign-in page instead of the REST response is unsupported.

## Discovery and tools

Discover the integration through Switchboard's `search` meta-tool, then call its read operations through `execute`:

```json
{"query":"OpenProject work packages tasks issues","integration":"openproject","limit":10}
```

| Tool | Inputs | Purpose |
|---|---|---|
| `openproject_list_work_packages` | Optional `subject_or_id` (at most 256 characters), `page` (positive integer, default 1), `limit` (1–50, default 20) | Start here to find work packages in the configured project, including open and closed statuses |
| `openproject_get_work_package` | Required positive numeric `id` from discovery | Inspect one work package's summary, dates and bounded description |

For example, pass the following objects to `execute` in order:

```json
{"tool_name":"openproject_list_work_packages","arguments":{"subject_or_id":"release readiness","page":1,"limit":20}}
```

```json
{"tool_name":"openproject_get_work_package","arguments":{"id":42}}
```

List requests use `GET /api/v3/workspaces/{configured_id}/work_packages`, not the deprecated `/projects/{id}/work_packages` collection alias. Exact reads use `GET /api/v3/work_packages/{numeric_id}`. Both are relative to the configured installation subpath and request `application/hal+json`.

### Version-specific collection route

The collection namespace and the project relationship namespace are intentionally different. In the supported **v17.9.2** release, the [workspace collection contract](https://github.com/opf/openproject/blob/v17.9.2/docs/api/apiv3/paths/workspace_work_packages.yml) defines `/api/v3/workspaces/{id}/work_packages`, and the [project collection contract](https://github.com/opf/openproject/blob/v17.9.2/docs/api/apiv3/paths/project_work_packages.yml) explicitly deprecates `/api/v3/projects/{id}/work_packages` in its favor. The [workspace router](https://github.com/opf/openproject/blob/v17.9.2/lib/api/v3/workspaces/nested_apis.rb) mounts `WorkPackagesByWorkspaceAPI`. By contrast, a work package's `_links.project.href` still uses `/api/v3/projects/{id}`. Do not derive the collection route from that relationship URL or substitute an older release's route in the client or fixtures.

`TestRequestAndProjection`, `TestHealthyUsesBoundedReadAndClose` and `TestUIOpenProject` intentionally require the workspace collection route, including the configured installation subpath. Real-product acceptance independently checks it: `liveOpenProjectProvision` requires an authenticated `GET /api/v3/workspaces/{id}/work_packages` to return HTTP 200, the seeded second-page record and correct paging metadata from the pinned official v17.9.2 container before running the adapter and MCP cases. These route assertions are not based only on mock agreement.

The adapter explicitly sends `filters=[]` when no search text is supplied, removing the API's default open-only status filter. Search text is encoded into a constructed `subjectOrId` filter with the documented `**` operator. It is not a caller-supplied filter object or arbitrary query language. Sorting is fixed to `[["id","asc"]]`. Callers cannot supply workspace overrides, URLs, paths, headers, credentials, `select`, grouping, sums, timestamps, baselines or arbitrary expansions.

### Paging and project isolation

Each list invocation fetches **one upstream page**. OpenProject's `offset` is a **one-based page number**, not a record offset. The adapter maps `page` to `offset` and `limit` to requested `pageSize`. Installation limits may produce a smaller actual page size; responses expose that upstream size rather than assuming the requested size was honored. There is no background pagination, recursive project traversal or automatic following of HAL links.

List output contains:

- `work_packages`: locally retained summary records.
- `total`, `count`: upstream collection total and the number of elements in that upstream page, before local project filtering.
- `page`, `page_size`: upstream page number and actual upstream page size.
- `returned`, `omitted`: number of summaries returned and number excluded for a mismatched or missing project reference.
- `truncated`: whether allowed text fields were shortened to meet their caps.
- `next_page`: the next upstream page number when metadata indicates continuation, otherwise null in raw/script output. Shared compaction may omit null fields.

OpenProject stores the work package's project relationship in `_links.project`. Its namespace is kind-specific: a project uses `/api/v3/projects/7`, while a portfolio is not a project reference. Deployment subpaths also occur in these links. The adapter accepts only the configured project's expected reference and never follows that URL. Work packages from other projects, including descendants returned by the collection, are omitted and counted. An exact read with a missing or mismatched project reference fails closed.

Local omission does **not** rewrite upstream totals, counts, page size or continuation. A page with no retained records may still have a `next_page`; it is not proof that the configured project has no matching records. This is bounded upstream search with local project isolation, not a complete local-search guarantee. Concurrent changes can shift records between pages. HTTP 404 can mean absent **or permission-hidden**, not proof of nonexistence.

## Output bounds and untrusted content

Every summary contains only `id`, `displayId`, `subject`, `status`, `type`, `project_id` and `updatedAt`. Status and type are labels from `_links.status.title` and `_links.type.title`, not full embedded objects. Exact reads add `createdAt`, `startDate`, `dueDate`, `description`, `description_truncated` and an overall `truncated` indicator. Unscheduled dates may be null in raw/script output and omitted by shared compaction.

Summary strings and date strings are capped at 512 Unicode characters. Description is the upstream `description.raw` string, capped at **8,000 characters**; `description_truncated` reports shortening. Normal exact-read execution presents a Markdown document with trusted numeric identifiers and a fenced text block containing the untrusted metadata and description. The fence is lengthened when necessary so source backticks cannot break out. If fencing would exceed the final 40 KiB bound, normal execution falls back to the bounded JSON projection without dropping content. Raw/script execution retains the allowlisted JSON detail object and its truncation flags. Upstream Markdown, HTML and other markup are never interpreted or executed. Work-package subjects, descriptions and labels are data, not agent instructions.

Typed field allowlists are applied **before raw/script output as well as normal MCP responses and compaction**. No `description.html`, custom fields, embedded users, email addresses, watchers, files, attachments, comments, costs or HAL action links are returned. The adapter does not fetch linked resources, execute actions, post comments or mutate work packages. Allowed text itself can contain sensitive information, so operators must still review the source content and account visibility.

Each operation has an overall **10-second deadline**, a **2 MiB decompressed upstream-body cap**, and a final projected JSON result strictly below **40 KiB**. If a projected page exceeds that output cap, request a smaller `limit`; the adapter does not silently discard records and invent continuation. HTTP 429 permits at most **two retries** with a valid bounded `Retry-After` delay of at most two seconds per wait, within the operation deadline. No universal upstream quota is assumed. Authentication/permission failures, redirects and arbitrary upstream errors do not trigger background retry or paging. Errors do not echo raw response bodies or tokens.

## Verification

```sh
go test -race ./integrations/openproject ./config ./server
go build -o /tmp/switchboard-openproject-ui ./cmd/server
SWITCHBOARD_UI_BINARY=/tmp/switchboard-openproject-ui go test -race -count=1 -v ./web -run '^TestUIOpenProject$'
make test-live INTEGRATION=openproject
make ci
```

`TestUIOpenProject` drives the real development binary using Go Playwright/Chromium, an isolated temporary HOME/config and an HTTPS HAL fixture. It checks fresh default fields and setup routing, placeholders and optional labels, token masking, invalid/missing/valid credentials, HTTP rejection, custom-CA verification, numeric workspace validation, save/enable, health refresh and recovery, reload/restart persistence, both tools discovered and executed over MCP, raw-script field exclusion, cross-project omission and disable persistence. The fixture verifies Basic username `apikey`, retained installation subpaths, explicit default-filter removal, encoded subject search and upstream page metadata.

Masked screenshots are named `openproject-desktop.png` and `openproject-mobile.png` under the configured `SWITCHBOARD_UI_ARTIFACTS` directory; `make test-ui` uses `dist/ui-artifacts/`. All inputs are masked. Browser JavaScript, console errors and unexpected browser HTTP errors fail the test. These fixtures prove the local UI lifecycle and the asserted contract, **not live authentication, production account authorization, rate-limit behavior or license compliance**.

Real-product validation is separate: run the live target against the disposable pinned product and inspect its outcomes and masked evidence before claiming it passed. Without a successful real-product run or explicitly supplied dedicated credentials and safe reads, **live authentication is unverified**. Never use a personal browser profile, production mutation probes or copied host configuration for acceptance testing.

## Upstream references

The contract and setup guidance were checked against the released v17.9.2 sources:

- [API introduction and authentication](https://github.com/opf/openproject/blob/v17.9.2/docs/api/apiv3/openapi-spec.yml)
- [Workspace work-package collection](https://github.com/opf/openproject/blob/v17.9.2/docs/api/apiv3/paths/workspace_work_packages.yml), [exact work-package read](https://github.com/opf/openproject/blob/v17.9.2/docs/api/apiv3/paths/work_package.yml), and [filter syntax](https://github.com/opf/openproject/blob/v17.9.2/docs/api/apiv3/tags/filters.yml)
- [Work-package schema](https://github.com/opf/openproject/blob/v17.9.2/docs/api/apiv3/components/schemas/work_package_model.yml) and [kind-specific workspace links](https://github.com/opf/openproject/blob/v17.9.2/lib/api/v3/workspaces/linked_resource.rb)
- [Access-token setup](https://github.com/opf/openproject/blob/v17.9.2/docs/user-guide/account-settings/access-tokens/README.md), [roles and permissions](https://github.com/opf/openproject/blob/v17.9.2/docs/system-admin-guide/users-permissions/roles-permissions/README.md), and [permissions guide](https://github.com/opf/openproject/blob/v17.9.2/docs/system-admin-guide/users-permissions/permissions-guide/README.md)
- [Official MCP guide](https://github.com/opf/openproject/blob/v17.9.2/docs/system-admin-guide/integrations/mcp-server/README.md) and [released MCP entitlement check](https://github.com/opf/openproject/blob/v17.9.2/lib/api/mcp.rb)
