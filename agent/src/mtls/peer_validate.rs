use std::fmt;
use std::io::Cursor;

use ring::digest::{digest, SHA256};

use crate::mtls::RevocationCache;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ValidatedPeerIdentity {
    pub workspace_id: String,
    pub device_id: String,
    pub cert_fingerprint: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum PeerValidationError {
    InvalidInput(&'static str),
    InvalidCertificateEncoding,
    SANExtensionNotFound,
    SANWorkspaceMismatch,
    SANDeviceMismatch,
    RevokedCertificate,
}

impl fmt::Display for PeerValidationError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::InvalidInput(msg) => write!(f, "{msg}"),
            Self::InvalidCertificateEncoding => write!(f, "invalid peer certificate encoding"),
            Self::SANExtensionNotFound => write!(f, "peer certificate is missing SAN extension"),
            Self::SANWorkspaceMismatch => {
                write!(f, "peer SAN workspace does not match local workspace")
            }
            Self::SANDeviceMismatch => write!(f, "peer SAN device identity is inconsistent"),
            Self::RevokedCertificate => write!(f, "peer certificate has been revoked"),
        }
    }
}

impl std::error::Error for PeerValidationError {}

pub fn authorize_peer_certificate(
    expected_workspace_id: &str,
    peer_cert_pem: &str,
    revocation_cache: &RevocationCache,
) -> Result<ValidatedPeerIdentity, PeerValidationError> {
    if expected_workspace_id.trim().is_empty() {
        return Err(PeerValidationError::InvalidInput(
            "expected workspace id is required",
        ));
    }
    if peer_cert_pem.trim().is_empty() {
        return Err(PeerValidationError::InvalidInput(
            "peer certificate pem is required",
        ));
    }

    let cert_der = first_cert_der(peer_cert_pem)?;
    let san = extract_san_claims(&cert_der)?;
    let claimed_device_id = select_device_for_workspace(expected_workspace_id, &san)?;
    let fingerprint = sha256_fingerprint_hex(&cert_der);

    if revocation_cache.is_revoked(&fingerprint) {
        return Err(PeerValidationError::RevokedCertificate);
    }

    Ok(ValidatedPeerIdentity {
        workspace_id: expected_workspace_id.to_string(),
        device_id: claimed_device_id,
        cert_fingerprint: fingerprint,
    })
}

#[derive(Debug, Default)]
struct SANClaims {
    dns_names: Vec<String>,
    uris: Vec<String>,
}

fn first_cert_der(cert_pem: &str) -> Result<Vec<u8>, PeerValidationError> {
    let mut cursor = Cursor::new(cert_pem.as_bytes());
    let mut certs = rustls_pemfile::certs(&mut cursor);
    let first = certs
        .next()
        .ok_or(PeerValidationError::InvalidCertificateEncoding)?;
    first
        .map(|cert| cert.as_ref().to_vec())
        .map_err(|_| PeerValidationError::InvalidCertificateEncoding)
}

fn sha256_fingerprint_hex(cert_der: &[u8]) -> String {
    let digest = digest(&SHA256, cert_der);
    let mut out = String::with_capacity(digest.as_ref().len() * 2);
    for b in digest.as_ref() {
        use std::fmt::Write as _;
        let _ = write!(&mut out, "{:02x}", b);
    }
    out
}

fn select_device_for_workspace(
    expected_workspace_id: &str,
    san: &SANClaims,
) -> Result<String, PeerValidationError> {
    let mut candidate_device_ids = Vec::new();
    for uri in &san.uris {
        if let Some((workspace, device)) = parse_ztna_uri(uri) {
            if workspace == expected_workspace_id {
                candidate_device_ids.push(device.to_string());
            }
        }
    }

    if candidate_device_ids.is_empty() {
        return Err(PeerValidationError::SANWorkspaceMismatch);
    }

    for device_id in candidate_device_ids {
        let expected_dns = format!("{device_id}.{expected_workspace_id}");
        if san.dns_names.iter().any(|dns| dns == &expected_dns) {
            return Ok(device_id);
        }
    }

    Err(PeerValidationError::SANDeviceMismatch)
}

