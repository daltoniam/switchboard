## Per-tool summary

| Tool / check | Result | Evidence |
|---|---|---|
| Build / unit tests | PASS | go vet, go test (specimport, netpolicy, config, web, cmd/server), go build at 68343a8 |
| search (discovery) | PASS | 19 `petstore_*` tools from upstream Petstore OpenAPI 3; 6 `countries_*` from live introspection; 6 `countries_fixture_*` from testdata; `[write]` tag on POST/PUT/DELETE; rejected imports register no tools |
| Petstore read: findPetsByStatus / getPetById | PASS | live petstore3: available + sold lists; pet 1; 404 surfaced cleanly; missing required `petId` rejected before any request |
| Petstore write: addPet → get → updatePetWithForm → deletePet | PASS* | JSON body POST, form/query update, delete all worked live. *Integer path/query args ≥1e6 go out as floats (`2.8989396e+08`) → upstream 400; worked only when the id was passed as a string |
| Petstore getInventory | INFO | petstore3 returns 500 for /store/inventory even with direct curl (upstream problem); error surfaced cleanly |
| Countries: country (US, JP), countries filter `code in [DE,FR]`, fixture BR | PASS | correct live data; `filter` input objects passed as GraphQL variables |
| Countries: continents / continent / language / countries `continent=OC` | FAIL | `Response exceeded 50KB` (65–539KB). The generated default selection set goes 3 levels deep through cycles (continent→countries→languages→countries…), so list-ish queries can't be used. Script `api.tryCall` hits the same cap |
| execute script chaining across both imports | PASS | `countries_country` + `petstore_getpetbyid` in one script → `{"country":"Canada","pet":"Pet1"}` |
| Loopback / metadata base URL rejected | PASS | 127.0.0.1, localhost, [::1], [::ffff:127.0.0.1], 169.254.169.254 (OpenAPI + GraphQL), metadata.google.internal, `credentials.base_url` loopback override, and file:// all refused at load with a clear error |
| 0.0.0.0 base URL | FAIL | accepted, and execute reached a local canary on :18999. `HostDisallowed` needs `ip.IsUnspecified()` |
| DNS / redirect SSRF | INFO | host check is string-only: no resolve-time or dial-time check, redirects not re-checked. 127.0.0.1.nip.io accepted at load (live hit not shown, because box DNS maps it to 198.18.0.1) |
| YAML spec | INFO | The upstream Petstore `openapi.yaml` is refused (`parse openapi json`). JSON only, so the proof converted it to JSON |

## All steps

