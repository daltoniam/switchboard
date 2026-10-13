import json, requests
class MCPClient:
    def __init__(s, base):
        s.base=base; s.http=requests.Session(); s.id=0
        s.h={"Content-Type":"application/json","Accept":"application/json, text/event-stream","Mcp-Protocol-Version":"2025-11-25"}
    def _parse(s,r):
        raw=r.text
        if "text/event-stream" in r.headers.get("Content-Type","") or raw.lstrip().startswith(("event:","data:")):
            last=None
            for line in raw.splitlines():
                if line.startswith("data:"):
                    try: last=json.loads(line[5:].strip())
                    except Exception: pass
            return last or {}
        return r.json() if raw.strip() else {}
    def rpc(s,method,params=None,name="",notify=False):
        s.id+=1; h=dict(s.h); h["Mcp-Method"]=method
        if name: h["Mcp-Name"]=name
        body={"jsonrpc":"2.0","method":method}
        if not notify: body["id"]=s.id
        if params is not None: body["params"]=params
        r=s.http.post(s.base,headers=h,json=body,timeout=120)
        if sid:=r.headers.get("Mcp-Session-Id"): s.h["Mcp-Session-Id"]=sid
        p=s._parse(r)
        if r.status_code>=400: raise RuntimeError(f"HTTP {r.status_code}: {p}")
        if "error" in p: raise RuntimeError(f"RPC error: {p['error']}")
        return p.get("result",p)
    def init(s):
        s.rpc("initialize",{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"specimport-198-e2e","version":"0"}})
        s.rpc("notifications/initialized",{},notify=True)
    def call(s,t,a): return s.rpc("tools/call",{"name":t,"arguments":a},name=t)
def text(res): return "\n".join(c.get("text","") for c in res.get("content") or [] if isinstance(c,dict) and c.get("type")=="text")
