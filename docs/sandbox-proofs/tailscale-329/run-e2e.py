#!/usr/bin/env python3
"""PR #328 Tailscale sandbox E2E over MCP.
  python3 run-e2e.py --mode mock        offline: local stateful mock API via base_url (semantics only)
  python3 run-e2e.py --mode fake        offline: real api.tailscale.com with fake token -> 401 paths + validation
  python3 run-e2e.py --mode live        live: needs /home/box/.sb-sandbox-secrets/tailscale-328.env
  python3 run-e2e.py --mode oauth       live via OAuth client creds (TS_OAUTH_CLIENT_ID/SECRET)
Every setting changed is snapshotted first and restored in a finally block (restore results logged)."""
import json, os, re, subprocess, sys, time, argparse, traceback
from mcpc import MCPClient, text
D=os.path.dirname(os.path.abspath(__file__)); os.chdir(D)
ap=argparse.ArgumentParser(); ap.add_argument('--mode',required=True,choices=['mock','fake','live','oauth']); ap.add_argument('--port',type=int,default=13854)
ap.add_argument('--no-node',action='store_true'); A=ap.parse_args(); M=A.mode
ENV={}
if M in('live','oauth'):
    for l in open('/home/box/.sb-sandbox-secrets/tailscale-328.env'):
        if '=' in l and not l.strip().startswith('#'): k,v=l.strip().split('=',1); ENV[k]=v.strip().strip('"\'')
SECRETS=[v for k,v in ENV.items() if v and k!='TS_TAILNET']
if M=='mock':
    open('mock-requests.jsonl','w').close()
    mock=subprocess.Popen([sys.executable,'mock_tailscale.py',str(A.port+4474),'mock-requests.jsonl']); time.sleep(0.8)
    creds={'api_key':'tskey-api-MOCK-ok','base_url':f'http://127.0.0.1:{A.port+4474}'}
elif M=='fake': creds={'api_key':'tskey-api-FAKE000-notreal'}
elif M=='live': creds={'api_key':ENV['TS_API_KEY'],'tailnet':ENV.get('TS_TAILNET','-')}
else: creds={'oauth_client_id':ENV['TS_OAUTH_CLIENT_ID'],'oauth_client_secret':ENV['TS_OAUTH_CLIENT_SECRET'],'tailnet':ENV.get('TS_TAILNET','-')}
subprocess.run(['./start-sb.sh',M,str(A.port),json.dumps(creds)],check=True)
c=MCPClient(f'http://127.0.0.1:{A.port}/mcp'); c.init()
steps=[]; raw=[]
def redact(s):
    s=json.dumps(s) if not isinstance(s,str) else s
    for x in SECRETS: s=s.replace(x,'<REDACTED>')
    return re.sub(r'tskey-(auth|api|client|webhook)-[A-Za-z0-9_-]+',lambda m:'tskey-'+m.group(1)+'-<REDACTED>',s)
def js(t):
    try: return json.loads(t)
    except Exception:
        try: return json.JSONDecoder().raw_decode(t.strip())[0]
        except Exception: return t
def X(tool,args):
    try: r=c.call('execute',{'tool_name':tool,'arguments':args}); t=text(r); return js(t),t,bool(r.get('isError'))
    except Exception as e: return None,str(e),True
def run(phase,step,tool,args,check):
    o,t,e=X(tool,args)
    try: ok,note=check(o,t,e)
    except Exception as ex: ok,note=False,f'check crashed: {ex}; {t[:200]}'
    st='INFO' if ok is None else ('PASS' if ok else 'FAIL')
    steps.append(dict(phase=phase,step=step,tool=tool,args=json.loads(redact(args)),status=st,note=redact(note),response=json.loads(redact(o)) if not isinstance(o,str) else redact(t)[:3000]))
    raw.append(dict(step=step,tool=tool,text=redact(t)))
    print(f'[{st}] {phase}:{step} {tool} — {redact(note)[:220]}',flush=True); return o,t,e
ok_=lambda o,t,e:(not e,t[:200])
err_=lambda frag: (lambda o,t,e:(e and (frag.lower() in t.lower() or 'missing required parameter' in t), t[:220]))
restore=[]  # (label, tool, args)
# ---------- discovery ----------
def search(q,expect):
    r=c.call('search',{'query':q,'limit':60}); o=js(text(r)); tl=o.get('tools',{}) if isinstance(o,dict) else []; n=[r[tl['columns'].index('name')] for r in tl['rows']] if isinstance(tl,dict) else [x.get('name') for x in tl]
    miss=[x for x in expect if x not in n]; st='PASS' if not miss else 'FAIL'
    steps.append(dict(phase='search',step=f'search "{q}"',tool='search',args={'query':q},status=st,note=f'{len(n)} tools; missing={miss}',response=n))
    print(f'[{st}] search "{q}" {len(n)} missing={miss}')
