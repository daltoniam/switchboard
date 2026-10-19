# Web UI
- Templ templates in `web/templates/` (see Commands section for generate workflow)
- Default port: 3847
- Go 1.22+ method-pattern routing (`"GET /integrations/{name}"`, `"POST /api/slack/save-tokens"`)
- Routes:
  - `GET /` — Dashboard with integration health status
  - `GET /projects` — Project Catalog browser (search + list)
  - `GET /projects/{id}` — Project definition detail, sources, context, work links
  - `GET /projects/{id}/work` — Project-scoped work hub
  - `GET /projects/{id}/work/profiles` — Work profiles for the project
  - `GET /projects/{id}/work/profiles/{profileID}` — Work profile detail
  - `GET /projects/{id}/work/agents` — Agent profiles linked to the project
  - `GET /projects/{id}/work/agents/{agentID}` — Agent profile detail
  - `GET /projects/{id}/work/sessions` — Work sessions (`state` query filter)
  - `GET /projects/{id}/work/sessions/{sessionID}` — Work session detail
  - `GET /integrations` — Integration list
  - `GET /integrations/{name}` — Integration detail + credential form
  - `POST /integrations/{name}` — Save integration credentials
  - `POST /integrations/{name}/identities` — Add or update a named identity (secret fields stay write-only)
  - `POST /integrations/{name}/identities/{identity}/delete` — Remove a named identity
- Native WASM OAuth: `POST /api/integrations/{name}/oauth/start` and `GET /api/integrations/{name}/oauth/callback`; see [plugin-oauth.md](plugin-oauth.md) for public-client PKCE setup and native refresh.
- **OAuth/Setup pages** (guided credential flows):
  - `GET /integrations/github/setup` — GitHub Device Flow OAuth
  - `GET /integrations/linear/setup` — Linear OAuth (PKCE)
  - `GET /integrations/figma/setup` — Figma hosted MCP OAuth (PKCE) for FigJam workflows
  - `GET /integrations/notion-mcp/setup`: independent Notion hosted MCP OAuth (PKCE). Uses `/api/remote/notion-mcp/oauth/start` and `/api/remote/notion-mcp/oauth/callback`; successful sign-in enables `notion-mcp` and refreshes discovery without changing the existing Notion integration.
  - `GET /integrations/sentry/setup` — Sentry Device Flow OAuth
  - `GET /integrations/google/setup` — Unified Google Workspace setup (one OAuth client, one sign-in, fans tokens out to all selected Google services). See [google-setup.md](google-setup.md). The 11 per-service pages (`/integrations/gmail/setup`, `/integrations/gcal/setup`, etc.) remain but link back to this unified page.
  - `GET /integrations/slack/setup` — Slack credentials (browser auto-extract, manual browser snippet, user OAuth token, direct entry)
  - `GET /integrations/notion/setup` — Notion token_v2 entry (browser snippet extraction, manual entry)
  - `GET /integrations/postgres/setup` — Postgres default plus additional aliased connections
  - `GET /integrations/clickhouse/setup` — ClickHouse default plus additional aliased cluster connections
  - `GET /integrations/microsoft365/setup` — Microsoft 365 OAuth (Azure AD / Entra ID PKCE) plus manual access token entry
- All setup pages save credentials to both the integration config and any external token files
- Integrations implementing `MultiIdentityIntegration` + `IdentityConfigHints` render a generic named-identity editor. Identity credentials and metadata persist under `integrations.<name>.identities` in the standard config file.

## Appearance and navigation

- **Appearance:** [themes.json](../web/templates/layouts/themes.json) defines six palettes and 80%/100%/120% font sizes. The header has a theme and font-size menus; choices persist in `localStorage`. First visits follow the system theme.
- **Shared styles:** use `interactive` and `status-*` shortcuts in [style.js](../web/style.js). Controls stay neutral at rest and use the accent for hover, focus, press, and selection. Use `aria-pressed` for toggles and `aria-current="page"` for navigation.
- **Status:** matching text/border hues and tinted backgrounds. Error styling covers the whole card; zero errors stay neutral. Dismiss controls use borderless Lucide × buttons with accessible labels.
- **Spacing:** cards `p-4`, groups and ranked lists `gap-3`, related text `gap-1`. Parent `card-grid`/`card-stack` controls gutters. Legacy setup pages retain inline overrides.
- **Accessibility:** keep native controls and visible focus. Selects use themed chevrons; transitions respect reduced motion. Without JavaScript, the system theme applies and appearance controls are disabled.

