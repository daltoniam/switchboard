#!/usr/bin/env bash
# Verifies calendar-version tag selection and the weekly release decision.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CALVER="${ROOT}/scripts/calver-next.sh"
WEEKLY="${ROOT}/scripts/weekly-release-needed.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*" >&2; }

repo="$(mktemp -d)"
trap 'rm -rf "$repo"' EXIT
git -C "$repo" init -q -b main
git -C "$repo" config user.email test@example.test
git -C "$repo" config user.name test
git -C "$repo" config commit.gpgsign false
git -C "$repo" config tag.gpgsign false
commit() { git -C "$repo" commit -q --allow-empty -m "$1"; }
calver() { (cd "$repo" && bash "$CALVER" "$1"); }
weekly() { (cd "$repo" && bash "$WEEKLY"); }

commit first
[ "$(calver 2026-11-04)" = v2026.1104.0 ] || fail "first tag of the day: $(calver 2026-11-04)"
ok "first tag of a day ends in .0"

[ "$(calver 2026-01-09)" = v2026.109.0 ] || fail "leading zeros: $(calver 2026-01-09)"
ok "month and day drop leading zeros for semver"

git -C "$repo" tag v2026.1104.0
git -C "$repo" tag v2026.1104.9
commit second
git -C "$repo" tag v2026.1104.10
[ "$(calver 2026-11-04)" = v2026.1104.11 ] || fail "numeric ordering: $(calver 2026-11-04)"
ok "the next tag follows the highest of the day numerically"

git -C "$repo" tag v2026.11040.0
[ "$(calver 2026-11-04)" = v2026.1104.11 ] || fail "prefix collision: $(calver 2026-11-04)"
ok "tags from a different day with the same prefix are ignored"

[ "$(weekly)" = no ] || fail "tagged head should not need a release"
ok "no weekly release when the head is already released"

commit "bot merge"
[ "$(weekly)" = yes ] || fail "untagged commits should need a release"
ok "weekly release when commits landed since the last tag"

empty="$(mktemp -d)"
git -C "$empty" init -q -b main
git -C "$empty" -c user.email=t@e -c user.name=t -c commit.gpgsign=false commit -q --allow-empty -m only
[ "$(cd "$empty" && bash "$WEEKLY")" = yes ] || fail "repository without tags should need a release"
rm -rf "$empty"
ok "weekly release when nothing was ever tagged"

echo "PASS: release tags" >&2
