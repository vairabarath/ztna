use std::collections::HashSet;
use std::fs::OpenOptions;
use std::io::Write;
use std::path::PathBuf;
use std::sync::Arc;
use std::time::Duration;

use crate::config::Config;
use crate::enrollment::EnrollmentMaterial;
use crate::grpc::{connect_channel, ClientMtlsIdentity};
use crate::identity::Identity;
use crate::mtls::RevocationCache;
use crate::pb::ztna::controlplane::v1::{
    device_service_client::DeviceServiceClient, StreamRevocationsRequest,
};

pub async fn spawn_revocation_stream(
    cfg: Config,
    identity: Identity,
    material: Arc<tokio::sync::RwLock<EnrollmentMaterial>>,
    revocation_cache: Arc<RevocationCache>,
) {
    let mut seen = HashSet::new();

    loop {
        let cert_pem = {
            let guard = material.read().await;
            guard.certificate_pem.clone()
        };

        let channel = match connect_channel(
            &cfg,
            Some(ClientMtlsIdentity {
                cert_pem,
                key_pem: identity.private_key_pem.clone(),
            }),
        )
        .await
        {
            Ok(ch) => ch,
            Err(err) => {
                tracing::error!("connect revocation stream channel failed: {err}");
                tokio::time::sleep(Duration::from_secs(5)).await;
                continue;
            }
        };

        let mut client = DeviceServiceClient::new(channel);
        let response = client
            .stream_revocations(StreamRevocationsRequest {
                workspace_id: cfg.workspace_id.clone(),
                device_id: identity.device_id.clone(),
            })
            .await;

        let mut stream = match response {
            Ok(resp) => resp.into_inner(),
            Err(err) => {
                tracing::error!("open revocation stream failed: {err}");
                tokio::time::sleep(Duration::from_secs(5)).await;
                continue;
            }
        };

        loop {
            match stream.message().await {
                Ok(Some(ev)) => {
                    if !seen.insert(ev.cert_fingerprint.clone()) {
                        continue;
                    }
                    tracing::warn!(
                        "revocation event workspace={} device={} fingerprint={} reason={}",
                        ev.workspace_id,
                        ev.device_id,
                        ev.cert_fingerprint,
                        ev.reason
                    );
                    if let Err(err) = record_revocation(&cfg.storage_dir, &ev.cert_fingerprint) {
                        tracing::error!("persist revocation fingerprint failed: {err}");
                    }
                    revocation_cache.mark_revoked(&ev.cert_fingerprint);

                    if ev.device_id == identity.device_id {
                        tracing::error!(
                            "local device certificate was revoked (fingerprint={}), shutting down connector loop",
                            ev.cert_fingerprint
                        );
                        return;
                    }
                }
                Ok(None) => {
                    tracing::warn!("revocation stream closed by server");
                    break;
                }
                Err(err) => {
                    tracing::error!("read revocation stream failed: {err}");
                    break;
                }
            }
        }

        tokio::time::sleep(Duration::from_secs(5)).await;
    }
}

fn record_revocation(storage_dir: &str, fingerprint: &str) -> std::io::Result<()> {
    let path = PathBuf::from(storage_dir).join("revoked_fingerprints.log");
    let mut file = OpenOptions::new().create(true).append(true).open(path)?;
    writeln!(file, "{}", fingerprint)?;
    Ok(())
}
