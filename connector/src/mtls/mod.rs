mod peer_validate;
mod revocation_cache;
pub(crate) mod tls_config;

use std::fs;
use std::path::PathBuf;

use anyhow::{Context, Result};

use crate::config::Config;
use crate::enrollment::EnrollmentMaterial;

pub use revocation_cache::RevocationCache;

const DEVICE_CERT_FILE: &str = "device.cert.pem";
const WORKSPACE_CA_FILE: &str = "workspace.ca.pem";
const DEVICE_FINGERPRINT_FILE: &str = "device.fingerprint";
const REVOKED_FINGERPRINTS_FILE: &str = "revoked_fingerprints.log";

pub fn install_runtime_material(cfg: &Config, material: &EnrollmentMaterial) -> Result<()> {
    let storage_dir = PathBuf::from(&cfg.storage_dir);
    fs::create_dir_all(&storage_dir)
        .with_context(|| format!("create storage dir {}", storage_dir.display()))?;

    fs::write(
        storage_dir.join(DEVICE_CERT_FILE),
        &material.certificate_pem,
    )
    .with_context(|| "write device cert".to_string())?;
    fs::write(storage_dir.join(WORKSPACE_CA_FILE), &material.ca_cert_pem)
        .with_context(|| "write workspace ca".to_string())?;
    fs::write(
        storage_dir.join(DEVICE_FINGERPRINT_FILE),
        format!("{}\n", material.cert_fingerprint),
    )
    .with_context(|| "write device fingerprint".to_string())?;

    Ok(())
}

pub fn workspace_ca_path(cfg: &Config) -> PathBuf {
    PathBuf::from(&cfg.storage_dir).join(WORKSPACE_CA_FILE)
}

pub fn revoked_fingerprints_path(cfg: &Config) -> PathBuf {
    PathBuf::from(&cfg.storage_dir).join(REVOKED_FINGERPRINTS_FILE)
}

pub fn load_revocation_cache(cfg: &Config) -> Result<RevocationCache> {
    RevocationCache::load_from_file(revoked_fingerprints_path(cfg))
}

pub fn authorize_peer_certificate(
    expected_workspace_id: &str,
    peer_cert_pem: &str,
    revocation_cache: &RevocationCache,
) -> std::result::Result<peer_validate::ValidatedPeerIdentity, peer_validate::PeerValidationError> {
    peer_validate::authorize_peer_certificate(
        expected_workspace_id,
        peer_cert_pem,
        revocation_cache,
    )
}

pub fn authorize_peer_certificate_der(
    expected_workspace_id: &str,
    peer_cert_der: &[u8],
    revocation_cache: &RevocationCache,
) -> std::result::Result<peer_validate::ValidatedPeerIdentity, peer_validate::PeerValidationError> {
    peer_validate::authorize_peer_certificate_der(
        expected_workspace_id,
        peer_cert_der,
        revocation_cache,
    )
}
