mod tools;

use serde_json;
use std::collections::HashMap;
use std::sync::Mutex;
use switchboard_guest_sdk as sdk;

static CONFIG: Mutex<Option<Config>> = Mutex::new(None);

struct Config {
    base_url: String,
    api_key: String,
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

fn api_key_str() -> String {
    with_config(|c| c.api_key.clone())
}

#[no_mangle]
pub extern "C" fn name() -> u64 {
    sdk::leaked_string("example")
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

    let bu = creds
        .get("base_url")
        .map(|s| s.trim_end_matches('/').to_string())
        .unwrap_or_default();
    if bu.is_empty() {
        return sdk::leaked_string("example: base_url is required");
    }
    let ak = creds.get("api_key").cloned().unwrap_or_default();
    if ak.is_empty() {
        return sdk::leaked_string("example: api_key is required");
    }

    *CONFIG.lock().unwrap() = Some(Config {
        base_url: bu,
        api_key: ak,
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
    match do_get("/health") {
        Ok(_) => 1,
        Err(_) => 0,
    }
}

#[no_mangle]
pub extern "C" fn metadata() -> u64 {
    sdk::leaked_metadata(&sdk::PluginMetadata {
        name: "example".into(),
        version: "0.1.0".into(),
        abi_version: 1,
        description: "Example plugin demonstrating the Switchboard WASM plugin SDK".into(),
        author: "Switchboard".into(),
        homepage: "https://github.com/daltoniam/switchboard".into(),
        license: "MIT".into(),
        capabilities: vec!["http".into()],
    })
}

type HandlerFn = fn(HashMap<String, serde_json::Value>) -> sdk::ToolResult;

fn dispatch(tool_name: &str, args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let handler: Option<HandlerFn> = match tool_name {
        "example_echo" => Some(echo),
        "example_http_get" => Some(http_get),
        "example_list_items" => Some(list_items),
        _ => None,
    };

    match handler {
        Some(f) => f(args),
        None => sdk::err_result(&format!("unknown tool: {tool_name}")),
    }
}

fn echo(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let message = sdk::arg_str(&args, "message");
    if message.is_empty() {
        return sdk::err_result("message is required");
    }
    let resp = serde_json::json!({ "echo": message });
    sdk::raw_result(resp.to_string())
}

fn http_get(args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    let path = sdk::arg_str(&args, "path");
    if path.is_empty() {
        return sdk::err_result("path is required");
    }
    match do_get(&path) {
        Ok(data) => sdk::raw_result(data),
        Err(e) => sdk::err_result(&e),
    }
}

fn list_items(_args: HashMap<String, serde_json::Value>) -> sdk::ToolResult {
    match do_get("/items") {
        Ok(data) => sdk::raw_result(data),
        Err(e) => sdk::err_result(&e),
    }
}

fn do_get(path: &str) -> Result<String, String> {
    let mut headers = HashMap::new();
    headers.insert("Authorization".into(), format!("Bearer {}", api_key_str()));

    let req = sdk::HttpRequest {
        method: "GET".into(),
        url: format!("{}{}", base_url_str(), path),
        headers,
        body: String::new(),
    };

    let resp = sdk::host_http_request(&req)?;
    if resp.status >= 400 {
        return Err(format!("API error ({}): {}", resp.status, resp.body));
    }
    Ok(resp.body)
}
