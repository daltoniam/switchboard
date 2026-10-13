### Sandbox E2E proof

**Not merge-ready on `68343a8` from the sandbox side:** OpenAPI import works against live Petstore, but a 0.0.0.0 base URL gets past the SSRF guard to loopback, and generated GraphQL list queries are over 50KB. A large-integer path-param bug is also open. Head `68343a87962ee3a88e73db7044b0ee823f5e7062`. Native HTTP (OpenAPI REST + GraphQL POST), no auth (public APIs). Live upstreams: petstore3.swagger.io, countries.trevorblades.com.

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

Other notes (minor):
- `search` lists every enabled `spec_imports` entry under `integrations`, including ones that failed to load (`EnabledIntegrations` reads config, not the registry). No tools are registered for them.
- Only the server log shows load errors. An agent sees the integration name but no reason it's missing.
- `api.tryCall` on a GraphQL tool returns `{"data":{"data":…}}` (double-wrapped).
- `[write]` GraphQL mutations weren't exercised (Countries has none).

Artifacts: `sandbox-e2e-proofs` → `docs/sandbox-proofs/specimport-198/` (REPORT.md, pass-fail.md, proof.mp4, responses.json, mcp-responses.json, specs/, screenshots/).
