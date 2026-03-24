mod handlers;
mod handlers_admin;
mod tools;

use serde_json;
use std::collections::HashMap;
use std::sync::Mutex;
use switchboard_guest_sdk as sdk;

static CONFIG: Mutex<Option<Config>> = Mutex::new(None);

struct Config {
    base_url: String,
    token: String,
    agent_run_id: String,
    flow_run_id: String,
}

fn with_config<F, R>(f: F) -> R
where
    F: FnOnce(&Config) -> R,
{
    let guard = CONFIG.lock().unwrap();
    f(guard.as_ref().expect("not configured"))
}

fn base_url_str() -> String {
    with_config(|c| c.base_url.clone())
}
fn token_str() -> String {
    with_config(|c| c.token.clone())
}
fn agent_run_id_str() -> String {
    with_config(|c| c.agent_run_id.clone())
}
fn flow_run_id_str() -> String {
    with_config(|c| c.flow_run_id.clone())
}

#[no_mangle]
pub extern "C" fn name() -> u64 {
    sdk::leaked_string("overmind")
}

#[no_mangle]
pub extern "C" fn tools() -> u64 {
    let defs = tools::tool_definitions();
    let data = serde_json::to_vec(&defs).unwrap_or_default();
    sdk::leaked_result(&data)
}

#[no_mangle]
pub extern "C" fn configure(ptr_size: u64) -> u64 {
    let input = sdk::read_input(ptr_size);
    let creds: HashMap<String, String> = match serde_json::from_slice(&input) {
        Ok(c) => c,
        Err(e) => return sdk::leaked_string(&format!("invalid credentials JSON: {e}")),
    };

    let bu = creds.get("base_url").map(|s| s.trim_end_matches('/').to_string()).unwrap_or_default();
    if bu.is_empty() {
        return sdk::leaked_string("overmind: base_url is required");
    }
    let tk = creds.get("token").cloned().unwrap_or_default();
    if tk.is_empty() {
        return sdk::leaked_string("overmind: token is required");
    }
    let ar = creds.get("agent_run_id").cloned().unwrap_or_default();
    if ar.is_empty() {
        return sdk::leaked_string("overmind: agent_run_id is required");
    }
    let fr = creds.get("flow_run_id").cloned().unwrap_or_default();
    if fr.is_empty() {
        return sdk::leaked_string("overmind: flow_run_id is required");
    }

    *CONFIG.lock().unwrap() = Some(Config {
        base_url: bu,
        token: tk,
        agent_run_id: ar,
        flow_run_id: fr,
    });
    0
}

#[no_mangle]
pub extern "C" fn execute(ptr_size: u64) -> u64 {
    let input = sdk::read_input(ptr_size);
    let req: sdk::ExecuteRequest = match serde_json::from_slice(&input) {
        Ok(r) => r,
        Err(e) => {
            let r = sdk::err_result(&format!("invalid request: {e}"));
            let data = serde_json::to_vec(&r).unwrap_or_default();
            return sdk::leaked_result(&data);
        }
    };

    let result = dispatch(&req.tool_name, req.args);
    let data = serde_json::to_vec(&result).unwrap_or_default();
    sdk::leaked_result(&data)
}

#[no_mangle]
pub extern "C" fn healthy() -> i32 {
    match do_get("/api/health") {
        Ok(_) => 1,
        Err(_) => 0,
    }
}

type HandlerFn = fn(HashMap<String, serde_json::Value>) -> sdk::ToolResult;

