use std::time::{Duration, SystemTime};

use anyhow::{anyhow, Context, Result};
use tokio::sync::mpsc;
use tokio_stream::wrappers::ReceiverStream;
use tonic::transport::{
    Certificate, Channel, ClientTlsConfig, Endpoint, Identity as TonicIdentity,
};

use crate::config::Config;
use crate::enrollment::EnrollmentMaterial;
use crate::identity::Identity as DeviceIdentity;
use crate::mtls::RevocationCache;
use crate::pb::ztna::dataplane::v1::{
    agent_connector_service_client::AgentConnectorServiceClient, stream_session_request,
    stream_session_response, AgentHello, AgentPing, StreamSessionRequest,
};

pub async fn spawn_stream_loop(
    cfg: Config,
    identity: DeviceIdentity,
    material: std::sync::Arc<tokio::sync::RwLock<EnrollmentMaterial>>,
    revocation_cache: std::sync::Arc<RevocationCache>,
) {
    loop {
        let current = {
            let guard = material.read().await;
            guard.clone()
        };

        if revocation_cache.is_revoked(&current.cert_fingerprint) {
            tracing::error!(
                "active certificate is revoked (fingerprint={}), stopping dataplane stream loop",
                current.cert_fingerprint
            );
            return;
        }

        match run_session(&cfg, &identity, &current).await {
            Ok(()) => {
                tracing::warn!(
                    "dataplane stream closed workspace={} connector_addr={}",
                    cfg.workspace_id,
                    cfg.connector_addr
                );
            }
            Err(err) => {
                tracing::error!(
                    "dataplane stream failed workspace={} connector_addr={} err={}",
                    cfg.workspace_id,
                    cfg.connector_addr,
                    err
                );
            }
        }

        tokio::time::sleep(Duration::from_secs(3)).await;
    }
}

async fn run_session(
    cfg: &Config,
    identity: &DeviceIdentity,
    material: &EnrollmentMaterial,
) -> Result<()> {
    let channel = connect_channel(cfg, identity, material).await?;
    let mut client = AgentConnectorServiceClient::new(channel);

    let (tx, rx) = mpsc::channel(32);
    let response = client
        .stream_session(ReceiverStream::new(rx))
        .await
        .context("open dataplane stream")?;
    let mut inbound = response.into_inner();

    tx.send(agent_hello_frame(
        &cfg.workspace_id,
        &identity.device_id,
        &material.cert_fingerprint,
    ))
    .await
    .map_err(|_| anyhow!("send hello frame"))?;

    let mut sequence = 0u64;
    let ping_every = Duration::from_secs(cfg.dataplane_ping_interval_secs.max(1));
    let mut ticker = tokio::time::interval(ping_every);
    ticker.tick().await;

    loop {
        tokio::select! {
            _ = ticker.tick() => {
                sequence = sequence.saturating_add(1);
                if tx.send(agent_ping_frame(sequence)).await.is_err() {
                    return Err(anyhow!("dataplane outbound stream is closed"));
                }
            }
            next = inbound.message() => {
                match next {
                    Ok(Some(frame)) => {
                        match frame.frame {
                            Some(stream_session_response::Frame::Hello(hello)) => {
                                if hello.workspace_id != cfg.workspace_id {
                                    return Err(anyhow!("connector hello workspace mismatch"));
                                }
                                if hello.peer_device_id != identity.device_id {
                                    return Err(anyhow!("connector hello peer_device_id mismatch"));
                                }
                                if hello.connector_cert_fingerprint.trim().is_empty() {
                                    return Err(anyhow!("connector hello missing cert fingerprint"));
                                }
                                tracing::info!(
                                    "dataplane session established workspace={} agent_device={} connector_device={} connector_fingerprint={}",
                                    cfg.workspace_id,
                                    identity.device_id,
                                    hello.connector_device_id,
                                    hello.connector_cert_fingerprint
                                );
                            }
                            Some(stream_session_response::Frame::Pong(pong)) => {
                                tracing::debug!(
                                    "dataplane pong workspace={} connector_addr={} seq={}",
                                    cfg.workspace_id,
                                    cfg.connector_addr,
                                    pong.sequence
                                );
                            }
                            Some(stream_session_response::Frame::Error(err)) => {
                                return Err(anyhow!("connector reported dataplane error: {}", err.message));
                            }
                            None => {
                                return Err(anyhow!("connector sent empty dataplane frame"));
                            }
                        }
                    }
                    Ok(None) => {
                        return Ok(());
                    }
                    Err(err) => {
                        return Err(anyhow!("read dataplane frame: {err}"));
                    }
                }
            }
        }
    }
}

async fn connect_channel(
    cfg: &Config,
    identity: &DeviceIdentity,
    material: &EnrollmentMaterial,
) -> Result<Channel> {
    let endpoint = normalize_endpoint(&cfg.connector_addr)?;
    let tls = ClientTlsConfig::new()
        .ca_certificate(Certificate::from_pem(material.ca_cert_pem.clone()))
        .identity(TonicIdentity::from_pem(
            material.certificate_pem.clone(),
            identity.private_key_pem.clone(),
        ))
        .domain_name(cfg.effective_connector_server_name());

    endpoint
        .tls_config(tls)
        .context("configure connector TLS")?
        .connect()
        .await
        .context("connect to connector")
}

fn normalize_endpoint(connector_addr: &str) -> Result<Endpoint> {
    let trimmed = connector_addr.trim();
    if trimmed.is_empty() {
        return Err(anyhow!(
            "connector address is required for dataplane stream (set --connector-addr)"
        ));
    }

    let uri = if trimmed.starts_with("https://") {
        trimmed.to_string()
    } else if trimmed.starts_with("http://") {
        return Err(anyhow!(
            "insecure connector URI is not allowed; use https://"
        ));
    } else {
        format!("https://{trimmed}")
    };

    Endpoint::from_shared(uri).context("parse connector endpoint")
}

fn agent_hello_frame(
    workspace_id: &str,
    device_id: &str,
    cert_fingerprint: &str,
) -> StreamSessionRequest {
    StreamSessionRequest {
        frame: Some(stream_session_request::Frame::Hello(AgentHello {
            workspace_id: workspace_id.to_string(),
            device_id: device_id.to_string(),
            cert_fingerprint: cert_fingerprint.to_string(),
        })),
    }
}

fn agent_ping_frame(sequence: u64) -> StreamSessionRequest {
    StreamSessionRequest {
        frame: Some(stream_session_request::Frame::Ping(AgentPing {
            sequence,
            sent_at: Some(prost_types::Timestamp::from(SystemTime::now())),
        })),
    }
}
