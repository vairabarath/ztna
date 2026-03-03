use anyhow::Result;
use rcgen::{CertificateParams, DistinguishedName, DnType, KeyPair, SanType};

use crate::config::Config;
use crate::identity::Identity;

pub fn build_csr(cfg: &Config, identity: &Identity) -> Result<String> {
    let key_pair = KeyPair::from_pem(&identity.private_key_pem)?;
    let connector_service_dns = cfg.dataplane_server_name();

    let mut params = CertificateParams::new(vec![
        format!("{}.{}", identity.device_id, cfg.workspace_id),
        connector_service_dns,
    ])?;

    let mut dn = DistinguishedName::new();
    dn.push(DnType::CommonName, identity.device_id.clone());
    dn.push(DnType::OrganizationName, cfg.workspace_id.clone());
    params.distinguished_name = dn;

    let uri = format!("ztna://{}/{}", cfg.workspace_id, identity.device_id).parse()?;
    params.subject_alt_names.push(SanType::URI(uri));

    let csr = params.serialize_request(&key_pair)?;
    Ok(csr.pem()?)
}

#[cfg(test)]
mod tests {
    use super::build_csr;
    use crate::config::Config;
    use crate::identity::Identity;
    use rcgen::{KeyPair, PKCS_ECDSA_P256_SHA256};
    use std::io::Cursor;

    #[test]
    fn csr_contains_workspace_and_device_claims() {
        let key = KeyPair::generate_for(&PKCS_ECDSA_P256_SHA256).expect("key");
        let identity = Identity {
            device_id: "device-123".to_string(),
            private_key_pem: key.serialize_pem(),
        };
        let cfg = Config {
            workspace_id: "demo".to_string(),
            bootstrap_token: "ignored".to_string(),
            controller_addr: "https://127.0.0.1:8443".to_string(),
            controller_ca_cert: String::new(),
            storage_dir: "/tmp/ztna-test".to_string(),
            dataplane_listen_addr: "0.0.0.0:9443".to_string(),
        };

        let csr_pem = build_csr(&cfg, &identity).expect("csr");
        assert!(csr_pem.contains("BEGIN CERTIFICATE REQUEST"));

        let mut cursor = Cursor::new(csr_pem.as_bytes());
        let der = rustls_pemfile::csr(&mut cursor)
            .expect("parse pem")
            .expect("csr present");
        let der = der.as_ref();

        assert_der_contains(der, identity.device_id.as_bytes());
        assert_der_contains(der, cfg.workspace_id.as_bytes());
        assert_der_contains(
            der,
            format!("{}.{}", identity.device_id, cfg.workspace_id).as_bytes(),
        );
        assert_der_contains(der, cfg.dataplane_server_name().as_bytes());
        assert_der_contains(
            der,
            format!("ztna://{}/{}", cfg.workspace_id, identity.device_id).as_bytes(),
        );
    }

    fn assert_der_contains(der: &[u8], needle: &[u8]) {
        assert!(
            der.windows(needle.len()).any(|window| window == needle),
            "missing DER claim: {}",
            String::from_utf8_lossy(needle)
        );
    }
}
