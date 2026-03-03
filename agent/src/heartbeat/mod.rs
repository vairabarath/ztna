use std::sync::Arc;
use std::time::Duration;

use crate::config::Config;
use crate::enrollment::EnrollmentMaterial;
use crate::grpc::{connect_channel, ClientMtlsIdentity};
use crate::identity::Identity;
use crate::mtls::RevocationCache;
use crate::pb::ztna::controlplane::v1::{
    device_service_client::DeviceServiceClient, HeartbeatRequest,
};

pub async fn spawn_heartbeat_loop(
    cfg: Config,
    identity: Identity,
    material: Arc<tokio::sync::RwLock<EnrollmentMaterial>>,
    revocation_cache: Arc<RevocationCache>,
) {
    let heartbeat_interval = cfg.dataplane_ping_interval_secs.max(5);

    loop {
        let current = {
            let guard = material.read().await;
            guard.clone()
        };

        if revocation_cache.is_revoked(&current.cert_fingerprint) {
            tracing::error!(
                "active certificate is revoked (fingerprint={}), stopping heartbeat loop",
                current.cert_fingerprint
            );
            return;
        }

        let channel = match connect_channel(
            &cfg,
            Some(ClientMtlsIdentity {
                cert_pem: current.certificate_pem,
                key_pem: identity.private_key_pem.clone(),
            }),
        )
        .await
        {
            Ok(ch) => ch,
            Err(err) => {
                tracing::warn!("heartbeat channel connect failed: {err}");
                tokio::time::sleep(Duration::from_secs(5)).await;
                continue;
            }
        };

        let mut client = DeviceServiceClient::new(channel);
        if let Err(err) = client
            .heartbeat(HeartbeatRequest {
                workspace_id: cfg.workspace_id.clone(),
                device_id: identity.device_id.clone(),
                cert_fingerprint: current.cert_fingerprint.clone(),
            })
            .await
        {
            tracing::warn!("heartbeat request failed: {err}");
            tokio::time::sleep(Duration::from_secs(5)).await;
            continue;
        }

        tokio::time::sleep(Duration::from_secs(heartbeat_interval)).await;
    }
}
