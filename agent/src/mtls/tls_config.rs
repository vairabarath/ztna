use std::fs;

use anyhow::{anyhow, Context, Result};
use tonic::transport::{Certificate, ClientTlsConfig, Identity};

use crate::config::Config;
use crate::grpc::ClientMtlsIdentity;
use crate::mtls;

pub fn build_controller_tls_config(
    cfg: &Config,
    identity: Option<ClientMtlsIdentity>,
) -> Result<ClientTlsConfig> {
    let ca_pem = load_controller_ca_pem(cfg)?;

    let mut tls = ClientTlsConfig::new().ca_certificate(Certificate::from_pem(ca_pem));
    if let Some(id) = identity {
        tls = tls.identity(Identity::from_pem(id.cert_pem, id.key_pem));
    }
    Ok(tls)
}

fn load_controller_ca_pem(cfg: &Config) -> Result<String> {
    if !cfg.controller_ca_cert.trim().is_empty() {
        return fs::read_to_string(&cfg.controller_ca_cert)
            .with_context(|| format!("read controller CA cert at {}", cfg.controller_ca_cert));
    }

    let runtime_ca = mtls::workspace_ca_path(cfg);
    if runtime_ca.exists() {
        return fs::read_to_string(&runtime_ca)
            .with_context(|| format!("read workspace CA cert at {}", runtime_ca.display()));
    }

    Err(anyhow!(
        "missing controller trust anchor: set --controller-ca-cert or run enrollment first"
    ))
}
