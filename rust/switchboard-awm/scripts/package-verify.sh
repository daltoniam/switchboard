#!/usr/bin/env bash
# Prove the published crate artifact is self-contained: proto generation,
# LICENSE text, and the README example all ship and build from `cargo package`.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

required=(
  examples/list_projects.rs
  LICENSE
  proto/awm.proto
  build.rs
  src/lib.rs
  README.md
)

list="$(cargo package --list --allow-dirty)"
echo "$list"
for path in "${required[@]}"; do
  if ! grep -Fxq "$path" <<<"$list"; then
    echo "error: packaged crate is missing $path" >&2
    exit 1
  fi
done
if grep -Fq -- '--no-verify' <<<"$*"; then
  echo "error: package verification must not use --no-verify" >&2
  exit 1
fi

# Default cargo package verifies by building the extracted artifact.
# Do not pass --no-verify here; that is the actual publish gate.
cargo package --allow-dirty

pkg="$root/target/package/switchboard-awm-0.1.0"
test -d "$pkg"
test -f "$pkg/examples/list_projects.rs"
test -f "$pkg/proto/awm.proto"
test -f "$pkg/LICENSE"
grep -q 'Elastic License 2.0' "$pkg/LICENSE"
grep -q '## Acceptance' "$pkg/LICENSE"
if grep -Eq 'repository root LICENSE|See the repository root' "$pkg/LICENSE"; then
  echo "error: packaged LICENSE still points at the repository root" >&2
  exit 1
fi

# Rebuild from the extracted package so build.rs/proto generation and the
# example are proven without the monorepo checkout layout.
cargo test --manifest-path "$pkg/Cargo.toml" --offline
cargo build --manifest-path "$pkg/Cargo.toml" --examples --offline
