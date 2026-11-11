#!/usr/bin/env bash
# Helpers for stopping Crush and descendant processes on overload-implement timeout.
# Crush is started under setsid so its PGID covers tool subprocesses in the same group.

# Return the process group ID for pid (empty if unknown).
overload_crush_pgid() {
  ps -o pgid= -p "$1" 2>/dev/null | tr -d ' ' || true
}

# Signal a process group. pgid is captured at Crush launch; pid is optional fallback.
overload_kill_crush_tree() {
  local pid=${1:-} signal=$2 pgid=${3:-}
  if [ -z "$pgid" ] && [ -n "$pid" ]; then
    pgid=$(overload_crush_pgid "$pid")
  fi
  if [ -n "$pgid" ]; then
    kill "-$signal" -- "-$pgid" 2>/dev/null || true
  elif [ -n "$pid" ]; then
    kill "-$signal" "$pid" 2>/dev/null || true
  fi
}

# Print one line of runner pressure: load, available memory, swap in use and
# free disk on the given path. OVERLOAD_PROC_ROOT lets tests supply /proc files.
overload_resource_sample() {
  local path=${1:-/} proc=${OVERLOAD_PROC_ROOT:-/proc} load mem swap disk
  load=$(cut -d' ' -f1 "$proc/loadavg" 2>/dev/null || echo "?")
  mem=$(awk '/^MemAvailable:/ {printf "%d", $2 / 1024}' "$proc/meminfo" 2>/dev/null)
  swap=$(awk '/^SwapTotal:/ {t=$2} /^SwapFree:/ {f=$2} END {printf "%d", (t - f) / 1024}' "$proc/meminfo" 2>/dev/null)
  disk=$(df -Pk "$path" 2>/dev/null | awk 'NR==2 {printf "%d", $4 / 1048576}')
  printf 'resources load=%s mem_avail_mb=%s swap_used_mb=%s disk_free_gb=%s\n' \
    "$load" "${mem:-?}" "${swap:-?}" "${disk:-?}"
}
