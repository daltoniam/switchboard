package agents

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// =========================================================================
	// ProjectService — gRPC via h2c
	// =========================================================================
	{
		Name:        "agents_project_list",
		Description: "List all registered projects via gRPC ProjectService.ListProjects. Returns Project objects with name, repo, branch, and agent templates.",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        "agents_project_register",
		Description: "Register a new project via gRPC ProjectService.RegisterProject. A project defines a git repo and agent templates for spawning.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Unique project name", Required: true}, {Name: mcp.ParamName("repo"), Description: "Path or URL to the git repository", Required: true}, {Name: mcp.ParamName("branch"), Description: "Default branch for new workspaces (default: main)"}, {Name: mcp.ParamName("agents"), Description: "JSON array of AgentTemplate objects. Each has: name (required), command (required), port_env, capabilities (string array), a2a_card_config ({name, description, skills, input_modes, output_modes, streaming})"}},
	},
	{
		Name:        "agents_project_unregister",
		Description: "Unregister a project via gRPC ProjectService.UnregisterProject. Fails if active workspaces with running agents exist.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Project name to unregister", Required: true}},
	},

	// =========================================================================
	// WorkspaceService — gRPC via h2c
	// =========================================================================
	{
		Name:        "agents_workspace_create",
		Description: "Create a new workspace via gRPC WorkspaceService.CreateWorkspace. Creates a git worktree and optionally auto-spawns agents.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Unique workspace name (used as worktree branch name)", Required: true}, {Name: mcp.ParamName("project"), Description: "Project to create workspace from (must be registered)", Required: true}, {Name: mcp.ParamName("branch"), Description: "Git branch override (defaults to project's default branch)"}, {Name: mcp.ParamName("auto_agents"), Description: `JSON array of template names to auto-spawn (e.g. ["crush", "reviewer"])`}},
	},
	{
		Name:        "agents_workspace_list",
		Description: "List workspaces via gRPC WorkspaceService.ListWorkspaces. Optionally filter by project or status.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("project"), Description: "Filter by project name"}, {Name: mcp.ParamName("status"), Description: "Filter by status: active or inactive"}},
	},
	{
		Name:        "agents_workspace_get",
		Description: "Get workspace details via gRPC WorkspaceService.GetWorkspace. Returns agents, directory path, and creation time.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Workspace name", Required: true}},
	},
	{
		Name:        "agents_workspace_destroy",
		Description: "Destroy a workspace via gRPC WorkspaceService.DestroyWorkspace. Stops all agents, cancels working tasks, and optionally removes the worktree.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Workspace name to destroy", Required: true}, {Name: mcp.ParamName("keep_worktree"), Description: "If true, preserve the git worktree directory on disk (default: false)"}},
	},

	// =========================================================================
	// AgentService — gRPC via h2c (lifecycle)
	// =========================================================================
	{
		Name:        "agents_agent_spawn",
		Description: "Spawn a new A2A agent via gRPC AgentService.SpawnAgent. Returns AgentInstance with id, port, direct_url, proxy_url, and status.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("workspace"), Description: "Workspace name to spawn the agent in", Required: true}, {Name: mcp.ParamName("template"), Description: "Agent template name from the project", Required: true}, {Name: mcp.ParamName("name"), Description: "Custom instance name (defaults to template name)"}, {Name: mcp.ParamName("env"), Description: "JSON object of additional environment variables"}, {Name: mcp.ParamName("prompt"), Description: "Initial prompt to send after agent reaches READY"}, {Name: mcp.ParamName("scope"), Description: `JSON Scope object: {"global": false, "projects": ["proj-a"]}`}, {Name: mcp.ParamName("permission"), Description: "Permission level: session, project, or admin"}},
	},
	{
		Name:        "agents_agent_list",
		Description: "List agent instances via gRPC AgentService.ListAgents. Optionally filter by workspace, status, or template.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("workspace"), Description: "Filter by workspace name"}, {Name: mcp.ParamName("status"), Description: "Filter by status: starting, ready, busy, error, stopping, stopped"}, {Name: mcp.ParamName("template"), Description: "Filter by template name"}},
	},
	{
		Name:        "agents_agent_status",
		Description: "Get agent status via gRPC AgentService.GetAgentStatus. Returns AgentInstance with resolved A2A AgentCard.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("agent_id"), Description: "Agent instance ID", Required: true}},
	},
	{
		Name:        "agents_agent_stop",
		Description: "Stop an agent via gRPC AgentService.StopAgent. Cancels working A2A tasks, sends SIGTERM, waits grace period, then SIGKILL.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("agent_id"), Description: "Agent instance ID to stop", Required: true}, {Name: mcp.ParamName("grace_period_ms"), Description: "Milliseconds to wait after SIGTERM before SIGKILL (default: 5000)"}},
	},
	{
		Name:        "agents_agent_restart",
		Description: "Restart an agent via gRPC AgentService.RestartAgent. Stop + spawn with same config. proxy_url stays stable.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("agent_id"), Description: "Agent instance ID to restart", Required: true}},
	},

	// =========================================================================
	// AgentService — gRPC via h2c (messaging)
	// =========================================================================
	{
		Name:        "agents_agent_message",
		Description: "Send a message to an agent via gRPC AgentService.SendAgentMessage. When blocking=true (default), polls until the task completes. When blocking=false, returns the task immediately for async tracking via agents_agent_task_status.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("agent_id"), Description: "Agent instance ID", Required: true}, {Name: mcp.ParamName("message"), Description: "Text message to send", Required: true}, {Name: mcp.ParamName("context_id"), Description: "A2A context_id for multi-turn conversations"}, {Name: mcp.ParamName("blocking"), Description: "If true (default), wait for response. If false, return immediately."}},
	},
	{
		Name:        "agents_agent_task",
		Description: "Create a task on an agent via gRPC AgentService.CreateAgentTask. Returns immediately with a Task for async tracking via agents_agent_task_status (never blocks on completion).",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("agent_id"), Description: "Agent instance ID", Required: true}, {Name: mcp.ParamName("message"), Description: "Task description", Required: true}, {Name: mcp.ParamName("context_id"), Description: "A2A context_id to continue a conversation"}},
	},
	{
		Name:        "agents_agent_task_status",
		Description: "Get task status via gRPC AgentService.GetAgentTaskStatus. Returns Task with status, artifacts, and history.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("agent_id"), Description: "Agent instance ID", Required: true}, {Name: mcp.ParamName("task_id"), Description: "A2A task ID to check", Required: true}, {Name: mcp.ParamName("history_length"), Description: "Maximum number of recent messages to include (default: 10)"}},
	},

	// =========================================================================
	// DiscoveryService — gRPC via h2c
	// =========================================================================
	{
		Name:        "agents_discover",
		Description: "Discover agents via gRPC DiscoveryService.DiscoverAgents. Returns enriched AgentCards. Supports local/network scope and capability filtering.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("scope"), Description: "Discovery scope: local (default) or network"}, {Name: mcp.ParamName("capability"), Description: `Filter by AgentSkill tag (e.g. "coding")`}, {Name: mcp.ParamName("urls"), Description: "JSON array of base URLs to probe for AgentCards (network scope)"}},
	},

	// =========================================================================
	// A2A proxy — HTTP endpoints on separate port
	// =========================================================================
	{
		Name:        "agents_proxy_list",
		Description: "List all A2A AgentCards via the ARP HTTP proxy at /a2a/agents. Returns cards for READY/BUSY agents with metadata.arp fields.",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        "agents_agent_card",
		Description: "Get an enriched A2A AgentCard via the ARP HTTP proxy. Includes metadata.arp and supportedInterfaces pointing to the proxy.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("agent_id"), Description: "Agent ID, name, or workspace/instance_name", Required: true}},
	},
	{
		Name:        "agents_proxy_send_message",
		Description: "Send an A2A message through the ARP HTTP proxy at /a2a/agents/{id}/message:send. Routes by agent ID, name, or workspace/name.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("agent_id"), Description: "Agent ID, name, or workspace/instance_name", Required: true}, {Name: mcp.ParamName("message"), Description: "Text message to send", Required: true}, {Name: mcp.ParamName("context_id"), Description: "A2A context_id for multi-turn conversation"}, {Name: mcp.ParamName("message_id"), Description: "Message ID (auto-generated if omitted)"}},
	},
	{
		Name:        "agents_proxy_get_task",
		Description: "Get an A2A task via the ARP HTTP proxy.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("agent_id"), Description: "Agent ID", Required: true}, {Name: mcp.ParamName("task_id"), Description: "A2A task ID", Required: true}},
	},
	{
		Name:        "agents_proxy_cancel_task",
		Description: "Cancel an A2A task via the ARP HTTP proxy.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("agent_id"), Description: "Agent ID", Required: true}, {Name: mcp.ParamName("task_id"), Description: "A2A task ID to cancel", Required: true}},
	},
	{
		Name:        "agents_route_message",
		Description: "Route an A2A message by skill tags via the ARP HTTP proxy at /a2a/route/message:send. Finds best matching agent (prefers READY over BUSY).",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("message"), Description: "Text message to send", Required: true}, {Name: mcp.ParamName("tags"), Description: `JSON array of skill tags to match (e.g. ["coding"])`, Required: true}, {Name: mcp.ParamName("context_id"), Description: "A2A context_id for multi-turn conversation"}, {Name: mcp.ParamName("message_id"), Description: "Message ID (auto-generated if omitted)"}},
	},
}
