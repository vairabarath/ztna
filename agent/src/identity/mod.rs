mod device_id;
mod keypair;
mod keystore;

use anyhow::Result;

#[derive(Debug, Clone)]
pub struct Identity {
    pub device_id: String,
    pub private_key_pem: String,
}

pub fn ensure_identity(storage_dir: &str, configured_device_id: Option<&str>) -> Result<Identity> {
    let device_id = device_id::load_or_create(storage_dir, configured_device_id)?;
    let private_key_pem = keypair::load_or_create_p256(storage_dir)?;
    Ok(Identity {
        device_id,
        private_key_pem,
    })
}

#[cfg(test)]
mod tests {
    use super::ensure_identity;
    use std::fs;
    use std::path::PathBuf;
    use std::process;
    use std::time::{SystemTime, UNIX_EPOCH};

    #[test]
    fn ensure_identity_is_stable_on_subsequent_calls() {
        let dir = temp_test_dir("identity");
        let first =
            ensure_identity(dir.to_str().expect("utf8 path"), None).expect("first identity");
        let second =
            ensure_identity(dir.to_str().expect("utf8 path"), None).expect("second identity");

        assert_eq!(first.device_id, second.device_id);
        assert_eq!(first.private_key_pem, second.private_key_pem);

        let _ = fs::remove_dir_all(dir);
    }

    fn temp_test_dir(prefix: &str) -> PathBuf {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("clock")
            .as_nanos();
        std::env::temp_dir().join(format!("ztna-agent-{prefix}-{}-{}", process::id(), nanos))
    }
}
