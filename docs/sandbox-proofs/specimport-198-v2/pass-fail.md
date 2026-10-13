## Before → after

| Finding | Before (68343a8) | After (78ca8f0) | Status |
|---|---|---|---|
| 1. SSRF: 0.0.0.0 / :: / 0/8 / 100.64/10 | FAIL: `http://0.0.0.0:18999` accepted; execute hit the local canary (`GET /store/inventory` from 127.0.0.1). `::`, 0/8, 100.64/10 were not blocked either | PASS: 0.0.0.0, [::], 0.1.2.3, 100.64.0.1, 100.127.255.254 all rejected at load; tools not registered (`tool not found`); canary GET count 0. 100.128.0.1 (just outside 100.64/10) still accepted, so the boundary is correct | **FIXED** |
| 2. GraphQL selection depth vs 50KB cap | FAIL: continents 539KB, continent EU 65KB, language es 64KB, countries(continent=OC) 68KB | continent EU 19.5KB PASS, language es 19.9KB PASS; **continents 147KB FAIL, countries(continent=OC) 68KB FAIL** (fixture continents also 147KB) | **PARTIAL**: 2 of 4 now fit; the list queries are still over the cap |
| 3. Integer ids ≥1e6 in path/query | FAIL: `289893960` sent as `2.8989396e+08` → upstream 400 on getPetById/update/delete; worked only as a string | PASS: addPet id 289910702 → getPetById (JSON number) 200 → updatePetWithForm → get shows renamed/sold → deletePet `Pet deleted` (cleanup verified: GET now `Pet not found`) | **FIXED** |

## Per-tool summary

| Tool / check | Result | Evidence |
|---|---|---|
| Build / unit tests | PASS | go vet, go test (specimport, netpolicy, config, web, cmd/server), go build at 78ca8f0 |
| search (discovery) | PASS | 19 `petstore_*`, 6 `countries_*` (live introspection), 6 `countries_fixture_*`; rejected imports register no tools |
| Petstore read: findPetsByStatus / getPetById | PASS | live available + sold lists; pet 1; 404 surfaced; missing `petId` rejected before any request |
| Petstore write: addPet → getPetById → updatePetWithForm → deletePet (id ≥1e6) | PASS | id 289910702 sent as an integer in path and query; full cycle live; test pet deleted |
| Petstore getInventory / findPetsByTags | INFO | upstream petstore3 returns 500 for both, even with direct curl. Array query params (`tags:["tag1"]`) are now rejected client-side (`cannot convert []interface {}`) |
| Countries: country US/JP/ZZ, countries `code in [DE,FR]`, fixture BR | PASS | correct live data |
| Countries: continent EU / language es | PASS | now 19.5KB / 19.9KB (were 65/64KB, over the cap) |
| Countries: continents / countries `continent=OC` | FAIL | still over the 50KB cap: continents 147KB (was 539KB), countries OC 68KB (unchanged) |
| execute script chaining across both imports | PASS | `{"country":"Canada","pet":"Pet1"}` |
| Base URL validation: loopback / metadata / scheme | PASS | 127.0.0.1, localhost, [::1], [::ffff:127.0.0.1], 169.254.169.254 (OpenAPI+GraphQL), metadata.google.internal, base_url override, file:// rejected |
| Base URL validation: 0.0.0.0, ::, 0/8, 100.64/10 | PASS | all rejected at load; local canary not hit; 100.128.0.1 boundary still allowed |
| DNS / redirect SSRF | INFO | unchanged: the host check is string-only; 127.0.0.1.nip.io accepted at load; no dial-time or redirect check |
| YAML spec | INFO | unchanged: JSON only |

## All steps

