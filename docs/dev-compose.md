# Stacklane-compatible Docker Compose DEV stack

Parallel worktree-friendly local stack: Switchboard HTTP + web UI with Air
hot-reload, published only on `127.0.0.1::<containerPort>` (ephemeral host
ports) with Stacklane labels for stable FQDNs when the Stacklane daemon is
installed.

**No GitHub/Linear/provider tokens are required.** The UI starts with
integrations disabled. Configure credentials later through the web UI or by
mounting a config volume if you choose.

Host fallback is **unchanged**:

- `air` / `make build` / `make install` for non-Docker development
- production `Dockerfile` + `docker-compose.yml` (`ghcr.io/daltoniam/switchboard:latest`)

## Prerequisites

- Docker Engine + Compose v2
- Optional: Stacklane daemon for `app.<instance>.switchboard.test:3847`
- Stacklane install/daemon is **separate**. This stack works with **direct
  loopback ephemeral ports** if the daemon is absent (`stacklane: BLOCKED` in
  status is OK).

## Copy-paste commands

```bash
make compose-up
make compose-status
make compose-logs
make compose-down
CONFIRM=switchboard-<instance>-destroy make compose-destroy
```

Fail-closed contract check only:

```bash
make compose-check
# or
bash scripts/compose-dev.sh check
```

Override instance slug (sanitized to `[a-z0-9-]`, max 48):

```bash
STACKLANE_INSTANCE=my-feature make compose-up
```

Default instance is the worktree directory name (e.g. `stacklane-compose`),
else branch name, else `dev`. Compose project is always
`switchboard-<instance>` via `docker compose -p` (never hardcoded in the YAML).

`logs` follows service output. **Ctrl-C leaves the stack running**; use
`make compose-down` to stop.

`down` never passes `-v`. `destroy` requires
`CONFIRM=switchboard-<instance>-destroy` and then removes named volumes for
**that compose project only**.

## Endpoints

| Path | Meaning |
|------|---------|
| `http://app.<instance>.switchboard.test:3847` | App via Stacklane VIP public port **3847** |
| `http://127.0.0.1:<ephemeral>/` | Direct UI (compose published port) |
| `http://127.0.0.1:<ephemeral>/api/health` | Health (`{"status":"healthy"}`) |
| `http://127.0.0.1:<ephemeral>/mcp` | MCP endpoint |

Print mappings:

```bash
make compose-endpoints
# or
bash scripts/compose-dev.sh endpoints
```

## Architecture

- **app** (`Dockerfile.dev`): golang `1.26.6-bookworm`, Air
  (`github.com/air-verse/air` MIT, pinned `v1.67.4`) as PID1, repo bind-mounted
  at `/src`, named volumes for Go mod/build caches, `/src/tmp` binary output,
  and `/root/.config/switchboard`.
- **Publish form:** only `127.0.0.1::3847` (ephemeral host port). Never
  `0.0.0.0`, empty host IP, fixed host ports, or host network.
- **Labels:** `stacklane.enable=true`, `stacklane.project=switchboard`,
  `stacklane.instance=${STACKLANE_INSTANCE}`, `stacklane.endpoint=app`,
  `stacklane.port=3847`. Container listen port equals public port, so
  `stacklane.target_port` is omitted.
- **Healthcheck:** `curl -fsS http://127.0.0.1:3847/api/health`.
- **Auth:** no provider env interpolation. Optional tokens stay on the host
  path; the DEV stack does not require them.

## Files

| File | Role |
|------|------|
| `docker-compose.dev.yml` | Stacklane DEV compose (this stack) |
| `docker-compose.yml` | Existing production image stack (host fallback) |
| `Dockerfile.dev` | Air + Go toolchain image |
| `Dockerfile` | Production scratch image (unchanged) |
| `scripts/compose-dev.sh` | Lifecycle: check/up/status/endpoints/logs/down/destroy |
| `paseo.json` | Workspace scripts wrapping the Make targets |

Do not mutate Stacklane daemon, DNS, or systemd from this lifecycle.
