mod client;
mod csr;

use anyhow::Result;
use std::time::SystemTime;

use crate::config::Config;
use crate::identity::Identity;

#[derive(Debug, Clone)]
pub struct EnrollmentMaterial {
    pub certificate_pem: String,
    pub ca_cert_pem: String,
    pub cert_fingerprint: String,
    pub expires_at: SystemTime,
}

pub async fn enroll(
    cfg: &Config,
    ca_cert_pem: &str,
    identity: &Identity,
) -> Result<EnrollmentMaterial> {
    let csr_pem = csr::build_csr(cfg, identity)?;
    client::enroll(cfg, ca_cert_pem, identity, &csr_pem).await
}

pub async fn renew(
    cfg: &Config,
    identity: &Identity,
    client_cert_pem: &str,
) -> Result<EnrollmentMaterial> {
    let csr_pem = csr::build_csr(cfg, identity)?;
    client::renew(cfg, identity, &csr_pem, client_cert_pem).await
}
