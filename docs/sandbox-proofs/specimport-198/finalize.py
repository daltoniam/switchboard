import json,re,glob,subprocess,datetime,collections
from PIL import Image,ImageDraw,ImageFont
D='/workspace/sb-sandbox-proofs/specimport-198'; HEAD='68343a87962ee3a88e73db7044b0ee823f5e7062'
NOW=datetime.datetime.now().strftime('%Y-%m-%d %H:%M CDT')
steps=json.load(open(f'{D}/responses.json'))
for s in steps:
    if 'via script api.tryCall' in s['step']:
        s['step']=s['step'].replace('(bypasses 50KB render cap)','(script path)'); s['note']='script api.tryCall hits the same 50KB cap — the generated selection set itself is too large'
    if s['step'].startswith('search bad_') or s['step'] in('search petstore_yaml','search probe_mapped'):
        s['phase']='validation'; s['note']='no tools registered for this import (rejected at load) — '+s['note']
log=open(f'{D}/switchboard-a.log').read()
val=[]
for n,why in [('bad_loopback_v4','http://127.0.0.1:18999'),('bad_localhost','http://localhost:18999'),('bad_loopback_v6','http://[::1]:18999'),('probe_mapped','http://[::ffff:127.0.0.1]:18999'),('bad_metadata','http://169.254.169.254/latest (OpenAPI)'),('bad_metadata_gql','http://169.254.169.254/graphql (GraphQL)'),('bad_gcp_meta','http://metadata.google.internal/x'),('bad_basecred','credentials.base_url=http://127.0.0.1:18999 override'),('bad_scheme','file:///etc/passwd')]:
    m=re.search(r'spec import \\"%s\\": (.*?)"\n'%n,log); ok=bool(m) and ('not allowed' in m.group(1) or 'http or https' in m.group(1))
    val.append(dict(phase='validation',step=f'reject {why}',tool='config spec_imports load',status='PASS' if ok else 'FAIL',note=(m.group(1).replace('\\"','"') if m else 'not rejected')))
m=re.search(r'spec import \\"petstore_yaml\\": (.*?)"\n',log)
val.append(dict(phase='validation',step='YAML spec via path (upstream openapi.yaml as-is)',tool='config spec_imports load',status='INFO',note='rejected: '+m.group(1).replace('\\"','"')+' — JSON only; converted YAML->JSON for the proof'))
val.append(dict(phase='validation',step='reject http://0.0.0.0:18999',tool='config + probe_unspecified_getinventory',status='FAIL',note='ACCEPTED at load and the execute reached a local canary on :18999 (canary log: GET /store/inventory from 127.0.0.1). netpolicy.HostDisallowed has no IsUnspecified() check'))
val.append(dict(phase='validation',step='hostname that resolves to loopback (127.0.0.1.nip.io)',tool='config + probe_dns_loopback_getinventory',status='INFO',note='accepted at load (host check is string-only, no DNS resolution / dial-time check, redirects not re-validated); box DNS maps it to 198.18.0.1 so the live hit could not be shown here'))
build=[dict(phase='build',step='go vet ./specimport ./netpolicy ./config ./web',tool='go',status='PASS',note='exit 0'),
 dict(phase='build',step='go test ./specimport ./netpolicy ./config ./web ./cmd/server',tool='go',status='PASS',note='all 5 packages ok'),
 dict(phase='build',step='go build ./cmd/server',tool='go',status='PASS',note='dist/switchboard built at 68343a8')]
