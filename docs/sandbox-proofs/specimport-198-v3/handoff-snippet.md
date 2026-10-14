### Sandbox E2E proof (re-run 2)

**Merge-ready on `d39e116` from the sandbox side:** all 3 GraphQL list queries that failed at 78ca8f0 are now well under the 50KB cap, and SSRF, integer ids, Petstore CRUD, lookups and chaining all still pass, with no regressions. The open items are INFO only. Head `d39e116ae221e3ea39156192ff8d2d2c9f79a48d`; earlier proof on `78ca8f0`. Native HTTP (OpenAPI REST + GraphQL POST), no auth. Live upstreams: petstore3.swagger.io, countries.trevorblades.com.

build + MCP E2E: 62 PASS / 0 FAIL / 7 INFO

| Finding | Before (78ca8f0) | After (d39e116) | Status |
|---|---|---|---|
| countries_continents | FAIL: 147KB (539KB at 68343a8) | PASS: 0.3KB, all 7 continents with code + name | **FIXED** |
| countries_countries(filter continent=OC) | FAIL: 68KB | PASS: 3.3KB, Oceania countries with scalar fields | **FIXED** |
| countries_fixture_continents (testdata spec) | FAIL: 147KB | PASS: 0.3KB | **FIXED** |
| Selections still useful | continent EU 19.5KB incl. nested countries; country included continent + languages | continent EU → `{code:"EU",name:"Europe"}`; language es → code/name/native/rtl; country US/JP/BR → scalars + `continent{code,name}`. Nested list-of-object fields (continent.countries, country.languages/states/subdivisions) are no longer selected | PASS (trade-off: INFO) |

| Tool / check | Result | Evidence |
|---|---|---|
| Build / unit tests | PASS | go vet, go test (specimport, netpolicy, config, web, cmd/server), go build at d39e116 |
| search (discovery) | PASS | 19 `petstore_*`, 6 `countries_*` (live introspection), 6 `countries_fixture_*`; rejected imports register no tools |
| Petstore read: findPetsByStatus / getPetById / findPetsByTags (string) | PASS | live lists; pet 1; 404 surfaced; missing `petId` rejected before any request |
| Petstore write: addPet → getPetById → updatePetWithForm → deletePet (id ≥1e6) | PASS | id 289951065 sent as an integer in path and query; full cycle live; test pet deleted (GET → `Pet not found`) |
| Petstore getInventory | INFO | upstream petstore3 returns 500 even with direct curl |
| Countries single lookups: country US/JP/ZZ, fixture BR, continent EU, language es, countries `code in [DE,FR]` | PASS | correct live data; country includes `continent{code,name}` |
| Countries lists: continents / countries `continent=OC` / fixture continents | PASS | 0.3KB / 3.3KB / 0.3KB, all under the 50KB cap (were 147 / 68 / 147KB) |
| execute script chaining across both imports | PASS | `{"country":"Canada","pet":"Pet1"}` |
| Base URL validation: loopback / metadata / scheme | PASS | 127.0.0.1, localhost, [::1], [::ffff:127.0.0.1], 169.254.169.254 (OpenAPI+GraphQL), metadata.google.internal, base_url override, file:// rejected |
| Base URL validation: 0.0.0.0, ::, 0/8, 100.64/10 | PASS | all rejected at load; local canary not hit (0 requests); 100.128.0.1 boundary still allowed |
| DNS / redirect SSRF | INFO | unchanged: the host check is string-only; 127.0.0.1.nip.io accepted at load; no dial-time or redirect check |
| YAML spec / array query params | INFO | unchanged: JSON specs only; `tags:["tag1"]` rejected client-side |

Trade-off (INFO): nested lists of objects are now never selected, so `continent` no longer returns its countries and `country` no longer returns languages/states/subdivisions. Getting those takes a second call, e.g. `countries(filter:{continent:{eq:"EU"}})`. A future option is a caller-supplied `fields`/selection argument.

Still open (INFO): the host check doesn't resolve DNS or re-check redirects; YAML specs aren't supported; array query params are refused client-side; `search` lists imports that failed to load; load errors appear only in the server log; GraphQL `api.tryCall` is double-wrapped (`data.data`). The getInventory 500 comes from upstream petstore3.

Artifacts: `sandbox-e2e-proofs` → `docs/sandbox-proofs/specimport-198-v3/` (REPORT.md, pass-fail.md, proof.mp4, responses.json, mcp-responses.json, screenshots/).
