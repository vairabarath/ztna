use std::fs;
use std::path::PathBuf;

use anyhow::{Context, Result};
use rcgen::{KeyPair, PKCS_ECDSA_P256_SHA256};

pub fn load_or_create_p256(storage_dir: &str) -> Result<String> {
    let path = PathBuf::from(storage_dir).join("device.key.pem");
    if path.exists() {
        let pem =
            fs::read_to_string(&path).with_context(|| format!("read key at {}", path.display()))?;
        return Ok(pem);
    }

    fs::create_dir_all(storage_dir)
        .with_context(|| format!("create storage dir {}", storage_dir))?;

    let key = KeyPair::generate_for(&PKCS_ECDSA_P256_SHA256)?;
    let pem = key.serialize_pem();

    fs::write(&path, &pem).with_context(|| format!("write key at {}", path.display()))?;
    Ok(pem)
}

#[cfg(test)]
mod tests {
    use super::load_or_create_p256;
    use rcgen::KeyPair;
    use std::fs;
    use std::path::PathBuf;
    use std::process;
    use std::time::{SystemTime, UNIX_EPOCH};

    #[test]
    fn persists_private_key_across_restarts() {
        let dir = temp_test_dir("keypair");

        let first = load_or_create_p256(dir.to_str().expect("utf8 path")).expect("first create");
        let second = load_or_create_p256(dir.to_str().expect("utf8 path")).expect("second load");

        assert_eq!(first, second);
        assert!(dir.join("device.key.pem").exists());
        KeyPair::from_pem(&second).expect("valid pem key");

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
