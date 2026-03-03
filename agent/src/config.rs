use crate::cli::Args;

#[derive(Debug, Clone)]
pub struct Config {
    pub workspace_id: String,
    pub bootstrap_token: String,
    pub controller_addr: String,
    pub controller_ca_cert: String,
    pub storage_dir: String,
    pub device_id: String,
    pub connector_addr: String,
    pub connector_server_name: String,
    pub dataplane_ping_interval_secs: u64,
}

impl Config {
    pub fn from_args(args: Args) -> Self {
        Self {
            workspace_id: args.workspace_id,
            bootstrap_token: args.bootstrap_token,
            controller_addr: args.controller_addr,
            controller_ca_cert: args.controller_ca_cert,
            storage_dir: args.storage_dir,
            device_id: args.device_id,
            connector_addr: args.connector_addr,
            connector_server_name: args.connector_server_name,
            dataplane_ping_interval_secs: args.dataplane_ping_interval_secs,
        }
    }

    pub fn effective_connector_server_name(&self) -> String {
        if self.connector_server_name.trim().is_empty() {
            return format!("connector.{}", self.workspace_id);
        }
        self.connector_server_name.trim().to_string()
    }
}
