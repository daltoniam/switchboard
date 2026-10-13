# Integration UI acceptance gate

Every new integration must be configurable from a fresh local Web UI, not only through a hand-written config file. Orca integration automations must load the repository's `add-integration` and `pr-shepherd` skills and complete this gate before declaring a PR ready. Never merge from an automation.

## Commands

```bash
make ui-install
make test-ui
make ci
```

On Linux, install Chromium's system dependencies with `make ui-install UI_INSTALL_ARGS=--with-deps`. The Playwright Go module pins the driver/browser version; `scripts/ui-install.sh` uses the current ESRP download mirror because the older Azure Edge mirrors no longer serve that version. The mirror does not serve Linux driver archives, so on Linux the script assembles the driver from the matching npm `playwright-core` release and the system Node.js (Node.js and npm are required there).

`make ci` includes `test-ui`. Ordinary `go test ./...` skips the opt-in UI tests so unit tests do not require a browser. The explicit UI target builds the development binary, sets `SWITCHBOARD_UI_BINARY`, and fails on missing drivers, Chromium, server startup, assertions, or browser errors. GitHub Actions runs a separate `ui` job with Chromium installed. Repository administrators should require the `ui` check in branch protection; adding a workflow does not change remote protection rules.

## Isolation and evidence

The binary lifecycle test starts an actual development binary on an ephemeral loopback port with a temporary `HOME` and `XDG_CONFIG_HOME`. Its environment is allowlisted: host credential variables cannot override fixture credentials. The provider is a local HTTP fixture. Browser contexts are fresh, servers are stopped before temporary directories are removed, and no host configuration or browser profile is copied.

Screenshots are written to `dist/ui-artifacts/`, with all input fields masked. CI uploads them under the tested commit SHA with seven-day retention. Never upload config files, raw DOM snapshots, traces, request bodies, or logs containing credentials. Ordinary password inputs currently contain their saved value in the DOM; password masking alone is not artifact redaction.

The baseline tests cover:

- Fresh integration listing, detail navigation, text/password credential inputs.
- Save, enable, valid/invalid/missing credentials, and configuration persistence across process restarts.
- Read-only MCP discovery followed by execution against the same configured server.
- Health refresh, disable, list state, and exclusion from default MCP discovery.
- Desktop and narrow-viewport screenshot evidence.
- Named identity add/edit/delete and blank-secret preservation without rendering identity secrets.
- Mocked OAuth sign-in, reauthorization, cancellation, and token-exchange errors without visiting a real identity provider.

## Requirements for each new adapter

The baseline examples are not coverage for an arbitrary new integration. Add a `TestUI<Integration>` test in `web/` using the helpers in `web/ui_test.go`. It must run under `make test-ui` and use local mock providers. Assert the actual default credential keys, placeholders, optional labels, setup routing, tool count, and authentication behavior for the new integration. Missing defaults must fail on a fresh install. Do not pre-seed credentials just to make fields appear.

For basic-token integrations, model the real-binary lifecycle test. For custom setup, OAuth, and named identities, add the integration-specific browser flows and retain existing handler tests. Include persistence and MCP discovery/execute verification for the new adapter, not just the shared examples. Intentional auth failures must assert the visible error state, not merely a successful save redirect.

## Orca browser walkthrough

In addition to repeatable tests, the automation must drive the new integration's actual local UI using Go Playwright, agent-browser, or Chrome DevTools. Playwright is already available and is the default; no additional CLI dependency is required. If using agent-browser, pin its version and load its installed workflow instructions. Never attach to a personal browser session.

Use a fresh sandbox with both `HOME` and `XDG_CONFIG_HOME`, an allowlisted server environment, a unique loopback port, and fixture credentials. Build before changing the runtime environment. Never use the host daemon, default host config, or copied personal credentials. Confirm the server URL belongs to the current worktree before filling any form. Register shutdown and browser cleanup before starting the walkthrough.

Exercise list -> setup/detail -> enter credentials -> save -> verify health -> reload -> restart -> verify persistence -> discover and execute a safe read -> disable. Inspect desktop and narrow-viewport screenshots for clipped inputs, overlapping controls, unreadable errors, and broken navigation. Check JavaScript errors and unexpected failed network requests. Turn discoveries into regression tests; do not waive them because unit tests pass.

With explicit dedicated test credentials, additionally perform live sign-in through the UI and a read-only MCP smoke test. Never automate MFA bypass or use a personal identity-provider profile. If credentials are unavailable, record **Live authentication: unverified**. Mock-backed UI verification is still mandatory and must pass; do not claim that mock success proves a production provider works.

## Completion evidence

The automation's PR test plan must include:

- Exact head SHA and successful `make ci` / `make test-ui` results.
- New integration browser test name and tested setup/auth scenarios.
- Local development URL, browser driver, desktop/mobile evidence paths, and console/network findings.
- MCP search/read result and whether it used fixtures or dedicated live credentials.
- Explicit live-auth status and any external blocker.

Re-run and replace evidence after changing the PR head. Missing browser infrastructure, missing integration-specific tests, or a skipped UI lane is a blocker, not a passing result. Remote CI and review remain required under the PR shepherd's exact-head contract.

## Automation prompt requirement

Include this in the Orca new-integration automation prompt:

> Load the repository add-integration and pr-shepherd skills. Follow docs/integration-ui-testing.md. Add integration-specific browser acceptance coverage, run make ui-install when necessary, and pass make test-ui and make ci. Drive the new integration's isolated local Web UI and inspect desktop/mobile evidence. Verify save, health, restart persistence, and read-only MCP discovery/execute. Report fixture versus live authentication separately. Bind evidence to the exact PR head. Do not declare completion with a skipped browser gate, and never merge.

The automation definition is managed by Orca, not by files in this repository. Apply the prompt through the running Orca CLI/UI; do not edit its application-state JSON directly.
