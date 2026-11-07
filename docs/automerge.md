# Auto-merge for bot integration PRs

The scheduled implementation job opens one integration PR at a time and fixes its review findings, but it never merges. The `Auto-merge` workflow (`.github/workflows/automerge.yml`) merges a bot PR once it can show, on its current head commit, that it works and stays inside its lane.

## Gates

`scripts/automerge-decide.sh` holds the PR unless every one of these is true:

| Gate | Why |
|---|---|
| Open, not a draft, same repository, base `main` | Forks and other branches never merge automatically |
| `overload-automation` label, no `automation-hold` label | Only bot PRs; `automation-hold` is the per-PR kill switch |
| Branch `feat/<name>-integration`, `integrations/<name>/` is new on `main` | Only new integrations; changes to shipped ones go to a person |
| `web/live_<name>_test.go` exists | The integration was tested against the real product |
| `cmd/server/main.go` adds its import and `<name>.New()`, and `config/config.go` adds its disabled-by-default entry | The merged integration is actually loaded and configurable |
| Every required check on the `main` ruleset passed (including `overload` and `live-integration`), and no other check run or commit status failed or is still running | The required list is read from rulesets and classic branch protection, so new required checks apply automatically. A check tied to an app only counts runs from that app |
| No unresolved review threads, no changes requested, mergeable and `CLEAN` | Review findings are addressed |
| Files stay in the integration's lane | See below |

Files a PR may change:

- Its own files: `integrations/<name>/`, `docs/<name>*.md`, `web/{<name>_ui,ui_<name>,live_<name>}_test.go`, `config/<name>_test.go`, `server/<name>_test.go` and `server/<name>_*_test.go`, `web/templates/pages/<name>_setup*`.
- Registration in `cmd/server/main.go` and `config/config.go`, checked line by line: no removed lines, and every added run must be the integration's import line, its `<name>.New(),` registry entry, or a `"<name>": { ... },` block that maps credentials only to `<NAME>_*` environment variables or lists credentials with `Enabled: false`. A statement, a comment opener, another integration's name, or an environment variable belonging to another product holds the PR.
- Listing in `README.md`, `docs/adapter-reference.md`, and the generated `web/static/style.css`, additions only (CI fails if the CSS differs from what `make styles` generates).
- Shared tests whose counts change: `config/config_test.go`, `web/tests/unit/ui.test.ts`, checked line by line. Each removed line must come back with exactly one change: the number in an `assert.Len(t, ..., N)` or `assert.equal(rules.length, N)` count going up by one, or one `"<name>"` element added inside a `[]string{...}` literal. Any extra added line must be one of the shapes that list a new integration: its credential keys in `TestDefaultConfig`, an `assert.Equal(t, "<NAME>_...", m["<name>"][...])` env mapping check, or its logo assertion in `ui.test.ts`.

Anything else, including `go.mod`/`go.sum` (new dependencies), workflows, `server/`, or another integration, holds the PR for a person.

## Modes

The repository variable `AUTOMERGE_MODE` controls the workflow without a code change:

| Mode | Behavior |
|---|---|
| `dry-run` (default when unset) | Comments "would merge" once per head commit; never merges |
| `merge` | Adds `release:skip`, then squash-merges with `--match-head-commit` so a push during evaluation cannot slip through |
| `disabled` | Does nothing |

Switch modes with `gh variable set AUTOMERGE_MODE --body merge` (or `dry-run`, `disabled`). The job summary of every run lists each PR and why it is held.

The workflow runs when CI, Live Integration, or Crush review finish, when overload posts its status, every 30 minutes, and by hand (`gh workflow run automerge.yml -f pr=<n>`). Its scripts always come from `main`, never from the PR being evaluated.

## Auto-fix

A held PR falls into one of three groups, reported as `next` by the decision script and shown in each run's summary:

| `next` | When | What happens |
|---|---|---|
| `wait` | Checks or the overload review are still running, or GitHub has not computed mergeability | Nothing; a later event re-evaluates it |
| `fix` | A conflict with `main` (GitHub runs no checks on a conflicting PR), or only things the bot can fix: a failed check, overload changes requested, open review threads, a missing live spec, registration or shared-test edits that drifted | The workflow starts the implementation job with `mode=shepherd` and `target=<pr>` and lists the findings in a PR comment |
| `person` | Scope the bot must not work around (new dependency, files outside the lane, an existing integration), `automation-hold`, a draft, a fork | Nothing; a person decides |

Auto-fix starts at most one job per run and only while no implementation run is queued or running (a second dispatch would replace the queued one). It asks once per head commit and set of findings, so a fix the job cannot make does not loop; a new push or new findings ask again. Set the repository variable `AUTOFIX` to `off` to stop it; `AUTOMERGE_MODE=disabled` stops both.

The loop this closes: the bot pushes, CI, Live Integration and overload run, the workflow re-evaluates, and the PR either merges or goes back to the bot with the findings. A merge raises integration counts on `main`, which makes the other bot PRs conflict; the next evaluation sends the bot to merge `main` into them one at a time.

## Releases

Bot merges carry `release:skip`, which `auto-release.yml` honors. `weekly-release.yml` tags `main` on Mondays at 15:00 UTC when anything landed since the last tag, and `release.yml` publishes it. Merges by people still release immediately. Both jobs compute the tag with `scripts/calver-next.sh`.

## Tests

`make test` runs `scripts/automerge-decide_test.sh` (every gate and the `next` classification), `scripts/automerge-facts_test.sh` (API pagination, rename sources), `scripts/automerge_test.sh` (dry run never merges, comments once per head, merge mode labels before merging at the evaluated head, auto-fix dispatch rules), and `scripts/release-tags_test.sh`.

To see what the gate thinks of a real PR without changing anything:

```bash
bash scripts/automerge-facts.sh <pr> | bash scripts/automerge-decide.sh
```
