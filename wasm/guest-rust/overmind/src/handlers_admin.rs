use std::collections::HashMap;
use switchboard_guest_sdk as sdk;

use crate::{build_body, do_del, do_get, do_post, do_put, url_encode};

fn build_flow_body(args: &HashMap<String, serde_json::Value>) -> serde_json::Value {
    let mut body = build_body(args, &[
        "name", "description", "prompt_template", "initial_agent_id",
        "repo_url", "repo_ref", "output_webhook_url", "output_webhook_template",
        "webhook_secret",
    ]);
    let obj = body.as_object_mut().unwrap();
    let agent_ids = sdk::arg_str_slice(args, "available_agent_ids");
    if !agent_ids.is_empty() {
        obj.insert("available_agent_ids".into(), serde_json::json!(agent_ids));
    }
    if let Some(v) = sdk::arg_int(args, "timeout_minutes") {
        if v > 0 { obj.insert("timeout_minutes".into(), serde_json::json!(v)); }
    }
    if let Some(v) = sdk::arg_bool(args, "enabled") {
        if args.contains_key("enabled") { obj.insert("enabled".into(), serde_json::json!(v)); }
    }
    body
}

macro_rules! require {
    ($args:expr, $key:expr) => {{
        let v = sdk::arg_str($args, $key);
        if v.is_empty() { return sdk::err_result(concat!($key, " is required")); }
        v
    }};
}

