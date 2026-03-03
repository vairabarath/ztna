use clap::Parser;

#[derive(Debug, Clone, Parser)]
#[command(name = "ztna-agent")]
pub struct Args {
    #[arg(long)]
    pub workspace_id: String,

    #[arg(long)]
    pub bootstrap_token: String,

    #[arg(long)]
    pub controller_addr: String,

    #[arg(long, default_value = "")]
    pub controller_ca_cert: String,

    #[arg(long, default_value = "/var/lib/ztna/agent")]
    pub storage_dir: String,

    #[arg(long, default_value = "")]
    pub device_id: String,

    #[arg(long, default_value = "")]
    pub connector_addr: String,

    #[arg(long, default_value = "")]
    pub connector_server_name: String,

    #[arg(long, default_value_t = 15)]
    pub dataplane_ping_interval_secs: u64,
}
