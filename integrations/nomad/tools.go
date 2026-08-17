package nomad

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Jobs ─────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("nomad_list_jobs"), Description: "List all jobs in the Nomad cluster. Start here for workload orchestration, container scheduling, and discovering running services.",
		Parameters: map[string]string{"namespace": "Namespace (default: default)", "prefix": "Filter by job ID prefix", "filter": "Filter expression (e.g., Status == \"running\")"},
	},
	{
		Name: mcp.ToolName("nomad_get_job"), Description: "Get full specification and status of a specific Nomad job, including task groups, constraints, and resource requirements. Use after list_jobs.",
		Parameters: map[string]string{"job_id": "Job ID", "namespace": "Namespace (default: default)"},
		Required:   []string{"job_id"},
	},
	{
		Name: mcp.ToolName("nomad_get_job_versions"), Description: "Get version history for a Nomad job. Shows previous configurations and when changes were made.",
		Parameters: map[string]string{"job_id": "Job ID", "namespace": "Namespace (default: default)"},
		Required:   []string{"job_id"},
	},
	{
		Name: mcp.ToolName("nomad_register_job"), Description: "Register (create or update) a Nomad job. Accepts a full job specification as JSON.",
		Parameters: map[string]string{"job": "Full job specification as JSON object", "namespace": "Namespace (default: default)"},
		Required:   []string{"job"},
	},
	{
		Name: mcp.ToolName("nomad_stop_job"), Description: "Stop (deregister) a running Nomad job. All allocations will be stopped.",
		Parameters: map[string]string{"job_id": "Job ID", "purge": "Completely purge the job from the system (true/false, default: false)", "namespace": "Namespace (default: default)"},
		Required:   []string{"job_id"},
	},
	{
		Name: mcp.ToolName("nomad_force_evaluate"), Description: "Force a new evaluation for a Nomad job, triggering rescheduling. Useful when allocations are unhealthy or stuck.",
		Parameters: map[string]string{"job_id": "Job ID", "namespace": "Namespace (default: default)"},
		Required:   []string{"job_id"},
	},

	// ── Allocations ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("nomad_list_allocations"), Description: "List all allocations across the Nomad cluster. Shows where tasks are placed and their health status.",
		Parameters: map[string]string{"namespace": "Namespace (default: default)", "prefix": "Filter by allocation ID prefix", "filter": "Filter expression"},
	},
	{
		Name: mcp.ToolName("nomad_get_allocation"), Description: "Get details of a specific Nomad allocation, including task states, events, restart history, and resource usage.",
		Parameters: map[string]string{"alloc_id": "Allocation ID"},
		Required:   []string{"alloc_id"},
	},
	{
		Name: mcp.ToolName("nomad_get_job_allocations"), Description: "List all allocations for a specific Nomad job. Shows placement, health, and status of each task instance.",
		Parameters: map[string]string{"job_id": "Job ID", "namespace": "Namespace (default: default)"},
		Required:   []string{"job_id"},
	},
	{
		Name: mcp.ToolName("nomad_stop_allocation"), Description: "Stop a specific Nomad allocation. The scheduler may place a replacement depending on job configuration.",
		Parameters: map[string]string{"alloc_id": "Allocation ID"},
		Required:   []string{"alloc_id"},
	},
	{
		Name: mcp.ToolName("nomad_restart_allocation"), Description: "Restart a task within a Nomad allocation. Optionally specify which task to restart.",
		Parameters: map[string]string{"alloc_id": "Allocation ID", "task": "Task name (optional, restarts all tasks if omitted)"},
		Required:   []string{"alloc_id"},
	},
	{
		Name: mcp.ToolName("nomad_read_allocation_logs"), Description: "Read stdout or stderr logs from a task in a Nomad allocation. Use for debugging container and workload issues.",
		Parameters: map[string]string{"alloc_id": "Allocation ID", "task": "Task name", "log_type": "Log type: stdout or stderr (default: stdout)", "plain": "Return plain text instead of JSON (true/false, default: true)", "origin": "Log origin: start or end (default: end)", "offset": "Byte offset to start reading from"},
		Required:   []string{"alloc_id", "task"},
	},

	// ── Nodes ────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("nomad_list_nodes"), Description: "List all client nodes in the Nomad cluster. Shows node status, datacenter, drivers, and scheduling eligibility.",
		Parameters: map[string]string{"prefix": "Filter by node ID prefix", "filter": "Filter expression (e.g., Status == \"ready\")"},
	},
	{
		Name: mcp.ToolName("nomad_get_node"), Description: "Get full details of a specific Nomad node, including attributes, resources, drivers, host volumes, and metadata.",
		Parameters: map[string]string{"node_id": "Node ID"},
		Required:   []string{"node_id"},
	},
	{
		Name: mcp.ToolName("nomad_get_node_allocations"), Description: "List all allocations placed on a specific Nomad node. Shows what workloads are running on the node.",
		Parameters: map[string]string{"node_id": "Node ID"},
		Required:   []string{"node_id"},
	},
	{
		Name: mcp.ToolName("nomad_drain_node"), Description: "Enable or disable drain mode on a Nomad node. Draining migrates all allocations off the node for maintenance.",
		Parameters: map[string]string{"node_id": "Node ID", "enable": "Enable drain (true) or disable (false)", "deadline": "Drain deadline duration (e.g., '1h', '30m'). Use -1 for no deadline", "ignore_system_jobs": "Skip draining system jobs (true/false, default: false)"},
		Required:   []string{"node_id", "enable"},
	},
	{
		Name: mcp.ToolName("nomad_node_eligibility"), Description: "Toggle scheduling eligibility for a Nomad node. Ineligible nodes won't receive new allocations but keep existing ones.",
		Parameters: map[string]string{"node_id": "Node ID", "eligible": "Set eligible (true) or ineligible (false)"},
		Required:   []string{"node_id", "eligible"},
	},

	// ── Deployments ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("nomad_list_deployments"), Description: "List deployments across the Nomad cluster. Shows rolling update status, canary progress, and deployment health.",
		Parameters: map[string]string{"namespace": "Namespace (default: default)", "prefix": "Filter by deployment ID prefix"},
	},
	{
		Name: mcp.ToolName("nomad_get_deployment"), Description: "Get details of a specific Nomad deployment, including task group status, health, and canary information.",
		Parameters: map[string]string{"deployment_id": "Deployment ID"},
		Required:   []string{"deployment_id"},
	},
	{
		Name: mcp.ToolName("nomad_promote_deployment"), Description: "Promote canary allocations in a Nomad deployment. Moves canaries to production after validation.",
		Parameters: map[string]string{"deployment_id": "Deployment ID", "all": "Promote all task groups (true/false, default: true)", "groups": "Comma-separated list of task groups to promote (alternative to all)"},
		Required:   []string{"deployment_id"},
	},
	{
		Name: mcp.ToolName("nomad_fail_deployment"), Description: "Mark a Nomad deployment as failed, triggering automatic rollback to the previous job version.",
		Parameters: map[string]string{"deployment_id": "Deployment ID"},
		Required:   []string{"deployment_id"},
	},

	// ── Evaluations ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("nomad_list_evaluations"), Description: "List evaluations in the Nomad scheduler queue. Shows scheduling decisions, blocked evaluations, and failures.",
		Parameters: map[string]string{"namespace": "Namespace (default: default)", "prefix": "Filter by evaluation ID prefix", "filter": "Filter expression (e.g., Status == \"blocked\")"},
	},

	// ── Services ─────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("nomad_list_services"), Description: "List all registered services in the Nomad service discovery catalog.",
		Parameters: map[string]string{"namespace": "Namespace (default: default)"},
	},

	// ── Cluster ──────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("nomad_get_agent_self"), Description: "Get the current Nomad agent's configuration, stats, and node information. Useful for diagnosing agent state.",
		Parameters: map[string]string{},
	},
	{
		Name: mcp.ToolName("nomad_get_cluster_status"), Description: "Get Nomad cluster status including Raft leader address and peer list. Use for cluster health checks.",
		Parameters: map[string]string{},
	},
	{
		Name: mcp.ToolName("nomad_gc"), Description: "Trigger garbage collection on the Nomad cluster to clean up dead allocations, evaluations, and deployments.",
		Parameters: map[string]string{},
	},
}
