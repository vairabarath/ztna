use anyhow::Result;

use crate::config::Config;
use crate::grpc::connect_channel;
use crate::pb::ztna::controlplane::v1::{
    workspace_service_client::WorkspaceServiceClient, GetWorkspaceCaRequest,
};

pub async fn fetch_workspace_ca(cfg: &Config) -> Result<String> {
    let channel = connect_channel(cfg, None).await?;
    let mut client = WorkspaceServiceClient::new(channel);

    let resp = client
        .get_workspace_ca(GetWorkspaceCaRequest {
            workspace_id: cfg.workspace_id.clone(),
        })
        .await?
        .into_inner();

    Ok(resp.ca_cert_pem)
}
