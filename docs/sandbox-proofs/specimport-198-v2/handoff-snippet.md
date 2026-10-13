### Sandbox E2E proof (re-run)

**Not merge-ready yet on `78ca8f0` from the sandbox side:** the SSRF and integer-id fixes are confirmed, but the GraphQL depth fix is partial. `countries_continents` (147KB) and `countries_countries(continent=OC)` (68KB) are still over the 50KB cap. Everything else passes, with no regressions. Head `78ca8f0aea871738e0501bf6f45337c0f4e36462`; earlier proof on `68343a8`. Native HTTP (OpenAPI REST + GraphQL POST), no auth. Live upstreams: petstore3.swagger.io, countries.trevorblades.com.

build + MCP E2E: 58 PASS / 3 FAIL / 8 INFO

| Finding | Before (68343a8) | After (78ca8f0) | Status |
|---|---|---|---|
| 1. SSRF: 0.0.0.0 / :: / 0/8 / 100.64/10 | FAIL: `http://0.0.0.0:18999` accepted; execute hit the local canary (`GET /store/inventory` from 127.0.0.1). `::`, 0/8, 100.64/10 were not blocked either | PASS: 0.0.0.0, [::], 0.1.2.3, 100.64.0.1, 100.127.255.254 all rejected at load; tools not registered (`tool not found`); canary GET count 0. 100.128.0.1 (just outside 100.64/10) still accepted, so the boundary is correct | **FIXED** |
| 2. GraphQL selection depth vs 50KB cap | FAIL: continents 539KB, continent EU 65KB, language es 64KB, countries(continent=OC) 68KB | continent EU 19.5KB PASS, language es 19.9KB PASS; **continents 147KB FAIL, countries(continent=OC) 68KB FAIL** (fixture continents also 147KB) | **PARTIAL**: 2 of 4 now fit; the list queries are still over the cap |
| 3. Integer ids ≥1e6 in path/query | FAIL: `289893960` sent as `2.8989396e+08` → upstream 400 on getPetById/update/delete; worked only as a string | PASS: addPet id 289910702 → getPetById (JSON number) 200 → updatePetWithForm → get shows renamed/sold → deletePet `Pet deleted` (cleanup verified: GET now `Pet not found`) | **FIXED** |

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

Ideas for the list-query sizes: depth 1 for list-returning roots, skip nested list-of-object fields, or let the caller pass a `fields`/selection argument.

Still open (INFO): the host check doesn't resolve DNS or re-check redirects; YAML specs aren't supported; array query params are refused client-side; `search` lists imports that failed to load; load errors appear only in the server log; GraphQL `api.tryCall` is double-wrapped (`data.data`). getInventory/findPetsByTags 500s come from upstream petstore3.

Artifacts: `sandbox-e2e-proofs` → `docs/sandbox-proofs/specimport-198-v2/` (REPORT.md, pass-fail.md, proof.mp4, responses.json, mcp-responses.json, screenshots/).
