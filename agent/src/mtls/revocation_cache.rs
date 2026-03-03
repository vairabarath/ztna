use std::collections::HashSet;
use std::fs;
use std::path::PathBuf;
use std::sync::RwLock;

use anyhow::{Context, Result};

#[derive(Debug, Default)]
pub struct RevocationCache {
    fingerprints: RwLock<HashSet<String>>,
}

impl RevocationCache {
    pub fn load_from_file(path: PathBuf) -> Result<Self> {
        let cache = Self::default();
        if !path.exists() {
            return Ok(cache);
        }

        let content = fs::read_to_string(&path)
            .with_context(|| format!("read revocation file {}", path.display()))?;
        for line in content.lines() {
            cache.mark_revoked(line);
        }
        Ok(cache)
    }

    pub fn mark_revoked(&self, fingerprint: &str) {
        self.insert(fingerprint);
    }

    pub fn is_revoked(&self, fingerprint: &str) -> bool {
        let normalized = normalize_fingerprint(fingerprint);
        if normalized.is_empty() {
            return false;
        }
        self.fingerprints
            .read()
            .map(|set| set.contains(&normalized))
            .unwrap_or(false)
    }

    fn insert(&self, fingerprint: &str) {
        let normalized = normalize_fingerprint(fingerprint);
        if normalized.is_empty() {
            return;
        }
        if let Ok(mut set) = self.fingerprints.write() {
            set.insert(normalized);
        }
    }
}

fn normalize_fingerprint(input: &str) -> String {
    input.trim().to_ascii_lowercase()
}

#[cfg(test)]
mod tests {
    use super::RevocationCache;

    #[test]
    fn marks_and_detects_fingerprints_case_insensitively() {
        let cache = RevocationCache::default();
        cache.mark_revoked("ABCD1234");
        assert!(cache.is_revoked("abcd1234"));
        assert!(cache.is_revoked("ABCD1234"));
    }

    #[test]
    fn ignores_empty_values() {
        let cache = RevocationCache::default();
        cache.mark_revoked("   ");
        assert!(!cache.is_revoked(""));
    }
}
