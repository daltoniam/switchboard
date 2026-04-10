package acp

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name:        "acp_list_agents",
		Description: "List available remote agents on ACP servers. Discover AI agents, bots, and autonomous workers before invoking them. Returns agent names, descriptions, and capabilities",
		Parameters: map[string]string{
			"server":         "Name of a pre-configured ACP server to query. Uses the first configured server if omitted",
			"server_url":     "URL of an ACP server to query directly (e.g. http://localhost:8199). Overrides server name lookup",
			"server_headers": "Optional JSON object of HTTP headers to send with the request (e.g. {\"Authorization\":\"Bearer sk-xxx\"})",
		},
	},
	{
		Name:        "acp_run_agent",
		Description: "Invoke a remote ACP agent with a message and get its response. Send a text prompt to an AI agent on a remote server. Use acp_list_agents first to discover available agents. If the agent enters an awaiting state (needs more input), the response includes a run_id — use acp_resume_run to continue",
		Parameters: map[string]string{
			"agent_name":     "Name of the remote agent to invoke",
			"input":          "Text message to send to the agent",
			"server":         "Name of a pre-configured ACP server. Uses the first configured server if omitted",
			"server_url":     "URL of an ACP server to connect to directly (e.g. http://localhost:8199). Overrides server name lookup",
			"server_headers": "Optional JSON object of HTTP headers to send with the request (e.g. {\"Authorization\":\"Bearer sk-xxx\"})",
			"session_id":     "Session ID for multi-turn conversations with the same agent",
		},
		Required: []string{"agent_name", "input"},
	},
	{
		Name:        "acp_resume_run",
		Description: "Resume an ACP agent run that is waiting for additional input. When acp_run_agent returns a response indicating the agent is awaiting input, use this tool to provide the requested information and continue the run",
		Parameters: map[string]string{
			"run_id":         "The run_id returned by acp_run_agent when the agent entered awaiting state",
			"input":          "Text response to provide to the awaiting agent",
			"server":         "Name of a pre-configured ACP server. Uses the first configured server if omitted",
			"server_url":     "URL of an ACP server to connect to directly (e.g. http://localhost:8199). Overrides server name lookup",
			"server_headers": "Optional JSON object of HTTP headers to send with the request (e.g. {\"Authorization\":\"Bearer sk-xxx\"})",
		},
		Required: []string{"run_id", "input"},
	},
}
