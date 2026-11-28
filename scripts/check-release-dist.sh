#!/usr/bin/env bash
# Checks a goreleaser dist/ directory after `scripts/goreleaser-cross.sh
# release --snapshot --clean --skip=publish` (the release dry run):
#   - every artifact the release publishes was produced, under the same names
#   - macOS binaries were built with cgo (1Password desktop sign-in)
#   - Linux and Windows binaries were built without cgo, Linux ones static
#   - both Docker images were built
# Usage: scripts/check-release-dist.sh [dist-dir]
set -euo pipefail

dist="${1:-dist}"
dist="${dist%/}"
fail=0
bad() { echo "FAIL: $*" >&2; fail=1; }
ok() { echo "ok: $*"; }

version="$(sed -n 's/.*"version":"\([^"]*\)".*/\1/p' "$dist/metadata.json")"
[[ -n "$version" ]] || { echo "FAIL: no version in $dist/metadata.json" >&2; exit 1; }
ok "version $version"

expect_file() {
  if [[ -f "$dist/$1" ]]; then ok "$1"; else bad "missing $1"; fi
}

for target in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64; do
  expect_file "switchboard_${version}_${target}.tar.gz"
done
for target in windows_amd64 windows_arm64; do
  expect_file "switchboard_${version}_${target}.zip"
done
for arch in amd64 arm64; do
  for ext in deb rpm apk pkg.tar.zst; do
    expect_file "switchboard_${version}_linux_${arch}.${ext}"
  done
done
expect_file checksums.txt
expect_file homebrew/Formula/switchboard.rb
expect_file homebrew/Casks/switchboard.rb
expect_file scoop/switchboard.json

cgo_setting() { go version -m "$1" | awk '$1 == "build" && $2 ~ /^CGO_ENABLED=/ { sub("CGO_ENABLED=", "", $2); print $2 }'; }

check_bin() {
  local goos="$1" goarch="$2" want_cgo="$3" bin
  bin="$(find "$dist" -path "$dist/*_${goos}_${goarch}*/switchboard*" -type f | head -1)"
  if [[ -z "$bin" ]]; then
    bad "no ${goos}/${goarch} binary"
    return
  fi
  local got
  got="$(cgo_setting "$bin")"
  if [[ "$got" == "$want_cgo" ]]; then
    ok "${goos}/${goarch} CGO_ENABLED=$got"
  else
    bad "${goos}/${goarch} CGO_ENABLED=${got:-unset}, want $want_cgo ($bin)"
  fi
  local desc
  desc="$(file -b "$bin")"
  case "$goos" in
    linux)
      if [[ "$desc" == *"statically linked"* ]]; then ok "${goos}/${goarch} static"; else bad "${goos}/${goarch} not static: $desc"; fi
      ;;
    darwin)
      if [[ "$desc" == Mach-O* ]]; then ok "${goos}/${goarch} $desc"; else bad "${goos}/${goarch} not Mach-O: $desc"; fi
      ;;
  esac
}

check_bin darwin amd64 1
check_bin darwin arm64 1
check_bin linux amd64 0
check_bin linux arm64 0
check_bin windows amd64 0
check_bin windows arm64 0

for arch in amd64 arm64; do
  image="ghcr.io/daltoniam/switchboard:${version}-${arch}"
  if got="$(docker image inspect --format '{{.Os}}/{{.Architecture}}' "$image" 2>/dev/null)" && [[ "$got" == "linux/$arch" ]]; then
    ok "image $image ($got)"
  else
    bad "image $image missing or wrong platform (${got:-none})"
  fi
done

exit "$fail"
