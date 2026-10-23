#!/usr/bin/env python3
"""PR #349 host-level E2E: a local Switchboard (PR head vs main) wired to the stand-in MCP server via
figma/slackmcp base_url, driven through MCP search/execute and the web Save form. Linear's MCP URL is
hard-coded in cmd/server/main.go, so Linear is covered by the adapter harness only."""
import json, os, subprocess, time, requests, sys
from mcpc import MCPClient, text
D=os.path.dirname(os.path.abspath(__file__)); os.chdir(D)
ST='http://127.0.0.1:18349'
BIN={'before':'/tmp/sb349main','after':'/tmp/sb349'}; PORT={'after':13855,'before':13856}
steps=[]; raw=[]
def rec(ver,step,status,note,ev=None):
    steps.append(dict(build=ver,step=step,status=status,note=note,server_events=ev)); print(f'[{ver}] [{status}] {step} — {note[:260]}',flush=True)
mark=[0]
def since():
    e=requests.get(ST+'/_events').json() or []; out=[x for x in e if x['seq']>mark[0] and not x.get('probe')]
    if e: mark[0]=e[-1]['seq']
    return out
def kinds(ev,kind,server=None): return [x.get('session_out') or x.get('session_in') for x in ev if x['kind']==kind and (server is None or x['server']==server)]
def probe(server,sid):
    r=requests.post(f'{ST}/{server}/mcp',headers={'Content-Type':'application/json','Accept':'application/json, text/event-stream','Authorization':'Bearer probe','Mcp-Session-Id':sid,'Mcp-Protocol-Version':'2025-06-18','X-Standin-Probe':'1'},json={'jsonrpc':'2.0','id':9,'method':'tools/list','params':{}}); return r.status_code
st=subprocess.Popen(['/tmp/standin349','-addr','127.0.0.1:18349','-log','logs/standin-host.jsonl'],stderr=open('logs/standin-host.stderr','w')); time.sleep(1)
for ver in ('before','after'):
    P=PORT[ver]
    cfg={"integrations":{
      "figma":{"enabled":True,"credentials":{"mcp_access_token":"standin-figma-token","base_url":ST+"/figma-a"}},
      "slackmcp":{"enabled":True,"credentials":{"base_url":ST+"/slack"},"identities":{
         "work":{"credentials":{"access_token":"standin-slack-work"}},"personal":{"credentials":{"access_token":"standin-slack-personal"}}}}}}
    since()
    subprocess.run(['./start-sb.sh',ver,str(P),json.dumps(cfg)],check=True,env=dict(os.environ,SB_BIN=BIN[ver]))
    time.sleep(2)
    c=MCPClient(f'http://127.0.0.1:{P}/mcp'); c.init()
    ev=since(); rec(ver,'startup: Switchboard configures figma + slackmcp(2 identities)','INFO',f"sessions opened at startup: figma-a={kinds(ev,'session_open','figma-a')} slack={kinds(ev,'session_open','slack')}",ev)
    s=c.call('search',{'query':'figma whoami'}); stx=text(s); raw.append(dict(build=ver,call='search figma whoami',text=stx[:3000]))
    rec(ver,'search finds figma_whoami','PASS' if 'figma_whoami' in stx else 'FAIL',stx[:200])
    s=c.call('search',{'query':'slackmcp read channel'}); stx=text(s); raw.append(dict(build=ver,call='search slackmcp',text=stx[:3000]))
    rec(ver,'search finds slackmcp_read_channel','PASS' if 'slackmcp_read_channel' in stx else 'FAIL',stx[:200])
    def ex(tool,args=None):
        r=c.call('execute',{'tool_name':tool,'arguments':args or {}}); t=text(r); raw.append(dict(build=ver,tool=tool,args=args,text=t,isError=r.get('isError'))); return t,bool(r.get('isError'))
    t,e=ex('figma_whoami'); ev=since(); fa=[x for x in kinds(ev,'session_open','figma-a')]
    rec(ver,'execute figma_whoami via Switchboard → stand-in figma-a','PASS' if not e and '"standin":"figma-a"' in t.replace('\\"','"') or 'figma-a' in t else 'FAIL',t[:200],ev)
    import re
    m=re.search(r'session\\?":\\?"([A-Z0-9]{20,})',t); sidA=m.group(1) if m else None
    tw,ew=ex('slackmcp_read_channel',{'identity_id':'work'}); tp,ep=ex('slackmcp_read_channel',{'identity_id':'personal'}); ev=since()
    sw=re.search(r'session\\?":\\?"([A-Z0-9]{20,})',tw); sp=re.search(r'session\\?":\\?"([A-Z0-9]{20,})',tp)
    sw=sw.group(1) if sw else None; sp=sp.group(1) if sp else None
    rec(ver,'execute slackmcp_read_channel for work + personal','PASS' if not ew and not ep and sw and sp and sw!=sp else 'FAIL',f'work sid={sw} personal sid={sp}',ev)
    # Change figma base_url via the web Save form (as a user clicking Save); the redirect target re-runs Configure.
    form={'enabled':'true','cred_mcp_access_token':'standin-figma-token','cred_base_url':ST+'/figma-b'}
    r=requests.post(f'http://127.0.0.1:{P}/integrations/figma',data=form,allow_redirects=True,headers={'Origin':f'http://127.0.0.1:{P}'}); time.sleep(1.5)
    ev=since(); dels=kinds(ev,'session_delete','figma-a'); pa=probe('figma-a',sidA) if sidA else -1
    ok=(sidA in dels and pa==404) if ver=='after' else True
    note=f"save HTTP {r.status_code} → {r.url}; DELETE on figma-a={dels}; probe old figma-a sid {sidA} → HTTP {pa}"
    rec(ver,'web Save figma base_url A→B ends old session on A', ('PASS' if ok else 'FAIL') if ver=='after' else ('INFO (leak)' if pa==200 else 'INFO'), note, ev)
    t,e=ex('figma_whoami'); ev=since()
    rec(ver,'execute figma_whoami after base_url change → figma-b','PASS' if not e and 'figma-b' in t else 'FAIL',t[:200],ev)
    # Remove the personal identity via config-level reconfigure is not exposed simply; check slack sessions still alive.
    rec(ver,'slack sessions still alive while instance in use','INFO',f'probe work → {probe("slack",sw) if sw else -1}, personal → {probe("slack",sp) if sp else -1}')
    subprocess.run(['kill',open(f'homes/{ver}/pid').read().strip()]); time.sleep(1.5)
    ev=since()
    rec(ver,'Switchboard shutdown (SIGTERM): sessions ended?','INFO',f"DELETEs on shutdown={kinds(ev,'session_delete')}; probes after exit: work={probe('slack',sw) if sw else -1} personal={probe('slack',sp) if sp else -1}",ev)
st.terminate()
json.dump(dict(steps=steps),open('responses-host.json','w'),indent=1); json.dump(raw,open('mcp-responses-host.json','w'),indent=1)
from collections import Counter; print('== host:',dict(Counter((s['build'],s['status'].split()[0]) for s in steps)))