fn dispatch(tool_name: &str, args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let handler: Option<HandlerFn> = match tool_name {
        "overmind_list_available_agents" => Some(handlers::list_available_agents),
        "overmind_launch_agent" => Some(handlers::launch_agent),
        "overmind_get_agent_status" => Some(handlers::get_agent_status),
        "overmind_get_agent_result" => Some(handlers::get_agent_result),
        "overmind_complete_flow" => Some(handlers::complete_flow),

        "overmind_list_agents" => Some(handlers_admin::list_agents),
        "overmind_get_agent" => Some(handlers_admin::get_agent),
        "overmind_create_agent" => Some(handlers_admin::create_agent),
        "overmind_update_agent" => Some(handlers_admin::update_agent),
        "overmind_delete_agent" => Some(handlers_admin::delete_agent),

        "overmind_list_flows" => Some(handlers_admin::list_flows),
        "overmind_get_flow" => Some(handlers_admin::get_flow),
        "overmind_create_flow" => Some(handlers_admin::create_flow),
        "overmind_update_flow" => Some(handlers_admin::update_flow),
        "overmind_delete_flow" => Some(handlers_admin::delete_flow),
        "overmind_clone_flow" => Some(handlers_admin::clone_flow),
        "overmind_run_flow" => Some(handlers_admin::run_flow),
        "overmind_validate_flow" => Some(handlers_admin::validate_flow),

        "overmind_list_flow_runs" => Some(handlers_admin::list_flow_runs),
        "overmind_get_flow_run" => Some(handlers_admin::get_flow_run),
        "overmind_cancel_flow_run" => Some(handlers_admin::cancel_flow_run),

        "overmind_list_agent_runs" => Some(handlers_admin::list_agent_runs),
        "overmind_get_agent_run" => Some(handlers_admin::get_agent_run),

        "overmind_list_mcp_identities" => Some(handlers_admin::list_mcp_identities),
        "overmind_get_mcp_identity" => Some(handlers_admin::get_mcp_identity),
        "overmind_create_mcp_identity" => Some(handlers_admin::create_mcp_identity),
        "overmind_update_mcp_identity" => Some(handlers_admin::update_mcp_identity),
        "overmind_delete_mcp_identity" => Some(handlers_admin::delete_mcp_identity),

        "overmind_list_mcp_roles" => Some(handlers_admin::list_mcp_roles),
        "overmind_get_mcp_role" => Some(handlers_admin::get_mcp_role),
        "overmind_create_mcp_role" => Some(handlers_admin::create_mcp_role),
        "overmind_update_mcp_role" => Some(handlers_admin::update_mcp_role),
        "overmind_delete_mcp_role" => Some(handlers_admin::delete_mcp_role),
        "overmind_create_mcp_role_entry" => Some(handlers_admin::create_mcp_role_entry),
        "overmind_update_mcp_role_entry" => Some(handlers_admin::update_mcp_role_entry),
        "overmind_delete_mcp_role_entry" => Some(handlers_admin::delete_mcp_role_entry),

        "overmind_list_pipelines" => Some(handlers_admin::list_pipelines),
        "overmind_get_pipeline" => Some(handlers_admin::get_pipeline),
        "overmind_create_pipeline" => Some(handlers_admin::create_pipeline),
        "overmind_update_pipeline" => Some(handlers_admin::update_pipeline),
        "overmind_delete_pipeline" => Some(handlers_admin::delete_pipeline),

        "overmind_list_tasks" => Some(handlers_admin::list_tasks),
        "overmind_get_task" => Some(handlers_admin::get_task),
        "overmind_create_task" => Some(handlers_admin::create_task),
        "overmind_update_task" => Some(handlers_admin::update_task),
        "overmind_delete_task" => Some(handlers_admin::delete_task),

        _ => None,
    };

    match handler {
        Some(f) => f(args),
        None => sdk::err_result(&format!("unknown tool: {tool_name}")),
    }
}

// ── HTTP helpers ────────────────────────────────────────────────────────────

fn do_request(method: &str, path: &str, body: Option<serde_json::Value>) -> Result<String, String> {
    let mut headers = HashMap::new();
    headers.insert("Authorization".into(), format!("Bearer {}", token_str()));

    let body_str = match body {
        Some(v) => {
            headers.insert("Content-Type".into(), "application/json".into());
            serde_json::to_string(&v).map_err(|e| e.to_string())?
        }
        None => String::new(),
    };

    let req = sdk::HttpRequest {
        method: method.into(),
        url: format!("{}{}", base_url_str(), path),
        headers,
        body: body_str,
    };

    let resp = sdk::host_http_request(&req)?;
    if resp.status >= 400 {
        return Err(format!("overmind API error ({}): {}", resp.status, resp.body));
    }
    if resp.status == 204 || resp.body.is_empty() {
        return Ok(r#"{"status":"success"}"#.into());
    }
    Ok(resp.body)
}

pub fn do_get(path: &str) -> Result<String, String> {
    do_request("GET", path, None)
}

pub fn do_get_fmt(path: &str) -> Result<String, String> {
    do_request("GET", path, None)
}

pub fn do_post(path: &str, body: Option<serde_json::Value>) -> Result<String, String> {
    do_request("POST", path, body)
}

pub fn do_put(path: &str, body: Option<serde_json::Value>) -> Result<String, String> {
    do_request("PUT", path, body)
}

pub fn do_del(path: &str) -> Result<String, String> {
    do_request("DELETE", path, None)
}

fn url_encode(s: &str) -> String {
    let mut result = String::with_capacity(s.len());
    for b in s.bytes() {
        match b {
            b'A'..=b'Z' | b'a'..=b'z' | b'0'..=b'9' | b'-' | b'_' | b'.' | b'~' => {
                result.push(b as char);
            }
            _ => {
                result.push_str(&format!("%{:02X}", b));
            }
        }
    }
    result
}

pub fn build_body(args: &HashMap<String, serde_json::Value>, keys: &[&str]) -> serde_json::Value {
    let mut body = serde_json::Map::new();
    for &k in keys {
        if let Some(v) = args.get(k) {
            if !v.is_null() && v.as_str().map_or(true, |s| !s.is_empty()) {
                body.insert(k.into(), v.clone());
            }
        }
    }
    serde_json::Value::Object(body)
}
