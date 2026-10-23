## Scenario summary

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

## CI

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

## Adapter harness — after (c118818)

| Scenario | Step | Result | Evidence |
|---|---|---|---|
| 1 Linear (mcp_access_token) | Configure → session opened → tool call works | PASS | tools=2, session_open=1 [DFJAKPWG47IAXXHQNTQXDRMPYD], linear_list_issues → {"query":"proof","session":"DFJAKPWG47IAXXHQNTQXDRMPYD","standin":"linear","tool":"list_issues"} |
| 1 Linear (mcp_access_token) | Close() → DELETE sent, session invalid server-side | PASS | Close err=<nil>; DELETE count=1 sid=[DFJAKPWG47IAXXHQNTQXDRMPYD]; probe tools/list on old sid → HTTP 404 |
| 1 Linear (mcp_access_token) | second Close() is a no-op | PASS | Close err=<nil>; new server events=0 (DELETE=0) |
| 1 Linear (mcp_access_token) | Execute after Close (before re-Configure) | INFO | IsError=true err=<nil> data=linear API error (401): {"errors":[{"message":"Authentication required, not authenticated","extensions":{"type":"authentication error","code":"AUTHENTICATION_ER… |
| 1 Linear (mcp_access_token) | re-Configure → fresh session id → tool call works | PASS | session_open=1 new=[LD5FZQRSEYBEVIKNDUPBHU6SNP] old=DFJAKPWG47IAXXHQNTQXDRMPYD; {"query":"proof","session":"LD5FZQRSEYBEVIKNDUPBHU6SNP","standin":"linear","tool":"list_issues"} |
| 1 Figma | Configure → session opened → tool call works | PASS | tools=2, session_open=1 [X2XUFV35PLPAY5XX6V6L6MZK37], figma_whoami → {"query":"","session":"X2XUFV35PLPAY5XX6V6L6MZK37","standin":"figma-a","tool":"whoami"} |
| 1 Figma | Close() → DELETE sent, session invalid server-side | PASS | Close err=<nil>; DELETE count=1 sid=[X2XUFV35PLPAY5XX6V6L6MZK37]; probe tools/list on old sid → HTTP 404 |
| 1 Figma | second Close() is a no-op | PASS | Close err=<nil>; new server events=0 (DELETE=0) |
| 1 Figma | Execute after Close (before re-Configure) | INFO | IsError=true err=<nil> data=integration not configured |
| 1 Figma | re-Configure → fresh session id → tool call works | PASS | session_open=1 new=[DPP4X67MKNG7TOBPKXHSDTRWF7] old=X2XUFV35PLPAY5XX6V6L6MZK37; {"query":"","session":"DPP4X67MKNG7TOBPKXHSDTRWF7","standin":"figma-a","tool":"whoami"} |
| 1 Slack MCP (1 identity) | Configure → session opened → tool call works | PASS | tools=3, session_open=1 [W43CYZ3F5TRBGNYJVLO5FJJLIE], slackmcp_search_channels → {"query":"general","session":"W43CYZ3F5TRBGNYJVLO5FJJLIE","standin":"slack","tool":"slack_search_channels"} |
| 1 Slack MCP (1 identity) | Close() → DELETE sent, session invalid server-side | PASS | Close err=<nil>; DELETE count=1 sid=[W43CYZ3F5TRBGNYJVLO5FJJLIE]; probe tools/list on old sid → HTTP 404 |
| 1 Slack MCP (1 identity) | second Close() is a no-op | PASS | Close err=<nil>; new server events=0 (DELETE=0) |
| 1 Slack MCP (1 identity) | Execute after Close (before re-Configure) | INFO | IsError=true err=<nil> data=unknown identity: work |
| 1 Slack MCP (1 identity) | re-Configure → fresh session id → tool call works | PASS | session_open=1 new=[OLSSYWD2D53TUMRMGPIA5JBOSR] old=W43CYZ3F5TRBGNYJVLO5FJJLIE; {"query":"general","session":"OLSSYWD2D53TUMRMGPIA5JBOSR","standin":"slack","tool":"slack_search_channels"} |
| 2 Linear mcp → api_key switch | remote mode session opened + call works | PASS | sid=[JNBRAXASB6ZHIVEFM2MZNC47CR] IsRemoteMCP=true |
| 2 Linear mcp → api_key switch | Configure(api_key) ends remote session | PASS | err=<nil> DELETE=1 [JNBRAXASB6ZHIVEFM2MZNC47CR]; probe old sid → HTTP 404; IsRemoteMCP=false |
| 2 Linear mcp → api_key switch | api_key native REST path wired (fake key → api.linear.app) | PASS | linear_viewer → IsError=true linear API error (401): {"errors":[{"message":"Authentication required, not authenticated","extensions":{"type":"authentication error","code":"AUTHENTICATION_ERROR","statu |
| 3 Figma base_url change | session on server A + call works | PASS | figma-a sid=[MZO7INX3V7E2EVDWT4ZEFT7W4T] |
| 3 Figma base_url change | same base_url + same token re-Configure reuses session | INFO | session_open=0 DELETE=0 |
| 3 Figma base_url change | Configure(base_url=B) ends session on A | PASS | err=<nil> DELETE on figma-a=1 [MZO7INX3V7E2EVDWT4ZEFT7W4T]; probe old A sid → HTTP 404 |
| 3 Figma base_url change | new session opened on B + call works | PASS | figma-b sid=[PPUTA47QMWITDL72OQY4O32QXD] → {"query":"","session":"PPUTA47QMWITDL72OQY4O32QXD","standin":"figma-b","tool":"whoami"} |
| 4 Slack MCP two identities | two identities → two sessions, calls work on each | PASS | err=<nil> sessions=[P7HJQMRZ6B7OQAWFAJDTIJSUAV QLDE5LABSO6RJYPUPW4JQS2RAJ] work→{"query":"","session":"QLDE5LABSO6RJYPUPW4JQS2RAJ","standin":"slack","tool":"slack_read_ch… personal→{"query":"","sessio |
| 4 Slack MCP two identities | Close() ends both sessions | PASS | err=<nil> DELETE=2 [P7HJQMRZ6B7OQAWFAJDTIJSUAV QLDE5LABSO6RJYPUPW4JQS2RAJ]; probes → 404, 404 |
| 4 Slack MCP two identities | second Close() no-op | PASS | err=<nil> events=0 |
| 4 Slack MCP two identities | Healthy false after Close | PASS |  |

## Adapter harness — before (main 5b82ef2)

| Scenario | Step | Result | Evidence |
|---|---|---|---|
| 1 Linear (mcp_access_token) | Configure → session opened → tool call works | PASS | tools=2, session_open=1 [55LHIA6G3BJQ7OSWEKYBRQJ7ST], linear_list_issues → {"query":"proof","session":"55LHIA6G3BJQ7OSWEKYBRQJ7ST","standin":"linear","tool":"list_issues"} |
| 1 Linear (mcp_access_token) | adapter implements io.Closer | FAIL | type does not implement io.Closer; host cannot end the session |
| 1 Linear (mcp_access_token) | session after host drops instance (no Close possible) | INFO | probe tools/list on 55LHIA6G3BJQ7OSWEKYBRQJ7ST → HTTP 200 (session still alive = leaked) |
| 1 Figma | Configure → session opened → tool call works | PASS | tools=2, session_open=1 [IBK64W7EY4RTMYGGQNKCFSRBXR], figma_whoami → {"query":"","session":"IBK64W7EY4RTMYGGQNKCFSRBXR","standin":"figma-a","tool":"whoami"} |
| 1 Figma | adapter implements io.Closer | FAIL | type does not implement io.Closer; host cannot end the session |
| 1 Figma | session after host drops instance (no Close possible) | INFO | probe tools/list on IBK64W7EY4RTMYGGQNKCFSRBXR → HTTP 200 (session still alive = leaked) |
| 1 Slack MCP (1 identity) | Configure → session opened → tool call works | PASS | tools=3, session_open=1 [UC2QP4HJEJ4RJJUDRWDLGRGY4N], slackmcp_search_channels → {"query":"general","session":"UC2QP4HJEJ4RJJUDRWDLGRGY4N","standin":"slack","tool":"slack_search_channels"} |
| 1 Slack MCP (1 identity) | adapter implements io.Closer | FAIL | type does not implement io.Closer; host cannot end the session |
| 1 Slack MCP (1 identity) | session after host drops instance (no Close possible) | INFO | probe tools/list on UC2QP4HJEJ4RJJUDRWDLGRGY4N → HTTP 200 (session still alive = leaked) |
| 2 Linear mcp → api_key switch | remote mode session opened + call works | PASS | sid=[5OBX4IBBLOEXKBDAG7YJYHBIHF] IsRemoteMCP=true |
| 2 Linear mcp → api_key switch | Configure(api_key) ends remote session | FAIL | err=<nil> DELETE=0 []; probe old sid → HTTP 200; IsRemoteMCP=false |
| 2 Linear mcp → api_key switch | before: session left open after switch | INFO | probe old sid → HTTP 200 (200 = leaked) |
| 2 Linear mcp → api_key switch | api_key native REST path wired (fake key → api.linear.app) | PASS | linear_viewer → IsError=true linear API error (401): {"errors":[{"message":"Authentication required, not authenticated","extensions":{"type":"authentication error","code":"AUTHENTICATION_ERROR","statu |
| 3 Figma base_url change | session on server A + call works | PASS | figma-a sid=[TAMWB2DT7QKRGNTZK3SVYZGALT] |
| 3 Figma base_url change | same base_url + same token re-Configure reuses session | INFO | session_open=0 DELETE=0 |
| 3 Figma base_url change | Configure(base_url=B) ends session on A | FAIL | err=<nil> DELETE on figma-a=0 []; probe old A sid → HTTP 200 |
| 3 Figma base_url change | new session opened on B + call works | PASS | figma-b sid=[PRIIQWIJ7LLVHPWHTI27RHRCID] → {"query":"","session":"PRIIQWIJ7LLVHPWHTI27RHRCID","standin":"figma-b","tool":"whoami"} |
| 4 Slack MCP two identities | two identities → two sessions, calls work on each | PASS | err=<nil> sessions=[UG3F2FP6NNKQR7LZTXXHDAV5AW 44RDJAADU66P7IN3KFY7KY3CB7] work→{"query":"","session":"44RDJAADU66P7IN3KFY7KY3CB7","standin":"slack","tool":"slack_read_ch… personal→{"query":"","sessio |
| 4 Slack MCP two identities | adapter implements io.Closer | FAIL | no Close on main |
| 4 Slack MCP two identities | sessions after host drops instance | INFO | probes → 200, 200 (200 = leaked) |

## Host-level (local Switchboard)

| Build | Step | Result | Note |
|---|---|---|---|
| before | startup: Switchboard configures figma + slackmcp(2 identities) | INFO | sessions opened at startup: figma-a=['SQOYZZOKKBS4367CF6QCCZ7BNA'] slack=['R2TXFLOUH3XHQYDTWZDLLMQ3UV', 'VBMIFBHF2BMFZMQWOHGDKR2ES6'] |
| before | search finds figma_whoami | PASS | {"has_more":false,"integrations":["figma","slackmcp"],"limit":20,"offset":0,"script_hint":"Tip: if your task requires multiple tool calls, use execute with a script to chain them in a single call and  |
| before | search finds slackmcp_read_channel | PASS | {"has_more":false,"integrations":["figma","slackmcp"],"limit":20,"offset":0,"script_hint":"These tools span multiple integrations. Use execute with a script to chain them in a single call — intermedia |
| before | execute figma_whoami via Switchboard → stand-in figma-a | PASS | {"query":"","session":"SQOYZZOKKBS4367CF6QCCZ7BNA","standin":"figma-a","tool":"whoami"}
pinned as $1 |
| before | execute slackmcp_read_channel for work + personal | PASS | work sid=VBMIFBHF2BMFZMQWOHGDKR2ES6 personal sid=R2TXFLOUH3XHQYDTWZDLLMQ3UV |
| before | web Save figma base_url A→B ends old session on A | INFO (leak) | save HTTP 200 → http://127.0.0.1:13856/integrations/figma/setup?success=Configuration+saved; DELETE on figma-a=[]; probe old figma-a sid SQOYZZOKKBS4367CF6QCCZ7BNA → HTTP 200 |
| before | execute figma_whoami after base_url change → figma-b | PASS | {"query":"","session":"JOGATVSCGGGCHIR7MS5VLSG6BP","standin":"figma-b","tool":"whoami"}
pinned as $4 |
| before | slack sessions still alive while instance in use | INFO | probe work → 200, personal → 200 |
| before | Switchboard shutdown (SIGTERM): sessions ended? | INFO | DELETEs on shutdown=[]; probes after exit: work=200 personal=200 |
| after | startup: Switchboard configures figma + slackmcp(2 identities) | INFO | sessions opened at startup: figma-a=['HZFNZQIODJYGLLSFZPVNK62ZPS'] slack=['B43U4GWKN2AIMUIGFR5SSXNKHB', '6AS6473GIFA27KSYLVSONZ4LRL'] |
| after | search finds figma_whoami | PASS | {"has_more":false,"integrations":["slackmcp","figma"],"limit":20,"offset":0,"script_hint":"Tip: if your task requires multiple tool calls, use execute with a script to chain them in a single call and  |
| after | search finds slackmcp_read_channel | PASS | {"has_more":false,"integrations":["figma","slackmcp"],"limit":20,"offset":0,"script_hint":"These tools span multiple integrations. Use execute with a script to chain them in a single call — intermedia |
| after | execute figma_whoami via Switchboard → stand-in figma-a | PASS | {"query":"","session":"HZFNZQIODJYGLLSFZPVNK62ZPS","standin":"figma-a","tool":"whoami"}
pinned as $1 |
| after | execute slackmcp_read_channel for work + personal | PASS | work sid=6AS6473GIFA27KSYLVSONZ4LRL personal sid=B43U4GWKN2AIMUIGFR5SSXNKHB |
| after | web Save figma base_url A→B ends old session on A | PASS | save HTTP 200 → http://127.0.0.1:13855/integrations/figma/setup?success=Configuration+saved; DELETE on figma-a=['HZFNZQIODJYGLLSFZPVNK62ZPS']; probe old figma-a sid HZFNZQIODJYGLLSFZPVNK62ZPS → HTTP 4 |
| after | execute figma_whoami after base_url change → figma-b | PASS | {"query":"","session":"4KFSIXN7MUA2VTHG3NJZGVAHXM","standin":"figma-b","tool":"whoami"}
pinned as $4 |
| after | slack sessions still alive while instance in use | INFO | probe work → 200, personal → 200 |
| after | Switchboard shutdown (SIGTERM): sessions ended? | INFO | DELETEs on shutdown=[]; probes after exit: work=200 personal=200 |
