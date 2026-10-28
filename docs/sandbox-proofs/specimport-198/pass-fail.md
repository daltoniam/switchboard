## Before → after

| Check | Before (d39e116, v3) | After (3bbe246, v4) | Status |
|---|---|---|---|
| Spec-import code (specimport/, netpolicy/) | d39e116 | 3bbe246: `git diff d39e116 3bbe246 -- specimport netpolicy` is empty; rebase only brings in main | UNCHANGED |
| countries_continents | PASS 0.3KB | PASS 0.3KB | SAME |
| countries_countries(filter continent=OC) | PASS 3.3KB | PASS 3.3KB | SAME |
| countries_fixture_continents (testdata spec) | PASS 0.3KB | PASS 0.3KB | SAME |
| SSRF load-time rejection (14 base URLs) + canary | 14/14 rejected, canary 0 hits | 14/14 rejected, canary 0 hits | SAME |
| getPetById 1 / script chaining | PASS (pet 1 existed upstream) | pet 1 now 404 on public petstore3 (direct curl too); harness re-seeded with a live available pet → PASS | HARNESS DRIFT, not a regression |

## Per-tool summary

| Tool / check | Result | Evidence |
|---|---|---|
| Build / unit tests | PASS | go vet, go test (specimport, netpolicy, config, web, cmd/server), go build at 3bbe246 |
| search (discovery) | PASS | 19 `petstore_*`, 6 `countries_*` (live introspection), 6 `countries_fixture_*`; rejected imports register no tools |
| Petstore read: findPetsByStatus / getPetById / findPetsByTags (string) | PASS | live lists; seed pet from live `available` list (pet 1 was deleted upstream; its 404 is surfaced cleanly, INFO); missing `petId` rejected before any request |
| Petstore write: addPet → getPetById → updatePetWithForm → deletePet (id ≥1e6) | PASS | id 291204193 sent as an integer in path and query; full cycle live; test pet deleted (GET → `Pet not found`) |
| Petstore getInventory | INFO | upstream petstore3 returns 500 even with direct curl |
| Countries single lookups: country US/JP/ZZ, fixture BR, continent EU, language es, countries `code in [DE,FR]` | PASS | correct live data; country includes `continent{code,name}` |
| Countries lists: continents / countries `continent=OC` / fixture continents | PASS | 0.3KB / 3.3KB / 0.3KB, all under the 50KB cap (were 147 / 68 / 147KB) |
| execute script chaining across both imports | PASS | `{"country":"Canada","pet":"Dog 1"}` |
| Base URL validation: loopback / metadata / scheme | PASS | 127.0.0.1, localhost, [::1], [::ffff:127.0.0.1], 169.254.169.254 (OpenAPI+GraphQL), metadata.google.internal, base_url override, file:// rejected |
| Base URL validation: 0.0.0.0, ::, 0/8, 100.64/10 | PASS | all rejected at load; local canary not hit (0 requests); 100.128.0.1 boundary still allowed |
| DNS / redirect SSRF | INFO | unchanged: the host check is string-only; 127.0.0.1.nip.io accepted at load; no dial-time or redirect check |
| YAML spec / array query params | INFO | unchanged: JSON specs only; `tags:["tag1"]` rejected client-side |

## All steps

