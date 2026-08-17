package agents

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// =========================================================================
	// ProjectService — gRPC via h2c
	// =========================================================================
	{
		Name:        "agents_project_list",
		Description: "List all registered projects via gRPC ProjectService.ListProjects. Returns Project objects with name, repo, branch, and agent templates.",
		Parameters:  map[string]string{},
	},
	{
		Name:        "agents_project_register",
		Description: "Register a new project via gRPC ProjectService.RegisterProject. A project defines a git repo and agent templates for spawning.",
		Parameters: map[string]string{
			"name":   "Unique project name",
			"repo":   "Path or URL to the git repository",
			"branch": "Default branch for new workspaces (default: main)",
			"agents": `JSON array of AgentTemplate objects. Each has: name (required), command (required), port_env, capabilities (string array), a2a_card_config ({name, description, skills, input_modes, output_modes, streaming})`,
		},
		Required: []string{"name", "repo"},
	},
	{
		Name:        "agents_project_unregister",
		Description: "Unregister a project via gRPC ProjectService.UnregisterProject. Fails if active workspaces with running agents exist.",
		Parameters: map[string]string{
			"name": "Project name to unregister",
		},
		Required: []string{"name"},
	},

	// =========================================================================
	// WorkspaceService — gRPC via h2c
	// =========================================================================
	{
		Name:        "agents_workspace_create",
		Description: "Create a new workspace via gRPC WorkspaceService.CreateWorkspace. Creates a git worktree and optionally auto-spawns agents.",
		Parameters: map[string]string{
			"name":        "Unique workspace name (used as worktree branch name)",
			"project":     "Project to create workspace from (must be registered)",
			"branch":      "Git branch override (defaults to project's default branch)",
			"auto_agents": `JSON array of template names to auto-spawn (e.g. ["crush", "reviewer"])`,
		},
		Required: []string{"name", "project"},
	},
	{
		Name:        "agents_workspace_list",
		Description: "List workspaces via gRPC WorkspaceService.ListWorkspaces. Optionally filter by project or status.",
		Parameters: map[string]string{
			"project": "Filter by project name",
			"status":  "Filter by status: active or inactive",
		},
	},
	{
		Name:        "agents_workspace_get",
		Description: "Get workspace details via gRPC WorkspaceService.GetWorkspace. Returns agents, directory path, and creation time.",
		Parameters: map[string]string{
			"name": "Workspace name",
		},
		Required: []string{"name"},
	},
	{
		Name:        "agents_workspace_destroy",
		Description: "Destroy a workspace via gRPC WorkspaceService.DestroyWorkspace. Stops all agents, cancels working tasks, and optionally removes the worktree.",
		Parameters: map[string]string{
			"name":          "Workspace name to destroy",
			"keep_worktree": "If true, preserve the git worktree directory on disk (default: false)",
		},
		Required: []string{"name"},
	},

	// =========================================================================
	// AgentService — gRPC via h2c (lifecycle)
	// =========================================================================
	{
		Name:        "agents_agent_spawn",
		Description: "Spawn a new A2A agent via gRPC AgentService.SpawnAgent. Returns AgentInstance with id, port, direct_url, proxy_url, and status.",
		Parameters: map[string]string{
			"workspace":  "Workspace name to spawn the agent in",
			"template":   "Agent template name from the project",
			"name":       "Custom instance name (defaults to template name)",
			"env":        "JSON object of additional environment variables",
			"prompt":     "Initial prompt to send after agent reaches READY",
			"scope":      `JSON Scope object: {"global": false, "projects": ["proj-a"]}`,
			"permission": "Permission level: session, project, or admin",
		},
		Required: []string{"workspace", "template"},
	},
	{
		Name:        "agents_agent_list",
		Description: "List agent instances via gRPC AgentService.ListAgents. Optionally filter by workspace, status, or template.",
		Parameters: map[string]string{
			"workspace": "Filter by workspace name",
			"status":    "Filter by status: starting, ready, busy, error, stopping, stopped",
			"template":  "Filter by template name",
		},
	},
	{
		Name:        "agents_agent_status",
		Description: "Get agent status via gRPC AgentService.GetAgentStatus. Returns AgentInstance with resolved A2A AgentCard.",
		Parameters: map[string]string{
			"agent_id": "Agent instance ID",
		},
		Required: []string{"agent_id"},
	},
	{
		Name:        "agents_agent_stop",
		Description: "Stop an agent via gRPC AgentService.StopAgent. Cancels working A2A tasks, sends SIGTERM, waits grace period, then SIGKILL.",
		Parameters: map[string]string{
			"agent_id":        "Agent instance ID to stop",
			"grace_period_ms": "Milliseconds to wait after SIGTERM before SIGKILL (default: 5000)",
		},
		Required: []string{"agent_id"},
	},
	{
		Name:        "agents_agent_restart",
		Description: "Restart an agent via gRPC AgentService.RestartAgent. Stop + spawn with same config. proxy_url stays stable.",
		Parameters: map[string]string{
			"agent_id": "Agent instance ID to restart",
		},
		Required: []string{"agent_id"},
	},

	// =========================================================================
	// AgentService — gRPC via h2c (messaging)
	// =========================================================================
	{
		Name:        "agents_agent_message",
		Description: "Send a message to an agent via gRPC AgentService.SendAgentMessage. When blocking=true (default), polls until the task completes. When blocking=false, returns the task immediately for async tracking via agents_agent_task_status.",
		Parameters: map[string]string{
			"agent_id":   "Agent instance ID",
			"message":    "Text message to send",
			"context_id": "A2A context_id for multi-turn conversations",
			"blocking":   "If true (default), wait for response. If false, return immediately.",
		},
		Required: []string{"agent_id", "message"},
	},
	{
		Name:        "agents_agent_task",
		Description: "Create a task on an agent via gRPC AgentService.CreateAgentTask. Returns immediately with a Task for async tracking via agents_agent_task_status (never blocks on completion).",
		Parameters: map[string]string{
			"agent_id":   "Agent instance ID",
			"message":    "Task description",
			"context_id": "A2A context_id to continue a conversation",
		},
		Required: []string{"agent_id", "message"},
	},
	{
		Name:        "agents_agent_task_status",
		Description: "Get task status via gRPC AgentService.GetAgentTaskStatus. Returns Task with status, artifacts, and history.",
		Parameters: map[string]string{
			"agent_id":       "Agent instance ID",
			"task_id":        "A2A task ID to check",
			"history_length": "Maximum number of recent messages to include (default: 10)",
		},
		Required: []string{"agent_id", "task_id"},
	},

	// =========================================================================
	// DiscoveryService — gRPC via h2c
	// =========================================================================
	{
		Name:        "agents_discover",
		Description: "Discover agents via gRPC DiscoveryService.DiscoverAgents. Returns enriched AgentCards. Supports local/network scope and capability filtering.",
		Parameters: map[string]string{
			"scope":      `Discovery scope: local (default) or network`,
			"capability": `Filter by AgentSkill tag (e.g. "coding")`,
			"urls":       `JSON array of base URLs to probe for AgentCards (network scope)`,
		},
	},

	// =========================================================================
	// A2A proxy — HTTP endpoints on separate port
	// =========================================================================
	{
		Name:        "agents_proxy_list",
		Description: "List all A2A AgentCards via the ARP HTTP proxy at /a2a/agents. Returns cards for READY/BUSY agents with metadata.arp fields.",
		Parameters:  map[string]string{},
	},
	{
		Name:        "agents_agent_card",
		Description: "Get an enriched A2A AgentCard via the ARP HTTP proxy. Includes metadata.arp and supportedInterfaces pointing to the proxy.",
		Parameters: map[string]string{
			"agent_id": "Agent ID, name, or workspace/instance_name",
		},
		Required: []string{"agent_id"},
	},
	{
		Name:        "agents_proxy_send_message",
		Description: "Send an A2A message through the ARP HTTP proxy at /a2a/agents/{id}/message:send. Routes by agent ID, name, or workspace/name.",
		Parameters: map[string]string{
			"agent_id":   "Agent ID, name, or workspace/instance_name",
			"message":    "Text message to send",
			"context_id": "A2A context_id for multi-turn conversation",
			"message_id": "Message ID (auto-generated if omitted)",
		},
		Required: []string{"agent_id", "message"},
	},
	{
		Name:        "agents_proxy_get_task",
		Description: "Get an A2A task via the ARP HTTP proxy.",
		Parameters: map[string]string{
			"agent_id": "Agent ID",
			"task_id":  "A2A task ID",
		},
		Required: []string{"agent_id", "task_id"},
	},
	{
		Name:        "agents_proxy_cancel_task",
		Description: "Cancel an A2A task via the ARP HTTP proxy.",
		Parameters: map[string]string{
			"agent_id": "Agent ID",
			"task_id":  "A2A task ID to cancel",
		},
		Required: []string{"agent_id", "task_id"},
	},
	{
		Name:        "agents_route_message",
		Description: "Route an A2A message by skill tags via the ARP HTTP proxy at /a2a/route/message:send. Finds best matching agent (prefers READY over BUSY).",
		Parameters: map[string]string{
			"message":    "Text message to send",
			"tags":       `JSON array of skill tags to match (e.g. ["coding"])`,
			"context_id": "A2A context_id for multi-turn conversation",
			"message_id": "Message ID (auto-generated if omitted)",
		},
		Required: []string{"message", "tags"},
	},
}
