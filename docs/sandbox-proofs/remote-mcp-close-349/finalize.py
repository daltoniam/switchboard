import json,subprocess,datetime,collections,re,glob,os
from PIL import Image,ImageDraw,ImageFont
D='/workspace/sb-sandbox-proofs/remote-mcp-close-349'; os.chdir(D)
HEAD=open('logs/head.txt').read().strip(); MAIN='5b82ef2'
NOW=datetime.datetime.now().strftime('%Y-%m-%d %H:%M CDT')
A=json.load(open('harness-after.json')); B=json.load(open('harness-before.json')); H=json.load(open('responses-host.json'))
ci=[l.split() for l in open('logs/ci-summary.txt') if 'rc=' in l]
CI=[
('`make ci` → styles / proto-check / test-ui','SKIPPED','`make` not installed on the box (no root); Node 26.8.1 pinned (box has v20), no `buf`, no Docker/Playwright. PR touches no web/proto files'),
('build (`go build ./cmd/server`)','PASS','logs/ci-build.log'),
('vet (`go vet ./...`)','PASS','logs/ci-vet.log'),
('test-race (`go test -race ./...`)','PASS','94 packages ok, 0 FAIL (logs/ci-test-race-all.log)'),
('`scripts/compose-dev_test.sh`','PASS','logs/ci-compose-dev-test.log'),
('lint (`go tool golangci-lint run`)','PASS','0 issues (logs/ci-lint.log)'),
('gosec (Makefile flags)','PASS','logs/ci-gosec.log'),
('govulncheck','SKIPPED','process killed on the box twice (~2-4 min); go.mod/go.sum unchanged by this PR'),
('test-web','SKIPPED','only test is .ts; needs Node ≥22 (box v20)'),
('New tests: TestRemoteMCPAdapters_ImplementClose + 6 adapter Close tests','PASS','7/7, also under -race'),
('Guard test catches the bug (run against main 5b82ef2)','PASS','fails on main naming figma, linear, slackmcp (logs/guard-test-on-main.log)'),
]
def get(chk,sc,step): 
    for c in chk['checks']:
        if c['scenario'].startswith(sc) and c['step'].startswith(step): return c
    return None
def hget(ver,step):
    for s in H['steps']:
        if s['build']==ver and s['step'].startswith(step): return s
def bres(sc,step):
    c=get(B,sc,step); return c['result'] if c else '—'
SC=[]
def row(name,sc,step,before_sc=None,before_step=None,before_text=None):
    c=get(A,sc,step); b=before_text or (bres(before_sc or sc,before_step or step))
    SC.append((name,c['result'],b,c['evidence'].replace('|','/')[:230]))
for ad,sc,srv in [('Linear','1 Linear','linear'),('Figma','1 Figma','figma-a'),('Slack MCP','1 Slack','slack')]:
    row(f'{ad}: Configure → session opened → tool call',sc,'Configure →')
    row(f'{ad}: Close() → DELETE, old session 404 server-side',sc,'Close() →',before_text='FAIL: no io.Closer; probe on dropped instance → 200 (leaked)')
    row(f'{ad}: second Close() no-op (no request, no error)',sc,'second Close',before_text='n/a')
    row(f'{ad}: re-Configure → fresh session id → tool call',sc,'re-Configure',before_text='n/a')
