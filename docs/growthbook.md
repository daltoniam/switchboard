# GrowthBook feature flags and experiments

The GrowthBook adapter reads feature flags, A/B test experiments and their results, metrics, and the configuration around them (projects, environments, SDK connections, saved groups, attributes, data sources, segments, audit log, code references, settings and members) through the GrowthBook REST API. It can also make explicit changes: turn one flag on or off in one environment, create a flag that starts disabled everywhere, edit a flag's targeting rules in a draft and publish it, create, update, start and stop experiments, and start a new experiment analysis.

It targets the REST API served by GrowthBook Cloud, which runs GrowthBook's main branch. Feature flag tools use the `/v2` endpoints GrowthBook recommends; the deprecated `/v1` feature flag routes are not used. Live acceptance pins a main-branch image (version 5.2.0, commit `ffe6dab`). Older self-hosted releases may lack some newer endpoints (the tagged 5.2.0 image has no REST data source creation, for example); a tool that calls a missing endpoint returns GrowthBook's `Unknown API endpoint` error rather than empty data.

## Credentials

Configure the disabled-by-default integration through **Integrations > GrowthBook**.

| Credential | Environment override | Requirement |
|---|---|---|
| `api_key` | `GROWTHBOOK_API_KEY` | A Secret Key (`secret_...`) from **Settings > API Keys** or a Personal Access Token. SDK client keys (`sdk-...`) cannot call the REST API and are rejected. |
| `base_url` | `GROWTHBOOK_BASE_URL` | Optional. Defaults to GrowthBook Cloud, `https://api.growthbook.io/api`. For self-hosted GrowthBook, use your HTTPS `API_HOST`; `/api` is appended when missing. |
| `ca_cert` | `GROWTHBOOK_CA_CERT` | Optional PEM CA certificate for a self-hosted API with a private CA; literal `\n` accepted. Certificate and hostname verification stay on. |

The base URL must be HTTPS without credentials, query, fragment, encoded path or `..` segments. Redirects are not followed, so the key is only ever sent to the configured host. The key is sent as a bearer token and never as a query parameter.

### Choosing a role

Give the key the smallest role that covers the tools you enable:

- **Read-only use**: a Secret Key with the `readonly` role. Every read tool works with it.
- **Toggling or creating flags, editing rules, running experiments, refreshing results**: a role that can publish features and run experiment queries in the environments you intend to change, such as `engineer` or a custom role limited to those environments and projects.
- **Keep people in the loop**: GrowthBook can require a person to confirm risky writes for a key. When it does, the write returns `status: pending_confirmation` with a confirmation link and nothing changes until a person approves it in GrowthBook. The adapter never retries or works around this, and never sends override fields such as `ignoreWarnings`.

Self-hosted GrowthBook without a commercial license only offers the `admin` role for keys. In that case, treat the key as an administrator credential: store it carefully, set an expiration, and disable the write tools with a tool policy if you only need reads.

## Tools

| Tool | Access | Purpose |
|---|---|---|
| `growthbook_list_features` | read | Flags with per-environment state, rules and default value. Start here for flags. |
| `growthbook_get_feature` | read | One flag's rules, targeting, prerequisites and revision |
| `growthbook_get_feature_staleness` | read | Whether flags are stale, per environment |
| `growthbook_list_feature_revisions` | read | Drafts (default) or `status=published` change history |
| `growthbook_list_experiments` | read | Experiments with status, variations, phases and result summary. Start here for A/B tests. |
| `growthbook_get_experiment` | read | One experiment's hypothesis, targeting, metrics and linked flags |
| `growthbook_get_experiment_results` | read | Latest analysis: users, lift, intervals, chance to win, SRM and power checks |
| `growthbook_get_snapshot` | read | Status of an analysis started by the refresh tool |
| `growthbook_list_fact_metrics`, `growthbook_get_fact_metric` | read | Current metric definitions |
| `growthbook_list_metrics` | read | Legacy SQL metrics (`met_` ids) |
| `growthbook_list_fact_tables` | read | Warehouse queries that metrics are built on |
| `growthbook_list_projects`, `growthbook_list_environments` | read | Ids used by other tools |
| `growthbook_list_sdk_connections` | read | Which client keys serve which environment and projects |
| `growthbook_list_saved_groups`, `growthbook_get_saved_group` | read | Reusable audiences used in targeting |
| `growthbook_list_attributes`, `growthbook_list_segments`, `growthbook_list_data_sources` | read | Targeting attributes, analysis segments and warehouse connections |
| `growthbook_list_audit_logs` | read | Who changed an entity, when and why |
| `growthbook_list_code_refs`, `growthbook_get_feature_code_refs` | read | Where flag keys appear in source code |
| `growthbook_get_settings` | read | Flag key format, statistics settings and review requirements |
| `growthbook_list_members` | read | Members, roles and emails |
| `growthbook_toggle_feature` | write | Turn one flag on or off in exactly one environment |
| `growthbook_create_feature` | write | Create a flag disabled in every environment, with no rules |
| `growthbook_refresh_experiment_results` | write | Run a new analysis against the warehouse (costs warehouse compute) |
| `growthbook_add_feature_rule` | write | Add a force, rollout or experiment-ref rule to a draft revision (a new one unless `version` is given) |
| `growthbook_update_feature_rule`, `growthbook_delete_feature_rule` | write | Change or remove one rule in a draft revision |
| `growthbook_publish_feature_revision` | write | Make a draft live; needs an audit `comment` |
| `growthbook_discard_feature_revision` | write | Drop a draft without publishing |
| `growthbook_create_experiment` | write | Create an experiment, always as a draft |
| `growthbook_update_experiment` | write | Change an experiment's settings (not its status) |
| `growthbook_start_experiment`, `growthbook_stop_experiment` | write | Launch a draft; stop with a result and optional temporary rollout |

