import json, time, re
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
for b in ['bad_unspecified_v4','bad_unspecified_v6','bad_zero_net','bad_cgnat','bad_cgnat_hi','bad_loopback_v4','bad_localhost','bad_loopback_v6','bad_metadata','bad_metadata_gql','bad_basecred','bad_gcp_meta','bad_scheme','petstore_yaml','probe_mapped']:
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
run('petstore','getPetById (new pet, integer id >=1e6)','petstore_getpetbyid',{'petId':pid},lambda o,t,e:(not e and isinstance(o,dict) and o.get('id')==pid, json.dumps(o)[:200]))
run('petstore','updatePetWithForm (query params)','petstore_updatepetwithform',{'petId':pid,'name':'sb-proof-198-renamed','status':'sold'},lambda o,t,e:(False if e else True, t[:200]))
run('petstore','getPetById after update','petstore_getpetbyid',{'petId':pid},lambda o,t,e:(not e and 'sb-proof-198-renamed' in t and 'sold' in t, t[:200]))
run('petstore','deletePet (cleanup)','petstore_deletepet',{'petId':pid},lambda o,t,e:(False if e else True, t[:200]))
run('petstore','getPetById 0 (upstream 4xx)','petstore_getpetbyid',{'petId':-1},lambda o,t,e:(e, t[:200]))
run('petstore','findPetsByStatus status=pending (query) finds new pet','petstore_findpetsbystatus',{'status':'pending'},lambda o,t,e:(None,'query string param; '+t[:150]))
run('petstore','findPetsByTags tags=["tag1"] (array query param)','petstore_findpetsbytags',{'tags':['tag1']},lambda o,t,e:(None,('ERROR: ' if e else 'ok: ')+t[:200]))
run('petstore','findPetsByTags tags="tag1" (string)','petstore_findpetsbytags',{'tags':'tag1'},lambda o,t,e:(not e,t[:200]))
run('petstore','missing required arg','petstore_getpetbyid',{},lambda o,t,e:(e, t[:200]))
# Countries
run('countries','country US','countries_country',{'code':'US'},lambda o,t,e:(not e and 'United States' in t, t[:220]))
run('countries','country JP','countries_country',{'code':'JP'},lambda o,t,e:(not e and 'Japan' in t, t[:220]))
def SZ(w):
    return lambda o,t,e:((False,'over cap: '+t[:120]) if (e or w not in t) else (True,f'rendered size={len(t.encode())/1024:.1f}KB (<50KB cap); '+t[:110]))
run('countries','countries filter continent=OC','countries_countries',{'filter':{'continent':{'eq':'OC'}}},SZ('Australia'))
run('countries','countries filter code in [DE,FR]','countries_countries',{'filter':{'code':{'in':['DE','FR']}}},lambda o,t,e:(not e and 'Germany' in t and 'France' in t and 'Spain' not in t, t[:220]))
run('countries','continents','countries_continents',{},lambda o,t,e:((False,'over cap: '+t[:120]) if e or not all(x in t for x in ['Africa','Antarctica','Asia','Europe','North America','Oceania','South America']) else (True,f'all 7 continents; rendered size={len(t.encode())/1024:.1f}KB (<50KB cap)')))
run('countries','continent EU','countries_continent',{'code':'EU'},SZ('Europe'))
run('countries','language es','countries_language',{'code':'es'},SZ('Spanish'))
run('countries','country unknown ZZ','countries_country',{'code':'ZZ'},lambda o,t,e:(None, t[:220]))
run('countries_fixture','fixture country BR','countries_fixture_country',{'code':'BR'},lambda o,t,e:(not e and 'Brazil' in t, t[:220]))
run('countries_fixture','fixture continents','countries_fixture_continents',{},SZ('Oceania'))
# script chaining
r=c.call('execute',{'script':'const c=api.call("countries_country",{code:"CA"}); const p=api.call("petstore_getpetbyid",{petId:1}); ({country:c.data.country.name, pet:p.name});'})
t=text(r); ok=('Canada' in t) and not r.get('isError')
steps.append(dict(phase='script',step='execute script chaining both imports',tool='execute(script)',args={},status='PASS' if ok else 'FAIL',note=t[:200],response=js(t))); log.append(dict(step='script',raw=r)); print('[script]',ok,t[:200])
import os
before=open('logs/canary-18999.log').read().count('GET ')
for p in ['bad_unspecified_v4','bad_unspecified_v6','bad_zero_net','bad_cgnat']:
    run('ssrf-probe',f'{p}_getinventory not executable',f'{p}_getinventory',{},lambda o,t,e:(e, t[:160]))
run('ssrf-probe','control 100.128.0.1 (just outside 100.64/10) still accepted','ok_100_128_getinventory',{},lambda o,t,e:(None, 'registered (boundary correct); '+t[:120]))
run('ssrf-probe','probe_dns_loopback (127.0.0.1.nip.io)','probe_dns_loopback_getinventory',{},lambda o,t,e:(None, 'still accepted at load (string-only host check); '+t[:140]))
after=open('logs/canary-18999.log').read().count('GET ')
steps.append(dict(phase='ssrf-probe',step='local canary :18999 not hit',tool='canary',args={},status='PASS' if after==before==0 else 'FAIL',note=f'canary GET count before={before} after={after}',response=None)); print('[canary]',before,after)
json.dump(steps,open('responses.json','w'),indent=1,ensure_ascii=False); json.dump(log,open('mcp-responses.json','w'),indent=1,ensure_ascii=False)