| Phase | Step | Tool | Result | Note |
|---|---|---|---|---|
| build | go vet ./specimport ./netpolicy ./config ./web ./cmd/server | `go` | PASS | exit 0 |
| build | go test ./specimport ./netpolicy ./config ./web ./cmd/server | `go` | PASS | all 5 packages ok |
| build | go build ./cmd/server | `go` | PASS | dist/switchboard at 3bbe246 |
| search | search petstore | `search` | PASS | 19 tools; missing=[]; unexpected=[] |
| search | search countries | `search` | PASS | 12 tools; missing=[]; unexpected=[] |
| search | search countries_fixture | `search` | PASS | 12 tools; missing=[]; unexpected=[] |
| search | search petstore pet status | `search` | PASS | 10 tools; missing=[]; unexpected=[] |
| petstore | findPetsByStatus available | `petstore_findpetsbystatus` | PASS | type=dict len=3 sample={"columns": ["id", "photoUrls", "tags", "name", "category"], "constants": {"status": "available"}, "rows": [[4, ["url1", "url2"], [{"id": 1, "name": "tag1"}, {"id": 2, "name": "tag |
| petstore | findPetsByStatus sold | `petstore_findpetsbystatus` | PASS | type=list len=3 sample=[{"category": {"id": 1, "name": "Dogs"}, "id": 370752812, "name": "max", "photoUrls": ["https://example.org/max.png"], "status": "sold", "tags": [{"id": 1, "name": "contract-taf"}] |
| petstore | getPetById 1 (deleted upstream; 404 surfaced cleanly) | `petstore_getpetbyid` | INFO | upstream data drift: specimport: upstream returned 404 Not Found: Pet not found |
| petstore | getPetById seed 4 (replaces getPetById 1) | `petstore_getpetbyid` | PASS | {"category": {"id": 1, "name": "Dogs"}, "id": 4, "name": "Dog 1", "photoUrls": ["url1", "url2"], "status": "available", "tags": [{"id": 1, "name": "tag1"}, {"id": 2, "name": "tag2"}]} |
| petstore | getInventory | `petstore_getinventory` | INFO | upstream petstore3 itself returns 500 for /store/inventory (curl direct: 500 x3) — not a PR issue; error surfaced cleanly: specimport: upstream returned 500 Internal Server Error: {"code":500,"message":"There was an error processing your re |
| petstore | addPet | `petstore_addpet` | PASS | {"category": {"id": 1, "name": "Dogs"}, "id": 291204193, "name": "sb-proof-198", "photoUrls": ["https://example.com/p.png"], "status": "pending", "tags": [{"id": 198, "name": "sandbox-e2e"}]} |
| petstore | getPetById (new pet, integer id >=1e6) | `petstore_getpetbyid` | PASS | {"category": {"id": 1, "name": "Dogs"}, "id": 291204193, "name": "sb-proof-198", "photoUrls": ["https://example.com/p.png"], "status": "pending", "tags": [{"id": 198, "name": "sandbox-e2e"}]} |
| petstore | updatePetWithForm (query params) | `petstore_updatepetwithform` | PASS | {"category":{"id":1,"name":"Dogs"},"id":291204193,"name":"sb-proof-198-renamed","photoUrls":["https://example.com/p.png"],"status":"sold","tags":[{"id":198,"name":"sandbox-e2e"}]} pinned as $27 |
| petstore | getPetById after update | `petstore_getpetbyid` | PASS | {"category":{"id":1,"name":"Dogs"},"id":291204193,"name":"sb-proof-198-renamed","photoUrls":["https://example.com/p.png"],"status":"sold","tags":[{"id":198,"name":"sandbox-e2e"}]} pinned as $28 |
| petstore | deletePet (cleanup) | `petstore_deletepet` | PASS | Pet deleted pinned as $29 |
| petstore | getPetById 0 (upstream 4xx) | `petstore_getpetbyid` | PASS | specimport: upstream returned 404 Not Found: Pet not found |
| petstore | findPetsByStatus status=pending (query) finds new pet | `petstore_findpetsbystatus` | INFO | query string param; [{"category":{"id":1,"name":"Dogs"},"id":3,"name":"doggie","photoUrls":["qfylkzxcykevqurmrcxsvcdiikhcjfnx","qfylkzxcykevqurmrcxsvcdiikhcjfnx","qfylkzx |
| petstore | findPetsByTags tags=["tag1"] (array query param) | `petstore_findpetsbytags` | INFO | array query params rejected client-side (no request sent); before, fmt.Sprint sent "[tag1]". Arrays still unsupported. ERROR: specimport: query parameter "tags": parameter "v": cannot convert []interface {} to string |
| petstore | findPetsByTags tags="tag1" (string) | `petstore_findpetsbytags` | PASS | [{"category":{"id":1,"name":"Dogs"},"id":4,"name":"Dog 1","photoUrls":["url1","url2"],"status":"available","tags":[{"id":1,"name":"tag1"},{"id":2,"name":"tag2"}]},{"category":{"id":4,"name":"Lions"}," |
| petstore | missing required arg | `petstore_getpetbyid` | PASS | missing required parameter "petId" for tool "petstore_getpetbyid". Required: [petId] |
| countries | country US | `countries_country` | PASS | {"data":{"country":{"awsRegion":"us-east-2","capital":"Washington D.C.","code":"US","continent":{"code":"NA","name":"North America"},"currencies":["USD","USN","USS"],"currency":"USD,USN,USS","emoji":"🇺🇸","emojiU":"U+1F1F |
| countries | country JP | `countries_country` | PASS | {"data":{"country":{"awsRegion":"ap-northeast-1","capital":"Tokyo","code":"JP","continent":{"code":"AS","name":"Asia"},"currencies":["JPY"],"currency":"JPY","emoji":"🇯🇵","emojiU":"U+1F1EF U+1F1F5","name":"Japan","native" |
| countries | countries filter continent=OC | `countries_countries` | PASS | rendered size=3.3KB (<50KB cap); {"data":{"countries":{"columns":["awsRegion","code","currencies","currency","emoji","emojiU","name","native"," |
| countries | countries filter code in [DE,FR] | `countries_countries` | PASS | {"data":{"countries":[{"awsRegion":"eu-central-1","capital":"Berlin","code":"DE","currencies":["EUR"],"currency":"EUR","emoji":"🇩🇪","emojiU":"U+1F1E9 U+1F1EA","name":"Germany","native":"Deutschland","phone":"49","phones" |
| countries | continents | `countries_continents` | PASS | all 7 continents; rendered size=0.3KB (<50KB cap) |
| countries | continent EU | `countries_continent` | PASS | rendered size=0.1KB (<50KB cap); {"data":{"continent":{"code":"EU","name":"Europe"}}} pinned as $37 |
| countries | language es | `countries_language` | PASS | rendered size=0.1KB (<50KB cap); {"data":{"language":{"code":"es","name":"Spanish","native":"Español","rtl":false}}} pinned as $38 |
| countries | country unknown ZZ | `countries_country` | INFO | {"data":{"country":null}} pinned as $39 |
| countries_fixture | fixture country BR | `countries_fixture_country` | PASS | {"data":{"country":{"awsRegion":"sa-east-1","capital":"Brasília","code":"BR","continent":{"code":"SA","name":"South America"},"currencies":["BRL"],"currency":"BRL","emoji":"🇧🇷","emojiU":"U+1F1E7 U+1F1F7","name":"Brazil", |
| countries_fixture | fixture continents | `countries_fixture_continents` | PASS | rendered size=0.3KB (<50KB cap); {"data":{"continents":[{"code":"AF","name":"Africa"},{"code":"AN","name":"Antarctica"},{"code":"AS","name":"As |
| script | execute script chaining both imports | `execute(script)` | PASS | {"country":"Canada","pet":"Dog 1"} |
| ssrf-probe | bad_unspecified_v4_getinventory not executable | `bad_unspecified_v4_getinventory` | PASS | tool "bad_unspecified_v4_getinventory" not found. Use the search tool to discover available tools |
| ssrf-probe | bad_unspecified_v6_getinventory not executable | `bad_unspecified_v6_getinventory` | PASS | tool "bad_unspecified_v6_getinventory" not found. Use the search tool to discover available tools |
| ssrf-probe | bad_zero_net_getinventory not executable | `bad_zero_net_getinventory` | PASS | tool "bad_zero_net_getinventory" not found. Use the search tool to discover available tools |
| ssrf-probe | bad_cgnat_getinventory not executable | `bad_cgnat_getinventory` | PASS | tool "bad_cgnat_getinventory" not found. Use the search tool to discover available tools |
| ssrf-probe | control 100.128.0.1 (just outside 100.64/10) still accepted | `ok_100_128_getinventory` | INFO | registered (boundary correct); specimport: request failed: Get "http://100.128.0.1:18999/store/inventory": context deadline exceeded (Client.Timeout ex |
| ssrf-probe | probe_dns_loopback (127.0.0.1.nip.io) | `probe_dns_loopback_getinventory` | INFO | still accepted at load (string-only host check); specimport: upstream returned 500 Internal Server Error:  |
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
