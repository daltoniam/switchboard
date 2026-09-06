//! Consumable tonic client for Switchboard's `switchboard.awm.v1` API.
//!
//! Bindings are generated from the canonical proto at
//! `api/switchboard/awm/v1/awm.proto`. Do not maintain a divergent schema.

#![deny(missing_docs)]

/// Generated `switchboard.awm.v1` messages and clients.
#[allow(missing_docs)]
pub mod proto {
    tonic::include_proto!("switchboard.awm.v1");
}

pub use proto::{
    agent_profile_service_client::AgentProfileServiceClient,
    project_catalog_service_client::ProjectCatalogServiceClient,
    resource_binding_service_client::ResourceBindingServiceClient,
    resource_service_client::ResourceServiceClient,
    work_profile_service_client::WorkProfileServiceClient,
    work_session_service_client::WorkSessionServiceClient, ProjectDefinition,
};

use tonic::transport::{Channel, Endpoint, Uri};

/// Default loopback h2c endpoint used by local Switchboard installs.
pub const DEFAULT_LOOPBACK_ENDPOINT: &str = "http://127.0.0.1:3847";

/// Connect to native AWM gRPC over loopback h2c.
///
/// `target` may be `http://127.0.0.1:3847`, `127.0.0.1:3847`, or empty to use
/// [`DEFAULT_LOOPBACK_ENDPOINT`].
pub async fn connect_loopback(target: impl AsRef<str>) -> Result<Channel, tonic::transport::Error> {
    let target = target.as_ref().trim();
    let uri = if target.is_empty() {
        DEFAULT_LOOPBACK_ENDPOINT.to_string()
    } else if target.starts_with("http://") || target.starts_with("https://") {
        target.to_string()
    } else {
        format!("http://{target}")
    };
    Endpoint::from_shared(uri)?.connect().await
}

/// Connect to native AWM gRPC over a Unix-domain socket.
///
/// HTTP/MCP is never served on this socket; only the typed AWM services are.
#[cfg(unix)]
pub async fn connect_unix(path: impl AsRef<std::path::Path>) -> Result<Channel, std::io::Error> {
    use hyper_util::rt::TokioIo;
    use std::path::PathBuf;
    use tokio::net::UnixStream;
    use tower::service_fn;

    let path = PathBuf::from(path.as_ref());
    let endpoint = Endpoint::from_static("http://[::]:0");
    endpoint
        .connect_with_connector(service_fn(move |_: Uri| {
            let path = path.clone();
            async move {
                let stream = UnixStream::connect(path).await?;
                Ok::<_, std::io::Error>(TokioIo::new(stream))
            }
        }))
        .await
        .map_err(std::io::Error::other)
}

/// Connect helper that is unavailable on Windows.
#[cfg(not(unix))]
pub async fn connect_unix(
    _path: impl AsRef<std::path::Path>,
) -> Result<Channel, std::io::Error> {
    Err(std::io::Error::new(
        std::io::ErrorKind::Unsupported,
        "unix domain sockets are not supported on this platform",
    ))
}

/// Typed client bundle for the implemented AWM services.
#[derive(Clone, Debug)]
pub struct AwmClients {
    /// Project catalog RPCs.
    pub projects: ProjectCatalogServiceClient<Channel>,
    /// Work profile RPCs.
    pub work_profiles: WorkProfileServiceClient<Channel>,
    /// Agent profile RPCs.
    pub agent_profiles: AgentProfileServiceClient<Channel>,
    /// Work session RPCs.
    pub work_sessions: WorkSessionServiceClient<Channel>,
    /// Resource RPCs.
    pub resources: ResourceServiceClient<Channel>,
    /// Resource binding RPCs.
    pub resource_bindings: ResourceBindingServiceClient<Channel>,
}

impl AwmClients {
    /// Wrap an existing channel.
    pub fn new(channel: Channel) -> Self {
        Self {
            projects: ProjectCatalogServiceClient::new(channel.clone()),
            work_profiles: WorkProfileServiceClient::new(channel.clone()),
            agent_profiles: AgentProfileServiceClient::new(channel.clone()),
            work_sessions: WorkSessionServiceClient::new(channel.clone()),
            resources: ResourceServiceClient::new(channel.clone()),
            resource_bindings: ResourceBindingServiceClient::new(channel),
        }
    }

    /// Connect over loopback h2c and return typed clients.
    pub async fn connect_loopback(
        target: impl AsRef<str>,
    ) -> Result<Self, tonic::transport::Error> {
        Ok(Self::new(connect_loopback(target).await?))
    }

    /// Connect over a Unix-domain socket and return typed clients.
    pub async fn connect_unix(path: impl AsRef<std::path::Path>) -> Result<Self, std::io::Error> {
        Ok(Self::new(connect_unix(path).await?))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn proto_includes_known_resource_ids() {
        let def = ProjectDefinition {
            known_resource_ids: vec!["repo".into()],
            ..Default::default()
        };
        assert_eq!(def.known_resource_ids, ["repo"]);
    }

    #[test]
    fn loopback_default_is_h2c_localhost() {
        assert_eq!(DEFAULT_LOOPBACK_ENDPOINT, "http://127.0.0.1:3847");
    }

    #[test]
    fn crate_proto_matches_canonical_when_present() {
        let crate_proto = include_str!("../proto/awm.proto");
        assert!(crate_proto.contains("package switchboard.awm.v1"));
        assert!(crate_proto.contains("repeated string known_resource_ids"));
        let canonical = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("../../api/switchboard/awm/v1/awm.proto");
        if let Ok(canonical) = std::fs::read_to_string(canonical) {
            assert_eq!(crate_proto, canonical);
        }
    }
}
