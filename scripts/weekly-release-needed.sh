#!/usr/bin/env bash
# Prints "yes" when HEAD has commits that no release tag includes yet, so
# the weekly release should tag it, and "no" otherwise.
set -euo pipefail

last="$(git describe --tags --abbrev=0 --match 'v[0-9]*' HEAD 2>/dev/null || true)"
if [ -z "$last" ] || [ -n "$(git rev-list "$last..HEAD")" ]; then
  echo yes
else
  echo no
fi
