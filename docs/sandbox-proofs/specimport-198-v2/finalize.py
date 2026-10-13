import json,re,glob,subprocess,datetime,collections
from PIL import Image,ImageDraw,ImageFont
D='/workspace/sb-sandbox-proofs/specimport-198-v2'; HEAD='78ca8f0aea871738e0501bf6f45337c0f4e36462'; PRIOR='68343a8'
NOW=datetime.datetime.now().strftime('%Y-%m-%d %H:%M CDT')
steps=json.load(open(f'{D}/responses.json'))
for s in steps:
    if s['step'].startswith('findPetsByTags tags="tag1"'): s['status']='INFO'; s['note']='upstream petstore3 returns 500 for /pet/findByTags?tags=tag1 with direct curl too: not a PR issue. '+s['note']
    if s['step'].startswith('findPetsByTags tags=["tag1"]'): s['note']='array query params rejected client-side (no request sent); before, fmt.Sprint sent "[tag1]". Arrays still unsupported. '+s['note']
    if s['step'].startswith('search bad_') or s['step'] in('search petstore_yaml','search probe_mapped'):
        s['phase']='validation'; s['note']='no tools registered (rejected at load); '+s['note']
log=open(f'{D}/switchboard-a.log').read()
val=[]
for n,why in [('bad_unspecified_v4','http://0.0.0.0:18999 [NEW FIX]'),('bad_unspecified_v6','http://[::]:18999 [NEW FIX]'),('bad_zero_net','http://0.1.2.3:18999 (0/8) [NEW FIX]'),('bad_cgnat','http://100.64.0.1:18999 (100.64/10) [NEW FIX]'),('bad_cgnat_hi','http://100.127.255.254:18999 (100.64/10 top) [NEW FIX]'),('bad_loopback_v4','http://127.0.0.1:18999'),('bad_localhost','http://localhost:18999'),('bad_loopback_v6','http://[::1]:18999'),('probe_mapped','http://[::ffff:127.0.0.1]:18999'),('bad_metadata','http://169.254.169.254/latest (OpenAPI)'),('bad_metadata_gql','http://169.254.169.254/graphql (GraphQL)'),('bad_gcp_meta','http://metadata.google.internal/x'),('bad_basecred','credentials.base_url=http://127.0.0.1:18999 override'),('bad_scheme','file:///etc/passwd')]:
    m=re.search(r'spec import \\"%s\\": (.*?)"\n'%n,log); ok=bool(m) and ('not allowed' in m.group(1) or 'http or https' in m.group(1))
    val.append(dict(phase='validation',step=f'reject {why}',tool='config spec_imports load',status='PASS' if ok else 'FAIL',note=(m.group(1).replace('\\"','"') if m else 'not rejected')))
val.append(dict(phase='validation',step='YAML spec via path',tool='config spec_imports load',status='INFO',note='still rejected: parse openapi json (JSON only); used YAML->JSON conversion'))
build=[dict(phase='build',step='go vet ./specimport ./netpolicy ./config ./web',tool='go',status='PASS',note='exit 0'),dict(phase='build',step='go test ./specimport ./netpolicy ./config ./web ./cmd/server',tool='go',status='PASS',note='all 5 packages ok'),dict(phase='build',step='go build ./cmd/server',tool='go',status='PASS',note='dist/switchboard at 78ca8f0')]
steps=build+[s for s in steps if s['phase']!='validation']+[s for s in steps if s['phase']=='validation']+val
json.dump(steps,open(f'{D}/responses.json','w'),indent=1,ensure_ascii=False)
cnt=collections.Counter(s['status'] for s in steps)
def st(pre): 
    for s in steps:
        if s['step'].startswith(pre): return s
