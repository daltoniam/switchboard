#!/usr/bin/env bash
# Prints the integrations whose live acceptance specs a change must run, one
# per line. Exits 1 when the change adds an integration without a live spec.
#
# Usage: scripts/live-targets.sh <base-ref> <head-ref>
set -euo pipefail

base=${1:?usage: live-targets.sh <base-ref> <head-ref>}
head=${2:?usage: live-targets.sh <base-ref> <head-ref>}

# --no-renames lists both sides of a move, so the integration losing a file
# is tested too.
changed="$(git diff --name-only --no-renames "$base...$head")"

# Specs present at head, as integration names (web/live_<name>_test.go).
specs="$(git ls-tree -r --name-only "$head" -- web/ |
  sed -n 's#^web/live_\(.*\)_test\.go$#\1#p' | sort -u)"

has_spec() { printf '%s\n' "$specs" | grep -qx "$1"; }

# Changes to shared code can break any integration, so they run every spec.
shared='^(go\.(mod|sum)|Makefile|[^/]+\.go|server/|config/|cmd/|web/[^/]+\.go|web/templates/|scripts/live-targets\.sh|\.github/workflows/live-integration\.yml)'

targets=""
missing=""
while IFS= read -r path; do
  [ -n "$path" ] || continue
  case "$path" in
    web/live_*_test.go)
      name="${path#web/live_}"
      name="${name%_test.go}"
      has_spec "$name" && targets="$targets $name"
      continue
      ;;
    integrations/*/*)
      name="${path#integrations/}"
      name="${name%%/*}"
      if has_spec "$name"; then
        targets="$targets $name"
      elif ! git cat-file -e "$base:integrations/$name" 2>/dev/null; then
        missing="$missing $name"
      fi
      continue
      ;;
  esac
  case "$path" in
    *_test.go) case "$path" in web/ui_test.go | web/live_test.go) ;; *) continue ;; esac ;;
  esac
  if printf '%s\n' "$path" | grep -Eq "$shared"; then
    targets="$targets $specs"
  fi
done <<<"$changed"

status=0
for name in $(printf '%s\n' $missing | sort -u); do
  echo "new integration $name has no live spec; add web/live_${name}_test.go (see docs/integration-ui-testing.md)" >&2
  status=1
done
[ "$status" = 0 ] || exit "$status"

printf '%s\n' $targets | sed '/^$/d' | sort -u
