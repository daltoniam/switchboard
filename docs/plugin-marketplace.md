# Plugin Marketplace

The plugin marketplace lets users discover, install, and manage WASM plugins from the Switchboard web UI.

## Architecture

### Plugin ABI (v1)

Switchboard WASM plugins **must** export these functions:

| Export | Signature | Required |
|--------|-----------|----------|
| `name()` | `-> ptr_size` | Yes |
| `tools()` | `-> ptr_size` | Yes |
| `configure(ptr_size)` | `-> ptr_size` | Yes |
| `execute(ptr_size)` | `-> ptr_size` | Yes |
| `healthy()` | `-> i32` | Yes |
| `metadata()` | `-> ptr_size` | Yes |
| `malloc(size)` / `guest_malloc(size)` | `-> ptr` | Yes |
| `free(ptr)` / `guest_free(ptr)` | | Yes |

The `metadata()` export returns JSON:

```json
{
  "name": "my-plugin",
  "version": "1.2.0",
  "abi_version": 1,
  "description": "What this plugin does",
  "author": "Author Name",
  "homepage": "https://github.com/...",
  "license": "MIT",
  "capabilities": ["http"]
}
```

ABI version compatibility uses a min/max range in manifests. The host declares `ABIVersion = 1`; plugins with `abi_min <= 1 <= abi_max` are compatible.

### Manifest Format (Schema v1)

Manifests are JSON files listing available plugins:

```json
{
  "schema_version": 1,
  "name": "switchboard-official",
  "description": "Official Switchboard plugin registry",
  "plugins": [
    {
      "name": "example",
      "description": "Example plugin",
      "author": "Switchboard",
      "homepage": "https://github.com/daltoniam/switchboard",
      "license": "MIT",
      "versions": [
        {
          "version": "1.0.0",
          "abi_min": 1,
          "abi_max": 1,
          "url": "https://example.com/plugins/example-1.0.0.wasm",
          "sha256": "abc123...",
          "size": 1048576,
          "released_at": "2025-01-15T00:00:00Z",
          "changelog": "Initial release",
          "platforms": []
        }
      ]
    }
  ]
}
```

The official manifest is committed at `plugins/manifest.json` in the Switchboard repo. Third-party manifests can be added via the web UI.

### Plugin Sources

Users can install plugins from four sources:

1. **Manifest browsing** — browse plugins from configured manifest URLs, one-click install
2. **Direct URL** — paste a URL to any `.wasm` file
3. **File upload** — drag and drop a `.wasm` from the browser
4. **Local path** — existing WASM Modules page (unchanged)

### Auto-Update System

- Configurable via web UI (global toggle + per-plugin)
- Default check interval: 6 hours (configurable via `check_interval` in config)
- Background goroutine checks manifests and downloads new versions
- SHA256 verification on every download
- Manual "Check Now" button in the web UI
- Can be fully disabled

### Configuration

The marketplace config is stored in `~/.config/switchboard/config.json` under the `marketplace` key:

```json
{
  "marketplace": {
    "manifest_sources": [
      {
        "url": "https://raw.githubusercontent.com/daltoniam/switchboard/main/plugins/manifest.json",
        "name": "Official",
        "enabled": true
      }
    ],
    "installed_plugins": [
      {
        "name": "example",
        "version": "1.0.0",
        "manifest_url": "...",
        "installed_at": "2025-01-15T10:30:00Z",
        "path": "~/.config/switchboard/plugins/example.wasm",
        "sha256": "abc123...",
        "auto_update": true
      }
    ],
    "auto_update": false,
    "check_interval": "6h",
    "plugin_dir": "",
    "last_check": "2025-01-15T16:30:00Z"
  }
}
```

### Web UI

The Plugin Marketplace page (`/plugins`) provides:

- **Installed plugins** — list with version, path, update/uninstall buttons
- **Available plugins** — browsed from configured manifests with install buttons
- **Install from URL** — text input for direct WASM URL
- **Upload plugin** — file picker for browser upload
- **Manifest sources** — add/remove third-party manifest URLs
- **Auto-update toggle** — global on/off with "Check Now" button

### Rust Guest SDK

The `switchboard-guest-sdk` crate provides a `PluginMetadata` struct and `leaked_metadata()` helper:

```rust
#[no_mangle]
pub extern "C" fn metadata() -> u64 {
    sdk::leaked_metadata(&sdk::PluginMetadata {
        name: "my-plugin".into(),
        version: "1.0.0".into(),
        abi_version: 1,
        description: "My plugin".into(),
        author: "Me".into(),
        ..Default::default()
    })
}
```

## File Layout

```
marketplace/
  marketplace.go       — Manager, manifest fetching, install/update/uninstall
  marketplace_test.go  — Full test suite
plugins/
  manifest.json        — Official plugin manifest (committed to repo)
wasm/
  runtime.go           — Added fnMetadata field (optional export)
  module.go            — Added Metadata() and HasMetadata() methods
  guest-rust/sdk/src/lib.rs — Added PluginMetadata + leaked_metadata()
web/
  web.go               — Updated New() to accept *marketplace.Manager
  web_marketplace.go   — All /plugins/* route handlers
  templates/pages/plugin_marketplace.templ — Marketplace UI template
  templates/layouts/base.templ — Added "Plugin Marketplace" nav item
mcp.go                 — Added MarketplaceConfig types, Config.Marketplace field
config/config.go       — Preserve marketplace config in mergeWithDefaults
cmd/server/main.go     — Wire marketplace manager, start auto-update loop
```
