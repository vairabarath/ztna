use std::sync::Arc;
use std::time::SystemTime;

use tokio::sync::mpsc;
use tokio_stream::wrappers::ReceiverStream;
use tonic::{Request, Response, Status};

use crate::mtls;
use crate::mtls::RevocationCache;
use crate::pb::ztna::dataplane::v1::{
    agent_connector_service_server::AgentConnectorService, stream_session_request,
    stream_session_response, AgentHello, AgentPing, ConnectorError, ConnectorHello, ConnectorPong,
    StreamSessionRequest, StreamSessionResponse,
};

#[derive(Debug, Clone)]
struct PeerClaims {
    device_id: String,
    cert_fingerprint: String,
}

#[derive(Clone)]
pub struct Handler {
    workspace_id: String,
    connector_device_id: String,
    connector_cert_fingerprint: String,
    revocation_cache: Arc<RevocationCache>,
}

impl Handler {
    pub fn new(
        workspace_id: String,
        connector_device_id: String,
        connector_cert_fingerprint: String,
        revocation_cache: Arc<RevocationCache>,
    ) -> Self {
        Self {
            workspace_id,
            connector_device_id,
            connector_cert_fingerprint,
            revocation_cache,
        }
    }
}

#[tonic::async_trait]
impl AgentConnectorService for Handler {
    type StreamSessionStream = ReceiverStream<Result<StreamSessionResponse, Status>>;

    async fn stream_session(
        &self,
        request: Request<tonic::Streaming<StreamSessionRequest>>,
    ) -> Result<Response<Self::StreamSessionStream>, Status> {
        let peer = authorize_peer(&self.workspace_id, &request, self.revocation_cache.as_ref())?;

        let peer_device_id = peer.device_id.clone();
        let peer_fingerprint = peer.cert_fingerprint.clone();
        tracing::info!(
            "agent dataplane session accepted workspace={} agent_device={} agent_fingerprint={}",
            self.workspace_id,
            peer_device_id,
            peer_fingerprint
        );

        let mut inbound = request.into_inner();
        let (tx, rx) = mpsc::channel(32);

        if tx
            .send(Ok(connector_hello_frame(
                &self.workspace_id,
                &self.connector_device_id,
                &self.connector_cert_fingerprint,
                &peer_device_id,
            )))
            .await
            .is_err()
        {
            return Err(Status::unavailable("stream closed before connector hello"));
        }

        tokio::spawn({
            let tx = tx.clone();
            let workspace_id = self.workspace_id.clone();
            let claimed_peer_device_id = peer_device_id.clone();
            let claimed_peer_fingerprint = peer_fingerprint.clone();

            async move {
                let mut hello_seen = false;

                loop {
                    let next = inbound.message().await;
                    let frame = match next {
                        Ok(Some(frame)) => frame,
                        Ok(None) => {
                            tracing::info!(
                                "agent dataplane stream closed workspace={} agent_device={}",
                                workspace_id,
                                claimed_peer_device_id
                            );
                            break;
                        }
                        Err(err) => {
                            tracing::warn!(
                                "read agent dataplane frame failed workspace={} agent_device={} err={}",
                                workspace_id,
                                claimed_peer_device_id,
                                err
                            );
                            break;
                        }
                    };

                    match frame.frame {
                        Some(stream_session_request::Frame::Hello(hello)) => {
                            match validate_hello(
                                &workspace_id,
                                &claimed_peer_device_id,
                                &claimed_peer_fingerprint,
                                &hello,
                            ) {
                                Ok(()) => {
                                    hello_seen = true;
                                }
                                Err(msg) => {
                                    let _ = tx.send(Ok(error_frame(msg))).await;
                                    break;
                                }
                            }
                        }
                        Some(stream_session_request::Frame::Ping(AgentPing {
                            sequence, ..
                        })) => {
                            if !hello_seen {
                                let _ = tx
                                    .send(Ok(error_frame(
                                        "agent hello frame is required before ping".to_string(),
                                    )))
                                    .await;
                                break;
                            }

                            let sent = tx.send(Ok(connector_pong_frame(sequence))).await;
                            if sent.is_err() {
                                break;
                            }
                        }
                        None => {
                            let _ = tx
                                .send(Ok(error_frame("received empty agent frame".to_string())))
                                .await;
                            break;
                        }
                    }
                }
            }
        });

        Ok(Response::new(ReceiverStream::new(rx)))
    }
}

fn authorize_peer(
    workspace_id: &str,
    request: &Request<tonic::Streaming<StreamSessionRequest>>,
    revocation_cache: &RevocationCache,
) -> Result<PeerClaims, Status> {
    let certs = request
        .peer_certs()
        .ok_or_else(|| Status::unauthenticated("client certificate is required"))?;
    let cert = certs
        .first()
        .ok_or_else(|| Status::unauthenticated("no client certificate presented"))?;

    let validated =
        mtls::authorize_peer_certificate_der(workspace_id, cert.as_ref(), revocation_cache)
            .map_err(|err| {
                Status::permission_denied(format!("peer certificate rejected: {err}"))
            })?;

    Ok(PeerClaims {
        device_id: validated.device_id,
        cert_fingerprint: validated.cert_fingerprint,
    })
}

fn validate_hello(
    expected_workspace_id: &str,
    expected_device_id: &str,
    expected_fingerprint: &str,
    hello: &AgentHello,
) -> Result<(), String> {
    if hello.workspace_id != expected_workspace_id {
        return Err("agent hello workspace_id mismatch".to_string());
    }
    if hello.device_id != expected_device_id {
        return Err("agent hello device_id mismatch".to_string());
    }
    if hello.cert_fingerprint != expected_fingerprint {
        return Err("agent hello cert_fingerprint mismatch".to_string());
    }
    Ok(())
}

fn connector_hello_frame(
    workspace_id: &str,
    connector_device_id: &str,
    connector_cert_fingerprint: &str,
    peer_device_id: &str,
) -> StreamSessionResponse {
    StreamSessionResponse {
        frame: Some(stream_session_response::Frame::Hello(ConnectorHello {
            workspace_id: workspace_id.to_string(),
            connector_device_id: connector_device_id.to_string(),
            peer_device_id: peer_device_id.to_string(),
            connector_cert_fingerprint: connector_cert_fingerprint.to_string(),
        })),
    }
}

fn connector_pong_frame(sequence: u64) -> StreamSessionResponse {
    StreamSessionResponse {
        frame: Some(stream_session_response::Frame::Pong(ConnectorPong {
            sequence,
            received_at: Some(prost_types::Timestamp::from(SystemTime::now())),
        })),
    }
}

fn error_frame(message: String) -> StreamSessionResponse {
    StreamSessionResponse {
        frame: Some(stream_session_response::Frame::Error(ConnectorError {
            message,
        })),
    }
}
