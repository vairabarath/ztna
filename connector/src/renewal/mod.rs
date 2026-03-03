mod scheduler;

use std::sync::Arc;
use std::time::Duration;

use crate::config::Config;
use crate::enrollment::{self, EnrollmentMaterial};
use crate::identity::Identity;
use crate::mtls;
use crate::mtls::RevocationCache;

pub async fn spawn_renewal_loop(
    cfg: Config,
    identity: Identity,
    material: Arc<tokio::sync::RwLock<EnrollmentMaterial>>,
    revocation_cache: Arc<RevocationCache>,
) {
    loop {
        let current = {
            let guard = material.read().await;
            guard.clone()
        };

        if revocation_cache.is_revoked(&current.cert_fingerprint) {
            tracing::error!(
                "active certificate is revoked (fingerprint={}), stopping renewal loop",
                current.cert_fingerprint
            );
            return;
        }
        let delay = scheduler::renewal_delay(std::time::SystemTime::now(), current.expires_at);
        tracing::info!("next certificate renewal in {:?}", delay);
        tokio::time::sleep(delay).await;

        match enrollment::renew(&cfg, &identity, &current.certificate_pem).await {
            Ok(new_material) => {
                if let Err(err) = mtls::install_runtime_material(&cfg, &new_material) {
                    tracing::error!("renewal succeeded but failed to persist cert material: {err}");
                    tokio::time::sleep(Duration::from_secs(5)).await;
                    continue;
                }
                let mut guard = material.write().await;
                *guard = new_material;
                tracing::info!("certificate renewed successfully");
            }
            Err(err) => {
                tracing::error!("certificate renewal failed: {err}");
                tokio::time::sleep(Duration::from_secs(30)).await;
            }
        }
    }
}
