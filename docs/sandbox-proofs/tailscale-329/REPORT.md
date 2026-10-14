# Sandbox E2E proof: daltoniam/switchboard PR #329 (Tell agents why Tailscale credentials were rejected)
- Head `9888bfc44a891af0b16921b2cbcf7be9cb41847b` (branch fix/tailscale-configure-error); parent `6bc872e` = current `main` = #328 squash-merged. Diff vs main: tailscale.go (+19/-6), tailscale_test.go (+41).
- Before baseline: binary built from main `6bc872e`. After: binary from 9888bfc. Isolated SWITCHBOARD_DEV_HOME/HOME per run under homes/, MCP on 127.0.0.1:13860-13862 / 13854.
- Run counts: configure-error suite {'PASS': 15}; mock API {'PASS': 69, 'INFO': 1}; fake token/validation {'PASS': 24}.

## Before / after messaging
| Case | Before | After | Result |
|---|---|---|---|
| `auth_key_as_api_key` | `tailscale: integration is not configured` | `… api_key looks like a device auth key; auth keys join devices and cannot call the API, use an API access token (tskey-api-...)` | PASS |
| `client_secret_as_api_key` | `tailscale: integration is not configured` | `… api_key looks like an OAuth client secret; put it in oauth_client_secret along with oauth_client_id` | PASS |
| `token_and_oauth` | `tailscale: integration is not configured` | `… set either api_key or oauth_client_id + oauth_client_secret, not both` | PASS |
| `half_oauth_id_only` | `tailscale: integration is not configured` | `… oauth_client_id and oauth_client_secret must both be set` | PASS |
| `half_oauth_secret_only` | `tailscale: integration is not configured` | `… oauth_client_id and oauth_client_secret must both be set` | PASS |
| `missing_creds` | `tailscale: integration is not configured` | `… api_key or oauth_client_id + oauth_client_secret is required` | PASS |

## Clear-on-success (single process, reconfigured via the integration Save form → redirect to detail page, which re-runs Configure)
| Step | Result | Evidence |
|---|---|---|
| 1 bad config -> specific reason | PASS | tailscale: integration is not configured: tailscale: api_key looks like a device auth key; auth keys join devices and cannot call the API, use an API access token (tskey- |
| 2 reconfigure fake well-formed key (real API) -> clean 401, stored error gone | PASS | save=200 /integrations/tailscale?success=Configuration+saved / tailscale API error (401): {"message":"API token invalid"} |
| 3 back to bad key -> reason returns, old client not reused | PASS | save=200 / tailscale: integration is not configured: tailscale: api_key looks like a device auth key; auth keys join devices and cannot call the API, use an API access to |
| 4 reconfigure valid key vs mock API -> real success, no stored error | PASS | save=200 / {"devices":[{"advertisedRoutes":["10.99.0.0/24"],"authorized":false,"hostname":"sb-node","id":"1001","name":"sb-node.tail-mock.ts.net","nodeId":"n1"}]} |
| 5 half OAuth after success -> reason, previous working client dropped | PASS | save=200 / tailscale: integration is not configured: tailscale: oauth_client_id and oauth_client_secret must both be set |
| 6 OAuth client creds (mock token endpoint missing) -> error is upstream OAuth, not stale configure error | PASS | save=200 / Get "http://127.0.0.1:18329/api/v2/tailnet/-/devices?fields=default": oauth2: cannot fetch token: 401 Unauthorized
Response: {"message": "API token invalid"} |

## PASS/FAIL
| Check | Result | Evidence |
|---|---|---|
| Build / vet / unit tests (tailscale, config) at 9888bfc | PASS | go-test.log: 200 subtests incl. new TestExecute_ReportsWhyConfigureFailed, TestExecute_ConfigureErrorClearedBySuccessfulConfigure, TestExecute_NeverConfigured |
| Bad creds surface specific reason on Execute (6 cases) | PASS 6/6 | before (main 6bc872e) every case = bare `tailscale: integration is not configured` |
| Clear-on-success in one running process (web UI save → re-Configure) | PASS 6/6 | bad → fake well-formed key gives clean real-API 401; bad again; valid key vs mock gives real data; half OAuth after success drops the working client; full OAuth gives upstream oauth2 error, not stale text |
| No credential values in server log / error text | PASS 3/3 | errors name the key kind only |
| Mock-API happy path, all main tools (44 discovered) | PASS 69 / INFO 1 | ACL HuJSON comment survival, 412 stale etag, devices/routes/keys/DNS/settings/invites/webhooks, restore byte-identical |
| Fake token vs real api.tailscale.com + client-side validation | PASS 24 | 401s, 13 validation rejections, 4 misconfig cases |
| Live real-tailnet happy path | WAITING | needs /home/box/.sb-sandbox-secrets/tailscale-328.env; then `./run-live.sh` |

## Caveats
- **Live tailnet: WAITING.** No sandbox Tailscale credential exists yet. Run `./run-live.sh` (329 build) or `./run-live.sh 328main` once `/home/box/.sb-sandbox-secrets/tailscale-328.env` exists.
- The happy-path checks use a local stateful mock (`mock_tailscale.py`) via `base_url`. That proves adapter behaviour and wire format, not Tailscale's real responses.
- **Host behaviour (not this PR):** the web Save handler itself does not call Configure. The adapter is re-Configured by the detail page the browser is redirected to, by the health cache, or on restart. A client that POSTs without following the redirect keeps the old stored error until then. I saw this when the redirect wasn't followed: after saving a good key, Execute still said "looks like a device auth key". Normal browser flow is fine.
- The message has a doubled prefix: `tailscale: integration is not configured: tailscale: api_key …`. This is cosmetic.
- `configureErr` is read in Execute and written in Configure with no lock, the same as the existing `httpClient`/`apiKey` fields. A concurrent reconfigure and Execute is a pre-existing data race, not new.
- Carried over from #328 review, not changed here: `create_webhook` returns the signing secret unredacted, there's no `get_webhook` tool, and `base_url` isn't checked against netpolicy.
- Merge readiness: ready from the sandbox side for what this PR changes. The live check only re-confirms #328 behaviour.
