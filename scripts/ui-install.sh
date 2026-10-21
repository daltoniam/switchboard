#!/usr/bin/env bash
# Install the Playwright driver pinned by playwright-go plus Chromium.
# The Playwright CDN no longer serves Linux driver archives, so when the
# download fails the driver is assembled from the matching npm
# playwright-core release and the system Node.js, which is what the
# archive contains.
set -euo pipefail

module=github.com/playwright-community/playwright-go
go mod download "$module"
module_dir="$(go list -m -f '{{.Dir}}' "$module")"
version="$(sed -n 's/^const playwrightCliVersion = "\(.*\)"$/\1/p' "$module_dir/run.go")"
if [ -z "$version" ]; then
	echo "ui-install: could not read the Playwright driver version from $module_dir/run.go" >&2
	exit 1
fi

if [ -n "${PLAYWRIGHT_DRIVER_PATH:-}" ]; then
	driver_dir="$PLAYWRIGHT_DRIVER_PATH"
elif [ "$(uname -s)" = "Darwin" ]; then
	driver_dir="$HOME/Library/Caches/ms-playwright-go/$version"
else
	driver_dir="$HOME/.cache/ms-playwright-go/$version"
fi

install_browsers() {
	go run "$module/cmd/playwright" install "$@" chromium
}

assemble_driver_from_npm() {
	local node_bin work
	node_bin="$(command -v node || true)"
	if [ -z "$node_bin" ] || ! command -v npm >/dev/null 2>&1; then
		echo "ui-install: Node.js and npm are required to assemble the Playwright $version driver" >&2
		exit 1
	fi
	work="$(mktemp -d)"
	if ! (cd "$work" && npm pack --silent "playwright-core@$version" >/dev/null && tar xzf "playwright-core-$version.tgz"); then
		rm -rf "$work"
		echo "ui-install: could not download playwright-core@$version from npm" >&2
		exit 1
	fi
	rm -rf "$driver_dir"
	mkdir -p "$driver_dir"
	mv "$work/package" "$driver_dir/package"
	rm -rf "$work"
	ln -s "$node_bin" "$driver_dir/node"
	echo "ui-install: assembled Playwright $version driver in $driver_dir from npm"
}

if [ -f "$driver_dir/package/cli.js" ]; then
	install_browsers "$@"
	exit 0
fi

status=0
install_browsers "$@" || status=$?
if [ "$status" -eq 0 ]; then
	exit 0
fi
if [ -f "$driver_dir/package/cli.js" ]; then
	echo "ui-install: Playwright install failed after the driver was installed" >&2
	exit "$status"
fi

echo "ui-install: driver download failed; falling back to npm playwright-core@$version" >&2
assemble_driver_from_npm
install_browsers "$@"