ALL=[l.split('"')[1] for l in open('/workspace/sb-tailscale-329/integrations/tailscale/tools.go') if 'Name:' in l and 'mcp.ToolName' in l]
search('tailscale',ALL); search('tailscale acl policy',['tailscale_get_policy_file','tailscale_set_policy_file']); search('tailscale webhook',['tailscale_create_webhook','tailscale_list_webhooks'])
# ---------- client-side validation (must not need API) ----------
V=[('set_policy_file missing if_match','tailscale_set_policy_file',{'policy':'{}'},'if_match is required'),
   ('set_policy_file missing policy','tailscale_set_policy_file',{'if_match':'abc'},'policy is required'),
   ('preview bad type','tailscale_preview_policy_rules',{'type':'bogus','preview_for':'x'},'type must be user or ipport'),
   ('validate neither','tailscale_validate_policy_file',{},'policy or tests is required'),
   ('validate tests not array','tailscale_validate_policy_file',{'tests':{'a':1}},'array'),
   ('split_dns bad value','tailscale_update_split_dns',{'split_dns':{'corp.example':'1.1.1.1'}},'must be a list'),
   ('split_dns empty','tailscale_update_split_dns',{'split_dns':{}},'split_dns is required'),
   ('nameservers missing','tailscale_set_dns_nameservers',{},'nameservers is required'),
   ('magic_dns missing','tailscale_set_dns_preferences',{},'magic_dns is required'),
   ('settings empty','tailscale_update_tailnet_settings',{'settings':{}},'settings is required'),
   ('webhook no subs','tailscale_create_webhook',{'endpoint_url':'https://example.com/h','subscriptions':[]},'subscriptions is required'),
   ('get_device no id','tailscale_get_device',{},'device_id is required'),
   ('delete_key no id','tailscale_delete_key',{},'key_id is required')]
for name,tool,args,frag in V: run('validation',name,tool,args,err_(frag))
if M=='fake':
    for tool,args in [('tailscale_list_devices',{}),('tailscale_get_policy_file',{}),('tailscale_get_dns',{}),('tailscale_list_webhooks',{})]:
        run('auth','fake token -> 401',tool,args,lambda o,t,e:(e and '401' in t, t[:200]))
    # misconfig: auth key in api_key, both creds, half oauth -> Configure errors surface in log
    for nm,cr in [('authkey_as_api_key',{'api_key':'tskey-auth-XXXX'}),('both',{'api_key':'tskey-api-x','oauth_client_id':'a','oauth_client_secret':'b'}),('half_oauth',{'oauth_client_id':'a'}),('client_secret_as_api_key',{'api_key':'tskey-client-x'})]:
        subprocess.run(['./start-sb.sh','cfg-'+nm,str(A.port+1),json.dumps(cr)],check=True); c2=MCPClient(f'http://127.0.0.1:{A.port+1}/mcp'); c2.init()
        r=c2.call('execute',{'tool_name':'tailscale_list_devices','arguments':{}}); t=text(r); lg=open(f'switchboard-cfg-{nm}.log').read()
        hit=[l for l in lg.splitlines() if 'tailscale:' in l][:1]
        st='PASS' if (r.get('isError') and hit) else 'FAIL'
        steps.append(dict(phase='config',step=f'misconfig {nm}',tool='Configure',args=cr,status=st,note=f'tool: {t[:120]} | log: {hit[0][-200:] if hit else "no tailscale config error logged"}',response=t[:500]))
        print(f'[{st}] config {nm}: {t[:100]} | {hit[:1]}')
        subprocess.run(['kill',open(f'homes/cfg-{nm}/pid').read().strip()])
