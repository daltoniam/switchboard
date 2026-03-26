use std::collections::HashMap;
use switchboard_guest_sdk as sdk;

use crate::{agent_run_id_str, do_get, do_post, flow_run_id_str, url_encode};

pub fn list_available_agents(_args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    match do_get(&format!("/api/flow_runs/{}/available_agents", url_encode(&flow_run_id_str()))) {
        Ok(data) => sdk::raw_result(data),
        Err(e) => sdk::err_result(&e),
    }
}

pub fn launch_agent(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let agent_id = sdk::arg_str(&args, "agent_id");
    let prompt = sdk::arg_str(&args, "prompt");
    let context = sdk::arg_str(&args, "context");
    if agent_id.is_empty() {
        return sdk::err_result("agent_id is required");
    }
    if prompt.is_empty() {
        return sdk::err_result("prompt is required");
    }
    let mut body = serde_json::json!({
        "agent_id": agent_id,
        "prompt": prompt,
        "parent_run_id": agent_run_id_str(),
    });
    if !context.is_empty() {
        body["context"] = serde_json::Value::String(context);
    }
    match do_post(&format!("/api/flow_runs/{}/launch_agent", url_encode(&flow_run_id_str())), Some(body)) {
        Ok(data) => sdk::raw_result(data),
        Err(e) => sdk::err_result(&e),
    }
}

pub fn get_agent_status(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = sdk::arg_str(&args, "agent_run_id");
    if id.is_empty() {
        return sdk::err_result("agent_run_id is required");
    }
    match do_get(&format!("/api/agent_runs/{}/status", url_encode(&id))) {
        Ok(data) => sdk::raw_result(data),
        Err(e) => sdk::err_result(&e),
    }
}

pub fn get_agent_result(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let id = sdk::arg_str(&args, "agent_run_id");
    if id.is_empty() {
        return sdk::err_result("agent_run_id is required");
    }
    match do_get(&format!("/api/agent_runs/{}/result", url_encode(&id))) {
        Ok(data) => sdk::raw_result(data),
        Err(e) => sdk::err_result(&e),
    }
}

pub fn complete_flow(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let summary = sdk::arg_str(&args, "summary");
    let mut status = sdk::arg_str(&args, "status");
    if summary.is_empty() {
        return sdk::err_result("summary is required");
    }
    if status.is_empty() {
        status = "success".into();
    }
    if status != "success" && status != "failure" {
        return sdk::err_result(&format!("status must be 'success' or 'failure', got {:?}", status));
    }
    let body = serde_json::json!({
        "summary": summary,
        "status": status,
        "agent_run_id": agent_run_id_str(),
    });
    match do_post(&format!("/api/flow_runs/{}/complete", url_encode(&flow_run_id_str())), Some(body)) {
        Ok(data) => sdk::raw_result(data),
        Err(e) => sdk::err_result(&e),
    }
}
