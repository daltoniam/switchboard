use std::collections::HashMap;
use switchboard_guest_sdk::ToolDefinition;

pub fn tool_definitions() -> Vec<ToolDefinition> {
    vec![
        ToolDefinition {
            name: "example_echo".into(),
            description: "Echo back a message. Demonstrates basic arg parsing and JSON response.".into(),
            parameters: {
                let mut m = HashMap::new();
                m.insert("message".into(), "The message to echo back".into());
                m
            },
            required: vec!["message".into()],
        },
        ToolDefinition {
            name: "example_http_get".into(),
            description: "Fetch a URL path from the configured API. Demonstrates host HTTP requests with auth.".into(),
            parameters: {
                let mut m = HashMap::new();
                m.insert("path".into(), "The API path to GET (e.g. /users)".into());
                m
            },
            required: vec!["path".into()],
        },
        ToolDefinition {
            name: "example_list_items".into(),
            description: "List all items from the API. Demonstrates a zero-arg tool that calls GET /items.".into(),
            parameters: HashMap::new(),
            required: vec![],
        },
    ]
}