row('Linear mcp_access_token → api_key ends remote session','2 Linear','Configure(api_key)',before_text='FAIL: 0 DELETE, old session → 200')
row('Linear api_key native REST still wired (fake key → api.linear.app 401)','2 Linear','api_key native',before_text='PASS')
row('Figma base_url A → B ends session on A','3 Figma','Configure(base_url=B)',before_text='FAIL: 0 DELETE, A session → 200')
row('Figma new session on B + call works','3 Figma','new session',before_text='PASS')
row('Slack MCP two identities → two sessions, calls on each','4 Slack','two identities',before_text='PASS')
row('Slack MCP Close() ends both sessions','4 Slack','Close() ends both',before_text='FAIL: no io.Closer; both → 200 (leaked)')
row('Slack MCP second Close() no-op','4 Slack','second Close',before_text='n/a')
row('Slack MCP Healthy=false after Close','4 Slack','Healthy',before_text='n/a')
hs=hget('after','web Save figma'); hb=hget('before','web Save figma')
HOST=[(s['step'],s['status'],s['note'].replace('|','/')[:200]) for s in H['steps'] if s['build']=='after']
SC.append(('Host: local Switchboard search/execute → figma_whoami, slackmcp_read_channel (2 identities)','PASS','PASS','PR build on 127.0.0.1:13855, base_url → stand-in; see responses-host.json'))
SC.append(('Host: web Save figma base_url A→B ends A session','PASS' if hs['status']=='PASS' else 'FAIL','INFO: 0 DELETE, A → 200 (leaked)',hs['note'].replace('|','/')[:200]))
cnt=collections.Counter([r[1] for r in SC]+[r[1] for r in CI])
ac=collections.Counter(c['result'] for c in A['checks']); bc=collections.Counter(c['result'] for c in B['checks'])
leftA=sum(len(v) for v in A['final_session_table']['live'].values()); leftB=sum(len(v) for v in B['final_session_table']['live'].values())
tbl='| Scenario | After (`'+HEAD[:7]+'`) | Before (main `'+MAIN+'`) | Evidence (after) |\n|---|---|---|---|\n'+'\n'.join(f'| {a} | {b} | {c} | {d} |' for a,b,c,d in SC)
citbl='| CI step | Result | Note |\n|---|---|---|\n'+'\n'.join(f'| {a} | {b} | {c} |' for a,b,c in CI)
cline=f"{cnt['PASS']} PASS / {cnt['FAIL']} FAIL / {cnt['SKIPPED']} SKIPPED (scenarios + CI). Adapter harness: after {ac['PASS']} PASS / {ac['FAIL']} FAIL; main {bc['PASS']} PASS / {bc['FAIL']} FAIL. Sessions still open on the stand-in at the end: after {leftA}, main {leftB}."
merge=f'**Merge-ready from the sandbox side at `{HEAD[:7]}`:** all three adapters now end their remote MCP session on Close(), Close is idempotent, re-Configure starts a fresh session, and the Linear api_key switch and Figma base_url change end the old session. On main the same runs leak every session. No FAILs.'
STANDIN='**Stand-in, not the real vendors.** No Linear/Figma/Slack hosted-MCP OAuth was used. Every session open/DELETE in this proof is against a local streamable-HTTP MCP server built on the official MCP Go SDK v1.7.0 (the same SDK and version `remotemcp` uses), mounted at `/linear`, `/figma-a`, `/figma-b`, `/slack` on 127.0.0.1:18349. It issues `Mcp-Session-Id`, serves tools/list and tool calls, logs every request, and answers 404 on deleted sessions. Linear native REST was checked against the real api.linear.app with a fake key (clean 401).'
CAV=[
'Stand-in server (see above). Real hosted servers may run stateless, with no `Mcp-Session-Id`; then the SDK client has nothing to DELETE and Close only drops local state. That is fine, but this proof can\'t show what Linear/Figma/Slack do server-side.',
'Linear\'s MCP URL is hard-coded in `cmd/server/main.go` (`https://mcp.linear.app`), so Linear could not be pointed at the stand-in through a running Switchboard. Linear was driven through `linear.New(<stand-in>)` in a scratch harness module (`harness/`, outside the PR tree, `replace` → worktree). Figma and Slack MCP were also run through a real local Switchboard via `base_url`.',
'The local Switchboard host never calls `Close()`: not on shutdown, not when dropping an instance. After SIGTERM the Slack sessions were still live on the stand-in (both builds). This PR is for hosts that do call Close (Switchboard Hosted per-request pools); the local server only benefits through the in-Configure replacement paths (Figma base_url, Linear api_key). Possible follow-up: close integrations on graceful shutdown.',
'Linear after Close() (without a re-Configure) falls through to the native GraphQL path, because `useRemote=false` and `apiKey=""`. A stray Execute then sends an unauthenticated request to api.linear.app and gets back a 401. Figma returns "integration not configured" and Slack returns "unknown identity". Minor, and only on use-after-Close; a nil check returning `ErrNotConfigured` would make it consistent.',
'Linear and Figma `Close`/`closeRemote` take no lock, the same as their existing unlocked fields. A concurrent Close and Execute is a pre-existing race pattern, not new; `remotemcp.remote.Close` itself is mutex-guarded. Slack MCP locks `cfgMu`+`mu`.',
'`make ci` could not run as a whole: no `make` binary or root on the box, Node 26.8.1 is pinned (box has v20), and there is no buf/Docker/Playwright. I ran the Go recipe commands directly: build, vet, `go test -race ./...`, compose-dev test, golangci-lint and gosec all PASS. govulncheck was killed on the box; go.mod is unchanged by this PR.',
]
cav='\n'.join('- '+c for c in CAV)
snippet=f'''### Sandbox E2E proof

{merge}

Head `{HEAD}` (branch `fix/remote-mcp-integrations-close`), compared with main `{MAIN}`. {cline}

{STANDIN}

{tbl}

{citbl}

Caveats:
{cav}

Artifacts: `sandbox-e2e-proofs` → `docs/sandbox-proofs/remote-mcp-close-349/` (REPORT.md, pass-fail.md, proof.mp4, responses.json, logs/standin-*.jsonl, screenshots/).
'''
open('handoff-snippet.md','w').write(snippet)
open('comment-draft.md','w').write(f'''Sandbox E2E @ {HEAD[:7]}: {cnt['PASS']} PASS / {cnt['FAIL']} FAIL / {cnt['SKIPPED']} SKIPPED. Stand-in streamable-HTTP MCP server (official Go SDK v1.7.0), not the hosted vendors.

- ✅ Linear, Figma and Slack MCP: Close() sends DELETE, the old session gets 404 server-side, a second Close is a no-op, and re-Configure opens a fresh session id that works.
- ✅ Linear mcp_access_token → api_key ends the remote session; api_key REST still wired (fake key → api.linear.app 401).
- ✅ Figma base_url A→B ends the A session, both in the harness and via the local Switchboard web Save. Slack MCP Close ends both identities' sessions.
- ✅ On main {MAIN}, the same runs leave {leftB} sessions open (0 DELETEs). The guard test fails on main naming all three adapters.
- ✅ go build/vet, `go test -race ./...`, golangci-lint and gosec pass. Not run on the box: full `make ci` (no make, Node 26, buf, Docker), govulncheck.

Merge-ready from the sandbox side. Notes: the local server never calls Close (not even on shutdown); Linear Execute after Close hits api.linear.app unauthenticated (401) instead of "not configured". Details and video: docs/sandbox-proofs/remote-mcp-close-349/ on sandbox-e2e-proofs.
''')
allrows=lambda chk:'\n'.join(f"| {c['scenario']} | {c['step']} | {c['result']} | {c['evidence'].replace('|','/')[:200]} |" for c in chk['checks'])
open('pass-fail.md','w').write(f"## Scenario summary\n\n{tbl}\n\n## CI\n\n{citbl}\n\n## Adapter harness — after ({HEAD[:7]})\n\n| Scenario | Step | Result | Evidence |\n|---|---|---|---|\n{allrows(A)}\n\n## Adapter harness — before (main {MAIN})\n\n| Scenario | Step | Result | Evidence |\n|---|---|---|---|\n{allrows(B)}\n\n## Host-level (local Switchboard)\n\n| Build | Step | Result | Note |\n|---|---|---|---|\n"+'\n'.join(f"| {s['build']} | {s['step']} | {s['status']} | {s['note'].replace('|','/')[:200]} |" for s in H['steps'])+'\n')
json.dump(dict(head=HEAD,main=MAIN,generated=NOW,ci=[dict(step=a,result=b,note=c) for a,b,c in CI],scenarios=[dict(scenario=a,after=b,before=c,evidence=d) for a,b,c,d in SC],harness_after=A,harness_before=B,host=H),open('responses.json','w'),indent=1,ensure_ascii=False)
diffstat=open('logs/diff-stat.txt').read()
open('REPORT.md','w').write(f'''# Sandbox E2E proof: daltoniam/switchboard PR #349 (Let hosts end remote MCP sessions for Linear, Figma, and Slack MCP)

- Head `{HEAD}` (branch `fix/remote-mcp-integrations-close`, matches the expected c118818). Merge base / before baseline: main `{MAIN}`. Fresh detached worktree `/workspace/sb-remote-mcp-349`; main worktree `/workspace/sb-remote-mcp-main`.
- Host runs: isolated `HOME`/`SWITCHBOARD_DEV_HOME` under `homes/`, PR build on 127.0.0.1:13855, main on 13856. Stand-in MCP on 127.0.0.1:18349.
- Generated {NOW}.

{merge}

{cline}

## What's real and what's a stand-in
{STANDIN}

## Diff
```
{diffstat}```

## Scenarios (PASS/FAIL, after vs before)
{tbl}

## CI
{citbl}

## How it was run
- `harness/standin/main.go`: the stand-in MCP server. It writes JSONL of every HTTP request (method, path, session in/out, JSON-RPC method, tool, status, sha256 fingerprint of the bearer, kind=`session_open`/`request`/`session_delete`/`rejected_deleted_session`).
- `harness/run/main.go`: the adapter-level scenarios, using the real `remotemcp` + SDK client over HTTP. Built against the PR (`go.mod`) and main (`go.main.mod`). After each `Close()` it sends a direct `tools/list` with the old `Mcp-Session-Id` (marked `probe`) to show the server-side state: 404 = ended, 200 = leaked.
- `run-host.py` + `start-sb.sh`: a real local Switchboard (PR vs main) with figma and slackmcp (2 identities) pointed at the stand-in. Drives MCP `search`/`execute`, then changes the Figma `base_url` through the web Save form (the redirect re-runs Configure), then SIGTERMs the server.
- Logs: `logs/standin-after.jsonl`, `logs/standin-before.jsonl`, `logs/standin-host.jsonl`, `logs/harness-*.log`, `logs/host-run.log`, `logs/ci-*.log`, `logs/guard-test-on-main.log`.

## Caveats
{cav}

## Merge readiness
Ready from the sandbox side. No FAILs on the PR head. Every leak shown on main is closed by the PR. Open items are INFO only: the local host never calls Close, and Linear's use-after-Close falls back to REST.
''')
# screenshots
font=ImageFont.truetype('/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf',15); bold=ImageFont.truetype('/usr/share/fonts/truetype/dejavu/DejaVuSansMono-Bold.ttf',22)
def card(fn,title,lines):
    img=Image.new('RGB',(1280,720),(24,26,33)); d=ImageDraw.Draw(img)
    d.text((30,20),title,font=bold,fill=(120,200,255)); d.text((30,52),f'Switchboard PR #349 · head {HEAD[:10]} vs main {MAIN} · stand-in MCP (official Go SDK) · {NOW}',font=font,fill=(150,150,150)); y=88
    for ln in lines:
        col=(230,200,110) if ln.startswith(('Merge-ready','30 PASS')) else (110,220,130) if 'PASS' in ln or '404' in ln else (240,110,110) if 'FAIL' in ln or 'leak' in ln.lower() else (220,220,220)
        while y<700:
            d.text((30,y),ln[:132],font=font,fill=col); y+=19; ln=ln[132:]
            if not ln: break
    img.save(f'screenshots/{fn}')
