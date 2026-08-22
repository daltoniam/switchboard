# Phase 2 catalog core evidence

## RED

`TestRevision_*` and `TestCatalog_*` were added before the catalog ports existed. The first compile failure under the leftover v1.5-era store was missing `HashDefinition`, `Catalog`, and typed conflict errors.

## GREEN

```bash
go test ./project -run 'TestRevision|TestCatalog_|TestLock_' -count=1
go test -race ./project -count=1
```

Both pass. Filesystem JSON under `$configRoot/projects/<id>.project.json` remains authoritative. Revision archives are derived immutable snapshots under `$configRoot/revisions/<id>/<digest>.json`. Cross-process advisory locking uses `github.com/gofrs/flock` on `$configRoot/.catalog.lock`.

## Notes

- Existing `Store.Create/Get/Delete` names collided with the frozen catalog ports; compatibility helpers are `CreateDefinition`, `Definition`, and `DeleteDefinition`.
- Compatibility last-write-wins remains on `CreateCompatibility` / `PatchCompatibility` / `DeleteCompatibility`.
- Context reads now reject symlink escapes that leave the declared root.
