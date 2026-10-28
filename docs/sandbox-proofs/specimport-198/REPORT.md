# Sandbox E2E proof (post-rebase regression): daltoniam/switchboard PR #198 spec import

- Head: `3bbe246552d118b5226909de25dfedd7de4e46fb` (branch feat/spec-imports, rebased onto main). Earlier proof: `d39e116` → ../specimport-198-v3/ (62 PASS / 0 FAIL / 7 INFO)
- Spec-import code diff vs d39e116: `git diff d39e116 3bbe246 -- specimport netpolicy` is **empty**; the rebase brings in main (143 files elsewhere).
- Run: 2026-10-28 11:17 CDT; same harness, specs and config as v3 (paths re-pointed to v4); isolated HOME / SWITCHBOARD_DEV_HOME = homes/a; MCP on 127.0.0.1:13853; canary on 127.0.0.1:18999
- Native HTTP (OpenAPI REST + GraphQL POST), no auth. Live upstreams: petstore3.swagger.io, countries.trevorblades.com. No Docker used.
- No GitHub posting, commits or pushes from this run.

## Verdict
**Still merge-ready on `3bbe246` (post-rebase) from the sandbox side:** spec-import code is byte-identical to d39e116, build/vet/tests pass on the rebased tree, and SSRF, integer ids, Petstore CRUD, GraphQL sizes and chaining all still pass. No regressions; open items are INFO only.

build + MCP E2E: 62 PASS / 0 FAIL / 8 INFO

## v3 → v4 comparison
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

## Regressions vs d39e116
v3 PASS steps not passing now: none.

Harness change (disclosed): attempt 1 (`harness-run-attempt1.log`, `logs/*-attempt1.json`) had 2 FAILs, `getPetById 1` and script chaining, both because pet 1 was deleted on the shared public petstore3 (direct `curl /api/v3/pet/1` → 404, outside Switchboard). The harness now seeds those two calls from the first live `available` pet and keeps the pet-1 call as an INFO step showing the 404 is surfaced cleanly. The attempt-1 run had no other differences.

## INFO / caveats
Still open (INFO): the host check doesn't resolve DNS or re-check redirects; YAML specs aren't supported; array query params are refused client-side; `search` lists imports that failed to load; load errors appear only in the server log; GraphQL `api.tryCall` is double-wrapped (`data.data`). The getInventory 500 comes from upstream petstore3.

## Test pet cleanup
Test pet 291204193 (`sb-proof-198`) was created, updated, then deleted; GET afterwards returns `Pet not found`.
