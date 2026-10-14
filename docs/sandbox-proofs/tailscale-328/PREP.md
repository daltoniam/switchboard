# PR #328 Tailscale: sandbox proof prep (offline done, live pending credential)
Head 48c1b02a25730a380ef9c703cdd9b617f61fcbf1 (worktree /workspace/sb-tailscale-328, binary /tmp/sb-ts328). go build ./..., go vet, go test tailscale+config: all pass (go-test.log).
Offline: harness-fake.log (24 PASS: search 44/44 tools, 13 validation rejections, 4x 401 via real API, 4 Configure misconfig errors),
harness-mock.log (69 PASS, 1 INFO: full flow against local stateful mock via base_url; HuJSON comment survival, 412 stale etag, quoted If-Match, webhook list strips secret, snapshot/restore byte-identical).
## Live run
1. /home/box/.sb-sandbox-secrets/tailscale-328.env (chmod 600): TS_API_KEY=tskey-api-..., TS_TAILNET=- (or org name), optional TS_OAUTH_CLIENT_ID/TS_OAUTH_CLIENT_SECRET, optional TS_PREVIEW_USER=<login email>
2. cd /workspace/sb-sandbox-proofs/tailscale-328 && python3 run-e2e.py --mode live | tee harness-live.log
   (creates its own auth key, joins userspace tailscaled node via node.sh advertising 10.99.0.0/24; --no-node to skip)
3. optional: python3 run-e2e.py --mode oauth | tee harness-oauth.log  (OAuth client needs scopes for devices, keys(auth_keys), policy_file, dns, users, settings, webhooks; tagged auth keys only with OAuth)
Outputs: responses-<mode>.json, mcp-raw-<mode>.json (redacted), switchboard-<mode>.log. Then record proof.mp4 + REPORT.md.
