### Sandbox E2E proof

**Merge-ready from the sandbox side at `c118818`:** all three adapters now end their remote MCP session on Close(), Close is idempotent, re-Configure starts a fresh session, and the Linear api_key switch and Figma base_url change end the old session. On main the same runs leak every session. No FAILs.

Head `c11881821e946c15189b8efc48360bed86a3acf2` (branch `fix/remote-mcp-integrations-close`), compared with main `5b82ef2`. 30 PASS / 0 FAIL / 3 SKIPPED (scenarios + CI). Adapter harness: after 22 PASS / 0 FAIL; main 8 PASS / 6 FAIL. Sessions still open on the stand-in at the end: after 0, main 8.

**Stand-in, not the real vendors.** No Linear/Figma/Slack hosted-MCP OAuth was used. Every session open/DELETE in this proof is against a local streamable-HTTP MCP server built on the official MCP Go SDK v1.7.0 (the same SDK and version `remotemcp` uses), mounted at `/linear`, `/figma-a`, `/figma-b`, `/slack` on 127.0.0.1:18349. It issues `Mcp-Session-Id`, serves tools/list and tool calls, logs every request, and answers 404 on deleted sessions. Linear native REST was checked against the real api.linear.app with a fake key (clean 401).

| Scenario | After (`c118818`) | Before (main `5b82ef2`) | Evidence (after) |
|---|---|---|---|
| Linear: Configure → session opened → tool call | PASS | PASS | tools=2, session_open=1 [DFJAKPWG47IAXXHQNTQXDRMPYD], linear_list_issues → {"query":"proof","session":"DFJAKPWG47IAXXHQNTQXDRMPYD","standin":"linear","tool":"list_issues"} |
| Linear: Close() → DELETE, old session 404 server-side | PASS | FAIL: no io.Closer; probe on dropped instance → 200 (leaked) | Close err=<nil>; DELETE count=1 sid=[DFJAKPWG47IAXXHQNTQXDRMPYD]; probe tools/list on old sid → HTTP 404 |
| Linear: second Close() no-op (no request, no error) | PASS | n/a | Close err=<nil>; new server events=0 (DELETE=0) |
| Linear: re-Configure → fresh session id → tool call | PASS | n/a | session_open=1 new=[LD5FZQRSEYBEVIKNDUPBHU6SNP] old=DFJAKPWG47IAXXHQNTQXDRMPYD; {"query":"proof","session":"LD5FZQRSEYBEVIKNDUPBHU6SNP","standin":"linear","tool":"list_issues"} |
| Figma: Configure → session opened → tool call | PASS | PASS | tools=2, session_open=1 [X2XUFV35PLPAY5XX6V6L6MZK37], figma_whoami → {"query":"","session":"X2XUFV35PLPAY5XX6V6L6MZK37","standin":"figma-a","tool":"whoami"} |
| Figma: Close() → DELETE, old session 404 server-side | PASS | FAIL: no io.Closer; probe on dropped instance → 200 (leaked) | Close err=<nil>; DELETE count=1 sid=[X2XUFV35PLPAY5XX6V6L6MZK37]; probe tools/list on old sid → HTTP 404 |
| Figma: second Close() no-op (no request, no error) | PASS | n/a | Close err=<nil>; new server events=0 (DELETE=0) |
| Figma: re-Configure → fresh session id → tool call | PASS | n/a | session_open=1 new=[DPP4X67MKNG7TOBPKXHSDTRWF7] old=X2XUFV35PLPAY5XX6V6L6MZK37; {"query":"","session":"DPP4X67MKNG7TOBPKXHSDTRWF7","standin":"figma-a","tool":"whoami"} |
| Slack MCP: Configure → session opened → tool call | PASS | PASS | tools=3, session_open=1 [W43CYZ3F5TRBGNYJVLO5FJJLIE], slackmcp_search_channels → {"query":"general","session":"W43CYZ3F5TRBGNYJVLO5FJJLIE","standin":"slack","tool":"slack_search_channels"} |
| Slack MCP: Close() → DELETE, old session 404 server-side | PASS | FAIL: no io.Closer; probe on dropped instance → 200 (leaked) | Close err=<nil>; DELETE count=1 sid=[W43CYZ3F5TRBGNYJVLO5FJJLIE]; probe tools/list on old sid → HTTP 404 |
| Slack MCP: second Close() no-op (no request, no error) | PASS | n/a | Close err=<nil>; new server events=0 (DELETE=0) |
| Slack MCP: re-Configure → fresh session id → tool call | PASS | n/a | session_open=1 new=[OLSSYWD2D53TUMRMGPIA5JBOSR] old=W43CYZ3F5TRBGNYJVLO5FJJLIE; {"query":"general","session":"OLSSYWD2D53TUMRMGPIA5JBOSR","standin":"slack","tool":"slack_search_channels"} |
| Linear mcp_access_token → api_key ends remote session | PASS | FAIL: 0 DELETE, old session → 200 | err=<nil> DELETE=1 [JNBRAXASB6ZHIVEFM2MZNC47CR]; probe old sid → HTTP 404; IsRemoteMCP=false |
| Linear api_key native REST still wired (fake key → api.linear.app 401) | PASS | PASS | linear_viewer → IsError=true linear API error (401): {"errors":[{"message":"Authentication required, not authenticated","extensions":{"type":"authentication error","code":"AUTHENTICATION_ERROR","statusCode":401,"userError":true,"… |
| Figma base_url A → B ends session on A | PASS | FAIL: 0 DELETE, A session → 200 | err=<nil> DELETE on figma-a=1 [MZO7INX3V7E2EVDWT4ZEFT7W4T]; probe old A sid → HTTP 404 |
| Figma new session on B + call works | PASS | PASS | figma-b sid=[PPUTA47QMWITDL72OQY4O32QXD] → {"query":"","session":"PPUTA47QMWITDL72OQY4O32QXD","standin":"figma-b","tool":"whoami"} |
| Slack MCP two identities → two sessions, calls on each | PASS | PASS | err=<nil> sessions=[P7HJQMRZ6B7OQAWFAJDTIJSUAV QLDE5LABSO6RJYPUPW4JQS2RAJ] work→{"query":"","session":"QLDE5LABSO6RJYPUPW4JQS2RAJ","standin":"slack","tool":"slack_read_ch… personal→{"query":"","session":"P7HJQMRZ6B7OQAWFAJDTIJSUAV |
| Slack MCP Close() ends both sessions | PASS | FAIL: no io.Closer; both → 200 (leaked) | err=<nil> DELETE=2 [P7HJQMRZ6B7OQAWFAJDTIJSUAV QLDE5LABSO6RJYPUPW4JQS2RAJ]; probes → 404, 404 |
| Slack MCP second Close() no-op | PASS | n/a | err=<nil> events=0 |
| Slack MCP Healthy=false after Close | PASS | n/a |  |
| Host: local Switchboard search/execute → figma_whoami, slackmcp_read_channel (2 identities) | PASS | PASS | PR build on 127.0.0.1:13855, base_url → stand-in; see responses-host.json |
| Host: web Save figma base_url A→B ends A session | PASS | INFO: 0 DELETE, A → 200 (leaked) | save HTTP 200 → http://127.0.0.1:13855/integrations/figma/setup?success=Configuration+saved; DELETE on figma-a=['HZFNZQIODJYGLLSFZPVNK62ZPS']; probe old figma-a sid HZFNZQIODJYGLLSFZPVNK62ZPS → HTTP 4 |

| CI step | Result | Note |
|---|---|---|
| `make ci` → styles / proto-check / test-ui | SKIPPED | `make` not installed on the box (no root); Node 26.8.1 pinned (box has v20), no `buf`, no Docker/Playwright. PR touches no web/proto files |
| build (`go build ./cmd/server`) | PASS | logs/ci-build.log |
| vet (`go vet ./...`) | PASS | logs/ci-vet.log |
| test-race (`go test -race ./...`) | PASS | 94 packages ok, 0 FAIL (logs/ci-test-race-all.log) |
| `scripts/compose-dev_test.sh` | PASS | logs/ci-compose-dev-test.log |
| lint (`go tool golangci-lint run`) | PASS | 0 issues (logs/ci-lint.log) |
| gosec (Makefile flags) | PASS | logs/ci-gosec.log |
| govulncheck | SKIPPED | process killed on the box twice (~2-4 min); go.mod/go.sum unchanged by this PR |
| test-web | SKIPPED | only test is .ts; needs Node ≥22 (box v20) |
| New tests: TestRemoteMCPAdapters_ImplementClose + 6 adapter Close tests | PASS | 7/7, also under -race |
| Guard test catches the bug (run against main 5b82ef2) | PASS | fails on main naming figma, linear, slackmcp (logs/guard-test-on-main.log) |

Caveats:
- Stand-in server (see above). Real hosted servers may run stateless, with no `Mcp-Session-Id`; then the SDK client has nothing to DELETE and Close only drops local state. That is fine, but this proof can't show what Linear/Figma/Slack do server-side.
- Linear's MCP URL is hard-coded in `cmd/server/main.go` (`https://mcp.linear.app`), so Linear could not be pointed at the stand-in through a running Switchboard. Linear was driven through `linear.New(<stand-in>)` in a scratch harness module (`harness/`, outside the PR tree, `replace` → worktree). Figma and Slack MCP were also run through a real local Switchboard via `base_url`.
- The local Switchboard host never calls `Close()`: not on shutdown, not when dropping an instance. After SIGTERM the Slack sessions were still live on the stand-in (both builds). This PR is for hosts that do call Close (Switchboard Hosted per-request pools); the local server only benefits through the in-Configure replacement paths (Figma base_url, Linear api_key). Possible follow-up: close integrations on graceful shutdown.
- Linear after Close() (without a re-Configure) falls through to the native GraphQL path, because `useRemote=false` and `apiKey=""`. A stray Execute then sends an unauthenticated request to api.linear.app and gets back a 401. Figma returns "integration not configured" and Slack returns "unknown identity". Minor, and only on use-after-Close; a nil check returning `ErrNotConfigured` would make it consistent.
- Linear and Figma `Close`/`closeRemote` take no lock, the same as their existing unlocked fields. A concurrent Close and Execute is a pre-existing race pattern, not new; `remotemcp.remote.Close` itself is mutex-guarded. Slack MCP locks `cfgMu`+`mu`.
- `make ci` could not run as a whole: no `make` binary or root on the box, Node 26.8.1 is pinned (box has v20), and there is no buf/Docker/Playwright. I ran the Go recipe commands directly: build, vet, `go test -race ./...`, compose-dev test, golangci-lint and gosec all PASS. govulncheck was killed on the box; go.mod is unchanged by this PR.

Artifacts: `sandbox-e2e-proofs` → `docs/sandbox-proofs/remote-mcp-close-349/` (REPORT.md, pass-fail.md, proof.mp4, responses.json, logs/standin-*.jsonl, screenshots/).