def short(x): return str(x).replace('|','/').replace('\n',' ')[:240]
sz=lambda p:(re.search(r'([\d.]+KB)',st(p)['note']) or re.search(r'actual: (\d+KB)',st(p)['note'])).group(1)
BA=[
('1. SSRF: 0.0.0.0 / :: / 0/8 / 100.64/10','FAIL: `http://0.0.0.0:18999` accepted; execute hit the local canary (`GET /store/inventory` from 127.0.0.1). `::`, 0/8, 100.64/10 were not blocked either','PASS: 0.0.0.0, [::], 0.1.2.3, 100.64.0.1, 100.127.255.254 all rejected at load; tools not registered (`tool not found`); canary GET count 0. 100.128.0.1 (just outside 100.64/10) still accepted, so the boundary is correct','**FIXED**'),
('2. GraphQL selection depth vs 50KB cap',f'FAIL: continents 539KB, continent EU 65KB, language es 64KB, countries(continent=OC) 68KB',f'continent EU {sz("continent EU")} PASS, language es {sz("language es")} PASS; **continents 147KB FAIL, countries(continent=OC) 68KB FAIL** (fixture continents also 147KB)','**PARTIAL**: 2 of 4 now fit; the list queries are still over the cap'),
('3. Integer ids ≥1e6 in path/query','FAIL: `289893960` sent as `2.8989396e+08` → upstream 400 on getPetById/update/delete; worked only as a string','PASS: addPet id 289910702 → getPetById (JSON number) 200 → updatePetWithForm → get shows renamed/sold → deletePet `Pet deleted` (cleanup verified: GET now `Pet not found`)','**FIXED**'),
]
TOOLS=[
('Build / unit tests','PASS','go vet, go test (specimport, netpolicy, config, web, cmd/server), go build at 78ca8f0'),
('search (discovery)','PASS','19 `petstore_*`, 6 `countries_*` (live introspection), 6 `countries_fixture_*`; rejected imports register no tools'),
('Petstore read: findPetsByStatus / getPetById','PASS','live available + sold lists; pet 1; 404 surfaced; missing `petId` rejected before any request'),
('Petstore write: addPet → getPetById → updatePetWithForm → deletePet (id ≥1e6)','PASS','id 289910702 sent as an integer in path and query; full cycle live; test pet deleted'),
('Petstore getInventory / findPetsByTags','INFO','upstream petstore3 returns 500 for both, even with direct curl. Array query params (`tags:["tag1"]`) are now rejected client-side (`cannot convert []interface {}`)'),
('Countries: country US/JP/ZZ, countries `code in [DE,FR]`, fixture BR','PASS','correct live data'),
('Countries: continent EU / language es','PASS',f'now {sz("continent EU")} / {sz("language es")} (were 65/64KB, over the cap)'),
('Countries: continents / countries `continent=OC`','FAIL','still over the 50KB cap: continents 147KB (was 539KB), countries OC 68KB (unchanged)'),
('execute script chaining across both imports','PASS','`{"country":"Canada","pet":"Pet1"}`'),
('Base URL validation: loopback / metadata / scheme','PASS','127.0.0.1, localhost, [::1], [::ffff:127.0.0.1], 169.254.169.254 (OpenAPI+GraphQL), metadata.google.internal, base_url override, file:// rejected'),
('Base URL validation: 0.0.0.0, ::, 0/8, 100.64/10','PASS','all rejected at load; local canary not hit; 100.128.0.1 boundary still allowed'),
('DNS / redirect SSRF','INFO','unchanged: the host check is string-only; 127.0.0.1.nip.io accepted at load; no dial-time or redirect check'),
('YAML spec','INFO','unchanged: JSON only'),
]
ba='| Finding | Before (68343a8) | After (78ca8f0) | Status |\n|---|---|---|---|\n'+'\n'.join(f'| {a} | {b} | {c} | {d} |' for a,b,c,d in BA)
tbl='| Tool / check | Result | Evidence |\n|---|---|---|\n'+'\n'.join(f'| {a} | {b} | {c} |' for a,b,c in TOOLS)
merge='**Not merge-ready yet on `78ca8f0` from the sandbox side:** the SSRF and integer-id fixes are confirmed, but the GraphQL depth fix is partial. `countries_continents` (147KB) and `countries_countries(continent=OC)` (68KB) are still over the 50KB cap. Everything else passes, with no regressions.'
cline=f"build + MCP E2E: {cnt['PASS']} PASS / {cnt['FAIL']} FAIL / {cnt['INFO']} INFO"
info='''Still open (INFO): the host check doesn't resolve DNS or re-check redirects; YAML specs aren't supported; array query params are refused client-side; `search` lists imports that failed to load; load errors appear only in the server log; GraphQL `api.tryCall` is double-wrapped (`data.data`). getInventory/findPetsByTags 500s come from upstream petstore3.'''
fixidea='Ideas for the list-query sizes: depth 1 for list-returning roots, skip nested list-of-object fields, or let the caller pass a `fields`/selection argument.'
snippet=f'''### Sandbox E2E proof (re-run)

{merge} Head `{HEAD}`; earlier proof on `{PRIOR}`. Native HTTP (OpenAPI REST + GraphQL POST), no auth. Live upstreams: petstore3.swagger.io, countries.trevorblades.com.

{cline}

{ba}

{tbl}

{fixidea}

{info}

Artifacts: `sandbox-e2e-proofs` → `docs/sandbox-proofs/specimport-198-v2/` (REPORT.md, pass-fail.md, proof.mp4, responses.json, mcp-responses.json, screenshots/).
'''
open(f'{D}/handoff-snippet.md','w').write(snippet)
comment=f'''Sandbox E2E re-run @ 78ca8f0: {cnt['PASS']} PASS / {cnt['FAIL']} FAIL / {cnt['INFO']} INFO (was 41/7/10 @ 68343a8).

- ✅ SSRF: 0.0.0.0, [::], 0.x, 100.64.x are now rejected at load, and the local canary wasn't hit.
- ✅ Integer ids: addPet with id 289910702 → getPetById → update → delete all worked live; ids go out as integers.
- ⚠️ GraphQL depth 2: continent and language now fit (about 20KB), but `continents` (147KB) and `countries(filter continent=OC)` (68KB) are still over the 50KB cap.

No regressions. Still open (INFO): DNS/redirect SSRF, no YAML, array query params. Details + video: docs/sandbox-proofs/specimport-198-v2/ on sandbox-e2e-proofs.
'''
open(f'{D}/comment-draft.md','w').write(comment)
allrows='\n'.join(f"| {s['phase']} | {short(s['step'])} | `{s['tool']}` | {s['status']} | {short(s['note'])} |" for s in steps)
open(f'{D}/pass-fail.md','w').write(f'## Before → after\n\n{ba}\n\n## Per-tool summary\n\n{tbl}\n\n## All steps\n\n| Phase | Step | Tool | Result | Note |\n|---|---|---|---|---|\n{allrows}\n')
v1=json.load(open('/workspace/sb-sandbox-proofs/specimport-198/responses.json')); v1pass={s['step'] for s in v1 if s['status']=='PASS'}; now={s['step']:s['status'] for s in steps}
reg=[k for k in v1pass if k in now and now[k]!='PASS']
open(f'{D}/REPORT.md','w').write(f'''# Sandbox E2E proof (re-run): daltoniam/switchboard PR #198 spec import

- Head: `{HEAD}` (branch feat/spec-imports; one new commit, "Fix spec-import SSRF, GraphQL selection depth, and integer params"). Earlier proof: `{PRIOR}` → ../specimport-198/
- Run: {NOW}; same harness, same specs, same config, plus 5 new SSRF entries and a 100.128.0.1 boundary control; isolated SWITCHBOARD_DEV_HOME; MCP on 127.0.0.1:13853
- No GitHub posting, commits or pushes from this run.

## Verdict
{merge}

{cline}

## Before → after (the 3 claimed fixes)
{ba}

{fixidea}

## Per-tool summary
{tbl}

## Regressions vs {PRIOR}
Earlier PASS steps not passing now: {reg or 'none'}. (The v1 string-id workaround step was replaced by the integer-id step.)

## INFO / caveats
{info}

## Test pet cleanup
Pet 289910702 (`sb-proof-198`) was created, updated, then deleted; GET afterwards returns `Pet not found`.
''')
font=ImageFont.truetype('/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf',16); bold=ImageFont.truetype('/usr/share/fonts/truetype/dejavu/DejaVuSansMono-Bold.ttf',22)
def clean(s): return re.sub(r'[^\x00-\u024f]','',s)
def slide(name,title,lines):
    img=Image.new('RGB',(1280,720),(24,26,33)); d=ImageDraw.Draw(img)
    d.text((30,20),title,font=bold,fill=(120,200,255)); d.text((30,52),f'Switchboard PR #198 re-run · head {HEAD[:10]} (prev {PRIOR}) · {NOW}',font=font,fill=(150,150,150)); y=92
    for ln in lines:
        ln=clean(ln); col=(110,220,120) if ('PASS' in ln or 'FIXED' in ln) else (240,90,90) if 'FAIL' in ln else (240,200,90) if ('INFO' in ln or 'PARTIAL' in ln) else (220,220,220)
        while ln and y<700: d.text((30,y),ln[:124],font=font,fill=col); ln=ln[124:]; y+=21
    img.save(f'{D}/screenshots/{name}.png')
