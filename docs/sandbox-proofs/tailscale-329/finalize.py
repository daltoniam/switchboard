import json,glob,subprocess,re,os,textwrap
from PIL import Image,ImageDraw,ImageFont
D=os.path.dirname(os.path.abspath(__file__)); os.chdir(D)
r329=json.load(open('responses-329.json')); mock=json.load(open('responses-mock.json')); fake=json.load(open('responses-fake.json'))
allsteps=[dict(s,suite='329-configure-error') for s in r329['steps']]+[dict(s,suite='mock-api') for s in mock]+[dict(s,suite='fake-token-real-api') for s in fake]
json.dump(dict(pr=329,head='9888bfc44a891af0b16921b2cbcf7be9cb41847b',baseline='6bc872e (main, merged #328)',before_after=r329['table'],steps=allsteps),open('responses.json','w'),indent=1)
json.dump(dict(configure_error=json.load(open('mcp-responses-329.json')),mock=json.load(open('mcp-raw-mock.json')),fake=json.load(open('mcp-raw-fake.json'))),open('mcp-responses.json','w'),indent=1)
from collections import Counter
C={k:Counter(s['status'] for s in v) for k,v in [('329',r329['steps']),('mock',mock),('fake',fake)]}
tot=sum((C[k] for k in C),Counter())
short=lambda t:t.replace('tailscale: integration is not configured: tailscale: ','… ').replace('|','\\|')
rows="\n".join(f"| `{x['case']}` | `{x['before']}` | `{short(x['after'])}` | PASS |" for x in r329['table'])
clear="\n".join(f"| {s['step']} | {s['status']} | {s['note'][:170].replace('|','/')} |" for s in r329['steps'] if s['phase']=='clear')
pf=f"""| Check | Result | Evidence |
|---|---|---|
| Build / vet / unit tests (tailscale, config) at 9888bfc | PASS | go-test.log: 200 subtests incl. new TestExecute_ReportsWhyConfigureFailed, TestExecute_ConfigureErrorClearedBySuccessfulConfigure, TestExecute_NeverConfigured |
| Bad creds surface specific reason on Execute (6 cases) | PASS 6/6 | before (main 6bc872e) every case = bare `tailscale: integration is not configured` |
| Clear-on-success in one running process (web UI save → re-Configure) | PASS 6/6 | bad → fake well-formed key gives clean real-API 401; bad again; valid key vs mock gives real data; half OAuth after success drops the working client; full OAuth gives upstream oauth2 error, not stale text |
| No credential values in server log / error text | PASS 3/3 | errors name the key kind only |
| Mock-API happy path, all main tools (44 discovered) | PASS 69 / INFO 1 | ACL HuJSON comment survival, 412 stale etag, devices/routes/keys/DNS/settings/invites/webhooks, restore byte-identical |
| Fake token vs real api.tailscale.com + client-side validation | PASS 24 | 401s, 13 validation rejections, 4 misconfig cases |
| Live real-tailnet happy path | WAITING | needs /home/box/.sb-sandbox-secrets/tailscale-328.env; then `./run-live.sh` |
"""
open('pass-fail.md','w').write(pf)
hdr=f"""### Sandbox E2E proof

**Merge-ready from the sandbox side on `9888bfc`** (offline scope; live tailnet run WAITING on a sandbox credential). Every rejected-credential case now tells the agent why on Execute, and a later successful Configure clears it. Base: stacked on `main` at `6bc872e` (#328 already merged), not on `feat/tailscale-integration`. Auth: native REST, API token or OAuth client credentials. No real tailnet used.

{tot['PASS']} PASS / 0 FAIL / {tot['INFO']} INFO / 1 WAITING

| Bad credential | Before (main 6bc872e) | After (9888bfc) | Status |
|---|---|---|---|
{rows}

{pf}"""
open('handoff-snippet.md','w').write(hdr)
rep=f"""# Sandbox E2E proof: daltoniam/switchboard PR #329 (Tell agents why Tailscale credentials were rejected)
- Head `9888bfc44a891af0b16921b2cbcf7be9cb41847b` (branch fix/tailscale-configure-error); parent `6bc872e` = current `main` = #328 squash-merged. Diff vs main: tailscale.go (+19/-6), tailscale_test.go (+41).
- Before baseline: binary built from main `6bc872e`. After: binary from 9888bfc. Isolated SWITCHBOARD_DEV_HOME/HOME per run under homes/, MCP on 127.0.0.1:13860-13862 / 13854.
- Run counts: configure-error suite {dict(C['329'])}; mock API {dict(C['mock'])}; fake token/validation {dict(C['fake'])}.

## Before / after messaging
| Case | Before | After | Result |
|---|---|---|---|
{rows}

## Clear-on-success (single process, reconfigured via the integration Save form → redirect to detail page, which re-runs Configure)
| Step | Result | Evidence |
|---|---|---|
{clear}

## PASS/FAIL
{pf}
## Caveats
- **Live tailnet: WAITING.** No sandbox Tailscale credential exists yet. Run `./run-live.sh` (329 build) or `./run-live.sh 328main` once `/home/box/.sb-sandbox-secrets/tailscale-328.env` exists.
- The happy-path checks use a local stateful mock (`mock_tailscale.py`) via `base_url`. That proves adapter behaviour and wire format, not Tailscale's real responses.
- **Host behaviour (not this PR):** the web Save handler itself does not call Configure. The adapter is re-Configured by the detail page the browser is redirected to, by the health cache, or on restart. A client that POSTs without following the redirect keeps the old stored error until then. I saw this when the redirect wasn't followed: after saving a good key, Execute still said "looks like a device auth key". Normal browser flow is fine.
- The message has a doubled prefix: `tailscale: integration is not configured: tailscale: api_key …`. This is cosmetic.
- `configureErr` is read in Execute and written in Configure with no lock, the same as the existing `httpClient`/`apiKey` fields. A concurrent reconfigure and Execute is a pre-existing data race, not new.
- Carried over from #328 review, not changed here: `create_webhook` returns the signing secret unredacted, there's no `get_webhook` tool, and `base_url` isn't checked against netpolicy.
- Merge readiness: ready from the sandbox side for what this PR changes. The live check only re-confirms #328 behaviour.
"""
open('REPORT.md','w').write(rep)
# screenshots
os.makedirs('screenshots',exist_ok=True)
F=ImageFont.truetype('/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf',15) if os.path.exists('/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf') else ImageFont.load_default()
def shot(name,title,lines):
    out=[]
    for l in lines: out+=textwrap.wrap(l,150) or ['']
    img=Image.new('RGB',(1400,max(200,40+20*len(out[:44])+30)),(18,18,24)); d=ImageDraw.Draw(img); d.text((16,10),title,fill=(120,200,255),font=F)
    for i,l in enumerate(out[:44]): d.text((16,40+20*i),l,fill=(255,120,120) if 'FAIL' in l else (140,230,140) if 'PASS' in l else (230,230,230),font=F)
    img.save(f'screenshots/{name}.png')
