use std::fs;
use std::path::PathBuf;

use anyhow::{bail, Context, Result};
use uuid::Uuid;

pub fn load_or_create(storage_dir: &str, configured_device_id: Option<&str>) -> Result<String> {
    let path = PathBuf::from(storage_dir).join("device_id");
    if path.exists() {
        let value = fs::read_to_string(&path)
            .with_context(|| format!("read device_id at {}", path.display()))?;
        let existing = value.trim().to_string();
        if let Some(configured) = normalize(configured_device_id) {
            if existing != configured {
                bail!(
                    "device_id mismatch in {}: existing={}, configured={}",
                    path.display(),
                    existing,
                    configured
                );
            }
        }
        return Ok(existing);
    }

    fs::create_dir_all(storage_dir)
        .with_context(|| format!("create storage dir {}", storage_dir))?;

    let id = normalize(configured_device_id).unwrap_or_else(|| Uuid::new_v4().to_string());
    fs::write(&path, format!("{}\n", id))
        .with_context(|| format!("write device_id at {}", path.display()))?;
    Ok(id)
}

fn normalize(device_id: Option<&str>) -> Option<String> {
    let value = device_id?.trim();
    if value.is_empty() {
        return None;
    }
    Some(value.to_string())
}

#[cfg(test)]
mod tests {
    use super::load_or_create;
    use std::fs;
    use std::path::PathBuf;
    use std::process;
    use std::time::{SystemTime, UNIX_EPOCH};

    #[test]
    fn persists_device_id_across_restarts() {
        let dir = temp_test_dir("device-id");

        let first = load_or_create(dir.to_str().expect("utf8 path"), None).expect("first create");
        let second = load_or_create(dir.to_str().expect("utf8 path"), None).expect("second load");

        assert_eq!(first, second);
        assert!(dir.join("device_id").exists());

        let _ = fs::remove_dir_all(dir);
    }

    #[test]
    fn uses_configured_device_id_when_file_is_missing() {
        let dir = temp_test_dir("configured-device");

        let device_id = load_or_create(dir.to_str().expect("utf8 path"), Some("agent-fixed"))
            .expect("create configured");

        assert_eq!(device_id, "agent-fixed");
        assert_eq!(
            fs::read_to_string(dir.join("device_id"))
                .expect("device_id file")
                .trim(),
            "agent-fixed"
        );

        let _ = fs::remove_dir_all(dir);
    }

    #[test]
    fn rejects_configured_device_id_that_differs_from_existing_file() {
        let dir = temp_test_dir("mismatch");
        let path = dir.join("device_id");
        fs::create_dir_all(&dir).expect("create dir");
        fs::write(&path, "existing-id\n").expect("write device id");

        let err = load_or_create(dir.to_str().expect("utf8 path"), Some("different-id"))
            .expect_err("mismatch must fail");
        let msg = err.to_string();
        assert!(msg.contains("device_id mismatch"), "{msg}");

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