card('01-diff-ci.png','1. PR diff + CI',open('logs/diff-stat.txt').read().splitlines()+['']+[f'{a}: {b}' for a,b,c in CI])
def hl(f,pat): return [l.rstrip() for l in open(f) if re.search(pat,l)]
card('02-lifecycle-after.png','2. Adapter lifecycle (PR head): Configure → call → Close → 404 → Close no-op → fresh session',[l.replace('[after] ','') for l in hl('logs/harness-after.log',r'^\[after\] .*\| 1 |^\[after\] (PASS|INFO) 1')])
card('03-switch-after.png','3. Linear api_key switch, Figma base_url change, Slack 2 identities (PR head)',[l.replace('[after] ','') for l in hl('logs/harness-after.log',r'^\[after\] \S+ [234] |PASS=')])
card('04-before-main.png','4. Same harness on main 5b82ef2: sessions leak',[l.replace('[before] ','') for l in hl('logs/harness-before.log',r'.')]+['',f'live sessions left on stand-in: main={leftB}, PR={leftA}'])
ev=[json.loads(l) for l in open('logs/standin-after.jsonl')]
card('05-standin-log.png','5. Stand-in server log (PR run): session opens, DELETEs, 404 probes',[f"#{e['seq']:>3} {e['server']:<8} {e['http_method']:<6} {e['kind']:<24} rpc={e.get('rpc','') or '-':<26} sid={(e.get('session_out') or e.get('session_in') or '-')[:26]:<27} {e['status']}{' (probe)' if e.get('probe') else ''}" for e in ev if e['kind']!='request' or e.get('probe')][:31])
card('06-host.png','6. Local Switchboard (search/execute + web Save figma base_url)',[l for l in open('logs/host-run.log').read().splitlines() if l.startswith('[')])
card('07-summary.png','7. Summary',[merge.replace('**',''),'',cline,'']+[f'{a[:70]:<70} {b:<6} before: {c[:50]}' for a,b,c,d in SC])
shots=sorted(glob.glob('screenshots/*.png'))
with open('/tmp/ffc349.txt','w') as f:
    for s in shots: f.write(f"file '{D}/{s}'\nduration 5\n")
    f.write(f"file '{D}/{shots[-1]}'\n")
