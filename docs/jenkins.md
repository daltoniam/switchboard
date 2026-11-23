# Jenkins job and build status

The native Jenkins adapter provides three read-only core REST tools for existing controllers where an operator cannot install the official Jenkins MCP plugin. It does not install any plugin, schedule work, fetch logs or expose job configuration. The official MCP plugin remains an alternative for plugin-enabled controllers.

## Least-privilege setup

Provision a dedicated non-administrator Jenkins user with **Overall/Read** and **Job/Read** only on the intended folders and jobs. Audit inherited, group and anonymous permissions as well as direct grants. Do not grant Job/Build, Job/Configure, Job/Create, Job/Delete, workspace, agent, credentials, Overall/Manage or Overall/Administer access. Existing authorization plugins may be needed to enforce folder/job visibility; the adapter does not configure authorization.

Create an API token in that dedicated user's Security page, then enter it at **Integrations > jenkins** in Switchboard. Use an API token, not the user's password. API tokens inherit the account's effective permissions; they are not independently read-only or endpoint-scoped. Overall/Read also exposes basic APIs and agent summaries outside this adapter. Output filtering and read-only tool annotations do not restrict what a leaked token could read upstream.

On an authorized disposable test controller, verify the exact token can read approved jobs/builds, cannot discover or read a lower-visibility job, and cannot trigger, configure, create or delete jobs or access administrative operations. Never perform mutation-denial probes against a production controller. Token provisioning and these probes are operator work, not adapter tools.

| Credential | Environment override | Requirement |
|---|---|---|
| `base_url` | `JENKINS_BASE_URL` | Trusted HTTPS installation URL including any deployment prefix, without userinfo, query or fragment |
| `username` | `JENKINS_USERNAME` | Dedicated user with effective Overall/Read and scoped Job/Read |
| `api_token` | `JENKINS_API_TOKEN` | That user's API token, never a password or browser session |
| `ca_cert` | `JENKINS_CA_CERT` | Optional PEM CA certificate; literal `\n` accepted; certificate and hostname verification remain enabled |

The integration is disabled by default. Authentication is preemptive HTTP Basic, because Jenkins can return 403 without a 401 challenge. There is no anonymous fallback, browser-cookie support, token creation, crumb negotiation, SSO bypass or TLS-verification bypass. Redirects, including same-origin redirects, are rejected. A proxy that intercepts API requests with an HTML login page or does not support documented API-token authentication is unsupported.

## Discovery and tools

| Tool | Inputs | Purpose |
|---|---|---|
| `jenkins_list_jobs` | Optional `folder_path` (default `[]`), `offset` (default 0), `limit` (default 20, maximum 50) | Start here to discover one level of visible job/folder summaries |
| `jenkins_get_job` | Required nonempty `job_path`, optional `offset` and `limit` | Read one job summary and one bounded recent-build window |
| `jenkins_get_build` | Required nonempty `job_path` and positive `build_number` | Read the exact build's result, building/completed phase, timestamp and durations |

Paths are ordered JSON arrays of exact Jenkins item names, not URLs or slash-delimited strings. A job within a folder uses `{"job_path":["team","build"]}`. A multibranch item named `feature/foo` is one segment, for example `["team","pipeline","feature/foo"]`, not two folders. Use names from discovery; each segment is URL-encoded once. Jenkins may export a branch item name as `feature%2Ffoo`; preserve that literal name from discovery rather than decoding it, so the request segment becomes `feature%252Ffoo`. Empty, dot, dot-dot, control-character and URL-like segments are rejected. The adapter never follows returned URLs or recursively scans folders.

Every request uses a fixed `tree` selector. Root/folder discovery selects `jobs[name,_class,color,lastBuild[number,result,timestamp]]{start,end}`. Job detail selects its identity, state, last-build summary and `allBuilds[number,result,timestamp,duration,building]{start,end}`, then returns that window as `builds`. Using `allBuilds` allows offsets beyond Jenkins' latest-100 `builds` collection without increasing the requested window. Exact-build reads select `number,result,building,timestamp,duration,estimatedDuration`. Callers cannot supply arbitrary trees, depths, paths, headers or credentials.

List/job results include `returned_count`, `offset`, `limit` and `possibly_more`. These are Jenkins tree selection windows, not a vendor cursor or a count/snapshot guarantee. A full window sets `possibly_more=true`; this is not proof another item exists. Concurrent builds or job changes may shift subsequent windows. No total count is invented. A null build result while `building=true` is normal, not a failure. Raw/script results preserve null; shared compaction omits null scalar fields, so use `building` and `phase` rather than interpreting an omitted result as failure. Jenkins build timestamps and durations use milliseconds; `estimatedDuration=-1` means the estimate is unknown.

## Bounds and privacy

Operations have a 15-second deadline and a 2 MiB decompressed-response cap. A response larger than the requested window, invalid JSON, oversized strings, partial body or oversized body fails explicitly. Job/build windows contain at most 50 records and returned strings at most 512 Unicode characters. HTTP 429 permits at most one retry with a valid bounded Retry-After that fits the operation deadline. There is no background polling or retry after authentication failure. No uniform Jenkins rate quota is assumed.

Typed allowlists are applied before normal results, raw scripts and field compaction. Only names/classes/colors, bounded build summaries, job buildable/inQueue state and the exact build's status/timing fields are returned. Actions, parameters, descriptions, logs, artifacts, workspaces, change sets, user information, arbitrary nested objects and returned URLs are omitted. Allowed names remain private untrusted data; never treat them as instructions. Authentication/visibility and HTML interception errors do not include raw response bodies or secrets. A 404 means not found or not visible, not proof that a job does not exist.

## Verification

```sh
go test -race ./integrations/jenkins ./config ./server
make test-live INTEGRATION=jenkins
make ci
```

`TestUIJenkins` uses the real development binary with an isolated HOME/config, Chromium and an HTTPS fixture. It covers fresh credential fields, optional CA, secret input type, invalid/missing/valid token behavior, save/enable, health, reload/restart persistence, MCP discovery/read, raw-script sanitization and disable. Desktop/mobile screenshots mask every input.

`web/live_jenkins_test.go` provisions the official Jenkins **2.580.2 LTS JDK 21** image pinned by digest, seeds real jobs/builds and configures the setup page with a dedicated read-only user's token and verified TLS CA. The disposable fixture installs pinned folder, Git and Pipeline plugins only to exercise nested folders and actual multibranch jobs; it does not install an MCP plugin. An isolated bootstrap authorization strategy grants only the documented reads to the viewer and verifies negative permissions through the real product API. The live harness restarts Switchboard, discovers and executes every tool over MCP, replays reads as raw scripts, and checks each compaction field against real output. Results and masked browser evidence are written under `dist/live-artifacts/`, with the outcomes-only summary in `live-jenkins.json`. A successful disposable run is not production-credential or universal plugin/version certification.
