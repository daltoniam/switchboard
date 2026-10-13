Sandbox E2E re-run @ 78ca8f0: 58 PASS / 3 FAIL / 8 INFO (was 41/7/10 @ 68343a8).

- ✅ SSRF: 0.0.0.0, [::], 0.x, 100.64.x are now rejected at load, and the local canary wasn't hit.
- ✅ Integer ids: addPet with id 289910702 → getPetById → update → delete all worked live; ids go out as integers.
- ⚠️ GraphQL depth 2: continent and language now fit (about 20KB), but `continents` (147KB) and `countries(filter continent=OC)` (68KB) are still over the 50KB cap.

No regressions. Still open (INFO): DNS/redirect SSRF, no YAML, array query params. Details + video: docs/sandbox-proofs/specimport-198-v2/ on sandbox-e2e-proofs.
