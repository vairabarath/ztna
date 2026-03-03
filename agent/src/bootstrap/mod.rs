mod ca_fetch;

use anyhow::Result;

use crate::config::Config;

pub async fn fetch_workspace_ca(cfg: &Config) -> Result<String> {
    ca_fetch::fetch_workspace_ca(cfg).await
}
