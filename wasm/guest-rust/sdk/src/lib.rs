use serde::{Deserialize, Serialize};
use std::collections::HashMap;

// ── Types matching the host ABI ─────────────────────────────────────────────

#[derive(Serialize, Deserialize, Clone)]
pub struct ToolDefinition {
    pub name: String,
    pub description: String,
    pub parameters: HashMap<String, String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub required: Vec<String>,
}

#[derive(Serialize, Deserialize)]
pub struct ToolResult {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub data: String,
    #[serde(default, skip_serializing_if = "is_false")]
    pub is_error: bool,
}

fn is_false(v: &bool) -> bool {
    !v
}

#[derive(Deserialize)]
pub struct ExecuteRequest {
    pub tool_name: String,
    pub args: HashMap<String, serde_json::Value>,
}

#[derive(Serialize)]
pub struct HttpRequest {
    pub method: String,
    pub url: String,
    #[serde(skip_serializing_if = "HashMap::is_empty")]
    pub headers: HashMap<String, String>,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub body: String,
}

#[derive(Deserialize)]
pub struct HttpResponse {
    pub status: i32,
    #[serde(default)]
    pub headers: HashMap<String, String>,
    #[serde(default)]
    pub body: String,
}

// ── Host imports ────────────────────────────────────────────────────────────

extern "C" {
    #[link_name = "host_http_request"]
    fn host_http_request_raw(ptr_size: u64) -> u64;
    #[link_name = "host_log"]
    fn host_log_raw(ptr: u32, size: u32);
}

pub fn host_log(msg: &str) {
    unsafe {
        host_log_raw(msg.as_ptr() as u32, msg.len() as u32);
    }
}

pub fn host_http_request(req: &HttpRequest) -> Result<HttpResponse, String> {
    let req_json = serde_json::to_vec(req).map_err(|e| e.to_string())?;
    let ptr_size = pack_ptr_size(req_json.as_ptr() as u32, req_json.len() as u32);
    let result = unsafe { host_http_request_raw(ptr_size) };
    let (r_ptr, r_size) = unpack_ptr_size(result);
    if r_size == 0 {
        return Err("empty response from host".into());
    }
    let resp_data = unsafe { read_bytes(r_ptr, r_size) };
    serde_json::from_slice(&resp_data).map_err(|e| e.to_string())
}

// ── Result helpers ──────────────────────────────────────────────────────────

pub fn err_result(msg: &str) -> ToolResult {
    ToolResult {
        data: msg.to_string(),
        is_error: true,
    }
}

pub fn raw_result(data: String) -> ToolResult {
    ToolResult {
        data,
        is_error: false,
    }
}

// ── Arg extraction ──────────────────────────────────────────────────────────

pub fn arg_str(args: &HashMap<String, serde_json::Value>, key: &str) -> String {
    args.get(key)
        .and_then(|v| v.as_str())
        .unwrap_or("")
        .to_string()
}

pub fn arg_str_slice(args: &HashMap<String, serde_json::Value>, key: &str) -> Vec<String> {
    match args.get(key) {
        Some(serde_json::Value::Array(arr)) => arr
            .iter()
            .filter_map(|v| v.as_str().map(String::from))
            .collect(),
        Some(serde_json::Value::String(s)) => {
            serde_json::from_str::<Vec<String>>(s).unwrap_or_default()
        }
        _ => Vec::new(),
    }
}

pub fn arg_int(args: &HashMap<String, serde_json::Value>, key: &str) -> Option<i64> {
    args.get(key).and_then(|v| v.as_i64())
}

pub fn arg_bool(args: &HashMap<String, serde_json::Value>, key: &str) -> Option<bool> {
    args.get(key).and_then(|v| v.as_bool())
}

pub fn arg_map(
    args: &HashMap<String, serde_json::Value>,
    key: &str,
) -> Option<HashMap<String, serde_json::Value>> {
    args.get(key).and_then(|v| {
        if let serde_json::Value::Object(m) = v {
            Some(m.iter().map(|(k, v)| (k.clone(), v.clone())).collect())
        } else {
            None
        }
    })
}

// ── Memory helpers ──────────────────────────────────────────────────────────

pub fn leaked_result(data: &[u8]) -> u64 {
    let boxed = data.to_vec().into_boxed_slice();
    let ptr = boxed.as_ptr() as u32;
    let size = boxed.len() as u32;
    std::mem::forget(boxed);
    pack_ptr_size(ptr, size)
}

pub fn leaked_string(s: &str) -> u64 {
    leaked_result(s.as_bytes())
}

pub fn read_input(ptr_size: u64) -> Vec<u8> {
    let (ptr, size) = unpack_ptr_size(ptr_size);
    unsafe { read_bytes(ptr, size) }
}

unsafe fn read_bytes(ptr: u32, size: u32) -> Vec<u8> {
    let slice = std::slice::from_raw_parts(ptr as *const u8, size as usize);
    slice.to_vec()
}

fn pack_ptr_size(ptr: u32, size: u32) -> u64 {
    ((ptr as u64) << 32) | (size as u64)
}

fn unpack_ptr_size(v: u64) -> (u32, u32) {
    ((v >> 32) as u32, v as u32)
}

// ── Allocator exports (called by host) ──────────────────────────────────────

extern "C" {
    fn malloc(size: usize) -> *mut u8;
    fn free(ptr: *mut u8);
}

#[no_mangle]
pub extern "C" fn guest_malloc(size: u32) -> u32 {
    unsafe { malloc(size as usize) as u32 }
}

#[no_mangle]
pub extern "C" fn guest_free(ptr: u32) {
    if ptr == 0 {
        return;
    }
    unsafe { free(ptr as *mut u8) }
}
