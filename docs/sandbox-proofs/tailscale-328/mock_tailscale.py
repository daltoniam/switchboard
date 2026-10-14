"""Minimal stateful Tailscale API v2 mock for offline proof (NOT a substitute for live). Logs every request to mock-requests.jsonl."""
import json, hashlib, re, sys, time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
TOKEN="tskey-api-MOCK-ok"
POL='// sandbox policy\n{\n  // groups comment\n  "acls": [{"action":"accept","src":["*"],"dst":["*:*"]},],\n  "tagOwners": {"tag:sb-proof": ["autogroup:admin"]},\n}\n'
S={"acl":POL,"devices":{"n1":{"id":"1001","nodeId":"n1","name":"sb-node.tail-mock.ts.net","hostname":"sb-node","authorized":False,"tags":[],"advertisedRoutes":["10.99.0.0/24"],"enabledRoutes":[]}},
   "keys":{},"dns":{"dns":["8.8.8.8"],"magicDNS":True,"searchPaths":[],"split":{}},"settings":{"devicesApprovalOn":False,"usersApprovalOn":False},
   "webhooks":{},"invites":{},"users":[{"id":"u1","loginName":"sb@example.com","role":"owner","status":"active"}]}
et=lambda: '"'+hashlib.sha1(S["acl"].encode()).hexdigest()[:16]+'"'
LOG=open(sys.argv[2] if len(sys.argv)>2 else "mock-requests.jsonl","a")
class H(BaseHTTPRequestHandler):
    def log_message(s,*a): pass
    def send(s,code,obj=None,raw=None,ct="application/json",hdr=None):
        b=raw.encode() if raw is not None else (json.dumps(obj).encode() if obj is not None else b"")
        s.send_response(code); s.send_header("Content-Type",ct)
        for k,v in (hdr or {}).items(): s.send_header(k,v)
        s.send_header("Content-Length",str(len(b))); s.end_headers(); s.wfile.write(b)
    def handle_any(s):
        n=int(s.headers.get("Content-Length") or 0); body=s.rfile.read(n).decode() if n else ""
        LOG.write(json.dumps({"m":s.command,"p":s.path,"ct":s.headers.get("Content-Type"),"if_match":s.headers.get("If-Match"),"body":body[:2000]})+"\n"); LOG.flush()
        if s.headers.get("Authorization")!="Bearer "+TOKEN: return s.send(401,{"message":"API token invalid"})
        p=s.path.split("?")[0]; m=s.command; j=lambda: json.loads(body or "null")
        mm=re.match(r"/api/v2/tailnet/[^/]+(/.*)$",p); t=mm.group(1) if mm else None
        if t=="/devices": return s.send(200,{"devices":list(S["devices"].values())})
        if mm2:=re.match(r"/api/v2/device/([^/]+)(/.*)?$",p):
            d=S["devices"].get(mm2.group(1)); sub=mm2.group(2) or ""
            if not d: return s.send(404,{"message":"not found"})
            if sub=="" and m=="GET": return s.send(200,d)
            if sub=="/authorized": d["authorized"]=j()["authorized"]; return s.send(200,raw="")
            if sub=="/name": d["name"]=j()["name"]; return s.send(200,raw="")
            if sub=="/tags": d["tags"]=j()["tags"]; return s.send(200,raw="")
            if sub=="/routes" and m=="GET": return s.send(200,{"advertisedRoutes":d["advertisedRoutes"],"enabledRoutes":d["enabledRoutes"]})
            if sub=="/routes": d["enabledRoutes"]=j()["routes"]; return s.send(200,{"advertisedRoutes":d["advertisedRoutes"],"enabledRoutes":d["enabledRoutes"]})
        if t=="/keys" and m=="POST":
            k="kMOCK%d"%len(S["keys"]); S["keys"][k]={"id":k,"description":j().get("description",""),"capabilities":j()["capabilities"]}
            return s.send(200,dict(S["keys"][k],key="tskey-auth-"+k+"-MOCKSECRET"))
        if t=="/keys": return s.send(200,{"keys":list(S["keys"].values())})
        if t and t.startswith("/keys/"):
            k=t[6:]
            if k not in S["keys"]: return s.send(404,{"message":"not found"})
            if m=="DELETE": del S["keys"][k]; return s.send(200,raw="")
            return s.send(200,S["keys"][k])
        if t=="/acl" and m=="GET": return s.send(200,raw=S["acl"],ct="application/hujson",hdr={"ETag":et()})
        if t=="/acl" and m=="POST":
            im=s.headers.get("If-Match")
            if im and im!=et() and im!='"ts-default"': return s.send(412,{"message":"precondition failed, invalid old hash"})
            S["acl"]=body; return s.send(200,raw=body,ct="application/hujson",hdr={"ETag":et()})
        if t=="/acl/validate": return s.send(200,{"message":"test(s) failed"} if '"bogus"' in body else {})
        if t=="/acl/preview": return s.send(200,{"matches":[{"users":["*"],"ports":["*:*"],"lineNumber":4}],"type":"user","previewFor":"sb@example.com"})
        if t=="/dns/configuration": d=S["dns"]; return s.send(200,{"nameservers":[{"address":a} for a in d["dns"]],"searchPaths":d["searchPaths"],"splitDNS":d["split"],"preferences":{"magicDNS":d["magicDNS"]}})
        if t=="/dns/nameservers": S["dns"]["dns"]=j()["dns"]; return s.send(200,{"dns":S["dns"]["dns"],"magicDNS":S["dns"]["magicDNS"]})
        if t=="/dns/preferences": S["dns"]["magicDNS"]=j()["magicDNS"]; return s.send(200,{"magicDNS":S["dns"]["magicDNS"]})
        if t=="/dns/searchpaths": S["dns"]["searchPaths"]=j()["searchPaths"]; return s.send(200,{"searchPaths":S["dns"]["searchPaths"]})
        if t=="/dns/split-dns":
            for k,v in j().items():
                if v is None: S["dns"]["split"].pop(k,None)
                else: S["dns"]["split"][k]=v
            return s.send(200,S["dns"]["split"])
        if t=="/users": return s.send(200,{"users":S["users"]})
        if t=="/user-invites" and m=="POST":
            i="inv%d"%len(S["invites"]); S["invites"][i]={"id":i,"email":j()[0].get("email"),"role":j()[0].get("role")}; return s.send(200,[S["invites"][i]])
        if t=="/user-invites": return s.send(200,list(S["invites"].values()))
        if mi:=re.match(r"/api/v2/user-invites/(.+)$",p): S["invites"].pop(mi.group(1),None); return s.send(200,raw="")
        if t=="/settings" and m=="PATCH": S["settings"].update(j()); return s.send(200,S["settings"])
        if t=="/settings": return s.send(200,S["settings"])
        if t=="/webhooks" and m=="POST":
            w="wh%d"%len(S["webhooks"]); S["webhooks"][w]={"endpointId":w,"endpointUrl":j()["endpointUrl"],"subscriptions":j()["subscriptions"],"providerType":"","secret":"tskey-webhook-MOCKSECRET"}; return s.send(200,S["webhooks"][w])
        if t=="/webhooks": return s.send(200,{"webhooks":list(S["webhooks"].values())})
        if mw:=re.match(r"/api/v2/webhooks/([^/]+)(/test)?$",p):
            w=S["webhooks"].get(mw.group(1))
            if not w: return s.send(404,{"message":"not found"})
            if m=="DELETE": del S["webhooks"][mw.group(1)]; return s.send(200,raw="")
            if mw.group(2): return s.send(202,raw="")
            return s.send(200,w)
        if t=="/contacts": return s.send(200,{"account":{"email":"sb@example.com"}})
        return s.send(404,{"message":"mock: unhandled "+m+" "+p})
    do_GET=do_POST=do_PATCH=do_DELETE=do_PUT=handle_any
ThreadingHTTPServer(("127.0.0.1",int(sys.argv[1])),H).serve_forever()
