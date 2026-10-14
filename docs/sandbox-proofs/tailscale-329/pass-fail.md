| Check | Result | Evidence |
|---|---|---|
| Build / vet / unit tests (tailscale, config) at 9888bfc | PASS | go-test.log: 200 subtests incl. new TestExecute_ReportsWhyConfigureFailed, TestExecute_ConfigureErrorClearedBySuccessfulConfigure, TestExecute_NeverConfigured |
| Bad creds surface specific reason on Execute (6 cases) | PASS 6/6 | before (main 6bc872e) every case = bare `tailscale: integration is not configured` |
| Clear-on-success in one running process (web UI save → re-Configure) | PASS 6/6 | bad → fake well-formed key gives clean real-API 401; bad again; valid key vs mock gives real data; half OAuth after success drops the working client; full OAuth gives upstream oauth2 error, not stale text |
| No credential values in server log / error text | PASS 3/3 | errors name the key kind only |
| Mock-API happy path, all main tools (44 discovered) | PASS 69 / INFO 1 | ACL HuJSON comment survival, 412 stale etag, devices/routes/keys/DNS/settings/invites/webhooks, restore byte-identical |
| Fake token vs real api.tailscale.com + client-side validation | PASS 24 | 401s, 13 validation rejections, 4 misconfig cases |
| Live real-tailnet happy path | WAITING | needs /home/box/.sb-sandbox-secrets/tailscale-328.env; then `./run-live.sh` |
