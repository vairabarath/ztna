use anyhow::{anyhow, Result};
use std::convert::TryInto;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use crate::config::Config;
use crate::enrollment::EnrollmentMaterial;
use crate::grpc::{connect_channel, ClientMtlsIdentity};
use crate::identity::Identity;
use crate::pb::ztna::controlplane::v1::{
    enrollment_service_client::EnrollmentServiceClient, EnrollRequest, EnrollTokenType,
    RenewRequest,
};

pub async fn enroll(
    cfg: &Config,
    _ca_cert_pem: &str,
    identity: &Identity,
    csr_pem: &str,
) -> Result<EnrollmentMaterial> {
    let channel = connect_channel(cfg, None).await?;
    let mut client = EnrollmentServiceClient::new(channel);

    let resp = client
        .enroll(EnrollRequest {
            workspace_id: cfg.workspace_id.clone(),
            bootstrap_token: cfg.bootstrap_token.clone(),
            r#type: EnrollTokenType::Agent as i32,
            csr_pem: csr_pem.to_string(),
            device_id: identity.device_id.clone(),
            hostname: String::new(),
            metadata: Default::default(),
        })
        .await?
        .into_inner();

    Ok(EnrollmentMaterial {
        certificate_pem: resp.certificate_pem,
        ca_cert_pem: resp.ca_cert_pem,
        cert_fingerprint: resp.cert_fingerprint,
        expires_at: timestamp_to_system_time(resp.expires_at)?,
    })
}

pub async fn renew(
    cfg: &Config,
    identity: &Identity,
    csr_pem: &str,
    client_cert_pem: &str,
) -> Result<EnrollmentMaterial> {
    let channel = connect_channel(
        cfg,
        Some(ClientMtlsIdentity {
            cert_pem: client_cert_pem.to_string(),
            key_pem: identity.private_key_pem.clone(),
        }),
    )
    .await?;
    let mut client = EnrollmentServiceClient::new(channel);

    let resp = client
        .renew(RenewRequest {
            workspace_id: cfg.workspace_id.clone(),
            csr_pem: csr_pem.to_string(),
            device_id: identity.device_id.clone(),
        })
        .await?
        .into_inner();

    Ok(EnrollmentMaterial {
        certificate_pem: resp.certificate_pem,
        ca_cert_pem: resp.ca_cert_pem,
        cert_fingerprint: resp.cert_fingerprint,
        expires_at: timestamp_to_system_time(resp.expires_at)?,
    })
}

fn timestamp_to_system_time(ts: Option<prost_types::Timestamp>) -> Result<SystemTime> {
    let Some(ts) = ts else {
        return Ok(SystemTime::now() + Duration::from_secs(24 * 60 * 60));
    };
    if ts.seconds < 0 {
        return Err(anyhow!("expires_at timestamp is before unix epoch"));
    }

    let nanos_u32: u32 = ts
        .nanos
        .try_into()
        .map_err(|_| anyhow!("invalid nanos in expires_at timestamp"))?;
    if nanos_u32 >= 1_000_000_000 {
        return Err(anyhow!("nanos out of range in expires_at timestamp"));
    }

    let secs: u64 = ts
        .seconds
        .try_into()
        .map_err(|_| anyhow!("expires_at seconds out of range"))?;
    Ok(UNIX_EPOCH + Duration::new(secs, nanos_u32))
}
