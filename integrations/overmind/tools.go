package overmind

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name: "flow_launch_agent",
		Description: `Launch a child agent within the current flow. The agent must be in the flow's available_agent_ids list. Returns the new AgentRun ID which can be polled for status and results.

Use this to delegate subtasks to specialized agents. The child agent runs in its own Pod with its own MCP integrations.`,
		Parameters: map[string]string{
			"agent_id": "The ID of the agent to launch (must be in the flow's available_agent_ids)",
			"prompt":   "The task prompt to give the child agent",
			"context":  "Optional additional context or data to pass to the child agent",
		},
		Required: []string{"agent_id", "prompt"},
	},
	{
		Name: "flow_get_agent_status",
		Description: `Check the current status of an agent run. Returns the agent's state (pending, launching, running, completed, failed, recovering, cancelled).

Poll this periodically after launching a child agent to check if it has completed.`,
		Parameters: map[string]string{
			"agent_run_id": "The AgentRun ID to check (returned by flow_launch_agent)",
		},
		Required: []string{"agent_run_id"},
	},
	{
		Name: "flow_get_agent_result",
		Description: `Get the output/result of a completed agent run. Only available after the agent reaches 'completed' state.

Use flow_get_agent_status first to verify the agent has completed before calling this.`,
		Parameters: map[string]string{
			"agent_run_id": "The AgentRun ID to get results for",
		},
		Required: []string{"agent_run_id"},
	},
	{
		Name: "flow_complete",
		Description: `Signal that the current flow is complete. This marks the flow as finished and triggers cleanup of all running agent Pods.

Call this when all work is done and results have been collected. After calling this, no further agent launches or tool calls should be made.`,
		Parameters: map[string]string{
			"summary": "A brief summary of the flow's outcome",
			"status":  "The completion status: 'success' or 'failure' (default: 'success')",
		},
		Required: []string{"summary"},
	},
}
