#!/usr/bin/env bash
# Runs goreleaser inside ghcr.io/goreleaser/goreleaser-cross, which ships
# osxcross clang and a macOS SDK. The macOS release binaries are built with
# cgo (the 1Password SDK needs it to reach the desktop app), and Go cannot
# link cgo for macOS without that SDK. Linux and Windows still build with
# CGO_ENABLED=0 (see .goreleaser.yml).
#
# Usage: scripts/goreleaser-cross.sh <goreleaser args...>
#   e.g. scripts/goreleaser-cross.sh release --snapshot --clean --skip=publish
#
# Environment:
#   GITHUB_TOKEN               passed through to goreleaser when set
#   GORELEASER_DOCKER_CONFIG   host Docker config dir (registry logins and the
#                              buildx builder) to mount as /root/.docker
#   GORELEASER_GO_CACHE=1      reuse the host module and build caches; only
#                              useful on a linux host
set -euo pipefail

# Go 1.27 line; GOTOOLCHAIN=auto fetches the exact go.mod toolchain.
IMAGE="ghcr.io/goreleaser/goreleaser-cross:v1.27.3-4"

root="$(git rev-parse --show-toplevel)"
# A git worktree keeps its objects in the main checkout; mount that too so
# goreleaser can read tags and commits.
common="$(cd "$root" && git rev-parse --path-format=absolute --git-common-dir)"

args=(
  --rm
  -v /var/run/docker.sock:/var/run/docker.sock
  -v "$root:$root"
  -w "$root"
)
case "$common" in
  "$root"/*) ;;
  *) args+=(-v "$common:$common") ;;
esac
if [[ -n "${GORELEASER_DOCKER_CONFIG:-}" ]]; then
  args+=(-v "$GORELEASER_DOCKER_CONFIG:/root/.docker")
fi
if [[ "${GORELEASER_GO_CACHE:-}" == 1 ]]; then
  args+=(-v "$(go env GOMODCACHE):/root/go/pkg/mod" -v "$(go env GOCACHE):/root/.cache/go-build")
fi
if [[ -n "${GITHUB_TOKEN:-}" ]]; then
  args+=(-e GITHUB_TOKEN)
fi

exec docker run "${args[@]}" "$IMAGE" "$@"
