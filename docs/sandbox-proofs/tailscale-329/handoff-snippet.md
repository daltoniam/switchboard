### Sandbox E2E proof

**Merge-ready from the sandbox side on `9888bfc`** (offline scope; live tailnet run WAITING on a sandbox credential). Every rejected-credential case now tells the agent why on Execute, and a later successful Configure clears it. Base: stacked on `main` at `6bc872e` (#328 already merged), not on `feat/tailscale-integration`. Auth: native REST, API token or OAuth client credentials. No real tailnet used.

108 PASS / 0 FAIL / 1 INFO / 1 WAITING

| Bad credential | Before (main 6bc872e) | After (9888bfc) | Status |
|---|---|---|---|
| `auth_key_as_api_key` | `tailscale: integration is not configured` | `… api_key looks like a device auth key; auth keys join devices and cannot call the API, use an API access token (tskey-api-...)` | PASS |
| `client_secret_as_api_key` | `tailscale: integration is not configured` | `… api_key looks like an OAuth client secret; put it in oauth_client_secret along with oauth_client_id` | PASS |
| `token_and_oauth` | `tailscale: integration is not configured` | `… set either api_key or oauth_client_id + oauth_client_secret, not both` | PASS |
| `half_oauth_id_only` | `tailscale: integration is not configured` | `… oauth_client_id and oauth_client_secret must both be set` | PASS |
| `half_oauth_secret_only` | `tailscale: integration is not configured` | `… oauth_client_id and oauth_client_secret must both be set` | PASS |
| `missing_creds` | `tailscale: integration is not configured` | `… api_key or oauth_client_id + oauth_client_secret is required` | PASS |

| Check | Result | Evidence |
|---|---|---|
| Build / vet / unit tests (tailscale, config) at 9888bfc | PASS | go-test.log: 200 subtests incl. new TestExecute_ReportsWhyConfigureFailed, TestExecute_ConfigureErrorClearedBySuccessfulConfigure, TestExecute_NeverConfigured |
| Bad creds surface specific reason on Execute (6 cases) | PASS 6/6 | before (main 6bc872e) every case = bare `tailscale: integration is not configured` |
| Clear-on-success in one running process (web UI save → re-Configure) | PASS 6/6 | bad → fake well-formed key gives clean real-API 401; bad again; valid key vs mock gives real data; half OAuth after success drops the working client; full OAuth gives upstream oauth2 error, not stale text |
| No credential values in server log / error text | PASS 3/3 | errors name the key kind only |
| Mock-API happy path, all main tools (44 discovered) | PASS 69 / INFO 1 | ACL HuJSON comment survival, 412 stale etag, devices/routes/keys/DNS/settings/invites/webhooks, restore byte-identical |
| Fake token vs real api.tailscale.com + client-side validation | PASS 24 | 401s, 13 validation rejections, 4 misconfig cases |
| Live real-tailnet happy path | WAITING | needs /home/box/.sb-sandbox-secrets/tailscale-328.env; then `./run-live.sh` |
