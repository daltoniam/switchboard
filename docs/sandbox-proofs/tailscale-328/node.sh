#!/usr/bin/env bash
# Userspace tailscaled test node (no docker on box). usage: node.sh up <authkey> | down
set -e; T=/home/box/tools/tailscale; D=$(cd "$(dirname "$0")"; pwd)/node; S=$D/tailscaled.sock; mkdir -p "$D"
case $1 in
up) "$T/tailscaled" --tun=userspace-networking --statedir="$D/state" --socket="$S" --port=0 >"$D/tailscaled.log" 2>&1 & echo $! >"$D/pid"; sleep 2
    "$T/tailscale" --socket="$S" up --authkey="$2" --hostname=sb-proof-328-node --advertise-routes=10.99.0.0/24 --timeout=60s >"$D/up.log" 2>&1 || true
    "$T/tailscale" --socket="$S" status --json > "$D/status.json" 2>/dev/null || true; cat "$D/up.log";;
down) "$T/tailscale" --socket="$S" logout 2>/dev/null || true; kill "$(cat "$D/pid")" 2>/dev/null || true;;
esac
