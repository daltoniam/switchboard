#!/usr/bin/env bash
# Helpers for stopping Crush and descendant processes on overload-implement timeout.
# Crush is started under setsid so its PGID covers tool subprocesses in the same group.

# Return the process group ID for pid (empty if unknown).
overload_crush_pgid() {
  ps -o pgid= -p "$1" 2>/dev/null | tr -d ' ' || true
}

# Signal pid's entire process group (Linux: negative PGID). Falls back to pid alone.
overload_kill_crush_tree() {
  local pid=$1 signal=$2
  local pgid
  pgid=$(overload_crush_pgid "$pid")
  if [ -n "$pgid" ]; then
    kill "-$signal" -- "-$pgid" 2>/dev/null || true
  else
    kill "-$signal" "$pid" 2>/dev/null || true
  fi
}
