mod service;

use std::net::SocketAddr;
use std::sync::Arc;

use anyhow::{Context, Result};
use tonic::transport::{Certificate, Identity, Server, ServerTlsConfig};

use crate::config::Config;
use crate::enrollment::EnrollmentMaterial;
use crate::identity::Identity as DeviceIdentity;
use crate::mtls::RevocationCache;
use crate::pb::ztna::dataplane::v1::agent_connector_service_server::AgentConnectorServiceServer;

pub async fn serve(
    cfg: Config,
    identity: DeviceIdentity,
    material: EnrollmentMaterial,
    revocation_cache: Arc<RevocationCache>,
) -> Result<()> {
    let addr: SocketAddr = cfg
        .dataplane_listen_addr
        .parse()
        .with_context(|| format!("parse dataplane listen addr {}", cfg.dataplane_listen_addr))?;

    let tls = ServerTlsConfig::new()
        .identity(Identity::from_pem(
            material.certificate_pem.clone(),
            identity.private_key_pem.clone(),
        ))
        .client_ca_root(Certificate::from_pem(material.ca_cert_pem.clone()));

    let service = service::Handler::new(
        cfg.workspace_id.clone(),
        identity.device_id.clone(),
        material.cert_fingerprint.clone(),
        revocation_cache,
    );

    tracing::info!(
        "dataplane gRPC listening on {} workspace={} connector_device={}",
        addr,
        cfg.workspace_id,
        identity.device_id
    );

    Server::builder()
        .tls_config(tls)
        .context("configure dataplane TLS")?
        .add_service(AgentConnectorServiceServer::new(service))
        .serve(addr)
        .await
        .context("run dataplane gRPC server")
}
