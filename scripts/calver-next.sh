#!/usr/bin/env bash
# Prints the next semver-compatible calendar version tag, vYYYY.MMDD.N,
# where N counts releases on that UTC day. Month and day drop leading zeros
# (November 4 is 1104, January 9 is 109) so the tag stays valid semver.
#
# Usage: scripts/calver-next.sh [YYYY-MM-DD]   (default: today, UTC)
set -euo pipefail

date=${1:-$(date -u +%F)}
year=${date%%-*}
rest=${date#*-}
month=$((10#${rest%%-*}))
day=$((10#${rest#*-}))
prefix="v${year}.$((month * 100 + day))"

last="$(git tag -l "${prefix}.*" | grep -E "^${prefix//./\\.}\.[0-9]+$" | sed "s/^${prefix//./\\.}\.//" | sort -n | tail -1 || true)"
if [ -z "$last" ]; then
  echo "${prefix}.0"
else
  echo "${prefix}.$((last + 1))"
fi
