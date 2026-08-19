use switchboard_awm::{proto::ListProjectsRequest, AwmClients};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let mut clients = match std::env::args().nth(1) {
        Some(path) if path.starts_with('/') || path.starts_with("unix:") => {
            let path = path.strip_prefix("unix:").unwrap_or(&path);
            AwmClients::connect_unix(path).await?
        }
        Some(target) => AwmClients::connect_loopback(target).await?,
        None => AwmClients::connect_loopback("").await?,
    };
    let response = clients
        .projects
        .list_projects(ListProjectsRequest::default())
        .await?
        .into_inner();
    for project in response.projects {
        println!(
            "{} rev={} known via get",
            project.project_id, project.revision
        );
    }
    Ok(())
}
