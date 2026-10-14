Sandbox E2E re-run @ d39e116: 62 PASS / 0 FAIL / 7 INFO (was 58/3/8 @ 78ca8f0).

- ✅ GraphQL list queries are now under the 50KB cap: continents 0.3KB (was 147KB), countries(continent=OC) 3.3KB (was 68KB), fixture continents 0.3KB. Single lookups still return useful data (continent EU → code + name; country → scalars + continent).
- ✅ No regressions: SSRF (0.0.0.0/::/0.x/100.64 rejected, canary not hit), integer ids ≥1e6 live, Petstore CRUD, script chaining.

Merge-ready from the sandbox side. INFO: nested object lists (continent.countries, country.languages) are no longer selected; DNS/redirect SSRF; no YAML; array query params. Details + video: docs/sandbox-proofs/specimport-198-v3/ on sandbox-e2e-proofs.
