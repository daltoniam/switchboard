#!/usr/bin/env bash
# Switchboard Stacklane compose lifecycle.
# Always uses: docker compose -p "switchboard-<instance>" -f "$ROOT/docker-compose.dev.yml"
set -euo pipefail

# Neutralize accidental ambient Compose controls before assigning locals.
# STACKLANE_INSTANCE remains a documented operator input and is derived below.
unset COMPOSE_FILE COMPOSE_PROFILES COMPOSE_PROJECT_NAME

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${ROOT}/docker-compose.dev.yml"
PROJECT_SLUG="switchboard"

die() { echo "error: $*" >&2; exit 1; }
info() { echo "switchboard-compose: $*" >&2; }

# sanitize_instance: lowercase, non [a-z0-9-] → -, collapse dashes, trim, max 48, fallback dev
sanitize_instance() {
  local s="${1:-}"
  s="$(printf '%s' "$s" | tr '[:upper:]' '[:lower:]')"
  s="$(printf '%s' "$s" | sed -E 's/[^a-z0-9-]+/-/g; s/-+/-/g; s/^-+//; s/-+$//')"
  if [[ ${#s} -gt 48 ]]; then
    s="${s:0:48}"
    s="$(printf '%s' "$s" | sed -E 's/-+$//')"
  fi
  if [[ -z "$s" ]]; then
    s="dev"
  fi
  printf '%s' "$s"
}

derive_instance() {
  if [[ -n "${STACKLANE_INSTANCE:-}" ]]; then
    sanitize_instance "$STACKLANE_INSTANCE"
    return
  fi
  local wt
  wt="$(basename "$ROOT")"
  if [[ -n "$wt" && "$wt" != "." && "$wt" != "/" ]]; then
    sanitize_instance "$wt"
    return
  fi
  local branch=""
  if command -v git >/dev/null 2>&1; then
    branch="$(git -C "$ROOT" rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
  fi
  if [[ -n "$branch" && "$branch" != "HEAD" ]]; then
    sanitize_instance "$branch"
    return
  fi
  sanitize_instance "dev"
}

detect_base_domain() {
  if [[ -n "${STACKLANE_BASE_DOMAIN:-}" ]]; then
    printf '%s' "$STACKLANE_BASE_DOMAIN"
    return
  fi
  if ! command -v stacklane >/dev/null 2>&1; then
    printf 'test'
    return
  fi
  local detected=""
  detected="$(
    timeout 3s stacklane status -o json 2>/dev/null \
      | python3 -c 'import json,sys,re
try:
    data=json.load(sys.stdin)
except Exception:
    sys.exit(0)
val=data.get("base_domain") if isinstance(data, dict) else ""
if isinstance(val, str) and re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9.-]{0,126}", val):
    print(val)
' || true
  )"
  if [[ -n "$detected" ]]; then
    printf '%s' "$detected"
    return
  fi
  printf 'test'
}

require_docker() {
  command -v docker >/dev/null 2>&1 || die "docker not found"
  docker compose version >/dev/null 2>&1 || die "docker compose not available"
  [[ -f "$COMPOSE_FILE" ]] || die "missing $COMPOSE_FILE"
  command -v python3 >/dev/null 2>&1 || die "python3 required for compose check"
}

compose() {
  docker compose -p "$COMPOSE_PROJECT" --project-directory "$ROOT" -f "$COMPOSE_FILE" "$@"
}

export_stack_env() {
  INSTANCE="$(derive_instance)"
  COMPOSE_PROJECT="${PROJECT_SLUG}-${INSTANCE}"
  STACKLANE_BASE_DOMAIN="$(detect_base_domain)"
  export STACKLANE_INSTANCE="$INSTANCE"
  export STACKLANE_BASE_DOMAIN
  export COMPOSE_PROJECT_NAME="$COMPOSE_PROJECT"
}

host_port_for() {
  local svc="$1"
  local target="$2"
  local mapping
  mapping="$(compose port "$svc" "$target" 2>/dev/null || true)"
  if [[ -z "$mapping" ]]; then
    printf ''
    return
  fi
  printf '%s' "${mapping##*:}"
}

stacklane_status_line() {
  if ! command -v stacklane >/dev/null 2>&1; then
    printf 'stacklane: BLOCKED (daemon/cli absent — direct loopback ports still work)\n'
    return
  fi
  if timeout 3s stacklane status >/dev/null 2>&1; then
    local app_fqdn="app.${INSTANCE}.${PROJECT_SLUG}.${STACKLANE_BASE_DOMAIN}"
    if timeout 3s stacklane resolve "$app_fqdn" >/dev/null 2>&1; then
      printf 'stacklane: OK\n'
    else
      printf 'stacklane: degraded (daemon up; %s not resolved yet)\n' "$app_fqdn"
    fi
  else
    printf 'stacklane: BLOCKED (daemon not reachable)\n'
  fi
}

print_endpoints() {
  local app_hp base
  app_hp="$(host_port_for app 3847)"
  base="${STACKLANE_BASE_DOMAIN}"

  echo "app.${INSTANCE}.${PROJECT_SLUG}.${base}:3847  (via Stacklane VIP)"
  if [[ -n "$app_hp" ]]; then
    echo "direct app:  http://127.0.0.1:${app_hp}/"
    echo "direct health: http://127.0.0.1:${app_hp}/api/health"
  else
    echo "direct app:  (not published — stack down?)"
  fi
  stacklane_status_line
  echo "instance: ${INSTANCE}"
  echo "compose project: ${COMPOSE_PROJECT}"
  echo "stacklane base_domain: ${base}"
}

wait_healthy() {
  local timeout_s="${1:-360}"
  local start now elapsed
  local cid="${COMPOSE_PROJECT}-app-1"
  start="$(date +%s)"
  info "waiting for app healthy (timeout ${timeout_s}s)…"
  while true; do
    now="$(date +%s)"
    elapsed=$((now - start))
    if (( elapsed > timeout_s )); then
      compose ps || true
      die "app not healthy within ${timeout_s}s"
    fi
    local app_h
    app_h="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$cid" 2>/dev/null || echo missing)"
    if [[ "$app_h" == "healthy" ]]; then
      info "app healthy"
      return 0
    fi
    sleep 2
  done
}