steps=build+[s for s in steps if s['phase']!='validation']+[s for s in steps if s['phase']=='validation']+val
json.dump(steps,open(f'{D}/responses.json','w'),indent=1,ensure_ascii=False)
cnt=collections.Counter(s['status'] for s in steps)
def short(x): return str(x).replace('|','/').replace('\n',' ')[:260]
TOOLS=[
('Build / unit tests','PASS','go vet, go test (specimport, netpolicy, config, web, cmd/server), go build at 68343a8'),
('search (discovery)','PASS','19 `petstore_*` tools from upstream Petstore OpenAPI 3; 6 `countries_*` from live introspection; 6 `countries_fixture_*` from testdata; `[write]` tag on POST/PUT/DELETE; rejected imports register no tools'),
('Petstore read: findPetsByStatus / getPetById','PASS','live petstore3: available + sold lists; pet 1; 404 surfaced cleanly; missing required `petId` rejected before any request'),
('Petstore write: addPet → get → updatePetWithForm → deletePet','PASS*','JSON body POST, form/query update, delete all worked live. *Integer path/query args ≥1e6 go out as floats (`2.8989396e+08`) → upstream 400; worked only when the id was passed as a string'),
('Petstore getInventory','INFO','petstore3 returns 500 for /store/inventory even with direct curl (upstream problem); error surfaced cleanly'),
('Countries: country (US, JP), countries filter `code in [DE,FR]`, fixture BR','PASS','correct live data; `filter` input objects passed as GraphQL variables'),
('Countries: continents / continent / language / countries `continent=OC`','FAIL','`Response exceeded 50KB` (65–539KB). The generated default selection set goes 3 levels deep through cycles (continent→countries→languages→countries…), so list-ish queries can\'t be used. Script `api.tryCall` hits the same cap'),
('execute script chaining across both imports','PASS','`countries_country` + `petstore_getpetbyid` in one script → `{"country":"Canada","pet":"Pet1"}`'),
('Loopback / metadata base URL rejected','PASS','127.0.0.1, localhost, [::1], [::ffff:127.0.0.1], 169.254.169.254 (OpenAPI + GraphQL), metadata.google.internal, `credentials.base_url` loopback override, and file:// all refused at load with a clear error'),
('0.0.0.0 base URL','FAIL','accepted, and execute reached a local canary on :18999. `HostDisallowed` needs `ip.IsUnspecified()`'),
('DNS / redirect SSRF','INFO','host check is string-only: no resolve-time or dial-time check, redirects not re-checked. 127.0.0.1.nip.io accepted at load (live hit not shown, because box DNS maps it to 198.18.0.1)'),
('YAML spec','INFO','The upstream Petstore `openapi.yaml` is refused (`parse openapi json`). JSON only, so the proof converted it to JSON'),
]
tbl='| Tool / check | Result | Evidence |\n|---|---|---|\n'+'\n'.join(f'| {a} | {b} | {c} |' for a,b,c in TOOLS)
merge=f'**Not merge-ready on `68343a8` from the sandbox side:** OpenAPI import works against live Petstore, but a 0.0.0.0 base URL gets past the SSRF guard to loopback, and generated GraphQL list queries are over 50KB. A large-integer path-param bug is also open.'
cline=f"MCP E2E + build: {cnt['PASS']} PASS / {cnt['FAIL']} FAIL / {cnt['INFO']} INFO"
other='''Other notes (minor):
- `search` lists every enabled `spec_imports` entry under `integrations`, including ones that failed to load (`EnabledIntegrations` reads config, not the registry). No tools are registered for them.
- Only the server log shows load errors. An agent sees the integration name but no reason it's missing.
- `api.tryCall` on a GraphQL tool returns `{"data":{"data":…}}` (double-wrapped).
- `[write]` GraphQL mutations weren't exercised (Countries has none).'''
snippet=f'''### Sandbox E2E proof

{merge} Head `{HEAD}`. Native HTTP (OpenAPI REST + GraphQL POST), no auth (public APIs). Live upstreams: petstore3.swagger.io, countries.trevorblades.com.

{cline}

{tbl}

{other}

Artifacts: `sandbox-e2e-proofs` → `docs/sandbox-proofs/specimport-198/` (REPORT.md, pass-fail.md, proof.mp4, responses.json, mcp-responses.json, specs/, screenshots/).
'''
open(f'{D}/handoff-snippet.md','w').write(snippet)
allrows='\n'.join(f"| {s['phase']} | {short(s['step'])} | `{s['tool']}` | {s['status']} | {short(s['note'])} |" for s in steps)
open(f'{D}/pass-fail.md','w').write(f'## Per-tool summary\n\n{tbl}\n\n## All steps\n\n| Phase | Step | Tool | Result | Note |\n|---|---|---|---|---|\n{allrows}\n')
open(f'{D}/REPORT.md','w').write(f'''# Sandbox E2E proof: daltoniam/switchboard PR #198 (OpenAPI/GraphQL spec import)

- Head: `{HEAD}` (branch `feat/spec-imports`, matches the expected 68343a8)
- Run: {NOW}, Linux box, isolated `SWITCHBOARD_DEV_HOME`, Switchboard on 127.0.0.1:13853, driven over MCP (`search` / `execute`)
- Config: `spec_imports` entries in config.json using `path` + `endpoint` (no fetch-from-URL in the PR)
  - `petstore`: openapi, upstream swagger-petstore `openapi.yaml` converted to JSON (`specs/petstore-openapi.json`), endpoint `https://petstore3.swagger.io/api/v3`
  - `countries`: graphql, live introspection of countries.trevorblades.com (`specs/countries-introspection.json`)
  - `countries_fixture`: graphql, `testdata/specimport/countries_graphql.json`
  - 12 negative/probe entries (loopback, metadata, scheme, YAML, 0.0.0.0, DNS)
- Auth: none. Both APIs are public, and no secrets were used.

## Verdict

{merge}

{cline}

{tbl}

## Bugs / findings

1. **SSRF: `0.0.0.0` accepted (FAIL).** `netpolicy.HostDisallowed` checks Loopback/Private/LinkLocal but not `IsUnspecified()`. On Linux, 0.0.0.0 dials localhost. The canary on :18999 logged `GET /store/inventory` from Switchboard. Fix: add `ip.IsUnspecified()` (and consider blocking `0.0.0.0/8`, CGNAT `100.64/10`).
2. **SSRF: string-only host check (INFO, design).** Hostnames that resolve to private IPs (e.g. `127.0.0.1.nip.io`, or DNS rebinding) and 3xx redirects to internal hosts aren't checked. Recommend a `net.Dialer.Control` check on the resolved IP plus `CheckRedirect` re-validation.
3. **GraphQL default selection too large (FAIL).** `selectionBuilder` recurses 3 levels through cyclic types. `countries_continents` comes back at 539KB, and continent/language/countries(filter) at 64–68KB, all over Switchboard's 50KB response cap. Fix: depth 1–2, skip list-of-object fields at nested levels, or allow a caller-supplied `fields`/selection.
4. **Integer path/query args formatted as floats (FAIL, partial).** `fmt.Sprint(v)` on a JSON float64 makes `289893960` into `2.8989396e+08`, so getPetById/updatePetWithForm/deletePet get a 400 for any id ≥ 1e6. Fix: format whole floats with `strconv.FormatFloat(v,'f',-1,64)` or decode with `UseNumber`. Passing the id as a string works around it.
5. **YAML not supported (INFO).** The upstream Petstore spec is YAML, and the parser is JSON-only. Either accept YAML or document "JSON only" in the UI.
6. {other.split(chr(10),1)[1].replace(chr(10)+'- ',chr(10)+'   - ')}

## Petstore cleanup
The test pet `sb-proof-198` (id in responses.json) was created, updated, then deleted on the public petstore3 demo. Final delete returned `Pet deleted`.

## Files
REPORT.md, pass-fail.md, handoff-snippet.md, responses.json (per step), mcp-responses.json (raw MCP results), harness-run.log, go-test.log, go-vet.log, build.log, switchboard-a.log, specs/, screenshots/, proof.mp4, run-e2e.py, mcpc.py, finalize.py, leak-scan.txt
''')
font=ImageFont.truetype('/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf',16); bold=ImageFont.truetype('/usr/share/fonts/truetype/dejavu/DejaVuSansMono-Bold.ttf',22)
def clean(s): return re.sub(r'[^\x00-\u024f]','',s)
def slide(name,title,lines):
    img=Image.new('RGB',(1280,720),(24,26,33)); d=ImageDraw.Draw(img)
    d.text((30,20),title,font=bold,fill=(120,200,255)); d.text((30,52),f'Switchboard PR #198 (spec import) · head {HEAD[:10]} · {NOW} · live upstreams',font=font,fill=(150,150,150)); y=92
    for ln in lines:
        ln=clean(ln); col=(110,220,120) if 'PASS' in ln else (240,90,90) if 'FAIL' in ln else (240,200,90) if 'INFO' in ln else (220,220,220)
        while ln and y<700: d.text((30,y),ln[:124],font=font,fill=col); ln=ln[124:]; y+=21
    img.save(f'{D}/screenshots/{name}.png')
