#!/usr/bin/env python3
"""PR #329: Configure-error surfacing on Execute, before (main 6bc872e = merged #328) vs after (9888bfc), plus clear-on-success via live reconfigure."""
import json, os, subprocess, time, requests
from mcpc import MCPClient, text
D=os.path.dirname(os.path.abspath(__file__)); os.chdir(D)
BIN={'before':'/tmp/sb-ts328main','after':'/tmp/sb-ts329'}
mock=subprocess.Popen(['python3','mock_tailscale.py','18329','mock-requests-329.jsonl']); time.sleep(0.8)
CASES=[('auth_key_as_api_key',{'api_key':'tskey-auth-kSBPROOF-notreal'},'device auth key'),
 ('client_secret_as_api_key',{'api_key':'tskey-client-kSBPROOF-notreal'},'OAuth client secret'),
 ('token_and_oauth',{'api_key':'tskey-api-SBPROOF-notreal','oauth_client_id':'cid','oauth_client_secret':'tskey-client-x-notreal'},'not both'),
 ('half_oauth_id_only',{'oauth_client_id':'cid'},'must both be set'),
 ('half_oauth_secret_only',{'oauth_client_secret':'tskey-client-x-notreal'},'must both be set'),
 ('missing_creds',{'api_key':''},'is required')]
steps=[]; raw=[]
def rec(phase,step,status,note,resp=None,args=None):
    steps.append(dict(phase=phase,step=step,status=status,note=note,args=args,response=resp)); print(f'[{status}] {phase}:{step} — {note[:230]}',flush=True)
def start(name,ver,creds,port):
    subprocess.run(['./start-sb.sh',name,str(port),json.dumps(creds)],check=True,env=dict(os.environ,SB_BIN=BIN[ver]))
    c=MCPClient(f'http://127.0.0.1:{port}/mcp'); c.init(); return c
def ex(c,tool='tailscale_list_devices',args=None):
    r=c.call('execute',{'tool_name':tool,'arguments':args or {}}); t=text(r).split('\npinned as')[0]; raw.append(dict(tool=tool,text=t,isError=r.get('isError'))); return t,bool(r.get('isError'))
def stop(name): subprocess.run(['kill',open(f'homes/{name}/pid').read().strip()])
table=[]
for nm,cr,frag in CASES:
    row={'case':nm}
    for ver,port in (('before',13860),('after',13861)):
        c=start(f'{ver}-{nm}',ver,cr,port); t,e=ex(c); stop(f'{ver}-{nm}'); row[ver]=t
    ok_after=frag.lower() in row['after'].lower() and 'not configured' in row['after']
    ok_before=row['before'].strip()=='tailscale: integration is not configured'
    rec('bad-creds',nm,'PASS' if ok_after else 'FAIL',f"before={row['before']!r} | after={row['after']!r}",row,cr)
    if not ok_before: rec('bad-creds',nm+' (baseline)','INFO','before build did not give bare not-configured: '+row['before'])
    table.append(row)
# Clear-on-success: one live process, reconfigure via the web UI save endpoint (same as clicking Save)
P=13862; c=start('after-reconf','after',{'api_key':'tskey-auth-kSBPROOF-notreal'},P)
def save(creds):
    form={'enabled':'true',**{'cred_'+k:v for k,v in creds.items()}}
    r=requests.post(f'http://127.0.0.1:{P}/integrations/tailscale',data=form,allow_redirects=True,headers={'Origin':f'http://127.0.0.1:{P}'}); time.sleep(1.0); return r.status_code,(r.history[0].headers.get('Location','') if r.history else r.url)
t,e=ex(c); rec('clear','1 bad config -> specific reason',('PASS' if e and 'device auth key' in t else 'FAIL'),t)
st,loc=save({'api_key':'tskey-api-SBPROOF-fake','oauth_client_id':'','oauth_client_secret':'','tailnet':'','base_url':''}); 
t,e=ex(c); rec('clear','2 reconfigure fake well-formed key (real API) -> clean 401, stored error gone',('PASS' if e and '401' in t and 'device auth key' not in t and 'not configured' not in t else 'FAIL'),f'save={st} {loc} | {t}')
st,loc=save({'api_key':'tskey-auth-kSBPROOF-notreal','base_url':''}); t,e=ex(c)
rec('clear','3 back to bad key -> reason returns, old client not reused',('PASS' if e and 'device auth key' in t and '401' not in t else 'FAIL'),f'save={st} | {t}')
st,loc=save({'api_key':'tskey-api-MOCK-ok','base_url':'http://127.0.0.1:18329','oauth_client_id':'','oauth_client_secret':''}); t,e=ex(c)
rec('clear','4 reconfigure valid key vs mock API -> real success, no stored error',('PASS' if not e and 'devices' in t else 'FAIL'),f'save={st} | {t[:200]}')
st,loc=save({'oauth_client_id':'cid','base_url':'http://127.0.0.1:18329'}); t,e=ex(c)
rec('clear','5 half OAuth after success -> reason, previous working client dropped',('PASS' if e and 'must both be set' in t else 'FAIL'),f'save={st} | {t}')
st,loc=save({'oauth_client_id':'cid','oauth_client_secret':'tskey-client-MOCK-x','base_url':'http://127.0.0.1:18329'}); t,e=ex(c)
rec('clear','6 OAuth client creds (mock token endpoint missing) -> error is upstream OAuth, not stale configure error',('PASS' if e and 'must both be set' not in t and 'device auth key' not in t and 'not configured' not in t else 'FAIL'),f'save={st} | {t[:220]}')
stop('after-reconf'); mock.terminate()
lg=open('switchboard-after-reconf.log').read()
for s in ['notreal','MOCK-ok','SBPROOF-fake']:
    rec('leak',f'credential fragment {s!r} in server log','PASS' if s not in lg else 'FAIL','absent' if s not in lg else 'PRESENT in switchboard-after-reconf.log')
json.dump(dict(table=table,steps=steps),open('responses-329.json','w'),indent=1); json.dump(raw,open('mcp-responses-329.json','w'),indent=1)
from collections import Counter; print('== 329:',dict(Counter(s['status'] for s in steps)))
