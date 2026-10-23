#!/usr/bin/env bash
# usage: start-sb.sh <homeName> <port> <configJSON>; isolated HOME + SWITCHBOARD_DEV_HOME per run
set -e; D=$(cd "$(dirname "$0")"; pwd); H=$D/homes/$1; P=$2
[ -f "$H/pid" ] && kill "$(cat "$H/pid")" 2>/dev/null || true; sleep 0.5
rm -rf "$H"; mkdir -p "$H/.config/switchboard"
printf '%s' "$3" > "$H/.config/switchboard/config.json"; chmod 600 "$H/.config/switchboard/config.json"
env -i PATH=$PATH HOME=$H SWITCHBOARD_DEV_HOME=$H ${SB_BIN:-/tmp/sb349} -port "$P" >"$D/logs/switchboard-$1.log" 2>&1 & echo $! >"$H/pid"
for i in $(seq 100); do curl -s -o /dev/null "http://127.0.0.1:$P/" && exit 0; sleep 0.2; done; echo "server failed"; tail "$D/logs/switchboard-$1.log"; exit 1