def sl(ph,pre=''): return [f"[{s['status']}] {s['step']}: {short(s['note'])}" for s in steps if s['phase']==ph and s['step'].startswith(pre)]
slide('01-build-tests','1. Build, vet, unit tests',sl('build')+['']+open(f'{D}/go-test.log').read().splitlines())
slide('02-config','2. spec_imports config (path + endpoint)',['petstore  openapi  specs/petstore-openapi.json  -> https://petstore3.swagger.io/api/v3','countries graphql  specs/countries-introspection.json (live introspection) -> https://countries.trevorblades.com','countries_fixture graphql testdata/specimport/countries_graphql.json','+ 12 negative entries: loopback / metadata / scheme / YAML / 0.0.0.0 / DNS',''])
slide('03-search','3. search: generated tools',sl('search'))
slide('04-petstore','4. Petstore (live petstore3.swagger.io)',sl('petstore')+sl('script'))
slide('05-countries','5. Countries GraphQL (live)',sl('countries')+sl('countries_fixture'))
slide('06-validation','6. Base URL validation / SSRF',[x for x in sl('validation') if not x.startswith('[PASS] search')]+sl('ssrf-probe'))
slide('07-summary','Summary',[cline,'']+[f'{b:6} {a}' for a,b,c in TOOLS]+['',re.sub(r'\*\*|`','',merge)])
shots=sorted(glob.glob(f'{D}/screenshots/*.png'))
with open('/tmp/ffc.txt','w') as fh:
    for s in shots: fh.write(f"file '{s}'\nduration 4\n")
    fh.write(f"file '{shots[-1]}'\n")
subprocess.run(['ffmpeg','-y','-loglevel','error','-f','concat','-safe','0','-i','/tmp/ffc.txt','-vf','fps=10,format=yuv420p','-c:v','libx264',f'{D}/proof.mp4'],check=True)
tok=re.compile(r'gh[pousr]_[A-Za-z0-9]{30,}|github_pat_\w{30,}|xox[bpa]-[\w-]{10,}|sk-[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16}|Bearer [A-Za-z0-9._-]{20,}')
hits=[]
for f in glob.glob(f'{D}/**/*',recursive=True):
    if f.endswith(('.png','.mp4')) or '/homes/' in f: continue
    try: t=open(f,errors='ignore').read()
    except Exception: continue
    for m in tok.finditer(t): hits.append(f'{f}: {m.group(0)[:12]}…')
    if '/home/box/.sb-sandbox-secrets' in t or 'dalton' in t.lower() and f.endswith('.log'): hits.append(f'{f}: personal path/name')
open(f'{D}/leak-scan.txt','w').write(f'{NOW}: scanned {D} (excluding homes/, binaries) for tokens/bearer/secret paths\n'+('\n'.join(hits) or 'CLEAN')+'\n')
print(cline); print(open(f'{D}/leak-scan.txt').read())