| Phase | Step | Tool | Result | Note |
|---|---|---|---|---|
| build | go vet ./specimport ./netpolicy ./config ./web | `go` | PASS | exit 0 |
| build | go test ./specimport ./netpolicy ./config ./web ./cmd/server | `go` | PASS | all 5 packages ok |
| build | go build ./cmd/server | `go` | PASS | dist/switchboard at 78ca8f0 |
| search | search petstore | `search` | PASS | 19 tools; missing=[]; unexpected=[] |
| search | search countries | `search` | PASS | 12 tools; missing=[]; unexpected=[] |
| search | search countries_fixture | `search` | PASS | 12 tools; missing=[]; unexpected=[] |
| search | search petstore pet status | `search` | PASS | 10 tools; missing=[]; unexpected=[] |
| petstore | findPetsByStatus available | `petstore_findpetsbystatus` | PASS | type=dict len=3 sample={"columns": ["id", "photoUrls", "tags", "name", "category"], "constants": {"status": "available"}, "rows": [[999, [], [], "test", null], [9001, ["string"], [], "doggie", {"id": 1,  |
| petstore | findPetsByStatus sold | `petstore_findpetsbystatus` | PASS | type=dict len=3 sample={"columns": ["id", "name", "photoUrls", "tags", "category"], "constants": {"status": "sold"}, "rows": [[735082001, "Rex", ["https://example.com/rex.png"], [], null], [8, "Laki", [" |
| petstore | getPetById 1 | `petstore_getpetbyid` | PASS | {"id": 1, "name": "Pet1", "photoUrls": ["test1", "test2"], "status": "available", "tags": []} |
| petstore | getInventory | `petstore_getinventory` | INFO | upstream petstore3 itself returns 500 for /store/inventory (curl direct: 500 x3) — not a PR issue; error surfaced cleanly: specimport: upstream returned 500 Internal Server Error: {"code":500,"message":"There was an error processing your re |
| petstore | addPet | `petstore_addpet` | PASS | {"category": {"id": 1, "name": "Dogs"}, "id": 289910702, "name": "sb-proof-198", "photoUrls": ["https://example.com/p.png"], "status": "pending", "tags": [{"id": 198, "name": "sandbox-e2e"}]} |
| petstore | getPetById (new pet, integer id >=1e6) | `petstore_getpetbyid` | PASS | {"category": {"id": 1, "name": "Dogs"}, "id": 289910702, "name": "sb-proof-198", "photoUrls": ["https://example.com/p.png"], "status": "pending", "tags": [{"id": 198, "name": "sandbox-e2e"}]} |
| petstore | updatePetWithForm (query params) | `petstore_updatepetwithform` | PASS | {"category":{"id":1,"name":"Dogs"},"id":289910702,"name":"sb-proof-198-renamed","photoUrls":["https://example.com/p.png"],"status":"sold","tags":[{"id":198,"name":"sandbox-e2e"}]} pinned as $6 |
| petstore | getPetById after update | `petstore_getpetbyid` | PASS | {"category":{"id":1,"name":"Dogs"},"id":289910702,"name":"sb-proof-198-renamed","photoUrls":["https://example.com/p.png"],"status":"sold","tags":[{"id":198,"name":"sandbox-e2e"}]} pinned as $7 |
| petstore | deletePet (cleanup) | `petstore_deletepet` | PASS | Pet deleted pinned as $8 |
| petstore | getPetById 0 (upstream 4xx) | `petstore_getpetbyid` | PASS | specimport: upstream returned 404 Not Found: Pet not found |
| petstore | findPetsByStatus status=pending (query) finds new pet | `petstore_findpetsbystatus` | INFO | query string param; [{"category":{"id":20370,"name":"tag2"},"id":542496,"name":"string","photoUrls":["æL´ç򯠔tk]","W","मनीष منش","Å𷹏ß򡔐ø~\rË\u0016Çù÷ Õ¾","IÅ\u0007ß齳)`","" |
| petstore | findPetsByTags tags=["tag1"] (array query param) | `petstore_findpetsbytags` | INFO | array query params rejected client-side (no request sent); before, fmt.Sprint sent "[tag1]". Arrays still unsupported. ERROR: specimport: query parameter "tags": parameter "v": cannot convert []interface {} to string |
| petstore | findPetsByTags tags="tag1" (string) | `petstore_findpetsbytags` | INFO | upstream petstore3 returns 500 for /pet/findByTags?tags=tag1 with direct curl too: not a PR issue. specimport: upstream returned 500 Internal Server Error: {"code":500,"message":"There was an error processing your request. It has been logge |
| petstore | missing required arg | `petstore_getpetbyid` | PASS | missing required parameter "petId" for tool "petstore_getpetbyid". Required: [petId] |
| countries | country US | `countries_country` | PASS | {"data":{"country":{"awsRegion":"us-east-2","capital":"Washington D.C.","code":"US","continent":{"code":"NA","countries":{"columns":[],"constants":{"__typename":"Country"},"rows":[[],[],[],[],[],[],[],[],[],[],[],[],[],[ |
| countries | country JP | `countries_country` | PASS | {"data":{"country":{"awsRegion":"ap-northeast-1","capital":"Tokyo","code":"JP","continent":{"code":"AS","countries":{"columns":[],"constants":{"__typename":"Country"},"rows":[[],[],[],[],[],[],[],[],[],[],[],[],[],[],[], |
| countries | countries filter continent=OC | `countries_countries` | FAIL | over cap: Response exceeded 50KB (actual: 68KB). Use more specific filters, lower limit/per_page, or fetch individual items. |
| countries | countries filter code in [DE,FR] | `countries_countries` | PASS | {"data":{"countries":[{"awsRegion":"eu-central-1","capital":"Berlin","code":"DE","continent":{"code":"EU","countries":[{"__typename":"Country"},{"__typename":"Country"},{"__typename":"Country"},{"__typename":"Country"},{ |
| countries | continents | `countries_continents` | FAIL | over cap: Response exceeded 50KB (actual: 147KB). Use more specific filters, lower limit/per_page, or fetch individual items. |
| countries | continent EU | `countries_continent` | PASS | rendered size=19.5KB (<50KB cap); {"data":{"continent":{"code":"EU","countries":{"columns":["awsRegion","capital","code","currencies","currency" |
| countries | language es | `countries_language` | PASS | rendered size=19.9KB (<50KB cap); {"data":{"language":{"code":"es","countries":{"columns":["awsRegion","capital","code","currencies","currency", |
| countries | country unknown ZZ | `countries_country` | INFO | {"data":{"country":null}} pinned as $17 |
| countries_fixture | fixture country BR | `countries_fixture_country` | PASS | {"data":{"country":{"awsRegion":"sa-east-1","capital":"Brasília","code":"BR","continent":{"code":"SA","countries":{"columns":[],"constants":{"__typename":"Country"},"rows":[[],[],[],[],[],[],[],[],[],[],[],[],[],[]]},"na |
| countries_fixture | fixture continents | `countries_fixture_continents` | FAIL | over cap: Response exceeded 50KB (actual: 147KB). Use more specific filters, lower limit/per_page, or fetch individual items. |
| script | execute script chaining both imports | `execute(script)` | PASS | {"country":"Canada","pet":"Pet1"} |
| ssrf-probe | bad_unspecified_v4_getinventory not executable | `bad_unspecified_v4_getinventory` | PASS | tool "bad_unspecified_v4_getinventory" not found. Use the search tool to discover available tools |
| ssrf-probe | bad_unspecified_v6_getinventory not executable | `bad_unspecified_v6_getinventory` | PASS | tool "bad_unspecified_v6_getinventory" not found. Use the search tool to discover available tools |
| ssrf-probe | bad_zero_net_getinventory not executable | `bad_zero_net_getinventory` | PASS | tool "bad_zero_net_getinventory" not found. Use the search tool to discover available tools |
| ssrf-probe | bad_cgnat_getinventory not executable | `bad_cgnat_getinventory` | PASS | tool "bad_cgnat_getinventory" not found. Use the search tool to discover available tools |
| ssrf-probe | control 100.128.0.1 (just outside 100.64/10) still accepted | `ok_100_128_getinventory` | INFO | registered (boundary correct); specimport: request failed: Get "http://100.128.0.1:18999/store/inventory": context deadline exceeded (Client.Timeout ex |
| ssrf-probe | probe_dns_loopback (127.0.0.1.nip.io) | `probe_dns_loopback_getinventory` | INFO | still accepted at load (string-only host check); specimport: request failed: Get "http://127.0.0.1.nip.io:18999/store/inventory": dial tcp 198.18.0.1:18999: i/o timeout (Client.Timeout exce |
| ssrf-probe | local canary :18999 not hit | `canary` | PASS | canary GET count before=0 after=0 |
| validation | search bad_unspecified_v4 | `search` | PASS | no tools registered (rejected at load); 0 tools; missing=[]; unexpected=[] |
| validation | search bad_unspecified_v6 | `search` | PASS | no tools registered (rejected at load); 0 tools; missing=[]; unexpected=[] |
| validation | search bad_zero_net | `search` | PASS | no tools registered (rejected at load); 0 tools; missing=[]; unexpected=[] |
| validation | search bad_cgnat | `search` | PASS | no tools registered (rejected at load); 0 tools; missing=[]; unexpected=[] |
| validation | search bad_cgnat_hi | `search` | PASS | no tools registered (rejected at load); 0 tools; missing=[]; unexpected=[] |
| validation | search bad_loopback_v4 | `search` | PASS | no tools registered (rejected at load); 19 tools; missing=[]; unexpected=[] |
| validation | search bad_localhost | `search` | PASS | no tools registered (rejected at load); 0 tools; missing=[]; unexpected=[] |
| validation | search bad_loopback_v6 | `search` | PASS | no tools registered (rejected at load); 19 tools; missing=[]; unexpected=[] |
| validation | search bad_metadata | `search` | PASS | no tools registered (rejected at load); 0 tools; missing=[]; unexpected=[] |
| validation | search bad_metadata_gql | `search` | PASS | no tools registered (rejected at load); 0 tools; missing=[]; unexpected=[] |
| validation | search bad_basecred | `search` | PASS | no tools registered (rejected at load); 0 tools; missing=[]; unexpected=[] |
| validation | search bad_gcp_meta | `search` | PASS | no tools registered (rejected at load); 0 tools; missing=[]; unexpected=[] |
| validation | search bad_scheme | `search` | PASS | no tools registered (rejected at load); 0 tools; missing=[]; unexpected=[] |
| validation | search petstore_yaml | `search` | PASS | no tools registered (rejected at load); 19 tools; missing=[]; unexpected=[] |
| validation | search probe_mapped | `search` | PASS | no tools registered (rejected at load); 19 tools; missing=[]; unexpected=[] |
| validation | reject http://0.0.0.0:18999 [NEW FIX] | `config spec_imports load` | PASS | specimport "bad_unspecified_v4": specimport: invalid base url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject http://[::]:18999 [NEW FIX] | `config spec_imports load` | PASS | specimport "bad_unspecified_v6": specimport: invalid base url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject http://0.1.2.3:18999 (0/8) [NEW FIX] | `config spec_imports load` | PASS | specimport "bad_zero_net": specimport: invalid base url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject http://100.64.0.1:18999 (100.64/10) [NEW FIX] | `config spec_imports load` | PASS | specimport "bad_cgnat": specimport: invalid base url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject http://100.127.255.254:18999 (100.64/10 top) [NEW FIX] | `config spec_imports load` | PASS | specimport "bad_cgnat_hi": specimport: invalid base url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject http://127.0.0.1:18999 | `config spec_imports load` | PASS | specimport "bad_loopback_v4": specimport: invalid base url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject http://localhost:18999 | `config spec_imports load` | PASS | specimport "bad_localhost": specimport: invalid base url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject http://[::1]:18999 | `config spec_imports load` | PASS | specimport "bad_loopback_v6": specimport: invalid base url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject http://[::ffff:127.0.0.1]:18999 | `config spec_imports load` | PASS | specimport "probe_mapped": specimport: invalid base url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject http://169.254.169.254/latest (OpenAPI) | `config spec_imports load` | PASS | specimport "bad_metadata": specimport: invalid base url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject http://169.254.169.254/graphql (GraphQL) | `config spec_imports load` | PASS | specimport "bad_metadata_gql": specimport: invalid base url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject http://metadata.google.internal/x | `config spec_imports load` | PASS | specimport "bad_gcp_meta": specimport: invalid base url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject credentials.base_url=http://127.0.0.1:18999 override | `config spec_imports load` | PASS | specimport "bad_basecred": specimport: invalid base_url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject file:///etc/passwd | `config spec_imports load` | PASS | specimport "bad_scheme": specimport: invalid base url: url must be http or https, got "file" |
| validation | YAML spec via path | `config spec_imports load` | INFO | still rejected: parse openapi json (JSON only); used YAML->JSON conversion |