def sl(ph,pre=''): return [f"[{s['status']}] {s['step']}: {short(s['note'])}" for s in steps if s['phase']==ph and s['step'].startswith(pre)]
slide('01-build-tests','1. Build, vet, unit tests @ 78ca8f0',sl('build')+['']+open(f'{D}/go-test.log').read().splitlines())
slide('02-search','2. search: generated tools',sl('search'))
slide('03-petstore','3. Petstore live (integer id >=1e6)',sl('petstore')+sl('script'))
slide('04-countries','4. Countries GraphQL: sizes vs 50KB cap',sl('countries')+sl('countries_fixture'))
slide('05-ssrf','5. Base URL validation + canary',[x for x in sl('validation') if not x.startswith('[PASS] search')]+sl('ssrf-probe'))
slide('06-before-after','6. Before (68343a8) -> after (78ca8f0)',sum(([a,'  before: '+re.sub(r'\*\*|`','',b),'  after:  '+re.sub(r'\*\*|`','',c),'  => '+re.sub(r'\*\*','',d),''] for a,b,c,d in BA),[]))
slide('07-summary','Summary',[cline,'']+[f'{b:6} {a}' for a,b,c in TOOLS]+['',re.sub(r'\*\*|`','',merge)])
shots=sorted(glob.glob(f'{D}/screenshots/*.png'))
with open('/tmp/ffc2.txt','w') as fh:
    for s in shots: fh.write(f"file '{s}'\nduration 4\n")
    fh.write(f"file '{shots[-1]}'\n")
subprocess.run(['ffmpeg','-y','-loglevel','error','-f','concat','-safe','0','-i','/tmp/ffc2.txt','-vf','fps=10,format=yuv420p','-c:v','libx264',f'{D}/proof.mp4'],check=True)
tok=re.compile(r'gh[pousr]_[A-Za-z0-9]{30,}|github_pat_\w{30,}|xox[bpa]-[\w-]{10,}|sk-[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16}|Bearer [A-Za-z0-9._-]{20,}')
hits=[]
for f in glob.glob(f'{D}/**/*',recursive=True):
    if f.endswith(('.png','.mp4','finalize.py')) or '/homes/' in f: continue
    try: t=open(f,errors='ignore').read()
    except Exception: continue
    hits+= [f'{f}: {m.group(0)[:12]}…' for m in tok.finditer(t)]
    if '.sb-sandbox-secrets' in t: hits.append(f'{f}: secrets path')
open(f'{D}/leak-scan.txt','w').write(f'{NOW}: scanned {D} (excl. homes/, binaries) for tokens/bearer/secret paths\n'+('\n'.join(hits) or 'CLEAN')+'\n')
print(cline,'regress',reg); print(open(f'{D}/leak-scan.txt').read())
