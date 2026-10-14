import json,re,glob,subprocess,datetime,collections
from PIL import Image,ImageDraw,ImageFont
D='/workspace/sb-sandbox-proofs/specimport-198-v3'; HEAD='d39e116ae221e3ea39156192ff8d2d2c9f79a48d'; PRIOR='78ca8f0'
NOW=datetime.datetime.now().strftime('%Y-%m-%d %H:%M CDT')
steps=json.load(open(f'{D}/responses.json'))
for s in steps:
    if s['step'].startswith('findPetsByTags tags=["tag1"]'): s['note']='array query params rejected client-side (no request sent); before, fmt.Sprint sent "[tag1]". Arrays still unsupported. '+s['note']
    if s['step'].startswith('search bad_') or s['step'] in('search petstore_yaml','search probe_mapped'):
        s['phase']='validation'; s['note']='no tools registered (rejected at load); '+s['note']
log=open(f'{D}/switchboard-a.log').read()
val=[]
for n,why in [('bad_unspecified_v4','http://0.0.0.0:18999 [NEW FIX]'),('bad_unspecified_v6','http://[::]:18999 [NEW FIX]'),('bad_zero_net','http://0.1.2.3:18999 (0/8) [NEW FIX]'),('bad_cgnat','http://100.64.0.1:18999 (100.64/10) [NEW FIX]'),('bad_cgnat_hi','http://100.127.255.254:18999 (100.64/10 top) [NEW FIX]'),('bad_loopback_v4','http://127.0.0.1:18999'),('bad_localhost','http://localhost:18999'),('bad_loopback_v6','http://[::1]:18999'),('probe_mapped','http://[::ffff:127.0.0.1]:18999'),('bad_metadata','http://169.254.169.254/latest (OpenAPI)'),('bad_metadata_gql','http://169.254.169.254/graphql (GraphQL)'),('bad_gcp_meta','http://metadata.google.internal/x'),('bad_basecred','credentials.base_url=http://127.0.0.1:18999 override'),('bad_scheme','file:///etc/passwd')]:
    m=re.search(r'spec import \\"%s\\": (.*?)"\n'%n,log); ok=bool(m) and ('not allowed' in m.group(1) or 'http or https' in m.group(1))
    val.append(dict(phase='validation',step=f'reject {why}',tool='config spec_imports load',status='PASS' if ok else 'FAIL',note=(m.group(1).replace('\\"','"') if m else 'not rejected')))
val.append(dict(phase='validation',step='YAML spec via path',tool='config spec_imports load',status='INFO',note='still rejected: parse openapi json (JSON only); used YAML->JSON conversion'))
build=[dict(phase='build',step='go vet ./specimport ./netpolicy ./config ./web',tool='go',status='PASS',note='exit 0'),dict(phase='build',step='go test ./specimport ./netpolicy ./config ./web ./cmd/server',tool='go',status='PASS',note='all 5 packages ok'),dict(phase='build',step='go build ./cmd/server',tool='go',status='PASS',note='dist/switchboard at d39e116')]
steps=build+[s for s in steps if s['phase']!='validation']+[s for s in steps if s['phase']=='validation']+val
_seen=set(); steps=[x for x in steps if not ((x['phase'],x['step']) in _seen or _seen.add((x['phase'],x['step'])))]
json.dump(steps,open(f'{D}/responses.json','w'),indent=1,ensure_ascii=False)
cnt=collections.Counter(s['status'] for s in steps)
def st(pre): 
    for s in steps:
        if s['step'].startswith(pre): return s
