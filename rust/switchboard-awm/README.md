# switchboard-awm

Consumable Rust client for Switchboard's native Agent Work Model gRPC API
(`switchboard.awm.v1`).

The crate generates tonic 0.13 / prost 0.13 bindings from the canonical proto
at [`api/switchboard/awm/v1/awm.proto`](../../api/switchboard/awm/v1/awm.proto).
Do not copy or hand-maintain a divergent schema.

## Compatibility

This crate pins **tonic 0.13** and **prost 0.13** to match Awesometree's
current gRPC stack. Bump those versions only together with Awesometree.

## Connect

```rust
use switchboard_awm::AwmClients;

# async fn demo() -> Result<(), Box<dyn std::error::Error>> {
// Default local h2c listener.
let mut clients = AwmClients::connect_loopback("").await?;

// Optional native-gRPC-only Unix socket.
let mut uds = AwmClients::connect_unix("/tmp/switchboard-awm.sock").await?;
let _ = (clients, uds);
# Ok(())
# }
```

HTTP, the web UI, and `/mcp` are not served on the Unix socket.

## Example

```bash
cargo run -p switchboard-awm --example list_projects
cargo run -p switchboard-awm --example list_projects -- unix:/tmp/switchboard-awm.sock
```

## Generation

`build.rs` compiles `proto/awm.proto`, which is copied from the canonical
`api/switchboard/awm/v1/awm.proto`. CI and `TestProtoContract` fail if the two
files diverge. Set `SWITCHBOARD_AWM_PROTO` only when building outside this tree.
