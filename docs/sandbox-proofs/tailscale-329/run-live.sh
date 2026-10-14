#!/usr/bin/env bash
# One command live run. usage: ./run-live.sh [329|328main] [live|oauth]   (default: 329 live)
# Needs /home/box/.sb-sandbox-secrets/tailscale-328.env (TS_API_KEY, TS_TAILNET, optional TS_OAUTH_CLIENT_ID/SECRET, TS_PREVIEW_USER)
set -e; cd "$(dirname "$0")"; V=${1:-329}; M=${2:-live}
[ -f /home/box/.sb-sandbox-secrets/tailscale-328.env ] || { echo "WAITING: /home/box/.sb-sandbox-secrets/tailscale-328.env missing"; exit 2; }
case $V in 329) B=/tmp/sb-ts329; W=/workspace/sb-tailscale-329;; 328main) B=/tmp/sb-ts328main; W=/workspace/sb-tailscale-main;; esac
[ -x "$B" ] || (cd "$W" && go build -o "$B" ./cmd/server)
SB_BIN=$B python3 run-e2e.py --mode "$M" | tee "harness-$M-$V.log"