else:
  node_key=None; dev=None; wh=None; inv=None
  try:
    # ---------- reads ----------
    run('devices','list_devices','tailscale_list_devices',{},lambda o,t,e:(not e and isinstance(o,dict) and 'devices' in o, f"{len(o.get('devices',[])) if isinstance(o,dict) else '?'} devices"))
    s0,_,_=run('settings','get_tailnet_settings (snapshot)','tailscale_get_tailnet_settings',{},ok_)
    d0,_,_=run('dns','get_dns (snapshot)','tailscale_get_dns',{},ok_)
    run('users','list_users','tailscale_list_users',{},lambda o,t,e:(not e and 'users' in t, t[:200]))
    run('users','list_user_invites','tailscale_list_user_invites',{},ok_)
    run('contacts','get_contacts','tailscale_get_contacts',{},ok_)
    run('keys','list_keys','tailscale_list_keys',{},ok_)
    # ---------- ACL ----------
    p0,_,_=run('acl','get_policy_file (snapshot, HuJSON+etag)','tailscale_get_policy_file',{},lambda o,t,e:(not e and o.get('etag') and isinstance(o.get('policy'),str), f"etag={o.get('etag')} len={len(o.get('policy',''))} has_comment={'//' in o.get('policy','')}"))
    POL0,ET0=p0['policy'],p0['etag']
    marker='// sb-proof-328 marker comment — must survive round trip'
    tagown='"tagOwners": {"tag:sb-proof": ["autogroup:admin"]},' if 'tag:sb-proof' not in POL0 else ''
    i=POL0.index('{'); POL1=POL0[:i+1]+'\n  '+marker+'\n  '+tagown+POL0[i+1:]
    if tagown and '"tagOwners"' in POL0:  # merge into existing tagOwners instead
        POL1=POL0[:i+1]+'\n  '+marker+POL0[i+1:]; POL1=re.sub(r'("tagOwners"\s*:\s*\{)',r'\1"tag:sb-proof": ["autogroup:admin"],',POL1,1)
    run('acl','validate_policy_file (edited)','tailscale_validate_policy_file',{'policy':POL1},lambda o,t,e:(not e and (o=={'valid':True} or 'message' not in t), t[:200]))
    run('acl','validate_policy_file (invalid HuJSON)','tailscale_validate_policy_file',{'policy':'{"acls": [ {"action": "bogus" '},lambda o,t,e:(e or 'message' in t or 'error' in t.lower(), t[:200]))
    me=(s0 and None) or ENV.get('TS_PREVIEW_USER','sb@example.com')
    run('acl','preview_policy_rules user (current)','tailscale_preview_policy_rules',{'type':'user','preview_for':me},lambda o,t,e:(not e, t[:200]))
    run('acl','preview_policy_rules ipport (draft)','tailscale_preview_policy_rules',{'type':'ipport','preview_for':'100.64.0.1:22','policy':POL1},lambda o,t,e:(not e, t[:200]))
    p1,_,_=run('acl','set_policy_file with etag','tailscale_set_policy_file',{'policy':POL1,'if_match':ET0},lambda o,t,e:(not e and o.get('status')=='updated', t[:200]))
    restore.append(('acl',None,None))
    g1,_,_=run('acl','get after save: comment survived','tailscale_get_policy_file',{},lambda o,t,e:(marker in o.get('policy',''), f"marker_present={marker in o.get('policy','')} etag={o.get('etag')}"))
    run('acl','set_policy_file STALE etag refused (412)','tailscale_set_policy_file',{'policy':POL0,'if_match':ET0},lambda o,t,e:(e and '412' in t and 'call tailscale_get_policy_file again' in t, t[:260]))
    run('acl','set_policy_file bare etag (unquoted) accepted','tailscale_set_policy_file',{'policy':POL1+'\n','if_match':g1['etag'].strip('"')},lambda o,t,e:(not e, t[:200]))
    # ---------- auth key + test node ----------
    k,_,_=run('keys','create_auth_key','tailscale_create_auth_key',{'description':'sb-proof-328','expiry_seconds':3600,'reusable':False,'preauthorized':False},lambda o,t,e:(not e and str(o.get('key','')).startswith('tskey-auth-'), f"id={o.get('id')} key returned once={bool(o.get('key'))}"))
    node_key=k.get('id'); ak=k.get('key'); SECRETS.append(ak or 'x')
    run('keys','get_key','tailscale_get_key',{'key_id':node_key},lambda o,t,e:(not e and o.get('id')==node_key and not o.get('key'), f"secret absent on get={not o.get('key')}"))
    if M!='mock':
        run('settings','enable devicesApprovalOn (for authorize test)','tailscale_update_tailnet_settings',{'settings':{'devicesApprovalOn':True}},ok_)
        restore.append(('settings','tailscale_update_tailnet_settings',{'settings':{'devicesApprovalOn':s0.get('devicesApprovalOn',False)}}))
        if not A.no_node:
            print(subprocess.run(['./node.sh','up',ak],capture_output=True,text=True).stdout[-400:])
            for _ in range(20):
                o,_,_=X('tailscale_list_devices',{'hostname':'sb-proof-328-node'}); ds=[d for d in (o or {}).get('devices',[]) if d.get('hostname','').startswith('sb-proof-328-node')]
                if ds: dev=ds[0]; break
                time.sleep(3)
    else: dev={'nodeId':'n1','id':'1001'}
    if dev:
        did=dev.get('nodeId') or dev['id']
        run('devices','get_device (all fields)','tailscale_get_device',{'device_id':did,'fields':'all'},lambda o,t,e:(not e and (o.get('nodeId')==did or o.get('id')==did), t[:200]))
        run('devices','authorize_device','tailscale_authorize_device',{'device_id':did,'authorized':True},ok_)
        run('devices','get_device authorized=true','tailscale_get_device',{'device_id':did},lambda o,t,e:(o.get('authorized') is True, f"authorized={o.get('authorized')}"))
        run('devices','set_device_name','tailscale_set_device_name',{'device_id':did,'name':'sb-proof-328-renamed'},ok_)
        run('devices','set_device_tags tag:sb-proof','tailscale_set_device_tags',{'device_id':did,'tags':['tag:sb-proof']},ok_)
        run('devices','get_device shows tag','tailscale_get_device',{'device_id':did},lambda o,t,e:('tag:sb-proof' in (o.get('tags') or []), f"tags={o.get('tags')} name={o.get('name')}"))
        run('routes','get_device_routes (advertised 10.99.0.0/24)','tailscale_get_device_routes',{'device_id':did},lambda o,t,e:('10.99.0.0/24' in (o.get('advertisedRoutes') or []), t[:200]))
        run('routes','set_device_routes approve','tailscale_set_device_routes',{'device_id':did,'routes':['10.99.0.0/24']},lambda o,t,e:(not e,t[:200]))
        run('routes','routes enabled','tailscale_get_device_routes',{'device_id':did},lambda o,t,e:('10.99.0.0/24' in (o.get('enabledRoutes') or []), t[:200]))
    else: steps.append(dict(phase='devices',step='test node join',tool='-',args={},status='FAIL',note='test node did not appear; see node/up.log',response=None))
    # ---------- DNS (each restored) ----------
    ns0=[n.get('address',n) if isinstance(n,dict) else n for n in (d0.get('nameservers') or [])]
    sp0=d0.get('searchPaths') or []; md0=(d0.get('preferences') or {}).get('magicDNS',True); split0=d0.get('splitDNS') or {}
    restore+= [('dns','tailscale_set_dns_nameservers',{'nameservers':ns0}),('dns','tailscale_set_dns_search_paths',{'search_paths':sp0}),('dns','tailscale_set_dns_preferences',{'magic_dns':md0}),('dns','tailscale_update_split_dns',{'split_dns':{'sb-proof-328.example':None}})]
    run('dns','set_dns_nameservers +9.9.9.9','tailscale_set_dns_nameservers',{'nameservers':ns0+['9.9.9.9']},lambda o,t,e:(not e and '9.9.9.9' in t,t[:200]))
    run('dns','set_dns_search_paths','tailscale_set_dns_search_paths',{'search_paths':sp0+['sb-proof-328.example']},lambda o,t,e:(not e and 'sb-proof-328.example' in t,t[:200]))
    if ns0 or M=='mock': run('dns','set_dns_preferences toggle MagicDNS','tailscale_set_dns_preferences',{'magic_dns':not md0},lambda o,t,e:(not e,t[:200]))
    run('dns','update_split_dns add','tailscale_update_split_dns',{'split_dns':{'sb-proof-328.example':['1.1.1.1']}},lambda o,t,e:(not e and 'sb-proof-328.example' in t,t[:200]))
    run('dns','get_dns reflects writes','tailscale_get_dns',{},lambda o,t,e:('9.9.9.9' in t and 'sb-proof-328.example' in t, t[:250]))
    # ---------- tailnet settings ----------
    run('settings','update_tailnet_settings usersApprovalOn toggle','tailscale_update_tailnet_settings',{'settings':{'usersApprovalOn':not s0.get('usersApprovalOn',False)}},ok_)
    restore.append(('settings','tailscale_update_tailnet_settings',{'settings':{'usersApprovalOn':s0.get('usersApprovalOn',False)}}))
    # ---------- invites ----------
    iv,_,_=run('users','create_user_invite (example.invalid, not delivered)','tailscale_create_user_invite',{'email':'sb-proof-328@example.invalid','role':'member'},lambda o,t,e:(None if e else True, t[:200]))
    if isinstance(iv,list) and iv: inv=iv[0].get('id')
    if inv: run('users','delete_user_invite','tailscale_delete_user_invite',{'invite_id':inv},ok_); inv=None
    # ---------- webhooks ----------
    w,t,_=run('webhooks','create_webhook','tailscale_create_webhook',{'endpoint_url':'https://example.com/sb-proof-328','subscriptions':['nodeCreated','policyUpdate']},lambda o,t,e:(not e and o.get('endpointId'), f"id={o.get('endpointId')} secret_in_create_result={'secret' in t}"))
    wh=w.get('endpointId') if isinstance(w,dict) else None
    run('webhooks','list_webhooks: secret stripped','tailscale_list_webhooks',{},lambda o,t,e:(not e and wh in t and '"secret"' not in t and 'tskey-webhook' not in t, f"contains_secret={'secret' in t}"))
    run('webhooks','create_webhook result: secret stripped?','tailscale_create_webhook',{'endpoint_url':'','subscriptions':['x']},err_('endpoint_url is required'))
    steps.append(dict(phase='webhooks',step='create_webhook result contains secret',tool='tailscale_create_webhook',args={},status='INFO' if 'secret' in t else 'PASS',note=('create returns the signing secret unredacted (handler uses rawResult; only list strips). Possibly intentional (shown once) but inconsistent with list.' if 'secret' in t else 'no secret in create result'),response=None))
    if wh: run('webhooks','test_webhook','tailscale_test_webhook',{'webhook_id':wh},lambda o,t,e:(None if e else True,t[:200]))
  except Exception: traceback.print_exc()
  finally:
    # ---------- RESTORE ----------
    if wh: run('restore','delete_webhook','tailscale_delete_webhook',{'webhook_id':wh},ok_); run('restore','webhook gone','tailscale_list_webhooks',{},lambda o,t,e:(wh not in t,t[:120]))
    if inv: run('restore','delete_user_invite','tailscale_delete_user_invite',{'invite_id':inv},ok_)
    for lab,tool,args in reversed(restore):
        if lab=='acl':
            g,_,_=X('tailscale_get_policy_file',{}); run('restore','restore original policy','tailscale_set_policy_file',{'policy':POL0,'if_match':g['etag']},ok_)
        else: run('restore',f'restore {lab}',tool,args,ok_)
    if dev and M!='mock':
        subprocess.run(['./node.sh','down']); run('restore','delete test device','tailscale_delete_device',{'device_id':dev.get('nodeId') or dev['id']},ok_)
    if node_key: run('restore','revoke auth key','tailscale_delete_key',{'key_id':node_key},ok_); run('restore','key revoked','tailscale_get_key',{'key_id':node_key},lambda o,t,e:(e or 'revoked' in t or 'invalid' in t,t[:200]))
    if 'POL0' in dir(): run('restore','policy byte-identical to snapshot','tailscale_get_policy_file',{},lambda o,t,e:(o.get('policy')==POL0,'identical' if o.get('policy')==POL0 else 'DIFFERS'))
    run('restore','dns matches snapshot','tailscale_get_dns',{},lambda o,t,e:(json.dumps(o,sort_keys=True)==json.dumps(d0,sort_keys=True), 'identical' if json.dumps(o,sort_keys=True)==json.dumps(d0,sort_keys=True) else 'DIFFERS: '+t[:200]))
    run('restore','settings match snapshot','tailscale_get_tailnet_settings',{},lambda o,t,e:(o==s0, 'identical' if o==s0 else 'DIFFERS: '+t[:200]))
if M=='mock':
    mock.terminate(); reqs=[json.loads(l) for l in open('mock-requests.jsonl')]
    acl=[r for r in reqs if r['p'].endswith('/acl') and r['m']=='POST']
    for r in acl: print('ACL POST', r['ct'], r['if_match'])
    steps.append(dict(phase='wire',step='ACL POSTs sent as application/hujson with quoted If-Match',tool='-',args={},status='PASS' if acl and all(r['ct']=='application/hujson' and r['if_match'] and r['if_match'].startswith('"') for r in acl) else 'FAIL',note=json.dumps([(r['ct'],r['if_match']) for r in acl]),response=None))
    bad=[r for r in reqs if 'bogus' in r['p']]; 
open(f'responses-{M}.json','w').write(json.dumps(steps,indent=1)); open(f'mcp-raw-{M}.json','w').write(json.dumps(raw,indent=1))
from collections import Counter; C=Counter(s['status'] for s in steps); print(f'== {M}: {dict(C)}')
subprocess.run(['kill',open(f'homes/{M}/pid').read().strip()])