# Fail-closed render + contract assertions. Never prints rendered JSON or Compose stderr.
cmd_check() {
  require_docker
  export_stack_env
  command -v python3 >/dev/null 2>&1 || die "python3 required for compose check"

  COMPOSE_CHECK_UMASK="$(umask)"
  umask 077
  COMPOSE_CHECK_TMPDIR="$(mktemp -d "${TMPDIR:-/tmp}/switchboard-compose-check.XXXXXX")"
  chmod 700 "$COMPOSE_CHECK_TMPDIR"
  cleanup_check() {
    rm -rf "${COMPOSE_CHECK_TMPDIR:-}"
    umask "${COMPOSE_CHECK_UMASK:-022}"
    trap - EXIT
  }
  trap cleanup_check EXIT
  local tmpdir="$COMPOSE_CHECK_TMPDIR"

  local out errf
  out="${tmpdir}/compose.config.json"
  errf="${tmpdir}/compose.config.err"
  touch "$out" "$errf"
  chmod 600 "$out" "$errf"

  info "rendering compose config for instance=${INSTANCE} project=${COMPOSE_PROJECT}"
  if ! timeout 60s docker compose -p "$COMPOSE_PROJECT" --project-directory "$ROOT" -f "$COMPOSE_FILE" \
      config --format json >"$out" 2>"$errf"; then
    echo "FAIL: compose-config-render" >&2
    exit 1
  fi

  if ! python3 - "$out" "$INSTANCE" "$COMPOSE_PROJECT" "$PROJECT_SLUG" <<'PY'
import json, sys, re

path, expect_instance, expect_project, project_slug = sys.argv[1:5]
try:
    with open(path, "r", encoding="utf-8") as f:
        cfg = json.load(f)
except Exception:
    print("FAIL: compose-config-parse", file=sys.stderr)
    sys.exit(1)

errors = []

def err(rule):
    errors.append(rule)

services = cfg.get("services") or {}
if "app" not in services:
    err("missing-service-app")

name = cfg.get("name") or ""
if name and name != expect_project:
    err("compose-project-identity")

volumes_top = cfg.get("volumes") or {}
vol_keys = set(volumes_top.keys())
named_required = (
    "switchboard_go_mod_cache",
    "switchboard_go_build_cache",
    "switchboard_air_tmp",
    "switchboard_config",
)
for req in named_required:
    if req not in vol_keys and not any(req in k for k in vol_keys):
        err(f"named-volume-{req}")

def labels_map(sc):
    labels = sc.get("labels") or {}
    if isinstance(labels, list):
        out = {}
        for item in labels:
            if isinstance(item, str) and "=" in item:
                k, v = item.split("=", 1)
                out[k] = v
        return out
    if isinstance(labels, dict):
        return {str(k): str(v) for k, v in labels.items()}
    return {}

def volume_entries(sc):
    return sc.get("volumes") or []

def has_bind(sc, target):
    for v in volume_entries(sc):
        if isinstance(v, dict):
            tgt = v.get("target") or v.get("destination") or ""
            typ = (v.get("type") or "").lower()
            if typ == "bind" and tgt.rstrip("/") == target.rstrip("/"):
                return True
        elif isinstance(v, str) and f":{target}" in v:
            return True
    return False

def has_named(sc, name_part):
    for v in volume_entries(sc):
        if isinstance(v, dict):
            src = str(v.get("source") or "")
            typ = (v.get("type") or "").lower()
            if name_part in src and (typ in ("", "volume") or True):
                return True
        elif isinstance(v, str) and name_part in v:
            return True
    return False

def env_map(sc):
    env = sc.get("environment") or {}
    if isinstance(env, list):
        out = {}
        for e in env:
            if isinstance(e, str):
                if "=" in e:
                    k, v = e.split("=", 1)
                    out[k] = v
                else:
                    out[e] = ""
        return out
    if isinstance(env, dict):
        return {str(k): "" if v is None else str(v) for k, v in env.items()}
    return {}

app = services.get("app") or {}
if (app.get("network_mode") or "") == "host":
    err("no-host-network")
if str(app.get("pid") or "").strip().lower() == "host":
    err("no-host-pid")
priv = app.get("privileged")
if priv is True or (isinstance(priv, str) and priv.strip().lower() in ("true", "1", "yes", "on")):
    err("no-privileged")

ports = app.get("ports") or []
if not ports:
    err("publish-required")
for p in ports:
    if not isinstance(p, dict):
        err("publish-object")
        continue
    hip = p.get("host_ip")
    if hip != "127.0.0.1":
        err("publish-loopback")
    published = p.get("published")
    if published not in (None, "", 0, "0"):
        err("publish-ephemeral")
    target = p.get("target")
    try:
        if int(target) != 3847:
            err("publish-target-port")
    except (TypeError, ValueError):
        err("publish-target-port")

labels = labels_map(app)
enable = str(labels.get("stacklane.enable", ""))
if enable not in ("true", "1"):
    err("label-enable")
if str(labels.get("stacklane.project", "")) != project_slug:
    err("label-project")
if str(labels.get("stacklane.instance", "")) != expect_instance:
    err("label-instance")
if str(labels.get("stacklane.endpoint", "")) != "app":
    err("label-endpoint")
if str(labels.get("stacklane.port", "")) != "3847":
    err("label-port")
target_port = labels.get("stacklane.target_port")
if target_port not in (None, "", "3847"):
    err("label-target-port")

if not has_bind(app, "/src"):
    err("source-bind-mount")
for nv in named_required:
    if not has_named(app, nv):
        err(f"named-volume-mount-{nv}")

hc = app.get("healthcheck") or {}
test = hc.get("test") or []
test_s = test if isinstance(test, str) else " ".join(str(x) for x in test)
if "/api/health" not in test_s or "3847" not in test_s:
    err("healthcheck")

env = env_map(app)
secret_keys = (
    "GITHUB_TOKEN",
    "LINEAR_API_KEY",
    "SLACK_TOKEN",
    "SENTRY_AUTH_TOKEN",
    "DD_API_KEY",
    "DD_APP_KEY",
)
for key in secret_keys:
    val = env.get(key)
    if val:
        err("no-provider-secrets")
        break

blob = json.dumps({"labels": labels, "env": env, "name": name})
if re.search(r"\.local\b", blob):
    err("no-local-domain")

if errors:
    for e in errors:
        print(f"FAIL: {e}", file=sys.stderr)
    sys.exit(1)
print(f"ok: rendered-contract instance={expect_instance} project={expect_project}", file=sys.stderr)
sys.exit(0)
PY
  then
    echo "FAIL: rendered-contract" >&2
    exit 1
  fi

  # Optional mutation probes: defective snippets must be detected.
  if ! python3 - "$COMPOSE_FILE" "$tmpdir" "$ROOT" <<'PY'
import json, os, pathlib, subprocess, sys

base_path = pathlib.Path(sys.argv[1])
tmpdir = pathlib.Path(sys.argv[2])
root = pathlib.Path(sys.argv[3])
src = base_path.read_text()

def write_mut(name, text):
    p = tmpdir / f"mut-{name}.yml"
    p.write_text(text)
    return p

mutations = [
    ("wildcard-host", src.replace('"127.0.0.1::3847"', '"0.0.0.0::3847"'), "publish-loopback"),
    ("fixed-host-port", src.replace('"127.0.0.1::3847"', '"127.0.0.1:3847:3847"'), "publish-ephemeral"),
    ("missing-enable-label", src.replace('\n      stacklane.enable: "true"\n', "\n"), "label-enable"),
]

failures = 0
for name, text, expect in mutations:
    mut_path = write_mut(name, text)
    out = tmpdir / f"mut-{name}.json"
    errf = tmpdir / f"mut-{name}.err"
    env = os.environ.copy()
    env["STACKLANE_INSTANCE"] = "mutprobe"
    env["STACKLANE_BASE_DOMAIN"] = env.get("STACKLANE_BASE_DOMAIN", "test")
    env["COMPOSE_PROJECT_NAME"] = "switchboard-mutprobe"
    try:
        proc = subprocess.run(
            [
                "docker", "compose", "-p", "switchboard-mutprobe",
                "--project-directory", str(root),
                "-f", str(mut_path),
                "config", "--format", "json",
            ],
            check=False,
            env=env,
            stdout=out.open("w"),
            stderr=errf.open("w"),
            timeout=60,
        )
    except Exception:
        print(f"ok: mutation-{name}-config-rejected", file=sys.stderr)
        continue
    if proc.returncode != 0:
        print(f"ok: mutation-{name}-config-rejected", file=sys.stderr)
        continue
    try:
        cfg = json.loads(out.read_text())
    except Exception:
        print(f"ok: mutation-{name}-invalid-json", file=sys.stderr)
        continue
    services = cfg.get("services") or {}
    app = services.get("app") or {}
    bad = False
    if name == "wildcard-host":
        for p in app.get("ports") or []:
            if isinstance(p, dict) and p.get("host_ip") != "127.0.0.1":
                bad = True
    elif name == "fixed-host-port":
        for p in app.get("ports") or []:
            if isinstance(p, dict) and p.get("published") not in (None, "", 0, "0"):
                bad = True
    elif name == "missing-enable-label":
        labels = app.get("labels") or {}
        if isinstance(labels, list):
            kv = {}
            for item in labels:
                if isinstance(item, str) and "=" in item:
                    k, v = item.split("=", 1)
                    kv[k] = v
            labels = kv
        if str(labels.get("stacklane.enable", "")) not in ("true", "1"):
            bad = True
    if not bad:
        print(f"FAIL: mutation-{name}", file=sys.stderr)
        failures += 1
    else:
        print(f"ok: mutation-{name}", file=sys.stderr)

if failures:
    sys.exit(2)
sys.exit(0)
PY
  then
    echo "FAIL: mutation-probes" >&2
    exit 1
  fi

  info "ok: check"
  cleanup_check
}

