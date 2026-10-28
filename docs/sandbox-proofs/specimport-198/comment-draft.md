Sandbox E2E post-rebase regression @ 3bbe246: 62 PASS / 0 FAIL / 8 INFO (was 62/0/7 @ d39e116).

- ✅ Rebase only: `specimport/` and `netpolicy/` are unchanged vs d39e116; go vet + go test (specimport, netpolicy, config, web, cmd/server) + go build pass on the rebased tree.
- ✅ No regressions: GraphQL lists still under the 50KB cap (continents 0.3KB, countries(continent=OC) 3.3KB, fixture 0.3KB); SSRF 14/14 rejected at load, canary not hit; integer ids ≥1e6 live; Petstore CRUD; script chaining.
- ℹ️ Public petstore3 deleted pet 1 since v3 (direct curl → 404), so the harness now seeds getPetById/chaining from a live `available` pet. Upstream data drift, not a PR change.

Still merge-ready from the sandbox side. INFO items unchanged: DNS/redirect SSRF (string-only host check), no YAML, array query params, nested object lists not selected. Details + video: docs/sandbox-proofs/specimport-198/ on sandbox-e2e-proofs.