pub fn list_agents(_args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    match do_get("/api/agents") { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn get_agent(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    match do_get(&format!("/api/agents/{}", url_encode(&id))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn create_agent(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let _ = require!(&args, "name");
    let body = build_body(&args, &["name", "description", "model", "model_provider", "base_prompt", "mcp_role_id"]);
    match do_post("/api/agents", Some(body)) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn update_agent(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    let body = build_body(&args, &["name", "description", "model", "model_provider", "base_prompt", "mcp_role_id"]);
    match do_put(&format!("/api/agents/{}", url_encode(&id)), Some(body)) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn delete_agent(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    match do_del(&format!("/api/agents/{}", url_encode(&id))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}

pub fn list_flows(_args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    match do_get("/api/flows") { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn get_flow(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    let expand = sdk::arg_str(&args, "expand");
    let mut path = format!("/api/flows/{}", url_encode(&id));
    if !expand.is_empty() { path.push_str(&format!("?expand={}", url_encode(&expand))); }
    match do_get(&path) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn create_flow(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let _ = require!(&args, "name");
    let body = build_flow_body(&args);
    match do_post("/api/flows", Some(body)) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn update_flow(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    let body = build_flow_body(&args);
    match do_put(&format!("/api/flows/{}", url_encode(&id)), Some(body)) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn delete_flow(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    match do_del(&format!("/api/flows/{}", url_encode(&id))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn clone_flow(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    let name = sdk::arg_str(&args, "name");
    let mut body = serde_json::json!({});
    if !name.is_empty() { body["name"] = serde_json::Value::String(name); }
    match do_post(&format!("/api/flows/{}/clone", url_encode(&id)), Some(body)) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn run_flow(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    let prompt = sdk::arg_str(&args, "prompt");
    let params = sdk::arg_map(&args, "params");
    let mut body = serde_json::json!({});
    if !prompt.is_empty() { body["prompt"] = serde_json::Value::String(prompt); }
    if let Some(p) = params { body["params"] = serde_json::json!(p); }
    match do_post(&format!("/api/flows/{}/run", url_encode(&id)), Some(body)) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn validate_flow(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let body = build_flow_body(&args);
    match do_post("/api/flows/validate", Some(body)) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}

pub fn list_flow_runs(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let flow_id = require!(&args, "flow_id");
    match do_get(&format!("/api/flow_runs?flow_id={}", url_encode(&flow_id))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn get_flow_run(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    match do_get(&format!("/api/flow_runs/{}", url_encode(&id))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn cancel_flow_run(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    match do_post(&format!("/api/flow_runs/{}/cancel", url_encode(&id)), None) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}

pub fn list_agent_runs(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let frid = require!(&args, "flow_run_id");
    match do_get(&format!("/api/agent_runs?flow_run_id={}", url_encode(&frid))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn get_agent_run(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    match do_get(&format!("/api/agent_runs/{}", url_encode(&id))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}

pub fn list_mcp_identities(_args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    match do_get("/api/mcp_identities") { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn get_mcp_identity(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    match do_get(&format!("/api/mcp_identities/{}", url_encode(&id))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn create_mcp_identity(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let name = require!(&args, "name");
    let int_name = require!(&args, "integration_name");
    let creds_raw = require!(&args, "credentials");
    let creds: serde_json::Value = match serde_json::from_str(&creds_raw) {
        Ok(v) => v, Err(e) => return sdk::err_result(&format!("credentials must be valid JSON: {e}")),
    };
    let body = serde_json::json!({"name": name, "integration_name": int_name, "credentials": creds});
    match do_post("/api/mcp_identities", Some(body)) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn update_mcp_identity(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    let mut body = serde_json::Map::new();
    let name = sdk::arg_str(&args, "name");
    if !name.is_empty() { body.insert("name".into(), serde_json::json!(name)); }
    let int_name = sdk::arg_str(&args, "integration_name");
    if !int_name.is_empty() { body.insert("integration_name".into(), serde_json::json!(int_name)); }
    let creds_raw = sdk::arg_str(&args, "credentials");
    if !creds_raw.is_empty() {
        match serde_json::from_str::<serde_json::Value>(&creds_raw) {
            Ok(v) => { body.insert("credentials".into(), v); }
            Err(e) => return sdk::err_result(&format!("credentials must be valid JSON: {e}")),
        }
    }
    match do_put(&format!("/api/mcp_identities/{}", url_encode(&id)), Some(serde_json::Value::Object(body))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn delete_mcp_identity(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    match do_del(&format!("/api/mcp_identities/{}", url_encode(&id))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}

pub fn list_mcp_roles(_args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    match do_get("/api/mcp_roles") { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn get_mcp_role(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    match do_get(&format!("/api/mcp_roles/{}", url_encode(&id))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn create_mcp_role(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let _ = require!(&args, "name");
    let body = build_body(&args, &["name", "description"]);
    match do_post("/api/mcp_roles", Some(body)) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn update_mcp_role(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    let body = build_body(&args, &["name", "description"]);
    match do_put(&format!("/api/mcp_roles/{}", url_encode(&id)), Some(body)) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn delete_mcp_role(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    match do_del(&format!("/api/mcp_roles/{}", url_encode(&id))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn create_mcp_role_entry(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let role_id = require!(&args, "role_id");
    let identity_id = require!(&args, "mcp_identity_id");
    let globs = sdk::arg_str_slice(&args, "tool_globs");
    let mut body = serde_json::json!({"mcp_identity_id": identity_id});
    if !globs.is_empty() { body["tool_globs"] = serde_json::json!(globs); }
    match do_post(&format!("/api/mcp_roles/{}/entries", url_encode(&role_id)), Some(body)) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn update_mcp_role_entry(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let role_id = require!(&args, "role_id");
    let entry_id = require!(&args, "entry_id");
    let mut body = serde_json::Map::new();
    let iid = sdk::arg_str(&args, "mcp_identity_id");
    if !iid.is_empty() { body.insert("mcp_identity_id".into(), serde_json::json!(iid)); }
    let globs = sdk::arg_str_slice(&args, "tool_globs");
    if !globs.is_empty() { body.insert("tool_globs".into(), serde_json::json!(globs)); }
    match do_put(&format!("/api/mcp_roles/{}/entries/{}", url_encode(&role_id), url_encode(&entry_id)), Some(serde_json::Value::Object(body))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn delete_mcp_role_entry(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let role_id = require!(&args, "role_id");
    let entry_id = require!(&args, "entry_id");
    match do_del(&format!("/api/mcp_roles/{}/entries/{}", url_encode(&role_id), url_encode(&entry_id))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}

pub fn list_pipelines(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let gc_id = sdk::arg_str(&args, "global_context_id");
    let mut path = "/api/pipelines".to_string();
    if !gc_id.is_empty() { path.push_str(&format!("?global_context_id={}", url_encode(&gc_id))); }
    match do_get(&path) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn get_pipeline(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    match do_get(&format!("/api/pipelines/{}", url_encode(&id))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn create_pipeline(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let name = require!(&args, "name");
    let gc_id = require!(&args, "global_context_id");
    let mut body = serde_json::json!({"name": name, "global_context_id": gc_id});
    let ctx_raw = sdk::arg_str(&args, "context");
    if !ctx_raw.is_empty() {
        match serde_json::from_str::<serde_json::Value>(&ctx_raw) {
            Ok(v) => { body["context"] = v; }
            Err(e) => return sdk::err_result(&format!("context must be valid JSON: {e}")),
        }
    }
    match do_post("/api/pipelines", Some(body)) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn update_pipeline(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    let mut body = serde_json::Map::new();
    let name = sdk::arg_str(&args, "name");
    if !name.is_empty() { body.insert("name".into(), serde_json::json!(name)); }
    let ctx_raw = sdk::arg_str(&args, "context");
    if !ctx_raw.is_empty() {
        match serde_json::from_str::<serde_json::Value>(&ctx_raw) {
            Ok(v) => { body.insert("context".into(), v); }
            Err(e) => return sdk::err_result(&format!("context must be valid JSON: {e}")),
        }
    }
    match do_put(&format!("/api/pipelines/{}", url_encode(&id)), Some(serde_json::Value::Object(body))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn delete_pipeline(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    match do_del(&format!("/api/pipelines/{}", url_encode(&id))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}

pub fn list_tasks(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let pid = require!(&args, "pipeline_id");
    match do_get(&format!("/api/tasks?pipeline_id={}", url_encode(&pid))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn get_task(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    match do_get(&format!("/api/tasks/{}", url_encode(&id))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn create_task(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let name = require!(&args, "name");
    let pid = require!(&args, "pipeline_id");
    let mut body = serde_json::json!({"name": name, "pipeline_id": pid});
    let ctx_raw = sdk::arg_str(&args, "context");
    if !ctx_raw.is_empty() {
        match serde_json::from_str::<serde_json::Value>(&ctx_raw) {
            Ok(v) => { body["context"] = v; }
            Err(e) => return sdk::err_result(&format!("context must be valid JSON: {e}")),
        }
    }
    let deps = sdk::arg_str_slice(&args, "depends_on");
    if !deps.is_empty() { body["depends_on"] = serde_json::json!(deps); }
    match do_post("/api/tasks", Some(body)) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn update_task(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    let mut body = serde_json::Map::new();
    let name = sdk::arg_str(&args, "name");
    if !name.is_empty() { body.insert("name".into(), serde_json::json!(name)); }
    let ctx_raw = sdk::arg_str(&args, "context");
    if !ctx_raw.is_empty() {
        match serde_json::from_str::<serde_json::Value>(&ctx_raw) {
            Ok(v) => { body.insert("context".into(), v); }
            Err(e) => return sdk::err_result(&format!("context must be valid JSON: {e}")),
        }
    }
    let deps = sdk::arg_str_slice(&args, "depends_on");
    if !deps.is_empty() { body.insert("depends_on".into(), serde_json::json!(deps)); }
    match do_put(&format!("/api/tasks/{}", url_encode(&id)), Some(serde_json::Value::Object(body))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
pub fn delete_task(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = require!(&args, "id");
    match do_del(&format!("/api/tasks/{}", url_encode(&id))) { Ok(d) => sdk::raw_result(d), Err(e) => sdk::err_result(&e) }
}
