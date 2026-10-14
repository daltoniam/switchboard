#!/usr/bin/env bash
# usage: start-sb.sh <homeName> <port> <credsJSON>   e.g. start-sb.sh live 13854 '{"api_key":"..."}'
set -e; D=$(cd "$(dirname "$0")"; pwd); H=$D/homes/$1; P=$2
[ -f "$H/pid" ] && kill "$(cat "$H/pid")" 2>/dev/null || true; sleep 0.5
mkdir -p "$H/.config/switchboard"
python3 - "$H/.config/switchboard/config.json" "$3" <<'PY'
import json,sys; json.dump({"integrations":{"tailscale":{"enabled":True,"credentials":json.loads(sys.argv[2])}}},open(sys.argv[1],"w"),indent=1)
PY
chmod 600 "$H/.config/switchboard/config.json"
env -i PATH=$PATH HOME=$H SWITCHBOARD_DEV_HOME=$H ${SB_BIN:-/tmp/sb-ts328} -port "$P" >"$D/switchboard-$1.log" 2>&1 & echo $! >"$H/pid"
for i in $(seq 50); do curl -s -o /dev/null "http://127.0.0.1:$P/" && exit 0; sleep 0.2; done; echo "server failed"; tail "$D/switchboard-$1.log"; exit 1
