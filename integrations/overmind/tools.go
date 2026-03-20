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
}