### Changing rules and running experiments

Rule edits always land in a draft revision and nothing reaches an SDK until `growthbook_publish_feature_revision`, so an agent can stage a change, show it (`growthbook_list_feature_revisions`), and publish only when a person asks. A new rule must name its environments (`environments` or `allEnvironments: true`) so production is never targeted by accident. Experiments are created as drafts and go live only through `growthbook_start_experiment`; linking one to a flag is an `experiment-ref` rule.

GrowthBook's gates are never bypassed. Required reviews, warnings, schema and hook failures, and the experiment launch checklist come back as errors naming the gate, for a person to resolve in GrowthBook. Override fields (`ignoreWarnings`, `skipSchemaValidation`, `skipHooks`, `skipChecklist`, `bypassApproval`, `allowUnscopedRules`) are rejected rather than forwarded, and ramp schedules are left to the app. If your organization lets the REST API bypass reviews (`restApiBypassesReviews`), a publish or start goes live at once; restrict these tools with a tool policy if agents should only read.

List tools take `limit` (1..100, default 20) and `offset`, and return GrowthBook's `total`, `hasMore` and `nextOffset`. The audit log pages with `before`, using `nextCursor` from the previous page.

## Data handling

Secrets are removed by the handlers before results reach either compaction or scripts: SDK connection encryption and proxy signing keys, any data source connection parameters, and the secure attribute hashing salt from settings. The compiled SDK payload GrowthBook repeats under each flag environment is also dropped because it restates the rules.

Members, owners and audit entries include names and email addresses, and saved groups can list user identifiers. Disable those tools with a tool policy when that is not appropriate.

GrowthBook Cloud allows 60 requests per minute. HTTP 429 and 5xx responses are retried by the server with the `Retry-After` delay. Publishing checks that block a write (HTTP 409 or 422 with `gates`) are returned as errors with GrowthBook's message, for a person to resolve.

## Verification

```bash
go test -race ./integrations/growthbook ./config ./server -run 'GrowthBook|TestSmoke_SearchResponseShape'
make test-live INTEGRATION=growthbook
make ci
```

`TestUIGrowthBook` drives the development binary in Chromium against an HTTPS fixture: fresh credential fields and placeholders, an invalid key, save and enable, connected health, restart persistence, MCP search and execute, a pending confirmation, and disable. Masked screenshots are written to `dist/ui-artifacts/growthbook-{desktop,mobile}.png`.

`web/live_growthbook_test.go` starts the official GrowthBook image with MongoDB and a PostgreSQL warehouse, all pinned by digest. It seeds the warehouse with exposure and purchase events, creates a data source, fact table, fact metric, legacy metric, segment, saved group, attribute, SDK connection, flags with force, rollout and experiment rules, a running experiment with computed results and a stopped experiment, a code reference, and a flag toggle, all through GrowthBook's REST API. The open-source image only issues admin-role keys, so the spec uses a dedicated admin Secret Key rather than the bootstrap user's session. The harness enters the credentials in the setup page, restarts Switchboard, calls every tool over MCP, and checks every compaction field against the real responses.
