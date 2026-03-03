mod bootstrap;
mod cli;
mod config;
mod dataplane;
mod enrollment;
mod grpc;
mod heartbeat;
mod identity;
mod mtls;
mod pb;
mod renewal;
mod revocations;
mod storage;

use anyhow::Result;
use clap::Parser;
use std::sync::Arc;

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(tracing_subscriber::EnvFilter::from_default_env())
        .init();

    let args = cli::Args::parse();
    let cfg = config::Config::from_args(args);

    let ca = bootstrap::fetch_workspace_ca(&cfg).await?;
    let identity = identity::ensure_identity(&cfg.storage_dir, Some(&cfg.device_id))?;

    let enrolled = enrollment::enroll(&cfg, &ca, &identity).await?;
    mtls::install_runtime_material(&cfg, &enrolled)?;
    let revocation_cache = Arc::new(mtls::load_revocation_cache(&cfg)?);
    let local_claims = mtls::authorize_peer_certificate(
        &cfg.workspace_id,
        &enrolled.certificate_pem,
        &revocation_cache,
    )
    .map_err(|err| anyhow::anyhow!("local certificate SAN validation failed: {err}"))?;
    tracing::info!(
        "runtime identity ready workspace={} device={} fingerprint={}",
        local_claims.workspace_id,
        local_claims.device_id,
        local_claims.cert_fingerprint
    );

    let shared_material = Arc::new(tokio::sync::RwLock::new(enrolled));
    let dataplane_task = if cfg.connector_addr.trim().is_empty() {
        None
    } else {
        Some(tokio::spawn(dataplane::spawn_stream_loop(
            cfg.clone(),
            identity.clone(),
            shared_material.clone(),
            revocation_cache.clone(),
        )))
    };

    let renew_task = tokio::spawn(renewal::spawn_renewal_loop(
        cfg.clone(),
        identity.clone(),
        shared_material.clone(),
        revocation_cache.clone(),
    ));
    let revocation_task = tokio::spawn(revocations::spawn_revocation_stream(
        cfg.clone(),
        identity.clone(),
        shared_material.clone(),
        revocation_cache.clone(),
    ));
    let heartbeat_task = tokio::spawn(heartbeat::spawn_heartbeat_loop(
        cfg.clone(),
        identity.clone(),
        shared_material.clone(),
        revocation_cache.clone(),
    ));

    if let Some(dataplane_task) = dataplane_task {
        tokio::select! {
            _ = renew_task => {}
            _ = revocation_task => {}
            _ = heartbeat_task => {}
            _ = dataplane_task => {}
        }
    } else {
        tracing::warn!(
            "agent dataplane stream is disabled; set --connector-addr to connect to a connector"
        );
        tokio::select! {
            _ = renew_task => {}
            _ = revocation_task => {}
            _ = heartbeat_task => {}
        }
    }
    Ok(())
}
