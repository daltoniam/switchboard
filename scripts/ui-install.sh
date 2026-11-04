#!/usr/bin/env bash
# Install the Playwright driver pinned by playwright-go plus Chromium.
# playwright-go downloads the matching playwright-core release from npm and
# a Node.js runtime itself, so no system Node.js is required.
set -euo pipefail

module=github.com/mxschmitt/playwright-go
go mod download "$module"
go run "$module/cmd/playwright" install "$@" chromium
