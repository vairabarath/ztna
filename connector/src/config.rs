use crate::cli::Args;

#[derive(Debug, Clone)]
pub struct Config {
    pub workspace_id: String,
    pub bootstrap_token: String,
    pub controller_addr: String,
    pub controller_ca_cert: String,
    pub storage_dir: String,
    pub device_id: String,
    pub dataplane_listen_addr: String,
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
            dataplane_listen_addr: args.dataplane_listen_addr,
        }
    }

    pub fn dataplane_server_name(&self) -> String {
        format!("connector.{}", self.workspace_id)
    }
}