| Phase | Step | Tool | Result | Note |
|---|---|---|---|---|
| build | go vet ./specimport ./netpolicy ./config ./web | `go` | PASS | exit 0 |
| build | go test ./specimport ./netpolicy ./config ./web ./cmd/server | `go` | PASS | all 5 packages ok |
| build | go build ./cmd/server | `go` | PASS | dist/switchboard built at 68343a8 |
| search | search petstore | `search` | PASS | 19 tools; missing=[]; unexpected=[] |
| search | search countries | `search` | PASS | 12 tools; missing=[]; unexpected=[] |
| search | search countries_fixture | `search` | PASS | 12 tools; missing=[]; unexpected=[] |
| search | search petstore pet status | `search` | PASS | 10 tools; missing=[]; unexpected=[] |
| petstore | findPetsByStatus available | `petstore_findpetsbystatus` | PASS | type=dict len=3 sample={"columns": ["id", "photoUrls", "tags", "name", "category"], "constants": {"status": "available"}, "rows": [[9, ["url1", "url2"], [{"id": 1, "name": "tag3"}, {"id": 2, "name": "tag |
| petstore | findPetsByStatus sold | `petstore_findpetsbystatus` | PASS | type=list len=5 sample=[{"category": {"id": 1, "name": "Dogs"}, "id": 5, "name": "Dog 2", "photoUrls": ["url1", "url2"], "status": "sold", "tags": [{"id": 1, "name": "tag2"}, {"id": 2, "name": "tag3"}]}, |
| petstore | getPetById 1 | `petstore_getpetbyid` | PASS | {"id": 1, "name": "Pet1", "photoUrls": ["test1", "test2"], "status": "available", "tags": []} |
| petstore | getInventory | `petstore_getinventory` | INFO | upstream petstore3 itself returns 500 for /store/inventory (curl direct: 500 x3) — not a PR issue; error surfaced cleanly: specimport: upstream returned 500 Internal Server Error: {"code":500,"message":"There was an error processing your request. It has been l |
| petstore | addPet | `petstore_addpet` | PASS | {"category": {"id": 1, "name": "Dogs"}, "id": 289893960, "name": "sb-proof-198", "photoUrls": ["https://example.com/p.png"], "status": "pending", "tags": [{"id": 198, "name": "sandbox-e2e"}]} |
| petstore | getPetById (new pet) | `petstore_getpetbyid` | FAIL | "specimport: upstream returned 400 Bad Request: {\"code\":400,\"message\":\"Input error: couldn't convert `2.8989396e+08` to type `class java.lang.Long`\"}" |
| petstore | getPetById (new pet, id as string — workaround) | `petstore_getpetbyid` | PASS | {"category":{"id":1,"name":"Dogs"},"id":289893960,"name":"sb-proof-198","photoUrls":["https://example.com/p.png"],"status":"pending","tags":[{"id":198,"name":"sandbox-e2e"}]} pinned as $19 |
| petstore | updatePetWithForm (query params) | `petstore_updatepetwithform` | PASS | {"category":{"id":1,"name":"Dogs"},"id":289893960,"name":"sb-proof-198-renamed","photoUrls":["https://example.com/p.png"],"status":"sold","tags":[{"id":198,"name":"sandbox-e2e"}]} pinned as $20 |
| petstore | getPetById after update | `petstore_getpetbyid` | PASS | {"category":{"id":1,"name":"Dogs"},"id":289893960,"name":"sb-proof-198-renamed","photoUrls":["https://example.com/p.png"],"status":"sold","tags":[{"id":198,"name":"sandbox-e2e"}]} pinned as $21 |
| petstore | deletePet (cleanup) | `petstore_deletepet` | PASS | Pet deleted pinned as $22 |
| petstore | getPetById 0 (upstream 4xx) | `petstore_getpetbyid` | PASS | specimport: upstream returned 404 Not Found: Pet not found |
| petstore | missing required arg | `petstore_getpetbyid` | PASS | missing required parameter "petId" for tool "petstore_getpetbyid". Required: [petId] |
| countries | country US | `countries_country` | PASS | {"data":{"country":{"awsRegion":"us-east-2","capital":"Washington D.C.","code":"US","continent":{"code":"NA","countries":{"columns":[],"constants":{"__typename":"Country"},"rows":[[],[],[],[],[],[],[],[],[],[],[],[],[],[ |
| countries | country JP | `countries_country` | PASS | {"data":{"country":{"awsRegion":"ap-northeast-1","capital":"Tokyo","code":"JP","continent":{"code":"AS","countries":{"columns":[],"constants":{"__typename":"Country"},"rows":[[],[],[],[],[],[],[],[],[],[],[],[],[],[],[], |
| countries | countries filter continent=OC | `countries_countries` | FAIL | Response exceeded 50KB (actual: 68KB). Use more specific filters, lower limit/per_page, or fetch individual items. |
| countries | countries filter code in [DE,FR] | `countries_countries` | PASS | {"data":{"countries":[{"awsRegion":"eu-central-1","capital":"Berlin","code":"DE","continent":{"code":"EU","countries":[{"__typename":"Country"},{"__typename":"Country"},{"__typename":"Country"},{"__typename":"Country"},{ |
| countries | continents | `countries_continents` | FAIL | Response exceeded 50KB (actual: 539KB). Use more specific filters, lower limit/per_page, or fetch individual items. |
| countries | continent EU | `countries_continent` | FAIL | Response exceeded 50KB (actual: 65KB). Use more specific filters, lower limit/per_page, or fetch individual items. |
| countries | language es | `countries_language` | FAIL | Response exceeded 50KB (actual: 64KB). Use more specific filters, lower limit/per_page, or fetch individual items. |
| countries | country unknown ZZ | `countries_country` | INFO | {"data":{"country":null}} pinned as $30 |
| countries_fixture | fixture country BR | `countries_fixture_country` | PASS | {"data":{"country":{"awsRegion":"sa-east-1","capital":"Brasília","code":"BR","continent":{"code":"SA","countries":{"columns":[],"constants":{"__typename":"Country"},"rows":[[],[],[],[],[],[],[],[],[],[],[],[],[],[]]},"na |
| countries_fixture | fixture continents | `countries_fixture_continents` | FAIL | Response exceeded 50KB (actual: 539KB). Use more specific filters, lower limit/per_page, or fetch individual items. |
| countries | countries_continents via script api.tryCall (script path) | `countries_continents` | INFO | script api.tryCall hits the same 50KB cap — the generated selection set itself is too large |
| countries | countries_continent via script api.tryCall (script path) | `countries_continent` | INFO | script api.tryCall hits the same 50KB cap — the generated selection set itself is too large |
| countries | countries_language via script api.tryCall (script path) | `countries_language` | INFO | script api.tryCall hits the same 50KB cap — the generated selection set itself is too large |
| countries | countries_countries via script api.tryCall (script path) | `countries_countries` | INFO | script api.tryCall hits the same 50KB cap — the generated selection set itself is too large |
| script | execute script chaining both imports | `execute(script)` | PASS | {"country":"Canada","pet":"Pet1"} |
| ssrf-probe | probe_unspecified getinventory reaches local canary? | `probe_unspecified_getinventory` | INFO | specimport: upstream returned 404 File not found: <!DOCTYPE HTML> <html lang="en">     <head>         <meta charset="utf-8">         <title>Error response</title>     </head>     <body>         <h1>Er |
| ssrf-probe | probe_dns_loopback getinventory reaches local canary? | `probe_dns_loopback_getinventory` | INFO | specimport: request failed: Get "http://127.0.0.1.nip.io:18999/store/inventory": context deadline exceeded (Client.Timeout exceeded while awaiting headers) |
| validation | search bad_loopback_v4 | `search` | PASS | no tools registered for this import (rejected at load) — 19 tools; missing=[]; unexpected=[] |
| validation | search bad_localhost | `search` | PASS | no tools registered for this import (rejected at load) — 0 tools; missing=[]; unexpected=[] |
| validation | search bad_loopback_v6 | `search` | PASS | no tools registered for this import (rejected at load) — 19 tools; missing=[]; unexpected=[] |
| validation | search bad_metadata | `search` | PASS | no tools registered for this import (rejected at load) — 0 tools; missing=[]; unexpected=[] |
| validation | search bad_metadata_gql | `search` | PASS | no tools registered for this import (rejected at load) — 0 tools; missing=[]; unexpected=[] |
| validation | search bad_basecred | `search` | PASS | no tools registered for this import (rejected at load) — 0 tools; missing=[]; unexpected=[] |
| validation | search bad_gcp_meta | `search` | PASS | no tools registered for this import (rejected at load) — 0 tools; missing=[]; unexpected=[] |
| validation | search bad_scheme | `search` | PASS | no tools registered for this import (rejected at load) — 0 tools; missing=[]; unexpected=[] |
| validation | search petstore_yaml | `search` | PASS | no tools registered for this import (rejected at load) — 19 tools; missing=[]; unexpected=[] |
| validation | search probe_mapped | `search` | PASS | no tools registered for this import (rejected at load) — 38 tools; missing=[]; unexpected=[] |
| validation | reject http://127.0.0.1:18999 | `config spec_imports load` | PASS | specimport "bad_loopback_v4": specimport: invalid base url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject http://localhost:18999 | `config spec_imports load` | PASS | specimport "bad_localhost": specimport: invalid base url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject http://[::1]:18999 | `config spec_imports load` | PASS | specimport "bad_loopback_v6": specimport: invalid base url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject http://[::ffff:127.0.0.1]:18999 | `config spec_imports load` | PASS | specimport "probe_mapped": specimport: invalid base url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject http://169.254.169.254/latest (OpenAPI) | `config spec_imports load` | PASS | specimport "bad_metadata": specimport: invalid base url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject http://169.254.169.254/graphql (GraphQL) | `config spec_imports load` | PASS | specimport "bad_metadata_gql": specimport: invalid base url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject http://metadata.google.internal/x | `config spec_imports load` | PASS | specimport "bad_gcp_meta": specimport: invalid base url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject credentials.base_url=http://127.0.0.1:18999 override | `config spec_imports load` | PASS | specimport "bad_basecred": specimport: invalid base_url: requests to private, loopback, link-local, or metadata addresses are not allowed |
| validation | reject file:///etc/passwd | `config spec_imports load` | PASS | specimport "bad_scheme": specimport: invalid base url: url must be http or https, got "file" |
| validation | YAML spec via path (upstream openapi.yaml as-is) | `config spec_imports load` | INFO | rejected: specimport "petstore_yaml": specimport: parse openapi json: invalid character 'o' looking for beginning of value — JSON only; converted YAML->JSON for the proof |
| validation | reject http://0.0.0.0:18999 | `config + probe_unspecified_getinventory` | FAIL | ACCEPTED at load and the execute reached a local canary on :18999 (canary log: GET /store/inventory from 127.0.0.1). netpolicy.HostDisallowed has no IsUnspecified() check |
| validation | hostname that resolves to loopback (127.0.0.1.nip.io) | `config + probe_dns_loopback_getinventory` | INFO | accepted at load (host check is string-only, no DNS resolution / dial-time check, redirects not re-validated); box DNS maps it to 198.18.0.1 so the live hit could not be shown here |