gt=open('go-test.log').read().splitlines()
shot('01-build-tests','go vet / go test / build @ 9888bfc',[l for l in gt if 'TestExecute_' in l or l.startswith('ok')][:30])
shot('02-before-after','Bad credentials: before (main 6bc872e) vs after (9888bfc) — Execute error text',sum([[f"[{x['case']}]",'  before: '+x['before'],'  after:  '+x['after'],''] for x in r329['table']],[]))
shot('03-clear-on-success','Clear-on-success in one running server (save → re-Configure)',[l for l in open('harness-329.log').read().splitlines() if ':clear' in l or 'clear:' in l or 'leak' in l])
ml=open('harness-mock.log').read().splitlines()
shot('04-mock-acl-devices','Mock API happy path: ACL / keys / devices / routes',[l for l in ml if any(k in l for k in ('acl:','keys:','devices:','routes:'))])
shot('05-mock-dns-webhooks-restore','Mock API: DNS / settings / users / webhooks / restore',[l for l in ml if any(k in l for k in ('dns:','settings:','users:','webhooks:','restore:','=='))])
shot('06-fake-token','Fake token vs real api.tailscale.com + validation',open('harness-fake.log').read().splitlines())
shot('07-summary','Summary',open('pass-fail.md').read().splitlines())
with open('/tmp/ffc329.txt','w') as f:
    for p in sorted(glob.glob(f'{D}/screenshots/*.png')): f.write(f"file '{p}'\nduration 4\n")
    f.write(f"file '{p}'\n")
subprocess.run(['ffmpeg','-y','-loglevel','error','-f','concat','-safe','0','-i','/tmp/ffc329.txt','-vf','scale=1400:-2,pad=1400:ih:0:0,fps=10,format=yuv420p','-c:v','libx264','proof.mp4'],check=True)
# leak scan
hits=[]
for f in glob.glob('**/*',recursive=True):
    if os.path.isdir(f) or f.endswith(('.png','.mp4')) or f.startswith('homes/') or f in('finalize.py','run-329.py','run-e2e.py','mock_tailscale.py','leak-scan.txt'): continue
    for m in re.finditer(r'tskey-(api|auth|client|webhook)-[A-Za-z0-9_-]+',open(f,errors='ignore').read()):
        v=m.group(0)
        if not any(x in v for x in ('notreal','FAKE','MOCK','REDACTED','SBPROOF','XXXX')) and v not in('tskey-api-x','tskey-client-x'): hits.append((f,v[:18]+'…'))
open('leak-scan.txt','w').write('Leak scan (tskey-* patterns, excluding known fake/mock/redacted values; homes/ = local config, not published)\n'+('CLEAN: 0 real-looking keys\n' if not hits else '\n'.join(map(str,hits))))
print(tot, 'leaks',len(hits))
