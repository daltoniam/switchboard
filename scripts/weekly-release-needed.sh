#!/usr/bin/env bash
# Prints "yes" when HEAD has commits that no release tag includes yet, so
# the weekly release should tag it, and "no" otherwise.
set -euo pipefail

if [ -n "$(git tag --list 'v[0-9]*' --contains HEAD)" ]; then
  echo no
else
  echo yes
fi
