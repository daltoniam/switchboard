# Sandbox E2E proof: daltoniam/switchboard PR #198 (OpenAPI/GraphQL spec import)

- Head: `68343a87962ee3a88e73db7044b0ee823f5e7062` (branch `feat/spec-imports`, matches the expected 68343a8)
- Run: 2026-10-13 07:20 CDT, Linux box, isolated `SWITCHBOARD_DEV_HOME`, Switchboard on 127.0.0.1:13853, driven over MCP (`search` / `execute`)
- Config: `spec_imports` entries in config.json using `path` + `endpoint` (no fetch-from-URL in the PR)
  - `petstore`: openapi, upstream swagger-petstore `openapi.yaml` converted to JSON (`specs/petstore-openapi.json`), endpoint `https://petstore3.swagger.io/api/v3`
  - `countries`: graphql, live introspection of countries.trevorblades.com (`specs/countries-introspection.json`)
  - `countries_fixture`: graphql, `testdata/specimport/countries_graphql.json`
  - 12 negative/probe entries (loopback, metadata, scheme, YAML, 0.0.0.0, DNS)
- Auth: none. Both APIs are public, and no secrets were used.

## Verdict

**Not merge-ready on `68343a8` from the sandbox side:** OpenAPI import works against live Petstore, but a 0.0.0.0 base URL gets past the SSRF guard to loopback, and generated GraphQL list queries are over 50KB. A large-integer path-param bug is also open.

MCP E2E + build: 41 PASS / 7 FAIL / 10 INFO

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

## Bugs / findings

1. **SSRF: `0.0.0.0` accepted (FAIL).** `netpolicy.HostDisallowed` checks Loopback/Private/LinkLocal but not `IsUnspecified()`. On Linux, 0.0.0.0 dials localhost. The canary on :18999 logged `GET /store/inventory` from Switchboard. Fix: add `ip.IsUnspecified()` (and consider blocking `0.0.0.0/8`, CGNAT `100.64/10`).
2. **SSRF: string-only host check (INFO, design).** Hostnames that resolve to private IPs (e.g. `127.0.0.1.nip.io`, or DNS rebinding) and 3xx redirects to internal hosts aren't checked. Recommend a `net.Dialer.Control` check on the resolved IP plus `CheckRedirect` re-validation.
3. **GraphQL default selection too large (FAIL).** `selectionBuilder` recurses 3 levels through cyclic types. `countries_continents` comes back at 539KB, and continent/language/countries(filter) at 64–68KB, all over Switchboard's 50KB response cap. Fix: depth 1–2, skip list-of-object fields at nested levels, or allow a caller-supplied `fields`/selection.
4. **Integer path/query args formatted as floats (FAIL, partial).** `fmt.Sprint(v)` on a JSON float64 makes `289893960` into `2.8989396e+08`, so getPetById/updatePetWithForm/deletePet get a 400 for any id ≥ 1e6. Fix: format whole floats with `strconv.FormatFloat(v,'f',-1,64)` or decode with `UseNumber`. Passing the id as a string works around it.
5. **YAML not supported (INFO).** The upstream Petstore spec is YAML, and the parser is JSON-only. Either accept YAML or document "JSON only" in the UI.
6. - `search` lists every enabled `spec_imports` entry under `integrations`, including ones that failed to load (`EnabledIntegrations` reads config, not the registry). No tools are registered for them.
   - Only the server log shows load errors. An agent sees the integration name but no reason it's missing.
   - `api.tryCall` on a GraphQL tool returns `{"data":{"data":…}}` (double-wrapped).
   - `[write]` GraphQL mutations weren't exercised (Countries has none).

## Petstore cleanup
The test pet `sb-proof-198` (id in responses.json) was created, updated, then deleted on the public petstore3 demo. Final delete returned `Pet deleted`.

## Files
REPORT.md, pass-fail.md, handoff-snippet.md, responses.json (per step), mcp-responses.json (raw MCP results), harness-run.log, go-test.log, go-vet.log, build.log, switchboard-a.log, specs/, screenshots/, proof.mp4, run-e2e.py, mcpc.py, finalize.py, leak-scan.txt