subprocess.run(['ffmpeg','-y','-loglevel','error','-f','concat','-safe','0','-i','/tmp/ffc349.txt','-vf','fps=10,format=yuv420p','-c:v','libx264','proof.mp4'],check=True)
# leak scan
pats=[r'lin_api_(?!sandboxFAKE)[A-Za-z0-9]{20,}',r'xox[abposr]-[A-Za-z0-9-]{10,}',r'figd_[A-Za-z0-9_-]{10,}',r'ghp_[A-Za-z0-9]{20,}',r'github_pat_',r'tskey-[a-z]+-[A-Za-z0-9]{10,}',r'-----BEGIN [A-Z ]*PRIVATE KEY',r'\.sb-sandbox-secrets/[^ ]*\.env',r'AKIA[0-9A-Z]{16}']
hits=[]
for p in glob.glob('**/*',recursive=True):
    if p.startswith('homes/') or not os.path.isfile(p) or p.endswith(('.mp4','.png')) or 'leak-scan' in p or p=='finalize.py': continue
    try: t=open(p,errors='ignore').read()
    except Exception: continue
    for pat in pats:
        for m in re.finditer(pat,t): hits.append(f'{p}: {pat} -> {m.group(0)[:40]}')
open('leak-scan.txt','w').write(f'{NOW}: scanned {D} (excl. homes/, mp4/png) for Linear/Slack/Figma/GitHub/Tailscale/AWS tokens, private keys, secret-file paths.\nOnly credentials used: stand-in placeholder tokens (standin-*) and the fake Linear key lin_api_sandboxFAKEkey0… (intentionally fake, excluded).\n'+('\n'.join(hits) or 'CLEAN')+'\n')
print(cline); print(dict(cnt)); print(open('leak-scan.txt').read())
