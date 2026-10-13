import json, time
from mcpc import MCPClient, text
c=MCPClient('http://127.0.0.1:13853/mcp'); c.init()
steps=[]; log=[]
def js(t):
    try: return json.loads(t)
    except Exception:
        try: return json.JSONDecoder().raw_decode(t.strip())[0]
        except Exception: return t
def run(phase, step, tool, args, check, via='execute'):
    try:
        r = c.call('execute',{'tool_name':tool,'arguments':args}) if via=='execute' else c.call(tool,args)
        t=text(r); err=r.get('isError',False)
    except Exception as e:
        r={'exception':str(e)}; t=str(e); err=True
    ok,note=check(js(t),t,err)
    st={True:'PASS',False:'FAIL',None:'INFO'}[ok]
    steps.append(dict(phase=phase,step=step,tool=tool,args=args,status=st,note=note,response=js(t)[:3000] if isinstance(js(t),str) else js(t)))
    log.append(dict(phase=phase,step=step,tool=tool,args=args,via=via,raw=r))
    print(f'[{st}] {phase}:{step} {tool} — {note}',flush=True)
def names(o):
    tl=o.get('tools',{}); 
    if isinstance(tl,dict) and 'rows' in tl:
        i=tl['columns'].index('name'); return [r[i] for r in tl['rows']]
    return [x.get('name') for x in tl] if isinstance(tl,list) else []
def S(q,expect,limit=50,absent=None):
    def ck(o,t,e):
        n=names(o) if isinstance(o,dict) else []
        miss=[x for x in expect if x not in n]; bad=[x for x in (absent or []) if any(y.startswith(x) for y in n)]
        return (not miss and not bad), f'{len(n)} tools; missing={miss}; unexpected={bad}' 
    run('search',f'search {q}','search',{'query':q,'limit':limit},ck,via='direct')
PET=['petstore_findpetsbystatus','petstore_getpetbyid','petstore_getinventory','petstore_addpet','petstore_updatepet','petstore_deletepet']
CTY=['countries_country','countries_countries','countries_continents','countries_continent','countries_language','countries_languages']
S('petstore',PET); S('countries',CTY); S('countries_fixture',[x.replace('countries_','countries_fixture_') for x in CTY])
S('petstore pet status',['petstore_findpetsbystatus'],limit=10)
for b in ['bad_loopback_v4','bad_localhost','bad_loopback_v6','bad_metadata','bad_metadata_gql','bad_basecred','bad_gcp_meta','bad_scheme','petstore_yaml','probe_mapped']:
    S(b,[],limit=50,absent=[b+'_'])
# Petstore
def lst(o,t,e):
    if e or not isinstance(o,(list,dict)): return False,t[:200]
    return True, f'type={type(o).__name__} len={len(o)} sample={json.dumps(o)[:180]}'