Palettes adapt [GitHub](https://github.com/primer/github-vscode-theme), [Atom One Light](https://github.com/akamud/vscode-theme-onelight), [Catppuccin](https://catppuccin.com/palette/), [Dracula](https://draculatheme.com/contribute), and [Nord](https://www.nordtheme.com/docs/colors-and-palettes/).

## Setup Page Design Rules

When adding or changing a setup page, reuse what the existing pages share. A one-off pattern makes one page look unlike every other. Shared UnoCSS shortcuts live in `web/style.js`; remaining CSS lives in `web/templates/layouts/base.templ`. Reuse these instead of adding inline styles.

- **One card per credential method.** Each method is a `card` with a `card-title`. When a page offers several methods, title them `Option N: <Method>` (title case noun phrase), numbered in page order. Status and settings cards (enable toggle, current token, default selection) sit above the options and are not numbered. Examples: `slack_setup.templ`, `notion_setup.templ`, `github_setup.templ`.
- **One description line under the title.** Use `<p class="slack-desc">` to say what the method does and when to pick it.
- **Multi-step instructions use the step list, not `<ol>`.** `slack-steps` → `slack-step` → `slack-step-num` + a `<strong>` step title + `<p class="slack-step-detail">`. The step's form or code block goes inside its step. See Option 2 in `slack_setup.templ`.
- **Copyable text uses `slack-code-block`.** A `<pre><code>` with a `btn btn-sm btn-outline` button labelled `Copy` that shows `Copied!` for two seconds (`copySnippet` in `slack_setup.templ`).
- **External links use `class="slack-link"` with `target="_blank"`.**
- **Form fields use `@components.FormGroup`**; secrets use input type `password`. Field ids must be unique on the page, because `FormGroup` uses the field name as the id.
- **Results and errors use the `flash flash-success` / `flash flash-error` banner** driven by the `result=` / `error=` query parameters, not inline text.
- **Copy is direct and imperative.** Step titles start with a verb ("Open Slack in your browser", "Paste the result below").

## Browser Acceptance

Run `make ui-install` once, then `make test-ui` to verify integration setup in isolated Chromium sessions. `make ci` includes the browser gate. Add integration-specific browser coverage for every new adapter and follow [integration-ui-testing.md](integration-ui-testing.md) for sandboxing, masked evidence, OAuth/identity scenarios, and Orca automation completion requirements.

## Build Tooling

- **Styles:** `make styles` compiles UnoCSS/Lucide from `web/style.js` and templates into `web/static/style.css`. Commit generated CSS; Go embeds it.
- **Development:** install versions from `make node-version npm-version`. Tools run through pinned `npx` commands without a package manifest. Run `make styles-watch` alongside Air; refresh the browser manually.
- **Style checks:** `make ci` rebuilds CSS; GitHub CI checks it is current.
- **TypeScript:** `make tsc` runs the pinned compiler; pass options with `TSC_FLAGS`. Browser code remains JavaScript.
- **Unit tests:** `make test-web` runs `web/tests/unit/{module}.test.{mjs,ts,mts}` with Node. TypeScript tests use type stripping, not type checking.
- **Browser tests:** `make test-browser` runs isolated Docker/Chromium tests for navigation, appearance, notifications, dashboard refresh, and native form interactions. Only Docker is required.
- **Templ**: `web/templates/*.templ` → run `templ generate` after edits. **Never edit `*_templ.go`** (generated)
- **Release**: GoReleaser via `.goreleaser.yml`. Ldflags: `main.version`, `main.commit`, `main.date`
- **Testing**: `stretchr/testify` assertions. Tests in every package
- **Linting**: `.golangci.yml` — errcheck, govet, ineffassign, nestif, staticcheck, unused
- **CI**: `.github/workflows/ci.yml` — build, test (race), gofmt, golangci-lint, gosec, govulncheck
- **Go 1.26** — deps: `go-sdk`, `go-github/v68`, `slack-go/slack`, `a-h/templ`, `lib/pq`, `clickhouse-go/v2`, `testify`

## CLI & Daemon

```bash
# Run (default — HTTP server with MCP + web UI on same port)
./switchboard
./switchboard --port 3847

# Run (stdio mode — legacy, for AI clients that need stdin/stdout)
./switchboard --stdio

# Daemon management
./switchboard daemon install              # Install as launchd (macOS) or systemd (Linux) service
./switchboard daemon install --verbose    # Persist debug logging in the installed service
./switchboard daemon uninstall            # Remove the system service
./switchboard daemon start                # Start the daemon (uses service if installed, else detached process)
./switchboard daemon start --port 9999    # Start on a custom port
./switchboard daemon stop                 # Stop the daemon
./switchboard daemon status               # Show daemon status + health
./switchboard daemon logs                 # Print how to follow logs (journalctl on systemd)

# Release (local snapshot for testing)
goreleaser release --snapshot --clean

# Release (production — triggered by pushing a git tag)
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
# CI (or manually): goreleaser release --clean

# Generate templ templates (required after editing .templ files in web/templates/)
make generate
```

### Local Development Daemon (systemd only)

On Linux systems with systemd, `make install` and `make deploy` manage a user-space daemon for local development. The binary is **copied** (not symlinked) to `~/.local/bin/switchboard`, so the daemon keeps running even if the source worktree is deleted. **Note**: macOS users with launchd should use `./switchboard daemon install` directly — these Makefile targets are Linux-specific.

```bash
# First time — build, install binary, create systemd user service, and start
make install

# After code changes — build, overwrite binary, restart service
make deploy

# Logs and status
journalctl --user -u switchboard -f
systemctl --user status switchboard
```

`make install` / `make deploy` enable `--verbose` and send stdout/stderr to journald. Search and execute requests then appear as `DEBUG` lines (`msg=search`, `msg=execute`). Fallback/launchd installs still write `~/.config/switchboard/switchboard.log`.

The systemd unit file is written to `~/.config/systemd/user/switchboard.service` and points at `~/.local/bin/switchboard`. The service restarts on failure automatically.
