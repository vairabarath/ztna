use clap::Parser;

#[derive(Debug, Clone, Parser)]
#[command(name = "ztna-connector")]
pub struct Args {
    #[arg(long)]
    pub workspace_id: String,

    #[arg(long)]
    pub bootstrap_token: String,

    #[arg(long)]
    pub controller_addr: String,

    #[arg(long, default_value = "")]
    pub controller_ca_cert: String,

    #[arg(long, default_value = "/var/lib/ztna/connector")]
    pub storage_dir: String,

    #[arg(long, default_value = "")]
    pub device_id: String,

    #[arg(long, default_value = "0.0.0.0:9443")]
    pub dataplane_listen_addr: String,
}