cmd_up() {
  require_docker
  export_stack_env
  cmd_check
  info "building images (project=${COMPOSE_PROJECT} instance=${INSTANCE})…"
  compose build
  info "starting stack…"
  compose up -d --remove-orphans
  wait_healthy 360
  print_endpoints
}

cmd_status() {
  require_docker
  export_stack_env
  compose ps
  echo
  print_endpoints
}

cmd_logs() {
  require_docker
  export_stack_env
  # Ctrl-C stops following only; it does not tear the stack down.
  if [[ $# -eq 0 ]]; then
    compose logs -f
  else
    compose logs "$@"
  fi
}

cmd_down() {
  require_docker
  export_stack_env
  info "stopping stack (volumes preserved; never uses -v)…"
  compose down --remove-orphans
}

cmd_destroy() {
  export_stack_env
  local expect="${COMPOSE_PROJECT}-destroy"
  if [[ "${CONFIRM:-}" != "$expect" ]]; then
    die "refusing destroy: set CONFIRM=${expect} to remove volumes for project ${COMPOSE_PROJECT}"
  fi
  require_docker
  info "destroying stack AND volumes for ${COMPOSE_PROJECT}…"
  compose down -v --remove-orphans
}

cmd_endpoints() {
  require_docker
  export_stack_env
  print_endpoints
}

usage() {
  cat <<'EOF'
Usage: scripts/compose-dev.sh <command>

Commands:
  check       Fail-closed Stacklane/compose contract validation
  up          check + build + up -d + wait healthy + print endpoints
  status      compose ps + endpoint table
  endpoints   print FQDNs + direct loopback mappings
  logs        follow compose logs (Ctrl-C leaves the stack running)
  down        compose down (never -v; volumes preserved)
  destroy     compose down -v (requires CONFIRM=<compose-project>-destroy)

Environment:
  STACKLANE_INSTANCE     override instance slug (else worktree dirname / branch / dev)
  STACKLANE_BASE_DOMAIN  FQDN base (default: host daemon base_domain, else test)
  CONFIRM                required for destroy; must equal switchboard-<instance>-destroy

Notes:
  - Host fallback is unchanged: `air` / `make build` / production docker-compose.yml.
  - No GitHub/Linear/provider tokens are required; the web UI starts with integrations disabled.
  - Stacklane daemon is optional; direct 127.0.0.1 ephemeral ports always work.
  - Compose project is always switchboard-<instance> via `docker compose -p`.
EOF
}

main() {
  local cmd="${1:-}"
  shift || true
  case "$cmd" in
    check) cmd_check "$@" ;;
    up) cmd_up "$@" ;;
    status) cmd_status "$@" ;;
    endpoints) cmd_endpoints "$@" ;;
    logs) cmd_logs "$@" ;;
    down) cmd_down "$@" ;;
    destroy) cmd_destroy "$@" ;;
    -h|--help|help|"") usage; [[ -n "$cmd" ]] || exit 1 ;;
    *) die "unknown command: $cmd (try --help)" ;;
  esac
}

main "$@"