fn parse_ztna_uri(uri: &str) -> Option<(&str, &str)> {
    let trimmed = uri.strip_prefix("ztna://")?;
    let (workspace, device) = trimmed.split_once('/')?;
    if workspace.is_empty() || device.is_empty() {
        return None;
    }
    Some((workspace, device))
}

fn extract_san_claims(cert_der: &[u8]) -> Result<SANClaims, PeerValidationError> {
    let tbs = parse_tbs_certificate(cert_der)?;
    let extensions = find_extensions(tbs)?;
    let san_der = find_san_extension(extensions)?;
    parse_general_names(san_der)
}

fn parse_tbs_certificate(cert_der: &[u8]) -> Result<&[u8], PeerValidationError> {
    let (cert, tail) = read_tlv(cert_der).ok_or(PeerValidationError::InvalidCertificateEncoding)?;
    if cert.tag != 0x30 || !tail.is_empty() {
        return Err(PeerValidationError::InvalidCertificateEncoding);
    }
    let (tbs, _) = read_tlv(cert.value).ok_or(PeerValidationError::InvalidCertificateEncoding)?;
    if tbs.tag != 0x30 {
        return Err(PeerValidationError::InvalidCertificateEncoding);
    }
    Ok(tbs.value)
}

fn find_extensions(tbs_der: &[u8]) -> Result<&[u8], PeerValidationError> {
    let mut rest = tbs_der;
    while !rest.is_empty() {
        let (tlv, tail) = read_tlv(rest).ok_or(PeerValidationError::InvalidCertificateEncoding)?;
        if tlv.tag == 0xA3 {
            let (seq, seq_tail) =
                read_tlv(tlv.value).ok_or(PeerValidationError::InvalidCertificateEncoding)?;
            if seq.tag != 0x30 || !seq_tail.is_empty() {
                return Err(PeerValidationError::InvalidCertificateEncoding);
            }
            return Ok(seq.value);
        }
        rest = tail;
    }
    Err(PeerValidationError::SANExtensionNotFound)
}

fn find_san_extension(extensions_der: &[u8]) -> Result<&[u8], PeerValidationError> {
    const SAN_OID_DER_VALUE: &[u8] = &[0x55, 0x1D, 0x11];

    let mut rest = extensions_der;
    while !rest.is_empty() {
        let (ext_seq, tail) =
            read_tlv(rest).ok_or(PeerValidationError::InvalidCertificateEncoding)?;
        if ext_seq.tag != 0x30 {
            return Err(PeerValidationError::InvalidCertificateEncoding);
        }

        let (oid, mut ext_rest) =
            read_tlv(ext_seq.value).ok_or(PeerValidationError::InvalidCertificateEncoding)?;
        if oid.tag != 0x06 {
            return Err(PeerValidationError::InvalidCertificateEncoding);
        }

        if !ext_rest.is_empty() {
            let maybe_critical =
                read_tlv(ext_rest).ok_or(PeerValidationError::InvalidCertificateEncoding)?;
            if maybe_critical.0.tag == 0x01 {
                ext_rest = maybe_critical.1;
            }
        }

        let (octets, _) =
            read_tlv(ext_rest).ok_or(PeerValidationError::InvalidCertificateEncoding)?;
        if octets.tag != 0x04 {
            return Err(PeerValidationError::InvalidCertificateEncoding);
        }

        if oid.value == SAN_OID_DER_VALUE {
            return Ok(octets.value);
        }
        rest = tail;
    }

    Err(PeerValidationError::SANExtensionNotFound)
}

