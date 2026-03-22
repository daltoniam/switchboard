package overmind

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name: "overmind_list_available_agents",
		Description: `List all agents available to launch in the current flow. Returns each agent's ID, name, and description.

Call this first to discover which agents you can dispatch with overmind_launch_agent. The returned agent IDs are the values you pass as agent_id to overmind_launch_agent.`,
		Parameters: map[string]string{},
		Required:   []string{},
	},
	{
		Name: "overmind_launch_agent",
		Description: `Launch a child agent within the current flow. Returns the new AgentRun ID which can be polled for status and results.

The agent_id is provided in your initial prompt or flow context. Use this to delegate subtasks to specialized agents. The child agent runs in its own Pod with its own MCP integrations.`,
		Parameters: map[string]string{
			"agent_id": "The ID of the agent to launch (must be in the flow's available_agent_ids)",
			"prompt":   "The task prompt to give the child agent",
			"context":  "Optional additional context or data to pass to the child agent",
		},
		Required: []string{"agent_id", "prompt"},
	},
	{
		Name: "overmind_get_agent_status",
		Description: `Check the current status of an agent run. Returns the agent's state (pending, launching, running, completed, failed, recovering, cancelled).

Poll this periodically after launching a child agent to check if it has completed.`,
		Parameters: map[string]string{
			"agent_run_id": "The AgentRun ID to check (returned by overmind_launch_agent)",
		},
		Required: []string{"agent_run_id"},
	},
	{
		Name: "overmind_get_agent_result",
		Description: `Get the output/result of a completed agent run. Only available after the agent reaches 'completed' state.

Use overmind_get_agent_status first to verify the agent has completed before calling this.`,
		Parameters: map[string]string{
			"agent_run_id": "The AgentRun ID to get results for",
		},
		Required: []string{"agent_run_id"},
	},
	{
		Name: "overmind_complete_flow",
		Description: `Signal that the current flow is complete. This marks the flow as finished and triggers cleanup of all running agent Pods. The calling agent's identity is injected automatically.

Call this when all work is done and results have been collected. After calling this, no further agent launches or tool calls should be made.`,
		Parameters: map[string]string{
			"summary": "A brief summary of the flow's outcome",
			"status":  "The completion status: 'success' or 'failure' (default: 'success')",
		},
		Required: []string{"summary"},
	},

	// --- CRUD tools ---

	{
		Name:        "overmind_list_mcp_roles",
		Description: `List all MCP roles in the system. Returns each role's ID, name, and description.`,
		Parameters:  map[string]string{},
		Required:    []string{},
	},
	{
		Name: "overmind_create_mcp_role",
		Description: `Create a new MCP role. An MCP role groups tool-access permissions and is assigned to agents.

Returns the created role with its generated ID.`,
		Parameters: map[string]string{
			"name":        "Name for the new role",
			"description": "Optional description of the role's purpose",
		},
		Required: []string{"name"},
	},
	{
		Name:        "overmind_list_agents",
		Description: `List all agents in the system. Returns each agent's ID, name, description, model, and MCP role.`,
		Parameters:  map[string]string{},
		Required:    []string{},
	},
	{
		Name: "overmind_create_agent",
		Description: `Create a new agent definition. An agent has a base prompt, model configuration, and an MCP role that controls its tool access.

Returns the created agent with its generated ID.`,
		Parameters: map[string]string{
			"name":           "Name for the new agent",
			"description":    "Optional description of the agent's purpose",
			"base_prompt":    "The system prompt that defines the agent's behavior",
			"model":          "The model to use (e.g. 'claude-sonnet-4-20250514')",
			"model_provider": "The model provider (e.g. 'anthropic')",
			"mcp_role_id":    "The ID of the MCP role to assign (from overmind_list_mcp_roles or overmind_create_mcp_role)",
		},
		Required: []string{"name", "base_prompt", "model", "model_provider", "mcp_role_id"},
	},
	{
		Name:        "overmind_list_flows",
		Description: `List all flows in the system. Returns each flow's ID, name, description, initial agent, webhook params, and enabled status.`,
		Parameters:  map[string]string{},
		Required:    []string{},
	},
	{
		Name: "overmind_create_flow",
		Description: `Create a new flow. A flow defines a multi-agent pipeline with a webhook trigger, prompt template, and initial agent.

The prompt_template uses Go template syntax with webhook param names (e.g. '{{ .param_name }}').
webhook_params is a JSON array of param objects: [{"name":"param_name","type":"string","required":true}].
output_params is a JSON array of output param objects: [{"name":"result","type":"string"}].

Returns the created flow with its generated ID and webhook secret.`,
		Parameters: map[string]string{
			"name":              "Name for the new flow",
			"description":       "Optional description of the flow's purpose",
			"initial_agent_id":  "The ID of the agent that runs first when the flow is triggered",
			"prompt_template":   "Go template for the agent prompt, rendered with webhook params",
			"webhook_params":    "JSON array of webhook parameter definitions",
			"output_params":     "JSON array of output parameter definitions",
			"enabled":           "Whether the flow is enabled (default: true)",
			"timeout_minutes":   "Max runtime in minutes before the flow is failed (default: 30)",
		},
		Required: []string{"name", "initial_agent_id", "prompt_template"},
	},
}