def short(x): return str(x).replace('|','/').replace('\n',' ')[:240]
sz=lambda p:(re.search(r'([\d.]+KB)',st(p)['note']) or re.search(r'actual: (\d+KB)',st(p)['note'])).group(1)
BA=[
('countries_continents','FAIL: 147KB (539KB at 68343a8)',f'PASS: {sz("continents")}, all 7 continents with code + name','**FIXED**'),
('countries_countries(filter continent=OC)','FAIL: 68KB',f'PASS: {sz("countries filter continent=OC")}, Oceania countries with scalar fields','**FIXED**'),
('countries_fixture_continents (testdata spec)','FAIL: 147KB',f'PASS: {sz("fixture continents")}','**FIXED**'),
('Selections still useful','continent EU 19.5KB incl. nested countries; country included continent + languages','continent EU → `{code:"EU",name:"Europe"}`; language es → code/name/native/rtl; country US/JP/BR → scalars + `continent{code,name}`. Nested list-of-object fields (continent.countries, country.languages/states/subdivisions) are no longer selected','PASS (trade-off: INFO)'),
]
TOOLS=[
('Build / unit tests','PASS','go vet, go test (specimport, netpolicy, config, web, cmd/server), go build at d39e116'),
('search (discovery)','PASS','19 `petstore_*`, 6 `countries_*` (live introspection), 6 `countries_fixture_*`; rejected imports register no tools'),
('Petstore read: findPetsByStatus / getPetById / findPetsByTags (string)','PASS','live lists; pet 1; 404 surfaced; missing `petId` rejected before any request'),
('Petstore write: addPet → getPetById → updatePetWithForm → deletePet (id ≥1e6)','PASS','id 289951065 sent as an integer in path and query; full cycle live; test pet deleted (GET → `Pet not found`)'),
('Petstore getInventory','INFO','upstream petstore3 returns 500 even with direct curl'),
('Countries single lookups: country US/JP/ZZ, fixture BR, continent EU, language es, countries `code in [DE,FR]`','PASS','correct live data; country includes `continent{code,name}`'),
('Countries lists: continents / countries `continent=OC` / fixture continents','PASS',f'{sz("continents")} / {sz("countries filter continent=OC")} / {sz("fixture continents")}, all under the 50KB cap (were 147 / 68 / 147KB)'),
('execute script chaining across both imports','PASS','`{"country":"Canada","pet":"Pet1"}`'),
('Base URL validation: loopback / metadata / scheme','PASS','127.0.0.1, localhost, [::1], [::ffff:127.0.0.1], 169.254.169.254 (OpenAPI+GraphQL), metadata.google.internal, base_url override, file:// rejected'),
('Base URL validation: 0.0.0.0, ::, 0/8, 100.64/10','PASS','all rejected at load; local canary not hit (0 requests); 100.128.0.1 boundary still allowed'),
('DNS / redirect SSRF','INFO','unchanged: the host check is string-only; 127.0.0.1.nip.io accepted at load; no dial-time or redirect check'),
('YAML spec / array query params','INFO','unchanged: JSON specs only; `tags:["tag1"]` rejected client-side'),
]
ba='| Finding | Before (78ca8f0) | After (d39e116) | Status |\n|---|---|---|---|\n'+'\n'.join(f'| {a} | {b} | {c} | {d} |' for a,b,c,d in BA)
tbl='| Tool / check | Result | Evidence |\n|---|---|---|\n'+'\n'.join(f'| {a} | {b} | {c} |' for a,b,c in TOOLS)
merge='**Merge-ready on `d39e116` from the sandbox side:** all 3 GraphQL list queries that failed at 78ca8f0 are now well under the 50KB cap, and SSRF, integer ids, Petstore CRUD, lookups and chaining all still pass, with no regressions. The open items are INFO only.'
cline=f"build + MCP E2E: {cnt['PASS']} PASS / {cnt['FAIL']} FAIL / {cnt['INFO']} INFO"
info='''Still open (INFO): the host check doesn't resolve DNS or re-check redirects; YAML specs aren't supported; array query params are refused client-side; `search` lists imports that failed to load; load errors appear only in the server log; GraphQL `api.tryCall` is double-wrapped (`data.data`). The getInventory 500 comes from upstream petstore3.'''
fixidea='Trade-off (INFO): nested lists of objects are now never selected, so `continent` no longer returns its countries and `country` no longer returns languages/states/subdivisions. Getting those takes a second call, e.g. `countries(filter:{continent:{eq:"EU"}})`. A future option is a caller-supplied `fields`/selection argument.'
snippet=f'''### Sandbox E2E proof (re-run 2)

{merge} Head `{HEAD}`; earlier proof on `{PRIOR}`. Native HTTP (OpenAPI REST + GraphQL POST), no auth. Live upstreams: petstore3.swagger.io, countries.trevorblades.com.

{cline}

{ba}

{tbl}

{fixidea}

{info}

Artifacts: `sandbox-e2e-proofs` → `docs/sandbox-proofs/specimport-198-v3/` (REPORT.md, pass-fail.md, proof.mp4, responses.json, mcp-responses.json, screenshots/).
'''
open(f'{D}/handoff-snippet.md','w').write(snippet)
comment=f'''Sandbox E2E re-run @ d39e116: {cnt['PASS']} PASS / {cnt['FAIL']} FAIL / {cnt['INFO']} INFO (was 58/3/8 @ 78ca8f0).

- ✅ GraphQL list queries are now under the 50KB cap: continents {sz("continents")} (was 147KB), countries(continent=OC) {sz("countries filter continent=OC")} (was 68KB), fixture continents {sz("fixture continents")}. Single lookups still return useful data (continent EU → code + name; country → scalars + continent).
- ✅ No regressions: SSRF (0.0.0.0/::/0.x/100.64 rejected, canary not hit), integer ids ≥1e6 live, Petstore CRUD, script chaining.

Merge-ready from the sandbox side. INFO: nested object lists (continent.countries, country.languages) are no longer selected; DNS/redirect SSRF; no YAML; array query params. Details + video: docs/sandbox-proofs/specimport-198-v3/ on sandbox-e2e-proofs.
'''
open(f'{D}/comment-draft.md','w').write(comment)
allrows='\n'.join(f"| {s['phase']} | {short(s['step'])} | `{s['tool']}` | {s['status']} | {short(s['note'])} |" for s in steps)
open(f'{D}/pass-fail.md','w').write(f'## Before → after\n\n{ba}\n\n## Per-tool summary\n\n{tbl}\n\n## All steps\n\n| Phase | Step | Tool | Result | Note |\n|---|---|---|---|---|\n{allrows}\n')
v1=json.load(open('/workspace/sb-sandbox-proofs/specimport-198-v2/responses.json')); v1pass={s['step'] for s in v1 if s['status']=='PASS'}; now={s['step']:s['status'] for s in steps}
reg=[k for k in v1pass if k in now and now[k]!='PASS']
open(f'{D}/REPORT.md','w').write(f'''# Sandbox E2E proof (re-run 2): daltoniam/switchboard PR #198 spec import

- Head: `{HEAD}` (branch feat/spec-imports; one new commit, "Shrink GraphQL default selections for list-returning fields"). Earlier proof: `{PRIOR}` → ../specimport-198-v2/
- Run: {NOW}; same harness, same specs, same config, identical to v2; isolated SWITCHBOARD_DEV_HOME; MCP on 127.0.0.1:13853
- No GitHub posting, commits or pushes from this run.

## Verdict
{merge}

{cline}

## Before → after (the 3 v2 FAILs + usefulness)
{ba}

{fixidea}

## Per-tool summary
{tbl}

## Regressions vs {PRIOR}
Earlier PASS steps not passing now: {reg or 'none'}. 

## INFO / caveats
{info}

## Test pet cleanup
Pet 289951065 (`sb-proof-198`) was created, updated, then deleted; GET afterwards returns `Pet not found`.
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
slide('01-build-tests','1. Build, vet, unit tests @ d39e116',sl('build')+['']+open(f'{D}/go-test.log').read().splitlines())
slide('02-search','2. search: generated tools',sl('search'))
slide('03-petstore','3. Petstore live (integer id >=1e6)',sl('petstore')+sl('script'))
slide('04-countries','4. Countries GraphQL: sizes vs 50KB cap',sl('countries')+sl('countries_fixture'))
slide('05-ssrf','5. Base URL validation + canary',[x for x in sl('validation') if not x.startswith('[PASS] search')]+sl('ssrf-probe'))
slide('06-before-after','6. Before (78ca8f0) -> after (d39e116)',sum(([a,'  before: '+re.sub(r'\*\*|`','',b),'  after:  '+re.sub(r'\*\*|`','',c),'  => '+re.sub(r'\*\*','',d),''] for a,b,c,d in BA),[]))
slide('07-summary','Summary',[cline,'']+[f'{b:6} {a}' for a,b,c in TOOLS]+['',re.sub(r'\*\*|`','',merge)])
shots=sorted(glob.glob(f'{D}/screenshots/*.png'))
with open('/tmp/ffc3.txt','w') as fh:
    for s in shots: fh.write(f"file '{s}'\nduration 4\n")
    fh.write(f"file '{shots[-1]}'\n")
subprocess.run(['ffmpeg','-y','-loglevel','error','-f','concat','-safe','0','-i','/tmp/ffc3.txt','-vf','fps=10,format=yuv420p','-c:v','libx264',f'{D}/proof.mp4'],check=True)
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