run('petstore','findPetsByStatus available','petstore_findpetsbystatus',{'status':'available'},lst)
run('petstore','findPetsByStatus sold','petstore_findpetsbystatus',{'status':'sold'},lst)
run('petstore','getPetById 1','petstore_getpetbyid',{'petId':1},lambda o,t,e:(not e and isinstance(o,dict) and o.get('id')==1, json.dumps(o)[:200]))
run('petstore','getInventory','petstore_getinventory',{},lambda o,t,e:((True if not e and isinstance(o,dict) and len(o)>0 else (None if '500' in t else False)), ('upstream petstore3 itself returns 500 for /store/inventory (curl direct: 500 x3) — not a PR issue; error surfaced cleanly: ' if '500' in t else '')+t[:200]))
pid=int(time.time())%100000000+198000000
pet={'id':pid,'name':'sb-proof-198','status':'pending','photoUrls':['https://example.com/p.png'],'category':{'id':1,'name':'Dogs'},'tags':[{'id':198,'name':'sandbox-e2e'}]}
run('petstore','addPet','petstore_addpet',{'body':pet},lambda o,t,e:(not e and isinstance(o,dict) and o.get('id')==pid and o.get('name')=='sb-proof-198', json.dumps(o)[:200]))
run('petstore','getPetById (new pet)','petstore_getpetbyid',{'petId':pid},lambda o,t,e:(not e and isinstance(o,dict) and o.get('id')==pid, json.dumps(o)[:200]))
run('petstore','getPetById (new pet, id as string — workaround)','petstore_getpetbyid',{'petId':str(pid)},lambda o,t,e:(not e and isinstance(o,dict) and o.get('id')==pid, t[:200]))
run('petstore','updatePetWithForm (query params)','petstore_updatepetwithform',{'petId':str(pid),'name':'sb-proof-198-renamed','status':'sold'},lambda o,t,e:(False if e else True, t[:200]))
run('petstore','getPetById after update','petstore_getpetbyid',{'petId':str(pid)},lambda o,t,e:(not e and 'sb-proof-198-renamed' in t and 'sold' in t, t[:200]))
run('petstore','deletePet (cleanup)','petstore_deletepet',{'petId':str(pid)},lambda o,t,e:(False if e else True, t[:200]))
run('petstore','getPetById 0 (upstream 4xx)','petstore_getpetbyid',{'petId':-1},lambda o,t,e:(e, t[:200]))
run('petstore','missing required arg','petstore_getpetbyid',{},lambda o,t,e:(e, t[:200]))
# Countries
run('countries','country US','countries_country',{'code':'US'},lambda o,t,e:(not e and 'United States' in t, t[:220]))
run('countries','country JP','countries_country',{'code':'JP'},lambda o,t,e:(not e and 'Japan' in t, t[:220]))
run('countries','countries filter continent=OC','countries_countries',{'filter':{'continent':{'eq':'OC'}}},lambda o,t,e:(not e and 'Australia' in t and 'Japan' not in t, t[:220]))
run('countries','countries filter code in [DE,FR]','countries_countries',{'filter':{'code':{'in':['DE','FR']}}},lambda o,t,e:(not e and 'Germany' in t and 'France' in t and 'Spain' not in t, t[:220]))
run('countries','continents','countries_continents',{},lambda o,t,e:(not e and all(x in t for x in ['Africa','Antarctica','Asia','Europe','North America','Oceania','South America']), t[:220]))
run('countries','continent EU','countries_continent',{'code':'EU'},lambda o,t,e:(not e and 'Europe' in t, t[:220]))
run('countries','language es','countries_language',{'code':'es'},lambda o,t,e:(not e and 'Spanish' in t, t[:220]))
run('countries','country unknown ZZ','countries_country',{'code':'ZZ'},lambda o,t,e:(None, t[:220]))
run('countries_fixture','fixture country BR','countries_fixture_country',{'code':'BR'},lambda o,t,e:(not e and 'Brazil' in t, t[:220]))
run('countries_fixture','fixture continents','countries_fixture_continents',{},lambda o,t,e:(not e and 'Oceania' in t, t[:220]))
for tl,a,w in [('countries_continents',{},'Oceania'),('countries_continent',{'code':'EU'},'Europe'),('countries_language',{'code':'es'},'Spanish'),('countries_countries',{'filter':{'continent':{'eq':'OC'}}},'Australia')]:
    r=c.call('execute',{'script':f'api.tryCall({json.dumps(tl)},{json.dumps(a)})'}); t=text(r); ok=(w in t) and not r.get('isError')
    steps.append(dict(phase='countries',step=f'{tl} via script api.tryCall (bypasses 50KB render cap)',tool=tl,args=a,status='INFO',note=f'{"upstream data correct" if ok else "failed"}; raw size={len(t)//1024}KB; '+t[:160],response=t[:3000])); log.append(dict(step=tl+' tryCall',raw={'size':len(t),'head':t[:5000]})); print('[INFO tryCall]',tl,ok,len(t))
# script chaining
r=c.call('execute',{'script':'const c=api.call("countries_country",{code:"CA"}); const p=api.call("petstore_getpetbyid",{petId:1}); ({country:c.data.country.name, pet:p.name});'})
t=text(r); ok=('Canada' in t) and not r.get('isError')
steps.append(dict(phase='script',step='execute script chaining both imports',tool='execute(script)',args={},status='PASS' if ok else 'FAIL',note=t[:200],response=js(t))); log.append(dict(step='script',raw=r)); print('[script]',ok,t[:200])
# probes reaching canary
for p in ['probe_unspecified','probe_dns_loopback']:
    run('ssrf-probe',f'{p} getinventory reaches local canary?',f'{p}_getinventory',{},lambda o,t,e:(None, t[:200]))
json.dump(steps,open('responses.json','w'),indent=1,ensure_ascii=False); json.dump(log,open('mcp-responses.json','w'),indent=1,ensure_ascii=False)