fn parse_general_names(san_der: &[u8]) -> Result<SANClaims, PeerValidationError> {
    let (seq, tail) = read_tlv(san_der).ok_or(PeerValidationError::InvalidCertificateEncoding)?;
    if seq.tag != 0x30 || !tail.is_empty() {
        return Err(PeerValidationError::InvalidCertificateEncoding);
    }

    let mut claims = SANClaims::default();
    let mut rest = seq.value;
    while !rest.is_empty() {
        let (name, tail) = read_tlv(rest).ok_or(PeerValidationError::InvalidCertificateEncoding)?;
        match name.tag {
            0x82 => claims
                .dns_names
                .push(String::from_utf8_lossy(name.value).to_string()),
            0x86 => claims
                .uris
                .push(String::from_utf8_lossy(name.value).to_string()),
            _ => {}
        }
        rest = tail;
    }

    if claims.dns_names.is_empty() || claims.uris.is_empty() {
        return Err(PeerValidationError::SANExtensionNotFound);
    }
    Ok(claims)
}

#[derive(Debug, Clone, Copy)]
struct DerTlv<'a> {
    tag: u8,
    value: &'a [u8],
}

fn read_tlv(input: &[u8]) -> Option<(DerTlv<'_>, &[u8])> {
    if input.len() < 2 {
        return None;
    }
    let tag = input[0];
    let (len, len_len) = parse_der_length(&input[1..])?;
    if input.len() < 1 + len_len + len {
        return None;
    }
    let value_start = 1 + len_len;
    let value_end = value_start + len;
    Some((
        DerTlv {
            tag,
            value: &input[value_start..value_end],
        },
        &input[value_end..],
    ))
}

fn parse_der_length(input: &[u8]) -> Option<(usize, usize)> {
    if input.is_empty() {
        return None;
    }
    let first = input[0];
    if first & 0x80 == 0 {
        return Some((first as usize, 1));
    }

    let num_bytes = (first & 0x7f) as usize;
    if num_bytes == 0 || num_bytes > 4 || input.len() < 1 + num_bytes {
        return None;
    }

    let mut len = 0usize;
    for b in &input[1..=num_bytes] {
        len = (len << 8) | (*b as usize);
    }
    Some((len, 1 + num_bytes))
}

#[cfg(test)]
mod tests {
    use super::{authorize_peer_certificate, PeerValidationError};
    use crate::mtls::RevocationCache;
    use rcgen::{CertificateParams, DistinguishedName, DnType, KeyPair, SanType};

    fn mint_peer_cert(workspace: &str, device: &str) -> String {
        let mut params = CertificateParams::new(vec![format!("{device}.{workspace}")]).unwrap();
        let mut dn = DistinguishedName::new();
        dn.push(DnType::CommonName, device);
        dn.push(DnType::OrganizationName, workspace);
        params.distinguished_name = dn;
        let uri = format!("ztna://{workspace}/{device}").parse().unwrap();
        params.subject_alt_names.push(SanType::URI(uri));
        let key = KeyPair::generate_for(&rcgen::PKCS_ECDSA_P256_SHA256).unwrap();
        params.self_signed(&key).unwrap().pem()
    }

    #[test]
    fn accepts_matching_workspace_san_identity() {
        let cert = mint_peer_cert("demo", "dev-1");
        let cache = RevocationCache::default();
        let identity = authorize_peer_certificate("demo", &cert, &cache).unwrap();
        assert_eq!(identity.workspace_id, "demo");
        assert_eq!(identity.device_id, "dev-1");
    }

    #[test]
    fn rejects_workspace_mismatch() {
        let cert = mint_peer_cert("other", "dev-1");
        let cache = RevocationCache::default();
        let err = authorize_peer_certificate("demo", &cert, &cache).unwrap_err();
        assert_eq!(err, PeerValidationError::SANWorkspaceMismatch);
    }

    #[test]
    fn rejects_revoked_fingerprint() {
        let cert = mint_peer_cert("demo", "dev-1");
        let cache = RevocationCache::default();
        let identity = authorize_peer_certificate("demo", &cert, &cache).unwrap();
        cache.mark_revoked(&identity.cert_fingerprint);

        let err = authorize_peer_certificate("demo", &cert, &cache).unwrap_err();
        assert_eq!(err, PeerValidationError::RevokedCertificate);
    }
}
