package overmind

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Flow tools (agent-facing) ────────────────────────────────────────
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

	// ── Agents admin ─────────────────────────────────────────────────────
	{
		Name:        "overmind_list_agents",
		Description: "List all agent definitions in overmind. Start here to discover available agents, their models, prompts, and MCP role assignments.",
		Parameters:  map[string]string{},
	},
	{
		Name:        "overmind_get_agent",
		Description: "Get an agent definition by ID. Returns name, model, model_provider, base_prompt, mcp_role_id, and description. Use after list_agents.",
		Parameters:  map[string]string{"id": "Agent ID (UUID)"},
		Required:    []string{"id"},
	},
	{
		Name: "overmind_create_agent",
		Description: `Create a new agent definition. Agents are AI workers with a model, prompt, and MCP role that determines which integrations and tools they can access.`,
		Parameters: map[string]string{
			"name":           "Unique agent name",
			"description":    "Human-readable description of the agent's purpose",
			"model":          "LLM model name (e.g. claude-sonnet-4-20250514)",
			"model_provider": "Model provider (e.g. anthropic, openai)",
			"base_prompt":    "System prompt for the agent",
			"mcp_role_id":    "MCP role ID that controls which integrations/tools this agent can access",
		},
		Required: []string{"name"},
	},
	{
		Name: "overmind_update_agent",
		Description: `Update an existing agent definition. Use after get_agent to modify name, model, prompt, or role assignment.`,
		Parameters: map[string]string{
			"id":             "Agent ID (UUID)",
			"name":           "Agent name",
			"description":    "Human-readable description",
			"model":          "LLM model name",
			"model_provider": "Model provider",
			"base_prompt":    "System prompt",
			"mcp_role_id":    "MCP role ID",
		},
		Required: []string{"id"},
	},
	{
		Name:        "overmind_delete_agent",
		Description: "Delete an agent definition. Fails if the agent is referenced by active flows.",
		Parameters:  map[string]string{"id": "Agent ID (UUID)"},
		Required:    []string{"id"},
	},

	// ── Flows admin ──────────────────────────────────────────────────────
	{
		Name:        "overmind_list_flows",
		Description: "List all flow definitions. Flows are orchestrated multi-agent pipelines with a prompt template, available agents, and webhook configuration. Start here for flow management.",
		Parameters:  map[string]string{},
	},
	{
		Name: "overmind_get_flow",
		Description: `Get a flow definition by ID. Returns name, prompt_template, available agents, webhook config, repo_url, and timeout. Use expand=agents to include agent details.`,
		Parameters: map[string]string{
			"id":     "Flow ID (UUID)",
			"expand": "Set to 'agents' to include agent details in the response",
		},
		Required: []string{"id"},
	},
	{
		Name: "overmind_create_flow",
		Description: `Create a new flow definition. A flow orchestrates multiple agents with a prompt template, webhook triggers, and output configuration.`,
		Parameters: map[string]string{
			"name":                    "Unique flow name",
			"description":             "Human-readable description",
			"prompt_template":         "Go template for the flow prompt (rendered with webhook params)",
			"initial_agent_id":        "ID of the first agent to run when the flow starts",
			"available_agent_ids":     "JSON array of agent IDs that can be launched within this flow",
			"repo_url":                "Git repository URL for the flow's code",
			"repo_ref":                "Git ref (branch/tag) to check out",
			"timeout_minutes":         "Maximum flow duration in minutes",
			"enabled":                 "Whether the flow accepts triggers (default: false)",
			"output_webhook_url":      "URL to POST flow results to on completion",
			"output_webhook_template": "Go template for the output webhook body",
			"webhook_secret":          "HMAC secret for validating inbound webhook triggers",
		},
		Required: []string{"name"},
	},
	{
		Name: "overmind_update_flow",
		Description: `Update an existing flow definition. Use after get_flow.`,
		Parameters: map[string]string{
			"id":                      "Flow ID (UUID)",
			"name":                    "Flow name",
			"description":             "Human-readable description",
			"prompt_template":         "Go template for the flow prompt",
			"initial_agent_id":        "ID of the first agent to run",
			"available_agent_ids":     "JSON array of agent IDs available in this flow",
			"repo_url":                "Git repository URL",
			"repo_ref":                "Git ref (branch/tag)",
			"timeout_minutes":         "Maximum flow duration in minutes",
			"enabled":                 "Whether the flow accepts triggers",
			"output_webhook_url":      "URL to POST flow results to",
			"output_webhook_template": "Go template for the output webhook body",
			"webhook_secret":          "HMAC secret for webhook triggers",
		},
		Required: []string{"id"},
	},
	{
		Name:        "overmind_delete_flow",
		Description: "Delete a flow definition. Fails if the flow has active runs.",
		Parameters:  map[string]string{"id": "Flow ID (UUID)"},
		Required:    []string{"id"},
	},
	{
		Name: "overmind_clone_flow",
		Description: `Clone an existing flow, creating a copy with a new name. Duplicates all settings including agent assignments.`,
		Parameters: map[string]string{
			"id":   "Flow ID (UUID) to clone",
			"name": "Name for the cloned flow (optional, defaults to 'Copy of <original>')",
		},
		Required: []string{"id"},
	},
	{
		Name: "overmind_run_flow",
		Description: `Trigger a flow run directly with a prompt or template parameters. Returns the flow_run_id and initial_agent_run_id. Use after get_flow.`,
		Parameters: map[string]string{
			"id":     "Flow ID (UUID) to run",
			"prompt": "Direct prompt text (used if no prompt_template is set on the flow)",
			"params": "JSON object of template parameters (used to render prompt_template)",
		},
		Required: []string{"id"},
	},
	{
		Name:        "overmind_validate_flow",
		Description: "Validate a flow definition without creating it. Returns validation errors if the flow configuration is invalid.",
		Parameters: map[string]string{
			"name":                    "Flow name",
			"description":             "Human-readable description",
			"prompt_template":         "Go template for the flow prompt",
			"initial_agent_id":        "ID of the first agent to run",
			"available_agent_ids":     "JSON array of agent IDs",
			"repo_url":                "Git repository URL",
			"repo_ref":                "Git ref (branch/tag)",
			"timeout_minutes":         "Maximum flow duration in minutes",
			"enabled":                 "Whether the flow accepts triggers",
			"output_webhook_url":      "Output webhook URL",
			"output_webhook_template": "Output webhook body template",
			"webhook_secret":          "HMAC secret for webhook triggers",
		},
	},

	// ── Flow runs admin ──────────────────────────────────────────────────
	{
		Name:        "overmind_list_flow_runs",
		Description: "List runs for a flow. Returns run ID, state (pending, running, completed, failed, cancelled), timestamps, and rendered prompt. Start here to monitor flow execution.",
		Parameters:  map[string]string{"flow_id": "Flow ID (UUID) to list runs for"},
		Required:    []string{"flow_id"},
	},
	{
		Name:        "overmind_get_flow_run",
		Description: "Get a flow run by ID with its agent runs. Returns state, rendered prompt, output, and all child agent runs. Use after list_flow_runs.",
		Parameters:  map[string]string{"id": "Flow Run ID (UUID)"},
		Required:    []string{"id"},
	},
	{
		Name:        "overmind_cancel_flow_run",
		Description: "Cancel a running flow. Marks the flow run and all active agent runs as cancelled.",
		Parameters:  map[string]string{"id": "Flow Run ID (UUID)"},
		Required:    []string{"id"},
	},

	// ── Agent runs admin ─────────────────────────────────────────────────
	{
		Name:        "overmind_list_agent_runs",
		Description: "List agent runs for a flow run. Returns agent_id, state, attempt count, timestamps, and worker assignment. Use after get_flow_run for more detail on individual agents.",
		Parameters:  map[string]string{"flow_run_id": "Flow Run ID (UUID)"},
		Required:    []string{"flow_run_id"},
	},
	{
		Name:        "overmind_get_agent_run",
		Description: "Get an agent run by ID with session messages. Returns full execution detail including state, output, heartbeat, pod_name, and conversation history.",
		Parameters:  map[string]string{"id": "Agent Run ID (UUID)"},
		Required:    []string{"id"},
	},

	// ── MCP identities admin ─────────────────────────────────────────────
	{
		Name:        "overmind_list_mcp_identities",
		Description: "List all MCP identities. Identities store encrypted credentials for a specific integration (e.g. GitHub token, Slack token). Start here for credential management.",
		Parameters:  map[string]string{},
	},
	{
		Name:        "overmind_get_mcp_identity",
		Description: "Get an MCP identity by ID. Returns name, integration_name, and masked credentials. Use after list_mcp_identities.",
		Parameters:  map[string]string{"id": "MCP Identity ID (UUID)"},
		Required:    []string{"id"},
	},
	{
		Name: "overmind_create_mcp_identity",
		Description: `Create an MCP identity with encrypted credentials for an integration. Used in MCP role entries to grant agents access to specific integrations.`,
		Parameters: map[string]string{
			"name":             "Unique identity name",
			"integration_name": "Integration this identity authenticates (e.g. github, slack, datadog)",
			"credentials":     "JSON object of credential key-value pairs for the integration",
		},
		Required: []string{"name", "integration_name", "credentials"},
	},
	{
		Name: "overmind_update_mcp_identity",
		Description: "Update an MCP identity's name, integration, or credentials. Use after get_mcp_identity.",
		Parameters: map[string]string{
			"id":               "MCP Identity ID (UUID)",
			"name":             "Identity name",
			"integration_name": "Integration name",
			"credentials":     "JSON object of credential key-value pairs",
		},
		Required: []string{"id"},
	},
	{
		Name:        "overmind_delete_mcp_identity",
		Description: "Delete an MCP identity. Fails if referenced by MCP role entries.",
		Parameters:  map[string]string{"id": "MCP Identity ID (UUID)"},
		Required:    []string{"id"},
	},

	// ── MCP roles admin ──────────────────────────────────────────────────
	{
		Name:        "overmind_list_mcp_roles",
		Description: "List all MCP roles. Roles group MCP identities with tool glob patterns to control which integrations and tools an agent can access. Start here for access control management.",
		Parameters:  map[string]string{},
	},
	{
		Name:        "overmind_get_mcp_role",
		Description: "Get an MCP role by ID with its entries. Each entry maps an MCP identity to tool glob patterns. Use after list_mcp_roles.",
		Parameters:  map[string]string{"id": "MCP Role ID (UUID)"},
		Required:    []string{"id"},
	},
	{
		Name: "overmind_create_mcp_role",
		Description: `Create an MCP role. Roles are assigned to agents and contain entries that pair MCP identities with tool glob patterns.`,
		Parameters: map[string]string{
			"name":        "Unique role name",
			"description": "Human-readable description of what this role grants",
		},
		Required: []string{"name"},
	},
	{
		Name: "overmind_update_mcp_role",
		Description: "Update an MCP role's name or description. Use after get_mcp_role.",
		Parameters: map[string]string{
			"id":          "MCP Role ID (UUID)",
			"name":        "Role name",
			"description": "Role description",
		},
		Required: []string{"id"},
	},
	{
		Name:        "overmind_delete_mcp_role",
		Description: "Delete an MCP role. Fails if assigned to agents.",
		Parameters:  map[string]string{"id": "MCP Role ID (UUID)"},
		Required:    []string{"id"},
	},
	{
		Name: "overmind_create_mcp_role_entry",
		Description: `Add an entry to an MCP role. Each entry grants access to an MCP identity's integration, optionally restricted by tool glob patterns.`,
		Parameters: map[string]string{
			"role_id":         "MCP Role ID (UUID)",
			"mcp_identity_id": "MCP Identity ID to grant access to",
			"tool_globs":      "JSON array of tool name glob patterns (e.g. [\"github_*\", \"slack_post_*\"]). Empty means all tools.",
		},
		Required: []string{"role_id", "mcp_identity_id"},
	},
	{
		Name: "overmind_update_mcp_role_entry",
		Description: "Update an MCP role entry's identity or tool glob patterns. Use after get_mcp_role.",
		Parameters: map[string]string{
			"role_id":         "MCP Role ID (UUID)",
			"entry_id":        "MCP Role Entry ID (UUID)",
			"mcp_identity_id": "MCP Identity ID",
			"tool_globs":      "JSON array of tool name glob patterns",
		},
		Required: []string{"role_id", "entry_id"},
	},
	{
		Name:        "overmind_delete_mcp_role_entry",
		Description: "Remove an entry from an MCP role.",
		Parameters: map[string]string{
			"role_id":  "MCP Role ID (UUID)",
			"entry_id": "MCP Role Entry ID (UUID)",
		},
		Required: []string{"role_id", "entry_id"},
	},

	// ── Pipelines admin ──────────────────────────────────────────────────
	{
		Name:        "overmind_list_pipelines",
		Description: "List pipelines, optionally filtered by global context. Pipelines are legacy task-based workflows with ordered task definitions.",
		Parameters: map[string]string{
			"global_context_id": "Filter by Global Context ID (UUID)",
		},
	},
	{
		Name:        "overmind_get_pipeline",
		Description: "Get a pipeline by ID. Returns name, context, and global_context_id. Use after list_pipelines.",
		Parameters:  map[string]string{"id": "Pipeline ID (UUID)"},
		Required:    []string{"id"},
	},
	{
		Name: "overmind_create_pipeline",
		Description: "Create a pipeline under a global context.",
		Parameters: map[string]string{
			"name":              "Pipeline name",
			"global_context_id": "Global Context ID (UUID)",
			"context":           "JSON context object with prompt, env, switchboard, and worktrees fields",
		},
		Required: []string{"name", "global_context_id"},
	},
	{
		Name: "overmind_update_pipeline",
		Description: "Update a pipeline's name or context. Use after get_pipeline.",
		Parameters: map[string]string{
			"id":      "Pipeline ID (UUID)",
			"name":    "Pipeline name",
			"context": "JSON context object",
		},
		Required: []string{"id"},
	},
	{
		Name:        "overmind_delete_pipeline",
		Description: "Delete a pipeline.",
		Parameters:  map[string]string{"id": "Pipeline ID (UUID)"},
		Required:    []string{"id"},
	},

	// ── Tasks admin ──────────────────────────────────────────────────────
	{
		Name:        "overmind_list_tasks",
		Description: "List tasks for a pipeline. Tasks are individual work units with context and dependency ordering.",
		Parameters:  map[string]string{"pipeline_id": "Pipeline ID (UUID)"},
		Required:    []string{"pipeline_id"},
	},
	{
		Name:        "overmind_get_task",
		Description: "Get a task by ID. Returns name, context, pipeline_id, and depends_on list. Use after list_tasks.",
		Parameters:  map[string]string{"id": "Task ID (UUID)"},
		Required:    []string{"id"},
	},
	{
		Name: "overmind_create_task",
		Description: "Create a task within a pipeline.",
		Parameters: map[string]string{
			"name":        "Task name",
			"pipeline_id": "Pipeline ID (UUID)",
			"context":     "JSON context object with prompt, env, switchboard, and worktrees fields",
			"depends_on":  "JSON array of task IDs this task depends on",
		},
		Required: []string{"name", "pipeline_id"},
	},
	{
		Name: "overmind_update_task",
		Description: "Update a task's name, context, or dependencies. Use after get_task.",
		Parameters: map[string]string{
			"id":         "Task ID (UUID)",
			"name":       "Task name",
			"context":    "JSON context object",
			"depends_on": "JSON array of task IDs",
		},
		Required: []string{"id"},
	},
	{
		Name:        "overmind_delete_task",
		Description: "Delete a task from a pipeline.",
		Parameters:  map[string]string{"id": "Task ID (UUID)"},
		Required:    []string{"id"},
	},
}
