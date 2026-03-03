use anyhow::{Context, Result};
use tonic::transport::{Channel, Endpoint};

use crate::config::Config;
use crate::mtls::tls_config::build_controller_tls_config;

#[derive(Debug, Clone)]
pub struct ClientMtlsIdentity {
    pub cert_pem: String,
    pub key_pem: String,
}

pub async fn connect_channel(cfg: &Config, mtls: Option<ClientMtlsIdentity>) -> Result<Channel> {
    let mut endpoint = normalize_endpoint(&cfg.controller_addr)?;

    if uses_tls(&cfg.controller_addr) {
        let tls = build_controller_tls_config(cfg, mtls)?;
        endpoint = endpoint
            .tls_config(tls)
            .context("configure controller TLS")?;
    }

    endpoint.connect().await.context("connect to controller")
}

fn normalize_endpoint(addr: &str) -> Result<Endpoint> {
    let trimmed = addr.trim();
    let uri = if trimmed.starts_with("http://") || trimmed.starts_with("https://") {
        trimmed.to_string()
    } else {
        format!("http://{trimmed}")
    };

    Endpoint::from_shared(uri).context("parse controller endpoint")
}

fn uses_tls(addr: &str) -> bool {
    addr.trim().starts_with("https://")
}
